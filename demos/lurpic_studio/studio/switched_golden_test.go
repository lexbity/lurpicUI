package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestSwitchedGolden_stageReproducesFreshAfterExhibitCycle is the RX-1 FR-21
// first increment and proves AC-1: on ONE harness, the fresh Realtime stage
// region is captured, the shell cycles through other exhibits (each pinned by a
// per-exhibit switched golden), and returning to Realtime reproduces the fresh
// capture byte-identically in the stage region.
//
// On pre-P2 code the projection cache could serve a gated exhibit's last output
// (A-1/A-2/A-3): stale pixels from a previous exhibit lingered in the frame.
// The empty-bounds gate (FR-1) prunes gated subtrees at the projection walk, so
// a revisit renders exactly the fresh pixels — a no-stale-pixels assertion.
func TestSwitchedGolden_stageReproducesFreshAfterExhibitCycle(t *testing.T) {
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
		FontRegistry: testkit.TestFontRegistry(t),
	}
	root := NewRoot(ctx, nil, seedRows(t), nil)
	h := testkit.NewStandardHarness(t, 1280, 800, root)
	h.RunFrames(3) // settle to a deterministic, feed-paused steady state

	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	if stage.IsEmpty() {
		t.Fatal("stage not arranged")
	}

	// FRESH capture: the stage region before any switch.
	testkit.AssertRegionGolden(t, h.Surface(), "switched_realtime_fresh", stage)

	// Cycle through other exhibits, pinning each switched stage region.
	cycle := []ExhibitID{ExhibitCapabilities, ExhibitPolicies, ExhibitPlayground}
	for _, id := range cycle {
		root.Shell().ActiveExhibit.Set(id)
		h.RunFrames(2)
		testkit.AssertRegionGolden(t, h.Surface(), "switched_"+string(id), stage)
	}

	// Return to Realtime. The stage region must reproduce the fresh golden
	// byte-identically — no stale pixels from the gated exhibits.
	root.Shell().ActiveExhibit.Set(ExhibitRealtime)
	h.RunFrames(2)
	testkit.AssertRegionGolden(t, h.Surface(), "switched_realtime_fresh", stage)
}
