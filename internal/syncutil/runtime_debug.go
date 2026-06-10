//go:build lurpic_debug && !android

package syncutil

import (
	"runtime"
	"strconv"
	"strings"
	"sync/atomic"
)

var runtimeGoroutineID atomic.Int64

// RegisterRuntimeThread records the current goroutine as the runtime thread.
// It panics if called more than once.
func RegisterRuntimeThread() {
	id := currentGoroutineID()
	if !runtimeGoroutineID.CompareAndSwap(0, id) {
		panic("syncutil: RegisterRuntimeThread called more than once")
	}
}

// AssertRuntimeThread panics if the current goroutine is not the runtime thread.
func AssertRuntimeThread() {
	registered := runtimeGoroutineID.Load()
	if registered == 0 {
		return
	}
	if currentGoroutineID() != registered {
		panic("syncutil: store mutation called from non-runtime goroutine - use job.Schedule for background work, not direct store mutations")
	}
}

// OnRuntimeThread reports whether the current goroutine is the runtime thread.
func OnRuntimeThread() bool {
	registered := runtimeGoroutineID.Load()
	return registered == 0 || currentGoroutineID() == registered
}

// ResetRuntimeThreadForTest clears the registered runtime goroutine ID.
func ResetRuntimeThreadForTest() {
	runtimeGoroutineID.Store(0)
}

func currentGoroutineID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	fields := strings.Fields(string(buf[:n]))
	if len(fields) < 2 {
		return 0
	}
	id, err := strconv.ParseInt(fields[1], 10, 64)
	if err != nil {
		return 0
	}
	return id
}
