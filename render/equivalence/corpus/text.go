package corpus

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/text"
)

func textFixtures(reg *text.FontRegistry) []equivalence.FrameFixture {
	face16 := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 16})
	face48 := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 48})

	makeRun := func(face text.FontFace, size float32, origin gfx.Point, glyphs []text.PositionedGlyph) text.GlyphRun {
		return text.GlyphRun{
			Face:   face,
			Size:   size,
			Style:  text.TextStyle{Family: "Noto Sans", Size: size},
			Glyphs: glyphs,
		}
	}

	latin := func(face text.FontFace, size float32) []text.PositionedGlyph {
		adv := size * 0.75
		return []text.PositionedGlyph{
			{GlyphID: 65, X: 0, Y: 0, Advance: adv},
			{GlyphID: 66, X: adv, Y: 0, Advance: adv},
			{GlyphID: 67, X: 2 * adv, Y: 0, Advance: adv},
			{GlyphID: 68, X: 3 * adv, Y: 0, Advance: adv},
		}
	}

	ink := gfx.ColorFromRGBA8(20, 20, 24, 255)

	return []equivalence.FrameFixture{
		// Bitmap glyphs (size < 24 px): the GPU blits the coverage mask from the
		// atlas 1:1, matching the oracle's per-pixel mask blit within rounding.
		fixture{
			name: "glyph_latin_small", width: 64, height: 64,
			frame: func() *render.Frame {
				run := makeRun(face16, 16, gfx.Point{X: 4, Y: 20}, latin(face16, 16))
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 4, Y: 20}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
		fixture{
			name: "glyph_latin_two_runs", width: 64, height: 64,
			frame: func() *render.Frame {
				runA := makeRun(face16, 16, gfx.Point{X: 4, Y: 16}, latin(face16, 16)[:2])
				runB := makeRun(face16, 16, gfx.Point{X: 4, Y: 40}, latin(face16, 16)[2:])
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawGlyphRun{Run: runA, Origin: gfx.Point{X: 4, Y: 16}, Brush: gfx.SolidBrush(ink)},
					gfx.DrawGlyphRun{Run: runB, Origin: gfx.Point{X: 4, Y: 40}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
		// The 24px boundary flips to the SDF channel (SDF_MIN_SIZE = 24).
		fixture{
			name: "glyph_latin_24px", width: 96, height: 96,
			frame: func() *render.Frame {
				face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: 24})
				run := makeRun(face, 24, gfx.Point{X: 8, Y: 36}, latin(face, 24))
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 96, 96), 1,
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 8, Y: 36}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
		// Large glyphs render from the SDF channel; the smoothstep reconstruction
		// is perceptually close to the oracle's exact coverage (a feature-specific
		// tolerance is recorded in the corpus runner).
		fixture{
			name: "glyph_latin_48px", width: 160, height: 96,
			frame: func() *render.Frame {
				run := makeRun(face48, 48, gfx.Point{X: 8, Y: 56}, latin(face48, 48))
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 160, 96), 1,
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 8, Y: 56}, Brush: gfx.SolidBrush(ink)},
				)
			},
		},
		// Many runs exercise batching (one instanced draw per brush/state).
		fixture{
			name: "glyph_many_runs", width: 128, height: 128,
			frame: func() *render.Frame {
				var cmds []gfx.Command
				for i := 0; i < 12; i++ {
					x := float32((i % 4) * 32)
					y := float32((i/4)*32 + 16)
					run := makeRun(face16, 16, gfx.Point{X: x, Y: y}, latin(face16, 16))
					cmds = append(cmds, gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: x, Y: y}, Brush: gfx.SolidBrush(ink)})
				}
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 128, 128), 1, cmds...)
			},
		},
	}
}
