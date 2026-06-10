//go:build !lurpic_debug || android

package syncutil

// RegisterRuntimeThread is a no-op in release builds or on Android.
func RegisterRuntimeThread() {}

// AssertRuntimeThread is a no-op in release builds or on Android.
func AssertRuntimeThread() {}

// OnRuntimeThread always returns true in release builds or on Android.
func OnRuntimeThread() bool {
	return true
}

// ResetRuntimeThreadForTest is a no-op in release builds or on Android.
func ResetRuntimeThreadForTest() {}

// BeginAnchorExport returns an empty cleanup function in release builds or on Android.
func BeginAnchorExport() func() {
	return func() {}
}

// AssertNotAnchorExporting is a no-op in release builds or on Android.
func AssertNotAnchorExporting(op string) {}
