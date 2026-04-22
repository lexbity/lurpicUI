package app

import (
	"path/filepath"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

type fakeSurface struct {
	width  int
	height int
}

func (s *fakeSurface) Buffer() []byte           { return nil }
func (s *fakeSurface) Stride() int              { return 0 }
func (s *fakeSurface) Size() (int, int)         { return s.width, s.height }
func (s *fakeSurface) Resize(width, height int) { s.width, s.height = width, height }
func (s *fakeSurface) Lock() error              { return nil }
func (s *fakeSurface) Unlock([]gfx.Rect) error  { return nil }

type fakeWindow struct {
	surface      *fakeSurface
	title        string
	shown        bool
	destroyed    bool
	showCalls    int
	contentScale float32
}

func (w *fakeWindow) Surface() platform.Surface { return w.surface }
func (w *fakeWindow) SetTitle(title string)     { w.title = title }
func (w *fakeWindow) Size() (int, int)          { return w.surface.width, w.surface.height }
func (w *fakeWindow) ContentScale() float32 {
	if w.contentScale > 0 {
		return w.contentScale
	}
	return 1
}
func (w *fakeWindow) SetIMECursorRect(rect gfx.Rect) {}
func (w *fakeWindow) Show()                          { w.shown = true; w.showCalls++ }
func (w *fakeWindow) Hide()                          { w.shown = false }
func (w *fakeWindow) Close()                         {}
func (w *fakeWindow) Destroy()                       { w.destroyed = true }

type fakeEventQueue struct{}

func (q *fakeEventQueue) Poll() []platform.Event                      { return nil }
func (q *fakeEventQueue) Wait(timeout time.Duration) []platform.Event { return nil }

type fakeClipboard struct{}

func (c *fakeClipboard) ReadText() (string, error)   { return "", nil }
func (c *fakeClipboard) WriteText(text string) error { return nil }

type fakeApp struct {
	window    *fakeWindow
	destroyed bool
	opts      platform.WindowOptions
}

func (a *fakeApp) NewWindow(opts platform.WindowOptions) (platform.Window, error) {
	a.opts = opts
	if a.window == nil {
		a.window = &fakeWindow{surface: &fakeSurface{width: opts.Width, height: opts.Height}}
	}
	return a.window, nil
}
func (a *fakeApp) Events() platform.EventQueue   { return &fakeEventQueue{} }
func (a *fakeApp) Clipboard() platform.Clipboard { return &fakeClipboard{} }
func (a *fakeApp) Destroy()                      { a.destroyed = true }

type fakeBackend struct {
	initialized bool
	surface     render.Surface
	destroyed   bool
}

func (b *fakeBackend) Initialize(surface render.Surface) error {
	b.initialized = true
	b.surface = surface
	return nil
}
func (b *fakeBackend) Submit(frame *render.Frame) error { return nil }
func (b *fakeBackend) Resize(width, height int) error   { return nil }
func (b *fakeBackend) Destroy()                         { b.destroyed = true }

type fakeRoot struct{ facet.Facet }

func (r *fakeRoot) Base() *facet.Facet               { return &r.Facet }
func (r *fakeRoot) OnAttach(ctx facet.AttachContext) {}
func (r *fakeRoot) OnDetach()                        {}
func (r *fakeRoot) OnActivate()                      {}
func (r *fakeRoot) OnDeactivate()                    {}

func TestDefaultConfig_non_zero_fps(t *testing.T) {
	cfg := DefaultConfig("title", 10, 20)
	if cfg.Runtime.TargetFPS <= 0 {
		t.Fatalf("TargetFPS = %d", cfg.Runtime.TargetFPS)
	}
}

func TestRun_nil_builder_errors(t *testing.T) {
	if err := Run(DefaultConfig("title", 10, 20), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_builder_returning_nil_errors(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	runRuntime = func(rt *runtime.Runtime) error { return nil }
	primeRuntime = func(rt *runtime.Runtime) {}

	if err := Run(DefaultConfig("title", 10, 20), func(BuildContext) facet.FacetImpl { return nil }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_invalid_font_path_errors(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	runRuntime = func(rt *runtime.Runtime) error { return nil }
	primeRuntime = func(rt *runtime.Runtime) {}

	cfg := DefaultConfig("title", 10, 20)
	cfg.Fonts = []FontSource{{Path: filepath.Join(t.TempDir(), "missing.ttf"), Name: "missing"}}
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_window_title_set(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	cfg := DefaultConfig("hello", 640, 480)
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if app.window == nil || app.window.title != "hello" {
		t.Fatalf("title = %q", app.window.title)
	}
	if app.opts.Title != "hello" {
		t.Fatalf("opts title = %q", app.opts.Title)
	}
}

func TestRun_window_shown_after_first_frame(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	observedHidden := false
	observedShown := false
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {
		observedHidden = !app.window.shown
	}
	runRuntime = func(rt *runtime.Runtime) error {
		observedShown = app.window.shown
		return nil
	}

	if err := Run(DefaultConfig("hello", 640, 480), func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if !observedHidden {
		t.Fatal("expected first frame to occur before window show")
	}
	if !observedShown {
		t.Fatal("expected window to be shown before runtime loop")
	}
}

func TestRun_build_context_content_scale(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{
		window: &fakeWindow{
			surface:      &fakeSurface{width: 640, height: 480},
			contentScale: 2,
		},
	}
	var observed float32
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	cfg := DefaultConfig("hello", 640, 480)
	if err := Run(cfg, func(ctx BuildContext) facet.FacetImpl {
		observed = ctx.ContentScale
		return &fakeRoot{}
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if observed != 2 {
		t.Fatalf("ContentScale = %v, want 2", observed)
	}
}

func TestRun_build_context_theme_passthrough(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	customTheme := theme.Default()
	var observed theme.Context
	cfg := DefaultConfig("hello", 640, 480)
	cfg.Theme = customTheme
	if err := Run(cfg, func(ctx BuildContext) facet.FacetImpl {
		observed = ctx.Theme
		return &fakeRoot{}
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if observed == nil {
		t.Fatal("expected a non-nil theme context")
	}
	if got, want := observed.Color(theme.ColorPrimary), customTheme.Color(theme.ColorPrimary); got != want {
		t.Fatalf("Theme passthrough failed: got %#v want %#v", got, want)
	}
}

func TestRun_font_data_requires_name(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func() render.Backend { return &fakeBackend{} }
	runRuntime = func(rt *runtime.Runtime) error { return nil }
	primeRuntime = func(rt *runtime.Runtime) {}

	cfg := DefaultConfig("title", 10, 20)
	cfg.Fonts = []FontSource{{Data: []byte("abc")}}
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err == nil {
		t.Fatal("expected error")
	}
}

func restoreHooks(t *testing.T) {
	t.Helper()
	oldNewPlatformApp := newPlatformApp
	oldNewBackend := newBackend
	oldNewFontRegistry := newFontRegistry
	oldNewRuntime := newRuntime
	oldPrimeRuntime := primeRuntime
	oldRunRuntime := runRuntime
	t.Cleanup(func() {
		newPlatformApp = oldNewPlatformApp
		newBackend = oldNewBackend
		newFontRegistry = oldNewFontRegistry
		newRuntime = oldNewRuntime
		primeRuntime = oldPrimeRuntime
		runRuntime = oldRunRuntime
	})
}
