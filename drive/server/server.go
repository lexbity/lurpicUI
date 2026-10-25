// Package server implements the LDP v2 transport: a Unix-socket listener
// with launcher-token auth, resumable app-instance sessions (D1), bounded
// reader/writer separation, strict envelope framing, and panic containment.
// The package imports nothing from runtime; the runtime side of the seam is
// drive.RuntimePort, declared in the drive root and installed explicitly at
// app construction before Listen (Principle 10).
package server

import (
	"bufio"
	"bytes"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// Transport policy constants (the finite defaults of the RFC table).
const (
	// outboxCapacity bounds each connection's writer queue; overflow is a
	// transport fault, never unbounded memory.
	outboxCapacity = 256
	// writeDeadline bounds one blocked write; expiry closes the connection.
	writeDeadline = 2 * time.Second
	// lineDeadline bounds one incomplete line, starting at its first byte.
	lineDeadline = 2 * time.Second
	// helloDeadline bounds the time to the authenticated hello from accept.
	helloDeadline = 2 * time.Second
	// maxClientNameBytes bounds sys.hello's client_name.
	maxClientNameBytes = 128
	// maxSessionsPerInstance mirrors drive.MaxSessions for the connection
	// bound (64 live authenticated connections).
	maxConnections = 64
)

// Config configures Listen.
type Config struct {
	// SocketPath is the absolute private Unix-socket path.
	SocketPath string
	// Token is the launcher token: 32 random bytes hex-encoded (64 chars).
	Token string
	// Grants is the launcher's grant policy applied to newly created
	// sessions. Resume never widens a session's grants.
	Grants drive.Grants
	// Sessions is the instance session table (owned by the app wiring).
	Sessions *drive.SessionTable
	// Port is the runtime side of the seam; installed before Listen.
	Port drive.RuntimePort
	// Stats is the shared drive counter set; a private one when nil.
	Stats *drive.Stats
	// Now is the wall clock (injectable for tests); time.Now when nil.
	Now func() time.Time
	// Supervisor is signaled when the listener goroutine fails fatally
	// (panic or accept error while not closing). The default logs to
	// stderr; a production app installs a shutdown signal.
	Supervisor func(err error)

	// acceptHook is an in-package test injection point invoked once when
	// the accept loop starts (nil in production). It lives on Config so
	// it is written before Listen starts the accept goroutine.
	acceptHook func()
}

// Server owns the listener socket and its connections, and implements the
// runtime-facing drive.TransportEvents side of the seam.
type Server struct {
	cfg        Config
	stats      *drive.Stats
	now        func() time.Time
	ln         net.Listener
	nextConn   sync.WaitGroup
	connsMu    sync.Mutex
	conns      map[*conn]struct{}
	nextConnID atomic.Uint64
	stopCh     chan struct{}
	stopOnce   sync.Once
	closed     bool
	// acceptPanicSeen is unused bookkeeping; the accept goroutine exits on
	// its own and Close joins only the per-connection goroutines.
	socketDev uint64 // owned socket inode (dev,ino for cleanup)
	socketIno uint64
	tokenDev  uint64
	tokenIno  uint64
}

// Listen binds the Unix socket and writes the token file under strict
// ownership rules, then starts accepting. It never unlinks a pre-existing
// socket or token: stale-path cleanup is an explicit operator action.
func Listen(cfg Config) (*Server, error) {
	if cfg.SocketPath == "" {
		return nil, fmt.Errorf("drive: SocketPath is required")
	}
	if cfg.Token == "" {
		return nil, fmt.Errorf("drive: Token is required")
	}
	if cfg.Port == nil {
		return nil, fmt.Errorf("drive: Port is required")
	}
	if cfg.Sessions == nil {
		return nil, fmt.Errorf("drive: Sessions table is required")
	}
	if !filepath.IsAbs(cfg.SocketPath) {
		return nil, fmt.Errorf("drive: SocketPath %q must be absolute", cfg.SocketPath)
	}
	if err := validateToken(cfg.Token); err != nil {
		return nil, err
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	bound, err := bind(cfg.SocketPath, cfg.Token)
	if err != nil {
		return nil, err
	}
	s := &Server{
		cfg:    cfg,
		stats:  cfg.Stats,
		now:    cfg.Now,
		ln:     bound.listener,
		conns:  make(map[*conn]struct{}),
		stopCh: make(chan struct{}),
	}
	if s.stats == nil {
		s.stats = &drive.Stats{}
	}
	s.socketDev, s.socketIno = bound.socketDev, bound.socketIno
	s.tokenDev, s.tokenIno = bound.tokenDev, bound.tokenIno
	go s.acceptLoop()
	return s, nil
}

// validateToken enforces the 32-byte hex token grammar before any
// comparison.
func validateToken(token string) error {
	if len(token) != 64 {
		return fmt.Errorf("drive: token must be 64 hex characters (32 bytes), got %d", len(token))
	}
	if _, err := hex.DecodeString(token); err != nil {
		return fmt.Errorf("drive: token is not hex: %w", err)
	}
	return nil
}

// SocketPath reports the bound socket path.
func (s *Server) SocketPath() string { return s.cfg.SocketPath }

// InstanceID reports the owning instance identity.
func (s *Server) InstanceID() string { return s.cfg.Sessions.InstanceID() }

// CloseTransport implements drive.TransportEvents: the runtime (or server
// policy) closes one connection; the reader observes the loss and fences
// the owning session's work through the control signal path.
func (s *Server) CloseTransport(connID drive.ConnectionID, reason drive.TransportReason) {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	for c := range s.conns {
		if c.id == connID {
			c.close(reason)
		}
	}
}

// ConnectionCount reports live connections (diagnostics).
func (s *Server) ConnectionCount() int {
	s.connsMu.Lock()
	defer s.connsMu.Unlock()
	return len(s.conns)
}

func (s *Server) supervisorError(err error) {
	if s.cfg.Supervisor != nil {
		s.cfg.Supervisor(err)
		return
	}
	fmt.Fprintf(os.Stderr, "drive: listener fatal: %v\n", err)
}

func (s *Server) acceptLoop() {
	// A panic in the listener goroutine signals the supervisor and closes
	// the listener; it never takes the app down. The close skips the
	// goroutine wait: this goroutine's own WaitGroup slot releases only
	// after this defer completes, so joining here would deadlock.
	defer func() {
		if r := recover(); r != nil {
			s.stats.AddSocketGoroutinePanic()
			err := fmt.Errorf("listener panic: %v", r)
			s.supervisorError(err)
			s.stopTransport(false)
		}
	}()
	if s.cfg.acceptHook != nil {
		s.cfg.acceptHook()
	}
	for {
		nc, err := s.ln.Accept()
		if err != nil {
			select {
			case <-s.stopCh:
				return
			default:
			}
			s.supervisorError(fmt.Errorf("accept: %w", err))
			_ = s.Close()
			return
		}
		s.connsMu.Lock()
		if s.closed || len(s.conns) >= maxConnections {
			s.connsMu.Unlock()
			_ = nc.Close()
			continue
		}
		c := &conn{
			srv:    s,
			id:     drive.ConnectionID(s.nextConnID.Add(1)),
			nc:     nc,
			outbox: make(chan []byte, outboxCapacity),
			done:   make(chan struct{}),
		}
		s.conns[c] = struct{}{}
		s.connsMu.Unlock()
		s.nextConn.Add(2)
		go c.writeLoop()
		go c.serve()
	}
}

// Close stops the transport: it fences connection generations, cancels
// readers/writers, unlinks only its own inodes, and never waits for an
// individual runtime reply.
func (s *Server) Close() error {
	s.stopTransport(true)
	return nil
}

// stopTransport performs the close body once. join controls whether the
// caller joins the connection goroutines: a self-close from the accept
// loop must not join, because that goroutine cannot wait on itself.
func (s *Server) stopTransport(join bool) {
	s.stopOnce.Do(s.stopBody)
	if !join {
		return
	}
	// Join the connection goroutines; readers blocked on a runtime reply
	// abort via their done channel and never hang the close.
	s.nextConn.Wait()
}

// stopBody is the idempotent close body.
func (s *Server) stopBody() {
	s.connsMu.Lock()
	s.closed = true
	close(s.stopCh)
	conns := make([]*conn, 0, len(s.conns))
	for c := range s.conns {
		conns = append(conns, c)
	}
	s.connsMu.Unlock()
	for _, c := range conns {
		c.close(drive.ReasonServerStop)
	}
	_ = s.ln.Close()
	s.unlinkOwned(s.cfg.SocketPath, s.socketDev, s.socketIno)
	s.unlinkOwned(s.cfg.SocketPath+".token", s.tokenDev, s.tokenIno)
}

// unlinkOwned removes path only when its inode matches the one this server
// created — cleanup never deletes an unrelated file.
func (s *Server) unlinkOwned(path string, dev, ino uint64) {
	st, err := os.Lstat(path)
	if err != nil {
		return
	}
	d, i := fileIdentity(st)
	if d == dev && i == ino {
		_ = os.Remove(path)
	}
}

// conn is one client connection: one reader goroutine (this serve), one
// writer goroutine draining the bounded outbox with a write deadline.
type conn struct {
	srv    *Server
	id     drive.ConnectionID
	nc     net.Conn
	outbox chan []byte
	done   chan struct{}
	// writeMu serializes close vs. writer-fault bookkeeping.
	closeOnce sync.Once

	authed  bool
	session *drive.Session

	// draining marks a controlled close: the writer finishes the queued
	// lines before the socket closes, so a final typed error still lands.
	draining atomic.Bool
	// pendingWriteLines counts lines enqueued but not yet written; the
	// graceful close waits for zero (channel length alone would race the
	// receive-before-write window).
	pendingWriteLines atomic.Int64
}

// close tears the connection down exactly once; every goroutine observing
// done aborts its pending work.
// finish stops the reader and closes the connection after the writer has
// flushed the queued lines.
func (c *conn) finish(reason drive.TransportReason) {
	c.draining.Store(true)
	_ = c.nc.SetReadDeadline(time.Time{})
	go func() {
		deadline := time.Now().Add(writeDeadline)
		for c.pendingWriteLines.Load() > 0 && time.Now().Before(deadline) {
			time.Sleep(50 * time.Microsecond)
		}
		c.close(reason)
	}()
}

func (c *conn) close(reason drive.TransportReason) {
	c.closeOnce.Do(func() {
		_ = c.nc.Close()
		close(c.done)
		c.srv.connsMu.Lock()
		delete(c.srv.conns, c)
		c.srv.connsMu.Unlock()
		if c.session != nil {
			gen := c.session.Generation()
			c.session.Detach(c.srv.now())
			c.srv.cfg.Port.SignalControl(drive.ControlSignal{
				Kind:       drive.SignalConnectionLost,
				Reason:     reason,
				Session:    c.session.ID,
				Generation: gen,
			})
			c.session = nil
		}
	})
}

// sendLine enqueues one encoded line onto the writer's outbox. A full
// outbox is a transport fault: the peer stopped reading, so the connection
// closes (never unbounded buffering, never blocking the reader).
func (c *conn) sendLine(line []byte) bool {
	c.pendingWriteLines.Add(1)
	select {
	case c.outbox <- line:
		return true
	default:
		c.pendingWriteLines.Add(-1)
		c.close(drive.ReasonWriteFault)
		return false
	}
}

func (c *conn) writeLoop() {
	defer c.srv.nextConn.Done()
	for {
		select {
		case <-c.done:
			return
		case line := <-c.outbox:
			_ = c.nc.SetWriteDeadline(c.srv.now().Add(writeDeadline))
			if _, err := c.nc.Write(line); err != nil {
				c.pendingWriteLines.Add(-1)
				c.close(drive.ReasonWriteFault)
				return
			}
			c.pendingWriteLines.Add(-1)
		}
	}
}

func (c *conn) serve() {
	defer c.srv.nextConn.Done()
	defer func() {
		if r := recover(); r != nil {
			c.srv.stats.AddSocketGoroutinePanic()
			fmt.Fprintf(os.Stderr, "drive: connection goroutine panic: %v\n", r)
			c.close(drive.ReasonWriteFault)
		}
	}()
	reader := bufio.NewReaderSize(c.nc, 64*1024)
	// Hello must arrive within its own deadline from accept.
	_ = c.nc.SetReadDeadline(c.srv.now().Add(helloDeadline))
	for {
		// Incomplete-line deadlines start at the first byte of each line
		// and are absolute for that line.
		_ = c.nc.SetReadDeadline(c.srv.now().Add(lineDeadline))
		line, err := readLine(reader)
		if err != nil {
			if errors.Is(err, errLineTooLong) {
				c.sendError("", proto.Error{
					Code:        proto.ErrLineLimit,
					Detail:      err.Error(),
					NonAdmitted: true,
				}, true)
				c.finish(drive.ReasonIdle)
				return
			}
			c.close(drive.ReasonEOF)
			return
		}
		closeAfter := c.handleLine(line)
		if closeAfter {
			c.finish(c.closeReason())
			return
		}
	}
}

// closeReason reports why the connection closes after the response of a
// failed handshake.
func (c *conn) closeReason() drive.TransportReason {
	if c.authed {
		return drive.ReasonEOF
	}
	return drive.ReasonAuth
}

// errLineTooLong marks a line beyond the 4 MiB bound.
var errLineTooLong = fmt.Errorf("line exceeds %d bytes", proto.MaxLineBytes)

// readLine reads one newline-terminated line without the trailing newline.
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

// isLineLimit reports a line-limit rejection without allocating a target
// pointer per check.
func isLineLimit(err error) bool {
	var lle *proto.LineLimitError
	return errors.As(err, &lle)
}

// handleLine decodes and executes one request line. Returns whether the
// connection must close after the response flushes.
func (c *conn) handleLine(line []byte) bool {
	req, err := proto.DecodeRequest(line)
	if err != nil {
		var verr *proto.VersionError
		switch {
		case errors.As(err, &verr):
			// A wrong major is a hard handshake break.
			c.sendError(req.ID, proto.Error{
				Code:        proto.ErrVersionMismatch,
				Detail:      verr.Error(),
				NonAdmitted: true,
			}, true)
			return true
		case isLineLimit(err):
			c.sendError("", proto.Error{
				Code:        proto.ErrLineLimit,
				Detail:      err.Error(),
				NonAdmitted: true,
			}, true)
			return true
		default:
			// Malformed framing happens before admission and never consumes
			// an ID. Pre-auth it is indistinguishable from an attack: close
			// after one safe error; post-auth it is a typed, survivable
			// error.
			c.sendError(req.ID, proto.Error{
				Code:        proto.ErrMalformed,
				Detail:      err.Error(),
				NonAdmitted: true,
			}, !c.authed)
			return !c.authed
		}
	}
	if !c.authed {
		if req.Method != "sys.hello" {
			c.sendError(req.ID, proto.Error{
				Code:        proto.ErrAuth,
				Detail:      "first request must be sys.hello",
				NonAdmitted: true,
			}, true)
			return true
		}
		return c.hello(req)
	}
	if req.Method == "sys.hello" {
		// Re-hello on an authenticated connection cannot re-negotiate the
		// session silently; the client opens a new connection for that.
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrAuth,
			Detail:      "session renegotiation requires a new connection",
			NonAdmitted: true,
		}, false)
		return false
	}
	c.dispatch(req)
	return false
}

// helloParams is the strictly validated sys.hello payload: exactly one of
// create_session or resume.
type helloParams struct {
	Token         string        `json:"token"`
	ClientName    string        `json:"client_name,omitempty"`
	CreateSession bool          `json:"create_session,omitempty"`
	Resume        *resumeParams `json:"resume,omitempty"`
}

type resumeParams struct {
	InstanceID string `json:"instance_id"`
	SessionID  string `json:"session_id"`
	Credential string `json:"resume_credential"`
}

func (c *conn) hello(req proto.Request) bool {
	if len(req.Params) == 0 {
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrInvalidParams,
			Detail:      "sys.hello requires params",
			NonAdmitted: true,
		}, true)
		return true
	}
	var hp helloParams
	dec := json.NewDecoder(bytes.NewReader(req.Params))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&hp); err != nil {
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrInvalidParams,
			Detail:      "sys.hello params: " + err.Error(),
			NonAdmitted: true,
		}, true)
		return true
	}
	if len(hp.ClientName) > maxClientNameBytes {
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrInvalidParams,
			Detail:      "client_name exceeds 128 bytes",
			NonAdmitted: true,
		}, true)
		return true
	}
	if (hp.CreateSession && hp.Resume != nil) || (!hp.CreateSession && hp.Resume == nil) {
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrInvalidParams,
			Detail:      "exactly one of create_session or resume is required",
			NonAdmitted: true,
		}, true)
		return true
	}
	// Constant-time token comparison after strict length validation.
	if subtle.ConstantTimeCompare([]byte(hp.Token), []byte(c.srv.cfg.Token)) != 1 {
		c.sendError(req.ID, proto.Error{Code: proto.ErrAuth, NonAdmitted: true}, true)
		return true
	}

	var result json.RawMessage
	var err *proto.Error
	if hp.Resume != nil {
		result, err = c.resume(hp)
	} else {
		result, err = c.create(hp)
	}
	if err != nil {
		c.sendError(req.ID, *err, true)
		return true
	}
	c.authed = true
	c.sendResult(req.ID, result, "", "")
	// Clear the hello deadline; authenticated sessions own the keepalive
	// lease, not per-line idle deadlines.
	_ = c.nc.SetReadDeadline(time.Time{})
	return false
}

// create allocates a fresh session under the launcher's grant policy.
func (c *conn) create(hp helloParams) (json.RawMessage, *proto.Error) {
	sess, cred, err := c.srv.cfg.Sessions.Create(c.srv.cfg.Grants)
	if err != nil {
		return nil, &proto.Error{Code: proto.ErrCapacityBusy, Detail: err.Error(), Retryable: true, NonAdmitted: true}
	}
	c.session = sess
	out := map[string]any{
		"instance_id":           sess.InstanceID,
		"session_id":            sess.ID,
		"resume_credential":     cred,
		"connection_generation": strconv.FormatUint(sess.Generation(), 10),
		"next_id":               strconv.FormatUint(sess.Ledger().HighWater()+1, 10),
		"grants":                grantList(sess.Grants),
		"schema_digest":         proto.SchemaDigest(),
		"limits":                limitsDocument(),
	}
	enc, merr := json.Marshal(out)
	if merr != nil {
		return nil, &proto.Error{Code: proto.ErrInternalPanic, Detail: "hello encoding failed"}
	}
	return enc, nil
}

// resume authenticates and attaches a new connection generation, fencing
// the replaced generation's outstanding work.
func (c *conn) resume(hp helloParams) (json.RawMessage, *proto.Error) {
	if hp.Resume.InstanceID != c.srv.cfg.Sessions.InstanceID() {
		return nil, &proto.Error{Code: proto.ErrInstanceChanged, Detail: "instance identity mismatch"}
	}
	sess, gen, err := c.srv.cfg.Sessions.Resume(hp.Resume.SessionID, hp.Resume.Credential)
	if err != nil {
		return nil, &proto.Error{Code: proto.ErrSessionExpired, Detail: "unknown or expired session or credential"}
	}
	c.session = sess
	// Fence the replaced generation even if its old socket has not reported
	// EOF yet; a delayed old-generation EOF must not cancel new work.
	c.srv.cfg.Port.SignalControl(drive.ControlSignal{
		Kind:       drive.SignalConnectionLost,
		Reason:     drive.ReasonServerStop,
		Session:    sess.ID,
		Generation: gen - 1,
	})
	out := map[string]any{
		"instance_id":           sess.InstanceID,
		"session_id":            sess.ID,
		"connection_generation": strconv.FormatUint(gen, 10),
		"next_id":               strconv.FormatUint(sess.Ledger().HighWater()+1, 10),
		"grants":                grantList(sess.Grants),
		"schema_digest":         proto.SchemaDigest(),
		"limits":                limitsDocument(),
	}
	enc, merr := json.Marshal(out)
	if merr != nil {
		return nil, &proto.Error{Code: proto.ErrInternalPanic, Detail: "hello encoding failed"}
	}
	return enc, nil
}

// dispatch forwards one authenticated, well-typed request to the runtime
// port. Admission — ledger claim, grants, availability, execution — happens
// on the runtime thread at a cut; socket receipt is not acceptance.
func (c *conn) dispatch(req proto.Request) {
	sess := c.session
	id, _ := proto.ValidateRequestID(req.ID)
	cmd := drive.Command{
		Connection: c.id,
		Session:    sess.ID,
		Generation: sess.Generation(),
		ID:         id,
		OpID:       sess.InstanceID + "/" + sess.ID + "/" + req.ID,
		Method:     req.Method,
		Params:     req.Params,
		Grants:     sess.Grants,
		Reply:      make(chan drive.Result, 1),
	}
	if err := c.srv.cfg.Port.Submit(cmd); err != nil {
		// Bounded-inbox overload is transient and explicitly non-admitted.
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrCapacityBusy,
			Detail:      "runtime inbox at capacity",
			Retryable:   true,
			NonAdmitted: true,
		}, false)
		return
	}
	// The reader goroutine waits for the runtime's exactly-once reply. A
	// connection close aborts the wait without writing a socket byte.
	select {
	case res := <-cmd.Reply:
		var perr *proto.Error
		if res.Err != nil {
			errors.As(res.Err, &perr)
			if perr == nil {
				perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: res.Err.Error()}
			}
		}
		if perr != nil {
			c.sendError(req.ID, *perr, false)
			return
		}
		c.sendResult(req.ID, res.Payload, res.Cut, res.Frame)
	case <-c.done:
	}
}

func (c *conn) sendResult(id string, payload json.RawMessage, cut, frame string) {
	res := proto.Response{V: proto.ProtocolMajor, ID: id, OK: true, Result: payload, Cut: cut, Frame: frame}
	c.send(res)
	c.srv.stats.AddResponseSent()
}

func (c *conn) sendError(id string, perr proto.Error, nonAdmitted bool) {
	res := proto.Response{V: proto.ProtocolMajor, ID: id, OK: false, Error: &perr, NonAdmitted: nonAdmitted || perr.NonAdmitted}
	c.send(res)
}

// send encodes one response line onto the outbox. Marshal of strictly
// validated payloads cannot fail; if it ever does, the connection closes.
func (c *conn) send(res proto.Response) {
	enc, err := json.Marshal(res)
	if err != nil {
		c.close(drive.ReasonWriteFault)
		return
	}
	c.sendLine(append(enc, '\n'))
}

// grantList renders the grant set as the canonical sorted-ish list for
// hello results.
func grantList(g drive.Grants) []string {
	var out []string
	for _, e := range []struct {
		on bool
		n  string
	}{
		{g.Read, drive.GrantRead}, {g.Input, drive.GrantInput},
		{g.Command, drive.GrantCommand}, {g.Time, drive.GrantTime},
		{g.Admin, drive.GrantAdmin},
	} {
		if e.on {
			out = append(out, e.n)
		}
	}
	return out
}

// limitsDocument is the finite resource-limit inventory of this build. S1
// declares the bounds its path enforces; later slices extend it.
func limitsDocument() map[string]any {
	return map[string]any{
		"sessions":                 drive.MaxSessions(),
		"connections":              maxConnections,
		"ledger_entries":           drive.LedgerMaxEntries(),
		"ledger_bytes":             drive.LedgerMaxBytes(),
		"line_bytes":               proto.MaxLineBytes,
		"incomplete_line_seconds":  int(lineDeadline.Seconds()),
		"write_seconds":            int(writeDeadline.Seconds()),
		"session_recovery_minutes": int(drive.SessionRecoveryTTL().Minutes()),
	}
}
