package main

import (
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

// startCLIServer binds a real v2 server with a synchronous fake runtime
// port answering pings; the CLI runs against it over a real Unix socket.
func startCLIServer(t *testing.T) (socket, token string) {
	t.Helper()
	dir, err := os.MkdirTemp("", "ldp") // 0700, short (unix path bound)
	if err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(dir) })
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
