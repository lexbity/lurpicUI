package drive_test

import (
	"bufio"
	"encoding/json"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	server "codeburg.org/lexbit/lurpicui/drive/server"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// minimalRoot is a tiny real facet tree: the live CLI gate does not need
// the studio, only a real runtime rendering real frames.
type minimalRoot struct {
	facet.Facet
	layout facet.LayoutRole
}

func (r *minimalRoot) Base() *facet.Facet           { return &r.Facet }
func (r *minimalRoot) OnAttach(facet.AttachContext) {}
func (r *minimalRoot) OnDetach()                    {}
func (r *minimalRoot) OnActivate()                  {}
func (r *minimalRoot) OnDeactivate()                {}

// TestLiveCLIPingPath boots a runtime, binds the real socket, and drives
// the full lurpic-drive-cli wire path (hello create → ping) through a raw
// socket client: the same bytes the CLI sends.
func TestLiveCLIPingPath(t *testing.T) {
	root := &minimalRoot{}
	root.Facet = facet.NewFacet()
	root.BindImpl(root)
	root.SetTestID("live-root")
	root.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 200, H: 120}}
	}
	root.layout.Child.SupportedPlacement = facet.SupportsFree
	root.AddRole(&root.layout)

	cfg := testkit.StandardHarnessConfig(t, 200, 120)
	h := testkit.NewDriveHarnessWithConfig(t, cfg, root)
	h.RunFrame()

	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatal(err)
	}
	socket := filepath.Join(dir, "d.sock")
	token := strings.Repeat("ab", 32)
	srv, err := server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true},
		Sessions:   h.Sessions(),
		Port:       h.Runtime().DrivePort(),
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	// The CLI's wire path: dial, hello create_session, ping, correlate by
	// id (never by "the next line").
	nc, err := net.Dial("unix", socket)
	if err != nil {
		t.Fatalf("dial: %v", err)
	}
	defer nc.Close()
	_ = nc.SetDeadline(time.Now().Add(30 * time.Second))
	rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
	send := func(id, method string, params any) {
		var raw []byte
		if params != nil {
			raw, err = json.Marshal(params)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, merr := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: id, Method: method, Params: raw})
		if merr != nil {
			t.Fatal(merr)
		}
		if _, werr := rw.Write(append(req, '\n')); werr != nil {
			t.Fatal(werr)
		}
		_ = rw.Flush()
	}
	send("1", "sys.hello", map[string]any{"token": token, "client_name": "lurpic-cli", "create_session": true})
	hres := recvLine(t, rw)
	if !hres.OK {
		t.Fatalf("hello: %s", hres.Error)
	}
	h.RunFrame()
	send("1", "sys.ping", nil)
	// The runtime replies at a cut; pump frames while the reader waits.
	type result struct {
		res proto.Response
		err error
	}
	ch := make(chan result, 1)
	go func() {
		line, rerr := rw.ReadString('\n')
		if rerr != nil {
			ch <- result{err: rerr}
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
			if !r.res.OK {
				t.Fatalf("ping: %s", r.res.Error)
			}
			var pong struct {
				Cut   string `json:"cut"`
				Frame string `json:"frame"`
			}
			_ = json.Unmarshal(r.res.Result, &pong)
			if _, perr := strconv.ParseUint(pong.Cut, 10, 64); perr != nil || pong.Cut == "0" {
				t.Fatalf("pong cut = %q", pong.Cut)
			}
			return
		default:
		}
		h.RunFrame()
		time.Sleep(time.Millisecond)
	}
	t.Fatal("ping did not reply within the pump budget")
}

func recvLine(t *testing.T, rw *bufio.ReadWriter) proto.Response {
	t.Helper()
	line, err := rw.ReadString('\n')
	if err != nil {
		t.Fatalf("read: %v", err)
	}
	res, err := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	return res
}
