package runtime

import (
	"testing"

	"image/color"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// buildBenchmarkTree builds a modest facet tree (a root hosting several
// children) to measure steady-state frame work.
func buildBenchmarkTree() *runtimeRenderFacet {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 800, 600), color.RGBA{R: 240, G: 240, B: 240, A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		for i, childBase := range root.Base().Children() {
			if childBase == nil {
				continue
			}
			role := childBase.LayoutRole()
			if role == nil {
				continue
			}
			role.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+float32(i)*80, bounds.Width(), 76))
		}
	}
	for i := 0; i < 8; i++ {
		child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 200, 76), color.RGBA{R: 60 + uint8(20*i), G: 100, B: 160, A: 255})
		root.AddChild(child.Base())
	}
	return root
}

// BenchmarkFrameSteadyState measures a repeated frame with no state changes.
// RX-1 AC-6 requires the quiet steady state to do near-zero work: the dirty
// tree stays empty, the layout pass is skipped, the derived flush evaluates
// nothing, and the projection walk is a cache-hit sweep.
func BenchmarkFrameSteadyState(b *testing.B) {
	root := buildBenchmarkTree()
	rt := mustRuntimeTree(b, root)
	rt.RunOneFrame() // warm up + settle

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rt.RunOneFrame()
	}
}

// TestFrameSteadyStateWorkIsNearZero asserts the AC-6 claim in work terms:
// repeated no-change frames keep every reactive counter at zero (no layout
// arrangements, no derived evaluations, no projection recomputes) while the
// projection cache-hit sweep still runs.
func TestFrameSteadyStateWorkIsNearZero(t *testing.T) {
	root := buildBenchmarkTree()
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame() // first frame reacts to dirty-all

	rt.RunOneFrame()
	stats := rt.LastFrameStats()
	if stats.ArrangeCount != 0 {
		t.Fatalf("steady-state ArrangeCount = %d, want 0", stats.ArrangeCount)
	}
	if stats.DerivedEvaluated != 0 || stats.DerivedRecomputed != 0 {
		t.Fatalf("steady-state derived counters = %d/%d, want 0/0", stats.DerivedEvaluated, stats.DerivedRecomputed)
	}
	if stats.DirtyFacets != 0 {
		t.Fatalf("steady-state DirtyFacets = %d, want 0", stats.DirtyFacets)
	}
	if stats.ProjectedFacets != 0 {
		t.Fatalf("steady-state ProjectedFacets = %d, want 0 (nothing re-projected)", stats.ProjectedFacets)
	}
	if stats.CacheHits == 0 {
		t.Fatal("steady-state cache-hit sweep did not run")
	}
	// Gate and hit counters are part of the read-only sweep (nonzero), while
	// the reactive counters are zero.
	if stats.CollectCount != 0 {
		t.Fatalf("steady-state CollectCount = %d, want 0 (no commands re-collected)", stats.CollectCount)
	}
}
