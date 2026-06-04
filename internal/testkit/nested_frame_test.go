package testkit

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
)

func TestStrokeNestedRoundedRects(t *testing.T) {
	outer := gfx.RoundedRectPath(gfx.RectFromXYWH(10, 10, 200, 150), 8)
	inner := gfx.RoundedRectPath(gfx.RectFromXYWH(30, 30, 100, 70), 6)

	cmds := gfx.CommandList{}
	cmds.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(0, 0, 220, 170),
		Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 255, 255, 255)),
	})
	cmds.Add(gfx.StrokePath{
		Path:   outer,
		Stroke: gfx.DefaultStroke(2),
		Brush:  gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 0, 255)),
	})
	cmds.Add(gfx.StrokePath{
		Path:   inner,
		Stroke: gfx.DefaultStroke(1.5),
		Brush:  gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 0, 255)),
	})

	bounds := gfx.RectFromXYWH(0, 0, 220, 170)
	surface := NewMemorySurface(220, 170)
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	if err := r.Submit(&render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    cmds,
			},
		},
	}); err != nil {
		t.Fatalf("submit frame: %v", err)
	}

	AssertGolden(t, surface, "nested_rounded_rect_strokes")
}
