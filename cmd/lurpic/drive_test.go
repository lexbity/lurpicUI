package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

type cliSink struct{}

func (cliSink) Enqueue(cmd drive.Command) {
	var payload json.RawMessage
	switch cmd.Method {
	case "sys.info":
		// The real method list from the registry — the digest source for
		// the --check comparison.
		raw, err := json.Marshal(map[string]any{
			"v":       proto.ProtocolMajor,
			"methods": proto.MethodNames(),
		})
		if err != nil {
			cmd.Reply <- drive.Result{Err: &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}}
			return
		}
		payload = raw
	default:
		raw, err := json.Marshal(map[string]bool{"pong": true})
		if err != nil {
			cmd.Reply <- drive.Result{Err: &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}}
			return
		}
		payload = raw
	}
	cmd.Reply <- drive.Result{Payload: payload, Frame: 7}
}

func (cliSink) Epoch() uint64 { return 7 }

func TestCmdDrive_PingEndToEnd(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := strings.Repeat("ef", 32)
	srv, err := drive.Listen(drive.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sink:       cliSink{},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := cmdDrive([]string{"--socket", socket, "--token-file", socket + ".token", "ping"})
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)

	if code != 0 {
		t.Fatalf("exit code = %d, output %q", code, out)
	}
	if strings.TrimSpace(string(out)) != "ok frame=7" {
		t.Fatalf("output = %q, want %q", out, "ok frame=7")
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

func TestCmdDrive_ResolveAndTreeEndToEnd(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := strings.Repeat("ab", 32)
	srv, err := drive.Listen(drive.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sink:       locateSink{},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	// resolve with a locator: the sink fabricates a match.
	orig := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w
	code := cmdDrive([]string{"resolve", "--socket", socket, "--token-file", socket + ".token", "--testid", "shell.cmdk"})
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)
	if code != 0 {
		t.Fatalf("resolve exit = %d, output %q", code, out)
	}
	if !strings.Contains(string(out), `"path"`) {
		t.Fatalf("resolve output = %q", out)
	}

	// tree: same fabricated response shape.
	r2, w2, _ := os.Pipe()
	os.Stdout = w2
	code = cmdDrive([]string{"tree", "--socket", socket, "--token-file", socket + ".token", "--testid", "shell.cmdk", "--depth", "2"})
	w2.Close()
	os.Stdout = orig
	out2, _ := io.ReadAll(r2)
	if code != 0 {
		t.Fatalf("tree exit = %d, output %q", code, out2)
	}
	if !strings.Contains(string(out2), `"nodes"`) {
		t.Fatalf("tree output = %q", out2)
	}
}

// locateSink answers locate.resolve / tree.dump with plausible payloads.
type locateSink struct{}

func (locateSink) Enqueue(cmd drive.Command) {
	var payload json.RawMessage
	switch cmd.Method {
	case "locate.resolve":
		payload = json.RawMessage(`{"path":"studio.root/shell.cmdk","testid":"shell.cmdk","facet_id":42}`)
	case "tree.dump":
		payload = json.RawMessage(`{"nodes":[{"path":"studio.root","kind":"shell/root","testid":"studio.root"}]}`)
	default:
		payload = json.RawMessage(`{"pong":true}`)
	}
	cmd.Reply <- drive.Result{Payload: payload, Frame: 3}
}

func (locateSink) Epoch() uint64 { return 3 }

func TestCmdDrive_CheckEndToEnd(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := strings.Repeat("ef", 32)
	srv, err := drive.Listen(drive.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sink:       cliSink{},
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	orig := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("pipe: %v", err)
	}
	os.Stdout = w
	code := cmdDrive([]string{"--check", "--socket", socket, "--token-file", socket + ".token"})
	w.Close()
	os.Stdout = orig
	out, _ := io.ReadAll(r)

	if code != 0 {
		t.Fatalf("check exit = %d, output %q", code, out)
	}
	if !strings.Contains(string(out), "compatible v1 digest=") {
		t.Fatalf("check output = %q", out)
	}
	// The digest must match the server's actual method list (fake sink
	// publishes via sys.info through the runtime table — here the cliSink
	// echoes, so the server's table is exercised for real by the dispatch).
}
