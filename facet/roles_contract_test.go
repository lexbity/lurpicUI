package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestRoleCoordinateContracts_zeroValues_areLocal(t *testing.T) {
	layoutRole := &LayoutRole{}
	if layoutRole.MeasuredSize != (gfx.Size{}) {
		t.Fatalf("LayoutRole measured size = %#v, want zero", layoutRole.MeasuredSize)
	}
	if layoutRole.ArrangedBounds != (gfx.Rect{}) {
		t.Fatalf("LayoutRole arranged bounds = %#v, want zero", layoutRole.ArrangedBounds)
	}

	viewportRole := &ViewportRole{}
	if viewportRole.Transform != (gfx.Transform{}) {
		t.Fatalf("ViewportRole transform = %#v, want zero transform", viewportRole.Transform)
	}
	if viewportRole.WorldBounds != (gfx.Rect{}) {
		t.Fatalf("ViewportRole world bounds = %#v, want zero", viewportRole.WorldBounds)
	}

	layer := ProjectionLayer{}
	if layer.Bounds != (gfx.Rect{}) {
		t.Fatalf("ProjectionLayer bounds = %#v, want zero", layer.Bounds)
	}
	if layer.Transform != (gfx.Transform{}) {
		t.Fatalf("ProjectionLayer transform = %#v, want zero", layer.Transform)
	}
	if layer.ClipRect != (gfx.Rect{}) {
		t.Fatalf("ProjectionLayer clip rect = %#v, want zero", layer.ClipRect)
	}
}

func TestViewportRole_panzoom_updates_local_transform(t *testing.T) {
	role := &ViewportRole{}
	role.SetPanZoom(gfx.Point{X: 12, Y: -8}, 2)
	if role.Transform.A != 2 || role.Transform.D != 2 {
		t.Fatalf("unexpected scale in transform: %#v", role.Transform)
	}
	if role.Transform.TX != 12 || role.Transform.TY != -8 {
		t.Fatalf("unexpected translation in transform: %#v", role.Transform)
	}
}

func TestProjectionContext_resolved_layer_prefers_layer_snapshot(t *testing.T) {
	ctx := ProjectionContext{
		Bounds: gfx.RectFromXYWH(1, 2, 3, 4),
		Viewport: &ViewportRole{
			Transform: gfx.Translation(9, 10),
		},
		Layer: ProjectionLayer{
			Bounds:    gfx.RectFromXYWH(5, 6, 7, 8),
			Transform: gfx.Translation(11, 12),
			ClipRect:  gfx.RectFromXYWH(13, 14, 15, 16),
		},
	}
	if got := ctx.ResolvedLayer(); got.Bounds != (gfx.RectFromXYWH(5, 6, 7, 8)) {
		t.Fatalf("resolved bounds = %#v, want layer bounds", got.Bounds)
	}
	if got := ctx.LayerBounds(); got != (gfx.RectFromXYWH(5, 6, 7, 8)) {
		t.Fatalf("LayerBounds = %#v, want layer bounds", got)
	}
	if got := ctx.LayerTransform(); got != gfx.Translation(11, 12) {
		t.Fatalf("LayerTransform = %#v, want layer transform", got)
	}
	if got := ctx.LayerClipRect(); got != (gfx.RectFromXYWH(13, 14, 15, 16)) {
		t.Fatalf("LayerClipRect = %#v, want layer clip", got)
	}
}

func TestProjectionContext_resolved_layer_falls_back_to_local_viewport(t *testing.T) {
	ctx := ProjectionContext{
		Bounds: gfx.RectFromXYWH(1, 2, 3, 4),
		Viewport: &ViewportRole{
			Transform: gfx.Translation(9, 10),
		},
	}
	got := ctx.ResolvedLayer()
	if got.Bounds != (gfx.RectFromXYWH(1, 2, 3, 4)) {
		t.Fatalf("resolved bounds = %#v, want context bounds", got.Bounds)
	}
	if got.Transform != gfx.Translation(9, 10) {
		t.Fatalf("resolved transform = %#v, want viewport transform", got.Transform)
	}
	if got.ClipRect != (gfx.RectFromXYWH(1, 2, 3, 4)) {
		t.Fatalf("resolved clip = %#v, want bounds clip", got.ClipRect)
	}
}
