package runtime

import (
	"encoding/json"
	"testing"
	"time"
)

// flushEventsTest: the cut flush publishes frame.completed every frame,
// tree.changed only on dirty frames, and state.changed only for named
// stores whose versions changed — a store mutated mid-frame yields exactly
// one state.changed at the frame's cut (QP-13).
func TestDriveEvents_CutFlushClasses(t *testing.T) {
	rt, probe := newReadRuntime(t)

	type event struct {
		class  string
		params map[string]any
	}
	var events []event
	rt.driveSink.SetEventConsumer(func(class, storeName, testID string, params json.RawMessage) {
		var m map[string]any
		_ = json.Unmarshal(params, &m)
		events = append(events, event{class: class, params: m})
	})
	if err := rt.RegisterNamedStore("probe.opacity", probe.Opacity); err != nil {
		t.Fatalf("register: %v", err)
	}

	epoch := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	// Frame 1: dirty (settling) → frame.completed + tree.changed +
	// state.changed for the freshly registered store.
	rt.RunOneFrameAt(epoch)
	// Frame 2: quiet → frame.completed only.
	rt.RunOneFrameAt(epoch.Add(time.Second))

	classes := func() []string {
		out := make([]string, 0, len(events))
		for _, e := range events {
			out = append(out, e.class)
		}
		return out
	}
	first := classes()
	hasClass := func(c string) bool {
		for _, k := range first {
			if k == c {
				return true
			}
		}
		return false
	}
	if !hasClass("frame.completed") {
		t.Fatalf("frame 1 events = %v, missing frame.completed", first)
	}
	if !hasClass("tree.changed") {
		t.Fatalf("dirty frame must publish tree.changed: %v", first)
	}
	if !hasClass("state.changed") {
		t.Fatalf("fresh named store must publish state.changed on first flush: %v", first)
	}

	// Frame 3: mutate the store → exactly one state.changed at this cut.
	events = nil
	probe.Opacity.Set(0.1)
	rt.RunOneFrameAt(epoch.Add(2 * time.Second))
	changed := 0
	for _, e := range events {
		if e.class == "state.changed" {
			changed++
		}
	}
	if changed != 1 {
		t.Fatalf("state.changed count after one mutation = %d, want 1 (cut-exact, no spam)", changed)
	}
	// Quiet frames after: no more state.changed.
	events = nil
	rt.RunOneFrameAt(epoch.Add(3 * time.Second))
	for _, e := range events {
		if e.class == "state.changed" {
			t.Fatal("quiet frame must not publish state.changed")
		}
	}
}

// TestDriveEvents_NoConsumerNoWork: with no consumer registered the flush
// allocates nothing (NFR-4 / QP-13 gating) — the consumer nil check is the
// whole cost.
func TestDriveEvents_NoConsumerNoWork(t *testing.T) {
	rt, _ := newReadRuntime(t)
	stats := rt.LastFrameStats()
	allocs := testing.AllocsPerRun(50, func() {
		rt.flushEvents(stats)
	})
	if allocs != 0 {
		t.Fatalf("flush without consumer allocates %v times, want 0", allocs)
	}
}

// TestDriveEvents_OwnPreemptedFannedOut: preemption publishes
// own.preempted through the same consumer (verified in the preemption
// test; here the payload shape is pinned).
func TestDriveEvents_OwnPreemptedPayload(t *testing.T) {
	rt, _ := newReadRuntime(t)
	var got map[string]any
	rt.driveSink.SetEventConsumer(func(class, store, testID string, params json.RawMessage) {
		if class == "own.preempted" {
			_ = json.Unmarshal(params, &got)
		}
	})
	rt.driveSink.notifyPreempted(7, "drive")
	if got["command_id"] != float64(7) || got["by"] != "human" {
		t.Fatalf("own.preempted payload = %+v", got)
	}
}
