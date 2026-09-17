package projection

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestGateTrace_concurrentUnderForkedProjection proves the RX-1 NFR-8 gate
// trace is safe under the forked projection walk: walkNode runs on forked
// goroutines (walkChildSubtrees), and two forked subtrees gating facets at the
// same time hit traceGate concurrently. The per-frame line counter and the
// trace writer are therefore shared mutable state on the hot gating path.
//
// The trace is force-enabled here without touching process env — consuming the
// sync.Once and setting the flag directly, then restoring both — so the test
// is deterministic and other tests in the package are unaffected. Under -race
// an unsynchronized counter is reported as a write-write race between the two
// forked walkers.
func TestGateTrace_concurrentUnderForkedProjection(t *testing.T) {
	gatesTraceOnce.Do(func() {})
	prevEnabled, prevWriter := gatesTraceEnabled, gatesTraceWriter
	gatesTraceEnabled = true
	var traceBuf bytes.Buffer
	gatesTraceWriter = &traceBuf
	defer func() {
		gatesTraceEnabled, gatesTraceWriter = prevEnabled, prevWriter
	}()

	// root → two fork hosts → leavesPerFork gated leaves each. The tree exceeds
	// projectionForkThreshold, so Run forks at the root and both hosts are
	// walked in their own goroutine, each tracing its gated leaves concurrently.
	const leavesPerFork = 200
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	for fork := 0; fork < 2; fork++ {
		host := newProjectionTestFacet(fmt.Sprintf("host%d", fork), gfx.RectFromXYWH(0, 0, 10, 10))
		root.AddChild(&host.Facet)
		for i := 0; i < leavesPerFork; i++ {
			leaf := newProjectionTestFacet(fmt.Sprintf("leaf%d_%d", fork, i), gfx.Rect{})
			host.AddChild(&leaf.Facet)
		}
	}
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})

	if got := sys.EmptyBoundsSkips; got != 2*leavesPerFork {
		t.Fatalf("EmptyBoundsSkips = %d, want %d", got, 2*leavesPerFork)
	}
	// The per-frame cap must hold under concurrency: 400 gated facets, at most
	// gateTraceLineCap trace lines.
	if lines := strings.Count(traceBuf.String(), "LURPIC_GATES_TRACE"); lines > gateTraceLineCap {
		t.Fatalf("trace lines = %d, want <= %d", lines, gateTraceLineCap)
	}
	if lines := strings.Count(traceBuf.String(), "LURPIC_GATES_TRACE"); lines == 0 {
		t.Fatal("no trace lines emitted with tracing enabled — trace never exercised")
	}
}
