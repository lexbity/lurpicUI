package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TestSyncThemeColor_ResolvesFromResolvedContext proves the helper resolves a
// real theme context (the value the runtime threads into MeasureContext/
// ArrangeContext.Theme) into the non-zero token color. This is the path the
// viz marks now rely on via their OnMeasure/OnArrange callbacks.
func TestSyncThemeColor_ResolvesFromResolvedContext(t *testing.T) {
	rc := theme.DefaultResolvedContext()
	want := rc.Color(theme.ColorPrimary)

	var got gfx.Color
	syncThemeColor(rc, &got, theme.ColorPrimary)

	if got != want {
		t.Fatalf("syncThemeColor value form: got %v, want %v", got, want)
	}
	if got == (gfx.Color{}) {
		t.Fatal("resolved ColorPrimary is zero — default theme is broken; the viz marks would render invisible")
	}
}

// TestSyncThemeColor_ResolvesFromPointerContext proves the *ResolvedContext
// branch (the runtime sometimes threads a pointer).
func TestSyncThemeColor_ResolvesFromPointerContext(t *testing.T) {
	rc := theme.DefaultResolvedContext()
	want := rc.Color(theme.ColorBorder)

	var got gfx.Color
	syncThemeColor(&rc, &got, theme.ColorBorder)

	if got != want {
		t.Fatalf("syncThemeColor pointer form: got %v, want %v", got, want)
	}
}

// TestSyncThemeColor_NilThemeIsNoOp pins the no-op-on-nil contract. This is
// why calling the helper from OnAttach (where AttachContext.Theme is nil) was
// a silent no-op — the test documents that behavior so it is never mistaken
// for "theme resolved to zero."
func TestSyncThemeColor_NilThemeIsNoOp(t *testing.T) {
	var got gfx.Color
	syncThemeColor(nil, &got, theme.ColorPrimary)

	if got != (gfx.Color{}) {
		t.Fatalf("nil theme must leave the color unchanged; got %v", got)
	}
}

// TestSyncThemeColor_PreservesCallerColor proves a non-zero caller value is
// not clobbered by the theme default — the helper only fills an empty slot.
// This is the resolveVizColor precondition: an explicit Color binding wins.
func TestSyncThemeColor_PreservesCallerColor(t *testing.T) {
	rc := theme.DefaultResolvedContext()
	caller := gfx.Color{R: 0.1, G: 0.3, B: 0.7, A: 1}

	got := caller
	syncThemeColor(rc, &got, theme.ColorPrimary)

	if got != caller {
		t.Fatalf("non-zero caller color must be preserved; got %v, want %v", got, caller)
	}
}
