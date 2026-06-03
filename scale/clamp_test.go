package scale

import (
	"math"
	"testing"
)

// --- clamp ---

func TestClamp_within_range(t *testing.T) {
	if got := clamp(5, 0, 10); got != 5 {
		t.Fatalf("clamp(5,0,10) = %f, want 5", got)
	}
}

func TestClamp_below_lo(t *testing.T) {
	if got := clamp(-5, 0, 10); got != 0 {
		t.Fatalf("clamp(-5,0,10) = %f, want 0", got)
	}
}

func TestClamp_above_hi(t *testing.T) {
	if got := clamp(15, 0, 10); got != 10 {
		t.Fatalf("clamp(15,0,10) = %f, want 10", got)
	}
}

func TestClamp_exact_boundaries(t *testing.T) {
	if got := clamp(0, 0, 10); got != 0 {
		t.Fatalf("clamp(0,0,10) = %f, want 0", got)
	}
	if got := clamp(10, 0, 10); got != 10 {
		t.Fatalf("clamp(10,0,10) = %f, want 10", got)
	}
}

func TestClamp_negative_interval(t *testing.T) {
	if got := clamp(-5, -10, -1); got != -5 {
		t.Fatalf("clamp(-5,-10,-1) = %f, want -5", got)
	}
	if got := clamp(-15, -10, -1); got != -10 {
		t.Fatalf("clamp(-15,-10,-1) = %f, want -10", got)
	}
	if got := clamp(0, -10, -1); got != -1 {
		t.Fatalf("clamp(0,-10,-1) = %f, want -1", got)
	}
}

func TestClamp_reversed_bounds(t *testing.T) {
	// lo > hi — implementation-defined but must not panic
	_ = clamp(5, 10, 0)
	_ = clamp(-5, 10, 0)
	_ = clamp(20, 10, 0)
}

func TestClamp_zero_width_interval(t *testing.T) {
	if got := clamp(5, 7, 7); got != 7 {
		t.Fatalf("clamp(5,7,7) = %f, want 7", got)
	}
	if got := clamp(10, 7, 7); got != 7 {
		t.Fatalf("clamp(10,7,7) = %f, want 7", got)
	}
	if got := clamp(7, 7, 7); got != 7 {
		t.Fatalf("clamp(7,7,7) = %f, want 7", got)
	}
}

func TestClamp_nan(t *testing.T) {
	if !math.IsNaN(clamp(math.NaN(), 0, 10)) {
		t.Fatal("clamp(NaN,0,10) should return NaN")
	}
}

func TestClamp_inf(t *testing.T) {
	if got := clamp(math.Inf(1), 0, 10); got != 10 {
		t.Fatalf("clamp(+Inf,0,10) = %f, want 10", got)
	}
	if got := clamp(math.Inf(-1), 0, 10); got != 0 {
		t.Fatalf("clamp(-Inf,0,10) = %f, want 0", got)
	}
}

// --- clampOutOfRange ---

func TestClampOutOfRange_extrapolate_default(t *testing.T) {
	if got := clampOutOfRange(15, 0, 10, OutOfRangeExtrapolate); got != 15 {
		t.Fatalf("clampOutOfRange(15,0,10,extrapolate) = %f, want 15", got)
	}
	if got := clampOutOfRange(-5, 0, 10, OutOfRangeExtrapolate); got != -5 {
		t.Fatalf("clampOutOfRange(-5,0,10,extrapolate) = %f, want -5", got)
	}
}

func TestClampOutOfRange_clamp(t *testing.T) {
	if got := clampOutOfRange(15, 0, 10, OutOfRangeClamp); got != 10 {
		t.Fatalf("clampOutOfRange(15,0,10,clamp) = %f, want 10", got)
	}
	if got := clampOutOfRange(-5, 0, 10, OutOfRangeClamp); got != 0 {
		t.Fatalf("clampOutOfRange(-5,0,10,clamp) = %f, want 0", got)
	}
}

func TestClampOutOfRange_within_bounds(t *testing.T) {
	if got := clampOutOfRange(5, 0, 10, OutOfRangeClamp); got != 5 {
		t.Fatalf("clampOutOfRange(5,0,10,clamp) = %f, want 5", got)
	}
	if got := clampOutOfRange(5, 0, 10, OutOfRangeExtrapolate); got != 5 {
		t.Fatalf("clampOutOfRange(5,0,10,extrapolate) = %f, want 5", got)
	}
}

func TestClampOutOfRange_degnerate_bounds(t *testing.T) {
	if got := clampOutOfRange(5, 7, 7, OutOfRangeClamp); got != 7 {
		t.Fatalf("clampOutOfRange(5,7,7,clamp) = %f, want 7", got)
	}
	if got := clampOutOfRange(5, 7, 7, OutOfRangeExtrapolate); got != 5 {
		t.Fatalf("clampOutOfRange(5,7,7,extrapolate) = %f, want 5", got)
	}
}

func TestClampOutOfRange_nan(t *testing.T) {
	if !math.IsNaN(clampOutOfRange(math.NaN(), 0, 10, OutOfRangeClamp)) {
		t.Fatal("clampOutOfRange(NaN,...,clamp) should return NaN")
	}
	if !math.IsNaN(clampOutOfRange(math.NaN(), 0, 10, OutOfRangeExtrapolate)) {
		t.Fatal("clampOutOfRange(NaN,...,extrapolate) should return NaN")
	}
}
