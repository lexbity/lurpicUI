//go:build linux && cgo

package vulkan_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// shadowFrame builds a single-shadow frame with the given blur radius.
func shadowFrame(radius float32, inner bool) *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.DrawBlurredShadow{
					Path:       gfx.RectPath(gfx.RectFromXYWH(16, 16, 24, 24)),
					Color:      gfx.ColorFromRGBA8(10, 10, 30, 160),
					BlurRadius: radius,
					Offset:     gfx.Point{X: 3, Y: 4},
					Inner:      inner,
				},
			}},
		}},
	}
}

// TestBlurredShadow_Rendered proves the GPU blur pipeline renders a shadow: the
// blurred coverage is nonzero just outside the (offset) shadow shape and the
// un-blurred far field stays empty.
func TestBlurredShadow_Rendered(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	gpu := renderVulkanPixels(t, shadowFrame(4, false), w, h)

	// The shadow is the path (16,16)-(40,40) blurred by 4 and offset (+3,+4):
	// a point just inside the path's top edge (outside the source shape but
	// within the blur falloff) must carry shadow alpha.
	if a := pixelAt(gpu, w, 20, 20)[3]; a == 0 {
		t.Fatal("expected shadow alpha inside the blur region, got 0")
	}
	// A point two blur radii outside the shadow region is empty.
	if a := pixelAt(gpu, w, 4, 4)[3]; a != 0 {
		t.Fatalf("expected empty far field, got alpha %d", a)
	}
}

// TestShadow_SoftwareParity proves the software oracle renders DrawBlurredShadow
// with the same geometry as the GPU: a pixel just inside the shadow falloff has
// shadow alpha on both backends, and both agree within the Q1 tolerance (the
// corpus fixture shadow_medium_radius_8 covers the byte-level comparison).
func TestShadow_SoftwareParity(t *testing.T) {
	const w, h = 64, 64
	soft, err := equivalence.RenderSoftware(shadowFrame(8, false), w, h)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	if a := pixelAt(soft, w, 20, 20)[3]; a == 0 {
		t.Fatal("software oracle must render shadow alpha inside the blur region")
	}
}

// TestBlurredShadow_BlurRadiusNegativeControl is the Slice 9 negative control:
// the GPU render of a shadow with the wrong blur radius must fail equivalence
// against the software oracle (the harness catches a blur-radius regression),
// and the two radii must produce different GPU output (the control is not
// void). The correct-radius baseline must pass first.
func TestBlurredShadow_BlurRadiusNegativeControl(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	soft, err := equivalence.RenderSoftware(shadowFrame(4, false), w, h)
	if err != nil {
		t.Fatalf("software render: %v", err)
	}
	gpuR4 := renderVulkanPixels(t, shadowFrame(4, false), w, h)
	gpuR8 := renderVulkanPixels(t, shadowFrame(8, false), w, h)

	baseline := equivalence.Compare(soft, gpuR4, w, h, equivalence.DefaultTolerance())
	if !baseline.Passed {
		t.Fatalf("baseline radius-4 shadow failed equivalence: %s", baseline.String())
	}

	// The two radii must produce different output, or the control is void.
	delta := equivalence.Compare(gpuR4, gpuR8, w, h, equivalence.DefaultTolerance())
	if delta.Passed {
		t.Fatalf("blur radii 4 and 8 produced byte-equivalent shadows; the negative control is void")
	}

	// The wrong radius must fail against the correct-radius oracle.
	report := equivalence.Compare(soft, gpuR8, w, h, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("wrong blur radius must fail equivalence against the oracle, got: %s", report.String())
	}
}
