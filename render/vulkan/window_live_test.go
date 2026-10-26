//go:build linux && cgo && vulkan_live

package vulkan_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/platform/linux"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// This explicit live-surface gate never skips. It exercises actual platform
// windows, swapchains, acquisition/submission fences and native surface
// switching. Offscreen pixel equivalence remains a separate assertion.
func TestVulkanLiveWindowSetAndRecreation(t *testing.T) {
	app, err := linux.NewApp()
	if err != nil {
		t.Fatal(err)
	}
	defer app.Destroy()
	creator, ok := platform.WindowCapableOf(app)
	if !ok {
		t.Fatal("Linux platform has no window capability")
	}
	create := func(title string) platform.Window {
		t.Helper()
		window, err := creator.NewWindow(platform.WindowOptions{Title: title, Width: 64, Height: 64})
		if err != nil {
			t.Fatal(err)
		}
		window.Show()
		return window
	}
	primary, secondary := create("lurpicUI P6 primary"), create("lurpicUI P6 secondary")
	var backend vulkan.Backend
	if err := backend.Initialize(primary.Surface()); err != nil {
		t.Fatal(err)
	}
	defer backend.Destroy()
	if err := backend.ConfigureWindows([]render.WindowTarget{{ID: render.PrimaryWindow, Surface: primary.Surface()}, {ID: "secondary", Surface: secondary.Surface()}}); err != nil {
		t.Fatal(err)
	}
	info, err := backend.DeviceInfo()
	if err != nil {
		t.Fatal(err)
	}
	t.Log(info)
	submit := func(id string, color gfx.Color) {
		t.Helper()
		frame := testframe.Seal(render.FrameInput{WindowID: id, Batches: []render.BatchInput{{
			Bounds: gfx.RectFromXYWH(0, 0, 64, 64), Opacity: 1,
			Commands: gfx.CommandList{Commands: []gfx.Command{gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 64, 64), Brush: gfx.SolidBrush(color)}}},
		}}})
		defer frame.Release()
		if err := backend.SubmitWindow(id, frame); err != nil {
			t.Fatalf("window %q: %v", id, err)
		}
	}
	for i := 0; i < 4; i++ {
		submit(render.PrimaryWindow, gfx.Color{R: 1, A: 1})
		submit("secondary", gfx.Color{B: 1, A: 1})
	}
	replacement := create("lurpicUI P6 replacement")
	if err := backend.Recreate(replacement.Surface()); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		submit("secondary", gfx.Color{G: 1, A: 1})
		submit(render.PrimaryWindow, gfx.Color{R: 1, B: 1, A: 1})
	}
}
