package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestNoTransientLayers_stableAcrossFrames pins the Q6 "no transient layers"
// contract: a mounted layer resolves to the same projection layer (LayerID,
// bounds) and the same assembled render-batch grouping on every frame — a
// layer present in one frame must not flicker out on the next.
func TestNoTransientLayers_stableAcrossFrames(t *testing.T) {
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
	overlay := newRuntimeHitFacet("overlay", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{B: 200, A: 255})
	facet.AttachLayer(root, overlay, facet.LayerAttachment{Band: facet.ZBandModal})
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()

	var lastLayer facet.LayerID
	var lastBounds gfx.Rect
	for i := 0; i < 5; i++ {
		rt.RunOneFrame()
		layer, ok := rt.ResolveProjectionLayer(overlay.Base().ID())
		if !ok {
			t.Fatalf("frame %d: overlay layer transiently disappeared", i)
		}
		if layer.LayerID == 0 {
			t.Fatalf("frame %d: overlay resolved with a zero LayerID", i)
		}
		if i == 0 {
			lastLayer = layer.LayerID
			lastBounds = layer.Bounds
			continue
		}
		if layer.LayerID != lastLayer || layer.Bounds != lastBounds {
			t.Fatalf("frame %d: overlay layer unstable (id %d→%d bounds %v→%v)",
				i, lastLayer, layer.LayerID, lastBounds, layer.Bounds)
		}
	}
}

// TestNoTransientLayers_batchGroupingStable pins the render side: the
// assembled frame's batch grouping for a mounted layer is identical across
// steady-state frames (no flicker at the batch level).
func TestNoTransientLayers_batchGroupingStable(t *testing.T) {
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
	overlay := newRuntimeHitFacet("overlay", gfx.RectFromXYWH(0, 0, 300, 200), color.RGBA{B: 200, A: 255})
	facet.AttachLayer(root, overlay, facet.LayerAttachment{Band: facet.ZBandModal})
	rt := mustRuntimeTree(t, root)
	rt.RunOneFrame()
	rt.RunOneFrame() // settle

	// Sample the assembled frame's batch sequence twice in the steady state.
	first := assembledBatchIDs(t, rt)
	for i := 0; i < 3; i++ {
		rt.RunOneFrame()
		got := assembledBatchIDs(t, rt)
		if len(got) != len(first) {
			t.Fatalf("iteration %d: batch sequence length %d != %d (transient batch)", i, len(got), len(first))
		}
		for j := range got {
			if got[j] != first[j] {
				t.Fatalf("iteration %d: batch[%d] = %d, want %d (transient batch)", i, j, got[j], first[j])
			}
		}
	}
}

// assembledBatchIDs returns the assembled render-batch IDs in frame order from
// the runtime's last frame output.
func assembledBatchIDs(t *testing.T, rt *Runtime) []uint64 {
	t.Helper()
	snap := rt.LastFrameStats()
	_ = snap
	// Re-run a projection through the runtime's window assembly to capture the
	// flat batch sequence; the runtime retains the last rendered frame.
	if rt.lastWindowFrames == nil {
		return nil
	}
	frame := rt.lastWindowFrames["__primary__"]
	if frame == nil {
		return nil
	}
	ids := make([]uint64, 0, len(frame.RenderBatchs))
	for _, b := range frame.RenderBatchs {
		ids = append(ids, uint64(b.ID))
	}
	return ids
}
