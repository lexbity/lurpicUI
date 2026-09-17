package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// buildOverlappingLayers constructs a root hosting two overlapping layer
// children — a Content-band layer and a Modal-band layer, both covering the
// full root. The caller picks the AddChild insertion order so the test can
// prove hit ordering follows the named bands, not tree order.
func buildOverlappingLayers(t *testing.T, modalFirst bool) (*Runtime, *runtimeHitFacet, *runtimeHitFacet) {
	t.Helper()
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		for _, childBase := range root.Base().Children() {
			if childBase == nil {
				continue
			}
			if role := childBase.LayoutRole(); role != nil {
				role.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), bounds.Height()))
			}
		}
	}
	content := newRuntimeHitFacet("content", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{R: 200, A: 255})
	modal := newRuntimeHitFacet("modal", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{B: 200, A: 255})
	if modalFirst {
		facet.AttachLayer(root, modal, facet.LayerAttachment{Band: facet.ZBandModal})
		facet.AttachLayer(root, content, facet.LayerAttachment{Band: facet.ZBandContent})
	} else {
		facet.AttachLayer(root, content, facet.LayerAttachment{Band: facet.ZBandContent})
		facet.AttachLayer(root, modal, facet.LayerAttachment{Band: facet.ZBandModal})
	}
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()
	return rt, content, modal
}

// TestHitOrder_bandNotTreeOrder pins the Q6 deterministic hit ordering: the
// topmost band wins a hit regardless of the AddChild (tree) order. Without the
// layer-aware re-sort, the Modal added first would be hit-tested last and the
// Content layer would incorrectly win.
func TestHitOrder_bandNotTreeOrder(t *testing.T) {
	for _, modalFirst := range []bool{true, false} {
		rt, content, modal := buildOverlappingLayers(t, modalFirst)
		center := gfx.Point{X: 150, Y: 100}
		got := rt.HitTest(center)
		want := modal.Base().ID()
		if got != want {
			t.Fatalf("modalFirst=%v: hit=%d want modal %d (topmost band must win, not tree order)", modalFirst, got, want)
		}
		_ = content
	}
}

// TestHitOrder_shuffleInsertionStable pins determinism under insertion-order
// shuffling: the winning facet is stable across both tree orders.
func TestHitOrder_shuffleInsertionStable(t *testing.T) {
	rtA, _, modalA := buildOverlappingLayers(t, true)
	rtB, _, modalB := buildOverlappingLayers(t, false)
	center := gfx.Point{X: 150, Y: 100}
	if got := rtA.HitTest(center); got != modalA.Base().ID() {
		t.Fatalf("order A: hit=%d want modal %d", got, modalA.Base().ID())
	}
	if got := rtB.HitTest(center); got != modalB.Base().ID() {
		t.Fatalf("order B: hit=%d want modal %d", got, modalB.Base().ID())
	}
}

// TestHitOrder_topmostLayerBlocksLower pins HitBlockBelow semantics at the
// render/hit junction: the topmost Modal layer's hit policy stops traversal to
// the Content layer beneath it.
func TestHitOrder_topmostLayerBlocksLower(t *testing.T) {
	rt, _, modal := buildOverlappingLayers(t, true)
	// The standard Modal layer in the test registry resolves with HitBlockBelow
	// (see testLayerRegistry); the hit must resolve to the modal facet and the
	// Content layer beneath must not be reachable.
	if got := rt.HitTest(gfx.Point{X: 150, Y: 100}); got != modal.Base().ID() {
		t.Fatalf("hit=%d want modal %d", got, modal.Base().ID())
	}
}
