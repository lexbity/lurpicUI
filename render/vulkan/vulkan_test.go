package vulkan_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

func TestVulkanBackendSatisfiesRenderBackend(t *testing.T) {
	var _ render.Backend = (*vulkan.Backend)(nil)
}
