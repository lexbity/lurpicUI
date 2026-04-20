//go:build linux && cgo

package linux

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform"
)

func TestLinuxNewApp_no_display_returns_error(t *testing.T) {
	t.Setenv("DISPLAY", "")
	app, err := NewApp()
	if err == nil {
		t.Fatal("expected error with no DISPLAY")
	}
	if app != nil {
		t.Fatalf("expected nil app, got %#v", app)
	}
}

func TestLinuxClipboard_andApp_nilSafety(t *testing.T) {
	var a *app
	a.DestroyClipboard()

	empty := &app{}
	empty.DestroyClipboard()
	if empty.clipboard != nil {
		t.Fatalf("clipboard unexpectedly set: %#v", empty.clipboard)
	}
	empty.setClipboard("ignored")
	if empty.clipboard != nil {
		t.Fatalf("clipboard unexpectedly set after setClipboard: %#v", empty.clipboard)
	}
}

func TestKeyTranslation_all_alpha_keys(t *testing.T) {
	for ch := 'A'; ch <= 'Z'; ch++ {
		got := keyFromKeysym(uint32(ch))
		want := platform.Key(int(ch-'A') + int(platform.KeyA))
		if got != want {
			t.Fatalf("keysym %q mapped to %v, want %v", ch, got, want)
		}
	}
}

func TestKeyTranslation_navigation_keys(t *testing.T) {
	cases := map[uint32]platform.Key{
		keysymLeft:      platform.KeyLeft,
		keysymRight:     platform.KeyRight,
		keysymUp:        platform.KeyUp,
		keysymDown:      platform.KeyDown,
		keysymHome:      platform.KeyHome,
		keysymEnd:       platform.KeyEnd,
		keysymPageUp:    platform.KeyPageUp,
		keysymPageDown:  platform.KeyPageDown,
		keysymEscape:    platform.KeyEscape,
		keysymReturn:    platform.KeyEnter,
		keysymTab:       platform.KeyTab,
		keysymBackSpace: platform.KeyBackspace,
	}
	for sym, want := range cases {
		if got := keyFromKeysym(sym); got != want {
			t.Fatalf("keysym %#x mapped to %v, want %v", sym, got, want)
		}
	}
}

func TestModifierMapping_shift_ctrl_alt(t *testing.T) {
	mods := modifiersFromState((1 << 0) | (1 << 2) | (1 << 3))
	want := platform.ModShift | platform.ModControl | platform.ModAlt
	if mods != want {
		t.Fatalf("unexpected modifiers: got %v want %v", mods, want)
	}
}
