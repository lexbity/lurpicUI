package scale

// LinearScale maps a continuous numeric domain to a numeric range using
// linear interpolation. It implements Scale and InvertibleScale.
// Zero value: domain = [0,0], range = [0,0], clamp = OutOfRangeExtrapolate.
type LinearScale struct {
	domain [2]float64
	rng    [2]float64
	clamp  OutOfRange
}

// NewLinear constructs a LinearScale with the given options.
// Default domain is [0,0], default range is [0,0], default clamp is
// OutOfRangeExtrapolate.
func NewLinear(opts ...Option) LinearScale {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := LinearScale{
		domain: o.domain,
		rng:    o.rng,
		clamp:  OutOfRangeExtrapolate,
	}
	if o.clamp != nil {
		s.clamp = *o.clamp
	}
	return s
}

// Map converts a domain value to a range position. Total: never panics.
// For a degenerate domain (lo == hi) the range midpoint is returned.
// For an out-of-domain value the behavior depends on the OutOfRange policy:
// extrapolate (default) projects the linear trend; clamp returns the nearest
// range endpoint. NaN and Inf inputs propagate.
func (s LinearScale) Map(value float64) float64 {
	if s.domain[0] == s.domain[1] {
		return lerp(s.rng[0], s.rng[1], 0.5)
	}
	t := normalize(value, s.domain[0], s.domain[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.rng[0], s.rng[1], t)
}

// Invert converts a range position back to a domain value. Total: never panics.
// For a degenerate range (lo == hi) the domain midpoint is returned.
// When the OutOfRange policy is clamp, positions outside the range are treated
// as if they were at the nearest range endpoint. NaN and Inf inputs propagate.
func (s LinearScale) Invert(position float64) float64 {
	if s.rng[0] == s.rng[1] {
		return lerp(s.domain[0], s.domain[1], 0.5)
	}
	t := normalize(position, s.rng[0], s.rng[1])
	if s.clamp == OutOfRangeClamp {
		t = clamp01(t)
	}
	return lerp(s.domain[0], s.domain[1], t)
}

// Domain returns the input interval [lo, hi].
func (s LinearScale) Domain() (lo, hi float64) {
	return s.domain[0], s.domain[1]
}

// Range returns the output interval [lo, hi] in local pixels.
func (s LinearScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Ticks returns approximately count tick values spanning the scale's domain,
// formatted as Tick structs. The step size uses the 1/2/5 mantissa algorithm.
// Degenerate domains and non-positive counts return nil.
func (s LinearScale) Ticks(count int) []Tick {
	return tickLabels(ticks(s.domain[0], s.domain[1], count))
}

// Kind returns KindLinear.
func (s LinearScale) Kind() ScaleKind {
	return KindLinear
}
