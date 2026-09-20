package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestCapabilityIndex_goldenTop pins the catalog at scroll top: the provenance
// note row renders at the top of the table (fully visible), and the table's
// chrome and first section are present (RX-1 FR-14 / AC-7).
func TestCapabilityIndex_goldenTop(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()
	testkit.AssertGolden(t, h.Surface(), "capindex_top")
}

// TestCapabilityIndex_goldenMiddle drives the table's scroll to a middle band
// (VisibleRange-driven) and pins the render (AC-7: middle rows sampled).
func TestCapabilityIndex_goldenMiddle(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()

	tbl := f.Table()
	// Scroll until the built window's first row is past the catalog's first
	// third (deterministic wheel steps; same state every run).
	for i := 0; i < 200; i++ {
		first, _ := tbl.VisibleRange()
		data := tbl.Data.Get()
		if first > len(data.Rows)/3 {
			break
		}
		driveScrollCenter(h)
	}
	testkit.AssertGolden(t, h.Surface(), "capindex_middle")
}

// TestCapabilityIndex_goldenBottom scrolls to the bottom of the catalog and
// pins the totals note row (AC-7: totals reachable by scroll).
func TestCapabilityIndex_goldenBottom(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()

	tbl := f.Table()
	scrollTableToBottom(t, h, tbl)
	testkit.AssertGolden(t, h.Surface(), "capindex_bottom")
}
