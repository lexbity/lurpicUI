package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
)

func TestViewportHost_world_to_screen_transform_used(t *testing.T) {
	child := &basic.Rect{
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 10, H: 10},
		Style:  basic.PrimitiveStyleProps{Visible: true, Opacity: 1},
	}
	root := &ViewportHost{
		Viewport: ViewportModel{
			Bounds:    gfx.RectFromXYWH(0, 0, 100, 100),
			Transform: gfx.Translation(20, 30),
		},
		Children: []marks.Mark{child},
	}
	specs := root.OnLayerSpecs()
	if len(specs) != 1 {
		t.Fatalf("LayerSpecs = %d, want 1", len(specs))
	}
	if specs[0].CoordSpace != layout.CoordViewport || specs[0].ClipPolicy != layout.ClipToViewport {
		t.Fatalf("LayerSpec = %#v, want viewport projection", specs[0])
	}
	if got := root.Base().ViewportRole().Transform; got != gfx.Translation(20, 30) {
		t.Fatalf("Viewport transform = %#v, want translation", got)
	}
}

func TestViewportHost_anchor_exports_view_bounds(t *testing.T) {
	root := &ViewportHost{
		Viewport: ViewportModel{
			Bounds:    gfx.RectFromXYWH(0, 0, 100, 50),
			Transform: gfx.Translation(10, 20),
		},
	}
	anchors := root.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["bounds-center"]
	if !ok {
		t.Fatal("missing bounds-center anchor")
	}
	if got != (gfx.Point{X: 60, Y: 45}) {
		t.Fatalf("anchor = %#v, want translated center", got)
	}
}
