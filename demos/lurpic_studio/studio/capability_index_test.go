package studio

import (
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

// TestCapabilityIndex_rendersCatalog builds the exhibit and asserts the catalog
// renders into the scroll region with the provenance note, the three grouped
// cards, and the totals line — all arranged and hit-testable via the scroll
// region (the read-only scroll_region genuinely hosts this content).
func TestCapabilityIndex_rendersCatalog(t *testing.T) {
	root := findStudioModuleRoot()
	if root == "" {
		t.Skip("source tree unavailable (no go.mod found)")
	}
	f := NewCapabilityIndexFacet()
	h := testkit.NewStandardHarness(t, 960, 600, f)
	h.RunFrame()

	children := f.Scroll().Children()
	// provenance note + Marks + Layouts + Layers + totals line.
	if len(children) < 5 {
		t.Fatalf("catalog scroll children = %d, want >= 5", len(children))
	}
	if b := f.Scroll().Base().LayoutRole().ArrangedBounds; b.IsEmpty() {
		t.Fatal("capability index scroll not arranged")
	}
	// The catalog content is taller than the viewport, so the last content row
	// extends below it — the scroll region genuinely has something to scroll.
	if last := children[len(children)-1].Layout.ArrangedBounds; last.IsEmpty() || last.Max.Y <= 600 {
		t.Fatalf("catalog content does not overflow the viewport (last=%v)", last)
	}
}
