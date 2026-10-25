package equivalence

import (
	"fmt"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/drive/proto"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestNegativeControl_SkewedTwinDiverges proves the harness has teeth
// (G6.5): a deliberately skewed GUI twin (skips the interaction entirely,
// mimicking a lying verb whose GUI twin was never actually run) MUST fail
// AssertEquivalent. The skew is injected into the honest input.click/button
// pair's GUI path.
func TestNegativeControl_SkewedTwinDiverges(t *testing.T) {
	p := Pair{
		Name:      "input.click/button",
		Fixture:   newButtonFixture,
		GUIHelper: "skewed: no interaction at all",
		Verb: func(h *testkit.DriveHarness) *proto.Error {
			_, perr := h.Call("input.click", map[string]any{"x": 240, "y": 120})
			return perr
		},
		GUI: func(h *testkit.Harness) {
			// THE SKEW: no click. If the harness can catch a lying twin, the
			// run diverges (activation counter stays 0 while the drive path
			// clicked).
		},
	}

	// recordingTB captures the assertion failure instead of failing the
	// outer test (the skew is expected to diverge).
	rec := &recordingTB{T: t}
	AssertEquivalent(rec, p)
	if len(rec.failures) == 0 {
		t.Fatal("the skewed twin must diverge — AssertEquivalent caught nothing (the harness has no teeth)")
	}
	t.Logf("the skew was caught: %v", rec.failures)
}

func TestNegativeWrongCadence(t *testing.T) {
	p := corpusPair(t, "input.click/button")
	original := p.GUI
	p.GUI = func(h *testkit.Harness) {
		original(h)
		h.RunFrame() // injected extra frame is the deliberate cadence fault
	}
	assertNegativeCaught(t, p, "trajectory frame counts differ")
}

func TestNegativeZeroScroll(t *testing.T) {
	p := corpusPair(t, "input.scroll/table")
	p.GUI = func(h *testkit.Harness) { testkit.DriveScroll(h, 240, 120, 0, 0) }
	assertNegativeCaught(t, p, "diverged")
}

func TestNegativeSkippedHover(t *testing.T) {
	p := corpusPair(t, "input.move/button")
	p.GUI = func(h *testkit.Harness) {
		testkit.Warmup(h)
		h.RunFrame() // preserve cadence while deliberately omitting hover
	}
	assertNegativeCaught(t, p, "diverged")
}

func TestNegativeMissingCancel(t *testing.T) {
	p := corpusPair(t, "input.release/slider")
	p.GUI = func(h *testkit.Harness) {
		testkit.Warmup(h)
		h.InjectEvent(testkit.PointerMove(240, 120))
		h.RunFrame()
		h.InjectEvent(testkit.PointerPress(240, 120, 1))
		h.RunFrame()
		// Deliberately omit release/cancel, leaving the gesture owned.
		h.RunFrame() // preserve the release path's frame count
	}
	assertNegativeCaught(t, p, "diverged")
}

func corpusPair(t *testing.T, name string) Pair {
	t.Helper()
	for _, pair := range corpusTest() {
		if pair.Name == name {
			return pair
		}
	}
	t.Fatalf("corpus pair %q not found", name)
	return Pair{}
}

func assertNegativeCaught(t *testing.T, pair Pair, expected string) {
	t.Helper()
	rec := &recordingTB{T: t}
	AssertEquivalent(rec, pair)
	if len(rec.failures) == 0 {
		t.Fatalf("negative control %s was not detected", pair.Name)
	}
	if !strings.Contains(strings.Join(rec.failures, "\n"), expected) {
		t.Fatalf("negative control %s failed for the wrong reason: %v", pair.Name, rec.failures)
	}
}

// recordingTB is a TB that records Fatalf calls instead of failing. It
// embeds *testing.T so the fixture helpers' testing.TB needs are satisfied;
// only Fatalf is overridden.
type recordingTB struct {
	*testing.T

	failures []string
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.failures = append(r.failures, fmt.Sprintf(format, args...))
}
