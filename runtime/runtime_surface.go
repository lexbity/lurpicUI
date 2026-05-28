package runtime

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
)

func (rt *Runtime) handlePlatformPause() {

	rt.lifecycleMu.Lock()
	rt.paused = true
	if rt.lifecycleCond != nil {
		rt.lifecycleCond.Broadcast()
	}
	rt.lifecycleMu.Unlock()
	if rt.jobPool != nil {
		rt.jobPool.Pause()
		rt.jobPool.CancelAll()
	}
	rt.setIMEVisible(false)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) handlePlatformResume() {

	rt.lifecycleMu.Lock()
	rt.paused = false
	if rt.lifecycleCond != nil {
		rt.lifecycleCond.Broadcast()
	}
	rt.lifecycleMu.Unlock()
	if rt.jobPool != nil {
		rt.jobPool.Resume()
	}
	rt.markTreeDirty(rt.root, facet.DirtyAll)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) clearRecoverableCaches() {

	if rt.projectionSystem != nil {
		rt.projectionSystem.Reset()
	}
	if len(rt.anchorCaches) > 0 {
		for id, cache := range rt.anchorCaches {
			if cache != nil {
				_ = cache.Reset()
			}
			delete(rt.anchorCaches, id)
		}
	}
	if rt.renderPipeline != nil && rt.renderPipeline.backend != nil {
		if evictor, ok := rt.renderPipeline.backend.(render.CacheEvictor); ok {
			evictor.EvictCaches()
		}
	}
	rt.frameTimer.RequestFrame()
	rt.markTreeDirty(rt.root, facet.DirtyAll)
}

func (rt *Runtime) handleSurfaceLost() {

	rt.lifecycleMu.Lock()
	rt.surfaceReady = false
	rt.lifecycleMu.Unlock()
	rt.setIMEVisible(false)
	if rt.renderPipeline != nil && rt.renderPipeline.backend != nil {
		rt.renderPipeline.backend.Destroy()
	}
	rt.markTreeDirty(rt.root, facet.DirtyAll)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) handleSurfaceCreated(surface platform.Surface) {

	if rt.renderPipeline != nil && rt.renderPipeline.backend != nil && surface != nil {
		if err := rt.renderPipeline.backend.Initialize(surface); err != nil {
			if rt.log != nil {
				rt.log.Error("runtime: reinitialize render backend after surface creation failed", "error", err)
			}
		} else {
			rt.lifecycleMu.Lock()
			rt.surfaceReady = true
			rt.lifecycleMu.Unlock()
		}
	}
	rt.markTreeDirty(rt.root, facet.DirtyAll)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) validateWindowBindings() error {
	if rt.layerRegistry == nil {
		return nil
	}
	descs := rt.layerRegistry.OrderedLayers()
	for _, desc := range descs {
		if desc.WindowBinding.Kind != layout.WindowBindingNamed {
			continue
		}
		if rt.windowBindings == nil {
			return fmt.Errorf("runtime: layer %q requires named window binding %q", desc.Name, desc.WindowBinding.Name)
		}
		if win, ok := rt.windowBindings[desc.WindowBinding.Name]; !ok || win == nil {
			return fmt.Errorf("runtime: layer %q requires named window binding %q", desc.Name, desc.WindowBinding.Name)
		}
	}
	return nil
}

func copyWindowBindings(src map[string]platform.Window, primary platform.Window) map[string]platform.Window {
	out := make(map[string]platform.Window)
	if primary != nil {
		out[windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary})] = primary
	}
	for name, win := range src {
		if name == "" {
			continue
		}
		out[name] = win
	}
	return out
}
