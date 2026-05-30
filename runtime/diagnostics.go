package runtime

import (
	"os"
	goruntime "runtime"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
)

func (rt *Runtime) LastFrameStats() diagnostics.FrameStats {

	return rt.lastStats
}

// checkDeviceGeneration queries the render backend's device generation and
// notifies the asset manager when the generation changes (device lost event).
func (rt *Runtime) checkDeviceGeneration() {
	if rt.renderPipeline == nil || rt.assetManager == nil {
		return
	}
	// The device generation is an optional interface on the backend.
	if dg, ok := rt.renderPipeline.backend.(render.DeviceGenerationProvider); ok {
		gen := dg.DeviceGeneration()
		if ev, ok := rt.assetManager.(interface{ CheckDeviceGeneration(uint64) bool }); ok {
			if ev.CheckDeviceGeneration(gen) {
				rt.log.Info("runtime: device generation changed; GPU LODs invalidated",
					"generation", gen)
			}
		}
	}
}

func (rt *Runtime) handlePlatformLowMemory() {
	rt.log.Warn("runtime: android low memory event received")
	rt.clearRecoverableCaches()
	// Android's LowMemoryNotification signals that the OS may kill background
	// processes. We force a GC here to reduce our RSS before the Android OOM
	// killer evaluates our process. This is per Android NDK guidelines.
	goruntime.GC()
}

// handleTrimMemory processes Android onTrimMemory levels.
//
// Level-to-eviction mapping:
//
//	UI_HIDDEN / BACKGROUND (20, 40)     → evict GPU LODs to low watermark
//	RUNNING_CRITICAL / COMPLETE (15, 80) → evict to minimum, drop all non-pinned LODs
//	Other                                → log and continue
func (rt *Runtime) handleTrimMemory(e platform.TrimMemoryEvent) {
	rt.log.Info("runtime: trim memory", "level", e.Level)
	if mgr := rt.assetManager; mgr != nil {
		if ev, ok := mgr.(interface{ TrimMemory(int) int }); ok {
			evicted := ev.TrimMemory(e.Level)
			rt.log.Debug("runtime: trim memory eviction", "evicted", evicted)
		}
	}
	rt.clearRecoverableCaches()
	goruntime.GC()
	_ = os.Getpid()
}

