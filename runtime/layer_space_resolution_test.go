package runtime

import (
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

type coordinateRootFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	viewport facet.ViewportRole
	specs    []layout.LayerSpec
}

func (r *coordinateRootFacet) Base() *facet.Facet {
	r.Facet.BindImpl(r)
	return &r.Facet
}

func (r *coordinateRootFacet) OnLayerSpecs() []layout.LayerSpec {
	return r.specs
}

func newCoordinateRootFacet(specs []layout.LayerSpec, transform gfx.Transform, worldBounds gfx.Rect) *coordinateRootFacet {
	root := &coordinateRootFacet{
		Facet: facet.NewFacet(),
		specs: specs,
	}
	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: 400, H: 300}
	}
	root.layout.OnArrange = func(bounds gfx.Rect) {
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
	h.Facet.BindImpl(h)
	return &h.Facet
}

func newCoordinateHitFacet(size gfx.Size) *coordinateHitFacet {
	child := &coordinateHitFacet{
		Facet: facet.NewFacet(),
		size:  size,
	}
	child.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return child.size
	}
	child.layout.OnArrange = func(bounds gfx.Rect) {
		child.layout.ArrangedBounds = bounds
	}
	child.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
	}
	child.AddRole(&child.layout)
	child.AddRole(&child.hit)
	return child
}

func TestLayerResolution_usesViewportTransformAndResolvedSnapshot(t *testing.T) {
	root := newCoordinateRootFacet([]layout.LayerSpec{
		{
			ID:          1,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordViewport,
			CoordLimits: layout.CoordLimits{
				Bounds:        gfx.RectFromXYWH(0, 0, 200, 200),
				AllowOverflow: true,
			},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 7,
			ClipPolicy:  layout.ClipToViewport,
		},
		{
			ID:          2,
			Placement:   layout.PlacementFree,
			Measurement: layout.MeasureNonStructural,
			CoordSpace:  layout.CoordScreenAligned,
			CoordLimits: layout.CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 80, 40)},
			HitPolicy:   layout.HitNormal,
			RenderOrder: 8,
			ClipPolicy:  layout.ClipNone,
		},
	}, gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 200, 200))

	childA := newCoordinateHitFacet(gfx.Size{W: 300, H: 300})
	childB := newRuntimeRenderFacet("screen-aligned", gfx.RectFromXYWH(0, 0, 40, 20), color.RGBA{A: 255})

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, childA, layout.ChildAttachment{LayerID: 1})
	rt.AddFacet(root, childB, layout.ChildAttachment{LayerID: 2})
	rt.RunOneFrame()

	layerA, ok := rt.ResolveProjectionLayer(childA.Base().ID())
	if !ok {
		t.Fatal("missing resolved layer for child A")
	}
	if layerA.Bounds != (gfx.RectFromXYWH(0, 0, 300, 300)) {
		t.Fatalf("layer A bounds = %#v, want child bounds", layerA.Bounds)
	}
	if layerA.Transform != gfx.Translation(15, 25) {
		t.Fatalf("layer A transform = %#v, want translated viewport", layerA.Transform)
	}
	if layerA.ClipRect != (gfx.RectFromXYWH(15, 25, 200, 200)) {
		t.Fatalf("layer A clip rect = %#v, want translated viewport clip", layerA.ClipRect)
	}
	if layerA.RenderOrder != 7 {
		t.Fatalf("layer A render order = %d, want 7", layerA.RenderOrder)
	}
	if layerA.HitPolicy != uint8(layout.HitNormal) {
		t.Fatalf("layer A hit policy = %d, want HitNormal", layerA.HitPolicy)
	}

	layerB, ok := rt.ResolveProjectionLayer(childB.Base().ID())
	if !ok {
		t.Fatal("missing resolved layer for child B")
	}
	if layerB.Bounds != (gfx.RectFromXYWH(0, 0, 40, 20)) {
		t.Fatalf("layer B bounds = %#v, want child bounds", layerB.Bounds)
	}
	if layerB.Transform != gfx.Identity() {
		t.Fatalf("layer B transform = %#v, want identity for screen-aligned layer", layerB.Transform)
	}
	if layerB.RenderOrder != 8 {
		t.Fatalf("layer B render order = %d, want 8", layerB.RenderOrder)
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
	if snaps[0].ChildCount != 1 || snaps[1].ChildCount != 1 {
		t.Fatalf("snapshot child counts = %d, %d, want 1, 1", snaps[0].ChildCount, snaps[1].ChildCount)
	}
}
