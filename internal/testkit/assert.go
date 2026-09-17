package testkit

import (
	"image/color"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type reporter interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// AssertPixelColor asserts a pixel at (x,y) matches expected within tolerance.
func AssertPixelColor(t reporter, surface *MemorySurface, x, y int, expected color.RGBA, tolerance uint8) {
	t.Helper()
	got := surface.PixelAt(x, y)
	if !rgbaWithin(got, expected, tolerance) {
		t.Errorf("pixel (%d,%d) = %#v, want %#v ±%d", x, y, got, expected, tolerance)
	}
}

// AssertRegionColor asserts all pixels in a region are within tolerance.
func AssertRegionColor(t reporter, surface *MemorySurface, region gfx.Rect, expected color.RGBA, tolerance uint8) {
	t.Helper()
	for y := int(region.Min.Y); y < int(region.Max.Y); y++ {
		for x := int(region.Min.X); x < int(region.Max.X); x++ {
			got := surface.PixelAt(x, y)
			if !rgbaWithin(got, expected, tolerance) {
				t.Errorf("pixel (%d,%d) = %#v, want %#v ±%d", x, y, got, expected, tolerance)
				return
			}
		}
	}
}

// AssertNotBlank asserts the surface has at least one non-transparent pixel.
func AssertNotBlank(t reporter, surface *MemorySurface) {
	t.Helper()
	img := surface.Capture()
	for _, px := range img.Pix {
		if px != 0 {
			return
		}
	}
	t.Errorf("surface is blank")
}

func rgbaWithin(got, want color.RGBA, tol uint8) bool {
	return absByte(got.R, want.R) <= tol &&
		absByte(got.G, want.G) <= tol &&
		absByte(got.B, want.B) <= tol &&
		absByte(got.A, want.A) <= tol
}

func absByte(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// AssertQuietFrame asserts the most recent frame's dirty set is empty (RX-1
// NFR-5 frame discipline): with the feed paused and no input or store writes,
// a frame must not re-project or re-lay anything. A frame loop that never goes
// quiet is a bug — an invalidation that fails to settle would repaint forever.
func AssertQuietFrame(t reporter, h *Harness) {
	t.Helper()
	if h == nil || h.Runtime() == nil {
		return
	}
	if snap := h.Runtime().LastDirtySnapshot(); len(snap) != 0 {
		t.Fatalf("expected a quiet frame, got %d dirty facets (frame never settles)", len(snap))
	}
}
