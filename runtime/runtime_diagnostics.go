package runtime

import (
	goruntime "runtime"

	"codeburg.org/lexbit/lurpicui/diagnostics"
)

func (rt *Runtime) LastFrameStats() diagnostics.FrameStats {

	return rt.lastStats
}

func (rt *Runtime) handlePlatformLowMemory() {

	if rt.log != nil {
		rt.log.Warn("runtime: android low memory event received")
	}
	rt.clearRecoverableCaches()
	goruntime.GC()
}
