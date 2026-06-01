package runtime

import (
	"fmt"
	"os"
	"sync"
)

var runtimeTraceOnce sync.Once
var runtimeTraceEnabled bool

func runtimeTraceActive() bool {
	runtimeTraceOnce.Do(func() {
		runtimeTraceEnabled = os.Getenv("LURPIC_DEBUG_RUNTIME_LOOP") == "1"
	})
	return runtimeTraceEnabled
}

func runtimeTracef(format string, args ...any) {
	if !runtimeTraceActive() {
		return
	}
	fmt.Fprintf(os.Stderr, "LURPIC_RUNTIME_TRACE: "+format+"\n", args...)
}

func isChannelClosed(ch <-chan struct{}) bool {
	if ch == nil {
		return false
	}
	select {
	case <-ch:
		return true
	default:
		return false
	}
}
