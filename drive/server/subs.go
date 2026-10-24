package server

import (
	"encoding/json"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

// Subscription machinery (LDP §QP-13): clients subscribe to event classes
// with optional server-side filters; each subscription owns a bounded FIFO;
// a full FIFO stops the connection's writer and a clock-measured deadline
// detaches the client loudly. Events are NEVER silently dropped.
const (
	// subFIFODepth is the bounded per-subscription FIFO depth (QP-13).
	subFIFODepth = 256
	// subDetachAfter is the deadline for draining a full FIFO, measured on
	// the runtime clock (QP-13: no sleeps; the fault tests advance virtual
	// frames instead).
	subDetachAfter = 5 * time.Second
)

// Subscription filter classes.
const (
	classStateChanged = "state.changed"
	classTreeChanged  = "tree.changed"
	classFrameDone    = "frame.completed"
	classOwnPreempted = "own.preempted"
)

// Filter selects which events a subscription receives.
type Filter struct {
	Class  string `json:"class"`             // state.changed | tree.changed | frame.completed | own.preempted
	TestID string `json:"test_id,omitempty"` // state.changed: the store's owner facet
	Store  string `json:"store,omitempty"`   // state.changed: the named store
}

// subscription is one client's watched stream: its filter, FIFO, and the
// deadline bookkeeping for backpressure detach.
type subscription struct {
	id     uint64
	filter Filter
	fifo   chan proto.Notification // bounded; full = backpressure

	// backpressureSince is the clock timestamp when the FIFO was first
	// observed full; zero when draining kept up. Detach fires when now -
	// backpressureSince exceeds subDetachAfter.
	backpressureSince time.Time
	detached          bool
}

// clockNow is the injected clock source (set by subTable.publish; nil falls
// back to wall time — the machinery is pure against any monotonic source).
// enqueue delivers one notification to the FIFO. Returns false when the FIFO
// is full: the event is NOT dropped — the caller retains it as the
// head-blocker and the deadline machinery takes over. clock stamps the
// backpressure start on the injected source.
func (s *subscription) enqueue(n proto.Notification, clock func() time.Time) bool {
	select {
	case s.fifo <- n:
		// Recovered within the window; reset (QP-13: detachment fires on a
		// sustained stall, not a single full tick).
		s.backpressureSince = time.Time{}
		return true
	default:
		if s.backpressureSince.IsZero() {
			s.backpressureSince = clock()
		}
		return false
	}
}

// deadlineExceeded reports whether the FIFO has been stalled past the detach
// deadline. The clock parameter is the runtime clock's Now (virtual-frame
// measured in tests, wall in production).
func (s *subscription) deadlineExceeded(now time.Time) bool {
	return !s.backpressureSince.IsZero() && now.Sub(s.backpressureSince) >= subDetachAfter
}

// matches reports whether a published event passes the subscription filter.
func (s *subscription) matches(class, store, testID string) bool {
	if s.filter.Class != class {
		return false
	}
	if s.filter.Store != "" && s.filter.Store != store {
		return false
	}
	if s.filter.TestID != "" && s.filter.TestID != testID {
		return false
	}
	return true
}

// subTable is the per-connection subscription registry. Owned by the
// runtime-side publisher at fan-out (mutex: publish runs on the runtime
// thread, subscribe/cancel run on the connection goroutine) and by the
// connection's writer pump.
type subTable struct {
	mu    sync.Mutex
	next  uint64
	subs  map[uint64]*subscription
	clock func() time.Time
	// detachCh fires (non-blocking) when a subscription crosses its deadline;	// the connection reads it and detaches.
	detachCh chan uint64
	// activity wakes the connection's writer pump (publish/subscribe).
	activity chan struct{}
}

func newSubTable(clock func() time.Time) *subTable {
	return &subTable{
		subs:     make(map[uint64]*subscription),
		clock:    clock,
		detachCh: make(chan uint64, 8),
		activity: make(chan struct{}, 1),
	}
}

// signal wakes the pump without blocking (coalesced).
func (t *subTable) signal() {
	select {
	case t.activity <- struct{}{}:
	default:
	}
}

// subscribe registers a filter and returns the subscription id.
func (t *subTable) subscribe(filter Filter) uint64 {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.next++
	id := t.next
	t.subs[id] = &subscription{
		id:   id,
		fifo: make(chan proto.Notification, subFIFODepth),
	}
	t.subs[id].filter = filter
	t.signal()
	return id
}

// cancel removes a subscription; returns false when the id is unknown.
func (t *subTable) cancel(id uint64) bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	if _, ok := t.subs[id]; !ok {
		return false
	}
	delete(t.subs, id)
	return true
}

// publish fans one event out to every matching subscription. For each FULL
// FIFO the event is retained as the head-blocker (never dropped) and the
// deadline check runs: sustained stalls detach with sub.dead.
func (t *subTable) publish(class, store, testID string, params json.RawMessage) {
	now := t.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	for _, s := range t.subs {
		if !s.matches(class, store, testID) {
			continue
		}
		s.enqueue(proto.Notification{Method: class, Params: params}, t.clock)
	}
	t.signal()
	// Deadline sweep (QP-13: detach within 5s of an unwritten FIFO).
	for id, s := range t.subs {
		if s.deadlineExceeded(now) && !s.detached {
			s.detached = true
			select {
			case t.detachCh <- id:
			default:
			}
		}
	}
}

// drain detaches and reports the subscriptions whose deadlines expired (the
// connection closes them with backpressure.detached). Pure inspection for
// tests: production detach flows through detachCh.
func (t *subTable) expired() []*subscription {
	now := t.clock()
	t.mu.Lock()
	defer t.mu.Unlock()
	var expired []*subscription
	for _, s := range t.subs {
		if s.deadlineExceeded(now) {
			expired = append(expired, s)
		}
	}
	return expired
}

// pending reports the queued notification count.
func (s *subscription) pending() int {
	return len(s.fifo)
}
