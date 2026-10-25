package server

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// fakePort answers synchronously on the submitting (server connection)
// goroutine, standing in for the runtime-thread port.
type fakePort struct {
	mu       sync.Mutex
	enqueued []uint64
	panics   bool
	noReply  bool
	epoch    uint64
}

func (f *fakePort) Submit(cmd drive.Command) error {
	f.mu.Lock()
	f.enqueued = append(f.enqueued, cmd.ID)
	panics, noReply := f.panics, f.noReply
	f.epoch++
	frame := f.epoch
	f.mu.Unlock()
	if panics {
		panic("fakePort: injected panic")
	}
	if noReply {
		<-make(chan struct{}) // parked forever; Close must not wait on it
	}
	// A concrete struct: the marshal cannot fail, so this fake never
	// invents an error path the real runtime does not have.
	pong := struct {
		Pong  bool   `json:"pong"`
		Cut   string `json:"cut"`
		Frame string `json:"frame"`
	}{true, strconv.FormatUint(frame, 10), strconv.FormatUint(frame, 10)}
	payload, _ := json.Marshal(pong) //nolint:errchkjson // concrete struct, cannot fail
	cmd.Reply <- drive.Result{Payload: payload, Cut: pong.Cut, Frame: pong.Frame}
	return nil
}

func (f *fakePort) SignalControl(drive.ControlSignal) {}
func (f *fakePort) Observe() drive.PublishedSnapshot  { return drive.PublishedSnapshot{} }

func (f *fakePort) enqueuedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.enqueued)
}

func testToken() string { return strings.Repeat("ab", 32) }

// mustJSON marshals a test payload, failing the test on error.
func mustJSON(t *testing.T, v any) json.RawMessage {
	t.Helper()
	enc, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return enc
}

// startServer binds a v2 server on a temp socket with a live session table.
func startServer(t *testing.T, grants drive.Grants) (*Server, *fakePort, string, string) {
	t.Helper()
	return startServerWithPort(t, grants, &fakePort{})
}

func startServerWithPort(t *testing.T, grants drive.Grants, port drive.RuntimePort) (*Server, *fakePort, string, string) {
	t.Helper()
	socket := filepath.Join(testDriveDir(t), "d.sock")
	token := testToken()
	srv, err := Listen(Config{
		SocketPath: socket,
		Token:      token,
		Grants:     grants,
		Port:       port,
		Sessions:   newTestSessions(t),
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	var fp *fakePort
	if p, ok := port.(*fakePort); ok {
		fp = p
	}
	return srv, fp, socket, token
}

// testDriveDir is a private temporary directory for Unix socket tests.
func testDriveDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod temp dir: %v", err)
	}
	return dir
}

func newTestSessions(t *testing.T) *drive.SessionTable {
	t.Helper()
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("instance: %v", err)
	}
	return drive.NewSessionTable(inst)
}

// client is a minimal test driver speaking strict v2 envelopes.
type client struct {
	t    *testing.T
	nc   net.Conn
	rw   *bufio.ReadWriter
	next uint64
}

func dial(t *testing.T, socket string) *client {
	t.Helper()
	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	t.Cleanup(func() { _ = nc.Close() })
	return &client{t: t, nc: nc, rw: bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))}
}

func (c *client) send(line string) {
	c.t.Helper()
	if _, err := c.rw.WriteString(line + "\n"); err != nil {
		c.t.Fatalf("write: %v", err)
	}
	if err := c.rw.Flush(); err != nil {
		c.t.Fatalf("flush: %v", err)
	}
}

func (c *client) recv() proto.Response {
	c.t.Helper()
	line, err := c.rw.ReadString('\n')
	if err != nil {
		c.t.Fatalf("read: %v", err)
	}
	res, err := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
	if err != nil {
		c.t.Fatalf("decode %q: %v", line, err)
	}
	return res
}

// hello authenticates with a fresh session and returns the decoded result.
func (c *client) hello(token string) helloResult {
	c.t.Helper()
	c.next++
	params := mustJSON(c.t, map[string]any{
		"token": token, "client_name": "test", "create_session": true,
	})
	req := mustJSON(c.t, proto.Request{V: proto.ProtocolMajor, ID: strconv.FormatUint(c.next, 10), Method: "sys.hello", Params: params})
	c.send(string(req))
	res := c.recv()
	if !res.OK {
		c.t.Fatalf("hello rejected: %s", res.Error.Error())
	}
	var hr helloResult
	if err := json.Unmarshal(res.Result, &hr); err != nil {
		c.t.Fatalf("hello payload: %v", err)
	}
	return hr
}

type helloResult struct {
	InstanceID   string   `json:"instance_id"`
	SessionID    string   `json:"session_id"`
	Credential   string   `json:"resume_credential"`
	Generation   string   `json:"connection_generation"`
	NextID       string   `json:"next_id"`
	Grants       []string `json:"grants"`
	SchemaDigest string   `json:"schema_digest"`
}

// request sends one post-auth request with the client's next ID.
func (c *client) request(method string, params any) proto.Response {
	c.t.Helper()
	c.next++
	return c.requestID(strconv.FormatUint(c.next, 10), method, params)
}

func (c *client) requestID(id, method string, params any) proto.Response {
	c.t.Helper()
	var raw json.RawMessage
	if params != nil {
		enc, err := json.Marshal(params)
		if err != nil {
			c.t.Fatalf("marshal params: %v", err)
		}
		raw = enc
	}
	req, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	c.send(string(req))
	return c.recv()
}

func TestServer_HelloCreateSession(t *testing.T) {
	srv, _, socket, token := startServer(t, drive.Grants{Read: true, Input: true})
	c := dial(t, socket)
	hr := c.hello(token)
	if hr.InstanceID != srv.InstanceID() {
		t.Fatalf("instance = %s, want %s", hr.InstanceID, srv.InstanceID())
	}
	if hr.SessionID == "" || hr.Credential == "" {
		t.Fatal("hello must allocate a session id and resume credential")
	}
	if hr.Generation != "1" || hr.NextID != "1" {
		t.Fatalf("generation/next = %s/%s, want 1/1", hr.Generation, hr.NextID)
	}
	if strings.Join(hr.Grants, ",") != "read,input" {
		t.Fatalf("grants = %v, want the launcher policy", hr.Grants)
	}
	if hr.SchemaDigest != proto.SchemaDigest() {
		t.Fatalf("schema digest = %s, want %s", hr.SchemaDigest, proto.SchemaDigest())
	}
	if strings.Contains(strings.ToLower(hr.SessionID+hr.Credential), testToken()) {
		t.Fatal("session secrets must be freshly generated")
	}
}

func TestServer_PingAtCut(t *testing.T) {
	_, port, socket, token := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	_ = c.hello(token)
	res := c.request("sys.ping", map[string]any{})
	if !res.OK {
		t.Fatalf("ping failed: %s", res.Error.Error())
	}
	var pong struct {
		Pong  bool   `json:"pong"`
		Cut   string `json:"cut"`
		Frame string `json:"frame"`
	}
	if err := json.Unmarshal(res.Result, &pong); err != nil || !pong.Pong {
		t.Fatalf("payload = %s (%v)", res.Result, err)
	}
	if port.enqueuedCount() != 1 {
		t.Fatalf("port handled %d commands, want 1", port.enqueuedCount())
	}
	if res.Cut == "" || res.Frame == "" {
		t.Fatalf("reply attribution missing: cut=%q frame=%q", res.Cut, res.Frame)
	}
}

func TestServer_AuthDenied(t *testing.T) {
	_, _, socket, _ := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	c.next++
	params := mustJSON(c.t, map[string]any{"token": strings.Repeat("ff", 32), "create_session": true})
	req := mustJSON(c.t, proto.Request{V: proto.ProtocolMajor, ID: strconv.FormatUint(c.next, 10), Method: "sys.hello", Params: params})
	c.send(string(req))
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrAuth {
		t.Fatalf("res = %+v, want auth.denied", res)
	}
	// Auth failures close after one safe error; the close drains
	// asynchronously, so the second write may already hit a closed pipe.
	_, _ = c.rw.WriteString(`{"v":2,"id":"99","method":"sys.ping"}` + "\n")
	_ = c.rw.Flush()
	if _, _, err := c.rw.ReadRune(); err == nil {
		t.Fatal("connection must close after an auth failure")
	}
}

func TestServer_FirstRequestMustBeHello(t *testing.T) {
	_, _, socket, _ := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	res := c.requestID("1", "sys.ping", map[string]any{})
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrAuth {
		t.Fatalf("res = %+v, want auth.denied pre-hello", res)
	}
}

func TestServer_VersionMismatch(t *testing.T) {
	_, _, socket, _ := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	c.send(`{"v":1,"id":"1","method":"sys.ping"}`)
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrVersionMismatch {
		t.Fatalf("res = %+v, want protocol.version_mismatch", res)
	}
	if !res.NonAdmitted {
		t.Fatal("version mismatch must be non-admitted")
	}
	// The server closes the connection asynchronously after the mismatch
	// reply drains; the second write may already hit a closed pipe.
	_, _ = c.rw.WriteString(`{"v":1,"id":"2","method":"sys.ping"}` + "\n")
	_ = c.rw.Flush()
	if _, _, err := c.rw.ReadRune(); err == nil {
		t.Fatal("wrong major must close the connection")
	}
}

func TestServer_MalformedJSONTypedError(t *testing.T) {
	_, _, socket, token := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	_ = c.hello(token)
	c.send(`{"v":2,"id":"1","method":`)
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrMalformed || !res.NonAdmitted {
		t.Fatalf("res = %+v, want protocol.malformed (non-admitted)", res)
	}
	// Post-auth malformed requests survive: the connection continues.
	res2 := c.request("sys.ping", map[string]any{})
	if !res2.OK {
		t.Fatalf("connection did not survive a malformed line: %s", res2.Error)
	}
}

func TestServer_LineOverBoundTypedError(t *testing.T) {
	_, _, socket, _ := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	c.send(`{"v":2,"id":"1","method":"sys.ping","pad":"` + strings.Repeat("x", proto.MaxLineBytes) + `"}`)
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrLineLimit {
		t.Fatalf("res = %+v, want protocol.line_limit", res)
	}
}

// TestStrictEnvelopeDispatch is the S1 wire-grammar gate: every rejection
// mode of the strict decoder is typed, non-admitted, and never reaches the
// runtime port.
func TestStrictEnvelopeDispatch(t *testing.T) {
	_, port, socket, token := startServer(t, drive.Grants{Read: true})
	c := dial(t, socket)
	_ = c.hello(token)
	cases := []struct {
		name string
		line string
		code proto.ErrorCode
	}{
		{"duplicate envelope key", `{"v":2,"v":2,"id":"2","method":"sys.ping"}`, proto.ErrMalformed},
		{"unknown envelope field", `{"v":2,"id":"2","method":"sys.ping","zap":1}`, proto.ErrMalformed},
		{"zero id", `{"v":2,"id":"0","method":"sys.ping"}`, proto.ErrMalformed},
		{"leading-zero id", `{"v":2,"id":"02","method":"sys.ping"}`, proto.ErrMalformed},
		{"numeric id", `{"v":2,"id":2,"method":"sys.ping"}`, proto.ErrMalformed},
		{"params array", `{"v":2,"id":"2","method":"sys.ping","params":[]}`, proto.ErrMalformed},
		{"duplicate params key", `{"v":2,"id":"2","method":"sys.ping","params":{"a":1,"a":1}}`, proto.ErrMalformed},
		{"overflowing number", `{"v":2,"id":"2","method":"sys.ping","params":{"n":1e999}}`, proto.ErrMalformed},
		{"trailing data", `{"v":2,"id":"2","method":"sys.ping"}{"v":2}`, proto.ErrMalformed},
		{"array envelope", `["nope"]`, proto.ErrMalformed},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.send(tc.line)
			res := c.recv()
			if res.OK || res.Error == nil || res.Error.Code != tc.code || !res.NonAdmitted {
				t.Fatalf("res = %+v, want %s non-admitted", res, tc.code)
			}
		})
	}
	if port.enqueuedCount() != 0 {
		t.Fatalf("malformed lines reached the runtime port (%d)", port.enqueuedCount())
	}
	// The connection is still healthy for well-typed traffic.
	if res := c.request("sys.ping", map[string]any{}); !res.OK {
		t.Fatalf("post-malformed ping failed: %s", res.Error)
	}
}

func TestServer_SinkPanicContainedServerSurvives(t *testing.T) {
	_, port, socket, token := startServer(t, drive.Grants{Read: true})
	port.mu.Lock()
	port.panics = true
	port.mu.Unlock()
	c := dial(t, socket)
	_ = c.hello(token)
	// A panic inside Submit happens on the connection goroutine: the
	// connection drops (the runtime converts handler panics to typed
	// internal.panic results — covered by the runtime tests). The server
	// itself keeps serving new connections.
	c.send(`{"v":2,"id":"1","method":"sys.ping"}`)
	if _, err := c.rw.ReadString('\n'); err == nil {
		t.Fatal("panicked connection must close")
	}
	port.mu.Lock()
	port.panics = false
	port.mu.Unlock()
	c2 := dial(t, socket)
	_ = c2.hello(token)
	if res := c2.request("sys.ping", map[string]any{}); !res.OK {
		t.Fatalf("server did not survive a handler panic: %s", res.Error)
	}
}

func TestServer_CloseRemovesSocketAndToken(t *testing.T) {
	socket := filepath.Join(testDriveDir(t), "d.sock")
	srv, err := Listen(Config{
		SocketPath: socket,
		Token:      testToken(),
		Grants:     drive.Grants{Read: true},
		Port:       &fakePort{},
		Sessions:   newTestSessions(t),
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	if st, err := os.Stat(socket); err != nil || st.Mode().Perm() != 0o600 {
		t.Fatalf("socket stat = %v %v", st, err)
	}
	tk, err := os.Stat(socket + ".token")
	if err != nil || tk.Mode().Perm() != 0o600 {
		t.Fatalf("token stat = %v %v", tk, err)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Lstat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket not removed: %v", err)
	}
	if _, err := os.Lstat(socket + ".token"); !os.IsNotExist(err) {
		t.Fatalf("token not removed: %v", err)
	}
	// Close is idempotent.
	if err := srv.Close(); err != nil {
		t.Fatalf("second close: %v", err)
	}
}

// TestSocketOwnershipAndUnwind is the S1 trust-boundary gate: the bind path
// refuses symlinked tokens, pre-existing sockets, world-accessible
// directories, and 0644-token preconditions; it never unlinks an unrelated
// file; and a failed startup unwinds the files it created.
func TestSocketOwnershipAndUnwind(t *testing.T) {
	dir := testDriveDir(t)
	socket := filepath.Join(dir, "drive.sock")

	// A pre-existing regular file at the socket path is refused, never
	// unlinked.
	if err := os.WriteFile(socket, []byte("not a socket"), 0o644); err != nil {
		t.Fatalf("seed socket path: %v", err)
	}
	_, err := Listen(Config{SocketPath: socket, Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("listen = %v, want an exists refusal", err)
	}
	if st, serr := os.Lstat(socket); serr != nil || !st.Mode().IsRegular() {
		t.Fatalf("refused listen destroyed the pre-existing file: %v (%v)", st, serr)
	}
	if err := os.Remove(socket); err != nil {
		t.Fatal(err)
	}

	// A pre-existing token file (even 0644) is refused, never replaced.
	if err := os.WriteFile(socket+".token", []byte("stale"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = Listen(Config{SocketPath: socket, Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("listen = %v, want a token-collision refusal", err)
	}
	if data, rerr := os.ReadFile(socket + ".token"); rerr != nil || string(data) != "stale" {
		t.Fatalf("refused listen replaced the stale token: %q (%v)", data, rerr)
	}
	if err := os.Remove(socket + ".token"); err != nil {
		t.Fatal(err)
	}

	// A symlinked token path is refused without following it.
	if err := os.Symlink(filepath.Join(dir, "elsewhere"), socket+".token"); err != nil {
		t.Fatal(err)
	}
	_, err = Listen(Config{SocketPath: socket, Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil || !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("listen = %v, want a symlink refusal", err)
	}
	_ = os.Remove(socket + ".token")

	// A group/world-accessible socket directory is refused.
	private := filepath.Join(dir, "private")
	if err := os.MkdirAll(private, 0o755); err != nil {
		t.Fatal(err)
	}
	_, err = Listen(Config{SocketPath: filepath.Join(private, "drive.sock"), Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil || !strings.Contains(err.Error(), "0700") {
		t.Fatalf("listen = %v, want a private-directory refusal", err)
	}

	// A relative socket path is refused.
	_, err = Listen(Config{SocketPath: "relative.sock", Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil {
		t.Fatal("relative socket path must be refused")
	}

	// A wrong-shape token is refused before any filesystem side effect.
	_, err = Listen(Config{SocketPath: filepath.Join(dir, "ok.sock"), Token: "short", Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	if err == nil {
		t.Fatal("malformed token must be refused")
	}
	if _, serr := os.Lstat(filepath.Join(dir, "ok.sock")); serr == nil {
		t.Fatal("a refused listen must not create the socket")
	}

	// Startup failure after token creation unwinds the created token.
	l, lerr := net.Listen("unix", socket) // occupy the path so bind fails late
	if lerr != nil {
		t.Fatalf("occupy: %v", lerr)
	}
	_, err = Listen(Config{SocketPath: socket, Token: testToken(), Grants: drive.Grants{Read: true}, Port: &fakePort{}, Sessions: newTestSessions(t)})
	_ = l.Close()
	if err == nil {
		t.Fatal("listen must fail on an occupied socket path")
	}
	if _, serr := os.Lstat(socket + ".token"); !os.IsNotExist(serr) {
		t.Fatalf("failed startup left the token behind: %v", serr)
	}
}

// TestCloseWithoutRuntimeReply is the S1 shutdown gate: Close completes
// promptly while the runtime port never replies; it never waits for an
// individual runtime reply, and it closes the waiting client connection.
func TestCloseWithoutRuntimeReply(t *testing.T) {
	port := &fakePort{noReply: true}
	srv, _, socket, token := startServerWithPort(t, drive.Grants{Read: true}, port)
	c := dial(t, socket)
	_ = c.hello(token)
	done := make(chan struct{})
	go func() {
		_ = srv.Close()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Close blocked on a runtime reply for over 2s")
	}
	// The client's connection is closed: the pending read faults.
	_, rerr := c.rw.ReadString('\n')
	if rerr == nil {
		t.Fatal("pending client connection must be closed by Close")
	}
}

// TestListenerPanicSupervisor is the S1 containment gate: a panic in the
// listener goroutine signals the supervisor and closes the socket without
// taking the process down.
func TestListenerPanicSupervisor(t *testing.T) {
	socket := filepath.Join(testDriveDir(t), "d.sock")
	panicked := make(chan error, 1)
	srv, err := Listen(Config{
		SocketPath: socket,
		Token:      testToken(),
		Grants:     drive.Grants{Read: true},
		Port:       &fakePort{},
		Sessions:   newTestSessions(t),
		Supervisor: func(err error) { panicked <- err },
		acceptHook: func() { panic("boom") },
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}

	select {
	case err := <-panicked:
		if err == nil || !strings.Contains(err.Error(), "boom") {
			t.Fatalf("supervisor err = %v, want the panic value", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("supervisor was not signaled")
	}
	// The listener closed: the socket path is unlinked and the server no
	// longer accepts.
	if _, serr := os.Lstat(socket); !os.IsNotExist(serr) {
		t.Fatalf("panicked listener left the socket behind: %v", serr)
	}
	_ = srv.Close()
}
