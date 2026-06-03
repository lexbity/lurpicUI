package scale

import (
	"math"
	"testing"
)

type scaleFactory func(lo, hi float64) (InvertibleScale, error)

func TestConformance_round_trip(t *testing.T) {
	factories := []struct {
		name string
		fn   scaleFactory
	}{
		{"linear", func(lo, hi float64) (InvertibleScale, error) {
			return NewLinear(WithDomain(lo, hi), WithRange(0, 500)), nil
		}},
		{"log", func(lo, hi float64) (InvertibleScale, error) {
			return NewLog(WithDomain(math.Abs(lo)+1, math.Abs(hi)+10), WithRange(0, 500))
		}},
		{"pow_exp2", func(lo, hi float64) (InvertibleScale, error) {
			return NewPow(WithDomain(lo, hi), WithRange(0, 500), WithExponent(2))
		}},
		{"pow_sqrt", func(lo, hi float64) (InvertibleScale, error) {
			return NewSqrt(WithDomain(lo, hi), WithRange(0, 500))
		}},
		{"time", func(lo, hi float64) (InvertibleScale, error) {
			return NewTime(WithDomain(lo, hi), WithRange(0, 500)), nil
		}},
	}

	for _, f := range factories {
		t.Run(f.name+"_round_trip", func(t *testing.T) {
			s, err := f.fn(0, 100)
			if err != nil {
				t.Fatal(err)
			}
			const eps = 1e-9
			lo, hi := s.Domain()
			for _, v := range []float64{lo, (lo + hi) / 2, hi} {
				got := s.Invert(s.Map(v))
				diff := math.Abs(got - v)
				if diff > eps {
					t.Errorf("Invert(Map(%g)) = %g, diff %g", v, got, diff)
				}
			}
		})

		t.Run(f.name+"_tick_monotonic", func(t *testing.T) {
			s, err := f.fn(0, 100)
			if err != nil {
				t.Fatal(err)
			}
			ticker, ok := s.(Ticker)
			if !ok {
				return
			}
			ticks := ticker.Ticks(10)
			for i := 1; i < len(ticks); i++ {
				if ticks[i].Value <= ticks[i-1].Value {
					t.Fatalf("non-monotonic at [%d]: %g <= %g", i, ticks[i].Value, ticks[i-1].Value)
				}
			}
		})

		t.Run(f.name+"_tick_labels", func(t *testing.T) {
			s, err := f.fn(0, 100)
			if err != nil {
				t.Fatal(err)
			}
			ticker, ok := s.(Ticker)
			if !ok {
				return
			}
			ticks := ticker.Ticks(5)
			if len(ticks) == 0 {
				t.Fatal("expected non-empty ticks")
			}
			for i, tk := range ticks {
				if tk.Label == "" {
					t.Errorf("tick[%d] has empty label", i)
				}
			}
		})

		t.Run(f.name+"_nan_propagates", func(t *testing.T) {
			s, err := f.fn(0, 100)
			if err != nil {
				t.Fatal(err)
			}
			if !math.IsNaN(s.Map(math.NaN())) {
				t.Error("Map(NaN) should return NaN")
			}
		})
	}
}

func TestConformance_band_implements_scale(t *testing.T) {
	var sc Scale = NewBand([]string{"A", "B", "C"}, WithRange(0, 300))
	_ = sc
}

func TestConformance_point_implements_scale(t *testing.T) {
	var sc Scale = NewPoint([]string{"A", "B", "C"}, WithRange(0, 300))
	_ = sc
}

func TestConformance_band_invertRange_defined(t *testing.T) {
	s := NewBand([]string{"A", "B"}, WithRange(0, 200))
	member, ok := s.InvertRange(50)
	if !ok || member != "A" {
		t.Fatalf("InvertRange(50) = (%q,%v), want (A,true)", member, ok)
	}
}

func TestConformance_point_invertRange_defined(t *testing.T) {
	s := NewPoint([]string{"A", "B"}, WithRange(0, 200))
	member, ok := s.InvertRange(50)
	if !ok || member != "A" {
		t.Fatalf("InvertRange(50) = (%q,%v), want (A,true)", member, ok)
	}
}
