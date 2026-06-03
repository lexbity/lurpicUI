package scale

import (
	"math"
	"testing"
	"time"
)

func TestTime_Map_endpoints(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(50, 200))

	ms0 := float64(t0.UnixMilli())
	ms1 := float64(t1.UnixMilli())

	if got := s.Map(ms0); got != 50 {
		t.Fatalf("Map(domain[0]) = %f, want 50", got)
	}
	if got := s.Map(ms1); got != 200 {
		t.Fatalf("Map(domain[1]) = %f, want 200", got)
	}
}

func TestTime_Invert_endpoints(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(50, 200))

	ms0 := float64(t0.UnixMilli())
	ms1 := float64(t1.UnixMilli())

	const eps = 1e-9
	if got := s.Invert(50); math.Abs(got-ms0) > eps {
		t.Fatalf("Invert(50) = %f, want %f", got, ms0)
	}
	if got := s.Invert(200); math.Abs(got-ms1) > eps {
		t.Fatalf("Invert(200) = %f, want %f", got, ms1)
	}
}

func TestTime_round_trip_millisecond_exact(t *testing.T) {
	// Round-trip across multiple decades must preserve millisecond precision.
	t0 := time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2050, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 1000))

	for year := 1995; year <= 2045; year += 5 {
		tm := time.Date(year, 6, 15, 12, 30, 45, 123_456_789, time.UTC)
		ms := float64(tm.UnixMilli())

		mapped := s.Map(ms)
		got := s.Invert(mapped)

		// Must be within 1ms across the full 60-year span
		diff := math.Abs(got - ms)
		if diff > 1.0 {
			t.Errorf("round-trip error for %v: %.2f ms (got %.0f, want %.0f)",
				tm, diff, got, ms)
		}
	}
}

func TestTime_round_trip_epoch_boundary(t *testing.T) {
	// Test near the Unix epoch (1970) where values are small.
	t0 := time.Date(1969, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500))

	for _, tm := range []time.Time{
		time.Date(1970, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(1970, 6, 15, 12, 0, 0, 0, time.UTC),
		time.Date(1969, 12, 31, 23, 59, 59, 0, time.UTC),
	} {
		ms := float64(tm.UnixMilli())
		got := s.Invert(s.Map(ms))
		diff := math.Abs(got - ms)
		if diff > 1.0 {
			t.Errorf("round-trip error for %v: %.2f ms", tm, diff)
		}
	}
}

func TestTime_reversed_domain(t *testing.T) {
	t0 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC) // earlier
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500))

	ms0 := float64(t0.UnixMilli())
	ms1 := float64(t1.UnixMilli())

	// Reversed domain: earlier time maps to range[1], later time to range[0]
	if got := s.Map(ms0); got != 0 {
		t.Fatalf("Map(later) with reversed domain = %f, want 0", got)
	}
	if got := s.Map(ms1); got != 500 {
		t.Fatalf("Map(earlier) with reversed domain = %f, want 500", got)
	}
}

func TestTime_float32_loses_precision(t *testing.T) {
	// Demonstrates why the engine uses float64 internally for scale
	// arithmetic (P2). A modern Unix timestamp in milliseconds exceeds
	// float32's exact-integer range (2^24 ≈ 16.7 million).
	modern := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	ms := modern.UnixMilli() // ≈ 1.7 trillion — far beyond 16 million

	ms64 := float64(ms)
	ms32 := float32(ms64)

	// float32 cannot represent this value exactly
	if float64(ms32) == ms64 {
		t.Fatal("float32 unexpectedly preserved millisecond precision")
	}
	loss := ms64 - float64(ms32)
	t.Logf("modern timestamp: %d ms", ms)
	t.Logf("float64:          %.0f", ms64)
	t.Logf("float32:          %.0f", float64(ms32))
	t.Logf("precision loss:   %.0f ms", loss)
}

func TestTime_degenerate_domain(t *testing.T) {
	tm := time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(tm, tm), WithRange(0, 100))
	if got := s.Map(float64(tm.UnixMilli())); got != 50 {
		t.Fatalf("Map with degenerate domain = %f, want 50", got)
	}
}

func TestTime_degenerate_range(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(42, 42))
	msMid := float64(t0.UnixMilli()+t1.UnixMilli()) / 2
	if got := s.Invert(50); math.Abs(got-msMid) > 1 {
		t.Fatalf("Invert with degenerate range = %f, want %f", got, msMid)
	}
}

func TestTime_clamp(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500), WithClamp(OutOfRangeClamp))

	// Before domain
	early := float64(time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	if got := s.Map(early); got != 0 {
		t.Fatalf("Map pre-domain with clamp = %f, want 0", got)
	}

	// After domain
	late := float64(time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC).UnixMilli())
	if got := s.Map(late); got != 500 {
		t.Fatalf("Map post-domain with clamp = %f, want 500", got)
	}
}

func TestTime_Invert_clamp(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500), WithClamp(OutOfRangeClamp))

	// Invert a position outside the range — should return the nearest domain endpoint
	ms0 := float64(t0.UnixMilli())
	ms1 := float64(t1.UnixMilli())

	if got := s.Invert(-100); math.Abs(got-ms0) > 1 {
		t.Fatalf("Invert(-100) with clamp = %f, want %f", got, ms0)
	}
	if got := s.Invert(1000); math.Abs(got-ms1) > 1 {
		t.Fatalf("Invert(1000) with clamp = %f, want %f", got, ms1)
	}
}

func TestTime_nan(t *testing.T) {
	s := NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	if !math.IsNaN(s.Map(math.NaN())) {
		t.Fatal("Map(NaN) should return NaN")
	}
	if !math.IsNaN(s.Invert(math.NaN())) {
		t.Fatal("Invert(NaN) should return NaN")
	}
}

func TestTime_Domain_Range(t *testing.T) {
	t0 := time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC)
	t1 := time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(50, 200))

	lo, hi := s.Domain()
	if lo != float64(t0.UnixMilli()) || hi != float64(t1.UnixMilli()) {
		t.Fatalf("Domain = (%f,%f)", lo, hi)
	}
	lo, hi = s.Range()
	if lo != 50 || hi != 200 {
		t.Fatalf("Range = (%f,%f)", lo, hi)
	}
}

func TestTime_kind(t *testing.T) {
	s := NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	if s.Kind() != KindTime {
		t.Fatalf("Kind = %s, want KindTime", s.Kind())
	}
}

func TestTime_implements_Scale(t *testing.T) {
	var sc Scale = NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	_ = sc
}

func TestTime_with_timezone_option(t *testing.T) {
	loc := time.FixedZone("EST", -5*60*60)
	t0 := time.Date(2024, 3, 10, 0, 0, 0, 0, loc)
	t1 := time.Date(2024, 3, 11, 0, 0, 0, 0, loc)
	s := NewTime(WithTimeDomain(t0, t1), WithRange(0, 500), WithTimeLocation(loc))
	ticks := s.Ticks(12)
	if len(ticks) == 0 {
		t.Fatal("expected ticks with timezone option")
	}
	// Ticks should be in the given location (EST, UTC-5)
	for _, tk := range ticks {
		tm := time.UnixMilli(int64(tk.Value))
		_, offset := tm.In(loc).Zone()
		if offset != -5*60*60 {
			t.Errorf("tick %v not in EST timezone", tm)
			break
		}
	}
}

func TestTime_implements_InvertibleScale(t *testing.T) {
	var inv InvertibleScale = NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2010, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	_ = inv
}

// --- benchmarks ---

func BenchmarkTimeScale_Map(b *testing.B) {
	s := NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	ms := float64(time.Date(2010, 6, 15, 12, 0, 0, 0, time.UTC).UnixMilli())
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Map(ms)
	}
}

func BenchmarkTimeScale_Invert(b *testing.B) {
	s := NewTime(WithTimeDomain(
		time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
		time.Date(2020, 1, 1, 0, 0, 0, 0, time.UTC),
	), WithRange(0, 500))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		s.Invert(250)
	}
}
