//go:build linux && cgo

package vulkan_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// holedPath builds an outer square (8,8)-(56,56) plus an inner square. When
// `innerOpposite` is true the inner contour runs opposite the outer, so the
// nonzero winding rule carves a hole; when false it runs the same direction,
// so the nonzero rule fills the inner region (the hole "disappears").
func holedPath(innerOpposite bool) gfx.Path {
	p := gfx.NewPath().
		MoveTo(gfx.Point{X: 8, Y: 8}).
		LineTo(gfx.Point{X: 56, Y: 8}).
		LineTo(gfx.Point{X: 56, Y: 56}).
		LineTo(gfx.Point{X: 8, Y: 56}).
		Close()
	if innerOpposite {
		p = p.
			MoveTo(gfx.Point{X: 24, Y: 24}).
			LineTo(gfx.Point{X: 24, Y: 40}).
			LineTo(gfx.Point{X: 40, Y: 40}).
			LineTo(gfx.Point{X: 40, Y: 24}).
			Close()
	} else {
		p = p.
			MoveTo(gfx.Point{X: 24, Y: 24}).
			LineTo(gfx.Point{X: 40, Y: 24}).
			LineTo(gfx.Point{X: 40, Y: 40}).
			LineTo(gfx.Point{X: 24, Y: 40}).
			Close()
	}
	return p.Build()
}

func pathFillFrame(path gfx.Path, brush gfx.Brush, w, h int) *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:       1,
			Bounds:   gfx.RectFromXYWH(0, 0, float32(w), float32(h)),
			Opacity:  1,
			Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillPath{Path: path, Brush: brush}}},
		}},
	}
}

func renderVulkanPixels(t *testing.T, frame *render.Frame, w, h int) []byte {
	t.Helper()
	packet, err := vulkan.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}
	out, err := vulkan.SubmitAndReadback(packet, w, h)
	if err != nil {
		t.Fatalf("readback: %v", err)
	}
	return out
}

func pixelAt(pixels []byte, width, x, y int) [4]byte {
	off := (y*width + x) * 4
	return [4]byte{pixels[off], pixels[off+1], pixels[off+2], pixels[off+3]}
}

func assertPixel(t *testing.T, pixels []byte, width, x, y int, want [4]byte) {
	t.Helper()
	got := pixelAt(pixels, width, x, y)
	if got != want {
		t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, got, want)
	}
}

// TestStencilFill_HoleRendered proves the winding-based stencil path fill
// carves the hole: opposite-winding inner contour is empty, the ring is filled,
// the outside is empty (FR-5: holes, explicit winding).
func TestStencilFill_HoleRendered(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	brush := gfx.SolidBrush(gfx.ColorFromRGBA8(230, 60, 60, 255))
	gpu := renderVulkanPixels(t, pathFillFrame(holedPath(true), brush, w, h), w, h)

	assertPixel(t, gpu, w, 32, 12, [4]byte{230, 60, 60, 255}) // ring top interior
	assertPixel(t, gpu, w, 12, 32, [4]byte{230, 60, 60, 255}) // ring left interior
	assertPixel(t, gpu, w, 32, 32, [4]byte{0, 0, 0, 0})       // hole center
	assertPixel(t, gpu, w, 4, 4, [4]byte{0, 0, 0, 0})         // outside
}

// TestStencilFill_WindingInvertNegativeControl is the Slice 7 negative control:
// inverting the hole's winding must make the hole disappear on the GPU, and the
// equivalence harness must fail when the inverted-winding GPU render is compared
// against the correct (hole-carving) software oracle. The baseline (correct
// winding on both backends) must pass first, proving the harness distinguishes
// the winding regression from normal AA-model differences.
func TestStencilFill_WindingInvertNegativeControl(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	brush := gfx.SolidBrush(gfx.ColorFromRGBA8(230, 60, 60, 255))

	correctFrame := pathFillFrame(holedPath(true), brush, w, h)
	invertedFrame := pathFillFrame(holedPath(false), brush, w, h)

	soft, err := equivalence.RenderSoftware(correctFrame, w, h)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	correctGPU := renderVulkanPixels(t, correctFrame, w, h)
	invertedGPU := renderVulkanPixels(t, invertedFrame, w, h)

	// The GPU must carve the hole with the correct winding and fill it with the
	// inverted winding.
	assertPixel(t, correctGPU, w, 32, 32, [4]byte{0, 0, 0, 0})
	if a := pixelAt(invertedGPU, w, 32, 32)[3]; a < 200 {
		t.Fatalf("inverted winding must fill the hole region, got alpha %d at the hole center", a)
	}

	// Baseline: correct winding on both backends passes equivalence.
	baseline := equivalence.Compare(soft, correctGPU, w, h, equivalence.DefaultTolerance())
	if !baseline.Passed {
		t.Fatalf("baseline hole fill failed equivalence: %s", baseline.String())
	}

	// Negative control: the inverted-winding GPU render must fail against the
	// correct software oracle.
	report := equivalence.Compare(soft, invertedGPU, w, h, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("inverted winding must fail equivalence against the hole-carving oracle, got: %s", report.String())
	}
}
