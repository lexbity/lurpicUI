package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestGroup_children_inherit_transform(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Group{
		Transform: gfx.Translation(40, 20),
		Children:  []marks.Mark{child},
	}
	out := projectStructure(t, root)
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", len(out.RenderBatchs))
	}
	if got := out.RenderBatchs[0].Transform; got != gfx.Translation(40, 20) {
		t.Fatalf("Transform = %#v, want translation", got)
	}
	if hit := out.HitMap.HitTest(gfx.Point{X: 45, Y: 25}); hit == nil || hit.FacetID != child.Base().ID() {
		t.Fatalf("HitTest = %#v", hit)
	}
}

func TestGroup_bounds_anchor_is_union_of_children(t *testing.T) {
	childA := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	childB := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	childB.viewport.Transform = gfx.Translation(50, 0)
	childB.AddRole(&childB.viewport)

	root := &Group{
		Children: []marks.Mark{childA, childB},
	}
	anchors := root.ExportAnchors(layout.AnchorExportContext{})
	got, ok := anchors["bounds-center"]
	if !ok {
		t.Fatal("missing bounds-center anchor")
	}
	want := gfx.Point{X: 30, Y: 5}
	if got != want {
		t.Fatalf("bounds-center = %#v, want %#v", got, want)
	}
}

func TestGroup_no_visual_output_of_own(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Group{Children: []marks.Mark{child}}
	out := projectStructure(t, root)
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", len(out.RenderBatchs))
	}
	if out.RenderBatchs[0].FacetID != child.Base().ID() {
		t.Fatalf("RenderBatch FacetID = %d, want child %d", out.RenderBatchs[0].FacetID, child.Base().ID())
	}
}
