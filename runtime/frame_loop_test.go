package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// TestFrameLoopSinglePhase pins the RX-1 P5 single-phase loop: RunOneFrame
// drives one collect→project→compose pass and reports a populated FrameStats
// with the hot-path counters.
func TestFrameLoopSinglePhase(t *testing.T) {
	root := newLayoutCountLeaf(gfx.Size{W: 100, H: 50})
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()

	stats := rt.LastFrameStats()
	if stats.FrameNumber != 1 {
		t.Fatalf("frame number = %d, want 1", stats.FrameNumber)
	}
	// The first frame projects the leaf (uncached) — projection ran.
	if stats.ProjectedFacets == 0 {
		t.Fatal("first frame projected nothing")
	}
	// The projection gate evaluated at least the leaf (GateCount >= 1).
	if stats.GateCount < 1 {
		t.Fatalf("gate count = %d, want >= 1", stats.GateCount)
	}
	// The first frame arranged the leaf's layout root.
	if stats.ArrangeCount == 0 {
		t.Fatal("first frame arranged nothing")
	}
}

// TestFrameLoopDirtyTreeRebuiltPerFrame pins the dirtyTree rebuild contract:
// after a frame that cleared the dirty set, a second frame with no new state
// changes performs near-zero work (no layout pass, no derived flush, no
// projection recompute) — the quiet steady state (RX-1 AC-6).
func TestFrameLoopDirtyTreeRebuiltPerFrame(t *testing.T) {
	root := newLayoutCountLeaf(gfx.Size{W: 100, H: 50})
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()
	first := rt.LastFrameStats()

	// No state changes: a second frame must not re-lay or re-flush.
	rt.RunOneFrame()
	second := rt.LastFrameStats()

	if len(rt.LastDirtySnapshot()) != 0 {
		t.Fatalf("steady-state frame has a non-empty dirty set: %d facets", len(rt.LastDirtySnapshot()))
	}
	if second.ArrangeCount != 0 {
		t.Fatalf("steady-state arrange count = %d, want 0", second.ArrangeCount)
	}
	if second.DerivedEvaluated != 0 || second.DerivedRecomputed != 0 {
		t.Fatalf("steady-state derived evals=%d recomputes=%d, want 0/0", second.DerivedEvaluated, second.DerivedRecomputed)
	}
	_ = first
}

// TestFrameLoopDerivedFlushBeforeProjection pins the flush-before-projection
// contract: a store write dirties a derived, and the frame's flush recomputes
// it (DerivedRecomputed > 0) before the projection reads it.
func TestFrameLoopDerivedFlushBeforeProjection(t *testing.T) {
	root := facet.NewFacet()
	rt := mustRuntimeTree(t, &root)
	// Start the runtime first so its signal queue is the live hook: store
	// writes defer derived invalidation through the runtime's queue, which the
	// next frame drains before the flush.
	rt.RunOneFrame()

	src := store.NewValueStore(1)
	derived := store.NewDerived(func() int { return src.Get() * 2 }, src)
	_ = derived.Get() // initialize (clean)
	store.ResetFrameDerivedCounters()

	src.Set(2)
	rt.RunOneFrame()

	stats := rt.LastFrameStats()
	if stats.DerivedEvaluated == 0 {
		t.Fatal("derived flush did not evaluate the dirty derived")
	}
	if stats.DerivedRecomputed == 0 {
		t.Fatal("derived flush did not recompute the dirty derived")
	}
	if got := derived.Get(); got != 4 {
		t.Fatalf("derived value = %d, want 4 (flush recomputed from source)", got)
	}
}

// TestFrameLoopLazyRecomputeCounts pins the derived counters covering lazy
// Get calls during projection as well as the flush: after a change, the frame's
// recompute count reflects the recomputation regardless of which path triggered
// it.
func TestFrameLoopLazyRecomputeCounts(t *testing.T) {
	src := store.NewValueStore(10)
	derived := store.NewDerived(func() int { return src.Get() + 1 }, src)
	if got := derived.Get(); got != 11 {
		t.Fatalf("initial derived = %d, want 11", got)
	}
	store.ResetFrameDerivedCounters()
	src.Set(20)
	if got := derived.Get(); got != 21 {
		t.Fatalf("lazy derived = %d, want 21", got)
	}
	evals, recomputes := store.SnapshotFrameDerivedCounters()
	if evals != 1 || recomputes != 1 {
		t.Fatalf("frame derived counters = %d/%d, want 1/1", evals, recomputes)
	}
}

// TestFrameLoopSteadyStateDerivedQuiet pins the exit-criteria claim directly:
// in the quiet steady state (no writes between frames) the derived flush
// evaluates and recomputes nothing.
func TestFrameLoopSteadyStateDerivedQuiet(t *testing.T) {
	root := facet.NewFacet()
	rt := mustRuntimeTree(t, &root)

	src := store.NewValueStore(1)
	_ = store.NewDerived(func() int { return src.Get() }, src)
	rt.RunOneFrame()
	rt.RunOneFrame()

	stats := rt.LastFrameStats()
	if stats.DerivedEvaluated != 0 || stats.DerivedRecomputed != 0 {
		t.Fatalf("steady-state derived evals=%d recomputes=%d, want 0/0", stats.DerivedEvaluated, stats.DerivedRecomputed)
	}
}
