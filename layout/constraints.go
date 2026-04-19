package layout

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout/space"
)

// Constraints is the shared layout constraint type.
type Constraints = space.Constraints

// Tight creates a fully constrained size.
func Tight(size gfx.Size) Constraints { return space.Tight(size) }

// Loose creates a constraint with no minimum and a maximum size.
func Loose(max gfx.Size) Constraints { return space.Loose(max) }

// Unconstrained creates unconstrained layout bounds.
func Unconstrained() Constraints { return space.Unconstrained() }
