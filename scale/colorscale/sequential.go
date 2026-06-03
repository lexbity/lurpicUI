package colorscale

import (
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// SequentialColor maps a numeric domain to a continuous color gradient using
// a configured ramp and interpolation space.
type SequentialColor struct {
	domain [2]float64
	ramp   Ramp
	space  InterpolationSpace
	clamp  bool
}

// NewSequential constructs a SequentialColor that maps values in [lo, hi]
// through the given ramp.
func NewSequential(lo, hi float64, ramp Ramp, space InterpolationSpace) SequentialColor {
	return SequentialColor{
		domain: [2]float64{lo, hi},
		ramp:   ramp,
		space:  space,
	}
}

// WithClamp returns a copy of the scale that clamps out-of-domain values
// to the nearest ramp endpoint.
func (s SequentialColor) WithClamp() SequentialColor {
	s.clamp = true
	return s
}

// Map converts a domain value to a color. Out-of-domain values are
// extrapolated from the ramp endpoints unless WithClamp was called.
func (s SequentialColor) Map(value float64) gfx.Color {
	t := float64(0)
	if s.domain[1] != s.domain[0] {
		t = (value - s.domain[0]) / (s.domain[1] - s.domain[0])
	}
	if s.clamp || math.IsNaN(t) || math.IsInf(t, 0) {
		if t < 0 || math.IsNaN(t) {
			t = 0
		} else if t > 1 {
			t = 1
		}
	}
	return s.ramp.At(t, s.space)
}
