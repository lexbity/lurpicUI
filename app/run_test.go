package app

import (
	"errors"
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
func (s *fakeSurface) Scale() float32           { return 1 }
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

func (q *fakeEventQueue) Push(platform.Event)                         {}
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

type minimalApp struct{}

func (a *minimalApp) Events() platform.EventQueue { return &fakeEventQueue{} }
func (a *minimalApp) Destroy()                    {}

type fakeBackend struct {
	initialized bool
	surface     render.Surface
	destroyed   bool
	initErr     error
}

func (b *fakeBackend) Initialize(surface render.Surface) error {
	b.initialized = true
	b.surface = surface
	return b.initErr
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
	if cfg.Render != RenderBackendVulkan {
		t.Fatalf("Render = %v, want vulkan", cfg.Render)
	}
}

func TestRun_nil_builder_errors(t *testing.T) {
	if err := Run(DefaultConfig("title", 10, 20), nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_platform_without_window_capability_errors(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &minimalApp{}, nil }
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	if err := Run(DefaultConfig("title", 10, 20), func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_builder_returning_nil_errors(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	runRuntime = func(rt *runtime.Runtime) error { return nil }
	primeRuntime = func(rt *runtime.Runtime) {}

	if err := Run(DefaultConfig("title", 10, 20), func(BuildContext) facet.FacetImpl { return nil }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_invalid_font_path_errors(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
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
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
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
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
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
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
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
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	customTheme := theme.DefaultResolvedContext()
	var observed theme.ResolvedContext
	cfg := DefaultConfig("hello", 640, 480)
	cfg.Theme = customTheme
	if err := Run(cfg, func(ctx BuildContext) facet.FacetImpl {
		observed = ctx.Theme
		return &fakeRoot{}
	}); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got, want := observed.Color(theme.ColorPrimary), customTheme.Color(theme.ColorPrimary); got != want {
		t.Fatalf("Theme passthrough failed: got %#v want %#v", got, want)
	}
}

func TestRun_font_data_requires_name(t *testing.T) {
	restoreHooks(t)
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	runRuntime = func(rt *runtime.Runtime) error { return nil }
	primeRuntime = func(rt *runtime.Runtime) {}

	cfg := DefaultConfig("title", 10, 20)
	cfg.Fonts = []FontSource{{Data: []byte("abc")}}
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err == nil {
		t.Fatal("expected error")
	}
}

func TestRun_default_backend_prefers_vulkan(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	var selected RenderBackendKind
	var requested []RenderBackendKind
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func(kind RenderBackendKind) render.Backend {
		requested = append(requested, kind)
		return &fakeBackend{}
	}
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	cfg := DefaultConfig("hello", 640, 480)
	cfg.OnBackendSelected = func(kind RenderBackendKind) { selected = kind }
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if selected != RenderBackendVulkan {
		t.Fatalf("selected backend = %v, want vulkan", selected)
	}
	if len(requested) == 0 || requested[0] != RenderBackendVulkan {
		t.Fatalf("requested backends = %#v, want vulkan first", requested)
	}
}

func TestRun_vulkan_falls_back_to_software(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	var selected RenderBackendKind
	var requested []RenderBackendKind
	newPlatformApp = func() (platform.App, error) { return app, nil }
	newBackend = func(kind RenderBackendKind) render.Backend {
		requested = append(requested, kind)
		if kind == RenderBackendVulkan {
			return &fakeBackend{initErr: errors.New("vulkan unavailable")}
		}
		return &fakeBackend{}
	}
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	cfg := DefaultConfig("hello", 640, 480)
	cfg.OnBackendSelected = func(kind RenderBackendKind) { selected = kind }
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if selected != RenderBackendSoftware {
		t.Fatalf("selected backend = %v, want software", selected)
	}
	if len(requested) != 2 || requested[0] != RenderBackendVulkan || requested[1] != RenderBackendSoftware {
		t.Fatalf("requested backends = %#v, want vulkan then software", requested)
	}
}

func TestInitBackend_envOverrideVulkan(t *testing.T) {
	t.Setenv("LURPIC_RENDER_BACKEND", "vulkan")
	var requested []RenderBackendKind
	restoreHooks(t)
	newBackend = func(kind RenderBackendKind) render.Backend {
		requested = append(requested, kind)
		return &fakeBackend{}
	}

	backend, kind, err := initBackend(RenderBackendSoftware, &fakeSurface{}, nil)
	if err != nil {
		t.Fatalf("initBackend: %v", err)
	}
	if kind != RenderBackendVulkan {
		t.Fatalf("expected vulkan, got %v", kind)
	}
	if len(requested) == 0 || requested[0] != RenderBackendVulkan {
		t.Fatalf("expected vulkan backend request, got %v", requested)
	}
	_ = backend
}

func TestInitBackend_envOverrideSoftware(t *testing.T) {
	t.Setenv("LURPIC_RENDER_BACKEND", "software")
	var requested []RenderBackendKind
	restoreHooks(t)
	newBackend = func(kind RenderBackendKind) render.Backend {
		requested = append(requested, kind)
		return &fakeBackend{}
	}

	backend, kind, err := initBackend(RenderBackendVulkan, &fakeSurface{}, nil)
	if err != nil {
		t.Fatalf("initBackend: %v", err)
	}
	if kind != RenderBackendSoftware {
		t.Fatalf("expected software, got %v", kind)
	}
	if len(requested) == 0 || requested[0] != RenderBackendSoftware {
		t.Fatalf("expected software backend request, got %v", requested)
	}
	_ = backend
}

func TestInitBackend_envOverrideInvalid(t *testing.T) {
	t.Setenv("LURPIC_RENDER_BACKEND", "invalid_value")

	_, _, err := initBackend(RenderBackendVulkan, &fakeSurface{}, nil)
	if err == nil {
		t.Fatal("expected error for invalid LURPIC_RENDER_BACKEND value")
	}
}

func TestInitBackend_envUnsetUsesDefault(t *testing.T) {
	var requested []RenderBackendKind
	restoreHooks(t)
	newBackend = func(kind RenderBackendKind) render.Backend {
		requested = append(requested, kind)
		return &fakeBackend{}
	}

	// Not setting LURPIC_RENDER_BACKEND — should use the preferred value
	backend, kind, err := initBackend(RenderBackendSoftware, &fakeSurface{}, nil)
	if err != nil {
		t.Fatalf("initBackend: %v", err)
	}
	if kind != RenderBackendSoftware {
		t.Fatalf("expected software (preferred), got %v", kind)
	}
	if len(requested) == 0 || requested[0] != RenderBackendSoftware {
		t.Fatalf("expected software backend request, got %v", requested)
	}
	_ = backend
}

func TestRun_reuses_supplied_platform_app(t *testing.T) {
	restoreHooks(t)
	app := &fakeApp{}
	calledConstructor := false
	newPlatformApp = func() (platform.App, error) {
		calledConstructor = true
		return nil, errors.New("constructor should not be called when PlatformApp is supplied")
	}
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	primeRuntime = func(rt *runtime.Runtime) {}
	runRuntime = func(rt *runtime.Runtime) error { return nil }

	cfg := DefaultConfig("hello", 640, 480)
	cfg.PlatformApp = app
	if err := Run(cfg, func(BuildContext) facet.FacetImpl { return &fakeRoot{} }); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if calledConstructor {
		t.Fatal("expected supplied platform app to bypass newPlatformApp")
	}
	if app.destroyed {
		t.Fatal("expected supplied platform app to remain caller-owned")
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
