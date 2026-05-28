package runtime

import (
	"errors"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// lifecycleApp implements platform.LifecycleCapable so the runtime's
// lifecycle callbacks are bound during start().
type lifecycleApp struct {
	events       platform.EventQueue
	lifecycleMu  sync.Mutex
	onPause      func()
	onResume     func()
	onLowMemory  func()
	onSurfaceLost func()
	onSurfaceCreated func(platform.Surface)
}

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
	if fn != nil { fn() }
}

func (a *lifecycleApp) triggerResume() {
	a.lifecycleMu.Lock()
	fn := a.onResume
	a.lifecycleMu.Unlock()
	if fn != nil { fn() }
}

func (a *lifecycleApp) triggerSurfaceLost() {
	a.lifecycleMu.Lock()
	fn := a.onSurfaceLost
	a.lifecycleMu.Unlock()
	if fn != nil { fn() }
}

func (a *lifecycleApp) triggerSurfaceCreated(s platform.Surface) {
	a.lifecycleMu.Lock()
	fn := a.onSurfaceCreated
	a.lifecycleMu.Unlock()
	if fn != nil { fn(s) }
}

func (a *lifecycleApp) triggerLowMemory() {
	a.lifecycleMu.Lock()
	fn := a.onLowMemory
	a.lifecycleMu.Unlock()
	if fn != nil { fn() }
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
	app := &lifecycleApp{}
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

func waitForFatal(t *testing.T, rt *Runtime, timeout time.Duration) error {
	t.Helper()
	select {
	case err := <-rt.renderPipeline.fatalCh:
		return err
	case <-time.After(timeout):
		return errors.New("timed out waiting for fatal error")
	}
}

var _ platform.Surface = (*testSurface)(nil)

// -- Tests --

func TestRuntime_clean_lifecycle(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()

	time.Sleep(50 * time.Millisecond)
	rt.Shutdown()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Run did not return after Shutdown")
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
	rt.RunOneFrame() // triggers start(), binds lifecycle callbacks

	app.triggerSurfaceCreated(&testSurface{})

	if bf.initCount.Load() != 1 {
		t.Fatalf("expected 1 init attempt, got %d", bf.initCount.Load())
	}
}

func TestRuntime_backend_submit_failure(t *testing.T) {
	bf := &backendFixture{submitErr: errors.New("gpu submit failed")}
	rt, _ := mustLifecycleRuntime(t, bf)

	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()
	defer rt.Shutdown()

	// Wait for the fatal error to arrive via render pipeline.
	select {
	case err := <-errCh:
		if err == nil {
			t.Fatal("expected error from Run")
		}
	case <-time.After(2 * time.Second):
		rt.Shutdown()
		t.Fatal("Run did not return error within timeout")
	}

	// Must have attempted at least one submit (read after render thread stops).
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

func TestRuntime_signal_delivery_order(t *testing.T) {
	bf := &backendFixture{}
	rt, _ := mustLifecycleRuntime(t, bf)

	// start() installs the signal queue hook so enqueueSignal queues
	// through the runtime instead of delivering recursively.
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	a := store.NewValueStore(1)
	b := store.NewValueStore("")

	// When a changes, set b to match.
	a.OnChange.Subscribe(func(c signal.Change[int]) {
		b.Set("seen")
	})

	// Queue a signal that sets store a.
	rt.queueSignal(func() {
		a.Set(2)
	})

	// Deliver signals — the hook routes through the runtime queue so
	// the set on a fires in batch 1, the handler for a fires in batch 2
	// (setting b), and b's notify fires in batch 3 (if subscribed).
	// After all batches, b's value is "seen".
	rt.deliverSignals()

	if got := b.Get(); got != "seen" {
		t.Fatalf("expected b to be \"seen\", got %q", got)
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
