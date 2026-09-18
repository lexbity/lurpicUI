package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
)

// TestRibbon_sectionTabClickActivates pins the composite internal-interaction
// contract (RX-1 F-e6-internal): a ribbon's section-tab buttons are internal
// sub-marks, but a real pointer click on a tab is driveable from outside the
// composite — the runtime routes the hit to the ribbon and the ribbon forwards
// the press to the tab button, which activates the section. Without the
// forwarding, the click would land on the ribbon and "Activated never fires".
func TestRibbon_sectionTabClickActivates(t *testing.T) {
	r := NewRibbon("Ribbon", []RibbonSection{
		{Key: "home", Label: "Home", Toolbars: []*Toolbar{NewToolbar(marks.Const("t"), nil, nil)}},
		{Key: "insert", Label: "Insert", Toolbars: []*Toolbar{NewToolbar(marks.Const("t"), nil, nil)}},
		{Key: "view", Label: "View", Toolbars: []*Toolbar{NewToolbar(marks.Const("t"), nil, nil)}},
	})
	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 400, 120), r)
	testkit.Warmup(h)

	if r.ActiveIndex != 0 {
		t.Fatalf("initial active section = %d, want 0", r.ActiveIndex)
	}
	// Click the third tab.
	tab := r.cachedTabBounds[2]
	if tab.IsEmpty() {
		t.Fatal("third tab has no arranged bounds")
	}
	cx := tab.Min.X + tab.Width()*0.5
	cy := tab.Min.Y + tab.Height()*0.5
	testkit.DriveClick(h, cx, cy)
	h.RunFrame()
	if r.ActiveIndex != 2 {
		t.Fatalf("after tab click active section = %d, want 2 (F-e6-internal)", r.ActiveIndex)
	}
	_ = gfx.Rect{}
}
