package corpus

import (
	"image"
	"image/color"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
)

func imageFixtures() []equivalence.FrameFixture {
	checker := func(w, h int) *image.RGBA {
		img := image.NewRGBA(image.Rect(0, 0, w, h))
		for y := 0; y < h; y++ {
			for x := 0; x < w; x++ {
				if (x+y)%2 == 0 {
					img.SetRGBA(x, y, color.RGBA{R: 220, G: 60, B: 60, A: 255})
				} else {
					img.SetRGBA(x, y, color.RGBA{R: 60, G: 120, B: 220, A: 255})
				}
			}
		}
		return img
	}

	return []equivalence.FrameFixture{
		fixture{
			name: "image_rgba_nearest_1to1", width: 64, height: 64,
			frame: func() *render.Frame {
				img := checker(16, 16)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawImage{
						Image:    img,
						DestRect: gfx.RectFromXYWH(8, 8, 16, 16),
						SrcRect:  gfx.RectFromXYWH(0, 0, 16, 16),
						Sampling: gfx.SamplingNearest,
						Opacity:  1,
					},
				)
			},
		},
		fixture{
			name: "image_scaled_nearest", width: 64, height: 64,
			frame: func() *render.Frame {
				img := checker(8, 8)
				return flatFrame(1, gfx.RectFromXYWH(0, 0, 64, 64), 1,
					gfx.DrawImage{
						Image:    img,
						DestRect: gfx.RectFromXYWH(8, 8, 32, 32),
						SrcRect:  gfx.RectFromXYWH(0, 0, 8, 8),
						Sampling: gfx.SamplingNearest,
						Opacity:  0.8,
					},
				)
			},
		},
	}
}
