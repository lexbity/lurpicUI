package projection

import (
	"fmt"
	"io"
	"os"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
)

// LURPIC_TRACE=gates enables per-frame gate-decision tracing (RX-1 NFR-8):
// every facet gated by the empty-bounds gate or the layer mount-state gate logs
// facet id + reason, capped at 64 lines per frame. This is a debug facility,
// separate from the runtime's LURPIC_DEBUG_RUNTIME_LOOP trace (the projection
// package must not import runtime, so the gate trace is self-contained here).
var gatesTraceOnce sync.Once
var gatesTraceEnabled bool

// gatesTraceWriter is the sink for gate-trace lines. A var so tests can capture
// output; written only while holding System.statsMu (see traceGate).
var gatesTraceWriter io.Writer = os.Stderr

func gatesTraceActive() bool {
	gatesTraceOnce.Do(func() {
		gatesTraceEnabled = os.Getenv("LURPIC_TRACE") == "gates"
	})
	return gatesTraceEnabled
}

// traceGate logs one gate decision for the current frame, honoring the
// per-frame line cap. Author callbacks and gate tracing run on the runtime
// thread, so the trace writer needs no cross-worker synchronization.
func (s *System) traceGate(facetID facet.FacetID, reason string) {
	if s == nil || !gatesTraceActive() {
		return
	}
	if s.gatesTraceLines >= gateTraceLineCap {
		return
	}
	s.gatesTraceLines++
	fmt.Fprintf(gatesTraceWriter, "LURPIC_GATES_TRACE: frame=%d facet=%d reason=%s\n", s.frameNumber, facetID, reason)
}

const gateTraceLineCap = 64
