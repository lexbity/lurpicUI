package runtime

import (
	goruntime "runtime"

	"codeburg.org/lexbit/lurpicui/diagnostics"
)

func (rt *Runtime) LastFrameStats() diagnostics.FrameStats {

	return rt.lastStats
}

func (rt *Runtime) handlePlatformLowMemory() {
	rt.log.Warn("runtime: android low memory event received")
	rt.clearRecoverableCaches()
	// Android's LowMemoryNotification signals that the OS may kill background
	// processes. We force a GC here to reduce our RSS before the Android OOM
	// killer evaluates our process. This is per Android NDK guidelines.
	goruntime.GC()
}
