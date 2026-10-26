package vulkan

import (
	"errors"
	"fmt"

	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan/internal"
)

// DeviceInfo summarizes the Vulkan physical device capabilities.
type DeviceInfo struct {
	Name                  string
	DeviceType            int32
	APIVersion            uint32
	DriverVersion         uint32
	MaxTextureDimension2D uint32
}

func (d DeviceInfo) String() string {
	typeNames := map[int32]string{
		0: "other",
		1: "integrated",
		2: "discrete",
		3: "virtual",
		4: "cpu",
	}
	tname := typeNames[d.DeviceType]
	if tname == "" {
		tname = "unknown"
	}
	return fmt.Sprintf("Vulkan %d.%d.%d | %s | driver 0x%x | %s GPU",
		d.APIVersion>>22,
		(d.APIVersion>>12)&0x3ff,
		d.APIVersion&0xfff,
		d.Name,
		d.DriverVersion,
		tname,
	)
}

// IsUnsupported reports whether err indicates the Vulkan backend is not
// available on this system (no ICD, no Vulkan 1.1+, etc.).
func IsUnsupported(err error) bool {
	return internal.IsUnsupported(err)
}

var errNotImplemented = errors.New("vulkan backend: not implemented")

type Backend struct {
	// Backend owns the Vulkan process-wide lifecycle in this Go wrapper.
	// Initialize/Destroy may be called multiple times across surface changes,
	// while Recreate handles the lighter surface/swapchain-only path.
	surfaces    map[string]uintptr
	targets     []render.WindowTarget
	initialized bool
	hasSurface  bool
	images      *imageCache
	// Reused across submits so the per-frame submission performs zero heap
	// allocations steady-state (NFR-6). Render-thread only.
	encoder frameEncoder
}

// Initialize brings up the Vulkan instance and optionally binds a platform
// surface. If the surface supports Vulkan presentation, the backend creates a
// native surface immediately so the runtime can submit frames on the same turn.
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
	b.surfaces = make(map[string]uintptr)
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
			b.surfaces[render.PrimaryWindow] = surface
		}
	}
	b.initialized = true
	return nil
}

// EncodeFrame encodes a Frame into the packet v2 wire format, uploading any
// DrawImage pixel data into the Rust image store as needed. Used by the
// equivalence harness and any caller that needs the packet independent of a
// live Backend instance.
func EncodeFrame(f *render.Frame) ([]byte, error) {
	return encodeFramePacketWithAssets(f, newImageCache())
}

func (b *Backend) Submit(f *render.Frame) error {
	if !b.initialized {
		return errNotImplemented
	}
	if f != nil {
		handle := b.surfaces[f.WindowID()]
		if handle == 0 {
			return fmt.Errorf("vulkan: window %q has no live native surface", f.WindowID())
		}
		if err := ActivateSurface(handle); err != nil {
			return err
		}
		packet, err := b.encoder.Encode(f, b.images)
		if err != nil {
			return err
		}
		if err := SubmitFrame(packet); err != nil {
			return translateSubmitError(err)
		}
	}
	return nil
}

// translateSubmitError maps a Rust-side submit failure to the render contract:
// device-lost and out-of-memory are unrecoverable GPU faults and surface as
// *render.ErrGPUFatal (FR-10), which the runtime answers with a one-shot
// GPU→software fallback. Other failures (encode/parse) are returned unchanged.
//
// The match is a type switch on the un-wrapped errors translateStatus returns
// (never errors.As) so the steady-state success path performs zero allocations
// (NFR-6): each errors.As branch boxes a target pointer and escapes to the heap.
func translateSubmitError(err error) error {
	switch err.(type) { //nolint:errorlint // translateStatus returns un-wrapped typed errors; errors.As would box a target pointer and allocate (NFR-6)
	case *internal.DeviceLostError, *internal.OutOfMemoryError:
		return &render.ErrGPUFatal{Err: err}
	default:
		return err
	}
}

// BuildPipelineProbe builds the renderer's solid pipeline against the current
// swapchain format, proving the device can construct the renderer's graphics
// pipelines (FR-11: honest capability selection — an actual pipeline build, not
// a symbol or feature check). app/initBackend calls it after Initialize and
// selects the software backend upfront when it fails.
func (b *Backend) BuildPipelineProbe() error {
	if !b.initialized {
		return errors.New("vulkan backend: not initialized")
	}
	return BuildPipelineProbe()
}

// Recreate rebuilds the Vulkan surface + swapchain for a new platform Surface.
// The runtime uses this when the platform surface changes but the Vulkan device
// itself can stay alive. That is cheaper than a full Initialize/Destroy cycle.
func (b *Backend) Recreate(surface render.Surface) error {
	if !b.initialized {
		return errors.New("vulkan backend: not initialized")
	}
	if _, ok := surface.(render.VulkanSurface); !ok {
		return errors.New("vulkan backend: surface does not support Vulkan")
	}
	targets := append([]render.WindowTarget(nil), b.targets...)
	if err := b.Initialize(surface); err != nil {
		return err
	}
	if len(targets) > 1 {
		for i := range targets {
			if targets[i].ID == render.PrimaryWindow {
				targets[i].Surface = surface
			}
		}
		return b.ConfigureWindows(targets)
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
		b.surfaces = nil
		b.targets = nil
	}
}

// DeviceInfo returns the Vulkan physical device capabilities, or an error
// if the backend is not initialized or the device is inaccessible.
func (b *Backend) DeviceInfo() (DeviceInfo, error) {
	if !b.initialized {
		return DeviceInfo{}, errors.New("vulkan backend: not initialized")
	}
	caps, err := QueryCapabilities()
	if err != nil {
		return DeviceInfo{}, err
	}
	return DeviceInfo{
		Name:                  caps.DeviceName,
		DeviceType:            caps.DeviceType,
		APIVersion:            caps.APIVersion,
		DriverVersion:         caps.DriverVersion,
		MaxTextureDimension2D: caps.MaxTextureDimension2D,
	}, nil
}

// DeviceGeneration returns a monotonically-increasing counter that increments
// on every device init and surface/swapchain recreation. The runtime compares
// this across frames to detect device-lost conditions and invalidate GPU-cached
// texture IDs that reference dead resources.
func (b *Backend) DeviceGeneration() uint64 {
	return DeviceGeneration()
}

// EvictCaches releases recoverable image and glyph caches without destroying
// the backend. Clears both the image cache and the Rust-side glyph atlas.
func (b *Backend) EvictCaches() {
	if b == nil || b.images == nil {
		return
	}
	b.images.destroyAll()
	ResetAtlas()
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

func (b *Backend) ConfigureWindows(targets []render.WindowTarget) error {
	if !b.initialized || len(targets) == 0 || len(targets) > render.MaxFrameWindows {
		return errors.New("vulkan: invalid window configuration")
	}
	if len(b.surfaces) > 1 {
		return errors.New("vulkan: window configuration is already installed")
	}
	seen := make(map[string]bool, len(targets))
	for _, target := range targets {
		if target.ID == "" || seen[target.ID] || target.Surface == nil {
			return fmt.Errorf("vulkan: invalid window target %q", target.ID)
		}
		seen[target.ID] = true
		if _, ok := target.Surface.(render.VulkanSurface); !ok {
			return fmt.Errorf("vulkan: window %q lacks native surface support", target.ID)
		}
	}
	if !seen[render.PrimaryWindow] || b.surfaces[render.PrimaryWindow] == 0 {
		return errors.New("vulkan: primary native surface unavailable")
	}
	configured := false
	defer func() {
		if !configured {
			b.Destroy()
		}
	}()
	for _, target := range targets {
		if target.ID == render.PrimaryWindow {
			continue
		}
		native := target.Surface.(render.VulkanSurface)
		handle, err := native.CreateVulkanSurface(InstanceHandle())
		if err != nil {
			return fmt.Errorf("vulkan: create window %q: %w", target.ID, err)
		}
		if handle == 0 {
			return fmt.Errorf("vulkan: window %q returned no surface", target.ID)
		}
		b.surfaces[target.ID] = handle
	}
	b.targets = append([]render.WindowTarget(nil), targets...)
	if err := ActivateSurface(b.surfaces[render.PrimaryWindow]); err != nil {
		return err
	}
	configured = true
	return nil
}
func (b *Backend) SubmitWindow(id string, frame *render.Frame) error {
	if frame == nil || frame.WindowID() != id {
		return fmt.Errorf("render: packet window does not match target %q", id)
	}
	if !b.initialized {
		return errNotImplemented
	}
	return b.Submit(frame)
}

var _ render.WindowBackend = (*Backend)(nil)
