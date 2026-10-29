package drive_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
)

// startAdminV2Server binds the harness runtime with an admin-capable grant
// set (session administration tests).
func startAdminV2Server(t *testing.T, h *testkit.DriveHarness, socket, token string) {
	t.Helper()
	h.Runtime().FreezeReadExports()
	srv, err := server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true, Input: true, Command: true, Time: true, Admin: true},
		Sessions:   h.Sessions(),
		Streams:    h.DriveStreams(),
		Port:       h.Runtime().DrivePort(),
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
}

// TestV2SocketSessionOwnStateStream is the S6-adjacent acceptance path: a
// driver connects to the live studio runtime, reads its own session and
// ownership state, opens a snapshot subscription, advances the app, and
// receives cut events with acks over the real socket.
func TestV2SocketSessionOwnStateStream(t *testing.T) {
	socket, token, h := v2SocketServer(t)
	startAdminV2Server(t, h, socket, token)

	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatal(err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
	next := 0
	send := func(method string, params any) {
		next++
		raw, merr := json.Marshal(params)
		if merr != nil {
			t.Fatalf("marshal: %v", merr)
		}
		req, merr := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: strconv.Itoa(next), Method: method, Params: raw})
		if merr != nil {
			t.Fatalf("marshal request: %v", merr)
		}
		if _, werr := rw.Write(append(req, '\n')); werr != nil {
			t.Fatalf("write: %v", werr)
		}
		if ferr := rw.Flush(); ferr != nil {
			t.Fatalf("flush: %v", ferr)
		}
	}
	// readLine blocks on one wire line; call pumps the app concurrently so
	// the runtime admits and replies at a cut.
	readLine := func() (string, error) {
		type result struct {
			line string
			err  error
		}
		ch := make(chan result, 1)
		go func() {
			line, err := rw.ReadString('\n')
			ch <- result{line, err}
		}()
		for i := 0; i < 200; i++ {
			select {
			case r := <-ch:
				return r.line, r.err
			default:
			}
			h.RunFrame()
			time.Sleep(time.Millisecond)
		}
		return "", fmt.Errorf("timeout waiting for a wire line")
	}
	call := func(method string, params any) proto.Response {
		send(method, params)
		for {
			line, err := readLine()
			if err != nil {
				t.Fatalf("read: %v", err)
			}
			// Notifications may interleave with the reply (stream delivery
			// continues while the pump runs); skip them here.
			if _, nerr := proto.DecodeClientNotification([]byte(strings.TrimRight(line, "\n"))); nerr == nil {
				continue
			}
			if _, rerr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n"))); rerr != nil {
				// Not a response: a server notification line.
				continue
			}
			res, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
			if derr != nil {
				t.Fatalf("decode response: %v (line: %s)", derr, line)
			}
			return res
		}
	}

	// hello: the response is the first line (no notification can precede it).
	send("sys.hello", map[string]any{"token": token, "client_name": "stream-test", "create_session": true})
	helloLine, hlErr := readLine()
	if hlErr != nil {
		t.Fatalf("read hello: %v", hlErr)
	}
	hello, dErr := proto.DecodeResponse([]byte(strings.TrimRight(helloLine, "\n")))
	if dErr != nil {
		t.Fatalf("decode hello: %v", dErr)
	}
	if !hello.OK {
		t.Fatalf("hello: %s", hello.Error)
	}
	var hr struct {
		SessionID  string `json:"session_id"`
		InstanceID string `json:"instance_id"`
	}
	_ = json.Unmarshal(hello.Result, &hr)
	// sys.hello is answered before admission: it consumes no ledger ID, so
	// the first post-hello request is id 1.
	next = 0

	// session.state: own generation and next id, no credential.
	res := call("session.state", map[string]any{})
	if !res.OK {
		t.Fatalf("session.state: %s", res.Error)
	}
	var state struct {
		SessionID string `json:"session_id"`
		NextID    string `json:"next_id"`
		ActiveOps []any  `json:"active_operations"`
	}
	_ = json.Unmarshal(res.Result, &state)
	if state.SessionID != hr.SessionID || state.NextID != "2" {
		t.Fatalf("session.state = %+v", state)
	}

	// own.state: the human owns input; wall mode.
	res = call("own.state", map[string]any{})
	if !res.OK {
		t.Fatalf("own.state: %s", res.Error)
	}
	var own struct {
		Owner string `json:"owner"`
		Mode  string `json:"mode"`
	}
	_ = json.Unmarshal(res.Result, &own)
	if own.Owner != "human" || own.Mode != "wall" {
		t.Fatalf("own.state = %+v", own)
	}

	// sub.watch with a snapshot: the stream binds to this connection.
	res = call("sub.watch", map[string]any{"snapshot": true, "credit": 4})
	if !res.OK {
		t.Fatalf("sub.watch: %s", res.Error)
	}
	var watch struct {
		StreamID string `json:"stream_id"`
		Snapshot struct {
			Cut    string            `json:"cut"`
			Stores map[string]string `json:"stores"`
		} `json:"snapshot"`
	}
	_ = json.Unmarshal(res.Result, &watch)
	if watch.StreamID == "" {
		t.Fatalf("sub.watch result missing stream_id: %s", string(res.Result))
	}

	// Drive the app: advance engine time so frames seal; the subscription
	// receives frame.completed / frame.ready records. A virtual-mode advance
	// needs the time lease; the direct Wall frames the harness pumps already
	// seal cuts, so run a frame and wait for delivery.
	h.RunFrame()
	h.RunFrame()
	deadline := time.Now().Add(10 * time.Second)
	delivered := 0
	for delivered == 0 && time.Now().Before(deadline) {
		line, rerr := readLine()
		if rerr != nil {
			t.Fatalf("read notification: %v", rerr)
		}
		if strings.HasPrefix(line, `{"v":2,"id":`) {
			continue // a reply, not a notification
		}
		var n proto.Notification
		if json.Unmarshal([]byte(line), &n) != nil {
			continue
		}
		switch n.Method {
		case "sub.data":
			var data struct {
				StreamID string `json:"stream_id"`
				Cursor   string `json:"cursor"`
			}
			_ = json.Unmarshal(n.Params, &data)
			if data.StreamID != watch.StreamID {
				t.Fatalf("sub.data for foreign stream %s", data.StreamID)
			}
			// Ack the cursor; credit must keep flowing. The ack is a client
			// notification: it consumes no request number.
			ack, aerr := json.Marshal(map[string]any{"v": proto.ProtocolMajor, "method": "sub.ack",
				"params": map[string]string{"stream_id": data.StreamID, "position": data.Cursor}})
			if aerr != nil {
				t.Fatalf("marshal ack: %v", aerr)
			}
			if _, werr := rw.Write(append(ack, '\n')); werr != nil {
				t.Fatalf("ack write: %v", werr)
			}
			if ferr := rw.Flush(); ferr != nil {
				t.Fatalf("ack flush: %v", ferr)
			}
			delivered++
		case "sys.shutdown":
			t.Fatal("unexpected server shutdown")
		}
	}
	if delivered == 0 {
		t.Fatal("no stream records delivered after frames")
	}

	// sub.state reflects live delivery with the retention window.
	res = call("sub.state", map[string]any{"stream_id": watch.StreamID})
	if !res.OK {
		t.Fatalf("sub.state: %s", res.Error)
	}
	var sub struct {
		State string `json:"state"`
		Sent  string `json:"sent"`
	}
	_ = json.Unmarshal(res.Result, &sub)
	if sub.State != "live" || sub.Sent == "0" {
		t.Fatalf("sub.state = %+v", sub)
	}

	// sub.cancel is terminal and idempotent.
	res = call("sub.cancel", map[string]any{"stream_id": watch.StreamID})
	if !res.OK {
		t.Fatalf("sub.cancel: %s", res.Error)
	}
	res = call("sub.cancel", map[string]any{"stream_id": watch.StreamID})
	if !res.OK {
		t.Fatalf("sub.cancel again: %s", res.Error)
	}
	var cancelled struct {
		State string `json:"state"`
	}
	_ = json.Unmarshal(res.Result, &cancelled)
	if cancelled.State != "cancelled" {
		t.Fatalf("repeated cancel = %+v", cancelled)
	}
}

// TestV2SocketAdminAndCancel exercises session.list/revoke (admin) and
// op.cancel over the real transport: a pending command operation is
// cancelled before its activation frame, its terminal outcome is
// cancelled, and revocation fences the target session.
func TestV2SocketAdminAndCancel(t *testing.T) {
	commands := runtimepkg.NewCommandRegistry()
	commands.Register(runtimepkg.CommandEntry{ID: "index.toggle", Version: 1, Title: "Toggle index", Execute: func() {}})
	socket, token, h := v2SocketServer(t, commands)
	startAdminV2Server(t, h, socket, token)

	// dial returns the connection's writer plus ONE persistent line reader:
	// exactly one goroutine ever reads the socket; every phase pulls lines
	// from the shared channel.
	dial := func() (*bufio.ReadWriter, *int, <-chan string) {
		nc, err := net.Dial("unix", socket)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = nc.Close() })
		_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
		rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
		next := new(int)
		lines := make(chan string, 64)
		go func() {
			for {
				line, err := rw.ReadString('\n')
				if err != nil {
					close(lines)
					return
				}
				lines <- line
			}
		}()
		return rw, next, lines
	}
	send := func(rw *bufio.ReadWriter, next *int, method string, params any) {
		*next++
		raw, merr := json.Marshal(params)
		if merr != nil {
			t.Fatalf("marshal: %v", merr)
		}
		req, merr := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: strconv.Itoa(*next), Method: method, Params: raw})
		if merr != nil {
			t.Fatalf("marshal request: %v", merr)
		}
		if _, werr := rw.Write(append(req, '\n')); werr != nil {
			t.Fatalf("write: %v", werr)
		}
		if ferr := rw.Flush(); ferr != nil {
			t.Fatalf("flush: %v", ferr)
		}
	}
	// recvLine pulls the next line with the app pump running concurrently.
	recvLine := func(lines <-chan string, h *testkit.DriveHarness) string {
		for i := 0; i < 400; i++ {
			select {
			case line := <-lines:
				return line
			default:
			}
			h.RunFrame()
			time.Sleep(time.Millisecond)
		}
		t.Fatal("no reply")
		return ""
	}
	recv := func(lines <-chan string, h *testkit.DriveHarness) proto.Response {
		line := recvLine(lines, h)
		res, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
		if derr != nil {
			t.Fatalf("decode response: %v (line: %.140s)", derr, line)
		}
		return res
	}
	call := func(rw *bufio.ReadWriter, lines <-chan string, next *int, method string, params any) proto.Response {
		send(rw, next, method, params)
		return recv(lines, h)
	}

	// The admin session.
	adminRW, adminNext, adminLines := dial()
	send(adminRW, adminNext, "sys.hello", map[string]any{"token": token, "client_name": "admin", "create_session": true})
	adminHello := recv(adminLines, h)
	if !adminHello.OK {
		t.Fatalf("admin hello: %s", adminHello.Error)
	}
	var adminID struct {
		SessionID  string `json:"session_id"`
		InstanceID string `json:"instance_id"`
	}
	_ = json.Unmarshal(adminHello.Result, &adminID)
	*adminNext = 0

	// A second (non-admin-authority) session whose operation gets cancelled.
	userRW, userNext, userLines := dial()
	send(userRW, userNext, "sys.hello", map[string]any{"token": token, "client_name": "user", "create_session": true})
	userHello := recv(userLines, h)
	if !userHello.OK {
		t.Fatalf("user hello: %s", userHello.Error)
	}
	var userID struct {
		SessionID string `json:"session_id"`
	}
	_ = json.Unmarshal(userHello.Result, &userID)
	*userNext = 0

	// session.list (admin): both sessions visible, redacted.
	res := call(adminRW, adminLines, adminNext, "session.list", map[string]any{})
	if !res.OK {
		t.Fatalf("session.list: %s", res.Error)
	}
	var list struct {
		Sessions []struct {
			SessionID string `json:"session_id"`
			State     string `json:"state"`
			Grants    []any  `json:"grants"`
		} `json:"sessions"`
	}
	_ = json.Unmarshal(res.Result, &list)
	// The harness's trusted in-process session also appears; locate the two
	// socket sessions by identity.
	socketSessions := 0
	for _, s := range list.Sessions {
		if s.SessionID == adminID.SessionID || s.SessionID == userID.SessionID {
			socketSessions++
		}
	}
	if socketSessions != 2 {
		t.Fatalf("session.list = %+v, want both socket sessions", list)
	}

	// The user executes a command and cancels it BEFORE any frame runs:
	// both requests admit at the same cut (FIFO), the command parks as a
	// pending activation, and the cancel removes it before its activation
	// frame. Nothing pumps between the two sends. The operation ID is the
	// deterministic instance/session/seq triple of the execute request.
	send(userRW, userNext, "cmd.execute", map[string]any{"name": "index.toggle", "version": 1})
	operationID := adminID.InstanceID + "/" + userID.SessionID + "/1"
	send(userRW, userNext, "op.cancel", map[string]any{"operation_id": operationID})
	receipt := recv(userLines, h)
	if !receipt.OK {
		t.Fatalf("cmd.execute receipt: %s", receipt.Error)
	}
	var accepted struct {
		OperationID string `json:"operation_id"`
		State       string `json:"state"`
	}
	_ = json.Unmarshal(receipt.Result, &accepted)
	if accepted.State != "accepted" || accepted.OperationID != operationID {
		t.Fatalf("receipt = %+v, want accepted %s", accepted, operationID)
	}
	// The cancel completes the operation inline, so its terminal
	// notification may precede the cancel's own response on the wire; read
	// until both are seen.
	var cancelRes proto.Response
	terminalCancelled := false
	for i := 0; i < 400 && (!terminalCancelled || cancelRes.ID == ""); i++ {
		line := recvLine(userLines, h)
		if strings.Contains(line, `"method":"op.completed"`) {
			if strings.Contains(line, `"state":"cancelled"`) {
				terminalCancelled = true
			} else {
				t.Fatalf("cancelled operation reported a non-cancelled outcome: %s", line)
			}
			continue
		}
		if parsed, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n"))); derr == nil {
			res = parsed
			cancelRes = parsed
		}
	}
	if !terminalCancelled {
		t.Fatalf("cancelled operation never reported its terminal outcome (cancelRes=%q resOk=%v resID=%q)", cancelRes.ID, res.OK, res.ID)
	}
	if cancelRes.ID == "" {
		t.Fatal("op.cancel never replied")
	}
	if !res.OK {
		t.Fatalf("op.cancel: %s", res.Error)
	}
	// session.state reports no active operations after the cancellation.
	res = call(userRW, userLines, userNext, "session.state", map[string]any{})
	if !res.OK {
		t.Fatalf("session.state: %s", res.Error)
	}
	var afterCancel struct {
		ActiveOps []any `json:"active_operations"`
	}
	_ = json.Unmarshal(res.Result, &afterCancel)
	if len(afterCancel.ActiveOps) != 0 {
		t.Fatalf("cancelled operation still active: %+v", afterCancel)
	}

	// Admin revokes the user session; the user's next request fails fenced
	// (the session is terminally gone), and its stream objects were closed.
	res = call(adminRW, adminLines, adminNext, "session.revoke", map[string]any{"session_id": userID.SessionID, "reason": "operator"})
	if !res.OK {
		t.Fatalf("session.revoke: %s", res.Error)
	}
	send(userRW, userNext, "own.state", map[string]any{})
	for i := 0; i < 400; i++ {
		line := recvLine(userLines, h)
		fenced, derr := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
		if derr != nil {
			continue // a trailing notification, not the reply
		}
		if fenced.OK {
			t.Fatal("a revoked session's request must not succeed")
		}
		if fenced.Error == nil || fenced.Error.Code != proto.ErrSessionExpired {
			t.Fatalf("fenced reply = %s, want session.expired", fenced.Error)
		}
		return
	}
	t.Fatal("no fenced reply after revocation")
}
