package animation

import (
	"sync"
	"time"
)

var (
	timelineMu sync.RWMutex
	timelines  []*Timeline
)

// Dispose unregisters the timeline from its tick source.
func (tl *Timeline) Dispose() {
	if tl == nil {
		return
	}
	tl.dispose()
}

func (tl *Timeline) bind() {
	if tl == nil {
		return
	}
	if tl.runtime != nil {
		tl.unregister = tl.runtime.RegisterPhase1TickHook(tl.tick)
		return
	}
	timelineMu.Lock()
	timelines = append(timelines, tl)
	timelineMu.Unlock()
	tl.unregister = func() {
		timelineMu.Lock()
		for i, candidate := range timelines {
			if candidate == tl {
				timelines[i] = nil
			}
		}
		timelineMu.Unlock()
	}
}

func (tl *Timeline) dispose() {
	if tl == nil {
		return
	}
	if tl.disposed {
		return
	}
	tl.disposed = true
	if tl.unregister != nil {
		tl.unregister()
	}
	tl.unregister = nil
	tl.runtime = nil
}

// dispatchTimelines advances all timeline instances registered without a runtime.
// It is used by tests and legacy callers.
func dispatchTimelines(dt time.Duration) {
	timelineMu.RLock()
	snapshot := append([]*Timeline(nil), timelines...)
	timelineMu.RUnlock()
	for _, tl := range snapshot {
		if tl != nil {
			tl.tick(dt)
		}
	}
}

// ResetTimelineRegistryForTest clears the nil-runtime timeline registry.
func ResetTimelineRegistryForTest() {
	timelineMu.Lock()
	timelines = nil
	timelineMu.Unlock()
}
