package scale

import (
	"math"
	"testing"
)

// FuzzLinear_round_trip verifies that Invert(Map(x)) ≈ x for values within
// the domain across random finite domain/range configurations.
func FuzzLinear_round_trip(f *testing.F) {
	// seed corpus: representative configurations
	seeds := []struct {
		dLo, dHi float64
		rLo, rHi float64
	}{
		{0, 100, 0, 500},
		{-100, 100, 0, 1000},
		{0, 1, 0, 1},
		{-1e6, 1e6, -500, 500},
		{100, 0, 0, 500},   // reversed domain
		{0, 100, 500, 0},   // reversed range
		{0.001, 0.001, 0, 100}, // degenerate domain
		{0, 100, 42, 42},   // degenerate range
	}
	for _, s := range seeds {
		f.Add(s.dLo, s.dHi, s.rLo, s.rHi)
	}

	f.Fuzz(func(t *testing.T, dLo, dHi, rLo, rHi float64) {
		// Round-trip Invert(Map(x)) ≈ x only holds for non-degenerate
		// domains and ranges and finite inputs. A range narrower than
		// 1e-12 relative span is treated as degenerate (float64 precision
		// limits round-trip fidelity for near-zero-width ranges).
		if dLo == dHi ||
			math.IsNaN(dLo) || math.IsNaN(dHi) ||
			math.IsNaN(rLo) || math.IsNaN(rHi) ||
			math.IsInf(dLo, 0) || math.IsInf(dHi, 0) ||
			math.IsInf(rLo, 0) || math.IsInf(rHi, 0) {
			t.Skip()
		}
		rngMag := math.Max(math.Abs(rLo), math.Abs(rHi))
		rngMag = math.Max(rngMag, 1.0)
		if math.Abs(rHi-rLo)/rngMag < 1e-12 {
			t.Skip()
		}

		s := NewLinear(WithDomain(dLo, dHi), WithRange(rLo, rHi))

		const eps = 1e-8
		vals := []float64{dLo, (dLo + dHi) / 2, dHi}
		for _, v := range vals {
			got := s.Invert(s.Map(v))
			diff := math.Abs(got - v)
			mag := math.Max(math.Abs(got), math.Abs(v))
			if diff > eps && diff/mag > eps {
				t.Fatalf("Invert(Map(%g)) = %g, want %g (diff %g rel %g) for domain [%g,%g] range [%g,%g]",
					v, got, v, diff, diff/mag, dLo, dHi, rLo, rHi)
			}
		}
	})
}

// FuzzLinear_finite checks that Map and Invert return finite results for
// finite, in-range inputs across random scale configurations.
func FuzzLinear_finite(f *testing.F) {
	seeds := []struct {
		dLo, dHi float64
		rLo, rHi float64
	}{
		{0, 100, 0, 500},
		{-100, 100, 0, 1000},
		{0, 1e6, -1e3, 1e3},
	}
	for _, s := range seeds {
		f.Add(s.dLo, s.dHi, s.rLo, s.rHi)
	}

	f.Fuzz(func(t *testing.T, dLo, dHi, rLo, rHi float64) {
		if math.IsNaN(dLo) || math.IsNaN(dHi) ||
			math.IsNaN(rLo) || math.IsNaN(rHi) ||
			math.IsInf(dLo, 0) || math.IsInf(dHi, 0) ||
			math.IsInf(rLo, 0) || math.IsInf(rHi, 0) {
			t.Skip()
		}

		s := NewLinear(WithDomain(dLo, dHi), WithRange(rLo, rHi))

		// Map on in-domain values must be finite
		mapResult := s.Map(dLo)
		if math.IsNaN(mapResult) || math.IsInf(mapResult, 0) {
			t.Fatalf("Map(%g) = %g for domain [%g,%g] range [%g,%g]",
				dLo, mapResult, dLo, dHi, rLo, rHi)
		}

		mapResult = s.Map(dHi)
		if math.IsNaN(mapResult) || math.IsInf(mapResult, 0) {
			t.Fatalf("Map(%g) = %g for domain [%g,%g] range [%g,%g]",
				dHi, mapResult, dLo, dHi, rLo, rHi)
		}

		// Invert on in-range values must be finite
		if rLo != rHi {
			invResult := s.Invert(rLo)
			if math.IsNaN(invResult) || math.IsInf(invResult, 0) {
				t.Fatalf("Invert(%g) = %g for domain [%g,%g] range [%g,%g]",
					rLo, invResult, dLo, dHi, rLo, rHi)
			}

			invResult = s.Invert(rHi)
			if math.IsNaN(invResult) || math.IsInf(invResult, 0) {
				t.Fatalf("Invert(%g) = %g for domain [%g,%g] range [%g,%g]",
					rHi, invResult, dLo, dHi, rLo, rHi)
			}
		}
	})
}
