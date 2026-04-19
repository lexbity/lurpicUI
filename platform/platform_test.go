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
func (s *memSurface) Lock() error {
	s.locked = true
	return nil
}
func (s *memSurface) Unlock(dirtyRects []gfx.Rect) error {
	s.locked = false
	return nil
}

type stubWindow struct {
	surface platform.Surface
}

func (w *stubWindow) Surface() platform.Surface      { return w.surface }
func (w *stubWindow) SetTitle(title string)          {}
func (w *stubWindow) Size() (width, height int)      { return 0, 0 }
func (w *stubWindow) ContentScale() float32          { return 1 }
func (w *stubWindow) SetIMECursorRect(rect gfx.Rect) {}
func (w *stubWindow) Show()                          {}
func (w *stubWindow) Hide()                          {}
func (w *stubWindow) Close()                         {}
func (w *stubWindow) Destroy()                       {}

type stubQueue struct{}

func (stubQueue) Poll() []platform.Event                      { return nil }
func (stubQueue) Wait(timeout time.Duration) []platform.Event { return nil }

type stubClipboard struct {
	text string
}

func (c *stubClipboard) ReadText() (string, error) { return c.text, nil }
func (c *stubClipboard) WriteText(text string) error {
	c.text = text
	return nil
}

type stubApp struct {
	queue stubQueue
	clip  stubClipboard
}

func (a *stubApp) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	return &stubWindow{surface: &memSurface{}}, nil
}
func (a *stubApp) Events() platform.EventQueue   { return a.queue }
func (a *stubApp) Clipboard() platform.Clipboard { return &a.clip }
func (a *stubApp) Destroy()                      {}

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
	var _ platform.App = (*stubApp)(nil)
	var _ platform.Window = (*stubWindow)(nil)
	var _ platform.EventQueue = stubQueue{}
	var _ platform.Clipboard = (*stubClipboard)(nil)

	app := &stubApp{}
	win, err := app.NewWindow(platform.WindowOptions{})
	if err != nil {
		t.Fatalf("new window: %v", err)
	}
	if win == nil {
		t.Fatal("expected window")
	}
	if q := app.Events(); q == nil {
		t.Fatal("expected event queue")
	}
	if clip := app.Clipboard(); clip == nil {
		t.Fatal("expected clipboard")
	}
}

func TestPlatform_ContentScale_default(t *testing.T) {
	win := &stubWindow{surface: &memSurface{}}
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

func TestSurfaceUnlock_nil_rects_is_valid(t *testing.T) {
	s := &memSurface{buf: make([]byte, 4), stride: 4, w: 1, h: 1}
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock(nil): %v", err)
	}
}
