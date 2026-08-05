package software

import (
	"bytes"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

const (
	determinismSceneW = 96
	determinismSceneH = 64
)

// buildRepresentativeScene returns the three rasterization paths most likely to
// diverge: a solid geometry fill (FillRect), an alpha-blended path (FillPath at
// 50% opacity), and text (DrawGlyphRun). The point is determinism, not visual
// richness.
func buildRepresentativeScene(t *testing.T) gfx.CommandList {
	t.Helper()
	return gfx.CommandList{Commands: []gfx.Command{
		gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 40, 24), Brush: gfx.SolidBrush(gfx.Color{R: 0.8, G: 0.2, B: 0.1, A: 1})},
		gfx.FillPath{
			Path: gfx.NewPath().
				MoveTo(gfx.Point{X: 8, Y: 56}).
				LineTo(gfx.Point{X: 88, Y: 34}).
				LineTo(gfx.Point{X: 56, Y: 60}).
				Close().
				Build(),
			Brush: gfx.SolidBrush(gfx.Color{G: 0.4, B: 0.9, A: 0.5}),
		},
		gfx.DrawGlyphRun{
			Run:    testGlyphRun(t, "Determinism", 16),
			Origin: gfx.Point{X: 4, Y: 40},
			Brush:  gfx.SolidBrush(gfx.Color{G: 1, A: 1}),
		},
	}}
}

// renderScene renders scene into a fresh renderer + surface and returns a copy
// of the rendered surface bytes. Each call performs the full lifecycle —
// create renderer, initialize against a new surface, submit, tear the renderer
// down — so every invocation is an independent dispose+recreate cycle.
func renderScene(t *testing.T, scene gfx.CommandList) []byte {
	t.Helper()
	r, s := newRenderer(t, determinismSceneW, determinismSceneH)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      gfx.RectFromXYWH(0, 0, determinismSceneW, determinismSceneH),
			Opacity:     1,
			CommandHash: 1,
			Commands:    scene,
		}},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	out := append([]byte(nil), s.buf...)
	r.Destroy()
	return out
}

func TestSoftwareRenderer_DeterministicOutput(t *testing.T) {
	scene := buildRepresentativeScene(t)

	out1 := renderScene(t, scene)
	out2 := renderScene(t, scene)

	if !hasNonZeroByte(out1) {
		t.Fatal("expected the representative scene to render non-blank output")
	}
	if !bytes.Equal(out1, out2) {
		t.Fatal("software renderer produced different output for identical input")
	}
}

func TestSoftwareRenderer_DeterministicAcrossRecreate(t *testing.T) {
	scene := buildRepresentativeScene(t)

	// Each renderScene call disposes its renderer and builds a fresh one over a
	// fresh surface; the recreated renderer must reproduce identical bytes.
	out1 := renderScene(t, scene)
	out2 := renderScene(t, scene)

	if !hasNonZeroByte(out1) {
		t.Fatal("expected the representative scene to render non-blank output")
	}
	if !bytes.Equal(out1, out2) {
		t.Fatal("software renderer output changed across dispose+recreate")
	}
}

func hasNonZeroByte(buf []byte) bool {
	for _, b := range buf {
		if b != 0 {
			return true
		}
	}
	return false
}
