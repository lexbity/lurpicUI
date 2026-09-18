package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
)

// TestTreeNavigator_rowClickSelects pins the composite internal-interaction
// contract (RX-1 F-e6-internal): a tree-navigator's rows are internal
// sub-facets, but a real pointer click on a row is driveable from outside the
// composite — the runtime routes the hit to the tree and the tree's own
// pointer handling publishes the selection to the FR-8 selection store.
func TestTreeNavigator_rowClickSelects(t *testing.T) {
	nodes := []TreeNode{
		{Key: "root", Label: "Root", Expanded: true, Children: []TreeNode{
			{Key: "child", Label: "Child"},
			{Key: "sibling", Label: "Sibling"},
		}},
	}
	selection := store.NewValueStore("")
	tree := NewTreeNavigator("Tree", nodes, selection)
	tree.Disabled = marks.Const(false)

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 300, 200), tree)
	testkit.Warmup(h)

	if len(tree.cachedRowBounds) < 3 {
		t.Fatalf("expected at least 3 visible rows, got %d", len(tree.cachedRowBounds))
	}
	// Click the third visible row ("Sibling").
	row := tree.cachedRowBounds[2]
	if row.IsEmpty() {
		t.Fatal("second row has no arranged bounds")
	}
	cx := row.Min.X + 8 // the row label area, away from the disclosure
	cy := row.Min.Y + row.Height()*0.5
	testkit.DriveClick(h, cx, cy)
	h.RunFrame()

	if got := selection.Get(); got != "root/sibling" {
		t.Fatalf("row click did not publish the sibling selection: got %q, want %q", got, "root/sibling")
	}
	_ = gfx.Rect{}
}
