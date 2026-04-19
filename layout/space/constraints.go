package space

import "codeburg.org/lexbit/lurpicui/gfx"

// Constraints describe the available layout space for a facet.
type Constraints struct {
	MinSize gfx.Size
	MaxSize gfx.Size
}

// Tight creates a fully constrained size.
func Tight(size gfx.Size) Constraints {
	return Constraints{MinSize: size, MaxSize: size}
}

// Loose creates a constraint with no minimum and a maximum size.
func Loose(max gfx.Size) Constraints {
	return Constraints{MaxSize: max}
}

// Unconstrained creates unconstrained layout bounds.
func Unconstrained() Constraints {
	return Constraints{}
}

// Constrain clamps a size to the constraint range.
func (c Constraints) Constrain(s gfx.Size) gfx.Size {
	s.W = clampAxis(s.W, c.MinSize.W, c.MaxSize.W)
	s.H = clampAxis(s.H, c.MinSize.H, c.MaxSize.H)
	return s
}

// IsTight reports whether the min and max sizes match exactly.
func (c Constraints) IsTight() bool {
	return c.MinSize == c.MaxSize
}

// WithMaxWidth returns a copy with a different maximum width.
func (c Constraints) WithMaxWidth(w float32) Constraints {
	c.MaxSize.W = w
	return c
}

// WithMaxHeight returns a copy with a different maximum height.
func (c Constraints) WithMaxHeight(h float32) Constraints {
	c.MaxSize.H = h
	return c
}

func clampAxis(v, min, max float32) float32 {
	if v < min {
		v = min
	}
	if max > 0 && v > max {
		v = max
	}
	return v
}
