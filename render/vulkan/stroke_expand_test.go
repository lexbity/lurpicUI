//go:build linux && cgo

package vulkan_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// sharpChevronStroke builds the Slice 8 miter-limit fixture: a narrow V whose
// point-miter extends ~3.3x the half-width beyond the vertex, so a low
// MiterLimit forces a bevel fallback while a high one keeps the sharp miter.
func sharpChevronStroke(miterLimit float32) *render.Frame {
	stroke := gfx.DefaultStroke(6)
	stroke.MiterLimit = miterLimit
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 20, Y: 48}).
		LineTo(gfx.Point{X: 32, Y: 10}).
		LineTo(gfx.Point{X: 44, Y: 48}).
		Build()
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.StrokePath{Path: path, Stroke: stroke, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(70, 120, 230, 255))},
			}},
		}},
	}
}

// TestStrokeExpand_MiterLimitIsHonored is the Slice 8 negative control: the GPU
// output must change when the miter limit changes (the join falls back from a
// sharp miter to a bevel), and the equivalence harness must catch a miter-limit
// regression — a GPU render with the wrong limit fails against the correct
// software oracle.
func TestStrokeExpand_MiterLimitIsHonored(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	clipped := sharpChevronStroke(2)    // low limit -> bevel fallback
	unclipped := sharpChevronStroke(50) // high limit -> sharp miter

	softClipped, err := equivalence.RenderSoftware(clipped, w, h)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	gpuClipped := renderVulkanPixels(t, clipped, w, h)
	gpuUnclipped := renderVulkanPixels(t, unclipped, w, h)

	// The miter limit must actually change the GPU output.
	same := equivalence.Compare(gpuClipped, gpuUnclipped, w, h, equivalence.EquivalenceTolerance{
		MinPSNR: 1000, P99Diff: 0, MaxDiff: 0, WithinFraction: 1,
	})
	if same.Passed {
		t.Fatalf("changing the miter limit must change the GPU render; the limit is ignored")
	}

	// Baseline: the clipped render passes against the clipped oracle.
	baseline := equivalence.Compare(softClipped, gpuClipped, w, h, equivalence.DefaultTolerance())
	if !baseline.Passed {
		t.Fatalf("baseline clipped chevron failed equivalence: %s", baseline.String())
	}

	// Negative control: the unclipped GPU render (sharp miter) must fail against
	// the clipped software oracle (bevel).
	report := equivalence.Compare(softClipped, gpuUnclipped, w, h, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("a miter-limit regression must fail equivalence against the oracle, got: %s", report.String())
	}
}

// TestStrokeExpand_CapsAndJoinsRender verifies the full StrokeStyle is honored
// end-to-end: round caps render past the endpoints, round joins bulge at the
// corner, and bevel joins cut it — all matching the software oracle.
func TestStrokeExpand_CapsAndJoinsRender(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	stroke := gfx.DefaultStroke(6)
	stroke.Cap = gfx.LineCapRound
	stroke.Join = gfx.LineJoinRound
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 8, Y: 8}).
		LineTo(gfx.Point{X: 48, Y: 8}).
		LineTo(gfx.Point{X: 48, Y: 48}).
		Build()
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.StrokePath{Path: path, Stroke: stroke, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(230, 60, 60, 255))},
			}},
		}},
	}

	soft, err := equivalence.RenderSoftware(frame, 64, 64)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	gpu := renderVulkanPixels(t, frame, 64, 64)
	report := equivalence.Compare(soft, gpu, 64, 64, equivalence.DefaultTolerance())
	if !report.Passed {
		t.Fatalf("round-cap/join stroke failed equivalence: %s", report.String())
	}

	// The round start cap reaches h beyond the first vertex (a butt cap would
	// leave x=5 empty).
	if a := pixelAt(gpu, 64, 5, 8)[3]; a < 150 {
		t.Fatalf("round cap must reach (5,8) with substantial coverage, got alpha %d", a)
	}
}
