package studio

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

// glyphLabel shapes label and returns a DrawGlyphRun command at the given
// baseline-origin x, y. Returns nil when the label is unshapeable or empty.
// The shaper is shared across exhibits (thread-safe since F-shape-fork-race).
func glyphLabel(x, y float32, label string, shaper *text.Shaper, style text.TextStyle, color gfx.Color) gfx.Command {
	if shaper == nil || label == "" {
		return nil
	}
	shaped := shaper.ShapeSimple(label, style)
	if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
		return nil
	}
	return gfx.DrawGlyphRun{
		Run:    shaped.Lines[0].Runs[0],
		Origin: gfx.Point{X: x, Y: y + 11},
		Brush:  gfx.SolidBrush(color),
	}
}
