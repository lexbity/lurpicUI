package scale

import "math"

// PointScale maps an ordered set of string members to evenly-spaced point
// positions within a numeric range. It implements Scale with float64 domain
// values representing member indices [0, n-1].
// Zero value: empty members, range = [0,0], no padding, align = 0.5.
type PointScale struct {
	members []string
	rng     [2]float64
	padding float64
	align   float64
	// precomputed
	step  float64
	start float64
	n     int
}

// NewPoint constructs a PointScale for the given ordered member names.
// Positions are placed at the center of each band using inner padding = 1,
// so bandwidth is always 0 and points are evenly spaced.
func NewPoint(members []string, opts ...Option) PointScale {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := PointScale{
		members: members,
		rng:     o.rng,
		padding: 0,
		align:   0.5,
	}
	if o.paddingOuter != nil {
		s.padding = clamp01(*o.paddingOuter)
	}
	if o.align != nil {
		s.align = clamp01(*o.align)
	}
	s.recompute()
	return s
}

func (s *PointScale) recompute() {
	s.n = len(s.members)
	if s.n <= 1 {
		if s.n == 1 {
			s.start = lerp(s.rng[0], s.rng[1], s.align)
		}
		s.step = 0
		return
	}
	layout := computeOrdinalLayout(s.n, s.rng[1]-s.rng[0], 1, s.padding, s.align)
	s.step = layout.step
	s.start = s.rng[0] + layout.start
}

// Position returns the pixel position of the given member, and whether the
// member exists. For missing members ok is false.
func (s PointScale) Position(member string) (float64, bool) {
	for i, m := range s.members {
		if m == member {
			return s.start + float64(i)*s.step, true
		}
	}
	return 0, false
}

// Step returns the distance between consecutive point positions in pixels.
func (s PointScale) Step() float64 {
	return s.step
}

// InvertRange maps a pixel position to the nearest member. Always returns
// the closest member (unlike BandScale, there are no gaps).
func (s PointScale) InvertRange(position float64) (member string, ok bool) {
	if s.n == 0 || s.step == 0 {
		return "", false
	}
	idxFloat := (position - s.start) / s.step
	idx := int(math.Round(idxFloat))
	if idx < 0 || idx >= s.n {
		return "", false
	}
	return s.members[idx], true
}

// Map converts a member index (as float64) to the point position.
// Out-of-range indices return NaN.
func (s PointScale) Map(value float64) float64 {
	idx := int(value)
	if idx < 0 || idx >= s.n {
		return math.NaN()
	}
	return s.start + float64(idx)*s.step
}

// Domain returns the index span [0, n-1]. Returns [0,0] for empty domains.
func (s PointScale) Domain() (lo, hi float64) {
	if s.n <= 1 {
		return 0, 0
	}
	return 0, float64(s.n - 1)
}

// Range returns the output interval [lo, hi] in local pixels.
func (s PointScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Kind returns KindPoint.
func (s PointScale) Kind() ScaleKind {
	return KindPoint
}
