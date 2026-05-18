package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func testRegistryWithCustomLayer(t *testing.T, name layout.LayerName, order layout.LayerOrder, hitPolicy layout.LayerHitPolicy) *layout.LayerRegistry {
	t.Helper()
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:      name,
		Order:     order,
		HitPolicy: hitPolicy,
	}); err != nil {
		t.Fatalf("register custom layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return reg
}

func TestLayerResolution_hitTraversal_respectsRegistryOrderAndPassThrough(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Identity(), gfx.RectFromXYWH(0, 0, 400, 300))
	base := newCoordinateHitFacet(gfx.Size{W: 100, H: 100})
	custom := newCoordinateHitFacet(gfx.Size{W: 100, H: 100})

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	reg := testRegistryWithCustomLayer(t, layout.LayerName("app.tooltip"), 2500, layout.HitPassThrough)
	rt.config.LayerRegistry = reg
	rt.layerRegistry = reg
	customLayer, ok := reg.LookupName("app.tooltip")
	if !ok {
		t.Fatal("missing custom layer")
	}
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	rt.AddFacet(root, custom, facet.Attachment{LayerID: facet.LayerID(customLayer.ID)})
	rt.window = &testWindow{width: 400, height: 300}
	rt.RunOneFrame()

	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != base.Base().ID() {
		t.Fatalf("HitTest with pass-through top layer = %d, want %d", got, base.Base().ID())
	}
}

func TestLayerResolution_hitTraversal_blocksLowerLayersWhenClipped(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Identity(), gfx.RectFromXYWH(0, 0, 400, 300))
	base := newCoordinateHitFacet(gfx.Size{W: 100, H: 100})
	blocker := newCoordinateHitFacet(gfx.Size{W: 100, H: 100})

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	reg := testRegistryWithCustomLayer(t, layout.LayerName("app.modal"), 2500, layout.HitBlockBelow)
	rt.config.LayerRegistry = reg
	rt.layerRegistry = reg
	blockerLayer, ok := reg.LookupName("app.modal")
	if !ok {
		t.Fatal("missing custom layer")
	}
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	rt.AddFacet(root, blocker, facet.Attachment{LayerID: facet.LayerID(blockerLayer.ID)})
	rt.window = &testWindow{width: 400, height: 300}
	rt.RunOneFrame()

	rt.projectionLayers[blocker.Base().ID()] = facet.ProjectionLayer{
		LayerID:       facet.LayerID(blockerLayer.ID),
		Bounds:        gfx.RectFromXYWH(0, 0, 100, 100),
		Transform:     gfx.Identity(),
		ClipRect:      gfx.RectFromXYWH(0, 0, 5, 5),
		CoordSpace:    uint8(layout.CoordViewport),
		RenderOrder:   int(blockerLayer.Order),
		HitPolicy:     uint8(layout.HitBlockBelow),
		ClipPolicy:    facet.ClipPolicy(layout.ClipToParent),
		RecipeVersion: 1,
	}

	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != 0 {
		t.Fatalf("HitTest with clipped blocker = %d, want 0", got)
	}
}
