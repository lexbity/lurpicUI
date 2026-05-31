package render

import (
	"errors"
	"fmt"
)

// VulkanBackend uploads textures through a Vulkan-capable hook registered by render/vulkan.
type VulkanBackend struct {
	target TextureFormat
}

// NewVulkanBackend returns a Vulkan texture backend with the provided transcode target.
func NewVulkanBackend(target TextureFormat) *VulkanBackend {
	if target != 0 && target != TextureFormatASTC4x4 && target != TextureFormatBC7 {
		target = TextureFormatBC7
	}
	return &VulkanBackend{target: target}
}

func (b *VulkanBackend) UploadTexture(req TextureUploadRequest) (TextureID, error) {
	if req.Width == 0 || req.Height == 0 {
		return 0, errors.New("vulkan backend: invalid texture dimensions")
	}
	if len(req.PixelData) == 0 {
		return 0, errors.New("vulkan backend: empty pixel data")
	}
	switch req.Format {
	case TextureFormatRGBA8, TextureFormatASTC4x4, TextureFormatBC7:
		// Accepted — the backend stores data as RGBA8 in host memory
		// regardless of the upload format hint. The format is used
		// for GPU budget accounting (see uploadGPUBytes).
	default:
		return 0, fmt.Errorf("vulkan backend: unsupported source format %s", req.Format)
	}
	hooks := currentVulkanHooks()
	if hooks.Upload == nil {
		return 0, errors.New("vulkan backend: no upload hook registered")
	}
	return hooks.Upload(req)
}

func (b *VulkanBackend) FreeTexture(id TextureID) {
	if id == 0 {
		return
	}
	hooks := currentVulkanHooks()
	if hooks.Free != nil {
		hooks.Free(id)
	}
}

func (b *VulkanBackend) UploadBudgetBytesPerFrame() int {
	hooks := currentVulkanHooks()
	if hooks.Budget != nil {
		return hooks.Budget(b.TranscodeTarget())
	}
	if b != nil && b.target == TextureFormatASTC4x4 {
		return 4 << 20
	}
	return 16 << 20
}

func (b *VulkanBackend) TranscodeTarget() TextureFormat {
	if b != nil && b.target != 0 {
		return b.target
	}
	hooks := currentVulkanHooks()
	if hooks.Target != nil {
		return hooks.Target()
	}
	return TextureFormatBC7
}

var _ TextureBackend = (*VulkanBackend)(nil)
