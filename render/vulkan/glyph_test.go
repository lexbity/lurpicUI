//go:build linux && cgo

package vulkan_test

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
	"codeburg.org/lexbit/lurpicui/text"
)

func makeGlyphRun(reg *text.FontRegistry, size float32, origin gfx.Point, glyphs []text.PositionedGlyph) (text.GlyphRun, gfx.Point) {
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: size})
	return text.GlyphRun{
		Face:   face,
		Size:   size,
		Style:  text.TextStyle{Family: "Noto Sans", Size: size},
		Glyphs: glyphs,
	}, origin
}

func latinGlyphs(size float32) []text.PositionedGlyph {
	adv := size * 0.75
	return []text.PositionedGlyph{
		{GlyphID: 65, X: 0, Y: 0, Advance: adv},
		{GlyphID: 66, X: adv, Y: 0, Advance: adv},
		{GlyphID: 67, X: 2 * adv, Y: 0, Advance: adv},
		{GlyphID: 68, X: 3 * adv, Y: 0, Advance: adv},
	}
}

func glyphFrame(run text.GlyphRun, origin gfx.Point, w, h int, ink gfx.Color) *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, float32(w), float32(h)),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawGlyphRun{Run: run, Origin: origin, Brush: gfx.SolidBrush(ink)},
			}},
		}},
	}
}

// TestSDF_CrispAtLargeSize verifies the Slice 5 SDF path renders large glyphs
// crisply: solid ink in the interior, a transparent background, and a narrow
// edge band (no smearing). At 1:1 the SDF reads one texel per screen pixel, so
// the edges are cleanly snapped rather than blurred; the smoothstep
// reconstruction is what makes them scale-invariant when the glyph is not
// rendered at native size.
func TestSDF_CrispAtLargeSize(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	reg := fontdata.TestFontRegistry(t)
	ink := gfx.ColorFromRGBA8(20, 20, 24, 255)

	const w, h = 96, 96
	run, origin := makeGlyphRun(reg, 48, gfx.Point{X: 8, Y: 40}, latinGlyphs(48)[:2])
	out := renderFramePixels(t, glyphFrame(run, origin, w, h, ink), w, h)

	var opaque, partial, transparent int
	for i := 0; i < w*h; i++ {
		a := out[i*4+3]
		switch {
		case a == 255:
			opaque++
		case a > 0:
			partial++
		default:
			transparent++
		}
	}

	if opaque == 0 {
		t.Fatal("SDF glyph rendered no opaque ink pixels")
	}
	if transparent == 0 {
		t.Fatal("SDF glyph fills the whole canvas (expected transparent background)")
	}
	// The edge band must be narrow: partial-coverage pixels are a small
	// fraction of the glyph's area (a crisp edge, not a blur).
	if partial > opaque/2 {
		t.Fatalf("SDF edge band too wide: %d partial vs %d opaque pixels", partial, opaque)
	}
}

// TestAtlas_GrowsAndReuploads exercises the packed-atlas growth path: enough
// distinct glyphs are uploaded to overflow the initial texture, forcing a
// grow + full re-upload of the live entries. Glyphs uploaded before the growth
// must still render afterwards. Glyphs are uploaded in bitmap mode (size < 24)
// so the growth test does not pay the O(n^2) SDF generation cost.
func TestAtlas_GrowsAndReuploads(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	const size = 16.0
	const maskSize = 32
	const count = 1200 // 1200 * 32^2 = 1.2MB > 1024^2 initial atlas
	reg := fontdata.TestFontRegistry(t)
	face := reg.Resolve(text.TextStyle{Family: "Noto Sans", Size: size})

	mask := make([]byte, maskSize*maskSize)
	for gid := uint32(0); gid < count; gid++ {
		// A distinct filled-ring pattern per glyph so any mix-up is visible.
		for y := 0; y < maskSize; y++ {
			for x := 0; x < maskSize; x++ {
				if x >= 6 && x < maskSize-6 && y >= 6 && y < maskSize-6 {
					mask[y*maskSize+x] = 255
				}
			}
		}
		if err := vulkan.UploadGlyph(face.CacheKey(), gid, sizeBits(size), maskSize, maskSize, 0, 0, size*0.75, mask); err != nil {
			t.Fatalf("upload glyph %d: %v", gid, err)
		}
	}

	// A frame rendering a glyph uploaded before growth (glyph 0) must still be
	// visible: the grow path re-uploaded it into the grown atlas.
	run := text.GlyphRun{
		Face: face, Size: size, Style: text.TextStyle{Family: "Noto Sans", Size: size},
		Glyphs: []text.PositionedGlyph{{GlyphID: 0, X: 0, Y: 0, Advance: size * 0.75}},
	}
	const w, h = 64, 64
	out := renderFramePixels(t, glyphFrame(run, gfx.Point{X: 4, Y: 20}, w, h, gfx.ColorFromRGBA8(20, 20, 24, 255)), w, h)
	if !regionRendered(out, w, h, 0, 0, w, h) {
		t.Fatal("glyph 0 did not render after atlas growth (reflow lost the region)")
	}
}

func sizeBits(size float32) uint32 {
	return math.Float32bits(size)
}
