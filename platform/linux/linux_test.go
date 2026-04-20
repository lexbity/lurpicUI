//go:build linux && cgo

package linux

import (
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
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

func TestLinuxAppWindowLifecycle_andSurfaceMethods(t *testing.T) {
	requireLiveDisplay(t)
	app, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer app.Destroy()

	if app.Events() == nil {
		t.Fatal("expected event queue")
	}
	if app.Clipboard() == nil {
		t.Fatal("expected clipboard")
	}

	win, err := app.NewWindow(platform.WindowOptions{Title: "linux-test", Width: 64, Height: 48})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	lw := win.(*window)
	defer lw.Destroy()

	if got := lw.Surface(); got == nil {
		t.Fatal("expected surface")
	}
	if w, h := lw.Size(); w != 64 || h != 48 {
		t.Fatalf("size = %dx%d", w, h)
	}
	if got := lw.ContentScale(); got != 1 {
		t.Fatalf("content scale = %v", got)
	}
	lw.SetTitle("updated")
	lw.SetIMECursorRect(gfx.RectFromXYWH(1, 2, 3, 4))
	lw.Show()
	lw.Hide()

	surf := lw.Surface().(*shmSurface)
	if buf := surf.Buffer(); len(buf) == 0 {
		t.Fatal("expected surface buffer")
	}
	if got := surf.Stride(); got <= 0 {
		t.Fatalf("stride = %d", got)
	}
	if w, h := surf.Size(); w != 64 || h != 48 {
		t.Fatalf("surface size = %dx%d", w, h)
	}
	if err := surf.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := surf.Unlock(nil); err != nil {
		t.Fatalf("Unlock(nil): %v", err)
	}
}

func TestLinuxEventTranslations_cover_helpers(t *testing.T) {
	requireLiveDisplay(t)
	app, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer app.Destroy()

	win, err := app.NewWindow(platform.WindowOptions{Title: "linux-events", Width: 32, Height: 24})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	lw := win.(*window)
	a := lw.app
	if a == nil {
		t.Fatal("expected app")
	}

	if got := a.translateEvent(nil); got != nil {
		t.Fatalf("nil event = %#v", got)
	}

	keyEvents := testTranslateKeyPress(a)
	if len(keyEvents) == 0 {
		t.Fatal("expected key events")
	}
	if _, ok := keyEvents[0].(platform.EventKey); !ok {
		t.Fatalf("key event = %T", keyEvents[0])
	}

	mouseEvents := testTranslatePointerButton(a)
	if len(mouseEvents) != 1 {
		t.Fatalf("mouse events = %#v", mouseEvents)
	}
	if got := testTranslatePointerButtonWithDetail(a, 2, true); len(got) != 1 {
		t.Fatalf("middle mouse = %#v", got)
	}
	if got := testTranslatePointerButtonWithDetail(a, 3, false); len(got) != 1 {
		t.Fatalf("right mouse release = %#v", got)
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

	if got := testTranslateFocus(a, uint32(lw.id), true); len(got) != 1 {
		t.Fatalf("focus in = %#v", got)
	}
	if got := testTranslateFocus(a, uint32(lw.id), false); len(got) != 1 {
		t.Fatalf("focus out = %#v", got)
	}

	if got := testTranslateConfigure(a, uint32(lw.id), 80, 60); len(got) != 1 {
		t.Fatalf("configure = %#v", got)
	}
	if got := testTranslateConfigure(a, 0, 81, 61); got != nil {
		t.Fatalf("configure missing window = %#v", got)
	}
	if w, h := lw.Size(); w != 80 || h != 60 {
		t.Fatalf("resized size = %dx%d", w, h)
	}

	if got := testTranslateClientMessage(a, uint32(lw.id)); len(got) != 1 {
		t.Fatalf("client message = %#v", got)
	}
	if got := testTranslateClientMessageWithData(a, uint32(lw.id), 1234); got != nil {
		t.Fatalf("client message mismatch = %#v", got)
	}
	if got := testTranslateClientMessage(a, 0); got != nil {
		t.Fatalf("client message missing window = %#v", got)
	}

	if got := a.translateEvent(makeUnknownEvent()); got != nil {
		t.Fatalf("unknown event = %#v", got)
	}

	a.setClipboard("hello")
	testHandleSelectionRequest(a, uint32(lw.id))
	testHandleSelectionRequestWithProperty(a, uint32(lw.id), a.atomUTF8String)
	a.DestroyClipboard()
}

func TestLinuxHelperFunctions_cover_remaining_paths(t *testing.T) {
	if got := align(5, 4); got != 8 {
		t.Fatalf("align = %d", got)
	}
	if got := align(7, 1); got != 7 {
		t.Fatalf("align = %d", got)
	}
	if got := align(7, 0); got != 7 {
		t.Fatalf("align = %d", got)
	}
	if vis, depth := findARGB32Visual(nil); vis != nil || depth != 0 {
		t.Fatalf("findARGB32Visual(nil) = %v %d", vis, depth)
	}
	if text, ok := textFromKeysym(0); ok || text != "" {
		t.Fatalf("textFromKeysym(0) = %q %v", text, ok)
	}
	if text, ok := textFromKeysym('A'); !ok || text != "A" {
		t.Fatalf("textFromKeysym(XK_A) = %q %v", text, ok)
	}

	a := &app{windows: make(map[uint32]*window)}
	if a.lookupWindow(1) != nil {
		t.Fatal("expected nil lookup")
	}
	a.clipboard = &clipboard{app: a}
	a.setClipboard("text")
	if a.clipboard == nil || a.clipboard.text != "text" || !a.clipboard.own {
		t.Fatalf("clipboard = %#v", a.clipboard)
	}
	a.DestroyClipboard()
	if a.clipboard.own {
		t.Fatal("expected clipboard ownership cleared")
	}

	var q eventQueue
	if got := q.Poll(); got != nil {
		t.Fatalf("poll = %#v", got)
	}
	if got := q.Wait(0); got != nil {
		t.Fatalf("wait = %#v", got)
	}
}

func TestLinuxClipboard_selectionPath_readsFromXSelection(t *testing.T) {
	requireLiveDisplay(t)
	platApp, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer platApp.Destroy()
	la := platApp.(*app)

	win, err := platApp.NewWindow(platform.WindowOptions{Title: "linux-clipboard-selection", Width: 24, Height: 24})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	cl := platApp.Clipboard().(*clipboard)
	if err := cl.WriteText("selection-path"); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	la.DestroyClipboard()

	stop := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-stop:
				return
			default:
			}
			_ = platApp.Events().Poll()
			time.Sleep(2 * time.Millisecond)
		}
	}()
	defer func() {
		close(stop)
		wg.Wait()
	}()

	got, err := cl.ReadText()
	if err != nil {
		t.Fatalf("ReadText: %v", err)
	}
	if got != "selection-path" {
		t.Fatalf("ReadText = %q, want selection-path", got)
	}
}

func TestLinuxQueue_waitAndPollOnLiveApp(t *testing.T) {
	requireLiveDisplay(t)
	app, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer app.Destroy()

	win, err := app.NewWindow(platform.WindowOptions{Title: "linux-queue", Width: 18, Height: 18})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	for {
		if got := app.Events().Poll(); len(got) == 0 {
			break
		}
	}
	if got := app.Events().Poll(); got != nil {
		t.Fatalf("poll = %#v", got)
	}
	if got := app.Events().Wait(1 * time.Millisecond); got != nil {
		t.Fatalf("wait = %#v", got)
	}
}

func TestLinuxSurface_resize_and_lock_contention(t *testing.T) {
	requireLiveDisplay(t)
	app, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer app.Destroy()

	win, err := app.NewWindow(platform.WindowOptions{Title: "linux-surface", Width: 20, Height: 20})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	defer win.Destroy()

	surf := win.(*window).Surface().(*shmSurface)
	if err := surf.resize(0, 0); err != nil {
		t.Fatalf("resize: %v", err)
	}
	if w, h := surf.Size(); w != 1 || h != 1 {
		t.Fatalf("resized surface = %dx%d", w, h)
	}

	surf.mu.Lock()
	surf.locked = true
	surf.mu.Unlock()

	done := make(chan struct{})
	go func() {
		time.Sleep(5 * time.Millisecond)
		surf.mu.Lock()
		surf.locked = false
		surf.mu.Unlock()
		surf.cond.Broadcast()
		close(done)
	}()
	if err := surf.Lock(); err != nil {
		t.Fatalf("Lock: %v", err)
	}
	if err := surf.Unlock(nil); err != nil {
		t.Fatalf("Unlock: %v", err)
	}
	<-done
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

func TestLinuxClipboard_roundtrip_and_close(t *testing.T) {
	requireLiveDisplay(t)
	app, err := NewApp()
	if err != nil {
		t.Skipf("NewApp unavailable in test environment: %v", err)
	}
	defer app.Destroy()

	win, err := app.NewWindow(platform.WindowOptions{Title: "linux-clipboard", Width: 16, Height: 16})
	if err != nil {
		t.Fatalf("NewWindow: %v", err)
	}
	lw := win.(*window)
	defer lw.Destroy()

	cl := app.Clipboard().(*clipboard)
	if err := cl.WriteText("hello"); err != nil {
		t.Fatalf("WriteText: %v", err)
	}
	if got, err := cl.ReadText(); err != nil || got != "hello" {
		t.Fatalf("ReadText = %q, %v", got, err)
	}
	lw.Close()
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

func requireLiveDisplay(t *testing.T) {
	t.Helper()
	display := os.Getenv("DISPLAY")
	if display == "" {
		t.Skip("DISPLAY not set")
	}
	if display[0] == ':' {
		rest := display[1:]
		if idx := indexByte(rest, '.'); idx >= 0 {
			rest = rest[:idx]
		}
		if _, err := strconv.Atoi(rest); err != nil {
			t.Skipf("DISPLAY %q is not a numeric local display", display)
		}
		socket := filepath.Join("/tmp/.X11-unix", "X"+rest)
		if _, err := os.Stat(socket); err != nil {
			t.Skipf("X display socket %q unavailable: %v", socket, err)
		}
	}
}

func indexByte(s string, c byte) int {
	for i := range s {
		if s[i] == c {
			return i
		}
	}
	return -1
}
