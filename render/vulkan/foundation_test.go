//go:build linux && cgo

package vulkan_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func TestQueryPipelineFeatures_ReportsDynamicRendering(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()

	features, err := vulkan.QueryPipelineFeatures()
	if err != nil {
		t.Fatalf("QueryPipelineFeatures: %v", err)
	}
	// Vulkan 1.3 core: dynamic rendering / synchronization2 are mandatory
	// features of the device we select (Slice 2 requires 1.3, Q7).
	if features.DynamicRendering != 1 {
		t.Fatalf("dynamic_rendering = %d, want 1 (Vulkan 1.3 device)", features.DynamicRendering)
	}
	if features.Synchronization2 != 1 {
		t.Fatalf("synchronization2 = %d, want 1 (Vulkan 1.3 device)", features.Synchronization2)
	}
	if features.ExtendedDynamicState != 1 {
		t.Fatalf("extended_dynamic_state = %d, want 1 (Vulkan 1.3 device)", features.ExtendedDynamicState)
	}
	if features.StencilFill != 1 {
		t.Fatalf("stencil_fill = %d, want 1 (stencil is core)", features.StencilFill)
	}
}

func TestQueryPipelineFeatures_RequiresInit(t *testing.T) {
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}
	if _, err := vulkan.QueryPipelineFeatures(); err == nil {
		t.Fatal("expected error when querying pipeline features before init")
	}
}

func TestDeviceGeneration_LinuxNonZero(t *testing.T) {
	requireVulkanAvailable(t)
	defer func() {
		if err := vulkan.Shutdown(); err != nil {
			t.Fatalf("shutdown: %v", err)
		}
	}()
	// The Linux dlsym path must resolve the real symbol (not the old weak stub).
	if gen := vulkan.DeviceGeneration(); gen == 0 {
		t.Fatal("device generation must be non-zero after init (non-stub symbol)")
	}
}

func TestGPUShutdown_NoValidationError(t *testing.T) {
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("pre-test shutdown: %v", err)
	}
	// Enable validation for the whole init/shutdown cycle.
	if err := vulkan.SetValidation(true); err != nil {
		t.Fatalf("SetValidation(true): %v", err)
	}
	defer func() {
		_ = vulkan.SetValidation(false)
	}()

	before, err := vulkan.TestValidationErrorCount()
	if err != nil {
		t.Skipf("test-exports unavailable: %v", err)
	}

	err = vulkan.Init()
	if err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") ||
			strings.Contains(msg, "no suitable vulkan physical device") ||
			strings.Contains(msg, "no vulkan physical devices") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("init with validation: %v", err)
	}
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("shutdown: %v", err)
	}

	after, err := vulkan.TestValidationErrorCount()
	if err != nil {
		t.Fatalf("validation error count: %v", err)
	}
	if after != before {
		t.Fatalf("init+shutdown produced %d validation errors (before=%d after=%d)",
			after-before, before, after)
	}
}

func TestEntryLoad_MissingVulkanSurfacesCleanly(t *testing.T) {
	if err := vulkan.Shutdown(); err != nil {
		t.Fatalf("shutdown before test: %v", err)
	}
	t.Setenv("LURPIC_RENDER_VULKAN_LIBRARY", "/definitely/not/a/real/libvulkan.so")
	err := vulkan.Init()
	if err == nil {
		t.Fatal("expected init to fail when the Vulkan loader path is invalid")
	}
	// The ash Entry::load() failure must surface as an Unsupported/init error
	// with a useful message, which initBackend catches to select software.
	msg := err.Error()
	if !strings.Contains(strings.ToLower(msg), "vulkan") && !strings.Contains(strings.ToLower(msg), "loader") {
		t.Fatalf("expected a useful Vulkan loader error, got %q", msg)
	}
}
