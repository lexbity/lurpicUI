package basic

import (
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func TestImage_fit_contain(t *testing.T) {
	img := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(100, 50)},
		Bounds:  BoundsProps{X: 0, Y: 0, W: 200, H: 200},
		Fit:     FitContain,
		Opacity: 1,
	}
	fit := img.resolveFit()
	if fit.content.Width() != 200 || fit.content.Height() != 100 {
		t.Fatalf("contain fit = %#v", fit.content)
	}
}

func TestImage_fit_cover(t *testing.T) {
	img := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(100, 50)},
		Bounds:  BoundsProps{X: 0, Y: 0, W: 100, H: 100},
		Fit:     FitCover,
		Opacity: 1,
	}
	fit := img.resolveFit()
	if fit.content.Width() < 100 || fit.content.Height() < 100 {
		t.Fatalf("cover fit did not cover bounds: %#v", fit.content)
	}
}

func TestImage_fit_none_uses_intrinsic_size(t *testing.T) {
	img := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(80, 40)},
		Bounds:  BoundsProps{X: 10, Y: 20, W: 200, H: 200},
		Fit:     FitNone,
		Opacity: 1,
	}
	fit := img.resolveFit()
	if fit.content.Width() != 80 || fit.content.Height() != 40 {
		t.Fatalf("fit none = %#v", fit.content)
	}
	if fit.content.Min != (gfx.Point{X: 10, Y: 20}) {
		t.Fatalf("fit none origin mismatch: %#v", fit.content)
	}
}

func TestImage_anchors_resolve_from_visible_content(t *testing.T) {
	img := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(100, 50)},
		Bounds:  BoundsProps{X: 0, Y: 0, W: 200, H: 200},
		Fit:     FitContain,
		Opacity: 1,
	}
	anchors := img.ExportAnchors(layout.AnchorExportContext{})
	got := anchors["bounds-center"]
	if got != (gfx.Point{X: 100, Y: 100}) {
		t.Fatalf("anchor = %+v", got)
	}
}

func TestImage_hit_follows_clip_policy(t *testing.T) {
	clipped := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(100, 50)},
		Bounds:  BoundsProps{X: 0, Y: 0, W: 100, H: 100},
		Fit:     FitCover,
		Clip:    true,
		Opacity: 1,
	}
	if !clipped.HitTest(gfx.Point{X: 10, Y: 10}) {
		t.Fatal("expected clipped image hit inside bounds")
	}
	unclipped := &Image{
		Source:  RGBAImageSource{ImageRef: newRGBA(100, 50)},
		Bounds:  BoundsProps{X: 0, Y: 0, W: 100, H: 100},
		Fit:     FitContain,
		Clip:    false,
		Opacity: 1,
	}
	if !unclipped.HitTest(gfx.Point{X: 10, Y: 60}) {
		t.Fatal("expected visible content hit")
	}
	if unclipped.HitTest(gfx.Point{X: 99, Y: 99}) {
		t.Fatal("expected outside visible content miss")
	}
}

func newRGBA(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	return img
}
