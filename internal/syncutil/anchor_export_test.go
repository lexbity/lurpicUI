package syncutil

import (
	"sync"
	"testing"
)

func TestAnchorExportAssertion_isolated_and_nested(t *testing.T) {
	// 1. By default, it should not panic
	AssertNotAnchorExporting("test1")

	// 2. When exporting begins, it should panic on the same goroutine
	func() {
		cleanup := BeginAnchorExport()
		defer cleanup()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic during anchor export pass")
			}
		}()
		AssertNotAnchorExporting("test2")
	}()

	// 3. After cleanup, it should no longer panic
	AssertNotAnchorExporting("test3")

	// 4. Test nested export passes
	func() {
		cleanup1 := BeginAnchorExport()
		defer cleanup1()

		cleanup2 := BeginAnchorExport()
		defer cleanup2()

		defer func() {
			if r := recover(); r == nil {
				t.Error("expected panic during nested anchor export pass")
			}
		}()
		AssertNotAnchorExporting("test_nested")
	}()

	// 5. Test concurrent isolation: goroutine A exporting should NOT cause goroutine B to panic
	var wg sync.WaitGroup
	wg.Add(1)

	// Goroutine A begins anchor export and blocks
	exportStarted := make(chan struct{})
	exportResume := make(chan struct{})
	go func() {
		defer wg.Done()
		cleanup := BeginAnchorExport()
		defer cleanup()

		close(exportStarted)
		<-exportResume
	}()

	<-exportStarted

	// Goroutine B (main test goroutine) asserts and should NOT panic
	AssertNotAnchorExporting("test_concurrent")

	// Clean up goroutine A
	close(exportResume)
	wg.Wait()

	// Verify the map has been cleaned up and is empty
	exportingGoroutinesMu.RLock()
	mapSize := len(exportingGoroutines)
	exportingGoroutinesMu.RUnlock()
	if mapSize != 0 {
		t.Errorf("expected exportingGoroutines map to be empty, got size %d", mapSize)
	}
}
