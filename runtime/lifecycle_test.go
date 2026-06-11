package runtime

import (
	"errors"
	"reflect"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

// lifecycleApp implements platform.LifecycleCapable so the runtime's
// lifecycle callbacks are bound during start().
type lifecycleApp struct {
	events           *fakeEventQueue
	lifecycleMu      sync.Mutex
	onPause          func()
	onResume         func()
	onLowMemory      func()
	onSurfaceLost    func()
	onSurfaceCreated func(platform.Surface)
}

type fakeEventQueue struct {
	mu     sync.Mutex
	events []platform.Event
}

func (q *fakeEventQueue) Push(e platform.Event) {
	q.mu.Lock()
	q.events = append(q.events, e)
	q.mu.Unlock()
}
func (q *fakeEventQueue) Poll() []platform.Event {
	q.mu.Lock()
	e := q.events
	q.events = nil
	q.mu.Unlock()
	return e
}
func (q *fakeEventQueue) Wait(timeout time.Duration) []platform.Event { return q.Poll() }

func (a *lifecycleApp) Events() platform.EventQueue { return a.events }
func (a *lifecycleApp) Destroy()                    {}
func (a *lifecycleApp) OnPause(fn func()) {
	a.lifecycleMu.Lock()
	a.onPause = fn
	a.lifecycleMu.Unlock()
}
func (a *lifecycleApp) OnResume(fn func()) {
	a.lifecycleMu.Lock()
	a.onResume = fn
	a.lifecycleMu.Unlock()
}
func (a *lifecycleApp) OnLowMemory(fn func()) {
	a.lifecycleMu.Lock()
	a.onLowMemory = fn
	a.lifecycleMu.Unlock()
}
func (a *lifecycleApp) OnSurfaceLost(fn func()) {
	a.lifecycleMu.Lock()
	a.onSurfaceLost = fn
	a.lifecycleMu.Unlock()
}
func (a *lifecycleApp) OnSurfaceCreated(fn func(platform.Surface)) {
	a.lifecycleMu.Lock()
	a.onSurfaceCreated = fn
	a.lifecycleMu.Unlock()
}

func (a *lifecycleApp) triggerPause() {
	a.lifecycleMu.Lock()
	fn := a.onPause
	a.lifecycleMu.Unlock()
	if fn != nil {
		fn()
	}
}

func (a *lifecycleApp) triggerResume() {
	a.lifecycleMu.Lock()
	fn := a.onResume
	a.lifecycleMu.Unlock()
	if fn != nil {
		fn()
	}
}

func (a *lifecycleApp) triggerSurfaceLost() {
	a.lifecycleMu.Lock()
	fn := a.onSurfaceLost
	a.lifecycleMu.Unlock()
	if fn != nil {
		fn()
	}
}

func (a *lifecycleApp) triggerSurfaceCreated(s platform.Surface) {
	a.lifecycleMu.Lock()
	fn := a.onSurfaceCreated
	a.lifecycleMu.Unlock()
	if fn != nil {
		fn(s)
	}
}

func (a *lifecycleApp) triggerEvent(ev platform.Event) {
	q := a.Events()
	if q != nil {
		q.Push(ev)
	}
}

var _ platform.App = (*lifecycleApp)(nil)
var _ platform.LifecycleCapable = (*lifecycleApp)(nil)

// testSurface is an in-memory platform.Surface for lifecycle tests.
type testSurface struct{}

func (s *testSurface) Size() (int, int) { return 800, 600 }
func (s *testSurface) Resize(int, int)  {}
func (s *testSurface) Scale() float32   { return 1 }

func mustLifecycleRuntime(t *testing.T, b render.Backend) (*Runtime, *lifecycleApp) {
	t.Helper()
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	app := &lifecycleApp{events: &fakeEventQueue{}}
	win := &testWindow{width: 800, height: 600}
	if b == nil {
		b = &backendFixture{}
	}
	rt, err := New(cfg, app, win, b, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt, app
}

func mustLifecycleRuntimeWithAssetsAndApp(t *testing.T, mgr assets.Manager, reg *assets.AssetRegistryStore) (*Runtime, *lifecycleApp) {
	t.Helper()
	root := facet.NewFacet()
	cfg := DefaultConfig()
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.AssetManager = mgr
	cfg.AssetRegistry = reg
	app := &lifecycleApp{events: &fakeEventQueue{}}
	win := &testWindow{width: 800, height: 600}
	rt, err := New(cfg, app, win, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt, app
}

var _ platform.Surface = (*testSurface)(nil)

// -- Tests --

func TestRuntime_clean_lifecycle(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	frameSubmitted := make(chan struct{}, 1)
	rt.onFrameSubmitted = func() {
		select {
		case frameSubmitted <- struct{}{}:
		default:
		}
	}

	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()

	select {
	case <-frameSubmitted:
	case <-time.After(2 * time.Second):
		rt.Shutdown()
		t.Fatal("first frame not submitted within timeout")
	}

	rt.Shutdown()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after Shutdown")
	}

	if bf.submitCount.Load() == 0 {
		t.Fatal("expected at least one frame submitted during run")
	}
}

func TestRuntime_shutdown_idempotent(t *testing.T) {
	rt, _ := mustLifecycleRuntime(t, nil)
	rt.Shutdown()
	rt.Shutdown()
	rt.Shutdown()
}

func TestRuntime_backend_initialize_failure(t *testing.T) {
	bf := &backendFixture{initializeErr: errors.New("gpu init failed")}
	rt, app := mustLifecycleRuntime(t, bf)
	rt.RunOneFrame()

	app.triggerSurfaceLost()

	app.triggerSurfaceCreated(&testSurface{})

	if bf.initCount.Load() != 1 {
		t.Fatalf("expected 1 init attempt, got %d", bf.initCount.Load())
	}

	if rt.isSurfaceReady() {
		t.Fatal("surface marked ready despite init failure")
	}

	bf.submitCount.Store(0)
	rt.RunOneFrame()
	if bf.submitCount.Load() != 0 {
		t.Fatal("submit occurred while surface is not ready")
	}
}

func TestRuntime_backend_submit_failure(t *testing.T) {
	sentinel := errors.New("gpu submit failed")
	bf := &backendFixture{submitErr: sentinel}
	rt, _ := mustLifecycleRuntime(t, bf)

	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()
	defer rt.Shutdown()

	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Run")
		}
		if !errors.Is(err, sentinel) {
			t.Fatalf("Run returned %v, want error wrapping %v", err, sentinel)
		}
	case <-time.After(2 * time.Second):
		rt.Shutdown()
		t.Fatal("Run did not return error within timeout")
	}

	if bf.submitCount.Load() == 0 {
		t.Fatal("expected at least one submit attempt")
	}
}

func TestRuntime_shutdown_blocks_until_started(t *testing.T) {
	rt, _ := mustLifecycleRuntime(t, nil)

	// Shutdown before start() completes should not deadlock.
	done := make(chan struct{})
	go func() {
		rt.Shutdown()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Shutdown deadlocked when called before start")
	}
}

func TestRuntime_surface_loss_prevents_submit(t *testing.T) {
	rb := &recreatableBackend{}
	rt, app := mustLifecycleRuntime(t, rb)

	rt.RunOneFrame()
	preLoss := rb.submitCount.Load()
	if preLoss == 0 {
		t.Fatal("expected at least 1 submit before loss")
	}

	// Surface loss — destroy is called, surfaceReady becomes false.
	app.triggerSurfaceLost()

	if rb.destroyCount.Load() != 1 {
		t.Fatalf("expected 1 destroy, got %d", rb.destroyCount.Load())
	}
	if rt.isSurfaceReady() {
		t.Fatal("expected surfaceReady=false after surface loss")
	}

	// Frames during surface loss should NOT submit (surface not ready).
	rb.submitCount.Store(0)
	rt.RunOneFrame()
	rt.RunOneFrame()
	if rb.submitCount.Load() != 0 {
		t.Fatalf("expected 0 submits while surface absent, got %d", rb.submitCount.Load())
	}

	// Surface restore — Recreate is called instead of Initialize.
	app.triggerSurfaceCreated(&testSurface{})

	if rb.recreateCount.Load() != 1 {
		t.Fatalf("expected 1 recreate after restore, got %d", rb.recreateCount.Load())
	}
	if rb.initCount.Load() != 0 {
		t.Fatalf("expected 0 init calls (used Recreate), got %d", rb.initCount.Load())
	}
	if !rt.isSurfaceReady() {
		t.Fatal("expected surfaceReady=true after surface restore")
	}

	// Frames after restore submit again.
	rb.submitCount.Store(0)
	rt.RunOneFrame()
	rt.RunOneFrame()
	if rb.submitCount.Load() != 2 {
		t.Fatalf("expected 2 submits after restore, got %d", rb.submitCount.Load())
	}
}

func TestRuntime_surface_loss_and_recovery(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	// Initial frame renders.
	rt.RunOneFrame()
	if bf.submitCount.Load() != 1 {
		t.Fatalf("expected 1 submit, got %d", bf.submitCount.Load())
	}

	// Surface loss — destroy is called.
	app.triggerSurfaceLost()

	if bf.destroyCount.Load() != 1 {
		t.Fatalf("expected 1 destroy, got %d", bf.destroyCount.Load())
	}

	// Frames during surface loss should not submit.
	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()
	if bf.submitCount.Load() != 1 {
		t.Fatalf("expected 1 submit (no change after loss), got %d", bf.submitCount.Load())
	}

	// Surface restore — reinitialize.
	app.triggerSurfaceCreated(&testSurface{})

	if bf.initCount.Load() != 1 {
		t.Fatalf("expected 1 init after restore, got %d", bf.initCount.Load())
	}

	// Frames after restore submit again.
	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()
	if bf.submitCount.Load() != 4 {
		t.Fatalf("expected 4 submits after restore, got %d", bf.submitCount.Load())
	}
}

func TestRuntime_pause_stops_frame_advancement(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	// First frame binds lifecycle callbacks.
	rt.RunOneFrame()
	bf.submitCount.Store(0) // reset counter after init

	app.triggerPause()

	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()

	if bf.submitCount.Load() != 0 {
		t.Fatalf("expected 0 submits while paused, got %d", bf.submitCount.Load())
	}
}

func TestRuntime_resume_after_pause(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	// First frame binds lifecycle callbacks.
	rt.RunOneFrame()
	bf.submitCount.Store(0) // reset counter after init

	app.triggerPause()
	rt.RunOneFrame()

	app.triggerResume()

	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()

	if bf.submitCount.Load() != 3 {
		t.Fatalf("expected 3 submits after resume, got %d", bf.submitCount.Load())
	}
}

func TestRuntime_signal_delivery_cross_store_cascade(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	a := store.NewValueStore(1)
	b := store.NewValueStore("")
	var order []string

	a.OnChange.Subscribe(func(c signal.Change[int]) {
		order = append(order, "a-handler")
		b.Set("seen")
	})

	b.OnChange.Subscribe(func(c signal.Change[string]) {
		order = append(order, "b-notify")
	})

	rt.queueSignal(func() {
		order = append(order, "queue-set")
		a.Set(2)
	})

	rt.deliverSignals()

	if got := b.Get(); got != "seen" {
		t.Fatalf("expected b to be \"seen\", got %q", got)
	}

	want := []string{"queue-set", "a-handler", "b-notify"}
	if len(order) != len(want) {
		t.Fatalf("delivery order = %v, want %v", order, want)
	}
	for i := range want {
		if order[i] != want[i] {
			t.Fatalf("delivery order = %v, want %v", order, want)
		}
	}
}

func TestRuntime_configurationChanged_triggersLayout(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()
	bf.submitCount.Store(0)

	// Send a configuration change event through the platform event queue.
	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation:    2, // landscape
		ScreenWidthDp:  800,
		ScreenHeightDp: 480,
		Density:        320,
		UiModeNight:    true,
		FontScale:      1.25,
		Language:       "en",
		Country:        "US",
	})

	// The event should be consumed by handleWindowEvents (not passed through
	// to the input system as a routed event). Instead, it marks the tree dirty.
	rt.RunOneFrame()

	// After config change, the frame should still submit (no crash).
	if bf.submitCount.Load() == 0 {
		t.Fatal("expected submit after config change")
	}

	// Content scale should be updated based on density.
	expectedScale := float32(320) / 160.0 // 2.0
	if rt.contentScale != expectedScale {
		t.Fatalf("expected contentScale=%.1f after density change, got %.1f", expectedScale, rt.contentScale)
	}
}

func TestRuntime_configurationChanged_darkModeTogglesTheme(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()
	initialScale := rt.contentScale
	bf.submitCount.Store(0)

	store, ok := rt.RootStyleContext().(*theme.StyleContextStore)
	if !ok {
		t.Fatal("expected root style context to be a StyleContextStore")
	}
	initialTokens := store.Get().Tokens

	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation: 1,
		UiModeNight: true,
		Density:     160,
	})

	rt.RunOneFrame()
	rt.RunOneFrame()

	if bf.submitCount.Load() != 2 {
		t.Fatalf("expected 2 submits after config change, got %d", bf.submitCount.Load())
	}

	if rt.contentScale != initialScale {
		t.Fatalf("contentScale changed from %f to %f despite density being unchanged",
			initialScale, rt.contentScale)
	}

	if reflect.DeepEqual(store.Get().Tokens, initialTokens) {
		t.Fatal("theme tokens did not change after dark mode config event")
	}

	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation: 1,
		UiModeNight: false,
		Density:     160,
	})

	rt.RunOneFrame()

	if reflect.DeepEqual(store.Get().Tokens, theme.DarkTokens()) {
		t.Fatal("theme tokens did not revert to light after UiModeNight=false")
	}
	if !reflect.DeepEqual(store.Get().Tokens, theme.DefaultTokens()) {
		t.Fatal("theme tokens should match DefaultTokens after UiModeNight=false")
	}
}

func TestRuntime_configurationChanged_densityUpdatesContentScale(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()

	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation: 1,
		Density:     320,
	})
	rt.RunOneFrame()

	expectedScale := float32(320) / 160.0
	if rt.contentScale != expectedScale {
		t.Fatalf("contentScale = %f, want %f after density change from 160 to 320",
			rt.contentScale, expectedScale)
	}
	if rt.contentScale <= 0 {
		t.Fatal("contentScale must be positive after density change")
	}
}

func TestRuntime_configurationChanged_localeDoesNotCrash(t *testing.T) {
	bf := &backendFixture{}
	rt, app := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()

	// Locale change from en/US to de/DE.
	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation: 1,
		Density:     160,
		Language:    "de",
		Country:     "DE",
	})

	rt.RunOneFrame()
	rt.RunOneFrame()

	// The important thing is that the runtime doesn't crash after a
	// locale change (the change triggers a tree-dirty, which triggers
	// re-projection, which re-acquires asset handles via LoadSVG etc.).
	if bf.submitCount.Load() == 0 {
		t.Fatal("expected submits after locale change")
	}
}

func TestRuntime_configurationChanged_assetManagerNotNiled(t *testing.T) {
	mgr := &assetDiagFixture{}
	reg := assets.NewAssetRegistryStore()
	rt, app := mustLifecycleRuntimeWithAssetsAndApp(t, mgr, reg)

	rt.RunOneFrame()

	before := rt.AssetManager()

	app.triggerEvent(platform.ConfigurationChangedEvent{
		Orientation:    1,
		ScreenWidthDp:  800,
		ScreenHeightDp: 480,
		Density:        320,
		UiModeNight:    false,
		FontScale:      1.0,
	})

	rt.RunOneFrame()

	if rt.AssetManager() == nil {
		t.Fatal("asset manager niled by config change")
	}
	if rt.AssetManager() != before {
		t.Fatal("asset manager replaced by config change")
	}
	if rt.AssetRegistry() == nil {
		t.Fatal("asset registry must not be nil after config change")
	}
}

func TestRuntime_empty_facet_tree(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()

	if bf.submitCount.Load() == 0 {
		t.Fatal("expected at least one submit with empty tree")
	}
}

func TestRuntime_run_one_frame_idempotent(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	rt.RunOneFrame()
	rt.RunOneFrame()
	rt.RunOneFrame()

	if bf.submitCount.Load() != 3 {
		t.Fatalf("expected 3 submits, got %d", bf.submitCount.Load())
	}
}
