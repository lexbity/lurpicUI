package projection

import (
	"os"
	"sort"
	"testing"
	"time"
)

// projectionBudgetBaselineNS is the recorded pre-change baseline for the wide
// projection workload (RX-1 NFR-2): the minimum of five batch medians of the
// dirty re-projection — the unloaded floor, the most stable estimate of the
// reference machine's true cost. The gate asserts the current floor stays
// within 10% of this baseline. Re-record it whenever the projection hot path
// changes materially.
const projectionBudgetBaselineNS = 110_000

// TestProjectionBudgetWithinBaseline is the RX-1 NFR-2 regression gate: the
// dirty re-projection floor of the wide workload stays within 10% of the
// recorded baseline. The floor is the minimum of five batch medians of 60 warm
// samples each, so GC/OS spikes (which only inflate the higher batch medians)
// are filtered; a real regression raises the floor and fails the gate.
//
// Timing gates must run in isolation, not inside the parallel `go test ./...`
// suite (parallel load moves µs-scale medians). The CMake test-budget target
// runs this with LURPIC_BUDGET_GATES=1.
func TestProjectionBudgetWithinBaseline(t *testing.T) {
	if os.Getenv("LURPIC_BUDGET_GATES") != "1" {
		t.Skip("RX-1 NFR-2 budget gate; set LURPIC_BUDGET_GATES=1 (the test-budget target)")
	}
	root := buildBenchmarkProjectionWide(4, 4)
	attachTree(root)

	medians := make([]time.Duration, 0, 5)
	for batch := 0; batch < 5; batch++ {
		sys := NewSystem()
		for i := 0; i < 20; i++ {
			sys.MarkDirty(root.Base().ID())
			sys.Run(root, FrameInfo{Number: uint64(i + 1)})
		}
		times := make([]time.Duration, 0, 60)
		for i := 0; i < 60; i++ {
			sys.MarkDirty(root.Base().ID())
			start := time.Now()
			sys.Run(root, FrameInfo{Number: uint64(i + 100)})
			times = append(times, time.Since(start))
		}
		sort.Slice(times, func(i, j int) bool { return times[i] < times[j] })
		medians = append(medians, times[len(times)/2])
	}
	sort.Slice(medians, func(i, j int) bool { return medians[i] < medians[j] })
	floor := medians[0]

	const capNS = projectionBudgetBaselineNS * 11 / 10
	if floor > time.Duration(capNS) {
		t.Fatalf("projection dirty floor = %v, exceeds NFR-2 budget %v (baseline %v + 10%%)",
			floor, time.Duration(capNS), time.Duration(projectionBudgetBaselineNS))
	}
	// Absolute safety net: a gross regression (or a broken cache that turns
	// every frame dirty) must not pass on a coincidentally-clean batch.
	if floor > 2*time.Millisecond {
		t.Fatalf("projection dirty floor = %v, grossly over budget", floor)
	}
}
