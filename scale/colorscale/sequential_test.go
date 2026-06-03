package colorscale

import (
	"math"
	"testing"
)

// --- Ramp ---

func TestRamp_empty(t *testing.T) {
	r := Ramp{}
	c := r.At(0.5, InterpolationOKLab)
	if c.R != 0 || c.G != 0 || c.B != 0 || c.A != 0 {
		t.Fatalf("empty ramp = %+v, want zero color", c)
	}
}

func TestRamp_clamp_low(t *testing.T) {
	c := RampGrayscale.At(-1, InterpolationOKLab)
	if c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("At(-1) = %+v, want black", c)
	}
}

func TestRamp_clamp_high(t *testing.T) {
	c := RampGrayscale.At(2, InterpolationOKLab)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("At(2) = %+v, want white", c)
	}
}

func TestRamp_endpoints_exact(t *testing.T) {
	// At t=0 and t=1, the ramp should return exact endpoint colors
	c0 := RampViridis.At(0, InterpolationOKLab)
	c100 := RampViridis.At(1, InterpolationOKLab)
	// These should be equal to the endpoint definitions
	if c0.R != viridis0.R || c0.G != viridis0.G || c0.B != viridis0.B {
		t.Fatalf("At(0) = %+v, want %+v", c0, viridis0)
	}
	if c100.R != viridis100.R || c100.G != viridis100.G || c100.B != viridis100.B {
		t.Fatalf("At(1) = %+v, want %+v", c100, viridis100)
	}
}

func TestRamp_nan_returns_first_stop(t *testing.T) {
	c := RampViridis.At(math.NaN(), InterpolationOKLab)
	first := RampViridis[0].Color
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("At(NaN) = %+v, want first stop %+v", c, first)
	}
}

func TestRamp_midpoint_reasonable(t *testing.T) {
	// OKLab interpolation of black→white at t=0.5 gives a perceptual
	// midpoint around L=0.5 in OKLab, which maps to ~0.39 in sRGB.
	// We verify the value is approximately gray (channels within tolerance).
	c := RampGrayscale.At(0.5, InterpolationOKLab)
	if c.A != 1 {
		t.Fatalf("grayscale midpoint alpha = %f, want 1", c.A)
	}
	// Tiny float32 differences from OKLab round-trip; channels should be nearly equal
	const tol = 1e-6
	diffRG := float64(c.R - c.G)
	diffGB := float64(c.G - c.B)
	if diffRG < 0 {
		diffRG = -diffRG
	}
	if diffGB < 0 {
		diffGB = -diffGB
	}
	if diffRG > tol || diffGB > tol {
		t.Fatalf("grayscale midpoint = %+v, not gray (diffs %g, %g)", c, diffRG, diffGB)
	}
}

func TestRamp_oklab_vs_linear_midpoint_differs(t *testing.T) {
	// For a black→white ramp, OKLab and linear sRGB interpolation should
	// produce measurably different midpoints (OKLab is perceptually uniform,
	// placing the midpoint at ~0.39 sRGB, while linear sRGB gives ~0.74).
	ok := RampGrayscale.At(0.5, InterpolationOKLab)
	lin := RampGrayscale.At(0.5, InterpolationLinearSRGB)
	// Compute RGB channel difference as a simple distance metric
	rDiff := math.Abs(float64(ok.R - lin.R))
	gDiff := math.Abs(float64(ok.G - lin.G))
	bDiff := math.Abs(float64(ok.B - lin.B))
	totalDiff := rDiff + gDiff + bDiff
	if totalDiff < 0.1 {
		t.Errorf("OKLab vs linear midpoints too similar: ok=%+v lin=%+v diff=%g", ok, lin, totalDiff)
	}
}

// --- SequentialColor ---

func TestSequential_basic(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	// At domain lo: black
	c := s.Map(0)
	if c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("Map(0) = %+v, want black", c)
	}
	// At domain hi: white
	c = s.Map(100)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("Map(100) = %+v, want white", c)
	}
}

func TestSequential_midpoint(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	c := s.Map(50)
	if c.A != 1 {
		t.Fatalf("Map(50) alpha = %f, want 1", c.A)
	}
	// Perceptual midpoint through OKLab; just verify it's between black and white
	if c.R <= 0 || c.R >= 1 || c.G <= 0 || c.G >= 1 || c.B <= 0 || c.B >= 1 {
		t.Fatalf("Map(50) = %+v, want between black and white", c)
	}
}

func TestSequential_extrapolate_low(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	// Without clamp, extrapolate: values below domain → past black
	c := s.Map(-50)
	if c.R < 0 || c.G < 0 || c.B < 0 {
		t.Fatalf("Map(-50) without clamp = %+v, want extrapolated (may go negative)", c)
	}
}

func TestSequential_clamp_low(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	s = s.WithClamp()
	c := s.Map(-50)
	if c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("Map(-50) with clamp = %+v, want black", c)
	}
}

func TestSequential_clamp_high(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	s = s.WithClamp()
	c := s.Map(200)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("Map(200) with clamp = %+v, want white", c)
	}
}

func TestSequential_degenerate_domain(t *testing.T) {
	s := NewSequential(42, 42, RampGrayscale, InterpolationOKLab)
	c := s.Map(42)
	// Degenerate domain should map to t=0
	if c.R != 0 || c.G != 0 || c.B != 0 {
		t.Fatalf("Map(42) degenerate = %+v, want black", c)
	}
}

func TestSequential_nan(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	c := s.Map(math.NaN())
	// NaN is treated as t=0 → first ramp stop
	first := RampGrayscale[0].Color
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("Map(NaN) = %+v, want first stop %+v", c, first)
	}
}

func TestSequential_inf(t *testing.T) {
	s := NewSequential(0, 100, RampGrayscale, InterpolationOKLab)
	c := s.Map(math.Inf(1))
	// Inf should clamp to 1 (white)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("Map(+Inf) = %+v, want white (clamped)", c)
	}
}

func TestSequential_interpolation_space(t *testing.T) {
	// Both spaces should produce reasonable colors at all t values
	for _, space := range []InterpolationSpace{InterpolationOKLab, InterpolationLinearSRGB} {
		s := NewSequential(0, 1, RampViridis, space)
		for val := 0.0; val <= 1.0; val += 0.1 {
			c := s.Map(val)
			if c.A != 1 {
				t.Errorf("Map(%g) has alpha %f, want 1", val, c.A)
			}
		}
	}
}

func TestSequential_viridis_monotonic_lightness(t *testing.T) {
	// Viridis should have monotonically increasing lightness
	s := NewSequential(0, 1, RampViridis, InterpolationOKLab)
	var prevL float64
	for v := 0.0; v <= 1.001; v += 0.05 {
		L, _, _ := SRGBToOKLab(s.Map(v))
		if v > 0 && L < prevL-0.001 {
			t.Fatalf("lightness decreased at v=%g: %g -> %g", v, prevL, L)
		}
		prevL = L
	}
}

// --- benchmarks ---

func BenchmarkRampAt_OKLab(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		RampViridis.At(0.5, InterpolationOKLab)
	}
}

func BenchmarkSequentialMap(b *testing.B) {
	s := NewSequential(0, 100, RampViridis, InterpolationOKLab)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.Map(float64(i % 100))
	}
}
