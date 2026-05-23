package runtime

import (
	"image/color"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRuntime_start_registers_thread(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !syncutil.OnRuntimeThread() {
		t.Fatal("expected runtime thread")
	}
	rt.Shutdown()
}

func TestRuntime_shutdown_clean(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.Shutdown()
}

func TestRuntime_phase1TickHooks_areScoped_and_cleared_on_shutdown(t *testing.T) {
	rt := mustRuntime(t)
	var count int
	unregister := rt.RegisterPhase1TickHook(func(time.Duration) {
		count++
	})
	rt.runPhase1TickHooks(time.Second)
	if count != 1 {
		t.Fatalf("count before shutdown = %d", count)
	}
	rt.shutdown()
	rt.runPhase1TickHooks(time.Second)
	if count != 1 {
		t.Fatalf("count after shutdown = %d", count)
	}
	unregister()
}

func TestRuntime_shutdownHooks_are_invoked_and_cleared(t *testing.T) {
	rt := mustRuntime(t)
	var count int
	unregister := rt.RegisterShutdownHook(func() {
		count++
	})
	rt.runShutdownHooks()
	if count != 1 {
		t.Fatalf("count before clear = %d", count)
	}
	unregister()
	rt.runShutdownHooks()
	if count != 1 {
		t.Fatalf("count after clear = %d", count)
	}
}

func TestRuntime_shutdown_disposes_tree_bottomup(t *testing.T) {
	order := []string{}
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root", detachOrder: &order}
	rt := mustRuntimeTree(t, root)
	rt.disposeTree(root)
	if len(order) != 1 || order[0] != "root" {
		t.Fatalf("order = %#v", order)
	}
}

func TestRuntime_attachtree_calls_onattach(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	if root.attachCount != 1 {
		t.Fatalf("attach count = %d", root.attachCount)
	}
}

func TestRuntime_subscribe_builder_integrates_with_lifecycle(t *testing.T) {
	s := store.NewValueStore(1)
	root := newRuntimeSubscriptionFacet(s)
	rt := mustRuntimeTree(t, root)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if got := root.Subs().Len(); got != 1 {
		t.Fatalf("subscriptions after attach = %d", got)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("subscribed versions after attach = %#v", got)
	}

	s.Set(2)
	rt.RunOneFrame()

	if root.changeCount != 1 {
		t.Fatalf("changeCount = %d", root.changeCount)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("subscribed versions after update = %#v", got)
	}

	rt.Shutdown()
	if s.OnChange.HasSubscribers() {
		t.Fatal("expected subscriptions to be released on shutdown")
	}
}

func TestRuntime_lifecycle_callbacks_pause_and_resume(t *testing.T) {
	app := newMockLifecycleApp()
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 10, 10), color.RGBA{A: 255})
	backend := &evictingBackend{}
	rt := mustRuntimeWithAppAndBackend(t, app, &root.Facet, backend)
	rt.anchorCaches[1] = layout.NewAnchorPositionCache()
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	app.firePause()
	if !rt.isPaused() {
		t.Fatal("expected runtime to pause")
	}

	rt.RunOneFrame()
	if got := backend.submitCount; got != 0 {
		t.Fatalf("unexpected submit count while paused = %d", got)
	}

	app.fireSurfaceLost()
	app.fireSurfaceCreated()
	if got := app.surfaceLostCount; got != 1 {
		t.Fatalf("surface lost count = %d, want 1", got)
	}
	if got := app.surfaceCreatedCount; got != 1 {
		t.Fatalf("surface created count = %d, want 1", got)
	}
	if got := backend.destroyCount; got != 1 {
		t.Fatalf("backend destroy count = %d, want 1", got)
	}
	if got := backend.initializeCount; got != 1 {
		t.Fatalf("backend initialize count = %d, want 1", got)
	}
	if backend.lastSurface == nil {
		t.Fatal("expected backend to receive recreated surface")
	}

	app.fireLowMemory()
	if got := app.lowMemoryCount; got != 1 {
		t.Fatalf("low memory count = %d, want 1", got)
	}
	if got := backend.evictCount; got != 1 {
		t.Fatalf("backend evict count = %d, want 1", got)
	}
	if got := len(rt.anchorCaches); got != 0 {
		t.Fatalf("anchor caches = %d, want 0", got)
	}

	app.fireResume()
	if rt.isPaused() {
		t.Fatal("expected runtime to resume")
	}

	rt.RunOneFrame()
	if got := backend.submitCount; got == 0 {
		t.Fatal("expected frame submission after resume")
	}

	rt.Shutdown()
}

func TestRuntime_updateIMECursorRect_toggles_keyboard_visibility(t *testing.T) {
	app := newMockLifecycleApp()
	root := newRuntimeIMEFacet()
	backend := &evictingBackend{}
	rt := mustRuntimeWithAppAndBackend(t, app, &root.Facet, backend)
	rt.window = &testWindow{}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	rt.SetFocus(root)
	rt.updateIMECursorRect()
	if got := app.imeShowCount; got != 1 {
		t.Fatalf("imeShowCount = %d, want 1", got)
	}
	if got := app.imeHideCount; got != 0 {
		t.Fatalf("imeHideCount = %d, want 0", got)
	}
	if got := rt.imeVisible; !got {
		t.Fatal("expected imeVisible to be true")
	}

	rt.ClearFocus()
	if got := app.imeHideCount; got != 1 {
		t.Fatalf("imeHideCount = %d, want 1", got)
	}
	if got := rt.imeVisible; got {
		t.Fatal("expected imeVisible to be false")
	}

	rt.Shutdown()
}

func TestRuntime_activatetree_calls_onactivate(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	rt.activateTree(root)
	if root.activateCount != 1 {
		t.Fatalf("activate count = %d", root.activateCount)
	}
}

func TestRuntime_marktreedirty_sets_flags(t *testing.T) {
	root, child, leaf := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	rt.markTreeDirty(root, facet.DirtyAll)
	if root.DirtyFlags() != facet.DirtyAll || child.DirtyFlags() != facet.DirtyAll || leaf.DirtyFlags() != facet.DirtyAll {
		t.Fatalf("dirty flags = %#v %#v %#v", root.DirtyFlags(), child.DirtyFlags(), leaf.DirtyFlags())
	}
}

type mockLifecycleApp struct {
	queue lifecycleStubQueue

	onPause          []func()
	onResume         []func()
	onLowMemory      []func()
	onSurfaceLost    []func()
	onSurfaceCreated []func(platform.Surface)

	surfaceLostCount    int
	surfaceCreatedCount int
	lowMemoryCount      int
	imeShowCount        int
	imeHideCount        int
}

type lifecycleStubQueue struct{}

func (lifecycleStubQueue) Push(platform.Event)                         {}
func (lifecycleStubQueue) Poll() []platform.Event                      { return nil }
func (lifecycleStubQueue) Wait(timeout time.Duration) []platform.Event { return nil }

func newMockLifecycleApp() *mockLifecycleApp {
	return &mockLifecycleApp{}
}

func (a *mockLifecycleApp) Events() platform.EventQueue { return a.queue }
func (a *mockLifecycleApp) Destroy()                    {}

func (a *mockLifecycleApp) OnPause(f func()) {
	a.onPause = append(a.onPause, f)
}

func (a *mockLifecycleApp) OnResume(f func()) {
	a.onResume = append(a.onResume, f)
}

func (a *mockLifecycleApp) OnLowMemory(f func()) {
	a.onLowMemory = append(a.onLowMemory, f)
}

func (a *mockLifecycleApp) OnSurfaceLost(f func()) {
	a.onSurfaceLost = append(a.onSurfaceLost, f)
}

func (a *mockLifecycleApp) OnSurfaceCreated(f func(platform.Surface)) {
	a.onSurfaceCreated = append(a.onSurfaceCreated, f)
}

func (a *mockLifecycleApp) ShowSoftKeyboard() {
	a.imeShowCount++
}

func (a *mockLifecycleApp) HideSoftKeyboard() {
	a.imeHideCount++
}

func (a *mockLifecycleApp) firePause() {
	for _, f := range a.onPause {
		f()
	}
}

func (a *mockLifecycleApp) fireResume() {
	for _, f := range a.onResume {
		f()
	}
}

type runtimeIMEFacet struct {
	facet.Facet
	focus  facet.FocusRole
	text   facet.TextRole
	layout facet.LayoutRole
}

func (f *runtimeIMEFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func newRuntimeIMEFacet() *runtimeIMEFacet {
	f := &runtimeIMEFacet{Facet: facet.NewFacet()}
	f.focus.Focusable = func() bool { return true }
	f.text.CaretVisible = true
	f.text.IMEEnabled = true
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 8, H: 8}}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.AddRole(&f.focus)
	f.AddRole(&f.text)
	f.AddRole(&f.layout)
	return f
}

func (a *mockLifecycleApp) fireLowMemory() {
	for _, f := range a.onLowMemory {
		a.lowMemoryCount++
		f()
	}
}

func (a *mockLifecycleApp) fireSurfaceLost() {
	for _, f := range a.onSurfaceLost {
		a.surfaceLostCount++
		f()
	}
}

func (a *mockLifecycleApp) fireSurfaceCreated() {
	surface := &mockSurface{width: 16, height: 16}
	for _, f := range a.onSurfaceCreated {
		a.surfaceCreatedCount++
		f(surface)
	}
}

var _ platform.App = (*mockLifecycleApp)(nil)
var _ platform.LifecycleCapable = (*mockLifecycleApp)(nil)

type mockSurface struct {
	width  int
	height int
}

func (s *mockSurface) Size() (int, int) { return s.width, s.height }

func (s *mockSurface) Resize(width, height int) {
	if width > 0 {
		s.width = width
	}
	if height > 0 {
		s.height = height
	}
}

func (s *mockSurface) Scale() float32 { return 1 }

var _ platform.Surface = (*mockSurface)(nil)

type evictingBackend struct {
	recordingBackend
	evictCount int
}

func (b *evictingBackend) EvictCaches() {
	b.evictCount++
}

var _ render.Backend = (*evictingBackend)(nil)
var _ render.CacheEvictor = (*evictingBackend)(nil)

type mockAssetManager struct {
	assets.Manager
	drainCount int
}

func (m *mockAssetManager) DrainCompleted() int {
	m.drainCount++
	return 5
}

func TestRuntime_drainJobResults_drains_assetManager(t *testing.T) {
	rt := mustRuntime(t)
	mgr := &mockAssetManager{}
	rt.assetManager = mgr

	committed, discarded := rt.drainJobResults()
	if mgr.drainCount != 1 {
		t.Fatalf("expected DrainCompleted to be called once, got %d", mgr.drainCount)
	}
	if committed != 5 {
		t.Fatalf("expected committed count to include 5 from assets manager, got %d", committed)
	}
	if discarded != 0 {
		t.Fatalf("expected 0 discarded, got %d", discarded)
	}
}
