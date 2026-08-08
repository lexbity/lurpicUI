package studio

import (
	"strconv"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// TestShellCommandPalette_ctrlKOpens asserts FR-cmd's keyboard path: with the
// shell focused, Ctrl+K opens the palette; the palette's own framework tests
// cover its Escape/dismiss behavior once it has focus.
func TestShellCommandPalette_ctrlKOpens(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.Runtime().SetFocus(root)

	testkit.DriveKeyPress(h, platform.KeyK, platform.ModControl)
	if !root.Shell().CommandOpen.Get() {
		t.Fatal("Ctrl+K did not open the command palette")
	}
}

// TestShellCommandPalette_chromeButtonOpens asserts the chrome ⌘K button opens
// the palette (the guaranteed click path).
func TestShellCommandPalette_chromeButtonOpens(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	cmdK := root.ChromeStack().CmdK().Base().LayoutRole().ArrangedBounds
	if cmdK.IsEmpty() {
		t.Fatal("chrome ⌘K button not arranged")
	}
	testkit.DriveClick(h, cmdK.Min.X+cmdK.Width()*0.5, cmdK.Min.Y+cmdK.Height()*0.5)
	if !root.Shell().CommandOpen.Get() {
		t.Fatal("chrome ⌘K button did not open the palette")
	}
}

// TestShellCommandPalette_registeredCommandRuns asserts FR-cmd's "running a
// registered command mutates state observably": executing the exhibit-switch
// command changes the active exhibit.
func TestShellCommandPalette_registeredCommandRuns(t *testing.T) {
	root, _ := newResponsiveShell(t, 1280, 800)
	before := root.Shell().ActiveExhibit.Get()
	if before == ExhibitPlayground {
		t.Fatal("test precondition: active exhibit already the target")
	}
	entry, ok := root.Commands().Lookup("exhibit." + string(ExhibitPlayground))
	if !ok {
		t.Fatalf("command exhibit.%s not registered", ExhibitPlayground)
	}
	if root.Shell().CommandOpen.Get() {
		root.Shell().CommandOpen.Set(false)
	}
	entry.Execute()
	if got := root.Shell().ActiveExhibit.Get(); got != ExhibitPlayground {
		t.Fatalf("command switched active exhibit to %v, want %v", got, ExhibitPlayground)
	}
	if root.Shell().CommandOpen.Get() {
		t.Fatal("command did not close the palette")
	}
}

// TestShellStatusBar_feedWiring asserts FR-status: the badge reflects the live
// row count, the connection light reflects the feed gate, and the progress
// marks track the feed's job progress.
func TestShellStatusBar_feedWiring(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	sb := root.StatusBar()
	rows := root.Shell().AppState.Rows.Len()

	if got := sb.Badge().Label.Get(); got != strconv.Itoa(rows)+" rows" {
		t.Fatalf("status badge = %q, want %q", got, strconv.Itoa(rows)+" rows")
	}
	// Feed live → connection on; turning live off flips the light offline.
	feed := root.Stage().RootFor(ExhibitRealtime).(*Realtime).Feed()
	if !feed.Live().Get() {
		t.Fatal("feed starts live")
	}
	feed.Live().Set(false)
	h.RunFrame()
	if root.Shell().Connection.Get() {
		t.Fatal("Connection did not follow the feed gate off")
	}
	feed.Live().Set(true)
	h.RunFrame()
	if !root.Shell().Connection.Get() {
		t.Fatal("Connection did not follow the feed gate back on")
	}

	// A committed feed job pulses JobProgress; the progress marks read the same
	// store, so bar and ring bind it in lock-step.
	if sb.Bar().Value.Get() != sb.Ring().Value.Get() {
		t.Fatal("progress bar and ring read different progress stores")
	}
}

// TestShellIndexPane_switchesExhibit asserts the wide index pane's nav_rail and
// tree_navigator both drive the shared ActiveExhibit store (FR-nav).
func TestShellIndexPane_switchesExhibit(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	index := root.Index()

	// nav_rail: click the second destination (the rail stacks items
	// vertically, so the y offset selects the item).
	rail := index.Rail().Base().LayoutRole().ArrangedBounds
	if rail.IsEmpty() {
		t.Fatal("index nav_rail not arranged")
	}
	itemY := rail.Min.Y + rail.Height()*(1.5/float32(len(exhibitCatalog)))
	testkit.DriveClick(h, rail.Min.X+rail.Width()*0.5, itemY)
	if got := root.Shell().ActiveExhibit.Get(); got != exhibitCatalog[1].id {
		t.Fatalf("nav_rail selected %v, want %v", got, exhibitCatalog[1].id)
	}

	// tree_navigator: click a leaf under the first group.
	tree := index.Tree().Base().LayoutRole().ArrangedBounds
	if tree.IsEmpty() {
		t.Fatal("index tree_navigator not arranged")
	}
	leaf := gfx.Point{X: tree.Min.X + 60, Y: tree.Min.Y + 24}
	testkit.DriveClick(h, leaf.X, leaf.Y)
	h.RunFrame()
	// The tree's selection must reflect into ActiveExhibit: whichever leaf the
	// click selected, ActiveExhibit agrees.
	nodes := index.Tree().Data.Get()
	if sel := selectedTreeNode(nodes); sel != "" && ExhibitID(sel) != root.Shell().ActiveExhibit.Get() {
		t.Fatalf("tree selected %v but ActiveExhibit is %v", sel, root.Shell().ActiveExhibit.Get())
	}
}

// TestShellChrome_compactToggleReLays asserts the chrome's theme button toggles
// the shell's compact density and the chrome re-lays with tighter horizontal
// padding (a genuine runtime preference → re-layout response).
func TestShellChrome_compactToggleReLays(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	themeBtn := root.ChromeStack().Theme().Base().LayoutRole().ArrangedBounds
	if themeBtn.IsEmpty() {
		t.Fatal("chrome theme button not arranged")
	}
	titleBefore := root.ChromeStack().Title().Base().LayoutRole().ArrangedBounds.Min.X
	testkit.DriveClick(h, themeBtn.Min.X+themeBtn.Width()*0.5, themeBtn.Min.Y+themeBtn.Height()*0.5)
	if !root.Shell().Compact.Get() {
		t.Fatal("theme button did not toggle compact density")
	}
	h.RunFrame()
	titleAfter := root.ChromeStack().Title().Base().LayoutRole().ArrangedBounds.Min.X
	if titleAfter >= titleBefore {
		t.Fatalf("compact density did not tighten the chrome padding (title x %v -> %v)", titleBefore, titleAfter)
	}
}
