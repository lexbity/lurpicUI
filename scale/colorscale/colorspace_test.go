package colorscale

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

const eps = 1e-6

func almostEqual(a, b float64) bool {
	return math.Abs(a-b) <= eps
}

// --- sRGB ↔ Linear ---

func TestSRGBToLinear_black(t *testing.T) {
	if got := SRGBToLinear(0); got != 0 {
		t.Fatalf("SRGBToLinear(0) = %f, want 0", got)
	}
}

func TestSRGBToLinear_white(t *testing.T) {
	if got := SRGBToLinear(1); !almostEqual(got, 1) {
		t.Fatalf("SRGBToLinear(1) = %g, want 1", got)
	}
}

func TestSRGBToLinear_midpoint(t *testing.T) {
	// 0.5 sRGB → ~0.214 linear
	got := SRGBToLinear(0.5)
	if got <= 0.2 || got >= 0.22 {
		t.Fatalf("SRGBToLinear(0.5) = %f, want ~0.214", got)
	}
}

func TestLinearToSRGB_black(t *testing.T) {
	if got := LinearToSRGB(0); got != 0 {
		t.Fatalf("LinearToSRGB(0) = %f, want 0", got)
	}
}

func TestLinearToSRGB_white(t *testing.T) {
	if got := LinearToSRGB(1); !almostEqual(got, 1) {
		t.Fatalf("LinearToSRGB(1) = %g, want 1", got)
	}
}

func TestSRGB_linear_round_trip(t *testing.T) {
	for _, v := range []float64{0, 0.1, 0.25, 0.5, 0.75, 0.9, 0.99, 1} {
		got := SRGBToLinear(LinearToSRGB(v))
		if math.Abs(got-v) > 1e-6 {
			t.Errorf("LinearToSRGB→SRGBToLinear(%f) = %f, want %f", v, got, v)
		}
	}
}

func TestLinearSRGB_round_trip(t *testing.T) {
	for _, v := range []float64{0, 0.1, 0.25, 0.5, 0.75, 0.9, 1} {
		got := LinearToSRGB(SRGBToLinear(v))
		if math.Abs(got-v) > 1e-6 {
			t.Errorf("SRGBToLinear→LinearToSRGB(%f) = %f, want %f", v, got, v)
		}
	}
}

// --- OKLab round trip ---

func TestOKLab_black(t *testing.T) {
	L, a, b := LinearRGBToOKLab(0, 0, 0)
	if L != 0 || a != 0 || b != 0 {
		t.Fatalf("OKLab black = (%f,%f,%f), want (0,0,0)", L, a, b)
	}
}

func TestOKLab_white(t *testing.T) {
	L, a, b := LinearRGBToOKLab(1, 1, 1)
	if !almostEqual(L, 1) || !almostEqual(a, 0) || !almostEqual(b, 0) {
		t.Fatalf("OKLab white = (%f,%f,%f), want (1,0,0)", L, a, b)
	}
}

func TestOKLab_linear_round_trip(t *testing.T) {
	vals := []struct{ r, g, b float64 }{
		{0, 0, 0},
		{0.25, 0.25, 0.25},
		{0.5, 0.5, 0.5},
		{0.75, 0.75, 0.75},
		{1, 1, 1},
		{1, 0, 0},
		{0, 1, 0},
		{0, 0, 1},
		{1, 1, 0},
		{0, 1, 1},
		{1, 0, 1},
		{0.5, 0, 0},
		{0, 0.5, 0},
		{0, 0, 0.5},
		{0.2, 0.4, 0.8},
		{0.8, 0.4, 0.2},
	}
	const tol = 1e-6
	for _, v := range vals {
		L, a, b := LinearRGBToOKLab(v.r, v.g, v.b)
		r, g, bb := OKLabToLinearRGB(L, a, b)
		if math.Abs(r-v.r) > tol || math.Abs(g-v.g) > tol || math.Abs(bb-v.b) > tol {
			t.Errorf("OKLab round trip (%g,%g,%g) → (%g,%g,%g)", v.r, v.g, v.b, r, g, bb)
		}
	}
}

// --- sRGB (gfx.Color) round trip ---

func TestSRGB_oklab_round_trip_via_gfx(t *testing.T) {
	colors := []gfx.Color{
		{R: 0, G: 0, B: 0, A: 1},
		{R: 1, G: 1, B: 1, A: 1},
		{R: 1, G: 0, B: 0, A: 1},
		{R: 0, G: 1, B: 0, A: 1},
		{R: 0, G: 0, B: 1, A: 1},
		{R: 0.5, G: 0.5, B: 0.5, A: 1},
		{R: 0.2, G: 0.4, B: 0.8, A: 1},
		{R: 0.8, G: 0.2, B: 0.4, A: 1},
	}
	tol := float32(1e-5)
	for _, c := range colors {
		L, a, b := SRGBToOKLab(c)
		got := OKLabToSRGB(L, a, b)
		if diffR := float32(math.Abs(float64(got.R - c.R))); diffR > tol {
			t.Errorf("R diff %g for input %+v", diffR, c)
		}
		if diffG := float32(math.Abs(float64(got.G - c.G))); diffG > tol {
			t.Errorf("G diff %g for input %+v", diffG, c)
		}
		if diffB := float32(math.Abs(float64(got.B - c.B))); diffB > tol {
			t.Errorf("B diff %g for input %+v", diffB, c)
		}
	}
}

func TestSRGB_oklab_premultiplied_alpha(t *testing.T) {
	// Test with a semi-transparent color
	c := gfx.Color{R: 0.5, G: 0.25, B: 0.125, A: 0.5}
	_, _, _ = SRGBToOKLab(c)
	got := OKLabToSRGB(0.5, 0, 0)
	if got.A != 1 {
		t.Fatalf("expected alpha=1, got %f", got.A)
	}
}

// --- known reference values ---

func TestSRGBToOKLab_known_white(t *testing.T) {
	// sRGB white → OKLab (1, 0, 0)
	L, a, b := SRGBToOKLab(gfx.Color{R: 1, G: 1, B: 1, A: 1})
	if !almostEqual(L, 1) || !almostEqual(a, 0) || !almostEqual(b, 0) {
		t.Fatalf("white = (%f,%f,%f), want (1,0,0)", L, a, b)
	}
}

func TestSRGBToOKLab_known_red(t *testing.T) {
	// sRGB red → approximate OKLab from reference
	L, a, b := SRGBToOKLab(gfx.Color{R: 1, G: 0, B: 0, A: 1})
	if L <= 0 || a <= 0 || math.Abs(b) <= 0 {
		t.Fatalf("red OKLab = (%f,%f,%f) — all components should be positive for red", L, a, b)
	}
}

func TestSRGBToOKLab_known_blue(t *testing.T) {
	// sRGB blue → negative b (blue-yellow axis)
	_, _, b := SRGBToOKLab(gfx.Color{R: 0, G: 0, B: 1, A: 1})
	if b >= 0 {
		t.Fatalf("blue OKLab b = %f, want negative", b)
	}
}

// --- benchmarks ---

func BenchmarkSRGBToLinear(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SRGBToLinear(0.5)
	}
}

func BenchmarkLinearRGBToOKLab(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		LinearRGBToOKLab(0.5, 0.3, 0.8)
	}
}

func BenchmarkSRGBToOKLab(b *testing.B) {
	col := gfx.Color{R: 0.5, G: 0.3, B: 0.8, A: 1}
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		SRGBToOKLab(col)
	}
}

func BenchmarkOKLabToSRGB(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		OKLabToSRGB(0.5, 0.2, -0.1)
	}
}
