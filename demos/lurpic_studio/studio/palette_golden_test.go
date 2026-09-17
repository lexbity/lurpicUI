package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// TestPalette_goldenClosedAbsent asserts the closed palette's modal region
// shows no palette content (AC-2 closed state).
func TestPalette_goldenClosedAbsent(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)
	if root.Shell().CommandOpen.Get() {
		t.Fatal("palette should start closed")
	}
	region := paletteModalRegion(t, root)
	testkit.AssertRegionGolden(t, h.Surface(), "palette_closed", region)
}

// TestPalette_goldenOpenViaCtrlK asserts Ctrl+K renders the palette (AC-2).
func TestPalette_goldenOpenViaCtrlK(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)
	h.Runtime().SetFocus(root)
	testkit.DriveKeyPress(h, platform.KeyK, platform.ModControl)
	h.RunFrames(2)
	if !root.Shell().CommandOpen.Get() {
		t.Fatal("Ctrl+K did not open the palette")
	}
	region := paletteModalRegion(t, root)
	testkit.AssertRegionGolden(t, h.Surface(), "palette_open", region)
}

// TestPalette_goldenOpenViaChromeCmdK asserts the chrome ⌘K button renders the
// palette with the same pixels as Ctrl+K (same state ⇒ same pixels).
func TestPalette_goldenOpenViaChromeCmdK(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)
	cmdK := root.ChromeStack().CmdK().Base().LayoutRole().ArrangedBounds
	testkit.DriveClick(h, cmdK.Min.X+cmdK.Width()*0.5, cmdK.Min.Y+cmdK.Height()*0.5)
	h.RunFrames(2)
	if !root.Shell().CommandOpen.Get() {
		t.Fatal("chrome ⌘K button did not open the palette")
	}
	region := paletteModalRegion(t, root)
	testkit.AssertRegionGolden(t, h.Surface(), "palette_open", region)
}

// TestPalette_escapeCloses asserts Escape closes the palette and the modal
// region reverts to the closed golden.
func TestPalette_escapeCloses(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)
	region := paletteModalRegion(t, root)
	root.Shell().CommandOpen.Set(true)
	h.RunFrames(2)
	if !root.Shell().CommandOpen.Get() {
		t.Fatal("store write did not open the palette")
	}
	testkit.DriveKeyPress(h, platform.KeyEscape, 0)
	h.RunFrames(2)
	if root.Shell().CommandOpen.Get() {
		t.Fatal("Escape did not close the palette")
	}
	testkit.AssertRegionGolden(t, h.Surface(), "palette_closed", region)
}

// TestPalette_rendersAfterExhibitSwitch asserts the palette renders correctly
// after switching exhibits and reopening (A-4/A-5 class: host-switch
// re-resolution).
func TestPalette_rendersAfterExhibitSwitch(t *testing.T) {
	root, h := newResponsiveShell(t, 1280, 800)
	h.RunFrames(2)
	root.Shell().ActiveExhibit.Set(ExhibitLayers)
	h.RunFrames(2)
	root.Shell().ActiveExhibit.Set(ExhibitRealtime)
	h.RunFrames(2)
	root.Shell().CommandOpen.Set(true)
	h.RunFrames(2)
	region := paletteModalRegion(t, root)
	testkit.AssertRegionGolden(t, h.Surface(), "palette_open", region)
}

// paletteModalRegion returns the centered modal region where the palette
// surface renders.
func paletteModalRegion(t *testing.T, root *Root) gfx.Rect {
	t.Helper()
	w, h := 1280, 800
	return gfx.RectFromXYWH(float32(w*2/5), float32(h/5), float32(w/5), float32(h*2/5))
}
