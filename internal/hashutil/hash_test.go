package hashutil

import (
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

func TestHashutil_same_commandlist_same_hash(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if got1, got2 := HashCommandList(cl1), HashCommandList(cl2); got1 != got2 {
		t.Fatalf("%d != %d", got1, got2)
	}
}

func TestHashutil_different_commands_different_hash(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}

func TestHashutil_order_matters(t *testing.T) {
	cl1 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
		gfx.FillRect{Rect: gfx.RectFromXYWH(10, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
	}}
	cl2 := gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(10, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
		gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
	}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}

func TestHashutil_image_content_affects_hash(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	cl1 := gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img}}}
	img2 := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img2.SetRGBA(0, 0, color.RGBA{G: 255, A: 255})
	cl2 := gfx.CommandList{Commands: []gfx.Command{gfx.DrawImage{Image: img2}}}
	if HashCommandList(cl1) == HashCommandList(cl2) {
		t.Fatal("expected different hashes")
	}
}
