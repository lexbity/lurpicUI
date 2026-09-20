package testkit

import (
	"image"
	"image/color"
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// aliveTolerance is the per-channel pixel tolerance for the alive visibility
// clause: a pixel within tolerance of the themed background counts as
// background, so a mark that renders nothing (or only the app background) is
// not visible.
const aliveTolerance = 3

// SampleNonBackground reports whether at least one pixel in region differs from
// bg by more than aliveTolerance. It samples a deterministic grid of `points`
// positions across the region (RX-1 FR-20's "visible" clause: a mark that
// renders only its themed background is not alive — the A-5/A-8 pixel gate).
func SampleNonBackground(t testing.TB, h *Harness, region gfx.Rect, bg color.RGBA, points int) bool {
	if t != nil {
		t.Helper()
	}
	if h == nil || h.Surface() == nil {
		return false
	}
	return SampleNonBackgroundImg(t, h.Surface().Capture(), region, bg, points)
}

// SampleNonBackgroundImg is the image-based variant: the caller captures the
// surface ONCE and samples many marks against it (the FR-20 walk checks ~48
// marks per frame, so capturing per mark would copy the surface ~48 times).
func SampleNonBackgroundImg(t testing.TB, img *image.RGBA, region gfx.Rect, bg color.RGBA, points int) bool {
	if t != nil {
		t.Helper()
	}
	if img == nil {
		return false
	}
	b := img.Bounds()
	if b.Dx() <= 0 || b.Dy() <= 0 {
		return false
	}
	if points < 1 {
		points = 1
	}
	side := int(math.Ceil(math.Sqrt(float64(points))))
	if side < 1 {
		side = 1
	}
	cellW := region.Width() / float32(side)
	cellH := region.Height() / float32(side)
	for gy := 0; gy < side; gy++ {
		for gx := 0; gx < side; gx++ {
			x := int(region.Min.X + cellW*float32(gx) + cellW*0.5)
			y := int(region.Min.Y + cellH*float32(gy) + cellH*0.5)
			x = aliveClamp(x, b.Min.X, b.Max.X-1)
			y = aliveClamp(y, b.Min.Y, b.Max.Y-1)
			px := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if aliveColorDiff(px, bg) > aliveTolerance {
				return true
			}
		}
	}
	return false
}

// RegionOf returns the facet's arranged bounds (RX-1 FR-20's "arranged"
// clause). A mark with empty arranged bounds contributes nothing and is not
// alive.
func RegionOf(f facet.FacetImpl) gfx.Rect {
	if f == nil || f.Base() == nil || f.Base().LayoutRole() == nil {
		return gfx.Rect{}
	}
	return f.Base().LayoutRole().ArrangedBounds
}

// ScreenPixel returns the presented pixel at a screen point.
func ScreenPixel(h *Harness, pt gfx.Point) color.RGBA {
	if h == nil || h.Surface() == nil {
		return color.RGBA{}
	}
	return h.Surface().PixelAt(int(pt.X), int(pt.Y))
}

func aliveClamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// aliveColorDiff returns the maximum per-channel difference between two
// colors.
func aliveColorDiff(a, b color.RGBA) uint8 {
	diff := func(x, y uint8) uint8 {
		if x > y {
			return x - y
		}
		return y - x
	}
	max := uint8(0)
	for _, d := range []uint8{diff(a.R, b.R), diff(a.G, b.G), diff(a.B, b.B), diff(a.A, b.A)} {
		if d > max {
			max = d
		}
	}
	return max
}
