package studio

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/cmd/lurpiclint/capabilities"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestCapabilityIndex_catalogMatchesScan asserts FR-capindex: the demo's
// catalog is produced by the same public seam (`capabilities.Scan`) that
// `lurpiclint capabilities` uses, over the same module root — "same generator,
// same module root" by construction (F-capindex-internal). When the source tree
// is unavailable (no go.mod found), the exhibit surfaces an honest note and the
// test skips.
func TestCapabilityIndex_catalogMatchesScan(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	caps, err := loadCapabilities()
	if err != nil {
		t.Fatalf("loadCapabilities: %v", err)
	}
	want, err := capabilities.Scan(root)
	if err != nil {
		t.Fatalf("capabilities.Scan: %v", err)
	}
	if len(caps) != len(want) {
		t.Fatalf("catalog size = %d, want %d from capabilities.Scan", len(caps), len(want))
	}
	if len(caps) == 0 {
		t.Fatal("catalog is empty")
	}
}

// TestCapabilityIndex_groups asserts the catalog is grouped by kind in the
// fixed Marks/Layouts/Layers order with paths sorted within each group, and
// that the totals line counts match the groups.
func TestCapabilityIndex_groups(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	caps, err := loadCapabilities()
	if err != nil {
		t.Fatalf("loadCapabilities: %v", err)
	}

	groups := splitCapabilityGroups(caps)
	if len(groups) != 3 {
		t.Fatalf("groups = %d, want 3 (Marks/Layouts/Layers)", len(groups))
	}
	wantTitles := []string{"Marks", "Layouts", "Layers"}
	for i, g := range groups {
		if g.title != wantTitles[i] {
			t.Fatalf("group[%d] title = %q, want %q", i, g.title, wantTitles[i])
		}
		for j := 1; j < len(g.caps); j++ {
			if g.caps[j-1].Path > g.caps[j].Path {
				t.Fatalf("group %q paths not sorted: %q before %q", g.title, g.caps[j-1].Path, g.caps[j].Path)
			}
		}
	}

	sum := capTotals(caps)
	total := 0
	for _, kind := range []capabilities.CapabilityKind{capabilities.KindMark, capabilities.KindLayout, capabilities.KindLayer} {
		total += sum[kind]
	}
	if total != len(caps) {
		t.Fatalf("capTotals sum = %d, want %d", total, len(caps))
	}
	for _, g := range groups {
		if sum[g.capsKind()] != len(g.caps) {
			t.Fatalf("group %q count %d != kind total %d", g.title, len(g.caps), sum[g.capsKind()])
		}
	}
}

// capsKind maps a capabilityGroup back to the kind its rows carry (only
// meaningful for non-empty groups; the demo scan always yields all three).
func (g *capabilityGroup) capsKind() capabilities.CapabilityKind {
	if len(g.caps) > 0 {
		return g.caps[0].Kind
	}
	return capabilities.KindMark
}

// TestCapabilityIndex_tableRows pins the catalog table's row structure
// (RX-1 FR-14 / P6): one table of Kind/Path/Intent columns whose rows are the
// 300+ capabilities plus 3 section headers plus 2 text lines (provenance on
// top, totals on the bottom).
func TestCapabilityIndex_tableRows(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	caps, err := loadCapabilities()
	if err != nil {
		t.Fatalf("loadCapabilities: %v", err)
	}
	if len(caps) < 300 {
		t.Fatalf("catalog size = %d, want >= 300 (the FR-14 row-count pin)", len(caps))
	}

	data := capabilityTableData(caps)
	wantRows := len(caps) + 3 + 2 // capabilities + section headers + text lines
	if len(data.Rows) != wantRows {
		t.Fatalf("table rows = %d, want %d (%d caps + 3 section headers + 2 text lines)", len(data.Rows), wantRows, len(caps))
	}
	if len(data.Columns) != 3 {
		t.Fatalf("table columns = %d, want 3 (Kind/Path/Intent)", len(data.Columns))
	}
	if data.Columns[0].Key != "kind" || data.Columns[1].Key != "path" || data.Columns[2].Key != "intent" {
		t.Fatalf("table columns = %+v, want Kind/Path/Intent", data.Columns)
	}

	// Provenance note is the first row, totals note the last.
	if data.Rows[0].Key != "note:provenance" {
		t.Fatalf("first row = %q, want the provenance note", data.Rows[0].Key)
	}
	if last := data.Rows[len(data.Rows)-1]; last.Key != "note:totals" {
		t.Fatalf("last row = %q, want the totals note", last.Key)
	}

	// Exactly three section headers, one per group, in group order.
	sectionKeys := make([]string, 0, 3)
	for _, r := range data.Rows {
		if len(r.Key) > 0 && r.Key[:8] == "section:" {
			sectionKeys = append(sectionKeys, r.Key)
		}
	}
	if len(sectionKeys) != 3 {
		t.Fatalf("section-header rows = %d, want 3: %v", len(sectionKeys), sectionKeys)
	}
	for i, g := range splitCapabilityGroups(caps) {
		if sectionKeys[i] != "section:"+g.title {
			t.Fatalf("section header[%d] = %q, want %q", i, sectionKeys[i], "section:"+g.title)
		}
	}

	// The totals note carries the live per-kind sums.
	sum := capTotals(caps)
	wantTotals := fmt.Sprintf("%d marks · %d layouts · %d layers", sum[capabilities.KindMark], sum[capabilities.KindLayout], sum[capabilities.KindLayer])
	if got := data.Rows[len(data.Rows)-1].Cells[1]; got != wantTotals {
		t.Fatalf("totals note = %q, want %q", got, wantTotals)
	}
}

// TestCapabilityIndex_rendersCatalog builds the exhibit and asserts the
// catalog renders through the table mark: the table is arranged to the full
// exhibit area, the provenance row is visible at the top, and the table's
// FR-7 row virtualization builds only a window of the rows (not all of them).
func TestCapabilityIndex_rendersCatalog(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()

	tbl := f.Table()
	if b := tbl.Base().LayoutRole().ArrangedBounds; b.IsEmpty() {
		t.Fatal("capability index table not arranged")
	}

	// Virtualization: the built row window is far smaller than the full row
	// set (FR-7) — the table builds only the visible window + overscan.
	first, last := tbl.VisibleRange()
	if first < 0 || last < first {
		t.Fatalf("VisibleRange = (%d, %d), want a built window", first, last)
	}
	data := tbl.Data.Get()
	if built := last - first + 1; built >= len(data.Rows) {
		t.Fatalf("built window %d rows >= total %d rows — row virtualization not in effect", built, len(data.Rows))
	}
	if first != 0 {
		t.Fatalf("VisibleRange first = %d, want 0 at scroll top (provenance visible)", first)
	}
}

// TestCapabilityIndex_totalsReachableByScroll asserts AC-7: scrolling the
// table to the bottom brings the totals row into the built window (reachable
// by scroll), and the provenance note is reachable back at the top.
func TestCapabilityIndex_totalsReachableByScroll(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()

	tbl := f.Table()
	data := tbl.Data.Get()
	lastRow := len(data.Rows) - 1

	// Scroll well past the bottom; the offset clamps and the window's last row
	// must reach the totals row.
	scrollTableToBottom(t, h, tbl)
	_, last := tbl.VisibleRange()
	if last != lastRow {
		t.Fatalf("bottom VisibleRange last = %d, want %d (totals row)", last, lastRow)
	}

	// Scroll back to the top: the provenance row is reachable again.
	scrollTableToTop(t, h, tbl)
	first, _ := tbl.VisibleRange()
	if first != 0 {
		t.Fatalf("top VisibleRange first = %d, want 0 (provenance row)", first)
	}
}

// scrollTableToBottom drives wheel scrolls over the table until its scroll
// offset clamps at the bottom (the window's last row is the final row).
func scrollTableToBottom(t *testing.T, h *testkit.Harness, tbl interface {
	VisibleRange() (int, int)
}) {
	t.Helper()
	_, last := tbl.VisibleRange()
	// Guard against a no-op (already at bottom) and an unbounded loop.
	previous := -1
	for i := 0; i < 200 && previous != last; i++ {
		previous = last
		driveScrollCenter(h)
		_, last = tbl.VisibleRange()
	}
}

// scrollTableToTop drives wheel scrolls upward until the window reaches row 0.
func scrollTableToTop(t *testing.T, h *testkit.Harness, tbl interface {
	VisibleRange() (int, int)
}) {
	t.Helper()
	first, _ := tbl.VisibleRange()
	previous := -1
	for i := 0; i < 200 && previous != first; i++ {
		previous = first
		driveScrollCenterUp(h)
		first, _ = tbl.VisibleRange()
	}
}

// driveScrollCenter sends one large downward wheel step at the harness center
// (the point over the table). A negative platform deltaY scrolls content down
// (the mark subtracts it from the scroll offset).
func driveScrollCenter(h *testkit.Harness) {
	h.InjectEvent(testkit.Scroll(480, 300, 0, -600))
	h.RunFrame()
}

// driveScrollCenterUp sends one large upward wheel step at the harness center.
func driveScrollCenterUp(h *testkit.Harness) {
	h.InjectEvent(testkit.Scroll(480, 300, 0, 600))
	h.RunFrame()
}
