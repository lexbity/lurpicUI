package platform_test

import (
	"os/exec"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	android "codeburg.org/lexbit/lurpicui/platform/android"
)

type memSurface struct {
	buf    []byte
	stride int
	w      int
	h      int
	locked bool
}

func (s *memSurface) Buffer() []byte { return s.buf }
func (s *memSurface) Stride() int    { return s.stride }
func (s *memSurface) Size() (width, height int) {
	return s.w, s.h
}
func (s *memSurface) Scale() float32 { return 1 }
func (s *memSurface) Resize(width, height int) {
	s.w = width
	s.h = height
	s.stride = width * 4
	s.buf = make([]byte, width*height*4)
}
func (s *memSurface) Lock() error {
	s.locked = true
	return nil
}
func (s *memSurface) Unlock(dirtyRects []gfx.Rect) error {
	s.locked = false
	return nil
}

type fakeWindow struct {
	surface platform.Surface
}

func (w *fakeWindow) Surface() platform.Surface      { return w.surface }
func (w *fakeWindow) SetTitle(title string)          {}
func (w *fakeWindow) Size() (width, height int)      { return 0, 0 }
func (w *fakeWindow) ContentScale() float32          { return 1 }
func (w *fakeWindow) SetIMECursorRect(rect gfx.Rect) {}
func (w *fakeWindow) Show()                          {}
func (w *fakeWindow) Hide()                          {}
func (w *fakeWindow) Close()                         {}
func (w *fakeWindow) Destroy()                       {}

type fakeQueue struct{}

func (fakeQueue) Push(platform.Event)                         {}
func (fakeQueue) Poll() []platform.Event                      { return nil }
func (fakeQueue) Wait(timeout time.Duration) []platform.Event { return nil }

type fakeClipboard struct {
	text string
}

func (c *fakeClipboard) ReadText() (string, error) { return c.text, nil }
func (c *fakeClipboard) WriteText(text string) error {
	c.text = text
	return nil
}

type capableApp struct {
	queue        fakeQueue
	clip         fakeClipboard
	window       *fakeWindow
	imeShowCount int
	imeHideCount int
}

func (a *capableApp) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	if a.window == nil {
		a.window = &fakeWindow{surface: &memSurface{w: opts.Width, h: opts.Height}}
	}
	return a.window, nil
}
func (a *capableApp) Events() platform.EventQueue   { return a.queue }
func (a *capableApp) Clipboard() platform.Clipboard { return &a.clip }
func (a *capableApp) SupportsHover() bool           { return true }
func (a *capableApp) ShowSoftKeyboard()             { a.imeShowCount++ }
func (a *capableApp) HideSoftKeyboard()             { a.imeHideCount++ }
func (a *capableApp) Destroy()                      {}

type plainApp struct {
	queue fakeQueue
}

func (a *plainApp) Events() platform.EventQueue { return a.queue }
func (a *plainApp) Destroy()                    {}

func TestPlatformSurfaceInterface_implementable(t *testing.T) {
	var _ platform.Surface = (*memSurface)(nil)
	s := &memSurface{buf: make([]byte, 16), stride: 8, w: 2, h: 2}
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if len(s.Buffer()) != 16 {
		t.Fatalf("unexpected buffer length: %d", len(s.Buffer()))
	}
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock(nil): %v", err)
	}
}

func TestPlatformAppInterface_implementable(t *testing.T) {
	var _ platform.App = (*capableApp)(nil)
	var _ platform.Window = (*fakeWindow)(nil)
	var _ platform.EventQueue = fakeQueue{}
	var _ platform.Clipboard = (*fakeClipboard)(nil)
	var _ platform.WindowCapable = (*capableApp)(nil)
	var _ platform.ClipboardCapable = (*capableApp)(nil)
	var _ platform.App = (*plainApp)(nil)

	app := &capableApp{}
	if q := app.Events(); q == nil {
		t.Fatal("expected event queue")
	}
	win, err := app.NewWindow(platform.WindowOptions{Width: 12, Height: 8})
	if err != nil {
		t.Fatalf("new window: %v", err)
	}
	if win == nil || win.Surface() == nil {
		t.Fatal("expected window surface")
	}
	if wc, ok := platform.WindowCapableOf(app); !ok || wc == nil {
		t.Fatal("expected window capability")
	}
	clipCap, ok := platform.ClipboardCapableOf(app)
	if !ok || clipCap == nil {
		t.Fatal("expected clipboard capability")
	}
	if pc, ok := platform.PointerCapableOf(app); !ok || pc == nil || !pc.SupportsHover() {
		t.Fatal("expected pointer capability")
	}
	if ic, ok := platform.IMECapableOf(app); !ok || ic == nil {
		t.Fatal("expected ime capability")
	}
	clip := clipCap.Clipboard()
	if err := clip.WriteText("x"); err != nil {
		t.Fatalf("clipboard write: %v", err)
	}
	if got, err := clip.ReadText(); err != nil || got != "x" {
		t.Fatalf("clipboard roundtrip = %q, %v", got, err)
	}
	if _, ok := platform.WindowCapableOf(&plainApp{}); ok {
		t.Fatal("expected plain app to lack window capability")
	}
	if _, ok := platform.ClipboardCapableOf(&plainApp{}); ok {
		t.Fatal("expected plain app to lack clipboard capability")
	}
	if _, ok := platform.PointerCapableOf(&plainApp{}); ok {
		t.Fatal("expected plain app to lack pointer capability")
	}
	if _, ok := platform.IMECapableOf(&plainApp{}); ok {
		t.Fatal("expected plain app to lack ime capability")
	}
}

func TestPlatform_ContentScale_default(t *testing.T) {
	win := &fakeWindow{surface: &memSurface{}}
	if got := win.ContentScale(); got != 1 {
		t.Fatalf("ContentScale = %v, want 1", got)
	}
}

func TestEventPointer_kind_constants_distinct(t *testing.T) {
	if platform.PointerMove == platform.PointerPress || platform.PointerMove == platform.PointerRelease || platform.PointerPress == platform.PointerRelease {
		t.Fatal("expected pointer event kinds to be distinct")
	}
}

func TestModifierKeys_bitmask_no_collision(t *testing.T) {
	if platform.ModShift == platform.ModControl || platform.ModShift == platform.ModAlt || platform.ModShift == platform.ModSuper || platform.ModControl == platform.ModAlt || platform.ModControl == platform.ModSuper || platform.ModAlt == platform.ModSuper {
		t.Fatal("expected modifier keys to be distinct powers of two")
	}
	if platform.ModShift|platform.ModControl|platform.ModAlt|platform.ModSuper != 15 {
		t.Fatalf("unexpected combined modifier mask: %d", platform.ModShift|platform.ModControl|platform.ModAlt|platform.ModSuper)
	}
}

func TestKey_constants_unique(t *testing.T) {
	keys := []platform.Key{
		platform.KeyA, platform.KeyB, platform.KeyC, platform.KeyD, platform.KeyE, platform.KeyF, platform.KeyG, platform.KeyH, platform.KeyI, platform.KeyJ,
		platform.KeyK, platform.KeyL, platform.KeyM, platform.KeyN, platform.KeyO, platform.KeyP, platform.KeyQ, platform.KeyR, platform.KeyS, platform.KeyT,
		platform.KeyU, platform.KeyV, platform.KeyW, platform.KeyX, platform.KeyY, platform.KeyZ,
		platform.KeyLeft, platform.KeyRight, platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd, platform.KeyPageUp, platform.KeyPageDown, platform.KeyEscape, platform.KeyEnter, platform.KeyTab, platform.KeyBackspace,
	}
	seen := make(map[platform.Key]struct{}, len(keys))
	for _, k := range keys {
		if _, ok := seen[k]; ok {
			t.Fatalf("duplicate key constant: %v", k)
		}
		seen[k] = struct{}{}
	}
}

func TestEventInterface_external_types_rejected(t *testing.T) {
	cmd := exec.Command("go", "test", "-tags=platformnegative", "./testdata/eventexternal")
	cmd.Env = append(cmd.Environ(), "GOCACHE=/tmp/lurpic-go-cache", "GOTMPDIR=/tmp/lurpic-go-tmp")
	out, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("expected external event package to fail compilation")
	}
	if !strings.Contains(string(out), "does not implement") {
		t.Fatalf("expected compile failure mentioning interface mismatch, got:\n%s", out)
	}
}

func TestAndroidNewApp_returns_error(t *testing.T) {
	app, err := android.NewApp()
	if err == nil {
		t.Fatal("expected non-nil error")
	}
	if app != nil {
		t.Fatalf("expected nil app, got %#v", app)
	}
	if err.Error() == "" {
		t.Fatal("expected descriptive error")
	}
}

func TestAndroidPermissionAPI_unavailable_on_non_android(t *testing.T) {
	if ch, err := android.RequestPermission(android.PermissionCamera); err == nil || ch != nil {
		t.Fatalf("expected request permission to fail on non-android, got chan=%v err=%v", ch, err)
	}
	if got, err := android.CheckPermission(android.PermissionCamera); err == nil || got != android.PermissionDenied {
		t.Fatalf("expected check permission to fail on non-android, got result=%v err=%v", got, err)
	}
}

func TestSurfaceUnlock_nil_rects_is_valid(t *testing.T) {
	s := &memSurface{buf: make([]byte, 4), stride: 4, w: 1, h: 1}
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock(nil): %v", err)
	}
}
