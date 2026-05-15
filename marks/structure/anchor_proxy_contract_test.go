package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestAnchorProxy_contract_forwards_and_offsets_anchors(t *testing.T) {
	src := &testShapeFacet{id: "source", bounds: gfx.RectFromXYWH(0, 0, 20, 10)}
	proxy := &AnchorProxy{
		Source:    AnchorSourceRef{MarkID: "source", Anchor: "bounds-center"},
		RenameMap: map[string]string{"bounds-center": "mid"},
		Offset:    gfx.Point{X: 2, Y: 3},
		Children:  []marks.Mark{src},
	}

	desc := proxy.Descriptor()
	if desc.Type != marks.TypeName("structure:anchorproxy") || !desc.ChildHosting || !desc.AnchorExporting {
		t.Fatalf("descriptor = %#v", desc)
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
