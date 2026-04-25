package perfscene

import (
	"fmt"
	"image"
	"image/color"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// NodeFrame returns a single-batch scene with many solid rect commands.
func NodeFrame(nodes int, width, height int, nonce uint64) *render.Frame {
	if nodes < 0 {
		nodes = 0
	}
	cmds := make([]gfx.Command, 0, nodes)
	if width <= 0 {
		width = 1
	}
	if height <= 0 {
		height = 1
	}
	cellW := float32(width) / 128.0
	if cellW < 1 {
		cellW = 1
	}
	cellH := float32(height) / 128.0
	if cellH < 1 {
		cellH = 1
	}
	for i := 0; i < nodes; i++ {
		x := float32((i*7)%width) + float32(i%3)*0.1
		y := float32((i*13)%height) + float32(i%5)*0.1
		cmds = append(cmds, gfx.FillRect{
			Rect:  gfx.RectFromXYWH(x, y, cellW, cellH),
			Brush: gfx.SolidBrush(gfx.Color{R: float32((i % 255)) / 255.0, G: 0.25, B: 0.75, A: 1}),
		})
	}
	bounds := gfx.RectFromXYWH(0, 0, float32(width), float32(height))
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: uint64(nodes)<<32 ^ nonce,
				Commands:    gfx.CommandList{Commands: cmds},
			},
		},
	}
}

// ImageFrame returns a single-batch scene with many image draws.
func ImageFrame(images int, width, height int, nonce uint64) *render.Frame {
	if images < 0 {
		images = 0
	}
	src := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			src.SetRGBA(x, y, imageRGBA(x, y))
		}
	}
	cmds := make([]gfx.Command, 0, images)
	for i := 0; i < images; i++ {
		x := float32((i * 9) % max(1, width-4))
		y := float32((i * 11) % max(1, height-4))
		cmds = append(cmds, gfx.DrawImage{
			Image:    src,
			DestRect: gfx.RectFromXYWH(x, y, 4, 4),
			SrcRect:  gfx.RectFromXYWH(0, 0, 4, 4),
			Sampling: gfx.SamplingNearest,
			Opacity:  1,
		})
	}
	bounds := gfx.RectFromXYWH(0, 0, float32(width), float32(height))
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: uint64(images)<<32 ^ nonce,
				Commands:    gfx.CommandList{Commands: cmds},
			},
		},
	}
}

func imageRGBA(x, y int) color.RGBA {
	switch (x + y) % 4 {
	case 0:
		return color.RGBA{R: 255, A: 255}
	case 1:
		return color.RGBA{G: 255, A: 255}
	case 2:
		return color.RGBA{B: 255, A: 255}
	default:
		return color.RGBA{R: 255, G: 255, A: 255}
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// CloneWithNonce returns a shallow copy whose command hashes are perturbed by
// nonce, forcing cache-driven renderers to process the frame again.
func CloneWithNonce(frame *render.Frame, nonce uint64) *render.Frame {
	if frame == nil {
		return nil
	}
	out := *frame
	if len(frame.RenderBatchs) > 0 {
		out.RenderBatchs = append([]render.RenderBatch(nil), frame.RenderBatchs...)
		for i := range out.RenderBatchs {
			out.RenderBatchs[i].CommandHash ^= nonce + uint64(i+1)
		}
	}
	if len(frame.Layers) > 0 {
		out.Layers = append([]render.LayeredBatch(nil), frame.Layers...)
		for i := range out.Layers {
			out.Layers[i].Batches = append([]render.RenderBatch(nil), frame.Layers[i].Batches...)
			for j := range out.Layers[i].Batches {
				out.Layers[i].Batches[j].CommandHash ^= nonce + uint64(i+j+1)
			}
		}
	}
	return &out
}

// Describe formats benchmark sub-test names.
func Describe(value int) string {
	return fmt.Sprintf("n=%d", value)
}
