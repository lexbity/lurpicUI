package vulkan

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/render"
)

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
		return err
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
