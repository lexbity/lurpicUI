package render

import (
	"fmt"

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

// DeviceGenerationProvider is optionally implemented by render backends that
// expose a monotonically-increasing device generation counter. The runtime
// compares this across frames to detect device-lost / swapchain-rebuild events
// and invalidate GPU-cached texture IDs that reference dead resources.
type DeviceGenerationProvider interface {
	DeviceGeneration() uint64
}

// ErrGPUFatal is returned by Backend.Submit when the GPU backend enters an
// unrecoverable state (device lost, unrecoverable driver fault, fatal
// out-of-memory). The runtime treats it as a one-shot signal (Q9/FR-10): it
// destroys the GPU backend and swaps in the software backend for the rest of
// the session. It is never auto-retried — retrying a flaky GPU every frame
// would stall the render thread.
type ErrGPUFatal struct {
	// Err is the underlying renderer error that caused the fatal failure.
	Err error
}

func (e *ErrGPUFatal) Error() string {
	if e == nil || e.Err == nil {
		return "render: gpu fatal error"
	}
	return fmt.Sprintf("render: gpu fatal error: %v", e.Err)
}

func (e *ErrGPUFatal) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
