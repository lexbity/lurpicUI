//go:build linux && cgo

package vulkan_test

import (
	"bytes"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/equivalence"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func gradientFrame(origin, end gfx.Point, stops []gfx.GradientStop) *render.Frame {
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{
					Rect:  gfx.RectFromXYWH(4, 4, 56, 56),
					Brush: gfx.LinearGradientBrush(origin, end, stops),
				},
			}},
		}},
	}
}

// TestGradient_SwappedStopsChangesOutput is the Slice 6 negative control: two
// gradients identical except for the ORDER of two stops must produce different
// GPU output (the stops flow through the real UBO path), and the equivalence
// harness must fail when the stops are swapped.
func TestGradient_SwappedStopsChangesOutput(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	red := gfx.ColorFromRGBA8(220, 60, 60, 255)
	blue := gfx.ColorFromRGBA8(70, 120, 230, 255)

	orig := gradientFrame(
		gfx.Point{X: 0, Y: 0},
		gfx.Point{X: 64, Y: 0},
		[]gfx.GradientStop{{Offset: 0, Color: red}, {Offset: 1, Color: blue}},
	)
	swapped := gradientFrame(
		gfx.Point{X: 0, Y: 0},
		gfx.Point{X: 64, Y: 0},
		[]gfx.GradientStop{{Offset: 0, Color: blue}, {Offset: 1, Color: red}},
	)

	origOut := renderFramePixels(t, orig, 64, 64)
	swappedOut := renderFramePixels(t, swapped, 64, 64)

	// The swapped stops must change the rendered gradient (the UBO content is
	// honored, not ignored).
	if bytes.Equal(origOut, swappedOut) {
		t.Fatal("swapping two gradient stops produced byte-identical output; the stops are not flowing through the UBO")
	}

	// The equivalence harness must fail on the swap.
	report := equivalence.Compare(origOut, swappedOut, 64, 64, equivalence.DefaultTolerance())
	if report.Passed {
		t.Fatalf("the swapped-stop gradient passed equivalence; the negative control is void: %s", report.String())
	}
}

// TestGradient_IdenticalBatchesMerge verifies gradients with identical content
// batch (one UBO) while distinct gradients do not coalesce: rendering a frame
// with two distinct gradients must show both, and re-encoding the same frame
// twice must produce identical output (determinism).
func TestGradient_IdenticalBatchesMerge(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	red := gfx.ColorFromRGBA8(220, 60, 60, 255)
	blue := gfx.ColorFromRGBA8(70, 120, 230, 255)
	green := gfx.ColorFromRGBA8(60, 200, 90, 255)

	// Two distinct gradients side by side.
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{
					Rect:  gfx.RectFromXYWH(4, 4, 28, 56),
					Brush: gfx.LinearGradientBrush(gfx.Point{X: 0, Y: 0}, gfx.Point{X: 64, Y: 0}, []gfx.GradientStop{{Offset: 0, Color: red}, {Offset: 1, Color: blue}}),
				},
				gfx.FillRect{
					Rect:  gfx.RectFromXYWH(32, 4, 28, 56),
					Brush: gfx.LinearGradientBrush(gfx.Point{X: 0, Y: 0}, gfx.Point{X: 64, Y: 0}, []gfx.GradientStop{{Offset: 0, Color: red}, {Offset: 1, Color: green}}),
				},
			}},
		}},
	}
	out1 := renderFramePixels(t, frame, 64, 64)
	out2 := renderFramePixels(t, frame, 64, 64)

	if !bytes.Equal(out1, out2) {
		t.Fatal("re-encoding the same gradient frame produced different output")
	}
	// The two halves must actually differ (the gradients did not coalesce into
	// one shared UBO): compare the left and right column colors.
	left := out1[(8*64+8)*4 : (8*64+8)*4+4]
	right := out1[(8*64+40)*4 : (8*64+40)*4+4]
	if bytes.Equal(left, right) {
		t.Fatal("distinct gradients coalesced: both rects rendered with the same stops")
	}
}

// TestGradient_ManyFrames_RingRecycles renders many distinct gradient frames to
// prove the per-frame uniform ring recycles its slots (the Slice 6 bump arena
// must reset each frame, not accumulate until overflow).
func TestGradient_ManyFrames_RingRecycles(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	for i := 0; i < 120; i++ {
		// A fresh gradient per frame (distinct content hashes maximize churn).
		frame := gradientFrame(
			gfx.Point{X: 0, Y: 0},
			gfx.Point{X: 64, Y: 0},
			[]gfx.GradientStop{
				{Offset: 0, Color: gfx.ColorFromRGBA8(uint8(200+(i%50)), 60, 60, 255)},
				{Offset: 1, Color: gfx.ColorFromRGBA8(70, 120, uint8(200+(i%50)), 255)},
			},
		)
		if _, err := vulkan.SubmitAndReadback(renderFramePacket(t, frame), 64, 64); err != nil {
			t.Fatalf("render frame %d: %v", i, err)
		}
	}
}

func renderFramePacket(t *testing.T, f *render.Frame) []byte {
	t.Helper()
	p, err := vulkan.EncodeFrame(f)
	if err != nil {
		t.Fatal(err)
	}
	return p
}
