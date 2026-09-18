package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

// newMountTestTree builds a root + host + a banded layer child with a mount
// gate and a full-coverage render role, mirroring how the studio mounts the
// command palette on the Modal band.
func newMountTestTree(t *testing.T) (*Runtime, *runtimeRenderFacet, *runtimeRenderFacet, *store.ValueStore[bool]) {
	t.Helper()
	reg := testLayerRegistry(t)
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{B: 40, A: 255})
	host := newRuntimeRenderFacet("host", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{G: 80, A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		host.layout.Arrange(ctx, bounds)
	}
	layer := newRuntimeRenderFacet("layer", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{R: 200, A: 255})
	mount := store.NewValueStore(false)
	root.AddChild(host.Base())
	facet.AttachLayer(host, layer, facet.LayerAttachment{Band: facet.ZBandModal, Mount: mount})

	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}
	return rt, host, layer, mount
}

// TestRuntimeLayersMountGatedUnmountedInert pins RX-1 Q4 visibility by mount
// state: an unmounted layer produces no projection layer, no projection
// output, and no hit.
func TestRuntimeLayersMountGatedUnmountedInert(t *testing.T) {
	rt, _, layer, mount := newMountTestTree(t)
	if mount.Get() {
		t.Fatal("mount store should start false")
	}
	rt.RunOneFrame()

	if _, ok := rt.ResolveProjectionLayer(layer.Base().ID()); ok {
		t.Fatalf("unmounted layer resolved a projection layer")
	}
	// The layer facet is a tree child but is gated: no commands projected.
	for _, s := range rt.projectionSystem.OutputSnapshots() {
		if s.FacetID == layer.Base().ID() && s.CommandCount != 0 {
			t.Fatalf("unmounted layer projected %d commands", s.CommandCount)
		}
	}
	// No hit region at the center.
	if got := rt.HitTest(gfx.Point{X: 200, Y: 150}); got != 0 {
		t.Fatalf("unmounted layer hit at center: hit=%d want 0", got)
	}
}

// TestRuntimeLayersMountFlipRendersWithinOneFrame pins the mount-flip contract:
// a Mount store write re-resolves the layer within one frame so the layer
// renders (RX-1 Q4 + FR-3).
func TestRuntimeLayersMountFlipRendersWithinOneFrame(t *testing.T) {
	rt, _, layer, mount := newMountTestTree(t)
	mount.Set(true)
	rt.RunOneFrame()

	pl, ok := rt.ResolveProjectionLayer(layer.Base().ID())
	if !ok {
		t.Fatalf("mounted layer did not resolve a projection layer")
	}
	if pl.Bounds.IsEmpty() {
		t.Fatalf("mounted layer projection bounds empty: %v", pl.Bounds)
	}
	rendered := false
	for _, s := range rt.projectionSystem.OutputSnapshots() {
		if s.FacetID == layer.Base().ID() && s.CommandCount > 0 {
			rendered = true
		}
	}
	if !rendered {
		t.Fatalf("mounted layer projected no commands in the same frame")
	}
}

// TestRuntimeLayersMountUnmountClearsPixels pins the reverse flip: unmounting a
// layer after it rendered removes its projection entirely.
func TestRuntimeLayersMountUnmountClearsPixels(t *testing.T) {
	rt, _, layer, mount := newMountTestTree(t)
	mount.Set(true)
	rt.RunOneFrame()
	if _, ok := rt.ResolveProjectionLayer(layer.Base().ID()); !ok {
		t.Fatalf("mounted layer did not resolve a projection layer")
	}
	mount.Set(false)
	rt.RunOneFrame()
	if _, ok := rt.ResolveProjectionLayer(layer.Base().ID()); ok {
		t.Fatalf("unmounted layer still resolves a projection layer")
	}
	for _, s := range rt.projectionSystem.OutputSnapshots() {
		if s.FacetID == layer.Base().ID() && s.CommandCount != 0 {
			t.Fatalf("unmounted layer projected %d commands", s.CommandCount)
		}
	}
}

// TestRuntimeLayersMountFlipMarksChildDirtyWithoutAppRouting pins RX-1 FR-3 +
// the runtime's Mount-store subscription (subscribeLayerMounts): a Mount store
// write surfaces the layer child in the frame's dirty wave (DirtyLayout) with
// zero app-side routing or parallel visibility plumbing. The app's old
// SetVisible compensation is gone; the framework owns the flip.
func TestRuntimeLayersMountFlipMarksChildDirtyWithoutAppRouting(t *testing.T) {
	sink := &recordingDirtySink{}
	reg := testLayerRegistry(t)
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{B: 40, A: 255})
	host := newRuntimeRenderFacet("host", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{G: 80, A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		host.layout.Arrange(ctx, bounds)
	}
	layer := newRuntimeRenderFacet("layer", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{R: 200, A: 255})
	mount := store.NewValueStore(false)
	root.AddChild(host.Base())
	// runtimeRenderFacet uses the embedded Base (no BindImpl); bind so the
	// runtime's Mount subscription (subscribeLayerMounts, keyed on Impl())
	// registers — real marks bind in their Base.
	layer.BindImpl(layer)
	facet.AttachLayer(host, layer, facet.LayerAttachment{Band: facet.ZBandModal, Mount: mount})

	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		cfg.DiagnosticsHook = sink
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}
	rt.RunOneFrame()

	mount.Set(true)
	rt.RunOneFrame()

	snap := sink.snapshots[len(sink.snapshots)-1]
	flags, ok := snap.Dirty[layer.Base().ID()]
	if !ok {
		t.Fatal("mount flip did not surface the layer child in the dirty wave")
	}
	if flags&facet.DirtyLayout == 0 || flags&facet.DirtyProjection == 0 {
		t.Fatalf("mount flip dirty flags = %v, want DirtyLayout|DirtyProjection", flags)
	}
}

// TestRuntimeLayersUnmountedSkipsCounter pins NFR-8: the layer pass counts
// unmounted layer skips and exports them via FrameStats.LayersUnmountedSkips —
// nonzero while the layer is closed, zero once mounted (and zero in the quiet
// steady state).
func TestRuntimeLayersUnmountedSkipsCounter(t *testing.T) {
	hook := &recordingFrameStats{}
	reg := testLayerRegistry(t)
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{B: 40, A: 255})
	host := newRuntimeRenderFacet("host", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{G: 80, A: 255})
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		host.layout.Arrange(ctx, bounds)
	}
	layer := newRuntimeRenderFacet("layer", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{R: 200, A: 255})
	mount := store.NewValueStore(false)
	root.AddChild(host.Base())
	facet.AttachLayer(host, layer, facet.LayerAttachment{Band: facet.ZBandModal, Mount: mount})

	rt, err := New(func() Config {
		cfg := DefaultConfig()
		cfg.LayerRegistry = reg
		cfg.DiagnosticsHook = hook
		return cfg
	}(), nil, nil, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.window = &testWindow{width: 400, height: 300}

	rt.RunOneFrame()
	if got := hook.last().LayersUnmountedSkips; got != 1 {
		t.Fatalf("LayersUnmountedSkips = %d, want 1 (unmounted)", got)
	}

	mount.Set(true)
	rt.RunOneFrame()
	if got := hook.last().LayersUnmountedSkips; got != 0 {
		t.Fatalf("LayersUnmountedSkips = %d, want 0 (mounted)", got)
	}
}

// TestRuntimeLayersUnmountedSkipDoesNotArrangeToZero pins RX-1 Q4: the layer
// system must NOT gate an unmounted layer by arranging it to gfx.Rect{} (the
// arrange-to-zero mechanism Q4 banned for hosts). An unmounted layer's arranged
// bounds are untouched; the projection walk gates it by mount state instead.
func TestRuntimeLayersUnmountedSkipDoesNotArrangeToZero(t *testing.T) {
	rt, _, layer, mount := newMountTestTree(t)
	// A stale, previously-mounted arranged bounds: the arrange-to-zero mechanism
	// would zero it here; the mount-state skip must leave it alone (the
	// projection gate keeps it from resurrecting pixels regardless).
	layer.layout.ArrangedBounds = gfx.RectFromXYWH(10, 10, 50, 50)
	if mount.Get() {
		t.Fatal("mount store should start false")
	}
	rt.RunOneFrame()
	if got := layer.layout.ArrangedBounds; got != (gfx.RectFromXYWH(10, 10, 50, 50)) {
		t.Fatalf("unmounted layer arranged bounds changed to %v, want untouched (10,10,50,50)", got)
	}
}

// TestRuntimeLayersBandOrderingStableUnderShuffle pins the named-band paint
// order: two layers mounted in opposite insertion order still paint in band
// order (Modal above Content) regardless of the AddChild order.
func TestRuntimeLayersBandOrderingStableUnderShuffle(t *testing.T) {
	for _, reversed := range []bool{false, true} {
		reg := testLayerRegistry(t)
		root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{B: 40, A: 255})
		content := newRuntimeRenderFacet("content", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{R: 80, A: 255})
		modal := newRuntimeRenderFacet("modal", gfx.RectFromXYWH(0, 0, 400, 300), color.RGBA{R: 200, A: 255})
		if reversed {
			facet.AttachLayer(root, modal, facet.LayerAttachment{Band: facet.ZBandModal})
			facet.AttachLayer(root, content, facet.LayerAttachment{Band: facet.ZBandContent})
		} else {
			facet.AttachLayer(root, content, facet.LayerAttachment{Band: facet.ZBandContent})
			facet.AttachLayer(root, modal, facet.LayerAttachment{Band: facet.ZBandModal})
		}
		rt, err := New(func() Config {
			cfg := DefaultConfig()
			cfg.LayerRegistry = reg
			return cfg
		}(), nil, nil, &backendFixture{}, root)
		if err != nil {
			t.Fatalf("new runtime: %v", err)
		}
		rt.window = &testWindow{width: 400, height: 300}
		rt.RunOneFrame()

		contentLayer, cok := rt.ResolveProjectionLayer(content.Base().ID())
		modalLayer, mok := rt.ResolveProjectionLayer(modal.Base().ID())
		if !cok || !mok {
			t.Fatalf("reversed=%v: layers not resolved (content=%v modal=%v)", reversed, cok, mok)
		}
		if contentLayer.RenderOrder >= modalLayer.RenderOrder {
			t.Fatalf("reversed=%v: content render order %d not below modal %d", reversed, contentLayer.RenderOrder, modalLayer.RenderOrder)
		}
	}
}

// TestAttachLayerPanicsOnUnknownBand pins the fail-fast attach contract: a
// LayerAttachment with an undeclared band panics at attach time.
func TestAttachLayerPanicsOnUnknownBand(t *testing.T) {
	parent := newRuntimeRenderFacet("parent", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	expectPanicContains(t, "valid ZBand", func() {
		facet.AttachLayer(parent, child, facet.LayerAttachment{Band: facet.ZBand(99)})
	})
}
