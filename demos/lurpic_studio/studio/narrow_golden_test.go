package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// newNarrowShell builds the narrow-mode shell (720x800 below the 960dp
// breakpoint) and runs a settling frame.
func newNarrowShell(t *testing.T) (*Root, *testkit.Harness) {
	t.Helper()
	root, h := newResponsiveShell(t, 720, 800)
	h.RunFrame()
	return root, h
}

// TestNarrow_goldenDefault pins the narrow shell with no overlays open
// (RX-1 FR-17): the stage content stops above the bottom action bar, the bar
// renders its icons, and no scrim is present.
func TestNarrow_goldenDefault(t *testing.T) {
	root, h := newNarrowShell(t)
	if root.Shell().IndexOpen.Get() || root.Shell().InspectorOpen.Get() {
		t.Fatal("precondition: an overlay is open by default")
	}
	testkit.AssertGolden(t, h.Surface(), "narrow_default")
}

// TestNarrow_goldenDrawer pins the nav_drawer open with its scrim dimming the
// stage behind it (FR-17b).
func TestNarrow_goldenDrawer(t *testing.T) {
	root, h := newNarrowShell(t)
	root.Shell().IndexOpen.Set(true)
	h.RunFrames(2)
	if !root.Shell().IndexOpen.Get() {
		t.Fatal("drawer did not open")
	}
	testkit.AssertGolden(t, h.Surface(), "narrow_drawer")
}

// TestNarrow_goldenSheet pins the inspector bottom sheet open with its scrim
// and drag handle (FR-17c).
func TestNarrow_goldenSheet(t *testing.T) {
	root, h := newNarrowShell(t)
	root.Shell().InspectorOpen.Set(true)
	h.RunFrames(2)
	if !root.Shell().InspectorOpen.Get() {
		t.Fatal("sheet did not open")
	}
	testkit.AssertGolden(t, h.Surface(), "narrow_sheet")
}

// TestNarrow_noContentUnderActionBar asserts RX-1 FR-17a structurally: the
// stage's arranged content stops at or above the bottom action bar, so the bar
// never occludes scrollable stage content.
func TestNarrow_noContentUnderActionBar(t *testing.T) {
	root, h := newNarrowShell(t)
	bar := root.Narrow().Rail().Base().LayoutRole().ArrangedBounds
	if bar.IsEmpty() {
		t.Fatal("bottom action bar not arranged")
	}
	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	if stage.IsEmpty() {
		t.Fatal("narrow stage not arranged")
	}
	if stage.Max.Y > bar.Min.Y {
		t.Fatalf("stage content %v overlaps the action bar %v (FR-17a occlusion)", stage, bar)
	}
	// The stage still fills the width above the bar.
	if stage.Min.X != 0 || stage.Max.X != 720 {
		t.Fatalf("narrow stage width = %v, want full 720", stage)
	}
	_ = h
}

// TestNarrow_scrimBlocksStage asserts RX-1 FR-17b: while a sheet is open, a
// click on the stage area hits the scrim (the stage beneath is blocked); with
// nothing open the scrim has no hit region.
func TestNarrow_scrimBlocksStage(t *testing.T) {
	root, h := newNarrowShell(t)
	scrim := root.Narrow().Scrim()
	scrimID := scrim.Base().ID()
	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	pt := gfx.Point{X: stage.Min.X + stage.Width()*0.6, Y: stage.Min.Y + stage.Height()*0.4}
	if stage.Contains(pt) != true {
		t.Fatal("sample point not inside the stage")
	}

	// Nothing open: the scrim has no hit region — the point resolves to the
	// stage (or below), not the scrim.
	if got := h.Runtime().HitTest(pt); got == scrimID {
		t.Fatal("scrim hit while no overlay is open")
	}

	// Open the sheet: the scrim's hit region now covers the stage and blocks it.
	root.Shell().InspectorOpen.Set(true)
	h.RunFrames(2)
	if got := h.Runtime().HitTest(pt); got != scrimID {
		t.Fatalf("open-sheet hit at %v = %d, want the scrim %d (FR-17b)", pt, got, scrimID)
	}
}

// TestNarrow_escapeClosesSheet asserts RX-1 FR-17c: Escape closes the open
// bottom sheet (the shell's key path).
func TestNarrow_escapeClosesSheet(t *testing.T) {
	root, h := newNarrowShell(t)
	root.Shell().InspectorOpen.Set(true)
	h.RunFrame()
	if !root.Shell().InspectorOpen.Get() {
		t.Fatal("precondition: sheet did not open")
	}
	h.Runtime().SetFocus(root)
	testkit.DriveKeyPress(h, platform.KeyEscape, 0)
	h.RunFrame()
	if root.Shell().InspectorOpen.Get() {
		t.Fatal("Escape did not close the sheet (FR-17c)")
	}
}

// TestNarrow_outsideTapClosesDrawer asserts RX-1 FR-17b: tapping the scrim
// (outside the drawer) dismisses the drawer.
func TestNarrow_outsideTapClosesDrawer(t *testing.T) {
	root, h := newNarrowShell(t)
	root.Shell().IndexOpen.Set(true)
	h.RunFrame()
	if !root.Shell().IndexOpen.Get() {
		t.Fatal("precondition: drawer did not open")
	}
	// Tap the scrim at a stage point away from the drawer's left edge.
	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	pt := gfx.Point{X: stage.Min.X + stage.Width()*0.7, Y: stage.Min.Y + stage.Height()*0.5}
	testkit.DriveClick(h, pt.X, pt.Y)
	h.RunFrame()
	if root.Shell().IndexOpen.Get() {
		t.Fatal("tapping the scrim did not close the drawer (FR-17b)")
	}
}
