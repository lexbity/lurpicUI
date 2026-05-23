package render

import (
	"fmt"
	"sync"
)

// TextureID is an opaque handle to a backend texture resource.
type TextureID uint64

// TextureFormat identifies the upload/transcode format expected by a backend.
type TextureFormat uint8

const (
	TextureFormatRGBA8 TextureFormat = iota
	TextureFormatASTC4x4
	TextureFormatBC7
)

func (f TextureFormat) String() string {
	switch f {
	case TextureFormatRGBA8:
		return "rgba8"
	case TextureFormatASTC4x4:
		return "astc4x4"
	case TextureFormatBC7:
		return "bc7"
	default:
		return fmt.Sprintf("TextureFormat(%d)", uint8(f))
	}
}

// TextureUploadRequest transfers pixel data to a backend texture store.
type TextureUploadRequest struct {
	AssetID   uint64
	PixelData []byte
	Width     uint16
	Height    uint16
	Format    TextureFormat
	MipLevels uint8
	ResultCh  chan<- TextureUploadResult
}

// TextureUploadResult reports the backend handle created for an upload request.
type TextureUploadResult struct {
	AssetID   uint64
	TextureID TextureID
	LOD       int
	Err       error
}

// TextureBackend abstracts texture uploads and releases.
type TextureBackend interface {
	UploadTexture(req TextureUploadRequest) (TextureID, error)
	FreeTexture(id TextureID)
	UploadBudgetBytesPerFrame() int
	TranscodeTarget() TextureFormat
}

// VulkanTextureHooks lets the Vulkan renderer package provide real upload/free behavior
// without introducing an import cycle into the root render package.
type VulkanTextureHooks struct {
	Upload func(req TextureUploadRequest) (TextureID, error)
	Free   func(id TextureID)
	Budget func(target TextureFormat) int
	Target func() TextureFormat
}

var (
	vulkanHooksMu sync.RWMutex
	vulkanHooks   VulkanTextureHooks
)

// RegisterVulkanTextureHooks installs the Vulkan texture bridge.
func RegisterVulkanTextureHooks(h VulkanTextureHooks) {
	vulkanHooksMu.Lock()
	defer vulkanHooksMu.Unlock()
	vulkanHooks = h
}

func currentVulkanHooks() VulkanTextureHooks {
	vulkanHooksMu.RLock()
	defer vulkanHooksMu.RUnlock()
	return vulkanHooks
}
