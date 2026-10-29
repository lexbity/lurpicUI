package drive_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/studio"
	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/store"

	server "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"

	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
)

// v2SocketServer binds a real v2 server against a REAL studio runtime's
// drive port: this is the production seam — socket → transport → runtime
// port → admission at a cut.
func v2SocketServer(t *testing.T, registries ...*runtimepkg.CommandRegistry) (socket, token string, h *testkit.DriveHarness) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	reg, err := studio.StudioLayerRegistry()
	if err != nil {
		t.Fatalf("layer registry: %v", err)
	}
	themeCtx := studio.StudioThemeContext().WithFontRegistry(fonts)
	root := studio.BuildRoot(app.BuildContext{
		StoreScope:   scope,
		FontRegistry: fonts,
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        themeCtx,
	}, studio.NewDirtySink(10), studioSeedRows(t), reg)

	cfg := testkit.StandardHarnessConfig(t, 1280, 800)
	cfg.LayerRegistry = reg
	cfg.ThemeResolver = themeCtx.Resolver
	cfg.DriveStreams = drive.NewStreamHub()
	if len(registries) > 0 {
		cfg.CommandRegistry = registries[0]
	}
	h = testkit.NewDriveHarnessWithConfig(t, scope, cfg, root)
	h.RunFrame() // settle the initial dirty-all frame

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod temp dir: %v", err)
	}
	socket = filepath.Join(dir, "d.sock")
	token = strings.Repeat("cd", 32)
	return socket, token, h
}

// TestV2SocketRuntimePing is the S1 acceptance path: a driver authenticates
// to a live app over a real Unix socket, negotiates the major-2 session,
// and pings the real runtime at a cut. The pong's cut/frame attribution
// must reflect the app's actual rendered progress.
func TestV2SocketRuntimePing(t *testing.T) {
	socket, token, h := v2SocketServer(t)
	srv, err := startV2Server(t, h, socket, token)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))

	// send writes one request line; recv reads one reply line.
	send := func(id, method string, params any) {
		var raw []byte
		if params != nil {
			raw, err = json.Marshal(params)
			if err != nil {
				t.Fatalf("marshal: %v", err)
			}
		}
		req, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if err != nil {
			t.Fatalf("marshal request: %v", err)
		}
		if _, err := rw.Write(append(req, '\n')); err != nil {
			t.Fatalf("write: %v", err)
		}
		if err := rw.Flush(); err != nil {
			t.Fatalf("flush: %v", err)
		}
	}
	// call hello (answered without the runtime); run exchanges one
	// post-hello request: the reader waits concurrently while the test
	// advances the app so the runtime admits and executes at a cut.
	call := func(id, method string, params any) proto.Response {
		t.Logf("call %s %s", id, method)
		send(id, method, params)
		type result struct {
			res proto.Response
			err error
		}
		ch := make(chan result, 1)
		go func() {
			line, err := rw.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			res, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
			ch <- result{res: res, err: derr}
		}()
		// Pump the app until the runtime admits and replies at a cut; the
		// server goroutine may enqueue the command after this frame's drain.
		for i := 0; i < 64; i++ {
			select {
			case r := <-ch:
				if r.err != nil {
					t.Fatalf("read: %v", r.err)
				}
				return r.res
			default:
			}
			h.RunFrame()
			time.Sleep(time.Millisecond)
		}
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			return r.res
		case <-time.After(30 * time.Second):
			t.Fatalf("%s %s: no reply within 30s", method, id)
		}
		return proto.Response{}
	}

	// Handshake: create the session.
	hres := call("1", "sys.hello", map[string]any{
		"token": token, "client_name": "v2-socket-test", "create_session": true,
	})
	if !hres.OK {
		t.Fatalf("hello rejected: %s", hres.Error)
	}
	var hr struct {
		InstanceID         string   `json:"instance_id"`
		SessionID          string   `json:"session_id"`
		Credential         string   `json:"resume_credential"`
		Generation         string   `json:"connection_generation"`
		NextID             string   `json:"next_id"`
		SchemaDigest       string   `json:"schema_digest"`
		ExportSchemaDigest string   `json:"export_schema_digest"`
		Grants             []string `json:"grants"`
	}
	if err := json.Unmarshal(hres.Result, &hr); err != nil {
		t.Fatalf("hello payload: %v", err)
	}
	if hr.InstanceID == "" || hr.SessionID == "" || hr.Credential == "" || hr.Generation != "1" {
		t.Fatalf("hello = %+v", hr)
	}
	if hr.NextID != "1" {
		t.Fatalf("next_id = %s, want 1", hr.NextID)
	}
	if hr.SchemaDigest != proto.SchemaDigest() {
		t.Fatalf("schema digest = %s, want %s", hr.SchemaDigest, proto.SchemaDigest())
	}
	if !strings.HasPrefix(hr.ExportSchemaDigest, "sha256:") {
		t.Fatalf("hello export schema digest = %q", hr.ExportSchemaDigest)
	}

	// Advance the app so the pong identifies real rendered progress.
	h.RunFrame()
	frameBefore := h.Runtime().LastFrameStats().FrameNumber

	// Ping the real runtime at a cut.
	pres := call("1", "sys.ping", map[string]any{})
	if !pres.OK {
		t.Fatalf("ping rejected: %s", pres.Error)
	}
	var pong struct {
		Pong  bool   `json:"pong"`
		Cut   string `json:"cut"`
		Frame string `json:"frame"`
	}
	if err := json.Unmarshal(pres.Result, &pong); err != nil || !pong.Pong {
		t.Fatalf("payload = %s (%v)", pres.Result, err)
	}
	// The pong is cut-exact: it identifies the sealed cut at the boundary
	// where the runtime admitted it — never behind the app's progress at
	// the time the client observed the reply.
	frameAtReply := decimalFrame(pong.Frame)
	if frameAtReply < frameBefore {
		t.Fatalf("pong frame = %s, behind the app's progress %d", pong.Frame, frameBefore)
	}
	if pong.Cut != pong.Frame {
		t.Fatalf("cut = %s, frame = %s: a sealed cut follows its submitted frame at rest", pong.Cut, pong.Frame)
	}

	// Progress the app again and observe the attribution move.
	h.RunFrame()
	pres2 := call("2", "sys.ping", map[string]any{})
	var pong2 struct {
		Frame string `json:"frame"`
	}
	_ = json.Unmarshal(pres2.Result, &pong2)
	if decimalFrame(pong2.Frame) <= frameAtReply {
		t.Fatalf("second pong frame %s did not advance past %s", pong2.Frame, pong.Frame)
	}

	// Retry the same ping ID: the retained outcome replays, and the runtime
	// handled exactly two pings plus nothing hidden.
	replay := call("1", "sys.ping", map[string]any{})
	if !replay.OK || string(replay.Result) != string(pres.Result) {
		t.Fatalf("replayed ping = %+v, want the original outcome", replay)
	}
}

// runLostCommandResponseRecovery proves the S2 recovery seam over the real
// Unix socket and studio runtime: the first terminal response is deliberately
// left unread and its connection is closed, then the same session retries
// the same ID after resume. The registered GUI dispatcher runs once.
func runLostCommandResponseRecovery(t *testing.T) {
	var activations atomic.Int32
	commands := runtimepkg.NewCommandRegistry()
	commands.Register(runtimepkg.CommandEntry{
		ID: "fixture.activate", Version: 1, Title: "Activate fixture",
		Execute: func() { activations.Add(1) },
	})
	socket, token, h := v2SocketServer(t, commands)
	srv, err := startV2Server(t, h, socket, token)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	encode := func(id, method string, params any) []byte {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		line, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if err != nil {
			t.Fatal(err)
		}
		return append(line, '\n')
	}
	readResponse := func(nc net.Conn) proto.Response {
		t.Helper()
		_ = nc.SetReadDeadline(time.Now().Add(10 * time.Second))
		line, err := bufio.NewReader(nc).ReadString('\n')
		if err != nil {
			t.Fatalf("read response: %v", err)
		}
		res, err := proto.DecodeResponse([]byte(strings.TrimSuffix(line, "\n")))
		if err != nil {
			t.Fatalf("decode response: %v", err)
		}
		return res
	}
	first, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Write(encode("1", "sys.hello", map[string]any{
		"token": token, "client_name": "s2-recovery", "create_session": true,
	})); err != nil {
		t.Fatal(err)
	}
	hello := readResponse(first)
	if !hello.OK {
		t.Fatalf("hello: %s", hello.Error)
	}
	var session struct {
		InstanceID string `json:"instance_id"`
		SessionID  string `json:"session_id"`
		Credential string `json:"resume_credential"`
	}
	if err := json.Unmarshal(hello.Result, &session); err != nil {
		t.Fatal(err)
	}
	commandParams := map[string]any{"name": "fixture.activate", "version": 1}
	if _, err := first.Write(encode("1", "cmd.execute", commandParams)); err != nil {
		t.Fatal(err)
	}
	receiptCh := make(chan proto.Response, 1)
	go func() { receiptCh <- readResponse(first) }()
	// Drive the real frame loop until the accepted receipt and callback are
	// observed. The terminal notification is intentionally left unread: a
	// caller can lose it after the effect and recover by the same ID.
	var receipt proto.Response
	receivedReceipt := false
	for i := 0; i < 64 && (!receivedReceipt || activations.Load() == 0); i++ {
		select {
		case receipt = <-receiptCh:
			receivedReceipt = true
		default:
		}
		h.RunFrame()
		time.Sleep(time.Millisecond)
	}
	if !receivedReceipt || !receipt.OK {
		t.Fatalf("command acceptance receipt not received: %+v", receipt)
	}
	var accepted struct {
		OperationID string `json:"operation_id"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(receipt.Result, &accepted); err != nil || accepted.State != "accepted" {
		t.Fatalf("command receipt=%s err=%v", receipt.Result, err)
	}
	if activations.Load() != 1 {
		t.Fatalf("first command activations = %d, want 1", activations.Load())
	}
	_ = first.Close()
	for i := 0; i < 64; i++ {
		sess, ok := h.Sessions().Get(session.SessionID)
		if ok && sess.State() == drive.SessionDetached {
			break
		}
		time.Sleep(time.Millisecond)
	}

	second, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Close()
	if _, err := second.Write(encode("1", "sys.hello", map[string]any{
		"token": token, "client_name": "s2-recovery", "resume": map[string]any{
			"instance_id": session.InstanceID, "session_id": session.SessionID, "resume_credential": session.Credential,
		},
	})); err != nil {
		t.Fatal(err)
	}
	resumed := readResponse(second)
	if !resumed.OK {
		t.Fatalf("resume: %s", resumed.Error)
	}
	var state struct {
		NextID string `json:"next_id"`
	}
	if err := json.Unmarshal(resumed.Result, &state); err != nil || state.NextID != "2" {
		t.Fatalf("resume next_id=%q err=%v, want 2", state.NextID, err)
	}
	if _, err := second.Write(encode("1", "cmd.execute", commandParams)); err != nil {
		t.Fatal(err)
	}
	awaitResponse := func() proto.Response {
		t.Helper()
		responseCh := make(chan proto.Response, 1)
		go func() { responseCh <- readResponse(second) }()
		for i := 0; i < 64; i++ {
			select {
			case res := <-responseCh:
				return res
			default:
				h.RunFrame()
				time.Sleep(time.Millisecond)
			}
		}
		t.Fatal("resumed request did not receive a response")
		return proto.Response{}
	}
	replay := awaitResponse()
	if !replay.OK {
		t.Fatalf("retry rejected: %s", replay.Error)
	}
	if activations.Load() != 1 {
		t.Fatalf("retry dispatched command again: activations=%d", activations.Load())
	}
	opID := session.InstanceID + "/" + session.SessionID + "/1"
	if _, err := second.Write(encode("2", "op.status", map[string]string{"operation_id": opID})); err != nil {
		t.Fatal(err)
	}
	statusResponse := awaitResponse()
	if !statusResponse.OK {
		t.Fatalf("op.status rejected: %s", statusResponse.Error)
	}
	var status struct {
		OperationID string `json:"operation_id"`
		State       string `json:"state"`
	}
	if err := json.Unmarshal(statusResponse.Result, &status); err != nil {
		t.Fatal(err)
	}
	if status.OperationID != opID || status.State != "completed" {
		t.Fatalf("op.status = %+v", status)
	}
	if _, err := second.Write(encode("1", "cmd.execute", map[string]any{"name": "fixture.other", "version": 1})); err != nil {
		t.Fatal(err)
	}
	conflict := awaitResponse()
	if conflict.OK || conflict.Error == nil || conflict.Error.Code != proto.ErrRequestConflict {
		t.Fatalf("changed payload retry = %+v, want request.conflict", conflict)
	}
	if activations.Load() != 1 {
		t.Fatalf("conflicting retry changed activation count to %d", activations.Load())
	}
}

func TestSessionLostResponseRecovered(t *testing.T) {
	runLostCommandResponseRecovery(t)
}

func TestRequestPayloadConflict(t *testing.T) {
	runLostCommandResponseRecovery(t)
}

type observedDrivePort struct {
	drive.RuntimePort
	submitted chan struct{}
}

func (p *observedDrivePort) Submit(cmd drive.Command) error {
	if err := p.RuntimePort.Submit(cmd); err != nil {
		return err
	}
	select {
	case p.submitted <- struct{}{}:
	default:
	}
	return nil
}

func TestSocketDisconnectQueuedDoesNotActivate(t *testing.T) {
	var activations atomic.Int32
	commands := runtimepkg.NewCommandRegistry()
	commands.Register(runtimepkg.CommandEntry{ID: "fixture.activate", Version: 1, Title: "Activate fixture", Execute: func() { activations.Add(1) }})
	socket, token, h := v2SocketServer(t, commands)
	port := &observedDrivePort{RuntimePort: h.Runtime().DrivePort(), submitted: make(chan struct{}, 1)}
	srv, err := server.Listen(server.Config{
		SocketPath: socket, Token: token,
		Grants:   drive.Grants{Read: true, Input: true, Command: true, Time: true},
		Sessions: h.Sessions(), Streams: h.DriveStreams(), Port: port,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()
	encode := func(id, method string, params any) []byte {
		t.Helper()
		raw, marshalErr := json.Marshal(params)
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		line, marshalErr := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if marshalErr != nil {
			t.Fatal(marshalErr)
		}
		return append(line, '\n')
	}
	first, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := first.Write(encode("1", "sys.hello", map[string]any{"token": token, "client_name": "s2-disconnect", "create_session": true})); err != nil {
		t.Fatal(err)
	}
	line, err := bufio.NewReader(first).ReadString('\n')
	if err != nil {
		t.Fatal(err)
	}
	var hello proto.Response
	if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &hello); err != nil || !hello.OK {
		t.Fatalf("hello=%+v err=%v", hello, err)
	}
	var session struct {
		SessionID string `json:"session_id"`
	}
	if err := json.Unmarshal(hello.Result, &session); err != nil {
		t.Fatal(err)
	}
	if _, err := first.Write(encode("1", "cmd.execute", map[string]any{"name": "fixture.activate", "version": 1})); err != nil {
		t.Fatal(err)
	}
	select {
	case <-port.submitted:
	case <-time.After(5 * time.Second):
		t.Fatal("socket request did not reach the runtime inbox")
	}
	_ = first.Close()
	detached := false
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); {
		sess, ok := h.Sessions().Get(session.SessionID)
		if ok && sess.State() == drive.SessionDetached {
			detached = true
			break
		}
		time.Sleep(time.Millisecond)
	}
	if !detached {
		t.Fatal("connection loss did not detach the session")
	}
	h.RunFrame()
	sess, ok := h.Sessions().Get(session.SessionID)
	if !ok {
		t.Fatal("detached session disappeared before its recovery lifetime")
	}
	if highWater := sess.Ledger().HighWater(); highWater != 0 {
		t.Fatalf("disconnected pre-admission request consumed sequence: high-water=%d", highWater)
	}
	if activations.Load() != 0 {
		t.Fatalf("queued request activated after disconnect: %d", activations.Load())
	}
}

// TestV2RejectsLegacyMutation is the AC-01/S1 negative gate over the real
// wire: the v1 direct-write verb and the v1 major are both gone, and the
// `write` grant fails startup parsing.
func TestV2RejectsLegacyMutation(t *testing.T) {
	socket, token, h := v2SocketServer(t)
	srv, err := startV2Server(t, h, socket, token)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))

	call := func(id, method string, params any) proto.Response {
		var raw []byte
		if params != nil {
			enc, merr := json.Marshal(params)
			if merr != nil {
				t.Fatalf("marshal: %v", merr)
			}
			raw = enc
		}
		req, merr := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if merr != nil {
			t.Fatalf("marshal request: %v", merr)
		}
		_, _ = rw.Write(append(req, '\n'))
		_ = rw.Flush()
		type result struct {
			res proto.Response
			err error
		}
		ch := make(chan result, 1)
		go func() {
			line, err := rw.ReadString('\n')
			if err != nil {
				ch <- result{err: err}
				return
			}
			res, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
			ch <- result{res: res, err: derr}
		}()
		for i := 0; i < 64; i++ {
			select {
			case r := <-ch:
				if r.err != nil {
					t.Fatalf("read: %v", r.err)
				}
				return r.res
			default:
			}
			h.RunFrame()
			time.Sleep(time.Millisecond)
		}
		select {
		case r := <-ch:
			if r.err != nil {
				t.Fatalf("read: %v", r.err)
			}
			return r.res
		case <-time.After(30 * time.Second):
			t.Fatalf("%s %s: no reply within 30s", method, id)
		}
		return proto.Response{}
	}

	if res := call("1", "sys.hello", map[string]any{
		"token": token, "client_name": "legacy", "create_session": true,
	}); !res.OK {
		t.Fatalf("hello rejected: %s", res.Error)
	}

	// store.write: the direct mutation surface does not exist. The request
	// is well-typed, so it consumes its ID and answers a typed
	// protocol.unknown_method — and no store mutates anywhere.
	res := call("1", "store.write", map[string]any{"name": "e1.chartType", "value": 3})
	if res.OK || res.Error == nil || res.Error.Code != proto.ErrUnknownMethod {
		t.Fatalf("store.write = %+v, want protocol.unknown_method", res)
	}
	if res.NonAdmitted {
		t.Fatal("a well-typed unknown method is admitted then rejected, not non-admitted")
	}

	// The v1 `write` grant is not part of major 2: parsing it fails startup.
	if _, err := drive.ParseGrants("read,write"); err == nil {
		t.Fatal(`parseGrants("read,write") must fail startup`)
	}

	// A v1 client (major 1) is rejected by the handshake and disconnected.
	req, merr := json.Marshal(proto.Request{V: 1, ID: "1", Method: "sys.ping"})
	if merr != nil {
		t.Fatalf("marshal: %v", merr)
	}
	_, _ = rw.Write(append(req, '\n'))
	_ = rw.Flush()
	line, err := rw.ReadString('\n')
	if err != nil {
		t.Fatalf("read version mismatch reply: %v", err)
	}
	vres, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
	if derr != nil || vres.OK || vres.Error == nil || vres.Error.Code != proto.ErrVersionMismatch {
		t.Fatalf("v1 envelope = %+v (%v), want protocol.version_mismatch", vres, derr)
	}
	if _, err := rw.ReadString('\n'); err == nil {
		t.Fatal("a v1 client must be disconnected after the mismatch")
	}

	// The runtime is still rendering after the rejections.
	h.RunFrame()
	if h.Runtime().LastFrameStats().FrameNumber == 0 {
		t.Fatal("runtime stopped rendering after legacy rejections")
	}
}

// startV2Server binds a real v2 server to the harness runtime's port and
// session table (the production wiring the app layer performs).
func startV2Server(t *testing.T, h *testkit.DriveHarness, socket, token string) (*server.Server, error) {
	t.Helper()
	h.Runtime().FreezeReadExports()
	return server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true, Input: true, Command: true, Time: true},
		Sessions:   h.Sessions(),
		Streams:    h.DriveStreams(),
		Port:       h.Runtime().DrivePort(),
	})
}

// decimalFrame parses a decimal wire frame attribution.
func decimalFrame(s string) uint64 {
	v, _ := strconv.ParseUint(s, 10, 64)
	return v
}

// TestV2SocketExplicitReadSurface exercises the S5 observations over the
// authenticated Unix transport and the real studio runtime.
func TestV2SocketExplicitReadSurface(t *testing.T) {
	socket, token, h := v2SocketServer(t)
	srv, err := startV2Server(t, h, socket, token)
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()
	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	if err := nc.SetDeadline(time.Now().Add(30 * time.Second)); err != nil {
		t.Fatal(err)
	}
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
	call := func(id, method string, params any) proto.Response {
		t.Helper()
		raw, err := json.Marshal(params)
		if err != nil {
			t.Fatal(err)
		}
		request, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rw.Write(append(request, '\n')); err != nil {
			t.Fatal(err)
		}
		if err := rw.Flush(); err != nil {
			t.Fatal(err)
		}
		type received struct {
			response proto.Response
			err      error
		}
		result := make(chan received, 1)
		go func() {
			line, err := rw.ReadString('\n')
			if err != nil {
				result <- received{err: err}
				return
			}
			response, err := proto.DecodeResponse([]byte(strings.TrimSuffix(line, "\n")))
			result <- received{response: response, err: err}
		}()
		for i := 0; i < 128; i++ {
			select {
			case got := <-result:
				if got.err != nil {
					t.Fatal(got.err)
				}
				return got.response
			default:
			}
			h.RunFrame()
			time.Sleep(time.Millisecond)
		}
		select {
		case got := <-result:
			if got.err != nil {
				t.Fatal(got.err)
			}
			return got.response
		case <-time.After(30 * time.Second):
			t.Fatalf("%s did not reply", method)
		}
		return proto.Response{}
	}
	hello := call("1", "sys.hello", map[string]any{"token": token, "client_name": "s5-read-test", "create_session": true})
	if !hello.OK {
		t.Fatalf("hello: %s", hello.Error)
	}
	listed := call("1", "store.list", map[string]any{"limit": 256})
	if !listed.OK {
		t.Fatalf("store.list: %s", listed.Error)
	}
	var catalog struct {
		Exports []struct {
			Name string `json:"name"`
		} `json:"exports"`
	}
	if err := json.Unmarshal(listed.Result, &catalog); err != nil {
		t.Fatalf("store.list result %s: %v", listed.Result, err)
	}
	found := false
	for _, entry := range catalog.Exports {
		found = found || entry.Name == "e1.chartType"
	}
	if !found {
		t.Fatalf("explicit export missing from catalog: %s", listed.Result)
	}
	read := call("2", "store.read", map[string]any{"name": "e1.chartType"})
	if !read.OK {
		t.Fatalf("store.read: %s", read.Error)
	}
	var value struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal(read.Result, &value); err != nil || value.Value == "" {
		t.Fatalf("store.read result %s: %v", read.Result, err)
	}
	resolved := call("3", "locate.resolve", map[string]any{"locator": map[string]string{"testid": "shell.cmdk"}})
	if !resolved.OK {
		t.Fatalf("locate.resolve: %s", resolved.Error)
	}
	dumped := call("4", "tree.dump", map[string]any{"limit": 1})
	if !dumped.OK {
		t.Fatalf("tree.dump: %s", dumped.Error)
	}
	var tree struct {
		Snapshot string `json:"snapshot"`
		Nodes    []any  `json:"nodes"`
	}
	if err := json.Unmarshal(dumped.Result, &tree); err != nil || tree.Snapshot == "" || len(tree.Nodes) != 1 {
		t.Fatalf("tree.dump result %s: %v", dumped.Result, err)
	}
}
