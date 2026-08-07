package runtime

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
)

// backendFallbackSink is defined in config.go next to DiagnosticsHook (see
// OnBackendFallback); the runtime emits the fallback diagnostic to any hook
// that structurally opts in.

// platformSurfaceProvider is implemented by platforms that deliver their
// surface through lifecycle events (e.g. Android) rather than a window.
type platformSurfaceProvider interface {
	Surface() platform.Surface
}

// currentSurface returns the live platform surface (the window's surface on
// desktop, the lifecycle-delivered surface otherwise), or nil when absent.
// Fields read here are immutable after construction, so this is safe to call
// from the render thread during a fallback swap.
func (rt *Runtime) currentSurface() render.Surface {
	if rt.window != nil {
		if s := rt.window.Surface(); s != nil {
			return s
		}
	}
	if sp, ok := rt.platformApp.(platformSurfaceProvider); ok {
		if s := sp.Surface(); s != nil {
			return s
		}
	}
	return nil
}

// softwareFallbackBackend is the RenderPipeline's fallback factory: it binds a
// freshly-built software backend (from Config.SoftwareBackendFactory, owned by
// the app layer) to the current platform surface (FR-12). Called on the render
// thread after a GPU-fatal submit error.
func (rt *Runtime) softwareFallbackBackend() (render.Backend, error) {
	if rt.config.SoftwareBackendFactory == nil {
		return nil, errors.New("runtime: no software backend factory configured for the GPU-fatal fallback")
	}
	surface := rt.currentSurface()
	if surface == nil {
		return nil, errors.New("runtime: no platform surface for software fallback")
	}
	return rt.config.SoftwareBackendFactory(surface)
}

// notifyBackendFallback is the RenderPipeline's onFallback hook: logs the
// swap, emits the OnBackendFallback diagnostic (for hooks that opt in), and
// re-wires the asset texture releaser/uploader to the software backend so
// texture eviction and upload keep working after the swap.
func (rt *Runtime) notifyBackendFallback(from, reason string) {
	rt.log.Warn("runtime: GPU render backend failed; falling back to software for the session",
		"from", from, "reason", reason)
	if diag := rt.diagnosticsHook(); diag != nil {
		if sink, ok := diag.(backendFallbackSink); ok {
			sink.OnBackendFallback(diagnostics.BackendFallback{
				From:   from,
				To:     "software",
				Reason: reason,
			})
		}
	}
	rt.rewireTextureBackendAfterFallback()
}

// rewireTextureBackendAfterFallback points the asset manager's texture releaser
// and uploader at the (software) backend that replaced a fatal GPU backend. The
// previous upload adapter's forwarding goroutine is closed.
func (rt *Runtime) rewireTextureBackendAfterFallback() {
	if rt.assetManager == nil {
		return
	}
	m, ok := rt.assetManager.(*assets.ManagerImpl)
	if !ok {
		return
	}
	if tb, ok := rt.renderPipeline.Backend().(render.TextureBackend); ok {
		m.SetTextureReleaser(&assetTextureReleaser{backend: tb})
	}
	if q := rt.renderPipeline.UploadQueue(); q != nil {
		if rt.assetUploader != nil {
			rt.assetUploader.Close()
		}
		rt.assetUploader = newAssetUploader(q)
		m.SetUploader(rt.assetUploader)
	}
}
