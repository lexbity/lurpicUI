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
