package runtime

import (
	"os"
	goruntime "runtime"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
)

func (rt *Runtime) LastFrameStats() diagnostics.FrameStats {

	return rt.lastStats
}

// LastDirtySnapshot returns the per-facet dirty set captured at the most
// recent frame's snapshot point (the copy taken before layout/projection), or
// nil if no frame has snapshotted yet. It is a test/observability seam for the
// reactivity contract (RX-1 FR-3): a test can assert that a facet entered a
// frame's dirty set — e.g. through a FromDerived binding firing in the signal
// phase — without parsing logs. The returned map is a copy; callers may not
// mutate the runtime's frame bookkeeping through it.
func (rt *Runtime) LastDirtySnapshot() map[facet.FacetID]facet.DirtyFlags {
	if rt == nil {
		return nil
	}
	out := make(map[facet.FacetID]facet.DirtyFlags, len(rt.lastDirtySnapshot))
	for id, flags := range rt.lastDirtySnapshot {
		if flags != 0 {
			out[id] = flags
		}
	}
	return out
}

// checkDeviceGeneration queries the render backend's device generation and
// notifies the asset manager when the generation changes (device lost event).
func (rt *Runtime) checkDeviceGeneration() {
	if rt.renderPipeline == nil || rt.assetManager == nil {
		return
	}
	// The device generation is an optional interface on the backend.
	if dg, ok := rt.renderPipeline.Backend().(render.DeviceGenerationProvider); ok {
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

// LastOutputCommands returns the command list projected for a facet in the most
// recent frame (retained projection output). Intended for tests and diagnostics
// that inspect a frame's output without re-invoking projection callbacks
// outside the phase (RX-1 P5).
func (rt *Runtime) LastOutputCommands(id facet.FacetID) []gfx.Command {
	if rt == nil || rt.projectionSystem == nil {
		return nil
	}
	return rt.projectionSystem.LastOutputCommands(id)
}
