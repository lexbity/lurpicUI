package studio

import (
	"fmt"
	"os"
	"strconv"
	"testing"
	"time"
)

// TestMain enforces the RX-1 NFR-6 suite budget (≤ 120s) when the CI sets
// LURPIC_SUITE_BUDGET=<seconds>. Local and -race runs (naturally slower) leave
// it unset and skip the check; the CMake test-unit target sets it for the CI
// gate.
func TestMain(m *testing.M) {
	start := time.Now()
	code := m.Run()
	elapsed := time.Since(start)
	if budget := os.Getenv("LURPIC_SUITE_BUDGET"); budget != "" {
		if secs, err := strconv.Atoi(budget); err == nil && secs > 0 {
			if elapsed > time.Duration(secs)*time.Second {
				fmt.Fprintf(os.Stderr, "lurpic_studio suite exceeded NFR-6 budget: %v > %ds\n", elapsed.Round(time.Millisecond), secs)
				os.Exit(1)
			}
		}
	}
	os.Exit(code)
}

// coverageBudgetSeconds is the RX-1 NFR-6 sub-budget for the alive-coverage
// suite (≤ 40s). The coverage tests wrap their body with assertCoverageBudget.
const coverageBudgetSeconds = 40

// assertCoverageBudget fails the test if the wrapped coverage work exceeds the
// NFR-6 alive-coverage budget.
func assertCoverageBudget(t *testing.T, start time.Time) {
	t.Helper()
	if elapsed := time.Since(start); elapsed > time.Duration(coverageBudgetSeconds)*time.Second {
		t.Fatalf("alive-coverage work exceeded NFR-6 budget: %v > %ds", elapsed.Round(time.Millisecond), coverageBudgetSeconds)
	}
}
