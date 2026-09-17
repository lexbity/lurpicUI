package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// TestReactivity_playgroundTabSwitch_golden proves RX-1 FR-3 at the shell
// level: switching a playground family tab re-lays the active body with zero
// author-written invalidation routing — the studio's tab subscription routes
// through layout.PropagateContentDirty (the framework entry point), and the
// stage region renders the newly active family.
func TestReactivity_playgroundTabSwitch_golden(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	root.Shell().ActiveExhibit.Set(ExhibitPlayground)
	h.RunFrames(2)

	e6 := root.Stage().RootFor(ExhibitPlayground).(*Playground)
	e6.ActiveTab().Set(3) // navigation family
	h.RunFrames(2)

	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	if stage.IsEmpty() {
		t.Fatal("stage not arranged")
	}
	testkit.AssertRegionGolden(t, h.Surface(), "reactivity_playground_nav", stage)
}

// TestReactivity_compactToggle_golden proves the chrome's compact-density
// toggle re-lays the chrome through the FR-3 propagation (padding is content):
// the chrome title tightens and the full shell renders the compact density.
func TestReactivity_compactToggle_golden(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)

	themeBtn := root.ChromeStack().Theme().Base().LayoutRole().ArrangedBounds
	if themeBtn.IsEmpty() {
		t.Fatal("chrome theme button not arranged")
	}
	testkit.DriveClick(h, themeBtn.Min.X+themeBtn.Width()*0.5, themeBtn.Min.Y+themeBtn.Height()*0.5)
	h.RunFrames(2)

	if !root.Shell().Compact.Get() {
		t.Fatal("theme button did not toggle compact density")
	}
	testkit.AssertGolden(t, h.Surface(), "reactivity_compact")
}
