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
	"strings"
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
	// idleLineDeadline bounds the wait for the next line between lines. It
	// is deliberately generous: a client legitimately waits for an async
	// reply (an operation notification, stream delivery) without owing the
	// server a line. Keepalive traffic (documented: every 2s) keeps lease
	// holders alive; the keepalive lease bound is separate (keepaliveIdle).
	idleLineDeadline = 30 * time.Second
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
	// Streams is the shared bounded event log (subscriptions). The app wires
	// the SAME instance into the runtime config; Listen binds its delivery
	// signal to this server's connections.
	Streams *drive.StreamHub
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
	connByID   map[drive.ConnectionID]*conn
	nextConnID atomic.Uint64
	stopCh     chan struct{}
	stopOnce   sync.Once
	acceptDone chan struct{}
	stopped    chan struct{}
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
	if cfg.Streams == nil {
		return nil, fmt.Errorf("drive: Streams hub is required")
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
		cfg:        cfg,
		stats:      cfg.Stats,
		now:        cfg.Now,
		ln:         bound.listener,
		conns:      make(map[*conn]struct{}),
		stopCh:     make(chan struct{}),
		acceptDone: make(chan struct{}),
		stopped:    make(chan struct{}),
	}
	if s.stats == nil {
		s.stats = &drive.Stats{}
	}
	s.connByID = make(map[drive.ConnectionID]*conn)
	s.socketDev, s.socketIno = bound.socketDev, bound.socketIno
	s.tokenDev, s.tokenIno = bound.tokenDev, bound.tokenIno
	// Stream delivery: the hub signals a ready stream; the signal is routed
	// to the owning connection's drain goroutine (never blocking the
	// runtime's append path).
	s.cfg.Streams.SetNotify(func(streamID string) {
		st, ok := s.cfg.Streams.Get(streamID)
		if !ok {
			return
		}
		s.connsMu.Lock()
		c := s.connByID[st.Connection]
		s.connsMu.Unlock()
		if c != nil {
			c.signalStreamDrain()
		}
	})
	go func() {
		<-s.acceptDone
		s.nextConn.Wait()
		close(s.stopped)
	}()
	go s.acceptLoop()
	go s.leaseLoop()
	return s, nil
}

// leaseLoop enforces the keepalive lease contract (RFC §Safe boundaries):
// a connection whose session owns input or time loses that lease after
// keepaliveIdle without any authenticated line; observers are never
// disturbed. Failure detection uses real (injected) wall time.
func (s *Server) leaseLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-s.stopCh:
			return
		case <-ticker.C:
		}
		snap := s.cfg.Port.Observe()
		now := s.now()
		s.connsMu.Lock()
		conns := make([]*conn, 0, len(s.conns))
		for c := range s.conns {
			conns = append(conns, c)
		}
		s.connsMu.Unlock()
		for _, c := range conns {
			sess, generation := c.attachment()
			if sess == nil {
				continue
			}
			if sess.ID != snap.TimeOwnerSession && sess.ID != snap.InputOwnerSession {
				continue
			}
			if now.UnixNano()-c.lastActivity.Load() > keepaliveIdle.Nanoseconds() {
				c.srv.cfg.Port.SignalControl(drive.ControlSignal{
					Kind:       drive.SignalLeaseExpired,
					Reason:     drive.ReasonIdle,
					Session:    sess.ID,
					Generation: generation,
				})
			}
		}
	}
}

// keepaliveIdle is the lease-expiry silence bound (RFC: ten seconds without
// an authenticated keepalive; clients send one every two seconds).
const keepaliveIdle = 10 * time.Second

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

// Stopped is closed after the listener has exited, every connection reader
// and writer has returned, and owned socket/token paths have been removed.
// A listener panic supervisor can await this signal instead of treating its
// notification as proof that transport cleanup has completed.
func (s *Server) Stopped() <-chan struct{} { return s.stopped }

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
	defer close(s.acceptDone)
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
			s.stopTransport(false)
			return
		}
		s.connsMu.Lock()
		if s.closed || len(s.conns) >= maxConnections {
			s.connsMu.Unlock()
			_ = nc.Close()
			continue
		}
		c := &conn{
			srv:          s,
			id:           drive.ConnectionID(s.nextConnID.Add(1)),
			nc:           nc,
			outbox:       make(chan []byte, outboxCapacity),
			done:         make(chan struct{}),
			pendingCalls: make(chan struct{}, 32),
			streamDrain:  make(chan struct{}, 1),
			streams:      make(map[string]struct{}),
		}
		c.lastActivity.Store(s.now().UnixNano())
		s.conns[c] = struct{}{}
		s.connByID[c.id] = c
		s.connsMu.Unlock()
		s.nextConn.Add(3)
		go c.writeLoop()
		go c.serve()
		go c.streamLoop()
	}
}

// Close stops the transport: it fences connection generations, cancels
// readers/writers, unlinks only its own inodes, and never waits for an
// individual runtime reply.
func (s *Server) Close() error {
	s.stopTransport(true)
	<-s.stopped
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
		c.sendNotification("sys.shutdown", []byte("{}"), "", "")
	}
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
	// pendingCalls bounds post-hello requests whose runtime outcome has not
	// yet been written. The reader itself never waits on runtime execution.
	pendingCalls chan struct{}
	// writeMu serializes close vs. writer-fault bookkeeping.
	closeOnce sync.Once
	attachMu  sync.RWMutex

	authed     bool
	session    *drive.Session
	generation uint64

	// streamDrain wakes the stream delivery loop; streams lists this
	// connection's bound subscriptions (bound after their sub.watch reply).
	streamDrain chan struct{}
	streamsMu   sync.Mutex
	streams     map[string]struct{}
	// lastActivity is the last authenticated line's receive instant
	// (keepalive lease enforcement).
	lastActivity atomic.Int64

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
		delete(c.srv.connByID, c.id)
		c.srv.connsMu.Unlock()
		if c.srv.cfg.Streams != nil {
			c.srv.cfg.Streams.CloseConnection(c.id)
		}
		sess, generation := c.attachment()
		if sess != nil {
			if sess.DetachGeneration(generation, c.srv.now()) {
				c.srv.cfg.Port.SignalControl(drive.ControlSignal{
					Kind:       drive.SignalConnectionLost,
					Reason:     reason,
					Session:    sess.ID,
					Generation: generation,
				})
			}
		}
	})
}

func (c *conn) attachment() (*drive.Session, uint64) {
	c.attachMu.RLock()
	defer c.attachMu.RUnlock()
	return c.session, c.generation
}

func (c *conn) attach(session *drive.Session, generation uint64) {
	c.attachMu.Lock()
	c.session = session
	c.generation = generation
	c.attachMu.Unlock()
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
		_ = c.nc.SetReadDeadline(c.srv.now().Add(idleLineDeadline))
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
	// Client notifications (no ID by contract) must route before request
	// decoding: a strict request decode would reject them as malformed.
	var peek struct {
		ID     string `json:"id"`
		Method string `json:"method"`
	}
	_ = json.Unmarshal(line, &peek)
	if peek.ID == "" && proto.IsClientNotification(peek.Method) {
		if !c.authed {
			c.sendError("", proto.Error{Code: proto.ErrAuth, Detail: "authenticate before notifications", NonAdmitted: true}, true)
			return true
		}
		notification, nerr := proto.DecodeClientNotification(line)
		if nerr != nil {
			c.sendError("", proto.Error{Code: proto.ErrMalformed, Detail: nerr.Error(), NonAdmitted: true}, false)
			return false
		}
		c.lastActivity.Store(c.srv.now().UnixNano())
		c.handleClientNotification(notification)
		return false
	}
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
	c.lastActivity.Store(c.srv.now().UnixNano())
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
	// Clear the hello deadline. Authenticated connections have no per-line
	// idle deadline; EOF/write failure is delivered as an out-of-band runtime
	// control signal and fences the attached generation.
	_ = c.nc.SetReadDeadline(time.Time{})
	return false
}

// create allocates a fresh session under the launcher's grant policy.
func (c *conn) create(hp helloParams) (json.RawMessage, *proto.Error) {
	sess, cred, err := c.srv.cfg.Sessions.Create(c.srv.cfg.Grants)
	if err != nil {
		return nil, &proto.Error{Code: proto.ErrCapacityBusy, Detail: err.Error(), Retryable: true, NonAdmitted: true}
	}
	c.attach(sess, sess.Generation())
	out := map[string]any{
		"instance_id":           sess.InstanceID,
		"session_id":            sess.ID,
		"resume_credential":     cred,
		"connection_generation": strconv.FormatUint(sess.Generation(), 10),
		"next_id":               strconv.FormatUint(sess.Ledger().HighWater()+1, 10),
		"grants":                grantList(sess.Grants),
		"schema_digest":         proto.SchemaDigest(),
		"export_schema_digest":  c.srv.cfg.Port.Observe().ExportSchemaDigest,
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
	c.attach(sess, gen)
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
		"export_schema_digest":  c.srv.cfg.Port.Observe().ExportSchemaDigest,
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
	select {
	case c.pendingCalls <- struct{}{}:
	default:
		c.sendError(req.ID, proto.Error{Code: proto.ErrCapacityBusy, Retryable: true, NonAdmitted: true}, false)
		return
	}
	sess, generation := c.attachment()
	if sess == nil {
		<-c.pendingCalls
		c.sendError(req.ID, proto.Error{Code: proto.ErrAuth, NonAdmitted: true}, false)
		return
	}
	id, _ := proto.ValidateRequestID(req.ID)
	cmd := drive.Command{
		Connection: c.id,
		Session:    sess.ID,
		Generation: generation,
		ID:         id,
		OpID:       sess.InstanceID + "/" + sess.ID + "/" + req.ID,
		Method:     req.Method,
		Params:     req.Params,
		Grants:     sess.Grants,
		Reply:      make(chan drive.Result, 1),
	}
	if req.Method == "cmd.execute" || req.Method == "time.mode" || req.Method == "time.advance" || req.Method == "frame.run" || strings.HasPrefix(req.Method, "input.") {
		cmd.Accepted = make(chan drive.Receipt, 1)
	}
	if err := c.srv.cfg.Port.Submit(cmd); err != nil {
		<-c.pendingCalls
		// Bounded-inbox overload is transient and explicitly non-admitted.
		c.sendError(req.ID, proto.Error{
			Code:        proto.ErrCapacityBusy,
			Detail:      "runtime inbox at capacity",
			Retryable:   true,
			NonAdmitted: true,
		}, false)
		return
	}
	go c.awaitCommand(req.ID, cmd)
}

func (c *conn) awaitCommand(requestID string, cmd drive.Command) {
	defer func() { <-c.pendingCalls }()
	var receipt *drive.Receipt
	if cmd.Accepted != nil {
		select {
		case r := <-cmd.Accepted:
			receipt = &r
		case res := <-cmd.Reply:
			c.sendCommandResult(requestID, res)
			return
		case <-c.done:
			return
		}
	}
	if receipt != nil {
		payload, err := json.Marshal(map[string]string{
			"operation_id":   receipt.OperationID,
			"state":          "accepted",
			"accepted_cut":   receipt.AcceptedCut,
			"accepted_frame": receipt.AcceptedFrame,
			"next_id":        strconv.FormatUint(receipt.NextID, 10),
		})
		if err != nil {
			c.close(drive.ReasonWriteFault)
			return
		}
		c.sendResult(requestID, payload, receipt.AcceptedCut, receipt.AcceptedFrame)
	}
	select {
	case res := <-cmd.Reply:
		if receipt != nil {
			c.sendOperationCompletion(cmd.OpID, res)
		} else {
			c.sendCommandResult(requestID, res)
		}
		if cmd.Method == "sub.watch" && res.Err == nil {
			var parsed struct {
				StreamID string `json:"stream_id"`
			}
			if json.Unmarshal(res.Payload, &parsed) == nil && parsed.StreamID != "" {
				c.bindStream(parsed.StreamID)
			}
		}
	case <-c.done:
	}
}

func (c *conn) sendCommandResult(id string, res drive.Result) {
	var perr *proto.Error
	if res.Err != nil {
		errors.As(res.Err, &perr)
		if perr == nil {
			perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: res.Err.Error()}
		}
	}
	if perr != nil {
		c.sendError(id, *perr, false)
		return
	}
	c.sendResult(id, res.Payload, res.Cut, res.Frame)
}

func (c *conn) sendOperationCompletion(operationID string, res drive.Result) {
	outcome := map[string]any{"operation_id": operationID, "state": "completed", "result": res.Payload, "frame": res.Frame, "cut": res.Cut}
	if res.Err != nil {
		var perr *proto.Error
		if !errors.As(res.Err, &perr) {
			perr = &proto.Error{Code: proto.ErrInternalPanic, Detail: "operation failed"}
		}
		state := "failed"
		if perr.Code == proto.ErrOperationCancelled {
			state = "cancelled"
		}
		outcome["state"] = state
		outcome["error"] = perr
	}
	params, err := json.Marshal(map[string]any{"operation_id": operationID, "outcome": outcome})
	if err != nil {
		c.close(drive.ReasonWriteFault)
		return
	}
	c.sendNotification("op.completed", params, res.Cut, res.Frame)
}

// handleClientNotification processes one authenticated client→server
// notification: sys.keepalive refreshes the lease clock (the line's
// lastActivity update is the refresh), sub.ack returns credit.
func (c *conn) handleClientNotification(n proto.ClientNotification) {
	if n.Method != "sub.ack" {
		return
	}
	var params struct {
		StreamID string `json:"stream_id"`
		Position string `json:"position"`
	}
	dec := json.NewDecoder(bytes.NewReader(n.Params))
	dec.DisallowUnknownFields()
	if len(n.Params) == 0 || dec.Decode(&params) != nil || params.StreamID == "" {
		c.sendError("", proto.Error{Code: proto.ErrMalformed, Detail: "sub.ack requires stream_id and position", NonAdmitted: true}, false)
		return
	}
	position, err := strconv.ParseUint(params.Position, 10, 64)
	if err != nil {
		c.sendError("", proto.Error{Code: proto.ErrMalformed, Detail: "sub.ack position must be a decimal cursor", NonAdmitted: true}, false)
		return
	}
	sess, _ := c.attachment()
	if sess == nil {
		return
	}
	state, _, sendable := c.srv.cfg.Streams.Ack(params.StreamID, sess.ID, c.id, position)
	if state == drive.StreamLive && sendable {
		c.signalStreamDrain()
	}
}

// bindStream registers a successfully created subscription for delivery on
// this connection (the owning session is re-verified by the hub).
func (c *conn) bindStream(streamID string) {
	if c.srv.cfg.Streams == nil {
		return
	}
	sess, _ := c.attachment()
	if sess == nil {
		return
	}
	state, reason := c.srv.cfg.Streams.BindDelivery(streamID, sess.ID, c.id)
	c.streamsMu.Lock()
	c.streams[streamID] = struct{}{}
	c.streamsMu.Unlock()
	if state == drive.StreamLive {
		c.signalStreamDrain()
		return
	}
	c.sendStreamTerminal(streamID, state, reason)
}

// signalStreamDrain wakes the delivery loop without blocking its caller.
func (c *conn) signalStreamDrain() {
	select {
	case c.streamDrain <- struct{}{}:
	default:
	}
}

// streamLoop delivers sub.data records while the connection lives: the hub
// owns retention, cursors, and credit; the loop only encodes and writes.
func (c *conn) streamLoop() {
	defer c.srv.nextConn.Done()
	for {
		select {
		case <-c.done:
			return
		case <-c.streamDrain:
			c.drainStreams()
		}
	}
}

// drainStreams drains every bound stream and writes one sub.data per
// record; an expired stream reports exactly one sub.expired notification
// (its terminal state stays queryable via sub.state).
func (c *conn) drainStreams() {
	if c.srv.cfg.Streams == nil {
		return
	}
	sess, _ := c.attachment()
	if sess == nil {
		return
	}
	c.streamsMu.Lock()
	ids := make([]string, 0, len(c.streams))
	for id := range c.streams {
		ids = append(ids, id)
	}
	c.streamsMu.Unlock()
	for _, id := range ids {
		recs, state, reason := c.srv.cfg.Streams.Drain(id, sess.ID, c.id)
		for _, rec := range recs {
			params, err := json.Marshal(map[string]any{
				"stream_id": id,
				"cursor":    strconv.FormatUint(rec.Cursor, 10),
				"cut":       rec.Cut,
				"frame":     rec.Frame,
				"events":    rec.Events,
			})
			if err != nil {
				c.close(drive.ReasonWriteFault)
				return
			}
			if !c.sendLine(mustLine(proto.Notification{V: proto.ProtocolMajor, Method: "sub.data", Params: params, Cut: rec.Cut, Frame: rec.Frame})) {
				return
			}
		}
		if state == drive.StreamExpired {
			c.sendStreamTerminal(id, state, reason)
			c.streamsMu.Lock()
			delete(c.streams, id)
			c.streamsMu.Unlock()
		}
	}
}

// sendStreamTerminal reports a stream's terminal state once.
func (c *conn) sendStreamTerminal(streamID string, state drive.StreamState, reason string) {
	params, err := json.Marshal(map[string]any{
		"stream_id": streamID,
		"state":     string(state),
		"reason":    reason,
	})
	if err != nil {
		c.close(drive.ReasonWriteFault)
		return
	}
	c.sendNotification("sub.expired", params, "", "")
}

// mustLine encodes one notification line; it never fails for protocol
// documents.
func mustLine(n proto.Notification) []byte {
	line, err := json.Marshal(n)
	if err != nil {
		return []byte(`{"v":2,"method":"sys.shutdown"}`)
	}
	return append(line, '\n')
}

func (c *conn) sendNotification(method string, params json.RawMessage, cut, frame string) {
	line, err := json.Marshal(proto.Notification{V: proto.ProtocolMajor, Method: method, Params: params, Cut: cut, Frame: frame})
	if err != nil {
		c.close(drive.ReasonWriteFault)
		return
	}
	c.sendLine(append(line, '\n'))
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
		"idle_line_seconds":        int(idleLineDeadline.Seconds()),
		"write_seconds":            int(writeDeadline.Seconds()),
		"session_recovery_minutes": int(drive.SessionRecoveryTTL().Minutes()),
		"stream_records":           drive.DefaultStreamRetention.MaxRecords,
		"stream_bytes":             drive.DefaultStreamRetention.MaxBytes,
		"stream_record_bytes":      drive.DefaultStreamRetention.MaxRecordBytes,
		"stream_retention_minutes": int(drive.DefaultStreamRetention.RetentionAge.Minutes()),
		"stream_credit_max":        8,
		"keepalive_idle_seconds":   int(keepaliveIdle.Seconds()),
	}
}
