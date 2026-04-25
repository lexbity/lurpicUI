package common

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/platform"
)

func TestKeyFromKeysym(t *testing.T) {
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
		keysymSpace:     platform.KeySpace,
		keysymTab:       platform.KeyTab,
		keysymBackSpace: platform.KeyBackspace,
	}
	for sym, want := range cases {
		if got := KeyFromKeysym(sym); got != want {
			t.Fatalf("keysym %#x mapped to %v, want %v", sym, got, want)
		}
	}
	if got := KeyFromKeysym('Z'); got != platform.KeyZ {
		t.Fatalf("KeyFromKeysym(Z) = %v", got)
	}
	if got := KeyFromKeysym(0); got != platform.KeyUnknown {
		t.Fatalf("KeyFromKeysym(0) = %v", got)
	}
}

func TestTextFromKeysym(t *testing.T) {
	cases := map[uint32]string{
		keysymSpace:  " ",
		keysymTab:    "\t",
		keysymReturn: "\n",
	}
	for sym, want := range cases {
		got, ok := TextFromKeysym(sym)
		if !ok || got != want {
			t.Fatalf("TextFromKeysym(%#x) = %q, %v", sym, got, ok)
		}
	}
	if got, ok := TextFromKeysym(keysymBackSpace); ok || got != "" {
		t.Fatalf("TextFromKeysym(backspace) = %q, %v", got, ok)
	}
}

func TestModifiersFromState(t *testing.T) {
	mods := ModifiersFromState((1 << 0) | (1 << 2) | (1 << 3) | (1 << 6))
	want := platform.ModShift | platform.ModControl | platform.ModAlt | platform.ModSuper
	if mods != want {
		t.Fatalf("ModifiersFromState = %v, want %v", mods, want)
	}
}
