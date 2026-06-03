package scale

import (
	"math"
	"testing"
)

// --- construction & validation ---

func TestLog_new_valid_positive(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lo, hi := s.Domain()
	if lo != 1 || hi != 1000 {
		t.Fatalf("domain = [%f,%f], want [1,1000]", lo, hi)
	}
	lo, hi = s.Range()
	if lo != 0 || hi != 500 {
		t.Fatalf("range = [%f,%f], want [0,500]", lo, hi)
	}
	if s.Kind() != KindLog {
		t.Fatalf("kind = %s, want KindLog", s.Kind())
	}
}

func TestLog_new_valid_negative(t *testing.T) {
	s, err := NewLog(WithDomain(-1000, -1), WithRange(0, 500))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	lo, hi := s.Domain()
	if lo != -1000 || hi != -1 {
		t.Fatalf("domain = [%f,%f], want [-1000,-1]", lo, hi)
	}
}

func TestLog_new_errors_on_zero_crossing(t *testing.T) {
	_, err := NewLog(WithDomain(-1, 10), WithRange(0, 500))
	if err != ErrDomainCrossesZero {
		t.Fatalf("expected ErrDomainCrossesZero for zero-crossing domain, got %v", err)
	}
}

func TestLog_new_errors_on_zero_included(t *testing.T) {
	_, err := NewLog(WithDomain(0, 100), WithRange(0, 500))
	if err != ErrDomainCrossesZero {
		t.Fatalf("expected ErrDomainCrossesZero for domain including zero, got %v", err)
	}
}

func TestLog_new_errors_on_invalid_base(t *testing.T) {
	_, err := NewLog(WithDomain(1, 100), WithRange(0, 500), WithBase(0))
	if err != ErrInvalidDomain {
		t.Fatalf("expected ErrInvalidDomain for base=0, got %v", err)
	}
	_, err = NewLog(WithDomain(1, 100), WithRange(0, 500), WithBase(1))
	if err != ErrInvalidDomain {
		t.Fatalf("expected ErrInvalidDomain for base=1, got %v", err)
	}
}

func TestLog_new_default_base_is_10(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// Map(10) → log10(10) = 1. log-space: [0,3]
	// normalize(1, 0, 3) = 1/3 ≈ 0.333
	got := s.Map(10)
	if math.Abs(got-166.66666666666666) > 1e-10 {
		t.Fatalf("Map(10) with base 10 = %f, want ~166.67", got)
	}
}

// --- Map ---

func TestLog_Map_endpoints(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(50, 200))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Map(1); got != 50 {
		t.Fatalf("Map(1) = %f, want 50", got)
	}
	if got := s.Map(1000); got != 200 {
		t.Fatalf("Map(1000) = %f, want 200", got)
	}
}

func TestLog_Map_mid_decade(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// log10(10) = 1, normalize(1, 0, 3) = 1/3
	if got := s.Map(10); math.Abs(got-166.66666666666666) > 1e-10 {
		t.Fatalf("Map(10) = %f, want ~166.667", got)
	}
	// log10(100) = 2, normalize(2, 0, 3) = 2/3
	if got := s.Map(100); math.Abs(got-333.3333333333333) > 1e-10 {
		t.Fatalf("Map(100) = %f, want ~333.333", got)
	}
}

func TestLog_Map_degenerate_domain(t *testing.T) {
	s, err := NewLog(WithDomain(5, 5), WithRange(0, 100))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Map(5); got != 50 {
		t.Fatalf("Map(5) with degenerate domain = %f, want 50", got)
	}
}

func TestLog_Map_nan(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN(s.Map(math.NaN())) {
		t.Fatal("Map(NaN) should return NaN")
	}
}

// --- Invert ---

func TestLog_Invert_endpoints(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(50, 200))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-12
	if got := s.Invert(50); math.Abs(got-1) > eps {
		t.Fatalf("Invert(50) = %g, want 1 (diff %e)", got, math.Abs(got-1))
	}
	if got := s.Invert(200); math.Abs(got-1000) > eps {
		t.Fatalf("Invert(200) = %g, want 1000 (diff %e)", got, math.Abs(got-1000))
	}
}

func TestLog_Invert_midpoint(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// Map(10) = 166.667 → Invert(166.667) ≈ 10
	pos := s.Map(10)
	got := s.Invert(pos)
	if math.Abs(got-10) > 1e-10 {
		t.Fatalf("Invert(Map(10)) = %f, want 10", got)
	}
}

// --- round-trip ---

func TestLog_round_trip_across_decades(t *testing.T) {
	s, err := NewLog(WithDomain(1, 10000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-9
	for v := 1.0; v <= 10000; v *= 10 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%g)) = %g, want %g", v, got, v)
		}
	}
}

// --- base support ---

func TestLog_base_2(t *testing.T) {
	s, err := NewLog(WithDomain(1, 256), WithRange(0, 100), WithBase(2))
	if err != nil {
		t.Fatal(err)
	}
	// log2 domain: [0, 8]
	// Map(2): log2(2) = 1, normalize(1, 0, 8) = 1/8
	if got := s.Map(2); math.Abs(got-12.5) > 1e-10 {
		t.Fatalf("Map(2) with base 2 = %f, want 12.5", got)
	}
	// Map(16): log2(16) = 4, normalize(4, 0, 8) = 4/8 = 0.5
	if got := s.Map(16); math.Abs(got-50) > 1e-10 {
		t.Fatalf("Map(16) with base 2 = %f, want 50", got)
	}
}

func TestLog_base_e(t *testing.T) {
	s, err := NewLog(WithDomain(1, math.E*math.E), WithRange(0, 100), WithBase(math.E))
	if err != nil {
		t.Fatal(err)
	}
	// ln domain: [0, 2]
	// Map(e): ln(e) = 1, normalize(1, 0, 2) = 0.5
	if got := s.Map(math.E); math.Abs(got-50) > 1e-10 {
		t.Fatalf("Map(e) with base e = %f, want 50", got)
	}
}

// --- OutOfRange clamp ---

func TestLog_Map_clamp(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Map(0.1); got != 0 {
		t.Fatalf("Map(0.1) with clamp = %f, want 0 (range lo)", got)
	}
	if got := s.Map(1000); got != 500 {
		t.Fatalf("Map(1000) with clamp = %f, want 500 (range hi)", got)
	}
}

func TestLog_Map_extrapolate(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// Map(0.1): log(0.1) = -2.303, log(1)=0, log(100)=4.605
	// normalize(-2.303, 0, 4.605) = -0.5
	// lerp(0, 500, -0.5) = -250
	if got := s.Map(0.1); math.Abs(got+250) > 1e-10 {
		t.Fatalf("Map(0.1) extrapolate = %f, want -250", got)
	}
}

// --- negative domain (reflected log) ---

func TestLog_negative_domain_Map(t *testing.T) {
	s, err := NewLog(WithDomain(-1000, -1), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// Reflected: transform(-10) = -log(10)/log(10) = -1
	// transform(-1000) = -log(1000)/log(10) = -3
	// transform(-1) = 0
	// normalize(-1, -3, 0) = 2/3
	if got := s.Map(-10); math.Abs(got-333.3333333333333) > 1e-10 {
		t.Fatalf("Map(-10) negative domain = %f, want ~333.333", got)
	}
}

func TestLog_negative_domain_Invert(t *testing.T) {
	s, err := NewLog(WithDomain(-1000, -1), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	pos := s.Map(-10)
	got := s.Invert(pos)
	if math.Abs(got+10) > 1e-10 {
		t.Fatalf("Invert(Map(-10)) = %f, want -10", got)
	}
}

func TestLog_negative_domain_round_trip(t *testing.T) {
	s, err := NewLog(WithDomain(-10000, -1), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-9
	for _, v := range []float64{-10000, -1000, -100, -10, -1} {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%g)) = %g, want %g (diff %e)", v, got, v, math.Abs(got-v))
		}
	}
}

// --- Ticks ---

func TestLog_Ticks_decades(t *testing.T) {
	s, err := NewLog(WithDomain(1, 10000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(5)
	if len(ticks) < 4 {
		t.Fatalf("expected at least 4 decade ticks for [1,10000], got %d", len(ticks))
	}
	// Should include decade values 1, 10, 100, 1000, at minimum
	values := make(map[float64]bool)
	for _, tk := range ticks {
		values[tk.Value] = true
	}
	for _, v := range []float64{1, 10, 100, 1000, 10000} {
		if !values[v] {
			t.Errorf("missing decade tick %g", v)
		}
	}
}

func TestLog_Ticks_subdecade(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// With count=10 and only 3 decades (1,10,100,1000), sub-decade ticks should appear
	ticks := s.Ticks(10)
	if len(ticks) < 8 {
		t.Fatalf("expected sub-decade ticks, got %d: %v", len(ticks), ticks)
	}
	// Should include 2, 5, 20, 50, 200, 500
	found2, found5 := false, false
	for _, tk := range ticks {
		if tk.Value == 2 {
			found2 = true
		}
		if tk.Value == 5 {
			found5 = true
		}
	}
	if !found2 || !found5 {
		t.Fatal("expected sub-decade tick values 2 and 5")
	}
}

func TestLog_Ticks_degenerate_domain(t *testing.T) {
	s, err := NewLog(WithDomain(5, 5), WithRange(0, 100))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Ticks(5); got != nil {
		t.Fatalf("expected nil for degenerate domain, got %v", got)
	}
}

func TestLog_Ticks_zero_count(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Ticks(0); got != nil {
		t.Fatalf("expected nil for count=0, got %v", got)
	}
}

func TestLog_Ticks_monotonic(t *testing.T) {
	s, err := NewLog(WithDomain(1, 10000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(10)
	for i := 1; i < len(ticks); i++ {
		if ticks[i].Value <= ticks[i-1].Value {
			t.Fatalf("non-monotonic at [%d]: %g <= %g", i, ticks[i].Value, ticks[i-1].Value)
		}
	}
}

func TestLog_Ticks_labels_non_empty(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	for _, tk := range s.Ticks(10) {
		if tk.Label == "" {
			t.Fatal("empty label")
		}
	}
}

// --- negative domain ticks ---

func TestLog_Ticks_negative_domain(t *testing.T) {
	s, err := NewLog(WithDomain(-10000, -1), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(5)
	if len(ticks) < 4 {
		t.Fatalf("expected at least 4 decade ticks for [-10000,-1], got %d", len(ticks))
	}
	// All tick values must be within [-10000, -1] and increasing
	for _, tk := range ticks {
		if tk.Value > -1 || tk.Value < -10000 {
			t.Errorf("tick %g outside domain [-10000,-1]", tk.Value)
		}
	}
	for i := 1; i < len(ticks); i++ {
		if ticks[i].Value <= ticks[i-1].Value {
			t.Fatalf("non-monotonic at [%d]: %g <= %g", i, ticks[i].Value, ticks[i-1].Value)
		}
	}
}

// --- interface satisfaction ---

func TestLog_implements_Scale(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	var scale Scale = s
	_ = scale
}

func TestLog_implements_InvertibleScale(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	var inv InvertibleScale = s
	_ = inv
}

func TestLog_implements_Ticker(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	var tk Ticker = s
	_ = tk
}

// --- sentinel errors ---

func TestLog_sentinel_errors_distinct(t *testing.T) {
	if ErrDomainCrossesZero == ErrInvalidDomain {
		t.Fatal("ErrDomainCrossesZero must be distinct from ErrInvalidDomain")
	}
	if ErrDomainCrossesZero == nil {
		t.Fatal("ErrDomainCrossesZero must not be nil")
	}
}

// --- edge cases ---

func TestLog_Invert_degenerate_range(t *testing.T) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(42, 42))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Invert(50); got != 500.5 {
		t.Fatalf("Invert(50) with degenerate range = %f, want 500.5", got)
	}
}

func TestLog_Invert_clamp(t *testing.T) {
	s, err := NewLog(WithDomain(1, 100), WithRange(0, 500), WithClamp(OutOfRangeClamp))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Invert(-100); got != 1 {
		t.Fatalf("Invert(-100) with clamp = %f, want 1", got)
	}
	if got := s.Invert(1000); got != 100 {
		t.Fatalf("Invert(1000) with clamp = %f, want 100", got)
	}
}

func TestLog_Ticks_reversed_domain(t *testing.T) {
	s, err := NewLog(WithDomain(10000, 1), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(5)
	if len(ticks) == 0 {
		t.Fatalf("expected ticks for reversed domain, got empty")
	}
	for i := 1; i < len(ticks); i++ {
		if ticks[i].Value <= ticks[i-1].Value {
			t.Fatalf("non-monotonic at [%d]: %g <= %g", i, ticks[i].Value, ticks[i-1].Value)
		}
	}
}

func TestLog_Ticks_subdecade_filtered_by_domain(t *testing.T) {
	// Domain [1, 15]: exp 0→1, 2, 5; exp 1→10 but 20 and 50 exceed hi=15
	s, err := NewLog(WithDomain(1, 15), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(10)
	for _, tk := range ticks {
		if tk.Value < 1 || tk.Value > 15 {
			t.Errorf("tick %g outside domain [1,15]", tk.Value)
		}
	}
	// Should include 1, 2, 5, 10 (sub-decade with bound filtering)
	values := make(map[float64]bool)
	for _, tk := range ticks {
		values[tk.Value] = true
	}
	for _, v := range []float64{1, 2, 5, 10} {
		if !values[v] {
			t.Errorf("missing tick %g", v)
		}
	}
	if values[20] || values[50] {
		t.Error("sub-decade values outside domain should be filtered")
	}
}

func TestLog_Ticks_empty_when_no_decades_fit(t *testing.T) {
	// Domain [3, 7] has no decade boundary within it → empty ticks
	s, err := NewLog(WithDomain(3, 7), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Ticks(10); got != nil {
		t.Fatalf("expected nil when no decades fit, got %v", got)
	}
}

func TestLog_Ticks_only_decades(t *testing.T) {
	// Low count should produce only decade ticks, no sub-decade
	s, err := NewLog(WithDomain(1, 10000), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(3)
	// With 3 ticks and 4 decades, numDecades*3 = 12 > 3, so no sub-decade
	for _, tk := range ticks {
		v := tk.Value
		// All values should be exact powers of 10
		log10 := math.Log10(v)
		if math.Abs(log10-math.Round(log10)) > 1e-12 {
			t.Errorf("unexpected sub-decade tick %g", v)
		}
	}
}

// --- benchmarks ---

func BenchmarkLogScale_Map(b *testing.B) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Map(float64(i%1000 + 1))
	}
}

func BenchmarkLogScale_Invert(b *testing.B) {
	s, err := NewLog(WithDomain(1, 1000), WithRange(0, 500))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Invert(float64(i % 500))
	}
}

func BenchmarkLogScale_Ticks(b *testing.B) {
	s, err := NewLog(WithDomain(1, 10000), WithRange(0, 500))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Ticks(10)
	}
}
