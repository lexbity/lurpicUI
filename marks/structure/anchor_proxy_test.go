package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestAnchorProxy_forwards_anchor(t *testing.T) {
	src := &testShapeFacet{id: "source", bounds: gfx.RectFromXYWH(0, 0, 20, 10)}
	proxy := &AnchorProxy{
		Source:   AnchorSourceRef{MarkID: "source", Anchor: "bounds-center"},
		Children: []marks.Mark{src},
	}
	anchors := proxy.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["bounds-center"]
	if !ok {
		t.Fatal("missing forwarded anchor")
	}
	if got != (gfx.Point{X: 10, Y: 5}) {
		t.Fatalf("anchor = %#v, want center of source", got)
	}
}

func TestAnchorProxy_renames_anchor(t *testing.T) {
	src := &testShapeFacet{id: "source", bounds: gfx.RectFromXYWH(0, 0, 20, 10)}
	proxy := &AnchorProxy{
		Source:    AnchorSourceRef{MarkID: "source", Anchor: "bounds-center"},
		RenameMap: map[string]string{"bounds-center": "mid"},
		Offset:    gfx.Point{X: 2, Y: 3},
		Children:  []marks.Mark{src},
	}
	anchors := proxy.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["mid"]
	if !ok {
		t.Fatal("missing renamed anchor")
	}
	if got != (gfx.Point{X: 12, Y: 8}) {
		t.Fatalf("anchor = %#v, want offset center", got)
	}
}

func TestAnchorProxy_missing_source_safe(t *testing.T) {
	proxy := &AnchorProxy{
		Source: AnchorSourceRef{MarkID: "missing", Anchor: "center"},
	}
	if anchors := proxy.ExportAnchors(layout.AnchorExportContext{}); len(anchors) != 0 {
		t.Fatalf("anchors = %#v, want none", anchors)
	}
}
