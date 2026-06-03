package scale

import (
	"math"
	"testing"
)

// --- construction ---

func TestPow_new_default_exponent_is_one(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// exp=1 ⇒ linear scale behavior
	if got := s.Map(50); got != 250 {
		t.Fatalf("Map(50) with default exp=1 = %f, want 250", got)
	}
	if s.Kind() != KindPow {
		t.Fatalf("kind = %s, want KindPow", s.Kind())
	}
}

func TestPow_new_invalid_exponent(t *testing.T) {
	_, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(0))
	if err != ErrInvalidDomain {
		t.Fatalf("expected ErrInvalidDomain for exp=0, got %v", err)
	}
	_, err = NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(-2))
	if err != ErrInvalidDomain {
		t.Fatalf("expected ErrInvalidDomain for exp=-2, got %v", err)
	}
}

func TestNewSqrt_convenience(t *testing.T) {
	s, err := NewSqrt(WithDomain(0, 100), WithRange(0, 500))
	if err != nil {
		t.Fatal(err)
	}
	// sqrt(100) = 10, sqrt(0) = 0
	// transformed: [0, 10]
	// Map(25): sqrt(25)=5, normalize(5,0,10)=0.5 ⇒ 250
	if got := s.Map(25); got != 250 {
		t.Fatalf("NewSqrt Map(25) = %f, want 250", got)
	}
}

// --- Map ---

func TestPow_Map_exponent2(t *testing.T) {
	s, err := NewPow(WithDomain(0, 10), WithRange(0, 100), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	// transformed: [0, 100]
	// Map(0): pow(0,2)=0, normalize(0,0,100)=0 ⇒ 0
	if got := s.Map(0); got != 0 {
		t.Fatalf("Map(0) exp=2 = %f, want 0", got)
	}
	// Map(10): pow(10,2)=100, normalize(100,0,100)=1 ⇒ 100
	if got := s.Map(10); got != 100 {
		t.Fatalf("Map(10) exp=2 = %f, want 100", got)
	}
	// Map(5): pow(5,2)=25, normalize(25,0,100)=0.25 ⇒ 25
	if got := s.Map(5); got != 25 {
		t.Fatalf("Map(5) exp=2 = %f, want 25", got)
	}
}

func TestPow_Map_exponent0_5(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(0.5))
	if err != nil {
		t.Fatal(err)
	}
	// sqrt(0)=0, sqrt(100)=10, transformed: [0, 10]
	// Map(25): sqrt(25)=5, normalize(5,0,10)=0.5 ⇒ 250
	if got := s.Map(25); got != 250 {
		t.Fatalf("Map(25) exp=0.5 = %f, want 250", got)
	}
	// Map(100): sqrt(100)=10, normalize(10,0,10)=1 ⇒ 500
	if got := s.Map(100); got != 500 {
		t.Fatalf("Map(100) exp=0.5 = %f, want 500", got)
	}
}

func TestPow_Map_exponent3(t *testing.T) {
	s, err := NewPow(WithDomain(-3, 3), WithRange(0, 100), WithExponent(3))
	if err != nil {
		t.Fatal(err)
	}
	// pow(-3,3)=-27, pow(3,3)=27, transformed: [-27, 27]
	// Map(-3): normalize(-27,-27,27)=0 ⇒ 0
	if got := s.Map(-3); got != 0 {
		t.Fatalf("Map(-3) exp=3 = %f, want 0", got)
	}
	// Map(3): normalize(27,-27,27)=1 ⇒ 100
	if got := s.Map(3); got != 100 {
		t.Fatalf("Map(3) exp=3 = %f, want 100", got)
	}
	// Map(0): pow(0,3)=0, normalize(0,-27,27)=0.5 ⇒ 50
	if got := s.Map(0); math.Abs(got-50) > 1e-12 {
		t.Fatalf("Map(0) exp=3 = %f, want 50", got)
	}
}

func TestPow_Map_degenerate_domain(t *testing.T) {
	s, err := NewPow(WithDomain(5, 5), WithRange(0, 100), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Map(5); got != 50 {
		t.Fatalf("Map(5) degenerate domain = %f, want 50", got)
	}
}

func TestPow_Map_nan(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	if !math.IsNaN(s.Map(math.NaN())) {
		t.Fatal("Map(NaN) should return NaN")
	}
}

// --- Invert ---

func TestPow_Invert_exponent2(t *testing.T) {
	s, err := NewPow(WithDomain(0, 10), WithRange(0, 100), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-12
	if got := s.Invert(0); math.Abs(got) > eps {
		t.Fatalf("Invert(0) exp=2 = %f, want 0", got)
	}
	if got := s.Invert(100); math.Abs(got-10) > eps {
		t.Fatalf("Invert(100) exp=2 = %f, want 10", got)
	}
}

func TestPow_Invert_degenerate_range(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(42, 42), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Invert(50); got != 50 {
		t.Fatalf("Invert(50) degenerate range = %f, want 50", got)
	}
}

// --- sign-preserving (negative values) ---

func TestPow_negative_domain_exp2(t *testing.T) {
	s, err := NewPow(WithDomain(-10, 10), WithRange(0, 100), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	// pow(-10,2)=100, pow(10,2)=100, transformed: [100, 100] → degenerate!
	// Wait — sign-preserving: -pow(10,2) = -100, pow(10,2)=100 → [-100, 100]
	// Map(-10): pow(-10,-2) = -100, normalize(-100,-100,100)=0 ⇒ 0
	if got := s.Map(-10); got != 0 {
		t.Fatalf("Map(-10) exp=2 = %f, want 0", got)
	}
	// Map(10): pow(10,2)=100, normalize(100,-100,100)=1 ⇒ 100
	if got := s.Map(10); got != 100 {
		t.Fatalf("Map(10) exp=2 = %f, want 100", got)
	}
	// Map(0): pow(0,2)=0, normalize(0,-100,100)=0.5 ⇒ 50
	if got := s.Map(0); math.Abs(got-50) > 1e-12 {
		t.Fatalf("Map(0) exp=2 = %f, want 50", got)
	}
}

func TestPow_negative_domain_exp3(t *testing.T) {
	s, err := NewPow(WithDomain(-8, 8), WithRange(0, 100), WithExponent(3))
	if err != nil {
		t.Fatal(err)
	}
	// pow(-8,3) = -512, pow(8,3) = 512
	// Map(-8): normalize(-512,-512,512)=0 ⇒ 0
	if got := s.Map(-8); got != 0 {
		t.Fatalf("Map(-8) exp=3 = %f, want 0", got)
	}
	// Map(1): pow(1,3)=1, normalize(1,-512,512) ≈ 0.501 → lerp ≈ 50.1
	got := s.Map(1)
	if got <= 0 || got >= 100 {
		t.Fatalf("Map(1) exp=3 = %f, want between 0 and 100", got)
	}
}

func TestPow_negative_sign_preserving_round_trip(t *testing.T) {
	s, err := NewPow(WithDomain(-8, 8), WithRange(0, 500), WithExponent(3))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-9
	for _, v := range []float64{-8, -1, 0, 1, 8} {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%g)) exp=3 = %g, want %g", v, got, v)
		}
	}
}

// --- round-trip ---

func TestPow_round_trip_exponent2(t *testing.T) {
	s, err := NewPow(WithDomain(0, 10), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-9
	for v := 0.0; v <= 10; v += 1 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%g)) exp=2 = %g, want %g (diff %e)", v, got, v, math.Abs(got-v))
		}
	}
}

func TestPow_round_trip_exponent0_5(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(0.5))
	if err != nil {
		t.Fatal(err)
	}
	const eps = 1e-9
	for v := 0.0; v <= 100; v += 10 {
		got := s.Invert(s.Map(v))
		if math.Abs(got-v) > eps {
			t.Errorf("Invert(Map(%g)) exp=0.5 = %g, want %g", v, got, v)
		}
	}
}

// --- Ticks ---

func TestPow_Ticks_reuses_linear(t *testing.T) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	ticks := s.Ticks(5)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks")
	}
	// Ticks from linear algorithm on domain [0,100] with count=5 → [0,20,40,60,80,100]
	if len(ticks) != 6 {
		t.Fatalf("expected 6 ticks, got %d: %v", len(ticks), ticks)
	}
	expected := []float64{0, 20, 40, 60, 80, 100}
	for i, tk := range ticks {
		if tk.Value != expected[i] {
			t.Errorf("tick[%d] = %g, want %g", i, tk.Value, expected[i])
		}
		if tk.Label == "" {
			t.Errorf("tick[%d] has empty label", i)
		}
	}
}

func TestPow_Ticks_degenerate_domain(t *testing.T) {
	s, err := NewPow(WithDomain(5, 5), WithRange(0, 100), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Ticks(5); got != nil {
		t.Fatalf("expected nil for degenerate domain, got %v", got)
	}
}

// --- accessors ---

func TestPow_Domain_Range(t *testing.T) {
	s, err := NewPow(WithDomain(10, 200), WithRange(50, 300), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	lo, hi := s.Domain()
	if lo != 10 || hi != 200 {
		t.Fatalf("Domain = (%g,%g), want (10,200)", lo, hi)
	}
	lo, hi = s.Range()
	if lo != 50 || hi != 300 {
		t.Fatalf("Range = (%g,%g), want (50,300)", lo, hi)
	}
}

func TestPow_Map_clamp(t *testing.T) {
	s, err := NewPow(WithDomain(0, 10), WithRange(0, 100), WithExponent(2), WithClamp(OutOfRangeClamp))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Map(-5); got != 0 {
		t.Fatalf("Map(-5) with clamp = %f, want 0", got)
	}
	if got := s.Map(15); got != 100 {
		t.Fatalf("Map(15) with clamp = %f, want 100", got)
	}
}

func TestPow_Invert_clamp(t *testing.T) {
	s, err := NewPow(WithDomain(0, 10), WithRange(0, 100), WithExponent(2), WithClamp(OutOfRangeClamp))
	if err != nil {
		t.Fatal(err)
	}
	if got := s.Invert(-50); got != 0 {
		t.Fatalf("Invert(-50) with clamp = %f, want 0", got)
	}
	if got := s.Invert(200); got != 10 {
		t.Fatalf("Invert(200) with clamp = %f, want 10", got)
	}
}

// --- interface satisfaction ---

func TestPow_implements_Scale(t *testing.T) {
	var scale Scale
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	scale = s
	_ = scale
}

func TestPow_implements_InvertibleScale(t *testing.T) {
	var inv InvertibleScale
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	inv = s
	_ = inv
}

func TestPow_implements_Ticker(t *testing.T) {
	var tk Ticker
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		t.Fatal(err)
	}
	tk = s
	_ = tk
}

// --- benchmarks ---

func BenchmarkPowScale_Map_exp2(b *testing.B) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Map(float64(i % 100))
	}
}

func BenchmarkPowScale_Invert_exp2(b *testing.B) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Invert(float64(i % 500))
	}
}

func BenchmarkPowScale_Ticks(b *testing.B) {
	s, err := NewPow(WithDomain(0, 100), WithRange(0, 500), WithExponent(2))
	if err != nil {
		b.Fatal(err)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Ticks(10)
	}
}
