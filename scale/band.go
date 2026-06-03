package scale

import "math"

// BandScale maps an ordered set of string members to contiguous bands within
// a numeric range. It implements Scale with float64 domain values
// representing member indices [0, n-1].
// Zero value: empty members, range = [0,0], no padding, align = 0.5.
type BandScale struct {
	members      []string
	rng          [2]float64
	paddingInner float64
	paddingOuter float64
	align        float64
	// precomputed
	step      float64
	bandwidth float64
	start     float64
	n         int
}

// NewBand constructs a BandScale for the given ordered member names.
func NewBand(members []string, opts ...Option) BandScale {
	var o options
	for _, opt := range opts {
		opt(&o)
	}
	s := BandScale{
		members:      members,
		rng:          o.rng,
		paddingInner: 0,
		paddingOuter: 0,
		align:        0.5,
	}
	if o.paddingInner != nil {
		s.paddingInner = clamp01(*o.paddingInner)
	}
	if o.paddingOuter != nil {
		s.paddingOuter = clamp01(*o.paddingOuter)
	}
	if o.align != nil {
		s.align = clamp01(*o.align)
	}
	s.recompute()
	return s
}

// ordinalLayout holds the precomputed geometry shared by BandScale and PointScale.
type ordinalLayout struct {
	step      float64
	bandwidth float64
	start     float64 // relative to 0 (add range[0] to get absolute offset)
}

// computeOrdinalLayout computes step, bandwidth, and relative start for a
// set of n ordinal members over a pixel span with the given padding and alignment.
func computeOrdinalLayout(n int, span, paddingInner, paddingOuter, align float64) ordinalLayout {
	if n == 0 {
		return ordinalLayout{}
	}
	denom := float64(n) - paddingInner + 2*paddingOuter
	if span == 0 || denom <= 0 {
		return ordinalLayout{start: span}
	}
	step := span / denom
	return ordinalLayout{
		step:      step,
		bandwidth: step * (1 - paddingInner),
		start:     (span - step*(float64(n)-paddingInner)) * align,
	}
}

func (s *BandScale) recompute() {
	s.n = len(s.members)
	layout := computeOrdinalLayout(s.n, s.rng[1]-s.rng[0], s.paddingInner, s.paddingOuter, s.align)
	s.step = layout.step
	s.bandwidth = layout.bandwidth
	s.start = s.rng[0] + layout.start
}

func (s BandScale) bandStart(idx int) float64 {
	return s.start + float64(idx)*s.step
}

// Band returns the start position and width of the band for member, and
// whether the member exists. For missing members ok is false.
func (s BandScale) Band(member string) (start, width float64, ok bool) {
	for i, m := range s.members {
		if m == member {
			return s.bandStart(i), s.bandwidth, true
		}
	}
	return 0, 0, false
}

// Center returns the midpoint of the band for member, and whether the member
// exists. For missing members ok is false.
func (s BandScale) Center(member string) (float64, bool) {
	for i, m := range s.members {
		if m == member {
			return s.bandStart(i) + s.bandwidth/2, true
		}
	}
	return 0, false
}

// Bandwidth returns the width of each band in pixels.
func (s BandScale) Bandwidth() float64 {
	return s.bandwidth
}

// Step returns the distance between consecutive band centers in pixels.
func (s BandScale) Step() float64 {
	return s.step
}

// InvertRange maps a pixel position to the member whose band contains it.
// Returns ok=false when the position falls in a gap or outside the range.
func (s BandScale) InvertRange(position float64) (member string, ok bool) {
	if s.n == 0 || s.step == 0 {
		return "", false
	}
	// Find the band index
	idxFloat := (position - s.start) / s.step
	idx := int(math.Floor(idxFloat))
	if idx < 0 || idx >= s.n {
		return "", false
	}
	// Check that the position is within the band (not in the gap)
	bandStart := s.bandStart(idx)
	if position < bandStart || position >= bandStart+s.bandwidth {
		return "", false
	}
	return s.members[idx], true
}

// Map converts a member index (as float64) to the band start position.
// Out-of-range indices return NaN.
func (s BandScale) Map(value float64) float64 {
	idx := int(value)
	if idx < 0 || idx >= s.n {
		return math.NaN()
	}
	return s.bandStart(idx)
}

// Domain returns the index span [0, n-1]. Returns [0,0] for empty domains.
func (s BandScale) Domain() (lo, hi float64) {
	if s.n <= 1 {
		return 0, 0
	}
	return 0, float64(s.n - 1)
}

// Range returns the output interval [lo, hi] in local pixels.
func (s BandScale) Range() (lo, hi float64) {
	return s.rng[0], s.rng[1]
}

// Kind returns KindBand.
func (s BandScale) Kind() ScaleKind {
	return KindBand
}
