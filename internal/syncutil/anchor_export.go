package syncutil

import "sync/atomic"

var anchorExportDepth atomic.Int64

// BeginAnchorExport marks the current goroutine as executing an anchor export pass.
func BeginAnchorExport() func() {
	anchorExportDepth.Add(1)
	return func() {
		anchorExportDepth.Add(-1)
	}
}

// AssertNotAnchorExporting panics when called from inside an anchor export pass.
func AssertNotAnchorExporting(op string) {
	if anchorExportDepth.Load() <= 0 {
		return
	}
	if op == "" {
		op = "operation"
	}
	panic("syncutil: " + op + " called during anchor export")
}
