//go:build linux && cgo

package vulkan_test

import (
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/fontdata"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// order_gpu_test.go: S6 (Vulkan painter order, one canonical operation stream).
// Mixed-kind fixtures are rendered offscreen with readback and compared
// against the software renderer's output for the same canonical frame, and
// each fixture asserts specific overlap pixels so a wrong composite order
// cannot hide behind a global PSNR pass. GPU absence skips with explicit
// missing evidence (the P9 required-backend convention), never a silent pass.

// renderBothBackends renders the frame through the software reference and the
// GPU, requires the GPU output to match the software output within the
// equivalence tolerance, and returns both pixel buffers (RGBA order).
func renderBothBackends(t *testing.T, frame *render.Frame, w, h int) ([]byte, []byte) {
	t.Helper()
	soft, err := equivalence.RenderSoftware(frame, w, h)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	gpu := renderVulkanPixels(t, frame, w, h)
	if report := equivalence.Compare(soft, gpu, w, h, equivalence.DefaultTolerance()); !report.Passed {
		t.Fatalf("GPU output must match the software reference: %s", report.String())
	}
	return soft, gpu
}

// solidRGBA builds a w*h image filled with one opaque color (the source for
// solid-texture DrawImage fixtures).
func solidRGBA(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

// TestPainterOrderPathThenRect graduates the painter-order overlay test
// (TestReviewVulkanPainterOrderAcrossCommandKinds): a path drawn before a
// rect must be covered by the rect at their overlap. Under the deleted
// three-pass structure the path fills rendered after all instanced draws, so
// the red path overpainted the blue rect.
func TestPainterOrderPathThenRect(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 32, 32
	rect := gfx.RectFromXYWH(0, 0, 16, 16)
	frame := testframe.Seal(render.FrameInput{
		Batches: []render.BatchInput{{
			ID:      1,
			Bounds:  rect,
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillPath{Path: gfx.RoundedRectPath(rect, 0), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255))},
				gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255))},
			}},
		}},
	})
	defer frame.Release()

	_, gpu := renderBothBackends(t, frame, w, h)

	// The later blue rectangle covers the earlier red path at their overlap.
	assertPixel(t, gpu, w, 8, 8, [4]byte{0, 0, 255, 255})
	// Outside the shared bounds the surface stays at the transparent clear.
	assertPixel(t, gpu, w, 24, 24, [4]byte{0, 0, 0, 0})
}

// TestPainterOrderRectThenPath is the mirror of the path-then-rect fixture:
// the path drawn after the rect must composite over it.
func TestPainterOrderRectThenPath(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	frame := testframe.Seal(render.FrameInput{
		Batches: []render.BatchInput{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, w, h),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, 48, 48), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(40, 120, 220, 255))},
				gfx.FillPath{Path: gfx.RoundedRectPath(gfx.RectFromXYWH(24, 24, 32, 32), 6), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(220, 60, 60, 255))},
			}},
		}},
	})
	defer frame.Release()

	_, gpu := renderBothBackends(t, frame, w, h)

	// The later path covers the rect at their overlap; the rect-only region
	// keeps the rect; the rest stays at the clear.
	assertPixel(t, gpu, w, 40, 40, [4]byte{220, 60, 60, 255})
	assertPixel(t, gpu, w, 16, 16, [4]byte{40, 120, 220, 255})
	assertPixel(t, gpu, w, 4, 4, [4]byte{0, 0, 0, 0})
}

// TestPainterOrderShadowAfterRect proves the shadow composites at its
// canonical position: issued after a rect it must darken the rect (over it),
// and the shadow-before-rect arrangement must produce a different overlap
// pixel (the rect covers the shadow) — the fixture distinguishes both orders,
// which the deleted global shadow pre-pass could not.
func TestPainterOrderShadowAfterRect(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	batch := func(shadowFirst bool) render.BatchInput {
		shadow := gfx.DrawBlurredShadow{
			Path:       gfx.RectPath(gfx.RectFromXYWH(16, 16, 32, 32)),
			Color:      gfx.ColorFromRGBA8(0, 0, 0, 200),
			BlurRadius: 6,
			Offset:     gfx.Point{},
			Inner:      false,
		}
		rect := gfx.FillRect{Rect: gfx.RectFromXYWH(16, 16, 32, 32), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(200, 200, 200, 255))}
		cmds := []gfx.Command{rect, shadow}
		if shadowFirst {
			cmds = []gfx.Command{shadow, rect}
		}
		return render.BatchInput{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}
	}

	afterFrame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{batch(false)}})
	defer afterFrame.Release()
	beforeFrame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{batch(true)}})
	defer beforeFrame.Release()

	_, gpuAfter := renderBothBackends(t, afterFrame, w, h)

	// Deep inside the rect the blur coverage saturates, so the shadow composes
	// over the full rect color: 0 + (1-200/255)*200 ≈ 43 per channel. A pixel
	// that stays at the pure rect color means the shadow rendered behind it.
	got := pixelAt(gpuAfter, w, 32, 32)
	if got[0] >= 100 {
		t.Fatalf("shadow issued after the rect must darken it at the overlap, got %v", got)
	}
	if got[3] != 255 {
		t.Fatalf("shadow-over-opaque-rect overlap must stay opaque, got %v", got)
	}

	// The reversed arrangement (shadow first) must differ: the rect covers the
	// shadow, so the overlap is the pure rect color.
	gpuBefore := renderVulkanPixels(t, beforeFrame, w, h)
	assertPixel(t, gpuBefore, w, 32, 32, [4]byte{200, 200, 200, 255})
}

// TestMixedKindInterleavedFourKind renders rect, path, glyph, and image
// alternating eight times in one batch — every consecutive pair of draws is a
// different kind, so nothing may merge and every op must composite at its
// canonical position. The final image draw must cover the rect/path/glyph
// content drawn before it at the shared center, while earlier-exclusive
// regions keep their own colors; the whole frame must match the software
// reference.
func TestMixedKindInterleavedFourKind(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	reg := fontdata.TestFontRegistry(t)
	run, _ := makeGlyphRun(reg, 32, gfx.Point{}, latinGlyphs(32))
	ink := gfx.ColorFromRGBA8(20, 20, 24, 255)
	tex := testframe.Image(solidRGBA(8, 8, color.RGBA{R: 0, G: 200, B: 0, A: 255}))

	const w, h = 288, 200
	cmds := make([]gfx.Command, 0, 32)
	for i := 0; i < 8; i++ {
		off := float32(i) * 6
		cmds = append(cmds,
			gfx.FillRect{Rect: gfx.RectFromXYWH(90+off, 30+off, 96, 96), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(40, 120, 220, 255))},
			gfx.FillPath{Path: gfx.RoundedRectPath(gfx.RectFromXYWH(102+off, 42+off, 72, 72), 8), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(220, 60, 60, 255))},
			gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 110 + off, Y: 96 + off}, Brush: gfx.SolidBrush(ink)},
			gfx.DrawImage{Image: tex, DestRect: gfx.RectFromXYWH(114+off, 54+off, 48, 48), SrcRect: gfx.RectFromXYWH(0, 0, 8, 8), Sampling: gfx.SamplingNearest, Opacity: 1},
		)
	}
	frame := testframe.Seal(render.FrameInput{
		Batches: []render.BatchInput{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, w, h),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: cmds},
		}},
	})
	defer frame.Release()

	_, gpu := renderBothBackends(t, frame, w, h)

	// The final command (the eighth image) covers the shared center over the
	// rect, path, and glyph content drawn before it.
	assertPixel(t, gpu, w, 160, 100, [4]byte{0, 200, 0, 255})
	// A region covered only by an early rect keeps the rect color: no later
	// draw reaches it, and the first rect's own successors (its path, glyph,
	// and image) start further in.
	assertPixel(t, gpu, w, 100, 40, [4]byte{40, 120, 220, 255})
}

// TestPainterOrderStrokeOverImage proves a stroke issued after a texture
// composites over it (the packet encoder expands strokes to FillPath, so this
// is the path-over-image interleave), with the image-only region keeping the
// texture color.
func TestPainterOrderStrokeOverImage(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	tex := testframe.Image(solidRGBA(8, 8, color.RGBA{R: 60, G: 120, B: 220, A: 255}))
	stroke := gfx.DefaultStroke(4)
	stroke.Join = gfx.LineJoinRound
	frame := testframe.Seal(render.FrameInput{
		Batches: []render.BatchInput{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, w, h),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawImage{Image: tex, DestRect: gfx.RectFromXYWH(8, 8, 48, 48), SrcRect: gfx.RectFromXYWH(0, 0, 8, 8), Sampling: gfx.SamplingNearest, Opacity: 1},
				gfx.StrokePath{Path: gfx.RoundedRectPath(gfx.RectFromXYWH(24, 24, 32, 32), 4), Stroke: stroke, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(220, 60, 60, 255))},
			}},
		}},
	})
	defer frame.Release()

	_, gpu := renderBothBackends(t, frame, w, h)

	// Mid-edge of the stroke ring, over the image: the later stroke wins.
	assertPixel(t, gpu, w, 24, 40, [4]byte{220, 60, 60, 255})
	// Image-only region keeps the texture color.
	assertPixel(t, gpu, w, 12, 12, [4]byte{60, 120, 220, 255})
	// Outside both shapes the surface stays at the clear.
	assertPixel(t, gpu, w, 60, 60, [4]byte{0, 0, 0, 0})
}
