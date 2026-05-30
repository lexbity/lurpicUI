//go:build linux && cgo

package vulkan_test

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

type fakeVulkanSurface struct {
	width    int
	height   int
	created  bool
	instance uintptr
}

func (s *fakeVulkanSurface) Size() (int, int) { return s.width, s.height }

func (s *fakeVulkanSurface) Resize(width, height int) {
	s.width = width
	s.height = height
}

func (s *fakeVulkanSurface) VulkanInstanceExtensions() []string {
	return []string{"VK_KHR_surface", "VK_KHR_xcb_surface"}
}

func (s *fakeVulkanSurface) CreateVulkanSurface(instance uintptr) (uintptr, error) {
	s.created = true
	s.instance = instance
	return 1, nil
}

func TestVulkanBackendSatisfiesRenderBackend(t *testing.T) {
	var _ render.Backend = (*vulkan.Backend)(nil)
}

func TestVersion_buildsAndCallsRustLibrary(t *testing.T) {
	if err := vulkan.BuildRustLibrary(); err != nil {
		t.Fatalf("BuildRustLibrary: %v", err)
	}

	got, err := vulkan.Version()
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if strings.TrimSpace(got) == "" {
		t.Fatal("expected non-empty version string")
	}
}

func TestBackendInitializeAndDestroy(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Initialize(nil); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	if err := backend.Submit(nil); err != nil {
		t.Fatalf("Submit: %v", err)
	}
	if err := backend.Resize(640, 480); err != nil {
		t.Fatalf("Resize: %v", err)
	}
	backend.Destroy()
}

func TestDeviceInfo_returnsCapabilities(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Initialize(nil); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") || strings.Contains(msg, "no vulkan loader") || strings.Contains(msg, "unsupported") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	defer backend.Destroy()

	info, err := backend.DeviceInfo()
	if err != nil {
		t.Fatalf("DeviceInfo: %v", err)
	}
	if info.Name == "" {
		t.Fatal("expected non-empty device name")
	}
	if info.APIVersion == 0 {
		t.Fatal("expected non-zero API version")
	}
	if info.DriverVersion == 0 {
		t.Fatal("expected non-zero driver version")
	}
	if info.MaxTextureDimension2D == 0 {
		t.Fatal("expected non-zero max texture dimension")
	}
	t.Logf("Device info: %s", info.String())
}

func TestDeviceInfo_failsWhenNotInitialized(t *testing.T) {
	var backend vulkan.Backend
	_, err := backend.DeviceInfo()
	if err == nil {
		t.Fatal("expected error from uninitialized backend")
	}
}

func TestBackendRecreate_afterInitialize(t *testing.T) {
	surface := &fakeVulkanSurface{width: 640, height: 480}
	var backend vulkan.Backend
	if err := backend.Initialize(surface); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") || strings.Contains(msg, "no vulkan loader") || strings.Contains(msg, "unsupported") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	defer backend.Destroy()

	// Recreate with a new surface (simulates Android surface recreation).
	newSurface := &fakeVulkanSurface{width: 800, height: 600}
	if err := backend.Recreate(newSurface); err != nil {
		t.Fatalf("Recreate: %v", err)
	}
	if !newSurface.created {
		t.Fatal("expected new Vulkan surface creation to be requested")
	}
}

func TestBackendRecreate_failsWhenNotInitialized(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Recreate(&fakeVulkanSurface{width: 640, height: 480}); err == nil {
		t.Fatal("expected error when recreating uninitialized backend")
	}
}

func TestBackendRecreate_failsWithoutVulkanSurface(t *testing.T) {
	var backend vulkan.Backend
	if err := backend.Initialize(nil); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") || strings.Contains(msg, "no vulkan loader") || strings.Contains(msg, "unsupported") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	defer backend.Destroy()

	// A nil surface (no VulkanSurface interface) should fail.
	if err := backend.Recreate(nil); err == nil {
		t.Fatal("expected error when recreating with nil surface")
	}
}

func TestBackendInitializeUsesVulkanSurfaceWhenAvailable(t *testing.T) {
	surface := &fakeVulkanSurface{width: 640, height: 480}
	var backend vulkan.Backend
	if err := backend.Initialize(surface); err != nil {
		msg := strings.ToLower(err.Error())
		if strings.Contains(msg, "vkresult -9") || strings.Contains(msg, "vkresult -3") || strings.Contains(msg, "no suitable vulkan physical device") || strings.Contains(msg, "no vulkan physical devices") {
			t.Skipf("Vulkan unavailable on this machine: %v", err)
		}
		t.Fatalf("Initialize: %v", err)
	}
	if !surface.created {
		t.Fatal("expected Vulkan surface creation to be requested")
	}
	if surface.instance == 0 {
		t.Fatal("expected a non-zero Vulkan instance handle")
	}
	backend.Destroy()
}
