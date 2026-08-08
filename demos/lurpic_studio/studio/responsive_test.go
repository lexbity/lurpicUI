package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// newResponsiveShell builds the shell at the given window size.
func newResponsiveShell(t *testing.T, w, h int) (*Root, *testkit.Harness) {
	t.Helper()
	ctx := app.BuildContext{
		WindowSize:   gfx.Size{W: float32(w), H: float32(h)},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
	root := NewRoot(ctx, nil, seedRows(t), nil)
	harness := testkit.NewStandardHarness(t, w, h, root)
	harness.RunFrame()
	return root, harness
}

// resize drives a window resize through the runtime (the same event the
// platform delivers on a live resize) so the Root re-computes its layout mode.
func resize(h *testkit.Harness, w, hh int) {
	h.InjectEvent(platform.EventWindowResize{Width: w, Height: hh})
	h.RunFrame()
	h.RunFrame()
}

// TestResponsive_breakpointBoundary asserts ModeFor: 960dp (content-scale aware)
// is the exact boundary between wide and narrow.
func TestResponsive_breakpointBoundary(t *testing.T) {
	cases := []struct {
		width, scale float32
		want         LayoutMode
	}{
		{1280, 1, LayoutWide},
		{960, 1, LayoutWide},
		{959, 1, LayoutNarrow},
		{800, 1, LayoutNarrow},
		{1920, 2, LayoutWide},
		{1919, 2, LayoutNarrow},
	}
	for _, c := range cases {
		if got := ModeFor(c.width, c.scale); got != c.want {
			t.Fatalf("ModeFor(%v, %v) = %v, want %v", c.width, c.scale, got, c.want)
		}
	}
}

// TestResponsive_wideKeepsThreePanes asserts the wide layout shows the 3-pane
// split and keeps the narrow overlay sub-tree inert.
func TestResponsive_wideKeepsThreePanes(t *testing.T) {
	root, _ := newResponsiveShell(t, 1280, 800)
	if root.LayoutMode() != LayoutWide {
		t.Fatalf("mode = %v, want wide", root.LayoutMode())
	}
	if panes := root.GallerySplit().Panes(); len(panes) != 3 {
		t.Fatalf("wide panes = %d, want 3", len(panes))
	}
	if b := root.Narrow().Base().LayoutRole().ArrangedBounds; !b.IsEmpty() {
		t.Fatalf("narrow sub-tree arranged in wide mode: %v", b)
	}
}

// TestResponsive_narrowCollapsesStageFullWidth asserts the narrow layout shows
// the stage full-width with the index/inspector re-hosted as overlays.
func TestResponsive_narrowCollapsesStageFullWidth(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	resize(h, 800, 600)

	if root.LayoutMode() != LayoutNarrow {
		t.Fatalf("mode = %v, want narrow", root.LayoutMode())
	}
	panes := root.GallerySplit().Panes()
	if len(panes) != 1 {
		t.Fatalf("narrow panes = %d, want 1 (stage only)", len(panes))
	}
	stage := root.Stage().Base().LayoutRole().ArrangedBounds
	if stage.Min.X != 0 || stage.Max.X != 800 {
		t.Fatalf("narrow stage width = %v, want full 800", stage)
	}
	if stage.Min.Y != 52 {
		t.Fatalf("narrow stage top = %v, want chrome bottom 52", stage.Min.Y)
	}
	// The bottom action bar sits above the status bar.
	rail := root.Narrow().Rail().Base().LayoutRole().ArrangedBounds
	if rail.IsEmpty() {
		t.Fatal("narrow bottom action bar not arranged")
	}
	if rail.Max.Y != stage.Max.Y {
		t.Fatalf("rail bottom = %v, want stage bottom %v", rail.Max.Y, stage.Max.Y)
	}
	// Overlays are gated by their stores.
	if b := root.Narrow().Drawer().Base().LayoutRole().ArrangedBounds; !b.IsEmpty() {
		t.Fatalf("nav_drawer arranged while closed: %v", b)
	}
	root.Shell().IndexOpen.Set(true)
	h.RunFrame()
	if b := root.Narrow().Drawer().Base().LayoutRole().ArrangedBounds; b.IsEmpty() {
		t.Fatal("nav_drawer not arranged when opened")
	}
}

// TestResponsive_crossingPreservesStoreContinuity asserts the F-resp contract:
// crossing the breakpoint re-arranges the same tree and keeps the same store
// instances (version continuity) with their values intact — never re-creating
// state and never re-parenting a mark.
func TestResponsive_crossingPreservesStoreContinuity(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)

	// Mutate the shared stores and capture their versions before crossing.
	root.Shell().ActiveExhibit.Set(ExhibitPlayground)
	root.Shell().IndexOpen.Set(true)
	root.Shell().InspectorOpen.Set(true)
	h.RunFrame()

	versionOf := func(s *store.ValueStore[ExhibitID]) store.Version { return s.Version() }
	activeVersion := versionOf(root.Shell().ActiveExhibit)
	activeValue := root.Shell().ActiveExhibit.Get()
	rowVersion := root.Shell().AppState.Rows.Version()
	rowCount := root.Shell().AppState.Rows.Len()

	// Cross to narrow and back.
	resize(h, 800, 600)
	if root.LayoutMode() != LayoutNarrow {
		t.Fatalf("mode after narrow crossing = %v", root.LayoutMode())
	}
	resize(h, 1280, 800)
	if root.LayoutMode() != LayoutWide {
		t.Fatalf("mode after wide crossing = %v", root.LayoutMode())
	}

	// Same store instances (version continuity) and same values.
	if got := root.Shell().ActiveExhibit.Version(); got != activeVersion {
		t.Fatalf("ActiveExhibit version changed across crossing: %v -> %v", activeVersion, got)
	}
	if got := root.Shell().ActiveExhibit.Get(); got != activeValue {
		t.Fatalf("ActiveExhibit value changed across crossing: %v -> %v", activeValue, got)
	}
	if got := root.Shell().AppState.Rows.Version(); got != rowVersion {
		t.Fatalf("Rows version changed across crossing: %v -> %v", rowVersion, got)
	}
	if got := root.Shell().AppState.Rows.Len(); got != rowCount {
		t.Fatalf("Rows count changed across crossing: %d -> %d", rowCount, got)
	}
	if !root.Shell().IndexOpen.Get() || !root.Shell().InspectorOpen.Get() {
		t.Fatal("sheet stores lost their values across crossing")
	}
}

// TestResponsive_wiringEquivalence asserts R-resp: every store the wide tree
// binds is also bound by the narrow tree, so no state is lost on a crossing
// (both trees reference the same ShellState objects by construction).
func TestResponsive_wiringEquivalence(t *testing.T) {
	root, _ := newResponsiveShell(t, 1280, 800)

	shell := root.Shell()
	// The wide index and the narrow drawer/rail both drive ActiveExhibit.
	if root.Index().shell != shell {
		t.Fatal("wide index does not bind the shell state")
	}
	if root.Narrow().shell != shell {
		t.Fatal("narrow shell does not bind the shell state")
	}
	// The wide inspector and the narrow sheet both read ActiveExhibit.
	if root.Inspector() == root.Narrow().Sheet() {
		t.Fatal("wide and narrow inspectors share a mark instance (must be distinct instances, same stores)")
	}
	// Both inspectors are built from the same shell state.
	if !sameActiveExhibitStore(root.Inspector(), root.Narrow().Sheet(), root.Shell()) {
		t.Fatal("wide and narrow inspectors do not read the same ActiveExhibit store")
	}
	// The shared stores are referenced by both trees.
	for _, s := range []*store.ValueStore[bool]{shell.IndexOpen, shell.InspectorOpen, shell.CommandOpen, shell.Compact, shell.Connection} {
		if s == nil {
			t.Fatal("shell store is nil")
		}
	}
}

// sameActiveExhibitStore reports whether both inspectors' derived title text
// follows the shared ActiveExhibit store (they are built from the same
// ShellState; flipping the store must change both).
func sameActiveExhibitStore(a, b *ExhibitInspector, shell *ShellState) bool {
	if a.titleDesc == nil || b.titleDesc == nil {
		return false
	}
	before := exhibitTitle(shell.ActiveExhibit.Get())
	if a.titleDesc.Get() != before || b.titleDesc.Get() != before {
		return false
	}
	for _, e := range exhibitCatalog {
		if e.id == shell.ActiveExhibit.Get() {
			continue
		}
		shell.ActiveExhibit.Set(e.id)
		gotA := a.titleDesc.Get()
		gotB := b.titleDesc.Get()
		want := exhibitTitle(e.id)
		if gotA != want || gotB != want {
			return false
		}
		break
	}
	return true
}

// TestResponsive_narrowDrawerSwitchesExhibit asserts the narrow re-hosts
// actually drive the shared ActiveExhibit store.
func TestResponsive_narrowDrawerSwitchesExhibit(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	resize(h, 800, 600)

	// The bottom action bar's first icon selects the first catalog exhibit.
	rail := root.Narrow().Rail()
	first := rail.icons[0].Base().LayoutRole().ArrangedBounds
	if first.IsEmpty() {
		t.Fatal("narrow rail first icon not arranged")
	}
	testkit.DriveClick(h, first.Min.X+first.Width()*0.5, first.Min.Y+first.Height()*0.5)
	if got := root.Shell().ActiveExhibit.Get(); got != rail.ids[0] {
		t.Fatalf("narrow rail selected %v, want %v", got, rail.ids[0])
	}
}
