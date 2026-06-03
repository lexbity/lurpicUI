package scale

import (
	"math"
)

// PowScale maps a continuous numeric domain to a numeric range using a
// power transform with a configurable exponent. It implements Scale,
// InvertibleScale, and Ticker.
// Zero value: domain = [0,0], range = [0,0], clamp = OutOfRangeExtrapolate,
// exponent = 1 (linear).
type PowScale struct {
	domain   [2]float64
	rng      [2]float64
	clamp    OutOfRange
	exponent float64
}

// NewPow constructs a PowScale with the given options.
// The exponent defaults to 1 (linear) unless overridden via WithExponent.
// The exponent must be positive; zero or negative exponents return an error.
func NewPow(opts ...Option) (PowScale, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := PowScale{
		domain:   o.domain,
		rng:      o.rng,
		clamp:    OutOfRangeExtrapolate,
		exponent: 1,
	}
	if o.clamp != nil {
		s.clamp = *o.clamp
	}
	if o.exponent != nil {
		s.exponent = *o.exponent
	}
	if s.exponent <= 0 {
		return PowScale{}, ErrInvalidDomain
	}
	return s, nil
}

// NewSqrt constructs a PowScale with exponent 0.5, the common area-to-radius
// encoding scale. Additional options override the defaults.
func NewSqrt(opts ...Option) (PowScale, error) {
	opts = append([]Option{WithExponent(0.5)}, opts...)
	return NewPow(opts...)
}

// powTransform maps x to sign-preserving x^exp.
func (s PowScale) powTransform(x float64) float64 {
	if x >= 0 {
		return math.Pow(x, s.exponent)
	}
	return -math.Pow(-x, s.exponent)
}

// powUntransform maps t (the transformed value) back to data space via
// sign-preserving t^(1/exp).
func (s PowScale) powUntransform(t float64) float64 {
	if t >= 0 {
		return math.Pow(t, 1.0/s.exponent)
	}
	return -math.Pow(-t, 1.0/s.exponent)
}

// Map converts a domain value to a range position using the power transform.
// Total: never panics. For a degenerate domain the range midpoint is returned.
func (s PowScale) Map(value float64) float64 {
	if s.domain[0] == s.domain[1] {
		return lerp(s.rng[0], s.rng[1], 0.5)
	}
	tv := s.powTransform(value)
	tLo := s.powTransform(s.domain[0])
	tHi := s.powTransform(s.domain[1])
	t := normalize(tv, tLo, tHi)
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.rng[0], s.rng[1], t)
}

// Invert converts a range position back to a domain value. Total: never panics.
// For a degenerate range the domain midpoint is returned.
func (s PowScale) Invert(position float64) float64 {
	if s.rng[0] == s.rng[1] {
		return lerp(s.domain[0], s.domain[1], 0.5)
	}
	tLo := s.powTransform(s.domain[0])
	tHi := s.powTransform(s.domain[1])
	t := normalize(position, s.rng[0], s.rng[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	tv := lerp(tLo, tHi, t)
	// Return exact domain endpoint when transformed value matches to
	// avoid sign ambiguity in powUntransform at t=0 for mixed-sign domains.
	if tv == tLo {
		return s.domain[0]
	}
	if tv == tHi {
		return s.domain[1]
	}
	return s.powUntransform(tv)
}

// Domain returns the input interval [lo, hi].
func (s PowScale) Domain() (lo, hi float64) {
	return s.domain[0], s.domain[1]
}

// Range returns the output interval [lo, hi] in local pixels.
func (s PowScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Ticks returns approximately count tick values spanning the scale's domain.
// Tick values are computed in data space using the linear 1/2/5 step algorithm.
func (s PowScale) Ticks(count int) []Tick {
	return tickLabels(ticks(s.domain[0], s.domain[1], count))
}

// Kind returns KindPow.
func (s PowScale) Kind() ScaleKind {
	return KindPow
}
