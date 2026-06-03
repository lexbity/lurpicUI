package scale

import (
	"math"
	"testing"
)

// --- construction ---

func TestNewLinear_defaults(t *testing.T) {
	s := NewLinear()
	lo, hi := s.Domain()
	if lo != 0 || hi != 0 {
		t.Fatalf("expected default domain [0,0], got [%f,%f]", lo, hi)
	}
	lo, hi = s.Range()
	if lo != 0 || hi != 0 {
		t.Fatalf("expected default range [0,0], got [%f,%f]", lo, hi)
	}
	if s.Kind() != KindLinear {
		t.Fatalf("expected KindLinear, got %s", s.Kind())
	}
}

func TestNewLinear_with_options(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(50, 200))
	lo, hi := s.Domain()
	if lo != 0 || hi != 100 {
		t.Fatalf("domain = [%f,%f], want [0,100]", lo, hi)
	}
	lo, hi = s.Range()
	if lo != 50 || hi != 200 {
		t.Fatalf("range = [%f,%f], want [50,200]", lo, hi)
	}
}

func TestNewLinear_default_clamp_is_extrapolate(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	// Without WithClamp, out-of-domain values should extrapolate
	if got := s.Map(200); got != 1000 {
		t.Fatalf("Map(200) without clamp = %f, want 1000 (extrapolated)", got)
	}
}

func TestNewLinear_with_clamp(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if got := s.Map(200); got != 500 {
		t.Fatalf("Map(200) with clamp = %f, want 500", got)
	}
}

// --- Map ---

func TestLinear_Map_endpoints(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(50, 200))
	if got := s.Map(0); got != 50 {
		t.Fatalf("Map(0) = %f, want 50", got)
	}
	if got := s.Map(100); got != 200 {
		t.Fatalf("Map(100) = %f, want 200", got)
	}
}

func TestLinear_Map_midpoint(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(50, 200))
	if got := s.Map(50); got != 125 {
		t.Fatalf("Map(50) = %f, want 125", got)
	}
}

func TestLinear_Map_arbitrary(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	tests := []struct {
		value float64
		want  float64
	}{
		{0, 0},
		{25, 125},
		{50, 250},
		{75, 375},
		{100, 500},
	}
	for _, tt := range tests {
		if got := s.Map(tt.value); got != tt.want {
			t.Errorf("Map(%f) = %f, want %f", tt.value, got, tt.want)
		}
	}
}

func TestLinear_Map_reversed_domain(t *testing.T) {
	s := NewLinear(WithDomain(100, 0), WithRange(0, 500))
	if got := s.Map(100); got != 0 {
		t.Fatalf("Map(100) with reversed domain = %f, want 0", got)
	}
	if got := s.Map(0); got != 500 {
		t.Fatalf("Map(0) with reversed domain = %f, want 500", got)
	}
	if got := s.Map(50); got != 250 {
		t.Fatalf("Map(50) with reversed domain = %f, want 250", got)
	}
}

func TestLinear_Map_reversed_range(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(500, 0))
	if got := s.Map(0); got != 500 {
		t.Fatalf("Map(0) with reversed range = %f, want 500", got)
	}
	if got := s.Map(100); got != 0 {
		t.Fatalf("Map(100) with reversed range = %f, want 0", got)
	}
	if got := s.Map(50); got != 250 {
		t.Fatalf("Map(50) with reversed range = %f, want 250", got)
	}
}

// --- extrapolation (default) ---

func TestLinear_Map_extrapolate_above(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if got := s.Map(150); got != 750 {
		t.Fatalf("Map(150) = %f, want 750", got)
	}
}

func TestLinear_Map_extrapolate_below(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if got := s.Map(-50); got != -250 {
		t.Fatalf("Map(-50) = %f, want -250", got)
	}
}

// --- clamp ---

func TestLinear_Map_clamp_above(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if got := s.Map(150); got != 500 {
		t.Fatalf("Map(150) with clamp = %f, want 500", got)
	}
}

func TestLinear_Map_clamp_below(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if got := s.Map(-50); got != 0 {
		t.Fatalf("Map(-50) with clamp = %f, want 0", got)
	}
}

func TestLinear_Map_clamp_within_domain_is_unchanged(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if got := s.Map(50); got != 250 {
		t.Fatalf("Map(50) with clamp = %f, want 250", got)
	}
}

// --- degenerate domain ---

func TestLinear_Map_degenerate_domain(t *testing.T) {
	s := NewLinear(WithDomain(5, 5), WithRange(0, 100))
	if got := s.Map(5); got != 50 {
		t.Fatalf("Map(5) with degenerate domain = %f, want 50 (range midpoint)", got)
	}
	if got := s.Map(100); got != 50 {
		t.Fatalf("Map(100) with degenerate domain = %f, want 50", got)
	}
	if got := s.Map(-100); got != 50 {
		t.Fatalf("Map(-100) with degenerate domain = %f, want 50", got)
	}
}

// --- degenerate range ---

func TestLinear_Map_degenerate_range(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(42, 42))
	if got := s.Map(0); got != 42 {
		t.Fatalf("Map(0) with degenerate range = %f, want 42", got)
	}
	if got := s.Map(50); got != 42 {
		t.Fatalf("Map(50) with degenerate range = %f, want 42", got)
	}
	if got := s.Map(100); got != 42 {
		t.Fatalf("Map(100) with degenerate range = %f, want 42", got)
	}
}

// --- Invert ---

func TestLinear_Invert_endpoints(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(50, 200))
	if got := s.Invert(50); got != 0 {
		t.Fatalf("Invert(50) = %f, want 0", got)
	}
	if got := s.Invert(200); got != 100 {
		t.Fatalf("Invert(200) = %f, want 100", got)
	}
}

func TestLinear_Invert_midpoint(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(50, 200))
	if got := s.Invert(125); got != 50 {
		t.Fatalf("Invert(125) = %f, want 50", got)
	}
}

func TestLinear_Invert_reversed_range(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(500, 0))
	if got := s.Invert(500); got != 0 {
		t.Fatalf("Invert(500) with reversed range = %f, want 0", got)
	}
	if got := s.Invert(0); got != 100 {
		t.Fatalf("Invert(0) with reversed range = %f, want 100", got)
	}
}

func TestLinear_Invert_degenerate_range(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(5, 5))
	if got := s.Invert(5); got != 50 {
		t.Fatalf("Invert(5) with degenerate range = %f, want 50 (domain midpoint)", got)
	}
	if got := s.Invert(0); got != 50 {
		t.Fatalf("Invert(0) with degenerate range = %f, want 50", got)
	}
	if got := s.Invert(100); got != 50 {
		t.Fatalf("Invert(100) with degenerate range = %f, want 50", got)
	}
}

func TestLinear_Invert_degenerate_domain(t *testing.T) {
	s := NewLinear(WithDomain(42, 42), WithRange(0, 100))
	if got := s.Invert(50); got != 42 {
		t.Fatalf("Invert(50) with degenerate domain = %f, want 42", got)
	}
	if got := s.Invert(0); got != 42 {
		t.Fatalf("Invert(0) with degenerate domain = %f, want 42", got)
	}
	if got := s.Invert(100); got != 42 {
		t.Fatalf("Invert(100) with degenerate domain = %f, want 42", got)
	}
}

func TestLinear_Invert_extrapolate(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	// 750 is outside range [0,500], should extrapolate
	if got := s.Invert(750); got != 150 {
		t.Fatalf("Invert(750) = %f, want 150 (extrapolated)", got)
	}
}

func TestLinear_Invert_clamp(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if got := s.Invert(750); got != 100 {
		t.Fatalf("Invert(750) with clamp = %f, want 100", got)
	}
	if got := s.Invert(-250); got != 0 {
		t.Fatalf("Invert(-250) with clamp = %f, want 0", got)
	}
}

// --- monotonicity ---

func TestLinear_Map_monotonic(t *testing.T) {
	s := NewLinear(WithDomain(-10, 10), WithRange(0, 1000))
	prev := s.Map(-10)
	for v := -9.5; v <= 10; v += 0.5 {
		cur := s.Map(v)
		if cur < prev {
			t.Fatalf("Map not monotonic at %f: %f < %f", v, cur, prev)
		}
		prev = cur
	}
}

func TestLinear_Invert_monotonic(t *testing.T) {
	s := NewLinear(WithDomain(-10, 10), WithRange(0, 1000))
	prev := s.Invert(0)
	for p := 1.0; p <= 1000; p += 10 {
		cur := s.Invert(p)
		if cur < prev {
			t.Fatalf("Invert not monotonic at %f: %f < %f", p, cur, prev)
		}
		prev = cur
	}
}

// --- round-trip property ---

func TestLinear_round_trip_within_domain(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	const eps = 1e-12
	for v := 0.0; v <= 100; v += 5 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%f)) = %f, want %f (diff %e)", v, got, v, math.Abs(got-v))
		}
	}
}

func TestLinear_round_trip_reversed_domain(t *testing.T) {
	s := NewLinear(WithDomain(100, 0), WithRange(0, 500))
	const eps = 1e-12
	for v := 0.0; v <= 100; v += 5 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%f)) with reversed domain = %f, want %f", v, got, v)
		}
	}
}

func TestLinear_round_trip_reversed_range(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(500, 0))
	const eps = 1e-12
	for v := 0.0; v <= 100; v += 5 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%f)) with reversed range = %f, want %f", v, got, v)
		}
	}
}

// --- NaN / Inf ---

func TestLinear_Map_nan(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if !math.IsNaN(s.Map(math.NaN())) {
		t.Fatal("Map(NaN) should return NaN")
	}
}

func TestLinear_Invert_nan(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if !math.IsNaN(s.Invert(math.NaN())) {
		t.Fatal("Invert(NaN) should return NaN")
	}
}

func TestLinear_Map_inf(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500))
	if got := s.Map(math.Inf(1)); !math.IsInf(got, 1) {
		t.Fatalf("Map(+Inf) = %f, want +Inf", got)
	}
	if got := s.Map(math.Inf(-1)); !math.IsInf(got, -1) {
		t.Fatalf("Map(-Inf) = %f, want -Inf", got)
	}
}

func TestLinear_Map_clamp_with_nan_domain_leaves_nan(t *testing.T) {
	s := NewLinear(WithDomain(0, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	// domain isn't NaN, but the value is — should propagate
	v := s.Map(math.NaN())
	if !math.IsNaN(v) {
		t.Fatalf("Map(NaN) with clamp = %f, want NaN", v)
	}
}

// --- interface satisfaction ---

func TestLinear_implements_Scale(t *testing.T) {
	var s Scale = NewLinear()
	_ = s
}

func TestLinear_implements_InvertibleScale(t *testing.T) {
	var s InvertibleScale = NewLinear()
	_ = s
}

// --- benchmarks ---

func BenchmarkLinearScale_Map(b *testing.B) {
	s := NewLinear(WithDomain(0, 1000), WithRange(0, 500))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Map(float64(i % 1000))
	}
}

func BenchmarkLinearScale_Invert(b *testing.B) {
	s := NewLinear(WithDomain(0, 1000), WithRange(0, 500))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Invert(float64(i % 500))
	}
}
