package server

import (
	"bufio"
	"encoding/json"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// fakeSink answers synchronously on the calling (server connection)
// goroutine, standing in for the runtime-thread sink.
type fakeSink struct {
	mu       sync.Mutex
	enqueued []uint64
	panics   bool
	epoch    atomic.Uint64
}

func (f *fakeSink) Enqueue(cmd Command) {
	f.mu.Lock()
	f.enqueued = append(f.enqueued, cmd.ID)
	panics := f.panics
	f.mu.Unlock()
	if panics {
		panic("fakeSink: injected panic")
	}
	frame := f.epoch.Add(1)
	payload, err := json.Marshal(map[string]bool{"pong": true})
	if err != nil {
		cmd.Reply <- Result{Err: &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}}
		return
	}
	cmd.Reply <- Result{Payload: payload, Frame: frame}
}

func (f *fakeSink) Epoch() uint64 { return f.epoch.Load() }

func (f *fakeSink) enqueuedCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.enqueued)
}

func testToken() string { return strings.Repeat("ab", 32) }

func startTestServer(t *testing.T, grants Grants) (*Server, *fakeSink, string, string) {
	t.Helper()
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := testToken()
	sink := &fakeSink{}
	srv, err := Listen(Config{
		SocketPath: socket,
		Token:      token,
		Grants:     grants,
		Sink:       sink,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, sink, socket, token
}

// client is a minimal test driver.
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
	t.Cleanup(func() { nc.Close() })
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
	var res proto.Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &res); err != nil {
		c.t.Fatalf("decode %q: %v", line, err)
	}
	return res
}

func (c *client) sendRequest(id uint64, v int, method string, params any) {
	c.t.Helper()
	var req proto.Request
	if params == nil {
		req = proto.Request{V: v, ID: id, Method: method}
	} else {
		raw, err := json.Marshal(params)
		if err != nil {
			c.t.Fatalf("marshal params: %v", err)
		}
		req = proto.Request{V: v, ID: id, Method: method, Params: raw}
	}
	out, err := json.Marshal(req)
	if err != nil {
		c.t.Fatalf("marshal request: %v", err)
	}
	c.send(string(out))
}

func (c *client) call(method string, params any) proto.Response {
	c.t.Helper()
	c.next++
	c.sendRequest(c.next, 1, method, params)
	return c.recv()
}

func (c *client) hello(token string) proto.Response {
	c.t.Helper()
	c.sendRequest(0, 1, "sys.hello", helloParams{Token: token})
	return c.recv()
}

// expectClosed requires a true EOF, not a mere read timeout: the server
// must have closed the connection, not merely gone quiet.
func (c *client) expectClosed() {
	c.t.Helper()
	_ = c.nc.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, err := c.rw.ReadString('\n')
	if err == nil {
		c.t.Fatal("connection should be closed")
	}
	if !errors.Is(err, io.EOF) {
		c.t.Fatalf("read error = %v, want io.EOF (server-side close)", err)
	}
}

func TestServer_HelloPing(t *testing.T) {
	_, sink, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	res := c.hello(token)
	if !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	var payload struct {
		V int `json:"v"`
	}
	if err := json.Unmarshal(res.Result, &payload); err != nil || payload.V != proto.ProtocolMajor {
		t.Fatalf("hello result = %s (%v)", res.Result, err)
	}
	res = c.call("sys.ping", nil)
	if !res.OK {
		t.Fatalf("ping failed: %+v", res.Error)
	}
	if res.Frame != 1 {
		t.Fatalf("frame = %d, want 1", res.Frame)
	}
	if sink.enqueuedCount() != 1 {
		t.Fatalf("enqueued = %d, want 1", sink.enqueuedCount())
	}
}

func TestServer_AuthDenied(t *testing.T) {
	_, sink, socket, _ := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	res := c.hello("wrong-token")
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrAuth {
		t.Fatalf("response = %+v, want auth.denied", res)
	}
	if sink.enqueuedCount() != 0 {
		t.Fatal("command crossed the sink after auth denial")
	}
	c.expectClosed()
}

func TestServer_FirstRequestMustBeHello(t *testing.T) {
	_, sink, socket, _ := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	c.sendRequest(1, 1, "sys.ping", nil)
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrAuth {
		t.Fatalf("response = %+v, want auth.denied", res)
	}
	if sink.enqueuedCount() != 0 {
		t.Fatal("command crossed the sink before hello")
	}
}

func TestServer_VersionMismatch(t *testing.T) {
	_, _, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	c.sendRequest(0, 2, "sys.hello", helloParams{Token: token})
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrVersion {
		t.Fatalf("response = %+v, want version.mismatch", res)
	}
	if !strings.Contains(res.Error.Detail, "2") || !strings.Contains(res.Error.Detail, "1") {
		t.Fatalf("detail %q must name both majors", res.Error.Detail)
	}
	c.expectClosed()
}

func TestServer_GrantDenied(t *testing.T) {
	srv, sink, socket, token := startTestServer(t, Grants{})
	before := srv.stats.grantDenied.Load()
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	res := c.call("sys.ping", nil)
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrGrant {
		t.Fatalf("response = %+v, want grant.denied", res)
	}
	if got := srv.stats.grantDenied.Load() - before; got != 1 {
		t.Fatalf("grant-denied counter delta = %d, want 1", got)
	}
	if sink.enqueuedCount() != 0 {
		t.Fatal("denied command must not reach the sink")
	}
}

func TestGrantMatrix(t *testing.T) {
	srv, sink, socket, token := startTestServer(t, Grants{})
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	before := srv.stats.grantDenied.Load()
	checked := 0
	for _, spec := range proto.Registry {
		if spec.Grant == "" {
			continue
		}
		checked++
		res := c.call(spec.Name, nil)
		if res.OK || res.Error == nil || res.Error.Code != proto.ErrGrant {
			t.Errorf("%s without %s grant = %+v, want grant.denied", spec.Name, spec.Grant, res)
		}
	}
	if checked == 0 {
		t.Fatal("registry has no grant-protected methods")
	}
	if got := srv.stats.grantDenied.Load() - before; got != uint64(checked) {
		t.Errorf("grant-denied counter delta = %d, want %d", got, checked)
	}
	if got := sink.enqueuedCount(); got != 0 {
		t.Errorf("denied methods reached the runtime sink %d times", got)
	}
}

func TestServer_RetryCachedDoesNotRedispatch(t *testing.T) {
	_, sink, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	c.sendRequest(5, 1, "sys.ping", nil)
	first := c.recv()
	if !first.OK || first.ID != 5 || first.Cached {
		t.Fatalf("first response = %+v", first)
	}
	// Retry the same id: cached response, no second dispatch.
	c.sendRequest(5, 1, "sys.ping", nil)
	second := c.recv()
	if !second.OK || second.ID != 5 || !second.Cached {
		t.Fatalf("retry response = %+v, want cached hit", second)
	}
	if first.Frame != second.Frame {
		t.Fatalf("cached frame = %d, want %d", second.Frame, first.Frame)
	}
	if sink.enqueuedCount() != 1 {
		t.Fatalf("dispatched %d times, want 1 (retry must not re-execute)", sink.enqueuedCount())
	}
}

func TestServer_UnknownMethodTypedError(t *testing.T) {
	_, sink, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	res := c.call("no.such.method", nil)
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrInternal {
		t.Fatalf("response = %+v", res)
	}
	if !strings.Contains(res.Error.Detail, "no.such.method") {
		t.Fatalf("detail %q must name the method", res.Error.Detail)
	}
	if sink.enqueuedCount() != 0 {
		t.Fatal("unknown method must not reach the sink")
	}
}

func TestServer_MalformedJSONTypedError(t *testing.T) {
	_, _, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	c.send("{not json")
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrInternal {
		t.Fatalf("response = %+v, want typed error", res)
	}
	// The connection survives a malformed line.
	if res := c.call("sys.ping", nil); !res.OK {
		t.Fatalf("ping after malformed line failed: %+v", res.Error)
	}
}

func TestServer_LineOverBoundTypedError(t *testing.T) {
	_, _, socket, token := startTestServer(t, Grants{Read: true})
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	c.send(strings.Repeat("x", proto.MaxLineBytes+64))
	res := c.recv()
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrInternal {
		t.Fatalf("response = %+v, want typed error for oversized line", res)
	}
}

func TestServer_SinkPanicContainedServerSurvives(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "panic.sock")
	token := testToken()
	sink := &fakeSink{}
	sink.mu.Lock()
	sink.panics = true
	sink.mu.Unlock()
	srv, err := Listen(Config{SocketPath: socket, Token: token, Grants: Grants{Read: true}, Sink: sink})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	c.sendRequest(1, 1, "sys.ping", nil)
	// The connection goroutine recovers the panic, counts it, and drops the
	// connection: the client sees EOF, not a crashed server.
	c.expectClosed()
	if got := srv.stats.socketGoroutinePanics.Load(); got != 1 {
		t.Fatalf("panic counter = %d, want 1", got)
	}
	// The server survives: a new connection completes the handshake.
	c2 := dial(t, socket)
	if res := c2.hello(token); !res.OK {
		t.Fatalf("hello after panic failed: %+v", res.Error)
	}
}

func TestServer_CloseRemovesSocketAndToken(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := testToken()
	srv, err := Listen(Config{
		SocketPath: socket, Token: token,
		Grants: Grants{Read: true}, Sink: &fakeSink{},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello failed: %+v", res.Error)
	}
	if err := srv.Close(); err != nil {
		t.Fatalf("close: %v", err)
	}
	if _, err := os.Stat(socket); !os.IsNotExist(err) {
		t.Fatalf("socket still present after Close: %v", err)
	}
	if _, err := os.Stat(socket + ".token"); !os.IsNotExist(err) {
		t.Fatalf("token still present after Close: %v", err)
	}
	c.expectClosed()
}

func TestServer_PerConnectionOrdering(t *testing.T) {
	_, _, socket, token := startTestServer(t, Grants{Read: true})
	var wg sync.WaitGroup
	for connIdx := 0; connIdx < 2; connIdx++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			c := dial(t, socket)
			if res := c.hello(token); !res.OK {
				t.Errorf("hello failed: %+v", res.Error)
				return
			}
			var lastFrame uint64
			for i := 0; i < 20; i++ {
				res := c.call("sys.ping", nil)
				if !res.OK {
					t.Errorf("ping %d failed: %+v", i, res.Error)
					return
				}
				if res.Frame <= lastFrame {
					t.Errorf("frame went backwards: %d after %d", res.Frame, lastFrame)
					return
				}
				lastFrame = res.Frame
			}
		}()
	}
	wg.Wait()
}

func TestParseGrants(t *testing.T) {
	g, err := ParseGrants("read,input")
	if err != nil || !g.Read || !g.Input || g.Write || g.Time || g.Admin {
		t.Fatalf("ParseGrants(read,input) = %+v, %v", g, err)
	}
	if g.String() != "read,input" {
		t.Fatalf("String() = %q", g.String())
	}
	if g, err := ParseGrants(""); err != nil || g.Read {
		t.Fatalf("empty grants = %+v, %v", g, err)
	}
	if _, err := ParseGrants("read,root"); err == nil {
		t.Fatal("unknown grant must be an error")
	}
	all, err := ParseGrants("read,input,write,time,admin")
	if err != nil {
		t.Fatalf("all grants: %v", err)
	}
	if all.String() != "read,input,write,time,admin" {
		t.Fatalf("String() = %q", all.String())
	}
}

func TestListenValidation(t *testing.T) {
	sink := &fakeSink{}
	if _, err := Listen(Config{SocketPath: "", Token: "t", Sink: sink}); err == nil {
		t.Fatal("empty SocketPath must fail")
	}
	if _, err := Listen(Config{SocketPath: "/tmp/x.sock", Token: "", Sink: sink}); err == nil {
		t.Fatal("empty Token must fail")
	}
	if _, err := Listen(Config{SocketPath: "/tmp/x.sock", Token: "t"}); err == nil {
		t.Fatal("nil Sink must fail")
	}
	if _, err := Listen(Config{SocketPath: "rel.sock", Token: "t", Sink: sink}); err == nil {
		t.Fatal("relative SocketPath must fail")
	}
}
