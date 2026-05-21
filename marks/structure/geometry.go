package structure

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func boundsAnchorSet(bounds gfx.Rect) layout.AnchorSet {
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds_center":       rectCenter(bounds),
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
	}
}

func rectCenter(bounds gfx.Rect) gfx.Point {
	return gfx.Point{
		X: (bounds.Min.X + bounds.Max.X) * 0.5,
		Y: (bounds.Min.Y + bounds.Max.Y) * 0.5,
	}
}
