package structure

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestTransform_rotates_descendants(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Transform{
		Matrix:   gfx.Rotation(float32(math.Pi / 2)),
		Children: []marks.Mark{child},
	}
	out := projectStructure(t, root)
	if len(out.RenderBatchs) != 1 {
		t.Fatalf("RenderBatchs = %d, want 1", len(out.RenderBatchs))
	}
	if !almostEqualTransform(out.RenderBatchs[0].Transform, gfx.Rotation(float32(math.Pi/2))) {
		t.Fatalf("Transform = %#v, want rotation", out.RenderBatchs[0].Transform)
	}
}

func TestTransform_inverse_hit_mapping(t *testing.T) {
	child := newTestShapeFacet(gfx.RectFromXYWH(0, 0, 10, 10))
	root := &Transform{
		Matrix:   gfx.Rotation(float32(math.Pi / 2)),
		Children: []marks.Mark{child},
	}
	out := projectStructure(t, root)
	hit := out.HitMap.HitTest(gfx.Point{X: -5, Y: 5})
	if hit == nil || hit.FacetID != child.Base().ID() {
		t.Fatalf("HitTest = %#v", hit)
	}
}

func almostEqualTransform(a, b gfx.Transform) bool {
	const eps = 1e-4
	return abs32(a.A-b.A) < eps &&
		abs32(a.B-b.B) < eps &&
		abs32(a.C-b.C) < eps &&
		abs32(a.D-b.D) < eps &&
		abs32(a.TX-b.TX) < eps &&
		abs32(a.TY-b.TY) < eps
}

func abs32(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}
