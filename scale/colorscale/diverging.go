package colorscale

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// DivergingColor maps a numeric domain to a two-sided color ramp that meets
// at a configurable midpoint. Values below the midpoint use the low ramp;
// values above use the high ramp.
type DivergingColor struct {
	domain   [2]float64
	midpoint float64
	rampLow  Ramp
	rampHigh Ramp
	space    InterpolationSpace
	clamp    bool
}

// NewDiverging constructs a DivergingColor. Values in [lo, midpoint] are
// mapped through rampLow; values in [midpoint, hi] through rampHigh.
// rampLow's last stop and rampHigh's first stop should match for continuity
// at the midpoint.
func NewDiverging(lo, hi, midpoint float64, rampLow, rampHigh Ramp, space InterpolationSpace) DivergingColor {
	return DivergingColor{
		domain:   [2]float64{lo, hi},
		midpoint: midpoint,
		rampLow:  rampLow,
		rampHigh: rampHigh,
		space:    space,
	}
}

// WithClamp returns a copy that clamps out-of-domain values.
func (s DivergingColor) WithClamp() DivergingColor {
	s.clamp = true
	return s
}

// Map converts a domain value to a color.
// Values below the midpoint use the low ramp; above use the high ramp.
func (s DivergingColor) Map(value float64) gfx.Color {
	midColor := s.rampHigh.At(0, s.space)

	if math.IsNaN(value) || math.IsInf(value, 0) {
		if s.clamp {
			if value < s.midpoint || math.IsNaN(value) {
				return s.rampLow.At(0, s.space)
			}
			return s.rampHigh.At(1, s.space)
		}
		return midColor
	}

	if value < s.midpoint {
		if s.domain[0] == s.midpoint {
			return s.rampLow.At(0, s.space)
		}
		t := (value - s.domain[0]) / (s.midpoint - s.domain[0])
		if s.clamp {
			t = clamp01(t)
		}
		return s.rampLow.At(t, s.space)
	}

	if s.midpoint == s.domain[1] {
		return s.rampHigh.At(0, s.space)
	}
	t := (value - s.midpoint) / (s.domain[1] - s.midpoint)
	if s.clamp {
		t = clamp01(t)
	}
	return s.rampHigh.At(t, s.space)
}
