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
