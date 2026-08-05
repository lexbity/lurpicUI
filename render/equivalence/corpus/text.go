package corpus

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/text"
)

func textFixtures(reg *text.FontRegistry) []equivalence.FrameFixture {
	const size float32 = 16
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: size})

	makeRun := func(origin gfx.Point, glyphs []text.PositionedGlyph) text.GlyphRun {
		return text.GlyphRun{
			Face:   face,
			Size:   size,
			Style:  text.TextStyle{Family: "Noto Sans", Size: size},
			Glyphs: glyphs,
		}
	}

	ink := gfx.ColorFromRGBA8(20, 20, 24, 255)

	return []equivalence.FrameFixture{
		fixture{
			name: "glyph_latin_small", width: 64, height: 64,
			frame: func() *render.Frame {
				run := makeRun(gfx.Point{X: 4, Y: 20}, []text.PositionedGlyph{
					{GlyphID: 65, X: 0, Y: 0, Advance: 12},
					{GlyphID: 66, X: 12, Y: 0, Advance: 12},
					{GlyphID: 67, X: 24, Y: 0, Advance: 12},
					{GlyphID: 68, X: 36, Y: 0, Advance: 12},
				})
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 4, Y: 20}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
		fixture{
			name: "glyph_latin_two_runs", width: 64, height: 64,
			frame: func() *render.Frame {
				runA := makeRun(gfx.Point{X: 4, Y: 16}, []text.PositionedGlyph{
					{GlyphID: 65, X: 0, Y: 0, Advance: 12},
					{GlyphID: 66, X: 12, Y: 0, Advance: 12},
				})
				runB := makeRun(gfx.Point{X: 4, Y: 40}, []text.PositionedGlyph{
					{GlyphID: 67, X: 0, Y: 0, Advance: 12},
					{GlyphID: 68, X: 12, Y: 0, Advance: 12},
				})
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawGlyphRun{Run: runA, Origin: gfx.Point{X: 4, Y: 16}, Brush: gfx.SolidBrush(ink)},
					gfx.DrawGlyphRun{Run: runB, Origin: gfx.Point{X: 4, Y: 40}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
	}
}
