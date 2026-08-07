//go:build linux && cgo

package vulkan_test

import (
	"errors"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// rectFrame builds a single-rect frame. `dirty` optionally sets the frame's
// dirty regions (Slice 10, Q13).
func rectFrame(dirty ...gfx.Rect) *render.Frame {
	return &render.Frame{
		DirtyRegions: dirty,
		RenderBatchs: []render.RenderBatch{{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
			Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{
					Rect:  gfx.RectFromXYWH(10, 10, 20, 20),
					Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(230, 60, 60, 255)),
				},
			}},
		}},
	}
}

// TestVulkanSubmitAndReadback_DeviceLostReturnsErrGPUFatal proves the device
// fault surface of FR-10: after InjectDeviceLost, the next render fails with
// *render.ErrGPUFatal (not a generic error), the device generation bumps, and a
// subsequent clean render still works.
func TestVulkanSubmitAndReadback_DeviceLostReturnsErrGPUFatal(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	const w, h = 64, 64
	frame := rectFrame()
	packet, err := vulkan.EncodeFrame(frame)
	if err != nil {
		t.Fatalf("encode: %v", err)
	}

	if _, err := vulkan.SubmitAndReadback(packet, w, h); err != nil {
		t.Fatalf("baseline readback: %v", err)
	}
	before := vulkan.DeviceGeneration()

	if err := vulkan.InjectDeviceLost(); err != nil {
		t.Fatalf("inject device lost: %v", err)
	}
	_, err = vulkan.SubmitAndReadback(packet, w, h)
	var fatal *render.ErrGPUFatal
	if !errors.As(err, &fatal) {
		t.Fatalf("device loss must surface as *render.ErrGPUFatal, got %v", err)
	}
	if vulkan.DeviceGeneration() <= before {
		t.Fatalf("device generation must bump on device loss: before=%d after=%d", before, vulkan.DeviceGeneration())
	}

	// The injected fault is one-shot; a subsequent render succeeds.
	if _, err := vulkan.SubmitAndReadback(packet, w, h); err != nil {
		t.Fatalf("post-fatal readback must succeed: %v", err)
	}
}

// TestDirtyRegions_DesktopConsumed proves the GPU consumes the frame's dirty
// regions (Q13) on non-tile-based devices: a draw outside the dirty union is
// culled (the readback target clears to transparent there), while a draw inside
// a dirty region renders. Skipped when the device gates dirty-region redraw off
// (tile-based mobile).
func TestDirtyRegions_DesktopConsumed(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() { _ = vulkan.Shutdown() }()

	features, err := vulkan.QueryPipelineFeatures()
	if err != nil {
		t.Fatalf("query pipeline features: %v", err)
	}
	if features.TileBased != 0 {
		t.Skipf("device is tile-based; dirty-region redraw is gated off (Q13)")
	}

	const w, h = 64, 64

	// The rect at (10,10)-(30,30) with a dirty region covering it renders.
	covered, err := vulkan.EncodeFrame(rectFrame(gfx.RectFromXYWH(0, 0, 40, 40)))
	if err != nil {
		t.Fatalf("encode covered: %v", err)
	}
	coveredPixels, err := vulkan.SubmitAndReadback(covered, w, h)
	if err != nil {
		t.Fatalf("covered readback: %v", err)
	}
	if a := pixelAt(coveredPixels, w, 15, 15)[3]; a == 0 {
		t.Fatal("draw inside the dirty region must render")
	}

	// The same rect with a dirty region that does NOT cover it is culled: the
	// rect's area stays at the clear (transparent) background.
	uncovered, err := vulkan.EncodeFrame(rectFrame(gfx.RectFromXYWH(50, 50, 60, 60)))
	if err != nil {
		t.Fatalf("encode uncovered: %v", err)
	}
	uncoveredPixels, err := vulkan.SubmitAndReadback(uncovered, w, h)
	if err != nil {
		t.Fatalf("uncovered readback: %v", err)
	}
	if a := pixelAt(uncoveredPixels, w, 15, 15)[3]; a != 0 {
		t.Fatalf("draw outside every dirty region must be culled, got alpha %d", a)
	}
}
