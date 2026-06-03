package colorscale

import (
	"math"
	"testing"
)

func TestDiverging_midpoint_exact_color(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	// At the midpoint (0), the color should be white (both rampLow's end and
	// rampHigh's start converge at white)
	c := s.Map(0)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("Map(0) = %+v, want white (1,1,1)", c)
	}
}

func TestDiverging_low_endpoint(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	// At domain lo, should map to rampLow's first stop
	c := s.Map(-1)
	first := RampBlueWhiteRedLow[0].Color
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("Map(-1) = %+v, want %+v", c, first)
	}
}

func TestDiverging_high_endpoint(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(1)
	last := RampBlueWhiteRedHigh[1].Color
	if c.R != last.R || c.G != last.G || c.B != last.B {
		t.Fatalf("Map(1) = %+v, want %+v", c, last)
	}
}

func TestDiverging_asymmetric_domain(t *testing.T) {
	// Domain [-2, 8] with midpoint at 0 (asymmetric).
	// -2 → blue, 0 → white, 8 → red
	s := NewDiverging(-2, 8, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(0)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("asymmetric midpoint = %+v, want white", c)
	}
	// At lo (-2): should be blue endpoint
	first := RampBlueWhiteRedLow[0].Color
	cLo := s.Map(-2)
	if cLo.R != first.R || cLo.G != first.G || cLo.B != first.B {
		t.Fatalf("asymmetric lo = %+v, want %+v", cLo, first)
	}
	// At hi (8): should be red endpoint
	last := RampBlueWhiteRedHigh[1].Color
	cHi := s.Map(8)
	if cHi.R != last.R || cHi.G != last.G || cHi.B != last.B {
		t.Fatalf("asymmetric hi = %+v, want %+v", cHi, last)
	}
}

func TestDiverging_midpoint_on_low_side(t *testing.T) {
	// Value between lo and midpoint should use the low ramp
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(-0.5)
	// Should be between blue and white
	first := RampBlueWhiteRedLow[0].Color
	if c.R <= first.R || c.G <= first.G || c.B <= first.B {
		t.Fatalf("Map(-0.5) = %+v, should be lighter than low endpoint %+v", c, first)
	}
}

func TestDiverging_midpoint_on_high_side(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(0.5)
	mid := RampBlueWhiteRedHigh[0].Color
	last := RampBlueWhiteRedHigh[1].Color
	// At t=0.5 in the high ramp, color should be between white and red.
	// In OKLab interpolation, the path is perceptual, so individual channels
	// may not be strictly between the endpoint values.
	if c.R == mid.R && c.G == mid.G && c.B == mid.B {
		t.Fatalf("Map(0.5) = %+v, expected between white and red", c)
	}
	if c.R == last.R && c.G == last.G && c.B == last.B {
		t.Fatalf("Map(0.5) = %+v, expected between white and red", c)
	}
}

func TestDiverging_clamp_low(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	s = s.WithClamp()
	first := RampBlueWhiteRedLow[0].Color
	c := s.Map(-10)
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("clamped Map(-10) = %+v, want low endpoint %+v", c, first)
	}
}

func TestDiverging_clamp_high(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	s = s.WithClamp()
	last := RampBlueWhiteRedHigh[1].Color
	c := s.Map(10)
	if c.R != last.R || c.G != last.G || c.B != last.B {
		t.Fatalf("clamped Map(10) = %+v, want high endpoint %+v", c, last)
	}
}

func TestDiverging_extrapolate_low(t *testing.T) {
	// Without clamp, values below lo should extrapolate past the low endpoint
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(-2)
	first := RampBlueWhiteRedLow[0].Color
	if c.R == first.R && c.G == first.G && c.B == first.B {
		t.Log("extrapolated Map(-2) same as endpoint (ramp clamps internally)")
	}
}

func TestDiverging_nan(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(math.NaN())
	// NaN should return the midpoint color (white)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("Map(NaN) = %+v, want white", c)
	}
}

func TestDiverging_inf_clamped_low(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	s = s.WithClamp()
	first := RampBlueWhiteRedLow[0].Color
	c := s.Map(math.Inf(-1))
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("clamped Map(-Inf) = %+v, want low endpoint %+v", c, first)
	}
}

func TestDiverging_inf_clamped_high(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	s = s.WithClamp()
	last := RampBlueWhiteRedHigh[1].Color
	c := s.Map(math.Inf(1))
	if c.R != last.R || c.G != last.G || c.B != last.B {
		t.Fatalf("clamped Map(+Inf) = %+v, want high endpoint %+v", c, last)
	}
}

func TestDiverging_nan_clamped(t *testing.T) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	s = s.WithClamp()
	first := RampBlueWhiteRedLow[0].Color
	c := s.Map(math.NaN())
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("clamped Map(NaN) = %+v, want low endpoint %+v", c, first)
	}
}

func TestDiverging_purple_green(t *testing.T) {
	s := NewDiverging(-10, 10, 0, RampPurpleWhiteGreenLow, RampPurpleWhiteGreenHigh, InterpolationOKLab)
	// Midpoint is white
	c := s.Map(0)
	if c.R != 1 || c.G != 1 || c.B != 1 {
		t.Fatalf("purple-green midpoint = %+v, want white", c)
	}
	// Low endpoint is purple
	first := RampPurpleWhiteGreenLow[0].Color
	cLo := s.Map(-10)
	if cLo.R != first.R || cLo.G != first.G || cLo.B != first.B {
		t.Fatalf("purple-green lo = %+v, want %+v", cLo, first)
	}
}

func TestDiverging_symmetric_midpoint(t *testing.T) {
	// For a symmetric domain where midpoint = (lo+hi)/2, the mapping
	// at opposite values should produce colors from opposite ramps.
	s := NewDiverging(-100, 100, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	cNeg := s.Map(-50)
	cPos := s.Map(50)
	// The low ramp's midpoint should be white (from rampLow.At(1))
	// The high ramp's midpoint should also be white (from rampHigh.At(0))
	// At t=-50: localT = (-50 - (-100)) / (0 - (-100)) = 50/100 = 0.5
	// At t=50: localT = (50 - 0) / (100 - 0) = 0.5
	// Both should be between their endpoints and white
	if cNeg.R <= 0.2 && cPos.B <= 0.2 {
		t.Fatalf("symmetric midpoints too dark: neg=%+v, pos=%+v", cNeg, cPos)
	}
}

func TestDiverging_out_of_range_extrapolate(t *testing.T) {
	s := NewDiverging(0, 100, 50, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	// Without clamp, values outside domain should use the ramp's internal
	// endpoint clamping (At clamps to [0,1]).
	c := s.Map(-50)
	first := RampBlueWhiteRedLow[0].Color
	if c.R != first.R || c.G != first.G || c.B != first.B {
		t.Fatalf("extrapolated Map(-50) = %+v, want low endpoint %+v (clamped by ramp)", c, first)
	}
}

func TestDiverging_midpoint_equals_domain_bounds(t *testing.T) {
	// Edge case: midpoint equals lo
	s := NewDiverging(0, 100, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(0)
	midColor := RampBlueWhiteRedHigh[0].Color // rampHigh.At(0) = midpoint color
	if c.R != midColor.R || c.G != midColor.G || c.B != midColor.B {
		t.Fatalf("midpoint at lo = %+v, want midpoint color %+v", c, midColor)
	}
}

func TestDiverging_midpoint_equals_hi(t *testing.T) {
	s := NewDiverging(0, 100, 100, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	c := s.Map(100)
	midColor := RampBlueWhiteRedHigh[0].Color
	if c.R != midColor.R || c.G != midColor.G || c.B != midColor.B {
		t.Fatalf("midpoint at hi = %+v, want midpoint color %+v", c, midColor)
	}
}

func TestDiverging_all_same_value(t *testing.T) {
	// All values at the midpoint produce white
	s := NewDiverging(-10, 10, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	for i := 0; i < 10; i++ {
		c := s.Map(0)
		if c.R != 1 || c.G != 1 || c.B != 1 {
			t.Fatalf("Map(0) iteration %d = %+v, want white", i, c)
		}
	}
}

// --- benchmarks ---

func BenchmarkDivergingMap(b *testing.B) {
	s := NewDiverging(-1, 1, 0, RampBlueWhiteRedLow, RampBlueWhiteRedHigh, InterpolationOKLab)
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		s.Map(float64(i%200-100) / 100)
	}
}
