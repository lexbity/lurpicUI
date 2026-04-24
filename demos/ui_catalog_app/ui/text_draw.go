package ui

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

// drawTextLine emits a shaped line as glyph-run draw commands.
func drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	if list == nil || len(line.Runs) == 0 {
		return
	}

	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	brush := gfx.SolidBrush(color)
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  brush,
		})
	}
}
