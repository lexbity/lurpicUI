//go:build linux && cgo

package vulkan_test

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func TestPathProbe(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()
	brush := gfx.LinearGradientBrush(
		gfx.Point{X: 0, Y: 0}, gfx.Point{X: 64, Y: 0},
		[]gfx.GradientStop{{Offset: 0, Color: gfx.ColorFromRGBA8(230, 60, 60, 255)}, {Offset: 1, Color: gfx.ColorFromRGBA8(70, 120, 230, 255)}},
	)
	frame := &render.Frame{RenderBatchs: []render.RenderBatch{{ID: 1, Bounds: gfx.RectFromXYWH(0, 0, 64, 64), Opacity: 1,
		Commands: gfx.CommandList{Commands: []gfx.Command{
			gfx.FillPath{Path: gfx.RectPath(gfx.RectFromXYWH(8, 8, 32, 32)), Brush: brush},
		}}}}}
	soft, err := equivalence.RenderSoftware(frame, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	gpu, err := equivalence.RenderVulkan(frame, 64, 64)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []struct{ x, y int }{{8, 8}, {24, 24}, {30, 24}, {39, 24}, {8, 39}} {
		so, g := soft[(p.y*64+p.x)*4:(p.y*64+p.x)*4+4], gpu[(p.y*64+p.x)*4:(p.y*64+p.x)*4+4]
		fmt.Printf("(%d,%d) soft=%v gpu=%v\n", p.x, p.y, so, g)
	}
}
