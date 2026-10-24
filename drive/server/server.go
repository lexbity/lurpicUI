// Package server implements the LDP transport: a Unix-socket listener with
// token auth, grant enforcement, per-connection ordering, idempotent
// retries, and panic containment (QP-1..QP-4, QP-16). The package imports
// nothing from runtime; the runtime-side seam is the Sink interface,
// declared here (Principle 10: interfaces live in the consumer).
package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// Grants is the capability bitmap baked into the connection at Listen time
// (QP-4). The launcher encodes the grant set into the token via
// LURPIC_DRIVE=on:read,input; the server parses it once.
type Grants struct {
	Read, Input, Write, Time, Admin bool
}

// ParseGrants parses a comma-separated grant list ("read,input"). Unknown
// grants are an error — fail-loud, no silent best-effort subset.
func ParseGrants(s string) (Grants, error) {
	var g Grants
	if s == "" {
		return g, nil
	}
	for _, name := range strings.Split(s, ",") {
		switch strings.TrimSpace(name) {
		case grantRead:
			g.Read = true
		case grantInput:
			g.Input = true
		case grantWrite:
			g.Write = true
		case grantTime:
			g.Time = true
		case grantAdmin:
			g.Admin = true
		default:
			return Grants{}, fmt.Errorf("drive: unknown grant %q (want read, input, write, time, admin)", name)
		}
	}
	return g, nil
}

// Allows reports whether the grant set authorizes the named grant. Admin
// implies every grant. This is the single grant decision table: the server
// checks it per connection request, and the runtime re-checks it per
// frame.run step.
func (g Grants) Allows(name string) bool {
	if name == "" {
		return true
	}
	if g.Admin {
		return true
	}
	switch name {
	case grantRead:
		return g.Read
	case grantInput:
		return g.Input
	case grantWrite:
		return g.Write
	case grantTime:
		return g.Time
	case grantAdmin:
		return false
	default:
		return false
	}
}

// String renders the grant set as the canonical comma list.
func (g Grants) String() string {
	names := make([]string, 0, 5)
	for _, entry := range []struct {
		on bool
		n  string
	}{
		{g.Read, grantRead}, {g.Input, grantInput}, {g.Write, grantWrite},
		{g.Time, grantTime}, {g.Admin, grantAdmin},
	} {
		if entry.on {
			names = append(names, entry.n)
		}
	}
	return strings.Join(names, ",")
}

// Stats is the shared, thread-safe drive counter set (§3.9). One instance is
// owned by the runtime sink and shared with the server so both sides count
// into the same FrameStats surface and sys.info output.
type Stats struct {
	commandsDrained       atomic.Uint64
	responsesSent         atomic.Uint64
	grantDenied           atomic.Uint64
	socketGoroutinePanics atomic.Uint64
}

// AddCommandsDrained and friends count into the shared drive counter set;
// exported because both the server (connection goroutines) and the runtime
// sink (runtime thread) own different counters.
func (s *Stats) AddCommandsDrained(n int) { s.commandsDrained.Add(uint64(n)) } //nolint:gosec // count is bounded by drained commands

func (s *Stats) AddResponseSent()         { s.responsesSent.Add(1) }
func (s *Stats) AddGrantDenied()          { s.grantDenied.Add(1) }
func (s *Stats) AddSocketGoroutinePanic() { s.socketGoroutinePanics.Add(1) }

// Snapshot returns the cumulative counter values.
func (s *Stats) Snapshot() (commandsDrained, responsesSent, grantDenied, socketGoroutinePanics uint64) {
	return s.commandsDrained.Load(), s.responsesSent.Load(),
		s.grantDenied.Load(), s.socketGoroutinePanics.Load()
}

// Command is one driver request crossing the seam toward the runtime thread.
type Command struct {
	ID     uint64
	Method string
	Params json.RawMessage
	Grants Grants
	Reply  chan Result // buffered 1; the runtime thread sends exactly once
}

// Result is the runtime thread's reply to a Command.
type Result struct {
	Payload json.RawMessage
	Err     *proto.Error
	Frame   uint64
}

// Sink is the runtime-side seam (QP-1). Implementations drain at a cut on
// the runtime thread and reply exactly once per command.
type Sink interface {
	// Enqueue is thread-safe; the runtime drains the command at the next cut.
	Enqueue(cmd Command)
	// Epoch reports the current frame epoch.
	Epoch() uint64
}

// Config configures Listen.
type Config struct {
	SocketPath string // absolute; parent dir created 0700
	Token      string // 32 random bytes, hex, by the launcher
	Grants     Grants // fixed for every connection of this server
	Sink       Sink   // must be non-nil
	Stats      *Stats // optional; a private one is created when nil
}

// idempotencyCapacity is the per-connection retry window (QP-3).
const idempotencyCapacity = 1024

// Grant names (the canonical vocabulary across tables, Allows, and the
// parser).
const (
	grantRead  = "read"
	grantInput = "input"
	grantWrite = "write"
	grantTime  = "time"
	grantAdmin = "admin"
)

// Server owns the listener socket and its connections.
type Server struct {
	cfg   Config
	stats *Stats
	clock func() time.Time
	// hasSubscribers gates the runtime's publish work (zero subscribers ⇒
	// the cut flush does nothing).
	hasSubscribers atomic.Bool
	ln             net.Listener
	connsMu        sync.Mutex
	conns          map[*conn]struct{}
	wg             sync.WaitGroup
	stopCh         chan struct{}
	closed         bool
}

// Listen binds the Unix socket (mode 0600) and writes the token file
// (<path>.token, mode 0600) before accepting connections.
func Listen(cfg Config) (*Server, error) {
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("drive: SocketPath is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("drive: Token is required")
	}
	if cfg.Sink == nil {
		return nil, fmt.Errorf("drive: Sink is required")
	}
	if !filepath.IsAbs(cfg.SocketPath) {
		return nil, fmt.Errorf("drive: SocketPath %q must be absolute", cfg.SocketPath)
	}
	if err := os.MkdirAll(filepath.Dir(cfg.SocketPath), 0o700); err != nil {
		return nil, fmt.Errorf("drive: socket dir: %w", err)
	}
	tokenPath := cfg.SocketPath + ".token"
	if err := os.WriteFile(tokenPath, []byte(cfg.Token), 0o600); err != nil {
		return nil, fmt.Errorf("drive: token file: %w", err)
	}
	// A stale socket from a previous run would make Listen fail; remove it.
	// The 0600 token file next to it gates access, so the unlink is safe.
	if err := os.Remove(cfg.SocketPath); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("drive: remove stale socket: %w", err)
	}
	ln, err := net.Listen("unix", cfg.SocketPath)
	if err != nil {
		return nil, fmt.Errorf("drive: listen: %w", err)
	}
	if err := os.Chmod(cfg.SocketPath, 0o600); err != nil {
		_ = ln.Close()
		return nil, fmt.Errorf("drive: socket mode: %w", err)
	}
	s := &Server{
		cfg:    cfg,
		stats:  cfg.Stats,
		clock:  time.Now,
		ln:     ln,
		conns:  make(map[*conn]struct{}),
		stopCh: make(chan struct{}),
	}
	if s.stats == nil {
		s.stats = &Stats{}
	}
	s.wg.Add(1)
	go s.acceptLoop()

	// Event fan-out registration (QP-1: the server declares the consumer-side
	// seam; the runtime sink opts in when it supports publishing).
	if consumer, ok := cfg.Sink.(interface {
		SetEventConsumer(func(class, store, testID string, params json.RawMessage))
	}); ok {
		consumer.SetEventConsumer(func(class, store, testID string, params json.RawMessage) {
			s.fanOut(class, store, testID, params)
		})
	}
	return s, nil
}

// fanOut routes one event to every connection's subscription table.
func (s *Server) fanOut(class, store, testID string, params json.RawMessage) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for c := range s.conns {
		c.fanOut(class, store, testID, params)
	}
}

// clockNowEpoch reports the sink's current epoch for replies that never
// cross the runtime (local methods).
func (s *Server) clockNowEpoch() uint64 { return s.cfg.Sink.Epoch() }

// SocketPath reports the bound socket path.
func (s *Server) SocketPath() string { return s.cfg.SocketPath }

func (s *Server) acceptLoop() {
	defer s.wg.Done()
	// QP-16: a panic in the listener goroutine shuts the socket down and
	// counts; it never takes the app down.
	defer func() {
		if r := recover(); r != nil {
			s.stats.AddSocketGoroutinePanic()
			_ = s.Close()
		}
	}()
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
			}
			return
		}
		c := &conn{srv: s, nc: nc}
		s.connsMu.Lock()
		if s.closed {
			s.connsMu.Unlock()
			_ = nc.Close()
			continue
		}
		s.conns[c] = struct{}{}
		s.connsMu.Unlock()
		s.wg.Add(1)
		go c.serve()
	}
}

// Close unlinks the socket and token file and detaches every client.
func (s *Server) Close() error {
	s.connsMu.Lock()
	if s.closed {
		s.connsMu.Unlock()
		return nil
	}
	s.closed = true
	close(s.stopCh)
	for c := range s.conns {
		_ = c.nc.Close()
	}
	s.connsMu.Unlock()
	err := s.ln.Close()
	_ = os.Remove(s.cfg.SocketPath)
	_ = os.Remove(s.cfg.SocketPath + ".token")
	s.wg.Wait()
	return err
}

// conn is one authenticated-or-pending client connection. The serve
// goroutine reads requests and writes responses; the subscription pump
// writes notifications (P7). Socket writes are serialized by writeMu so the
// two writers never interleave mid-line; per-connection request ordering
// (QP-3) is unaffected.
type conn struct {
	srv      *Server
	nc       net.Conn
	cache    *responseCache
	authed   bool
	subs     *subTable
	writeMu  sync.Mutex
	pumpOnce sync.Once
	stopPump chan struct{}
}

// writeLine serializes one line onto the socket. A line already ending in
// a newline (the response encoder's terminator) passes through unchanged;
// bare notification payloads get one appended.
func (c *conn) writeLine(line []byte) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()
	if len(line) > 0 && line[len(line)-1] == '\n' {
		_, _ = c.nc.Write(line)
		return
	}
	_, _ = c.nc.Write(append(line, '\n'))
}

// startPump launches the subscription writer goroutine (once): drains each
// subscription FIFO to the socket and detaches on backpressure deadlines
// (QP-13: detached loudly with sub.dead, never blocking the runtime thread).
// The pump wakes on the signal channel (publish/subscribe/detach all notify
// it) — no sleep-polling.
func (c *conn) startPump() {
	c.pumpOnce.Do(func() {
		go func() {
			for {
				select {
				case <-c.stopPump:
					return
				case id := <-c.subs.detachCh:
					c.subs.cancel(id)
					n := proto.Notification{
						Method: "sub.dead",
						Params: json.RawMessage(`{"id":` + fmt.Sprint(id) + `,"reason":"backpressure"}`),
					}
					if out, err := json.Marshal(n); err == nil {
						c.writeLine(out)
					}
				case <-c.subs.activity:
				}
				// Drain everything currently queued before parking again.
				c.subs.mu.Lock()
				for _, sub := range c.subs.subs {
					for {
						select {
						case n := <-sub.fifo:
							if out, err := json.Marshal(n); err == nil {
								c.writeLine(out)
							}
							continue
						default:
						}
						break
					}
				}
				c.subs.mu.Unlock()
			}
		}()
	})
}

// fanOut routes one runtime event into this connection's subscriptions.
func (c *conn) fanOut(class, store, testID string, params json.RawMessage) {
	c.subs.publish(class, store, testID, params)
}

// helloParams is the sys.hello parameter payload.
type helloParams struct {
	Token string `json:"token"`
}

func (c *conn) serve() {
	defer c.srv.wg.Done()
	defer c.srv.removeConn(c)
	// QP-16: a panic on the connection goroutine is recovered, counted, and
	// the connection is dropped. The app keeps rendering (NFR-7).
	defer func() {
		if r := recover(); r != nil {
			c.srv.stats.AddSocketGoroutinePanic()
			_ = c.nc.Close()
		}
	}()
	reader := bufio.NewReaderSize(c.nc, 64*1024)
	writer := bufio.NewWriter(c.nc)
	c.cache = newResponseCache(idempotencyCapacity)
	c.subs = newSubTable(c.srv.clock)
	defer close(c.stopPump)
	c.startPump()

	first := true
	for {
		line, err := readLine(reader)
		if err != nil {
			if errors.Is(err, errLineTooLong) {
				// Typed error with no request id to attribute (the line never
				// decoded); not cached, since caching is id-keyed.
				res := proto.Response{OK: false, Error: &proto.Error{
					Code:   proto.ErrInternal,
					Detail: fmt.Sprintf("line exceeds %d bytes", proto.MaxLineBytes),
				}, Frame: c.srv.cfg.Sink.Epoch()}
				c.srv.stats.AddResponseSent()
				if enc, err := json.Marshal(res); err == nil {
					enc = append(enc, '\n')
					_, _ = writer.Write(enc)
					_ = writer.Flush()
				}
			}
			_ = c.nc.Close()
			return
		}
		if len(line) > 0 {
			if _, closeAfter := c.handleLine(line, first, writer); closeAfter {
				_ = writer.Flush()
				_ = c.nc.Close()
				return
			}
		}
		_ = writer.Flush()
		first = false
	}
}

// errLineTooLong marks a line that exceeded the 4 MiB input bound (NFR-9).
var errLineTooLong = fmt.Errorf("line exceeds %d bytes", proto.MaxLineBytes)

// readLine reads one newline-terminated line without the trailing newline.
// A line longer than MaxLineBytes is consumed and reported as
// errLineTooLong; the caller answers a typed error and closes.
func readLine(r *bufio.Reader) ([]byte, error) {
	var buf []byte
	for {
		chunk, err := r.ReadSlice('\n')
		if errors.Is(err, bufio.ErrBufferFull) {
			buf = append(buf, chunk...)
			if len(buf) > proto.MaxLineBytes {
				return nil, errLineTooLong
			}
			continue
		}
		if err != nil {
			return nil, err
		}
		line := chunk[:len(chunk)-1]
		if len(buf) > 0 {
			if len(buf)+len(line) > proto.MaxLineBytes {
				return nil, errLineTooLong
			}
			return append(buf, line...), nil
		}
		return line, nil
	}
}

// handleLine decodes and executes one request line. It returns whether the
// connection must close after the response is flushed.
func (c *conn) handleLine(line []byte, first bool, w *bufio.Writer) (resp proto.Response, closeAfter bool) {
	var req proto.Request
	if err := json.Unmarshal(line, &req); err != nil {
		return c.writeError(w, 0, &proto.Error{
			Code:   proto.ErrInternal,
			Detail: "malformed request: " + err.Error(),
		}), !c.authed
	}
	if req.V != proto.ProtocolMajor {
		return c.writeError(w, req.ID, &proto.Error{
			Code:   proto.ErrVersion,
			Detail: fmt.Sprintf("client major %d, server major %d", req.V, proto.ProtocolMajor),
		}), true
	}
	if !c.authed {
		if !first || req.Method != "sys.hello" {
			return c.writeError(w, req.ID, &proto.Error{
				Code:   proto.ErrAuth,
				Detail: "first request must be sys.hello",
			}), true
		}
		return c.hello(req, w)
	}
	return c.dispatch(req, w), false
}

func (c *conn) hello(req proto.Request, w *bufio.Writer) (resp proto.Response, closeAfter bool) {
	var hp helloParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &hp); err != nil {
			return c.writeError(w, req.ID, &proto.Error{
				Code:   proto.ErrAuth,
				Detail: "malformed sys.hello params",
			}), true
		}
	}
	if hp.Token != c.srv.cfg.Token {
		return c.writeError(w, req.ID, &proto.Error{Code: proto.ErrAuth}), true
	}
	c.authed = true
	return c.writeResult(w, req, json.RawMessage(fmt.Sprintf(`{"v":%d}`, proto.ProtocolMajor)), 0), false
}

// methodSpec is the server-side dispatch entry. Grant is enforced before
// execution (FR-3); local methods are answered in-connection without
// crossing the sink.
type methodSpec struct {
	grant string // "" = no grant needed
	local bool   // handled on the connection goroutine
}

// methods is the method table. Unknown methods fail with a typed error,
// never a panic (NFR-9). frame.run's grant is the union of its steps —
// checked per step by the runtime against Grants.Allows — so the connection
// level needs no grant of its own.
var methods = map[string]methodSpec{
	"sub.watch":      {local: true, grant: grantRead},
	"sub.cancel":     {local: true, grant: grantRead},
	"sys.hello":      {local: true},
	"sys.ping":       {grant: "read"},
	"sys.info":       {grant: "read"},
	"locate.resolve": {grant: "read"},
	"tree.dump":      {grant: "read"},
	"store.list":     {grant: "read"},
	"store.read":     {grant: "read"},
	"derived.read":   {grant: "read"},
	"facet.query":    {grant: "read"},
	"frame.pixels":   {grant: "read"},
	"frame.packet":   {grant: "read"},
	"store.write":    {grant: "write"},
	"input.click":    {grant: grantInput},
	"input.press":    {grant: grantInput},
	"input.release":  {grant: grantInput},
	"input.move":     {grant: grantInput},
	"input.drag":     {grant: grantInput},
	"input.type":     {grant: grantInput},
	"input.scroll":   {grant: grantInput},
	"input.focus":    {grant: grantInput},
	"cmd.list":       {grant: "read"},
	"cmd.execute":    {grant: "write"},
	"time.mode":      {grant: "time"},
	"time.advance":   {grant: "time"},
	"frame.run":      {},
}

// MethodGrant reports the grant a method requires ("" = none at connection
// level). The authoritative table is proto.Registry — this wrapper keeps the
// connection-level grant enforcement and the runtime's per-step checks on
// the single source (P8 registry).
func MethodGrant(method string) (string, bool) {
	return proto.MethodGrant(method)
}

func (c *conn) dispatch(req proto.Request, w *bufio.Writer) proto.Response {
	if cached, ok := c.cache.get(req.ID); ok {
		cached.Cached = true
		c.emit(w, cached)
		return cached
	}
	spec, known := methods[req.Method]
	if !known {
		return c.writeError(w, req.ID, &proto.Error{
			Code:   proto.ErrInternal,
			Detail: "unknown method: " + req.Method,
		})
	}
	if !c.hasGrant(spec.grant) {
		c.srv.stats.AddGrantDenied()
		return c.writeError(w, req.ID, &proto.Error{
			Code:   proto.ErrGrant,
			Detail: spec.grant,
		})
	}
	if spec.local {
		switch req.Method {
		case "sub.watch":
			return c.subWatch(req, w)
		case "sub.cancel":
			return c.subCancel(req, w)
		default:
			return c.writeError(w, req.ID, &proto.Error{
				Code:   proto.ErrInternal,
				Detail: "method " + req.Method + " is not available post-handshake",
			})
		}
	}
	// Cross the seam: the runtime thread executes at the next cut and
	// replies exactly once (QP-1, QP-6).
	cmd := Command{
		ID:     req.ID,
		Method: req.Method,
		Params: req.Params,
		Grants: c.srv.cfg.Grants,
		Reply:  make(chan Result, 1),
	}
	c.srv.cfg.Sink.Enqueue(cmd)
	result := <-cmd.Reply
	if result.Err != nil {
		return c.writeError(w, req.ID, result.Err)
	}
	return c.writeResult(w, req, result.Payload, result.Frame)
}

// subWatchParams is the sub.watch payload.
type subWatchParams struct {
	Filter Filter `json:"filter"`
}

// subWatch registers a subscription filter and replies with its id
// (QP-13). Events flow from the runtime's cut flush through the server's
// fan-out into this connection's FIFO.
func (c *conn) subWatch(req proto.Request, w *bufio.Writer) proto.Response {
	var p subWatchParams
	if len(req.Params) > 0 {
		if err := json.Unmarshal(req.Params, &p); err != nil {
			return c.writeError(w, req.ID, &proto.Error{Code: proto.ErrInternal, Detail: "sub.watch params: " + err.Error()})
		}
	}
	switch p.Filter.Class {
	case classStateChanged, classTreeChanged, classFrameDone, classOwnPreempted:
	default:
		return c.writeError(w, req.ID, &proto.Error{
			Code:   proto.ErrInternal,
			Detail: `filter.class must be state.changed, tree.changed, frame.completed, or own.preempted`,
		})
	}
	id := c.subs.subscribe(p.Filter)
	payload, err := json.Marshal(map[string]uint64{"id": id})
	if err != nil {
		return c.writeError(w, req.ID, &proto.Error{Code: proto.ErrInternal, Detail: err.Error()})
	}
	c.srv.connsMu.Lock()
	c.srv.hasSubscribers.Store(true)
	c.srv.connsMu.Unlock()
	return c.writeResult(w, req, payload, c.srv.clockNowEpoch())
}

// subCancelParams is the sub.cancel payload.
type subCancelParams struct {
	ID uint64 `json:"id"`
}

// subCancel removes a subscription.
func (c *conn) subCancel(req proto.Request, w *bufio.Writer) proto.Response {
	var p subCancelParams
	if len(req.Params) == 0 || json.Unmarshal(req.Params, &p) != nil {
		return c.writeError(w, req.ID, &proto.Error{Code: proto.ErrInternal, Detail: "sub.cancel requires id"})
	}
	if !c.subs.cancel(p.ID) {
		return c.writeError(w, req.ID, &proto.Error{Code: proto.ErrUnresolved, Detail: "no subscription"})
	}
	return c.writeResult(w, req, json.RawMessage(`{"cancelled":true}`), c.srv.clockNowEpoch())
}

func (c *conn) hasGrant(name string) bool {
	return c.srv.cfg.Grants.Allows(name)
}

func (c *conn) writeResult(w *bufio.Writer, req proto.Request, payload json.RawMessage, frame uint64) proto.Response {
	res := proto.Response{ID: req.ID, OK: true, Result: payload, Frame: frame}
	c.finish(w, res)
	return res
}

func (c *conn) writeError(w *bufio.Writer, id uint64, perr *proto.Error) proto.Response {
	res := proto.Response{ID: id, OK: false, Error: perr, Frame: c.srv.cfg.Sink.Epoch()}
	c.finish(w, res)
	return res
}

func (c *conn) finish(w *bufio.Writer, res proto.Response) {
	if c.authed {
		c.cache.put(res.ID, res)
	}
	c.srv.stats.AddResponseSent()
	c.emit(w, res)
}

// mustNoPayloadResponse encodes an internal error response with no raw
// payload fields. Marshal of a payload-free response cannot fail; a panic
// here would be contained by the connection recover anyway.
func mustNoPayloadResponse(res proto.Response) []byte {
	enc, err := json.Marshal(proto.Response{
		ID:    res.ID,
		OK:    false,
		Error: &proto.Error{Code: proto.ErrInternal, Detail: "response encoding failed"},
		Frame: res.Frame,
	})
	if err != nil {
		panic("drive: payload-free response marshal failed: " + err.Error())
	}
	return enc
}

// emit marshals and writes one response; it neither caches nor counts
// (used for cache hits, which were already counted when first produced).
func (c *conn) emit(w *bufio.Writer, res proto.Response) {
	out, err := json.Marshal(res)
	if err != nil {
		// Marshal of already-raw payloads cannot fail in practice; if it
		// does, contain it as a typed error rather than killing the app.
		c.srv.stats.AddSocketGoroutinePanic()
		// The fallback carries no raw payload, so its marshal cannot fail.
		fallback := mustNoPayloadResponse(res)
		out = append(fallback, '\n')
	} else {
		out = append(out, '\n')
	}
	c.writeLine(out)
	_ = w
}

func (s *Server) removeConn(c *conn) {
	s.connsMu.Lock()
	delete(s.conns, c)
	s.connsMu.Unlock()
}
