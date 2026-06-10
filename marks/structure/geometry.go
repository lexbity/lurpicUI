package structure

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

func boundsAnchorSet(bounds gfx.Rect) layout.AnchorSet {
	var c marks.Core
	return c.DefaultAnchors(bounds, layout.AnchorExportContext{})
}

func rectCenter(bounds gfx.Rect) gfx.Point {
	return gfx.Point{
		X: (bounds.Min.X + bounds.Max.X) * 0.5,
		Y: (bounds.Min.Y + bounds.Max.Y) * 0.5,
	}
}
