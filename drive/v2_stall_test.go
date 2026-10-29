package drive_test

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/server"
)

// readWithPump reads one line while pumping the app so the runtime admits
// the pending request at a cut.
func readWithPump(rw *bufio.ReadWriter, h interface{ RunFrame() }) (string, error) {
	type result struct {
		line string
		err  error
	}
	ch := make(chan result, 1)
	go func() {
		line, err := rw.ReadString('\n')
		ch <- result{line, err}
	}()
	for i := 0; i < 400; i++ {
		select {
		case r := <-ch:
			return r.line, r.err
		default:
		}
		h.RunFrame()
		time.Sleep(time.Millisecond)
	}
	return "", errnoTimeout
}

var errnoTimeout = errTimeout{}

type errTimeout struct{}

func (errTimeout) Error() string { return "timeout waiting for a reply" }

// TestStalledSocketDoesNotBlockRuntime pins the D7 delivery contract over a
// real socket: a subscriber that stops reading must never slow the runtime.
// The stalled connection's bounded outbox fills, the transport closes it,
// its stream terminates explicitly, and the app keeps sealing cuts while
// the healthy peer keeps receiving records.
func TestStalledSocketDoesNotBlockRuntime(t *testing.T) {
	socket, token, h := v2SocketServer(t)
	h.Runtime().FreezeReadExports()
	srv, err := server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sessions:   h.Sessions(),
		Streams:    h.DriveStreams(),
		Port:       h.Runtime().DrivePort(),
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	// watch authenticates, opens a snapshot subscription, and returns the
	// connection, its reader, the stream ID, and the owning session ID.
	type watcher struct {
		nc      net.Conn
		rw      *bufio.ReadWriter
		stream  string
		session string
	}
	watch := func() watcher {
		nc, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = nc.Close() })
		_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
		rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
		hello, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: "1", Method: "sys.hello",
			Params: mustRaw(map[string]any{"token": token, "client_name": "stall-test", "create_session": true})})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rw.Write(append(hello, '\n')); err != nil {
			t.Fatal(err)
		}
		if err := rw.Flush(); err != nil {
			t.Fatal(err)
		}
		line, err := readWithPump(rw, h)
		if err != nil {
			t.Fatalf("hello: %v", err)
		}
		res, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
		if derr != nil || !res.OK {
			t.Fatalf("hello failed: %v %s", derr, line)
		}
		var hr struct {
			SessionID string `json:"session_id"`
		}
		_ = json.Unmarshal(res.Result, &hr)
		req, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: "1", Method: "sub.watch",
			Params: mustRaw(map[string]any{"snapshot": true, "credit": 8})})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := rw.Write(append(req, '\n')); err != nil {
			t.Fatal(err)
		}
		if err := rw.Flush(); err != nil {
			t.Fatal(err)
		}
		line, err = readWithPump(rw, h)
		if err != nil {
			t.Fatalf("watch: %v", err)
		}
		res, derr = proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
		if derr != nil || !res.OK {
			t.Fatalf("watch failed: %v %s", derr, line)
		}
		var parsed struct {
			StreamID string `json:"stream_id"`
		}
		_ = json.Unmarshal(res.Result, &parsed)
		return watcher{nc: nc, rw: rw, stream: parsed.StreamID, session: hr.SessionID}
	}
	stalled := watch()
	healthy := watch()

	// Drive frames until the healthy peer receives a record; it acks each
	// cursor. The stalled peer reads nothing.
	deadline := time.Now().Add(10 * time.Second)
	healthyRecords := 0
	for healthyRecords == 0 && time.Now().Before(deadline) {
		h.RunFrame()
		if healthy.rw.Reader.Buffered() == 0 {
			continue
		}
		line, err := healthy.rw.ReadString('\n')
		if err != nil {
			t.Fatalf("healthy read: %v", err)
		}
		if !strings.Contains(line, `"method":"sub.data"`) {
			continue
		}
		var data struct {
			StreamID string `json:"stream_id"`
			Cursor   string `json:"cursor"`
		}
		_ = json.Unmarshal([]byte(line), &data)
		ack, err := json.Marshal(map[string]any{"v": proto.ProtocolMajor, "method": "sub.ack",
			"params": map[string]string{"stream_id": data.StreamID, "position": data.Cursor}})
		if err != nil {
			t.Fatalf("marshal ack: %v", err)
		}
		if _, err := healthy.rw.Write(append(ack, '\n')); err != nil {
			t.Fatalf("ack: %v", err)
		}
		if err := healthy.rw.Flush(); err != nil {
			t.Fatalf("ack flush: %v", err)
		}
		healthyRecords++
	}
	if healthyRecords == 0 {
		t.Fatal("healthy subscriber received nothing")
	}

	// Pump hard while the stalled peer never reads: the runtime's cut must
	// keep advancing (no frame-loop block). Churn the log until retention
	// passes the stalled stream's position, so its expiry is deterministic.
	before := h.Runtime().DrivePort().Observe().Cut
	sent, _ := h.DriveStreams().Bounds()
	if stream, ok := h.DriveStreams().Get(stalled.stream); ok {
		sent, _ = stream.Position()
	}
	deadline = time.Now().Add(20 * time.Second)
	for time.Now().Before(deadline) {
		h.RunFrame()
		floor, _ := h.DriveStreams().Bounds()
		if floor > sent+1 {
			break
		}
	}
	after := h.Runtime().DrivePort().Observe().Cut
	if after == before {
		t.Fatalf("runtime stalled behind a dead peer: cut fixed at %s", before)
	}
	// The expired stream's pending sub.expired and the closed transport:
	// the stalled reader never drains its socket, so the bounded outbox
	// fills and the transport closes the connection.
	stalledClosed := false
	deadline = time.Now().Add(10 * time.Second)
	for !stalledClosed && time.Now().Before(deadline) {
		if got, ok := h.DriveStreams().Status(stalled.stream, stalled.session); ok && got.State != drive.StreamLive {
			stalledClosed = true
			break
		}
		h.RunFrame()
		time.Sleep(2 * time.Millisecond)
	}
	if !stalledClosed {
		t.Fatal("stalled connection was never closed and its stream never terminated")
	}
	got, ok := h.DriveStreams().Status(stalled.stream, stalled.session)
	if !ok || got.State == drive.StreamLive {
		t.Fatalf("stalled stream = %+v ok=%v, want a terminal state", got, ok)
	}

	// The healthy peer still receives records after the stall.
	more := 0
	deadline = time.Now().Add(5 * time.Second)
	for more == 0 && time.Now().Before(deadline) {
		h.RunFrame()
		if healthy.rw.Reader.Buffered() == 0 {
			continue
		}
		line, err := healthy.rw.ReadString('\n')
		if err != nil {
			t.Fatalf("healthy read: %v", err)
		}
		if strings.Contains(line, `"method":"sub.data"`) {
			more++
		}
	}
	if more == 0 {
		t.Fatal("healthy subscriber stopped receiving after the stall")
	}
	_ = srv
}

// mustRaw marshals v or fails the test.
func mustRaw(v any) json.RawMessage {
	raw, err := json.Marshal(v)
	if err != nil {
		panic(err)
	}
	return raw
}
