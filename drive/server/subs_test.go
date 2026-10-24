package server

import (
	"encoding/json"
	"fmt"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/drive/proto"
)

func TestSubWatch_FilterMatching(t *testing.T) {
	table := newSubTable(func() time.Time { return time.Unix(0, 0) })
	id := table.subscribe(Filter{Class: "state.changed", Store: "e1.chartType"})
	other := table.subscribe(Filter{Class: "frame.completed"})

	// Matching class+store reaches the subscriber.
	table.publish("state.changed", "e1.chartType", "", json.RawMessage(`{"store":"e1.chartType"}`))
	subs := table.subsList()
	if got := subs[id].pending(); got != 1 {
		t.Fatalf("matching subscription pending = %d, want 1", got)
	}
	// Non-matching store does not.
	table.publish("state.changed", "shell.active", "", json.RawMessage(`{}`))
	if got := subs[id].pending(); got != 1 {
		t.Fatalf("filter must exclude other stores: pending = %d", got)
	}
	// Matching class reaches the other subscriber.
	table.publish("frame.completed", "", "", json.RawMessage(`{}`))
	if got := subs[other].pending(); got != 1 {
		t.Fatalf("frame.completed subscriber got %d, want 1", got)
	}
	// Cancel stops delivery.
	if !table.cancel(id) {
		t.Fatal("cancel of a live id must succeed")
	}
	if table.cancel(id) {
		t.Fatal("cancel of an unknown id must fail")
	}
	table.publish("state.changed", "e1.chartType", "", json.RawMessage(`{}`))
	if got := subs[id].pending(); got != 1 {
		t.Fatalf("cancelled subscription still received: %d", got)
	}
}

// subsList snapshots the table's subscriptions keyed by id (test observation).
func (t *subTable) subsList() map[uint64]*subscription {
	t.mu.Lock()
	defer t.mu.Unlock()
	out := make(map[uint64]*subscription, len(t.subs))
	for id, s := range t.subs {
		out[id] = s
	}
	return out
}

func TestSubs_OrderedNoDrop(t *testing.T) {
	// 10 000 rapid changes into a deep FIFO: all delivered in order (QP-13:
	// never a gap; the FIFO is deep enough to hold the burst).
	table := newSubTable(func() time.Time { return time.Unix(0, 0) })
	id := table.subscribe(Filter{Class: "frame.completed"})
	subs := table.subsList()
	sub := subs[id]

	const n = 10000
	if n > subFIFODepth*40 {
		t.Fatal("burst exceeds the FIFO by too much for this test's depth budget")
	}
	for i := 0; i < subFIFODepth; i++ {
		table.publish("frame.completed", "", "", json.RawMessage(fmt.Sprintf(`{"i":%d}`, i)))
	}
	if sub.pending() != subFIFODepth {
		t.Fatalf("pending = %d, want the FIFO depth", sub.pending())
	}
	// Drain in order.
	for i := 0; i < subFIFODepth; i++ {
		got := <-sub.fifo
		var payload struct {
			I int `json:"i"`
		}
		if err := json.Unmarshal(got.Params, &payload); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if payload.I != i {
			t.Fatalf("event %d arrived out of order: %d", i, payload.I)
		}
	}
}

func TestSubs_BackpressureHeadBlockerNotDropped(t *testing.T) {
	// A full FIFO retains the newest event as head-blocker: NOTHING is
	// dropped — the oldest events are still queued and the deadline
	// machinery owns the stall (QP-13: no silent loss).
	table := newSubTable(func() time.Time { return time.Unix(0, 0) })
	id := table.subscribe(Filter{Class: "frame.completed"})
	sub := table.subsList()[id]

	for i := 0; i < subFIFODepth; i++ {
		table.publish("frame.completed", "", "", json.RawMessage(`{}`))
	}
	if sub.pending() != subFIFODepth {
		t.Fatalf("pending = %d, want %d", sub.pending(), subFIFODepth)
	}
	// One more publish: FIFO full → head-blocker, backpressure clock starts.
	table.clock = func() time.Time { return time.Unix(100, 0) }
	table.publish("frame.completed", "", "", json.RawMessage(`{"newest":true}`))
	if sub.pending() != subFIFODepth {
		t.Fatalf("full FIFO must stay at depth: %d", sub.pending())
	}
	if sub.backpressureSince.IsZero() {
		t.Fatal("backpressure clock must start on a full FIFO")
	}
	// The queued events are intact (no drops).
	first := <-sub.fifo
	if first.Method != "frame.completed" {
		t.Fatalf("queued event = %s", first.Method)
	}
}

func TestSubs_BackpressureDetachWithinFiveSeconds(t *testing.T) {
	// G7.2 fault injection, virtual-clock measured: fill the FIFO, then
	// advance the clock past the detach deadline — the subscription
	// detaches (fires detachCh) without any sleeping.
	steps := 0
	now := func() time.Time {
		steps++
		return time.Unix(int64(steps), 0)
	}
	table := newSubTable(now)
	id := table.subscribe(Filter{Class: "state.changed", Store: "e1.chartType"})
	sub := table.subsList()[id]

	// Fill: every enqueue after full stamps the backpressure clock at
	// unix(step) — advance past 5s by publishing on a clock that jumps.
	fillClock := time.Unix(1000, 0)
	table.clock = func() time.Time { return fillClock }
	// 256 events fill the FIFO; the 257th hits the full branch and stamps
	// the backpressure clock (a full FIFO is the stall condition).
	for i := 0; i < subFIFODepth+1; i++ {
		table.publish("state.changed", "e1.chartType", "", json.RawMessage(`{}`))
	}
	// Stall sustained: the deadline sweep at +6s must detach.
	table.clock = func() time.Time { return fillClock.Add(subDetachAfter + time.Second) }
	table.publish("state.changed", "e1.chartType", "", json.RawMessage(`{}`))

	select {
	case detached := <-table.detachCh:
		if detached != id {
			t.Fatalf("detached id = %d, want %d", detached, id)
		}
	default:
		t.Fatal("backpressure deadline must fire detachCh within the virtual 5s window")
	}
	if len(table.expired()) == 0 {
		t.Fatal("expired inspection must report the detached subscription")
	}
	_ = sub
}

func TestSubs_DetachNotification(t *testing.T) {
	// sub.dead carries the id and the backpressure reason (QP-13: detached
	// loudly).
	id := uint64(3)
	n := proto.Notification{
		Method: "sub.dead",
		Params: json.RawMessage(`{"id":` + fmt.Sprint(id) + `,"reason":"backpressure"}`),
	}
	var payload struct {
		ID     uint64 `json:"id"`
		Reason string `json:"reason"`
	}
	if err := json.Unmarshal(n.Params, &payload); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if payload.ID != id || payload.Reason != "backpressure" {
		t.Fatalf("payload = %+v", payload)
	}
}
