package selection

import "codeburg.org/lexbit/lurpicui/gfx"

func rectCenterPoint(bounds gfx.Rect) gfx.Point {
	return gfx.Point{
		X: (bounds.Min.X + bounds.Max.X) * 0.5,
		Y: (bounds.Min.Y + bounds.Max.Y) * 0.5,
	}
}
