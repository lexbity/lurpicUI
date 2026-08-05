//go:build linux && cgo

package vulkan_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

// isVulkanUnsupported reports whether the initialization error means Vulkan is
// simply not available on this machine (no ICD, no suitable physical device,
// unsupported result code). Such failures are environmental, not bugs, so the
// smoke test skips instead of failing.
func isVulkanUnsupported(err error) bool {
	if err == nil {
		return false
	}
	if vulkan.IsUnsupported(err) {
		return true
	}
	msg := strings.ToLower(err.Error())
	for _, frag := range []string{
		"vkresult -9",
		"vkresult -3",
		"no suitable vulkan physical device",
		"no vulkan physical devices",
		"no vulkan loader",
		"unsupported",
	} {
		if strings.Contains(msg, frag) {
			return true
		}
	}
	return false
}

// TestVulkanBackend_InitAndSubmitSmoke proves the Vulkan backend initializes a
// loader/instance/device and submits a minimal frame through the FFI on machines
// with a working Vulkan driver, skipping gracefully elsewhere.
//
// The backend is initialized without a platform surface: the Present path needs
// a real native window surface (a test fake cannot back a real swapchain), while
// the no-surface path still exercises the full loader + instance + device +
// frame-submit pipeline that this smoke is for. It asserts only init + submit
// success — true software<->Vulkan pixel parity needs an offscreen readback path
// and is out of scope (see the readiness spec, NG-3).
func TestVulkanBackend_InitAndSubmitSmoke(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Initialize(nil); err != nil {
		if isVulkanUnsupported(err) {
			t.Skipf("Vulkan unavailable: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	defer backend.Destroy()

	frame := &render.Frame{RenderBatchs: []render.RenderBatch{}}
	if err := backend.Submit(frame); err != nil {
		t.Fatalf("Submit: %v", err)
	}
}
