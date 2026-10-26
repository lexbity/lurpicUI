package runtime

import (
	"fmt"
	"sort"

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
	if rt.renderPipeline != nil {
		rt.lifecycleMu.Lock()
		generation := rt.surfaceLifecycleGeneration
		rt.lifecycleMu.Unlock()
		if err := rt.renderPipeline.QueueCacheEviction(generation); err != nil && rt.log != nil {
			rt.log.Error("runtime: queue render cache eviction", "error", err)
		}
	}
	rt.frameTimer.RequestFrame()
	rt.markTreeDirty(rt.root, facet.DirtyAll)
}

func (rt *Runtime) handleSurfaceLost() {

	// Surface loss is a render-lifecycle event, not a full app shutdown. The
	// runtime keeps its state, but it must stop submitting frames until a new
	// surface arrives and the backend has been torn down or recreated.
	rt.lifecycleMu.Lock()
	rt.surfaceReady = false
	rt.surfaceLifecycleGeneration++
	generation := rt.surfaceLifecycleGeneration
	rt.lifecycleMu.Unlock()
	rt.setIMEVisible(false)
	if rt.renderPipeline != nil {
		if err := rt.renderPipeline.QueueSurfaceLost(generation); err != nil && rt.log != nil {
			rt.log.Error("runtime: queue render surface loss", "generation", generation, "error", err)
		}
	}
	rt.markTreeDirty(rt.root, facet.DirtyAll)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) handleSurfaceCreated(surface platform.Surface) {
	if rt.renderPipeline != nil && surface != nil {
		rt.lifecycleMu.Lock()
		rt.surfaceReady = false
		rt.surfaceLifecycleGeneration++
		generation := rt.surfaceLifecycleGeneration
		rt.lifecycleMu.Unlock()
		if err := rt.renderPipeline.QueueSurfaceCreated(surface, generation, rt.renderWindowTargets(surface)); err != nil && rt.log != nil {
			rt.log.Error("runtime: queue render surface creation", "generation", generation, "error", err)
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

func (rt *Runtime) configureRenderWindows(backend render.Backend) error {
	targets := rt.renderWindowTargets(rt.currentSurface())
	if len(targets) <= 1 {
		return nil
	}
	multi, ok := backend.(render.WindowBackend)
	if !ok {
		return fmt.Errorf("runtime: %T lacks required window-set rendering contract", backend)
	}
	if len(targets) > render.MaxFrameWindows {
		return fmt.Errorf("runtime: window set exceeds %d windows", render.MaxFrameWindows)
	}
	for _, target := range targets {
		if target.Surface == nil {
			return fmt.Errorf("runtime: window %q has no surface", target.ID)
		}
	}
	return multi.ConfigureWindows(targets)
}

func (rt *Runtime) renderWindowTargets(primary render.Surface) []render.WindowTarget {
	keys := make([]string, 0, len(rt.windowBindings))
	for key := range rt.windowBindings {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	targets := make([]render.WindowTarget, 0, len(keys))
	for _, key := range keys {
		var surface render.Surface
		if window := rt.windowBindings[key]; window != nil {
			surface = window.Surface()
		}
		if key == render.PrimaryWindow && primary != nil {
			surface = primary
		}
		targets = append(targets, render.WindowTarget{ID: key, Surface: surface})
	}
	if primary != nil {
		found := false
		for _, target := range targets {
			if target.ID == render.PrimaryWindow {
				found = true
			}
		}
		if !found {
			targets = append(targets, render.WindowTarget{ID: render.PrimaryWindow, Surface: primary})
		}
	}
	return targets
}
