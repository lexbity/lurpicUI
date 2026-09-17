package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestFrameStatsShape pins the RX-1 P5 FrameStats shape: every hot-path
// counter is present on the struct, is frame-scoped (reset each frame), and is
// monotonic within a frame.
func TestFrameStatsShape(t *testing.T) {
	root := newLayoutCountLeaf(gfx.Size{W: 100, H: 50})
	rt := mustRuntimeTree(t, root)

	rt.RunOneFrame()
	first := rt.LastFrameStats()

	// Frame-scoped counters exist and are populated on a frame that reacted to
	// the initial dirty-all state.
	if first.GateCount == 0 {
		t.Error("GateCount is zero on the first frame")
	}
	if first.PruneCount < 0 || first.CollectCount < 0 || first.MaterializeCount < 0 {
		t.Errorf("a hot-path counter went negative: %+v", first)
	}
	if first.HitTestCount < 0 || first.LayerResolveCount < 0 {
		t.Errorf("a hot-path counter went negative: %+v", first)
	}

	// A second, no-change frame resets the counters: the quiet steady state
	// reports zero arrange and zero derived work (RX-1 AC-6).
	rt.RunOneFrame()
	second := rt.LastFrameStats()
	if second.ArrangeCount != 0 {
		t.Errorf("steady-state ArrangeCount = %d, want 0", second.ArrangeCount)
	}
	if second.DerivedEvaluated != 0 || second.DerivedRecomputed != 0 {
		t.Errorf("steady-state derived counters = %d/%d, want 0/0", second.DerivedEvaluated, second.DerivedRecomputed)
	}
	if second.DirtyFacets != 0 {
		t.Errorf("steady-state DirtyFacets = %d, want 0", second.DirtyFacets)
	}
	if second.FrameNumber != first.FrameNumber+1 {
		t.Errorf("FrameNumber = %d, want %d", second.FrameNumber, first.FrameNumber+1)
	}

}
