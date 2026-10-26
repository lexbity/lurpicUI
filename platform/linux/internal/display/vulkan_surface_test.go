//go:build linux && cgo && vulkan_device

package display

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func TestVulkanNativeWindowPresentationAndSwapchainResize(t *testing.T) {
	platformApp, err := NewApp()
	if err != nil {
		t.Fatalf("open required X11 platform for Vulkan surface test: %v", err)
	}
	a := platformApp.(*app)
	defer a.Destroy()

	window, err := a.NewWindow(platform.WindowOptions{Title: "lurpicui-p9-vulkan", Width: 160, Height: 120})
	if err != nil {
		t.Fatalf("create Vulkan presentation window: %v", err)
	}
	defer window.Destroy()
	window.Show()

	surface := window.Surface()
	if _, ok := surface.(render.VulkanSurface); !ok {
		t.Fatal("native window surface does not implement the Vulkan surface contract")
	}
	backend := new(vulkan.Backend)
	if err := backend.Initialize(surface); err != nil {
		t.Fatalf("initialize Vulkan against the native XCB surface: %v", err)
	}
	defer backend.Destroy()
	if err := backend.BuildPipelineProbe(); err != nil {
		t.Fatalf("build pipeline for the real swapchain format: %v", err)
	}
	startGeneration := backend.DeviceGeneration()
	if startGeneration == 0 {
		t.Fatal("Vulkan device generation is zero after native surface initialization")
	}

	for _, size := range [][2]int{{160, 120}, {320, 240}} {
		if size != [2]int{160, 120} {
			if err := backend.Resize(size[0], size[1]); err != nil {
				t.Fatalf("recreate native swapchain at %dx%d: %v", size[0], size[1], err)
			}
		}
		frame := testframe.Seal(render.FrameInput{WindowID: render.PrimaryWindow, Batches: []render.BatchInput{{
			Bounds: gfx.RectFromXYWH(0, 0, float32(size[0]), float32(size[1])), Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{Rect: gfx.RectFromXYWH(8, 8, float32(size[0]-16), float32(size[1]-16)), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(32, 96, 192, 255))},
			}},
		}}})
		if err := backend.Submit(frame); err != nil {
			frame.Release()
			t.Fatalf("present frame on %dx%d native Vulkan surface: %v", size[0], size[1], err)
		}
		frame.Release()
	}
	if generation := backend.DeviceGeneration(); generation != startGeneration {
		t.Fatalf("swapchain resize changed device generation: %d -> %d", startGeneration, generation)
	}
	usage, err := vulkan.ResourceSnapshot()
	if err != nil {
		t.Fatal(err)
	}
	if usage.TotalManaged() > usage.BudgetBytes {
		t.Fatalf("native presentation exceeded the Vulkan wallet: managed=%d budget=%d", usage.TotalManaged(), usage.BudgetBytes)
	}
}
