package runtime

import (
	"os"
	"sort"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TestLayoutPassBudget is the RX-1 NFR-3 regression gate: a content change
// routed to a deep leaf re-lays a studio-sized tree (10 cards x 30 leaves at
// 1280x800) with p50 ≤ 2ms and p95 ≤ 5ms, measured over the median of 40
// passes. The budgets are absolute (not a baseline ratio) so the gate is
// machine-stable — the observed median is ~1ms with 2-5x headroom.
//
// Timing gates must run in isolation, not inside the parallel `go test ./...`
// suite (parallel load moves millisecond medians). The CMake test-budget
// target runs this with LURPIC_BUDGET_GATES=1.
func TestLayoutPassBudget(t *testing.T) {
	if os.Getenv("LURPIC_BUDGET_GATES") != "1" {
		t.Skip("RX-1 NFR-3 budget gate; set LURPIC_BUDGET_GATES=1 (the test-budget target)")
	}
	root := buildBenchmarkStudioTree(10, 30)
	rt := mustRuntimeTree(t, root)
	rt.markTreeDirty(root, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 1280, H: 800})

	var leaves []*layoutCountLeaf
	collectLeaves(root, &leaves)
	if len(leaves) == 0 {
		t.Fatal("no leaves in benchmark tree")
	}

	times := make([]time.Duration, 0, 40)
	for i := 0; i < 40; i++ {
		leaf := leaves[i%len(leaves)]
		rt.dirtyFacets = map[facet.FacetID]facet.DirtyFlags{leaf.Base().ID(): facet.DirtyLayout}
		start := time.Now()
		rt.runLayoutPass(gfx.Size{W: 1280, H: 800})
		times = append(times, time.Since(start))
	}
	sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
	median := times[len(times)/2]
	p95 := times[int(float64(len(times))*0.95)]

	const (
		medianCap = 2 * time.Millisecond
		p95Cap    = 5 * time.Millisecond
	)
	if median > medianCap {
		t.Fatalf("layout pass p50 = %v, exceeds NFR-3 budget %v", median, medianCap)
	}
	if p95 > p95Cap {
		t.Fatalf("layout pass p95 = %v, exceeds NFR-3 budget %v", p95, p95Cap)
	}
}
