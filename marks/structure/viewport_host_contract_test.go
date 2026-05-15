package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestViewportHost_contract_declares_nested_viewport_boundary(t *testing.T) {
	root := &ViewportHost{
		Viewport: ViewportModel{
			Bounds:    gfx.RectFromXYWH(0, 0, 100, 50),
			Transform: gfx.Translation(20, 30),
		},
	}

	desc := root.Descriptor()
	if desc.Type != marks.TypeName("structure:viewporthost") || !desc.ChildHosting || !desc.AnchorExporting {
		t.Fatalf("descriptor = %#v", desc)
	}

	specs := root.OnLayerSpecs()
	if len(specs) != 1 {
		t.Fatalf("LayerSpecs = %d, want 1", len(specs))
	}
	if specs[0].CoordSpace != layout.CoordViewport {
		t.Fatalf("CoordSpace = %v, want viewport", specs[0].CoordSpace)
	}
	if specs[0].ClipPolicy != layout.ClipToViewport {
		t.Fatalf("ClipPolicy = %v, want viewport clip", specs[0].ClipPolicy)
	}
	if got := root.Base().ViewportRole().Transform; got != gfx.Translation(20, 30) {
		t.Fatalf("Viewport transform = %#v, want authored transform", got)
	}
	anchors := root.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["bounds-center"]
	if !ok {
		t.Fatal("missing bounds-center anchor")
	}
	if got != (gfx.Point{X: 70, Y: 55}) {
		t.Fatalf("anchor = %#v, want translated center", got)
	}
}
