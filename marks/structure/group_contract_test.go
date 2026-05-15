package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestGroup_contract_composes_children_under_local_transform(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Group{
		Transform: gfx.Translation(40, 20),
		Children:  []marks.Mark{child},
	}

	desc := root.Descriptor()
	if desc.Type != marks.TypeName("structure:group") || !desc.ChildHosting || !desc.AnchorExporting {
		t.Fatalf("descriptor = %#v", desc)
	}

	out := projectStructure(t, root)
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", len(out.RenderBatchs))
	}
	if out.RenderBatchs[0].FacetID != child.Base().ID() {
		t.Fatalf("RenderBatch FacetID = %d, want child %d", out.RenderBatchs[0].FacetID, child.Base().ID())
	}
	if got := out.RenderBatchs[0].Transform; got != gfx.Translation(40, 20) {
		t.Fatalf("Transform = %#v, want local translation", got)
	}
	if hit := out.HitMap.HitTest(gfx.Point{X: 45, Y: 25}); hit == nil || hit.FacetID != child.Base().ID() {
		t.Fatalf("HitTest = %#v", hit)
	}
	if anchors := root.ExportAnchors(layout.AnchorExportContext{}); len(anchors) == 0 {
		t.Fatal("expected exported anchors from composed children")
	}
}
