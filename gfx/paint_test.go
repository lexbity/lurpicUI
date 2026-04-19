package gfx

import "testing"

func TestColorFromRGBA8_roundtrip(t *testing.T) {
	got := ColorFromRGBA8(255, 128, 0, 255)
	r, g, b, a := got.ToRGBA8()
	if r != 255 || g != 128 || b != 0 || a != 255 {
		t.Fatalf("roundtrip mismatch: got %d %d %d %d", r, g, b, a)
	}
}

func TestColorFromHex_opaque(t *testing.T) {
	got := ColorFromHex(0xFF8000FF)
	if !almostEqual(got.R, 1.0) || !almostEqual(got.G, 128.0/255.0) || !almostEqual(got.B, 0) || !almostEqual(got.A, 1.0) {
		t.Fatalf("unexpected color from hex: %+v", got)
	}
}

func TestColorWithAlpha_preserves_rgb(t *testing.T) {
	orig := Color{R: 0.25, G: 0.5, B: 0.75, A: 1}
	got := orig.WithAlpha(0.2)
	if got.R != orig.R || got.G != orig.G || got.B != orig.B {
		t.Fatalf("expected rgb to be preserved: got %+v want %+v", got, orig)
	}
	if got.A != 0.2 {
		t.Fatalf("expected alpha to be updated, got %v", got.A)
	}
}

func TestColorPremultiply_fully_transparent(t *testing.T) {
	got := (Color{R: 1, G: 0, B: 0, A: 0}).Premultiply()
	if got.R != 0 || got.G != 0 || got.B != 0 || got.A != 0 {
		t.Fatalf("expected fully transparent color to premultiply to zero, got %+v", got)
	}
}

func TestColorPremultiply_opaque_unchanged(t *testing.T) {
	orig := Color{R: 0.25, G: 0.5, B: 0.75, A: 1}
	got := orig.Premultiply()
	if got != orig {
		t.Fatalf("expected opaque color to remain unchanged, got %+v want %+v", got, orig)
	}
}

func TestSolidBrush_kind(t *testing.T) {
	if got := SolidBrush(Color{}).Kind; got != BrushSolid {
		t.Fatalf("expected solid brush kind, got %v", got)
	}
}

func TestLinearGradientBrush_stops_preserved(t *testing.T) {
	stops := []GradientStop{
		{Offset: 0, Color: Color{A: 1}},
		{Offset: 1, Color: Color{R: 1, A: 1}},
	}
	got := LinearGradientBrush(Point{1, 2}, Point{3, 4}, stops)
	if got.Kind != BrushLinearGradient {
		t.Fatalf("expected linear gradient brush kind, got %v", got.Kind)
	}
	if got.GradientStart != (Point{1, 2}) || got.GradientEnd != (Point{3, 4}) {
		t.Fatalf("unexpected gradient endpoints: %+v", got)
	}
	if len(got.GradientStops) != len(stops) {
		t.Fatalf("expected %d stops, got %d", len(stops), len(got.GradientStops))
	}
	for i := range stops {
		if got.GradientStops[i] != stops[i] {
			t.Fatalf("stop %d mismatch: got %+v want %+v", i, got.GradientStops[i], stops[i])
		}
	}
}

func TestDefaultStroke_values(t *testing.T) {
	got := DefaultStroke(3.5)
	if got.Width != 3.5 {
		t.Fatalf("expected width 3.5, got %v", got.Width)
	}
	if got.Cap != LineCapButt {
		t.Fatalf("expected butt cap, got %v", got.Cap)
	}
	if got.Join != LineJoinMiter {
		t.Fatalf("expected miter join, got %v", got.Join)
	}
	if got.MiterLimit != 10 {
		t.Fatalf("expected miter limit 10, got %v", got.MiterLimit)
	}
	if len(got.Dash) != 0 {
		t.Fatalf("expected empty dash, got %#v", got.Dash)
	}
}
