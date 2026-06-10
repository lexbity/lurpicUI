//go:build lurpic_debug && !android

package syncutil

import (
	"sync"
)

var (
	exportingGoroutinesMu sync.RWMutex
	exportingGoroutines   = make(map[int64]int)
)

// BeginAnchorExport marks the current goroutine as executing an anchor export pass.
func BeginAnchorExport() func() {
	id := currentGoroutineID()
	exportingGoroutinesMu.Lock()
	exportingGoroutines[id]++
	exportingGoroutinesMu.Unlock()

	return func() {
		exportingGoroutinesMu.Lock()
		exportingGoroutines[id]--
		if exportingGoroutines[id] <= 0 {
			delete(exportingGoroutines, id)
		}
		exportingGoroutinesMu.Unlock()
	}
}

// AssertNotAnchorExporting panics when called from inside an anchor export pass.
func AssertNotAnchorExporting(op string) {
	id := currentGoroutineID()
	exportingGoroutinesMu.RLock()
	depth := exportingGoroutines[id]
	exportingGoroutinesMu.RUnlock()

	if depth <= 0 {
		return
	}
	if op == "" {
		op = "operation"
	}
	panic("syncutil: " + op + " called during anchor export")
}
