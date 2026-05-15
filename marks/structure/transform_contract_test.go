package structure

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestTransform_contract_applies_local_matrix_only(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Transform{
		Matrix:   gfx.Rotation(float32(math.Pi / 2)),
		Children: []marks.Mark{child},
	}

	desc := root.Descriptor()
	if desc.Type != marks.TypeName("structure:transform") || !desc.ChildHosting || !desc.AnchorExporting {
		t.Fatalf("descriptor = %#v", desc)
	}

	out := projectStructure(t, root)
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", len(out.RenderBatchs))
	}
	if !almostEqualTransform(out.RenderBatchs[0].Transform, gfx.Rotation(float32(math.Pi/2))) {
		t.Fatalf("Transform = %#v, want authored rotation", out.RenderBatchs[0].Transform)
	}
	hit := out.HitMap.HitTest(gfx.Point{X: -5, Y: 5})
	if hit == nil || hit.FacetID != child.Base().ID() {
		t.Fatalf("HitTest = %#v", hit)
	}
}
