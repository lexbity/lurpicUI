package main

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/drive/server"
)

func TestCLICommandNotificationMayOvertakeReceipt(t *testing.T) {
	clientConn, serverConn := net.Pipe()
	defer clientConn.Close()
	defer serverConn.Close()
	c := &driveJSON{rw: bufio.NewReadWriter(bufio.NewReader(clientConn), bufio.NewWriter(clientConn)), conn: clientConn}
	serverDone := make(chan error, 1)
	go func() {
		defer close(serverDone)
		reader := bufio.NewReader(serverConn)
		if _, err := reader.ReadString('\n'); err != nil {
			serverDone <- err
			return
		}
		writer := bufio.NewWriter(serverConn)
		notification, err := json.Marshal(proto.Notification{
			V: proto.ProtocolMajor, Method: "op.completed",
			Params: json.RawMessage(`{"operation_id":"instance/session/1","outcome":{"state":"completed","result":{"render_status":"ready"}}}`),
		})
		if err != nil {
			serverDone <- err
			return
		}
		if _, err := writer.Write(append(notification, '\n')); err != nil {
			serverDone <- err
			return
		}
		if err := writer.Flush(); err != nil {
			serverDone <- err
			return
		}
		receipt, err := json.Marshal(proto.Response{
			V: proto.ProtocolMajor, ID: "1", OK: true,
			Result: json.RawMessage(`{"operation_id":"instance/session/1","state":"accepted"}`),
		})
		if err != nil {
			serverDone <- err
			return
		}
		_, err = writer.Write(append(receipt, '\n'))
		if err == nil {
			err = writer.Flush()
		}
		serverDone <- err
	}()

	result, err := c.Call("cmd.execute", map[string]any{"name": "fixture.activate", "version": 1})
	if err != nil {
		t.Fatalf("Call: %v", err)
	}
	var outcome struct {
		State string `json:"state"`
	}
	if err := json.Unmarshal(result, &outcome); err != nil || outcome.State != "completed" {
		t.Fatalf("outcome=%s err=%v", result, err)
	}
	if err := <-serverDone; err != nil {
		t.Fatal(err)
	}
}

func TestCLIRecoversLostCommandReceiptWithSameID(t *testing.T) {
	firstClient, firstServer := net.Pipe()
	secondClient, secondServer := net.Pipe()
	c := &driveJSON{rw: bufio.NewReadWriter(bufio.NewReader(firstClient), bufio.NewWriter(firstClient)), conn: firstClient}
	c.reconnect = func() (*driveJSON, error) {
		return &driveJSON{rw: bufio.NewReadWriter(bufio.NewReader(secondClient), bufio.NewWriter(secondClient)), conn: secondClient, next: 1}, nil
	}
	firstDone := make(chan struct{})
	go func() {
		defer close(firstDone)
		_, _ = bufio.NewReader(firstServer).ReadString('\n')
		_ = firstServer.Close()
	}()
	secondDone := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(secondServer).ReadString('\n')
		if err != nil {
			secondDone <- err
			return
		}
		var request proto.Request
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			secondDone <- err
			return
		}
		if request.ID != "1" || request.Method != "cmd.execute" {
			secondDone <- fmt.Errorf("retried request id=%q method=%q", request.ID, request.Method)
			return
		}
		writer := bufio.NewWriter(secondServer)
		_, err = fmt.Fprintln(writer, `{"v":2,"id":"1","ok":true,"result":{"operation_id":"I/S/1","state":"completed","render_status":"ready"}}`)
		if err == nil {
			err = writer.Flush()
		}
		secondDone <- err
	}()
	result, err := c.Call("cmd.execute", map[string]any{"name": "fixture.activate", "version": 1})
	if err != nil {
		t.Fatalf("Call after lost receipt: %v", err)
	}
	if !strings.Contains(string(result), `"state":"completed"`) {
		t.Fatalf("recovered result = %s", result)
	}
	if err := <-secondDone; err != nil {
		t.Fatal(err)
	}
	<-firstDone
	_ = firstClient.Close()
	_ = secondClient.Close()
	_ = secondServer.Close()
}

func TestCLIAwaitRecoversThroughOperationStatus(t *testing.T) {
	firstClient, firstServer := net.Pipe()
	secondClient, secondServer := net.Pipe()
	c := &driveJSON{rw: bufio.NewReadWriter(bufio.NewReader(firstClient), bufio.NewWriter(firstClient)), conn: firstClient}
	c.reconnect = func() (*driveJSON, error) {
		return &driveJSON{rw: bufio.NewReadWriter(bufio.NewReader(secondClient), bufio.NewWriter(secondClient)), conn: secondClient, next: 1}, nil
	}
	go func() {
		_ = firstServer.Close()
	}()
	statusDone := make(chan error, 1)
	go func() {
		line, err := bufio.NewReader(secondServer).ReadString('\n')
		if err != nil {
			statusDone <- err
			return
		}
		var request proto.Request
		if err := json.Unmarshal([]byte(line), &request); err != nil {
			statusDone <- err
			return
		}
		if request.ID != "2" || request.Method != "op.status" {
			statusDone <- fmt.Errorf("recovery request id=%q method=%q", request.ID, request.Method)
			return
		}
		writer := bufio.NewWriter(secondServer)
		_, err = fmt.Fprintln(writer, `{"v":2,"id":"2","ok":true,"result":{"state":"completed","result":{"operation_id":"I/S/1","render_status":"ready"}}}`)
		if err == nil {
			err = writer.Flush()
		}
		statusDone <- err
	}()
	result, err := c.Await("I/S/1")
	if err != nil {
		t.Fatalf("Await after disconnect: %v", err)
	}
	if !strings.Contains(string(result), `"render_status":"ready"`) {
		t.Fatalf("recovered outcome = %s", result)
	}
	if err := <-statusDone; err != nil {
		t.Fatal(err)
	}
	_ = firstClient.Close()
	_ = secondClient.Close()
	_ = secondServer.Close()
}

// startCLIServer binds a real v2 server with a synchronous fake runtime
// port answering pings; the CLI runs against it over a real Unix socket.
func startCLIServer(t *testing.T) (socket, token string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod temp dir: %v", err)
	}
	socket = filepath.Join(dir, "d.sock")
	token = strings.Repeat("ef", 32)
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("instance: %v", err)
	}
	srv, err := server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sessions:   drive.NewSessionTable(inst),
		Streams:    drive.NewStreamHub(),
		Port:       &cliFakePort{},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	// The token file the CLI reads.
	if err := os.WriteFile(socket+".token", []byte(token), 0o600); err != nil {
		t.Fatalf("token file: %v", err)
	}
	t.Cleanup(func() { _ = os.Remove(socket + ".token") })
	return socket, token
}

// cliFakePort answers sys.ping with the cut/frame payload.
type cliFakePort struct{ n int }

func (p *cliFakePort) Submit(cmd drive.Command) error {
	if cmd.Method != "sys.ping" {
		cmd.Reply <- drive.Result{Err: &proto.Error{Code: proto.ErrUnknownMethod, Detail: cmd.Method}}
		return nil
	}
	p.n++
	payload, merr := json.Marshal(map[string]string{"pong": "true", "cut": "12", "frame": "9"})
	if merr != nil {
		payload = json.RawMessage(`{"pong":"true","cut":"12","frame":"9"}`)
	}
	cmd.Reply <- drive.Result{Payload: payload, Cut: "12", Frame: "9"}
	return nil
}

func (p *cliFakePort) SignalControl(drive.ControlSignal) {}
func (p *cliFakePort) Observe() drive.PublishedSnapshot  { return drive.PublishedSnapshot{} }

// runCmdDrive captures stdout and stderr around one CLI invocation.
func runCmdDrive(t *testing.T, args ...string) (int, string) {
	t.Helper()
	origOut, origErr := os.Stdout, os.Stderr
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	re, we, err2 := os.Pipe()
	if err2 != nil {
		t.Fatalf("pipe: %v", err2)
	}
	os.Stdout, os.Stderr = w, we
	code := cmdDrive(args)
	_ = w.Close()
	_ = we.Close()
	os.Stdout, os.Stderr = origOut, origErr
	out, _ := io.ReadAll(r)
	errOut, _ := io.ReadAll(re)
	return code, string(out) + string(errOut)
}

func TestCmdDrive_PingEndToEnd(t *testing.T) {
	socket, _ := startCLIServer(t)
	tokenFile := socket + ".token"

	code, out := runCmdDrive(t, "--socket", socket, "--token-file", tokenFile, "ping")
	if code != 0 {
		t.Fatalf("exit code = %d, output %q", code, out)
	}
	if out != "ok cut=12 frame=9\n" {
		t.Fatalf("output = %q, want the pong cut/frame", out)
	}
}

func TestCmdDrive_RequiresFlagsAndVerb(t *testing.T) {
	if code := cmdDrive([]string{"ping"}); code == 0 {
		t.Fatal("missing flags must fail")
	}
	if code := cmdDrive([]string{"--socket", "/x", "--token-file", "/y", "nope"}); code == 0 {
		t.Fatal("unknown verb must fail")
	}
}

func TestCmdDrive_UnadvertisedVerbFails(t *testing.T) {
	// The v1 watch/resolve/tree verbs are gone from the major-2 CLI; the
	// old spellings must fail rather than speak for an unadvertised method.
	if code := cmdDrive([]string{"watch", "--socket", "/x", "--token-file", "/y"}); code == 0 {
		t.Fatal("watch must fail")
	}
	if code := cmdDrive([]string{"resolve", "--socket", "/x", "--token-file", "/y"}); code == 0 {
		t.Fatal("resolve must fail")
	}
}

func TestCmdDrive_CheckEndToEnd(t *testing.T) {
	socket, _ := startCLIServer(t)
	tokenFile := socket + ".token"

	code, out := runCmdDrive(t, "--socket", socket, "--token-file", tokenFile, "--check")
	if code != 0 {
		t.Fatalf("check exit = %d, output %q", code, out)
	}
	if !strings.HasPrefix(out, "compatible v2 digest=sha256:") {
		t.Fatalf("check output = %q", out)
	}
}

func TestCLIResumes0600SessionFile(t *testing.T) {
	socket, _ := startCLIServer(t)
	tokenFile := socket + ".token"
	sessionFile := filepath.Join(t.TempDir(), "session.json")
	args := []string{"--socket", socket, "--token-file", tokenFile, "--session-file", sessionFile, "--check"}
	for i := 0; i < 2; i++ {
		code, out := runCmdDrive(t, args...)
		if code != 0 || !strings.HasPrefix(out, "compatible v2 digest=sha256:") {
			t.Fatalf("invocation %d exit=%d output=%q", i, code, out)
		}
	}
	info, err := os.Stat(sessionFile)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("session file mode=%#o, want 0600", info.Mode().Perm())
	}
	conn, err := driveConnect(socket, tokenFile, sessionFile)
	if err != nil {
		t.Fatalf("connect with saved session: %v", err)
	}
	defer conn.close()
	resumed, err := conn.reconnect()
	if err != nil {
		t.Fatalf("resume an already resumed session: %v", err)
	}
	defer resumed.close()
	if resumed.sessionID != conn.sessionID || resumed.generation == conn.generation {
		t.Fatalf("second resume session/generation = %s/%s, want %s/new generation", resumed.sessionID, resumed.generation, conn.sessionID)
	}
}

func TestCmdDrive_CheckDetectsSkew(t *testing.T) {
	socket, _ := startCLIServer(t)
	tokenFile := socket + ".token"

	// A server whose hello digest disagrees with the client's method list
	// must fail the check. Simulate by pointing the CLI at a raw listener
	// that returns a bogus digest.
	l, err := net.Listen("unix", socket+"-skew")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer l.Close()
	go func() {
		for {
			nc, err := l.Accept()
			if err != nil {
				return
			}
			go func() {
				buf := make([]byte, 4096)
				_, _ = nc.Read(buf)
				fmt.Fprintf(nc, `{"v":2,"id":"1","ok":true,"result":{"instance_id":"i","session_id":"s","connection_generation":"1","next_id":"1","grants":["read"],"schema_digest":"sha256:bogus","limits":{}}}`+"\n")
				_ = nc.Close()
			}()
		}
	}()

	code, out := runCmdDrive(t, "--socket", socket+"-skew", "--token-file", tokenFile, "--check")
	if code == 0 {
		t.Fatalf("skewed digest must fail the check, output %q", out)
	}
	if !strings.Contains(out, "INCOMPATIBLE") || !strings.Contains(out, "sha256:bogus") {
		t.Fatalf("check output = %q, want both digests", out)
	}
}
