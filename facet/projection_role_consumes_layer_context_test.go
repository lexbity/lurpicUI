package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestProjectionRole_consumes_resolved_layer_context(t *testing.T) {
	var got ProjectionContext
	role := &ProjectionRole{
		OnProject: func(ctx ProjectionContext) *gfx.CommandList {
			got = ctx
			return &gfx.CommandList{}
		},
	}

	layer := ProjectionLayer{
		Bounds:    gfx.RectFromXYWH(4, 5, 6, 7),
		Transform: gfx.Translation(9, 11),
		ClipRect:  gfx.RectFromXYWH(4, 5, 6, 7),
	}
	ctx := ProjectionContext{
		Bounds: gfx.RectFromXYWH(1, 2, 3, 4),
		Layer:  layer,
	}

	if cmds := role.Project(ctx); cmds == nil {
		t.Fatal("expected command list")
	}
	if got.Layer != layer {
		t.Fatalf("projection layer = %#v, want resolved layer", got.Layer)
	}
	if got.LayerBounds() != layer.Bounds {
		t.Fatalf("LayerBounds = %#v, want %#v", got.LayerBounds(), layer.Bounds)
	}
	if got.LayerTransform() != layer.Transform {
		t.Fatalf("LayerTransform = %#v, want %#v", got.LayerTransform(), layer.Transform)
	}
	if got.LayerClipRect() != layer.ClipRect {
		t.Fatalf("LayerClipRect = %#v, want %#v", got.LayerClipRect(), layer.ClipRect)
	}
}
