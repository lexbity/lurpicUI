package layout

import "codeburg.org/lexbit/lurpicui/gfx"

// Alignment places smaller children within a container or layer cell.
type Alignment uint8

const (
	AlignStretch Alignment = iota
	AlignStart
	AlignCenter
	AlignEnd
	AlignTopLeft
	AlignTopCenter
	AlignTopRight
	AlignCenterLeft
	AlignCenterRight
	AlignBottomLeft
	AlignBottomCenter
	AlignBottomRight
	AlignBaseline
)

func alignedOrigin(childSize gfx.Size, bounds gfx.Rect, a Alignment) gfx.Point {
	dx := bounds.Width() - childSize.W
	dy := bounds.Height() - childSize.H
	if dx < 0 {
		dx = 0
	}
	if dy < 0 {
		dy = 0
	}

	x := bounds.Min.X
	y := bounds.Min.Y

	switch a {
	case AlignTopCenter, AlignCenter, AlignBottomCenter:
		x += dx / 2
	case AlignTopRight, AlignCenterRight, AlignBottomRight:
		x += dx
	}

	switch a {
	case AlignCenterLeft, AlignCenter, AlignCenterRight:
		y += dy / 2
	case AlignBottomLeft, AlignBottomCenter, AlignBottomRight:
		y += dy
	}

	return gfx.Point{X: x, Y: y}
}
