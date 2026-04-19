//go:build lurpic_debug

package syncutil

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
// It exists so packages outside internal/syncutil can isolate debug-mode tests.
func ResetRuntimeThreadForTest() {
	runtimeGoroutineID.Store(0)
}
