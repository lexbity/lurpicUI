//go:build linux && cgo

package vulkan_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func TestInitShutdownRepeated(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("final shutdown: %v", err)
		}
	}()
	for i := 0; i < 3; i++ {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown %d: %v", i, err)
		}
		if err := vulkan.Init(); err != nil {
			t.Fatalf("init %d: %v", i, err)
		}
	}
}

func TestQueryCapabilitiesReturnsDeviceInfo(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	caps, err := vulkan.QueryCapabilities()
	if err != nil {
		t.Fatalf("QueryCapabilities: %v", err)
	}
	if strings.TrimSpace(caps.DeviceName) == "" {
		t.Fatal("expected non-empty device name")
	}
	if caps.MaxTextureDimension2D == 0 {
		t.Fatal("expected a non-zero max texture dimension")
	}
}

func TestInitFailsWithInvalidLoaderPath(t *testing.T) {
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("shutdown before failure test: %v", err)
	}
	t.Setenv("LURPIC_RENDER_VULKAN_LIBRARY", "/definitely/not/a/real/libvulkan.so")
	err := vulkan.Init()
	if err == nil {
		t.Fatal("expected init to fail with invalid Vulkan loader path")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "vulkan") && !strings.Contains(strings.ToLower(err.Error()), "loader") {
		t.Fatalf("expected useful Vulkan loader error, got %v", err)
	}
}

func requireVulkanAvailable(t *testing.T) {
	t.Helper()
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("pre-test shutdown: %v", err)
	}
	if err := vulkan.Init(); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("init: %v", err)
	}
}
