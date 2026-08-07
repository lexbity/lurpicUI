// Package corpus defines the acceptance fixtures for the equivalence harness.
// The fixtures cover the full gfx command surface: solid fills, width-honoring
// strokes, paths, polylines, points, selection rects, glyphs, images,
// gradients, clips, opacity, transforms, and blurred shadows. Each slice that
// adds a rendered feature adds fixtures that must meet the tolerance.
package corpus

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/text"
)

type fixture struct {
	name   string
	width  int
	height int
	frame  func() *render.Frame
}

func (f fixture) Name() string     { return f.name }
func (f fixture) Size() (int, int) { return f.width, f.height }

// Frame lazily builds the fixture's frame; the frame func may capture a
// prepared font registry for text fixtures.
func (f fixture) Frame() *render.Frame { return f.frame() }

var _ equivalence.FrameFixture = fixture{}

// All returns every baseline corpus fixture. reg is required for text fixtures
// and may be nil to exclude them.
func All(reg *text.FontRegistry) []equivalence.FrameFixture {
	var out []equivalence.FrameFixture
	out = append(out, geometryFixtures()...)
	if reg != nil {
		out = append(out, textFixtures(reg)...)
	}
	out = append(out, imageFixtures()...)
	return out
}

// flatFrame builds a layerless frame from a single batch's commands.
func flatFrame(id uint64, bounds gfx.Rect, opacity float32, cmds ...gfx.Command) *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       render.RenderBatchID(id),
			Bounds:   bounds,
			Opacity:  opacity,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	}
}
