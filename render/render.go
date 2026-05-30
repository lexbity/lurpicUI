package render

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

var _ = text.GlyphRun{}

type Surface interface {
	Size() (width, height int)
	Resize(width, height int)
}

// VulkanSurface exposes backend-specific native surface creation for Vulkan-capable surfaces.
type VulkanSurface interface {
	Surface
	VulkanInstanceExtensions() []string
	CreateVulkanSurface(instance uintptr) (uintptr, error)
}

// SoftwareSurface extends Surface with direct pixel access for the software renderer.
type SoftwareSurface interface {
	Surface
	Buffer() []byte
	Stride() int
	Lock() error
	Unlock(dirtyRects []gfx.Rect) error
}

type RenderBatchID uint64

type RenderBatch struct {
	ID          RenderBatchID
	Bounds      gfx.Rect
	Opacity     float32
	Commands    gfx.CommandList
	CommandHash uint64
}

// LayeredBatch groups render batches by layer order and clip rect.
type LayeredBatch struct {
	RenderOrder int
	ClipRect    gfx.Rect
	Batches     []RenderBatch
}

// FramePacket carries layer-ordered batches.
type FramePacket struct {
	Layers []LayeredBatch
}

type Frame struct {
	FramePacket
	RenderBatchs []RenderBatch
	DirtyRegions []gfx.Rect
}

type Backend interface {
	Initialize(surface Surface) error
	Submit(frame *Frame) error
	Resize(width, height int) error
	Destroy()
}

// CacheEvictor is implemented by render backends that retain recoverable caches.
// It allows the runtime to ask the backend to release memory under pressure
// without tearing the backend down completely.
type CacheEvictor interface {
	EvictCaches()
}

// RecreatableBackend is optionally implemented by render backends that can
// rebuild their surface + swapchain in-place without a full re-initialization.
// This is used on lifecycle-based platforms (e.g. Android) where the native
// window surface is destroyed and recreated during pause/resume cycles.
type RecreatableBackend interface {
	Recreate(surface Surface) error
}
