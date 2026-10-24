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
	payload, err := json.Marshal(map[string]bool{"pong": true})
	if err != nil {
		cmd.Reply <- drive.Result{Err: &proto.Error{Code: proto.ErrInternal, Detail: err.Error()}}
		return
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
