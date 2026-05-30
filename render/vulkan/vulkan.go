package vulkan

import (
	"errors"
	"fmt"

	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan/internal"
)

// IsUnsupported reports whether err indicates the Vulkan backend is not
// available on this system (no ICD, no Vulkan 1.1+, etc.).
func IsUnsupported(err error) bool {
	return internal.IsUnsupported(err)
}

var errNotImplemented = errors.New("vulkan backend: not implemented")

type Backend struct {
	initialized bool
	hasSurface  bool
	images      *imageCache
}

func (b *Backend) Initialize(s render.Surface) error {
	if b.initialized {
		b.Destroy()
	}
	if err := Init(); err != nil {
		if internal.IsUnsupported(err) {
			return err
		}
		return fmt.Errorf("vulkan backend: init: %w", err)
	}
	b.images = newImageCache()
	if vs, ok := s.(render.VulkanSurface); ok {
		instance := InstanceHandle()
		if instance == 0 {
			_ = Shutdown()
			return errors.New("vulkan backend: instance unavailable after init")
		}
		if surface, err := vs.CreateVulkanSurface(instance); err != nil {
			_ = Shutdown()
			return err
		} else if surface != 0 {
			b.hasSurface = true
		}
	}
	b.initialized = true
	return nil
}

func (b *Backend) Submit(f *render.Frame) error {
	if !b.initialized {
		return errNotImplemented
	}
	if f != nil {
		packet, err := encodeFramePacketWithAssets(f, b.images)
		if err != nil {
			return err
		}
		if err := SubmitFrame(packet); err != nil {
			return err
		}
	}
	if !b.hasSurface {
		return nil
	}
	return Present()
}

// Recreate rebuilds the Vulkan surface + swapchain for a new platform Surface.
// Used on Android when the native window is recreated after pause/resume or
// configuration change. On desktop this calls the platform's CreateVulkanSurface
// to recreate the surface for the new window, then Resize to rebuild the swapchain.
func (b *Backend) Recreate(s render.Surface) error {
	if !b.initialized {
		return errors.New("vulkan backend: not initialized")
	}
	vs, ok := s.(render.VulkanSurface)
	if !ok {
		return errors.New("vulkan backend: surface does not support Vulkan")
	}
	instance := InstanceHandle()
	if instance == 0 {
		return errors.New("vulkan backend: instance unavailable")
	}
	w, h := s.Size()

	// Destroy the old surface state and create a fresh one. On Android this
	// calls into Rust which does lurpic_render_recreate_surface_android.
	surface, err := vs.CreateVulkanSurface(instance)
	if err != nil {
		return fmt.Errorf("vulkan backend: recreate surface: %w", err)
	}
	if surface == 0 {
		b.hasSurface = false
		return errors.New("vulkan backend: recreate returned zero surface")
	}
	b.hasSurface = true

	// Resize the swapchain to match the new surface dimensions.
	if err := Resize(w, h); err != nil {
		return fmt.Errorf("vulkan backend: recreate resize: %w", err)
	}
	return nil
}

func (b *Backend) Resize(w, h int) error {
	if !b.initialized {
		return errNotImplemented
	}
	if !b.hasSurface {
		return nil
	}
	return Resize(w, h)
}

func (b *Backend) Destroy() {
	if b.initialized {
		if b.images != nil {
			b.images.destroyAll()
			b.images = nil
		}
		_ = Shutdown()
		b.initialized = false
		b.hasSurface = false
	}
}

// EvictCaches releases recoverable image caches without destroying the backend.
func (b *Backend) EvictCaches() {
	if b == nil || b.images == nil {
		return
	}
	b.images.destroyAll()
}

var _ render.Backend = (*Backend)(nil)
var _ render.CacheEvictor = (*Backend)(nil)
var _ render.RecreatableBackend = (*Backend)(nil)
var _ render.TextureBackend = (*Backend)(nil)

func (b *Backend) UploadTexture(req render.TextureUploadRequest) (render.TextureID, error) {
	vb := render.NewVulkanBackend(0)
	return vb.UploadTexture(req)
}

func (b *Backend) FreeTexture(id render.TextureID) {
	vb := render.NewVulkanBackend(0)
	vb.FreeTexture(id)
}

func (b *Backend) UploadBudgetBytesPerFrame() int {
	vb := render.NewVulkanBackend(0)
	return vb.UploadBudgetBytesPerFrame()
}

func (b *Backend) TranscodeTarget() render.TextureFormat {
	vb := render.NewVulkanBackend(0)
	return vb.TranscodeTarget()
}
