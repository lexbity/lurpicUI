package scale

import "time"

// TimeScale maps a domain of Unix-millisecond timestamps (float64) to a
// numeric range using linear interpolation. It implements Scale and
// InvertibleScale. The domain is stored as float64 Unix milliseconds and
// delegates the core mapping to the same linear primitives as LinearScale.
// Zero value: domain = [0,0], range = [0,0], clamp = OutOfRangeExtrapolate.
type TimeScale struct {
	domain [2]float64
	rng    [2]float64
	clamp  OutOfRange
}

// NewTime constructs a TimeScale with the given options. The domain values
// are Unix-millisecond timestamps (float64). Use WithTimeDomain for the
// convenient time.Time-based constructor.
func NewTime(opts ...Option) TimeScale {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := TimeScale{
		domain: o.domain,
		rng:    o.rng,
		clamp:  OutOfRangeExtrapolate,
	}
	if o.clamp != nil {
		s.clamp = *o.clamp
	}
	return s
}

// WithTimeDomain sets the scale's input domain as time.Time values,
// converting them to Unix-millisecond float64 internally.
func WithTimeDomain(t0, t1 time.Time) Option {
	return WithDomain(float64(t0.UnixMilli()), float64(t1.UnixMilli()))
}

// Map converts a Unix-millisecond timestamp (as float64) to a range position.
// Total: never panics. For a degenerate domain the range midpoint is returned.
func (s TimeScale) Map(value float64) float64 {
	if s.domain[0] == s.domain[1] {
		return lerp(s.rng[0], s.rng[1], 0.5)
	}
	t := normalize(value, s.domain[0], s.domain[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.rng[0], s.rng[1], t)
}

// Invert converts a range position back to a Unix-millisecond timestamp (as
// float64). Total: never panics. For a degenerate range the domain midpoint
// is returned.
func (s TimeScale) Invert(position float64) float64 {
	if s.rng[0] == s.rng[1] {
		return lerp(s.domain[0], s.domain[1], 0.5)
	}
	t := normalize(position, s.rng[0], s.rng[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.domain[0], s.domain[1], t)
}

// Domain returns the input interval [lo, hi] in Unix milliseconds.
func (s TimeScale) Domain() (lo, hi float64) {
	return s.domain[0], s.domain[1]
}

// Range returns the output interval [lo, hi] in local pixels.
func (s TimeScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Kind returns KindTime.
func (s TimeScale) Kind() ScaleKind {
	return KindTime
}
