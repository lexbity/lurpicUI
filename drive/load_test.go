package drive_test

import (
	"bufio"
	"encoding/json"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	drive "codeburg.org/lexbit/lurpicui/drive/server"
)

// loadSink answers every command at the cut with a per-command frame epoch.
type loadSink struct {
	mu    sync.Mutex
	epoch uint64
}

func (s *loadSink) Enqueue(cmd drive.Command) {
	s.mu.Lock()
	s.epoch++
	frame := s.epoch
	s.mu.Unlock()
	payload, merr := json.Marshal(map[string]uint64{"frame": frame})
	if merr != nil { // cannot fail for this payload; keep the sink total
		payload = json.RawMessage(`{"frame":0}`)
	}
	cmd.Reply <- drive.Result{Payload: payload, Frame: frame}
}

func (s *loadSink) Epoch() uint64 {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.epoch
}

// TestLoadTenClients is the G10.4 load gate: 10 concurrent clients, mixed
// verbs, budget-gated. Every reply must be well-formed and carry a
// monotonically increasing frame within its client.
func TestLoadTenClients(t *testing.T) {
	dir := t.TempDir()
	socket := filepath.Join(dir, "drive.sock")
	token := strings.Repeat("ab", 32)
	sink := &loadSink{}
	srv, err := drive.Listen(drive.Config{
		SocketPath: socket,
		Token:      token,
		Grants:     drive.Grants{Read: true, Write: true, Input: true, Time: true, Admin: true},
		Sink:       sink,
	})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	// Surface the server-side panic counter in the failure output: a broken
	// pipe on every client means a connection goroutine panicked.
	t.Cleanup(func() {
		d, s, g, p := srv.StatsSnapshot()
		t.Logf("drive stats: drained=%d sent=%d grantDenied=%d panics=%d", d, s, g, p)
	})

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
			next := uint64(0)
			// params is pre-encoded JSON (json.RawMessage): marshaling any
			// here would double-encode []byte params as base64 strings.
			call := func(method string, params json.RawMessage) (proto.Response, error) {
				next++
				req, err := json.Marshal(proto.Request{V: proto.ProtocolMajor, ID: next, Method: method, Params: params})
				if err != nil {
					return proto.Response{}, err
				}
				if _, err := rw.Write(append(req, '\n')); err != nil {
					return proto.Response{}, err
				}
				if err := rw.Flush(); err != nil {
					return proto.Response{}, err
				}
				line, err := rw.ReadString('\n')
				if err != nil {
					return proto.Response{}, err
				}
				var res proto.Response
				if err := json.Unmarshal([]byte(strings.TrimSpace(line)), &res); err != nil {
					return proto.Response{}, err
				}
				return res, nil
			}
			// Handshake.
			helloParams, err := json.Marshal(map[string]string{"token": token})
			if err != nil {
				errCh <- fmt.Errorf("client %d marshal hello: %w", client, err)
				return
			}
			if _, err := call("sys.hello", helloParams); err != nil {
				errCh <- fmt.Errorf("client %d hello: %w", client, err)
				return
			}
			// Mixed verbs, ordered per client.
			var lastFrame uint64
			for i := 0; i < perClient; i++ {
				var res proto.Response
				switch i % 3 {
				case 0:
					res, err = call("sys.ping", nil)
				case 1:
					clickParams, merr := json.Marshal(map[string]float32{"x": float32(i), "y": float32(client)})
					if merr != nil {
						errCh <- fmt.Errorf("client %d marshal click: %w", client, merr)
						return
					}
					res, err = call("input.click", clickParams)
				default:
					res, err = call("store.read", json.RawMessage(`{"name":"store"}`))
				}
				if err != nil {
					errCh <- fmt.Errorf("client %d call %d: %w", client, i, err)
					return
				}
				if !res.OK {
					errCh <- fmt.Errorf("client %d call %d failed: %s", client, i, res.Error.Error())
					return
				}
				if res.Frame < lastFrame {
					errCh <- fmt.Errorf("client %d frame went backwards: %d after %d", client, res.Frame, lastFrame)
					return
				}
				lastFrame = res.Frame
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
	if sink.Epoch() < clients*perClient/3 {
		t.Fatalf("sink epoch = %d, want at least the ping volume", sink.Epoch())
	}
}
