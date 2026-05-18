package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestLayerResolution_hitTraversal_respectsResolvedClip(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 200, 200))

	child := newCoordinateHitFacet(gfx.Size{W: 300, H: 300})

	rt := mustRuntimeWithBackend(t, root, &backendFixture{})
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, child, facet.Attachment{LayerID: facet.LayerID(1)})
	rt.RunOneFrame()

	if got := rt.HitTest(gfx.Point{X: 20, Y: 30}); got != child.Base().ID() {
		t.Fatalf("HitTest inside clip = %d, want %d", got, child.Base().ID())
	}
	if got := rt.HitTest(gfx.Point{X: 240, Y: 250}); got != 0 {
		t.Fatalf("HitTest outside clip = %d, want 0", got)
	}
}

func TestLayerResolution_hitTraversal_respectsGroupClip(t *testing.T) {
	root := newCoordinateRootFacet(gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 400, 300))
	root.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 200, H: 200}}
	}
	root.layout.Parent = facet.GroupParentContract{Clipping: facet.GroupClipBounds}
	child := newCoordinateHitFacet(gfx.Size{W: 300, H: 300})

	rt := mustRuntimeWithBackend(t, root, &backendFixture{})
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, child, facet.Attachment{LayerID: facet.LayerID(1)})
	rt.RunOneFrame()

	if got := rt.HitTest(gfx.Point{X: 240, Y: 250}); got != 0 {
		t.Fatalf("HitTest outside group clip = %d, want 0", got)
	}
}
