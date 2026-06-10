package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func testRegistryWithOrders(t *testing.T, orders ...layout.LayerOrder) *layout.LayerRegistry {
	t.Helper()
	b := layout.NewLayerRegistryBuilder()
	for i, order := range orders {
		if _, err := b.RegisterLayer(layout.LayerRegistration{
			Name:  layout.LayerName(string(rune('a' + i))),
			Order: order,
			CoordSpace: func() layout.CoordSpace {
				if i == 1 {
					return layout.CoordScreenAligned
				}
				return layout.CoordViewport
			}(),
		}); err != nil {
			t.Fatalf("register layer %d: %v", i, err)
		}
	}
	r, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return r
}

type coordinateRootFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	viewport facet.ViewportRole
}

func (r *coordinateRootFacet) Base() *facet.Facet {
	r.BindImpl(r)
	return &r.Facet
}

func newCoordinateRootFacet(transform gfx.Transform, worldBounds gfx.Rect) *coordinateRootFacet {
	root := &coordinateRootFacet{
		Facet: facet.NewFacet(),
	}
	root.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 400, H: 300}}
	}
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
	}
	root.viewport.Transform = transform
	root.viewport.WorldBounds = worldBounds
	root.AddRole(&root.layout)
	root.AddRole(&root.viewport)
	return root
}

type coordinateHitFacet struct {
	facet.Facet
	layout facet.LayoutRole
	hit    facet.HitRole
	size   gfx.Size
}

func (h *coordinateHitFacet) Base() *facet.Facet {
	h.BindImpl(h)
	return &h.Facet
}

func newCoordinateHitFacet(size gfx.Size) *coordinateHitFacet {
	child := &coordinateHitFacet{
		Facet: facet.NewFacet(),
		size:  size,
	}
	child.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: child.size}
	}
	child.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		child.layout.ArrangedBounds = bounds
	}
	child.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	child.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
	}
	child.AddRole(&child.layout)
	child.AddRole(&child.hit)
	return child
}

func TestLayerResolution_usesViewportTransformAndResolvedSnapshot(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 200, 200))

	childA := newCoordinateHitFacet(gfx.Size{W: 300, H: 300})
	childB := newRuntimeRenderFacet("screen-aligned", gfx.RectFromXYWH(0, 0, 40, 20), color.RGBA{A: 255})

	rt := mustRuntimeWithBackend(t, root, &backendFixture{})
	reg := testRegistryWithOrders(t, 7, 8)
	rt.config.LayerRegistry = reg
	rt.layerRegistry = reg
	rt.window = &testWindow{width: 400, height: 300}
	regLayerA, ok := reg.LookupName("a")
	if !ok {
		t.Fatal("missing registry layer a")
	}
	regLayerB, ok := reg.LookupName("b")
	if !ok {
		t.Fatal("missing registry layer b")
	}
	rt.AddFacet(root, childA, facet.Attachment{LayerID: facet.LayerID(regLayerA.ID)})
	rt.AddFacet(root, childB, facet.Attachment{LayerID: facet.LayerID(regLayerB.ID)})
	rt.RunOneFrame()

	projA, ok := rt.ResolveProjectionLayer(childA.Base().ID())
	if !ok {
		t.Fatal("missing resolved layer for child A")
	}
	if projA.Bounds != (gfx.RectFromXYWH(0, 0, 80, 60)) {
		t.Fatalf("layer A bounds = %#v, want default grid cell bounds", projA.Bounds)
	}
	if projA.Transform != gfx.Translation(15, 25) {
		t.Fatalf("layer A transform = %#v, want translated viewport", projA.Transform)
	}
	if projA.ClipRect != (gfx.RectFromXYWH(15, 25, 200, 200)) {
		t.Fatalf("layer A clip rect = %#v, want translated viewport clip", projA.ClipRect)
	}
	if projA.RenderOrder != 7 {
		t.Fatalf("layer A render order = %d, want 7", projA.RenderOrder)
	}
	if projA.HitPolicy != uint8(layout.HitNormal) {
		t.Fatalf("layer A hit policy = %d, want HitNormal", projA.HitPolicy)
	}

	projB, ok := rt.ResolveProjectionLayer(childB.Base().ID())
	if !ok {
		t.Fatal("missing resolved layer for child B")
	}
	if projB.Bounds != (gfx.RectFromXYWH(0, 0, 80, 60)) {
		t.Fatalf("layer B bounds = %#v, want default grid cell bounds", projB.Bounds)
	}
	if projB.Transform != gfx.Identity() {
		t.Fatalf("layer B transform = %#v, want identity for screen-aligned layer", projB.Transform)
	}
	if projB.RenderOrder != 8 {
		t.Fatalf("layer B render order = %d, want 8", projB.RenderOrder)
	}

	snaps := rt.LayerSnapshots(root.Base().ID())
	if len(snaps) != 2 {
		t.Fatalf("LayerSnapshots = %d, want 2", len(snaps))
	}
	if snaps[0].CoordSpace != layout.CoordViewport {
		t.Fatalf("snapshot[0] coord space = %v, want viewport", snaps[0].CoordSpace)
	}
	if snaps[1].CoordSpace != layout.CoordScreenAligned {
		t.Fatalf("snapshot[1] coord space = %v, want screen aligned", snaps[1].CoordSpace)
	}
	if snaps[0].ChildCount != 0 || snaps[1].ChildCount != 0 {
		t.Fatalf("snapshot child counts = %d, %d, want 0, 0", snaps[0].ChildCount, snaps[1].ChildCount)
	}
	if snaps[0].LayerName == "" || snaps[0].WindowBinding == "" || !snaps[0].Materialized {
		t.Fatalf("snapshot[0] metadata = %#v", snaps[0])
	}
	if snaps[0].CommandCount == 0 && snaps[0].HitRegionCount == 0 {
		t.Fatalf("snapshot[0] should include command or hit metadata = %#v", snaps[0])
	}
}
