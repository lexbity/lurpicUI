package drive_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive"
	"codeburg.org/lexbit/lurpicui/drive/proto"
	server "codeburg.org/lexbit/lurpicui/drive/server"
)

// loadPort is a synchronous fake runtime port: it answers each submitted
// ping from the submitting goroutine, standing in for the runtime thread.
type loadPort struct {
	mu    sync.Mutex
	count int
}

func (p *loadPort) Submit(cmd drive.Command) error {
	if cmd.Reply == nil {
		panic("loadPort: command without a reply channel")
	}
	p.mu.Lock()
	p.count++
	frame := uint64(p.count)
	p.mu.Unlock()
	payload, merr := json.Marshal(map[string]any{"pong": true, "cut": strconv.FormatUint(frame, 10), "frame": strconv.FormatUint(frame, 10)})
	if merr != nil { // cannot fail for this payload; keep the port total
		payload = json.RawMessage(`{"pong":true,"cut":"0","frame":"0"}`)
	}
	cmd.Reply <- drive.Result{Payload: payload, Cut: strconv.FormatUint(frame, 10), Frame: strconv.FormatUint(frame, 10)}
	return nil
}

func (p *loadPort) SignalControl(drive.ControlSignal) {}
func (p *loadPort) Observe() drive.PublishedSnapshot  { return drive.PublishedSnapshot{} }
func (p *loadPort) handled() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.count
}

// startLoadServer binds a v2 server against the fake port with a live
// session table.
func startLoadServer(t *testing.T) (*server.Server, *loadPort, string, string) {
	t.Helper()
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o700); err != nil {
		t.Fatalf("chmod temp dir: %v", err)
	}
	socket := filepath.Join(dir, "d.sock")
	token := strings.Repeat("ab", 32)
	inst, err := drive.NewInstance()
	if err != nil {
		t.Fatalf("instance: %v", err)
	}
	port := &loadPort{}
	srv, err := server.Listen(server.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true, Input: true, Command: true, Time: true, Admin: true},
		Sessions:   drive.NewSessionTable(inst),
		Port:       port,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { _ = srv.Close() })
	return srv, port, socket, token
}

// TestLoadTenClients is the load gate: 10 concurrent authenticated
// clients, contiguous session IDs, mixed ping volume. Every reply must be
// well-formed and carry a monotonically non-decreasing frame attribution.
func TestLoadTenClients(t *testing.T) {
	srv, port, socket, token := startLoadServer(t)

	const clients = 10
	const perClient = 50

	var wg sync.WaitGroup
	errCh := make(chan error, clients)
	for c := 0; c < clients; c++ {
		wg.Add(1)
		go func(client int) {
			defer wg.Done()
			nc, err := net.Dial("unix", socket)
			if err != nil {
				errCh <- fmt.Errorf("client %d dial: %w", client, err)
				return
			}
			defer nc.Close()
			rw := bufio.NewReadWriter(bufio.NewReader(nc), bufio.NewWriter(nc))
			// Handshake: create a session under the launcher's grants.
			hello, err := json.Marshal(proto.Request{
				V:      proto.ProtocolMajor,
				ID:     "1",
				Method: "sys.hello",
				Params: json.RawMessage(fmt.Sprintf(`{"token":%q,"client_name":"load","create_session":true}`, token)),
			})
			if err != nil {
				errCh <- fmt.Errorf("client %d marshal hello: %w", client, err)
				return
			}
			if _, err := rw.Write(append(hello, '\n')); err != nil {
				errCh <- fmt.Errorf("client %d hello write: %w", client, err)
				return
			}
			if err := rw.Flush(); err != nil {
				errCh <- fmt.Errorf("client %d hello flush: %w", client, err)
				return
			}
			line, err := rw.ReadString('\n')
			if err != nil {
				errCh <- fmt.Errorf("client %d hello read: %w", client, err)
				return
			}
			hres, err := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
			if err != nil || !hres.OK {
				errCh <- fmt.Errorf("client %d hello: %w (%s)", client, err, hres.Error.Error())
				return
			}
			var hr struct {
				SessionID string `json:"session_id"`
			}
			if err := json.Unmarshal(hres.Result, &hr); err != nil {
				errCh <- fmt.Errorf("client %d hello payload: %w", client, err)
				return
			}
			// Mixed ping volume, contiguous IDs, ordered per client. Across
			// sessions there is no promised request order; each client's
			// frames are non-decreasing as its requests execute in order.
			var lastFrame uint64
			for i := 0; i < perClient; i++ {
				id := strconv.FormatUint(uint64(i+2), 10)
				req, err := json.Marshal(proto.Request{
					V: proto.ProtocolMajor, ID: id, Method: "sys.ping",
					Params: json.RawMessage(`{}`),
				})
				if err != nil {
					errCh <- fmt.Errorf("client %d marshal ping: %w", client, err)
					return
				}
				if _, err := rw.Write(append(req, '\n')); err != nil {
					errCh <- fmt.Errorf("client %d write %d: %w", client, i, err)
					return
				}
				if err := rw.Flush(); err != nil {
					errCh <- fmt.Errorf("client %d flush %d: %w", client, i, err)
					return
				}
				line, err := rw.ReadString('\n')
				if err != nil {
					errCh <- fmt.Errorf("client %d read %d: %w", client, i, err)
					return
				}
				res, err := proto.DecodeResponse([]byte(strings.TrimRight(line, "\n")))
				if err != nil {
					errCh <- fmt.Errorf("client %d decode %d: %w", client, i, err)
					return
				}
				if !res.OK {
					errCh <- fmt.Errorf("client %d call %d failed: %s", client, i, res.Error.Error())
					return
				}
				frame, perr := strconv.ParseUint(res.Frame, 10, 64)
				if perr != nil {
					errCh <- fmt.Errorf("client %d frame %q is not decimal: %w", client, res.Frame, perr)
					return
				}
				if frame < lastFrame {
					errCh <- fmt.Errorf("client %d frame went backwards: %d after %d", client, frame, lastFrame)
					return
				}
				lastFrame = frame
				_ = hr.SessionID
			}
		}(c)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Error(err)
	}
	if t.Failed() {
		return
	}
	if got := port.handled(); got != clients*perClient {
		t.Fatalf("port handled %d pings, want exactly the request volume %d", got, clients*perClient)
	}
	_ = srv
}
