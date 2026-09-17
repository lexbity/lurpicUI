package projection

import (
	"fmt"
	"io"
	"os"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
)

// LURPIC_TRACE=gates enables per-frame gate-decision tracing (RX-1 NFR-8):
// every facet gated by the empty-bounds gate logs facet id + reason, capped at
// 64 lines per frame. This is a debug facility, separate from the runtime's
// LURPIC_DEBUG_RUNTIME_LOOP trace (the projection package must not import
// runtime, so the gate trace is self-contained here).
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
// per-frame line cap. walkNode — and therefore traceGate — runs on forked
// goroutines (walkChildSubtrees), so the line counter and writer are guarded
// by System.statsMu; holding it across the write also keeps concurrent trace
// lines from interleaving mid-line.
func (s *System) traceGate(facetID facet.FacetID, reason string) {
	if s == nil || !gatesTraceActive() {
		return
	}
	s.statsMu.Lock()
	defer s.statsMu.Unlock()
	if s.gatesTraceLines >= gateTraceLineCap {
		return
	}
	s.gatesTraceLines++
	fmt.Fprintf(gatesTraceWriter, "LURPIC_GATES_TRACE: frame=%d facet=%d reason=%s\n", s.frameNumber, facetID, reason)
}

const gateTraceLineCap = 64
