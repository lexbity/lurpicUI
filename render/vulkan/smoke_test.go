//go:build linux && cgo

package vulkan_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testframe"
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

// TestVulkanHeadlessPacketAdmission checks native packet decoding separately
// from surface readiness. A backend with no native surface must refuse a
// promised render; the direct FFI admission path is an encoding test only.
func TestVulkanHeadlessPacketAdmission(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Initialize(nil); err != nil {
		if isVulkanUnsupported(err) {
			t.Skipf("Vulkan unavailable: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	defer backend.Destroy()

	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{}})
	defer frame.Release()
	if err := backend.Submit(frame); err == nil {
		t.Fatal("headless backend reported a successful render")
	}
	packet, err := vulkan.EncodeFrame(frame)
	if err != nil {
		t.Fatal(err)
	}
	if err := vulkan.SubmitFrame(packet); err != nil {
		t.Fatalf("native packet admission: %v", err)
	}
}
