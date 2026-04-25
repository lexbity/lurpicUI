//go:build linux && cgo

package linux

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	platformcommon "codeburg.org/lexbit/lurpicui/platform/internal/common"
)

func TestLinuxPureWindowGuards(t *testing.T) {
	var nilWin *window
	nilWin.SetTitle("ignored")
	nilWin.Show()
	nilWin.Hide()
	nilWin.Close()
	nilWin.Destroy()
	nilWin.SetIMECursorRect(gfx.RectFromXYWH(1, 2, 3, 4))

	w := &window{closed: true}
	w.SetTitle("ignored")
	w.Show()
	w.Hide()
	w.Close()
	w.Destroy()
	w.SetIMECursorRect(gfx.RectFromXYWH(1, 2, 3, 4))

	w = &window{app: &app{}}
	w.Close()
	w.Destroy()
}

func TestLinuxPureAppGuards(t *testing.T) {
	var nilApp *app
	nilApp.Destroy()
	nilApp.DestroyClipboard()

	a := &app{}
	a.Destroy()
	a.DestroyClipboard()
	a.setClipboard("ignored")
	if a.lookupWindow(1) != nil {
		t.Fatal("expected nil lookup")
	}
	if got := a.Events(); got != nil {
		if !reflect.ValueOf(got).IsNil() {
			t.Fatalf("Events = %#v", got)
		}
	}
	if got := a.Clipboard(); got != nil {
		if !reflect.ValueOf(got).IsNil() {
			t.Fatalf("Clipboard = %#v", got)
		}
	}
}

func TestLinuxPureKeyAndTextMappings(t *testing.T) {
	cases := map[uint32]platform.Key{
		0xff51: platform.KeyLeft,
		0xff53: platform.KeyRight,
		0xff52: platform.KeyUp,
		0xff54: platform.KeyDown,
		0xff50: platform.KeyHome,
		0xff57: platform.KeyEnd,
		0xff55: platform.KeyPageUp,
		0xff56: platform.KeyPageDown,
		0xff1b: platform.KeyEscape,
		0xff0d: platform.KeyEnter,
		0x20:   platform.KeySpace,
		0xff09: platform.KeyTab,
		0xff08: platform.KeyBackspace,
	}
	for sym, want := range cases {
		if got := platformcommon.KeyFromKeysym(sym); got != want {
			t.Fatalf("keysym %#x mapped to %v, want %v", sym, got, want)
		}
	}
	if got := platformcommon.KeyFromKeysym('Z'); got != platform.KeyZ {
		t.Fatalf("keyFromKeysym(Z) = %v", got)
	}
	if got := platformcommon.KeyFromKeysym(0); got != platform.KeyUnknown {
		t.Fatalf("keyFromKeysym(0) = %v", got)
	}

	textCases := map[uint32]string{
		0x20:   " ",
		0xff09: "\t",
		0xff0d: "\n",
	}
	for sym, want := range textCases {
		got, ok := platformcommon.TextFromKeysym(sym)
		if !ok || got != want {
			t.Fatalf("textFromKeysym(%#x) = %q, %v", sym, got, ok)
		}
	}
	if got, ok := platformcommon.TextFromKeysym(0xff08); ok || got != "" {
		t.Fatalf("textFromKeysym(backspace) = %q, %v", got, ok)
	}
	if got, ok := platformcommon.TextFromKeysym(0); ok || got != "" {
		t.Fatalf("textFromKeysym(0) = %q, %v", got, ok)
	}

	mods := platformcommon.ModifiersFromState((1 << 0) | (1 << 2) | (1 << 3) | (1 << 6))
	wantMods := platform.ModShift | platform.ModControl | platform.ModAlt | platform.ModSuper
	if mods != wantMods {
		t.Fatalf("modifiersFromState = %v, want %v", mods, wantMods)
	}
}

func TestLinuxPureTranslationBranches(t *testing.T) {
	a := &app{
		windows: make(map[uint32]*window),
	}
	win := &window{app: a, id: 7}
	a.windows[7] = win
	a.atomWMDelete = 11
	a.atomUTF8String = 12
	a.atomClipboard = 13

	if got := a.translateEvent(nil); got != nil {
		t.Fatalf("nil event = %#v", got)
	}
	if got := a.translateEvent(makeUnknownEvent()); got != nil {
		t.Fatalf("unknown event = %#v", got)
	}
	if got := testTranslateKeyPress(a); len(got) != 1 {
		t.Fatalf("key press = %#v", got)
	}

	if got := testTranslatePointerButton(a); len(got) != 1 {
		t.Fatalf("pointer press = %#v", got)
	}
	if got := testTranslatePointerButtonWithDetail(a, 2, true); len(got) != 1 {
		t.Fatalf("pointer middle press = %#v", got)
	}
	if got := testTranslatePointerButtonWithDetail(a, 3, false); len(got) != 1 {
		t.Fatalf("pointer right release = %#v", got)
	}
	if got := testTranslateMotion(a); len(got) != 1 {
		t.Fatalf("motion = %#v", got)
	}
	if got := testTranslateEnterLeave(a, true); len(got) != 1 {
		t.Fatalf("enter = %#v", got)
	}
	if got := testTranslateEnterLeave(a, false); len(got) != 1 {
		t.Fatalf("leave = %#v", got)
	}
	if got := testTranslateFocus(a, 7, true); len(got) != 1 {
		t.Fatalf("focus in = %#v", got)
	}
	if got := testTranslateFocus(a, 7, false); len(got) != 1 {
		t.Fatalf("focus out = %#v", got)
	}
	if got := testTranslateFocus(a, 999, true); got != nil {
		t.Fatalf("focus missing window = %#v", got)
	}

	if got := testTranslateConfigure(a, 7, 80, 60); len(got) != 1 {
		t.Fatalf("configure = %#v", got)
	}
	if w, h := win.Size(); w != 80 || h != 60 {
		t.Fatalf("configured size = %dx%d", w, h)
	}
	if got := testTranslateConfigure(a, 999, 10, 20); got != nil {
		t.Fatalf("configure missing window = %#v", got)
	}

	if got := testTranslateClientMessage(a, 7); len(got) != 1 {
		t.Fatalf("client message = %#v", got)
	}
	if got := testTranslateClientMessageWithData(a, 7, 0); got != nil {
		t.Fatalf("client message mismatch = %#v", got)
	}
	if got := testTranslateClientMessage(a, 999); got != nil {
		t.Fatalf("client message missing window = %#v", got)
	}

	a.setClipboard("hello")
}

func TestLinuxPureClipboardBranches(t *testing.T) {
	if _, err := (&clipboard{}).ReadText(); err == nil {
		t.Fatal("expected ReadText error without app")
	}
	if err := (&clipboard{}).WriteText("x"); err == nil {
		t.Fatal("expected WriteText error without app")
	}

	a := &app{}
	c := &clipboard{app: a}
	if got, err := c.ReadText(); err == nil || got != "" {
		t.Fatalf("ReadText = %q, %v", got, err)
	}
	if err := c.WriteText("x"); err == nil {
		t.Fatal("expected WriteText error without connection")
	}

	c.app = &app{clipboardOwner: 0}
	if got, err := c.ReadText(); err == nil || got != "" {
		t.Fatalf("ReadText owner unavailable = %q, %v", got, err)
	}

	owning := &clipboard{app: &app{}, text: "owned", own: true}
	if got, err := owning.ReadText(); err != nil || got != "owned" {
		t.Fatalf("owning ReadText = %q, %v", got, err)
	}
}
