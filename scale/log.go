package scale

import (
	"math"
)

// LogScale maps a continuous numeric domain to a numeric range using a
// logarithmic transform. It implements Scale, InvertibleScale, and Ticker.
// The domain must be strictly positive or strictly negative; zero-crossing
// domains are rejected at construction.
// Zero value: domain = [0,0], range = [0,0], clamp = OutOfRangeExtrapolate,
// base = 10.
type LogScale struct {
	domain [2]float64
	rng    [2]float64
	clamp  OutOfRange
	base   float64
}

// NewLog constructs a LogScale with the given options.
// The domain must be strictly positive or strictly negative; domains that
// cross or include zero return ErrDomainCrossesZero.
// Default domain is [0,0], default range is [0,0], default clamp is
// OutOfRangeExtrapolate, default base is 10.
func NewLog(opts ...Option) (LogScale, error) {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := LogScale{
		domain: o.domain,
		rng:    o.rng,
		clamp:  OutOfRangeExtrapolate,
		base:   10,
	}
	if o.clamp != nil {
		s.clamp = *o.clamp
	}
	if o.base != nil {
		s.base = *o.base
	}
	if s.base <= 0 || s.base == 1 {
		return LogScale{}, ErrInvalidDomain
	}
	if s.domain[0] != s.domain[1] {
		if (s.domain[0] <= 0 && s.domain[1] >= 0) || (s.domain[0] >= 0 && s.domain[1] <= 0) {
			return LogScale{}, ErrDomainCrossesZero
		}
	}
	return s, nil
}

// logTransform maps a domain value to log-space.
// For positive x: returns log(x)/log(base).
// For negative x: returns -log(-x)/log(base) (reflected log).
func (s LogScale) logTransform(x float64) float64 {
	if x < 0 {
		return -math.Log(-x) / math.Log(s.base)
	}
	return math.Log(x) / math.Log(s.base)
}

// logUntransform maps a log-space value back to a domain value.
// For positive t: returns exp(t * ln(base)).
// For negative t: returns -exp(-t * ln(base)).
func (s LogScale) logUntransform(t float64) float64 {
	if t < 0 {
		return -math.Exp(-t * math.Log(s.base))
	}
	return math.Exp(t * math.Log(s.base))
}

// Map converts a domain value to a range position using logarithmic scaling.
// Total: never panics. Values outside the domain are governed by the
// OutOfRange policy. For a degenerate domain the range midpoint is returned.
func (s LogScale) Map(value float64) float64 {
	if s.domain[0] == s.domain[1] {
		return lerp(s.rng[0], s.rng[1], 0.5)
	}
	tv := s.logTransform(value)
	tLo := s.logTransform(s.domain[0])
	tHi := s.logTransform(s.domain[1])
	t := normalize(tv, tLo, tHi)
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.rng[0], s.rng[1], t)
}

// Invert converts a range position back to a domain value. Total: never panics.
// For a degenerate range the domain midpoint is returned.
func (s LogScale) Invert(position float64) float64 {
	if s.rng[0] == s.rng[1] {
		return lerp(s.domain[0], s.domain[1], 0.5)
	}
	tLo := s.logTransform(s.domain[0])
	tHi := s.logTransform(s.domain[1])
	t := normalize(position, s.rng[0], s.rng[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	tv := lerp(tLo, tHi, t)
	// When the transformed value matches a domain endpoint, return the
	// exact domain value to avoid sign ambiguity in logUntransform(t=0)
	// for pure negative domains.
	if tv == tLo {
		return s.domain[0]
	}
	if tv == tHi {
		return s.domain[1]
	}
	return s.logUntransform(tv)
}

// Domain returns the input interval [lo, hi].
func (s LogScale) Domain() (lo, hi float64) {
	return s.domain[0], s.domain[1]
}

// Range returns the output interval [lo, hi] in local pixels.
func (s LogScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Ticks returns approximately count tick values spanning the scale's domain.
// Ticks land on decade boundaries (e.g. 1, 10, 100) and, when the requested
// count is large enough, on sub-decade marks (2, 5).
func (s LogScale) Ticks(count int) []Tick {
	if count <= 0 {
		return nil
	}
	lo, hi := s.domain[0], s.domain[1]
	if lo == hi {
		return nil
	}
	if lo > hi {
		lo, hi = hi, lo
	}

	var vals []float64
	if lo > 0 {
		vals = logTicks(lo, hi, s.base, count)
	} else if hi < 0 {
		posVals := logTicks(-hi, -lo, s.base, count)
		vals = make([]float64, len(posVals))
		for i, v := range posVals {
			vals[len(vals)-1-i] = -v
		}
	}
	return tickLabels(vals)
}

// Kind returns KindLog.
func (s LogScale) Kind() ScaleKind {
	return KindLog
}

// logTicks generates tick values for a positive-only log domain.
func logTicks(lo, hi float64, base float64, count int) []float64 {
	logBase := math.Log(base)
	startExp := int(math.Ceil(math.Log(lo) / logBase))
	endExp := int(math.Floor(math.Log(hi) / logBase))
	if startExp > endExp {
		return nil
	}

	numDecades := endExp - startExp + 1
	includeSub := count > numDecades

	var vals []float64
	for exp := startExp; exp <= endExp; exp++ {
		d := math.Pow(base, float64(exp))
		if d >= lo && d <= hi {
			vals = append(vals, d)
		}
		if includeSub {
			for _, m := range []float64{2, 5} {
				v := m * d
				if v >= lo && v <= hi {
					vals = append(vals, v)
				}
			}
		}
	}
	return vals
}
