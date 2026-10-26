package projection

import (
	"bytes"
	"fmt"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestGateTraceSerialProjection proves the gate trace remains bounded and
// ordered during the serial projection walk.
//
// The trace is force-enabled here without touching process env — consuming the
// sync.Once and setting the flag directly, then restoring both — so the test
// is deterministic and other tests in the package are unaffected. Under -race
// the assertion stays independent of environment state.
func TestGateTraceSerialProjection(t *testing.T) {
	gatesTraceOnce.Do(func() {})
	prevEnabled, prevWriter := gatesTraceEnabled, gatesTraceWriter
	gatesTraceEnabled = true
	var traceBuf bytes.Buffer
	gatesTraceWriter = &traceBuf
	defer func() {
		gatesTraceEnabled, gatesTraceWriter = prevEnabled, prevWriter
	}()

	// root → two hosts → 200 gated leaves each.
	const leavesPerHost = 200
	root := newProjectionTestFacet("root", gfx.RectFromXYWH(0, 0, 10, 10))
	for hostIndex := 0; hostIndex < 2; hostIndex++ {
		host := newProjectionTestFacet(fmt.Sprintf("host%d", hostIndex), gfx.RectFromXYWH(0, 0, 10, 10))
		root.AddChild(&host.Facet)
		for i := 0; i < leavesPerHost; i++ {
			leaf := newProjectionTestFacet(fmt.Sprintf("leaf%d_%d", hostIndex, i), gfx.Rect{})
			host.AddChild(&leaf.Facet)
		}
	}
	attachTree(root)

	sys := NewSystem()
	sys.Run(root, FrameInfo{Number: 1, WallTime: time.Unix(0, 0)})

	if got := sys.EmptyBoundsSkips; got != 2*leavesPerHost {
		t.Fatalf("EmptyBoundsSkips = %d, want %d", got, 2*leavesPerHost)
	}
	// The per-frame cap must hold: 400 gated facets, at most
	// gateTraceLineCap trace lines.
	if lines := strings.Count(traceBuf.String(), "LURPIC_GATES_TRACE"); lines > gateTraceLineCap {
		t.Fatalf("trace lines = %d, want <= %d", lines, gateTraceLineCap)
	}
	if lines := strings.Count(traceBuf.String(), "LURPIC_GATES_TRACE"); lines == 0 {
		t.Fatal("no trace lines emitted with tracing enabled — trace never exercised")
	}
}
