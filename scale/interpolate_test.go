package scale

import (
	"math"
	"testing"
)

// --- lerp ---

func TestLerp_endpoints(t *testing.T) {
	if got := lerp(3, 7, 0); got != 3 {
		t.Fatalf("lerp(3,7,0) = %f, want 3", got)
	}
	if got := lerp(3, 7, 1); got != 7 {
		t.Fatalf("lerp(3,7,1) = %f, want 7", got)
	}
}

func TestLerp_midpoint(t *testing.T) {
	if got := lerp(3, 7, 0.5); got != 5 {
		t.Fatalf("lerp(3,7,0.5) = %f, want 5", got)
	}
}

func TestLerp_extrapolate(t *testing.T) {
	if got := lerp(3, 7, 2); got != 11 {
		t.Fatalf("lerp(3,7,2) = %f, want 11", got)
	}
	if got := lerp(3, 7, -1); got != -1 {
		t.Fatalf("lerp(3,7,-1) = %f, want -1", got)
	}
}

func TestLerp_degenerate_range(t *testing.T) {
	if got := lerp(5, 5, 0); got != 5 {
		t.Fatalf("lerp(5,5,0) = %f, want 5", got)
	}
	if got := lerp(5, 5, 0.5); got != 5 {
		t.Fatalf("lerp(5,5,0.5) = %f, want 5", got)
	}
	if got := lerp(5, 5, 1); got != 5 {
		t.Fatalf("lerp(5,5,1) = %f, want 5", got)
	}
}

func TestLerp_negative_interval(t *testing.T) {
	if got := lerp(10, 0, 0.5); got != 5 {
		t.Fatalf("lerp(10,0,0.5) = %f, want 5", got)
	}
}

func TestLerp_nan_propagates(t *testing.T) {
	if !math.IsNaN(lerp(math.NaN(), 7, 0.5)) {
		t.Fatal("lerp with NaN a should return NaN")
	}
	if !math.IsNaN(lerp(3, math.NaN(), 0.5)) {
		t.Fatal("lerp with NaN b should return NaN")
	}
	if !math.IsNaN(lerp(3, 7, math.NaN())) {
		t.Fatal("lerp with NaN t should return NaN")
	}
}

func TestLerp_inf(t *testing.T) {
	// lerp with finite interval and Inf t still works
	if got := lerp(0, 10, math.Inf(1)); !math.IsInf(got, 1) {
		t.Fatalf("lerp(0,10,+Inf) = %f, want +Inf", got)
	}
	if got := lerp(0, 10, math.Inf(-1)); !math.IsInf(got, -1) {
		t.Fatalf("lerp(0,10,-Inf) = %f, want -Inf", got)
	}
	// degenerate Inf-Inf produces NaN per IEEE 754
	got := lerp(math.Inf(1), math.Inf(1), 0.5)
	if !math.IsNaN(got) {
		t.Fatalf("lerp(+Inf,+Inf,0.5) = %f, want NaN (Inf-Inf is NaN)", got)
	}
}

// --- normalize ---

func TestNormalize_endpoints(t *testing.T) {
	if got := normalize(0, 0, 10); got != 0 {
		t.Fatalf("normalize(0,0,10) = %f, want 0", got)
	}
	if got := normalize(10, 0, 10); got != 1 {
		t.Fatalf("normalize(10,0,10) = %f, want 1", got)
	}
}

func TestNormalize_midpoint(t *testing.T) {
	if got := normalize(5, 0, 10); got != 0.5 {
		t.Fatalf("normalize(5,0,10) = %f, want 0.5", got)
	}
}

func TestNormalize_extrapolate(t *testing.T) {
	if got := normalize(20, 0, 10); got != 2 {
		t.Fatalf("normalize(20,0,10) = %f, want 2", got)
	}
	if got := normalize(-5, 0, 10); got != -0.5 {
		t.Fatalf("normalize(-5,0,10) = %f, want -0.5", got)
	}
}

func TestNormalize_degenerate_span(t *testing.T) {
	if got := normalize(0, 5, 5); got != 0 {
		t.Fatalf("normalize(0,5,5) = %f, want 0", got)
	}
	if got := normalize(5, 5, 5); got != 0 {
		t.Fatalf("normalize(5,5,5) = %f, want 0", got)
	}
	if got := normalize(100, 5, 5); got != 0 {
		t.Fatalf("normalize(100,5,5) = %f, want 0", got)
	}
}

func TestNormalize_reversed_domain(t *testing.T) {
	if got := normalize(5, 10, 0); got != 0.5 {
		t.Fatalf("normalize(5,10,0) = %f, want 0.5", got)
	}
	if got := normalize(0, 10, 0); got != 1 {
		t.Fatalf("normalize(0,10,0) = %f, want 1", got)
	}
}

func TestNormalize_nan(t *testing.T) {
	if !math.IsNaN(normalize(math.NaN(), 0, 10)) {
		t.Fatal("normalize with NaN value should return NaN")
	}
}

func TestNormalize_inf(t *testing.T) {
	if got := normalize(math.Inf(1), 0, 10); !math.IsInf(got, 1) {
		t.Fatalf("normalize(+Inf,0,10) = %f, want +Inf", got)
	}
	if got := normalize(math.Inf(-1), 0, 10); !math.IsInf(got, -1) {
		t.Fatalf("normalize(-Inf,0,10) = %f, want -Inf", got)
	}
}

// --- uninterpolate ---

func TestUninterpolate_basic(t *testing.T) {
	f := uninterpolate(0, 10)
	if got := f(0); got != 0 {
		t.Fatalf("uninterpolate(0,10)(0) = %f, want 0", got)
	}
	if got := f(5); got != 0.5 {
		t.Fatalf("uninterpolate(0,10)(5) = %f, want 0.5", got)
	}
	if got := f(10); got != 1 {
		t.Fatalf("uninterpolate(0,10)(10) = %f, want 1", got)
	}
}

func TestUninterpolate_degenerate_span(t *testing.T) {
	f := uninterpolate(5, 5)
	if got := f(0); got != 0 {
		t.Fatalf("uninterpolate(5,5)(0) = %f, want 0", got)
	}
	if got := f(5); got != 0 {
		t.Fatalf("uninterpolate(5,5)(5) = %f, want 0", got)
	}
	if got := f(100); got != 0 {
		t.Fatalf("uninterpolate(5,5)(100) = %f, want 0", got)
	}
}

func TestUninterpolate_reversed(t *testing.T) {
	f := uninterpolate(10, 0)
	if got := f(5); got != 0.5 {
		t.Fatalf("uninterpolate(10,0)(5) = %f, want 0.5", got)
	}
}

// --- clamp01 ---

func TestClamp01_normal(t *testing.T) {
	if got := clamp01(0.5); got != 0.5 {
		t.Fatalf("clamp01(0.5) = %f, want 0.5", got)
	}
}

func TestClamp01_boundaries(t *testing.T) {
	if got := clamp01(0); got != 0 {
		t.Fatalf("clamp01(0) = %f, want 0", got)
	}
	if got := clamp01(1); got != 1 {
		t.Fatalf("clamp01(1) = %f, want 1", got)
	}
}

func TestClamp01_below_clamps(t *testing.T) {
	if got := clamp01(-0.5); got != 0 {
		t.Fatalf("clamp01(-0.5) = %f, want 0", got)
	}
	if got := clamp01(-1e10); got != 0 {
		t.Fatalf("clamp01(-1e10) = %f, want 0", got)
	}
	if got := clamp01(math.Inf(-1)); got != 0 {
		t.Fatalf("clamp01(-Inf) = %f, want 0", got)
	}
}

func TestClamp01_above_clamps(t *testing.T) {
	if got := clamp01(1.5); got != 1 {
		t.Fatalf("clamp01(1.5) = %f, want 1", got)
	}
	if got := clamp01(1e10); got != 1 {
		t.Fatalf("clamp01(1e10) = %f, want 1", got)
	}
	if got := clamp01(math.Inf(1)); got != 1 {
		t.Fatalf("clamp01(+Inf) = %f, want 1", got)
	}
}

func TestClamp01_nan(t *testing.T) {
	if !math.IsNaN(clamp01(math.NaN())) {
		t.Fatal("clamp01(NaN) should return NaN")
	}
}
