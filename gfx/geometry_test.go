package gfx

import (
	"math"
	"testing"
)

func almostEqual(a, b float32) bool {
	const eps = 1e-5
	return float32(math.Abs(float64(a-b))) <= eps
}

func almostEqualPoint(a, b Point) bool {
	return almostEqual(a.X, b.X) && almostEqual(a.Y, b.Y)
}

func almostEqualRect(a, b Rect) bool {
	return almostEqualPoint(a.Min, b.Min) && almostEqualPoint(a.Max, b.Max)
}

func TestRectContains_point_on_boundary(t *testing.T) {
	r := RectFromXYWH(10, 20, 30, 40)
	points := []Point{
		{10, 20},
		{40, 20},
		{10, 60},
		{40, 60},
	}
	for _, p := range points {
		if !r.Contains(p) {
			t.Fatalf("expected point %+v to be contained in %+v", p, r)
		}
	}
}

func TestRectIntersects_touching_edge(t *testing.T) {
	a := RectFromXYWH(0, 0, 10, 10)
	b := RectFromXYWH(10, 2, 5, 4)
	if !a.Intersects(b) {
		t.Fatalf("expected %+v and %+v to intersect on touching edge", a, b)
	}
}

func TestRectUnion_one_empty(t *testing.T) {
	empty := Rect{}
	nonEmpty := RectFromXYWH(1, 2, 3, 4)
	if got := empty.Union(nonEmpty); !almostEqualRect(got, nonEmpty) {
		t.Fatalf("expected union with empty rect to return non-empty operand, got %+v want %+v", got, nonEmpty)
	}
	if got := nonEmpty.Union(empty); !almostEqualRect(got, nonEmpty) {
		t.Fatalf("expected union with empty rect to return non-empty operand, got %+v want %+v", got, nonEmpty)
	}
}

func TestRectInset_negative_grows(t *testing.T) {
	r := RectFromXYWH(10, 20, 30, 40)
	got := r.Inset(-5, -5)
	want := RectFromXYWH(5, 15, 40, 50)
	if !almostEqualRect(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestTransformIdentity_noop(t *testing.T) {
	p := Point{3, 4}
	r := RectFromXYWH(-2, -1, 5, 6)
	if got := Identity().TransformPoint(p); !almostEqualPoint(got, p) {
		t.Fatalf("identity point transform changed point: got %+v want %+v", got, p)
	}
	if got := Identity().TransformRect(r); !almostEqualRect(got, r) {
		t.Fatalf("identity rect transform changed rect: got %+v want %+v", got, r)
	}
}

func TestTransformMultiply_associativity(t *testing.T) {
	combos := []Transform{
		Identity(),
		Translation(3, -2),
		Scale(2, 3),
		Rotation(float32(math.Pi / 6)),
		Translation(-5, 7).Multiply(Scale(0.5, 1.5)),
	}

	for i := range combos {
		for j := range combos {
			for k := range combos {
				left := combos[i].Multiply(combos[j]).Multiply(combos[k])
				right := combos[i].Multiply(combos[j].Multiply(combos[k]))
				if !almostEqualTransform(left, right) {
					t.Fatalf("associativity failed for i=%d j=%d k=%d: left=%+v right=%+v", i, j, k, left, right)
				}
			}
		}
	}
}

func almostEqualTransform(a, b Transform) bool {
	return almostEqual(a.A, b.A) &&
		almostEqual(a.B, b.B) &&
		almostEqual(a.C, b.C) &&
		almostEqual(a.D, b.D) &&
		almostEqual(a.TX, b.TX) &&
		almostEqual(a.TY, b.TY)
}

func TestTransformInverse_roundtrip(t *testing.T) {
	tform := Translation(12, -7).Multiply(Rotation(float32(math.Pi / 8))).Multiply(Scale(2, 0.75))
	inv, ok := tform.Inverse()
	if !ok {
		t.Fatal("expected transform to be invertible")
	}

	original := Point{1.25, -3.5}
	roundtrip := inv.TransformPoint(tform.TransformPoint(original))
	if !almostEqualPoint(roundtrip, original) {
		t.Fatalf("roundtrip failed: got %+v want %+v", roundtrip, original)
	}
}

func TestTransformInverse_degenerate_zero_scale(t *testing.T) {
	if inv, ok := Scale(0, 1).Inverse(); ok {
		t.Fatalf("expected zero-scale transform to be non-invertible, got %+v", inv)
	}
}

func TestTransformRect_preserves_area_under_scale(t *testing.T) {
	got := Scale(2, 3).TransformRect(RectFromXYWH(0, 0, 1, 1))
	want := RectFromXYWH(0, 0, 2, 3)
	if !almostEqualRect(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}

func TestTransformRect_rotation_90deg(t *testing.T) {
	got := Rotation(float32(math.Pi / 2)).TransformRect(RectFromXYWH(0, 0, 2, 1))
	want := Rect{
		Min: Point{-1, 0},
		Max: Point{0, 2},
	}
	if !almostEqualRect(got, want) {
		t.Fatalf("expected %+v, got %+v", want, got)
	}
}
