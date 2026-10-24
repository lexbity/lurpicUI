package server

import (
	"encoding/json"
	"strings"
	"sync"
	"testing"
	"time"
)

// watchingSink is a fake sink that implements the event-consumer seam so the
// server registers its fan-out (the real path: the runtime sink does this
// structurally).
type watchingSink struct {
	fakeSink
	mu       sync.Mutex
	consumer func(class, store, testID string, params json.RawMessage)
}

func (w *watchingSink) SetEventConsumer(fn func(class, store, testID string, params json.RawMessage)) {
	w.mu.Lock()
	w.consumer = fn
	w.mu.Unlock()
}

// publish drives one event through the registered consumer (the fan-out the
// runtime's cut flush performs).
func (w *watchingSink) publish(class, store, testID string, params json.RawMessage) {
	w.mu.Lock()
	fn := w.consumer
	w.mu.Unlock()
	if fn != nil {
		fn(class, store, testID, params)
	}
}

// TestServer_WatchEndToEnd delivers notifications to a real socket client:
// sub.watch filters events, sub.cancel stops them, and notifications carry
// their class (G7.2/G7.3/G7.4 at the wire level).
func TestServer_WatchEndToEnd(t *testing.T) {
	dir := t.TempDir()
	socket := dir + "/drive.sock"
	token := testToken()
	sink := &watchingSink{}
	srv, err := Listen(Config{SocketPath: socket, Token: token, Grants: Grants{Read: true}, Sink: sink})
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	defer srv.Close()

	c := dial(t, socket)
	if res := c.hello(token); !res.OK {
		t.Fatalf("hello: %+v", res.Error)
	}

	res := c.call("sub.watch", map[string]any{"filter": map[string]string{"class": "frame.completed"}})
	if !res.OK {
		t.Fatalf("sub.watch: %+v", res.Error)
	}
	var sub struct {
		ID uint64 `json:"id"`
	}
	if err := json.Unmarshal(res.Result, &sub); err != nil || sub.ID == 0 {
		t.Fatalf("sub.watch result = %s (%v)", res.Result, err)
	}

	// Drive an event through the fan-out (the runtime's cut flush path).
	sink.publish("frame.completed", "", "", json.RawMessage(`{"epoch":1}`))

	// Bounded read (no sleeps): the pump delivers within the deadline.
	c.nc.SetReadDeadline(time.Now().Add(2 * time.Second))
	got, err := c.rw.ReadString('\n')
	if err != nil {
		t.Fatalf("watched notification read: %v", err)
	}
	got = strings.TrimSpace(got)
	c.nc.SetReadDeadline(time.Time{})
	if !strings.Contains(got, "frame.completed") {
		t.Fatalf("watched notification = %q", got)
	}

	// sub.cancel stops delivery: after cancel, a published event produces
	// nothing within the wait window.
	if res := c.call("sub.cancel", map[string]any{"id": sub.ID}); !res.OK {
		t.Fatalf("sub.cancel: %+v", res.Error)
	}
	sink.publish("frame.completed", "", "", json.RawMessage(`{"epoch":2}`))
	c.nc.SetReadDeadline(time.Now().Add(200 * time.Millisecond))
	if line, err := c.rw.ReadString('\n'); err == nil && strings.TrimSpace(line) != "" {
		t.Fatalf("cancelled subscription still delivered: %q", line)
	}
	c.nc.SetReadDeadline(time.Time{})
}
