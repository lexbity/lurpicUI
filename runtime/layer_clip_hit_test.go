package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestLayerResolution_hitTraversal_respectsResolvedClip(t *testing.T) {
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
			RenderOrder: 1,
			ClipPolicy:  layout.ClipToViewport,
		},
	}, gfx.Translation(15, 25), gfx.RectFromXYWH(0, 0, 200, 200))

	child := newCoordinateHitFacet(gfx.Size{W: 300, H: 300})

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.window = &testWindow{width: 400, height: 300}
	rt.AddFacet(root, child, layout.ChildAttachment{LayerID: 1})
	rt.RunOneFrame()

	if got := rt.HitTest(gfx.Point{X: 20, Y: 30}); got != child.Base().ID() {
		t.Fatalf("HitTest inside clip = %d, want %d", got, child.Base().ID())
	}
	if got := rt.HitTest(gfx.Point{X: 240, Y: 250}); got != 0 {
		t.Fatalf("HitTest outside clip = %d, want 0", got)
	}
}
