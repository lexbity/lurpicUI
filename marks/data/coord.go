package data

import "codeburg.org/lexbit/lurpicui/gfx"

// Pt narrows float64 coordinates to float32 for use with the gfx package.
// This is the standard narrowing boundary between scale arithmetic (float64)
// and the render coordinate system (float32).
func Pt(x, y float64) gfx.Point {
	return gfx.Point{X: float32(x), Y: float32(y)}
}
