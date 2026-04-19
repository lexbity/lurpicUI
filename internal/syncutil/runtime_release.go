//go:build !lurpic_debug

package syncutil

// AssertRuntimeThread is a no-op in production builds.
func AssertRuntimeThread() {}

// OnRuntimeThread always reports true in production builds.
func OnRuntimeThread() bool { return true }

// ResetRuntimeThreadForTest is a no-op in production builds.
func ResetRuntimeThreadForTest() {}
