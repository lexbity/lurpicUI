package app

import (
	goruntime "runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/runtime"
)

// raceIterations is how many Run/Shutdown lifecycles each race-test run
// exercises. Each lifecycle deterministically overlaps disposeTree with an
// in-flight frame (see raceTickFacet), so the race detector reliably fires if
// frameMu regresses. The -count=100 outer loop (NFR-1) multiplies this.
const raceIterations = 10

// raceReadWindow is how long the race tree's tick keeps reading facet state,
// guaranteeing a concurrent disposeTree write overlaps a tree read (§2.3).
const raceReadWindow = 10 * time.Millisecond

// raceTickFacet blocks the runtime thread inside its tick callback while
// reading the facet tree, so a concurrent disposeTree is guaranteed to overlap
// a frame's tree read. Without the frameMu write lock in disposeTree this is a
// write-vs-read data race on facet.Facet.state; with it, disposal waits for
// the in-flight frame.
type raceTickFacet struct {
	facet.Facet
	tick    facet.TickRole
	entered chan struct{}
	reads   atomic.Int32
}

func (f *raceTickFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *raceTickFacet) OnAttach(ctx facet.AttachContext) {}
func (f *raceTickFacet) OnDetach()                        {}
func (f *raceTickFacet) OnActivate()                      {}
func (f *raceTickFacet) OnDeactivate()                    {}

// raceChildFacet projects a command, contributing real projection work so the
// frame pass reads facet state beyond the tick.
type raceChildFacet struct {
	facet.Facet
	proj facet.ProjectionRole
}

func (f *raceChildFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *raceChildFacet) OnAttach(ctx facet.AttachContext) {}
func (f *raceChildFacet) OnDetach()                        {}
func (f *raceChildFacet) OnActivate()                      {}
func (f *raceChildFacet) OnDeactivate()                    {}

func newRaceChild(index int) *raceChildFacet {
	f := &raceChildFacet{Facet: facet.NewFacet()}
	f.proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		list := &gfx.CommandList{}
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(float32(index*20), 0, 20, 20),
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
		})
		return list
	}
	f.AddRole(&f.proj)
	return f
}

// newRaceTree builds a root whose tick reads facet state for a bounded window
// (signalling entered first), with projection children for realistic frame
// work.
func newRaceTree(entered chan struct{}) *raceTickFacet {
	root := &raceTickFacet{Facet: facet.NewFacet(), entered: entered}
	root.tick.OnTick = func(dt time.Duration) {
		root.reads.Add(1)
		_ = root.Base().Children()
		select {
		case root.entered <- struct{}{}:
		default:
		}
		// Read facet state continuously so a concurrent disposeTree write is
		// guaranteed to overlap a tree read.
		deadline := time.Now().Add(raceReadWindow)
		for time.Now().Before(deadline) {
			_ = root.Base().State()
			goruntime.Gosched()
		}
	}
	root.tick.RequestTick()
	root.AddRole(&root.tick)
	for i := 0; i < 4; i++ {
		root.AddChild(&newRaceChild(i).Facet)
	}
	return root
}

func setupRaceAppHooks() {
	newPlatformApp = func() (platform.App, error) { return &fakeApp{}, nil }
	newBackend = func(RenderBackendKind) render.Backend { return &fakeBackend{} }
	primeRuntime = func(*runtime.Runtime) {}
	initAssetManager = func(*runtime.Config) {}
}

// TestApp_Run_ShutdownRace_NoDataRace drives the real app.Run →
// runRuntime → rt.Run + defer rt.Shutdown path (the one real applications
// take) with Shutdown fired while a frame is mid-tick. Under -race this must
// be clean: frameMu serializes the in-flight frame against disposeTree.
func TestApp_Run_ShutdownRace_NoDataRace(t *testing.T) {
	restoreHooks(t)
	setupRaceAppHooks()

	for i := 0; i < raceIterations; i++ {
		runOneAppLifecycle(t)
	}
}

func runOneAppLifecycle(t *testing.T) {
	t.Helper()
	entered := make(chan struct{}, 1)
	var rtCh = make(chan *runtime.Runtime, 1)
	runRuntime = func(rt *runtime.Runtime) error {
		rtCh <- rt
		defer rt.Shutdown()
		return rt.Run()
	}

	var tree *raceTickFacet
	done := make(chan error, 1)
	cfg := DefaultConfig("race", 320, 240)
	cfg.Render = RenderBackendSoftware
	go func() {
		done <- Run(cfg, func(BuildContext) facet.FacetImpl {
			tree = newRaceTree(entered)
			return tree
		})
	}()

	var rt *runtime.Runtime
	select {
	case rt = <-rtCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runtime not started")
	}
	select {
	case <-entered:
	case <-time.After(10 * time.Second):
		t.Fatal("frame did not enter the tick callback")
	}
	if tree.reads.Load() == 0 {
		t.Fatal("tree tick never read facet state")
	}

	rt.Shutdown()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
}

// poisonedFrameHook records the maximum PoisonedFacets observed across frames.
type poisonedFrameHook struct {
	mu    sync.Mutex
	count int
}

func (h *poisonedFrameHook) OnFrame(stats diagnostics.FrameStats) {
	h.mu.Lock()
	if stats.PoisonedFacets > h.count {
		h.count = stats.PoisonedFacets
	}
	h.mu.Unlock()
}

func (h *poisonedFrameHook) poisonedCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.count
}

// appPanicProjectionFacet panics in OnProject; the runtime's projection
// recovery (Slice 3) quarantines it instead of terminating the app.
type appPanicProjectionFacet struct {
	facet.Facet
	proj facet.ProjectionRole
}

func (f *appPanicProjectionFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *appPanicProjectionFacet) OnAttach(ctx facet.AttachContext) {}
func (f *appPanicProjectionFacet) OnDetach()                        {}
func (f *appPanicProjectionFacet) OnActivate()                      {}
func (f *appPanicProjectionFacet) OnDeactivate()                    {}

func newPanicProjectionTree() facet.FacetImpl {
	f := &appPanicProjectionFacet{Facet: facet.NewFacet()}
	f.proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList { panic("boom") }
	f.AddRole(&f.proj)
	return f
}

// TestApp_PoisonedFacet_DoesNotCrashRun asserts the end-to-end recovery at the
// app layer: a facet whose OnProject panics quarantines itself, surfaces
// through FrameStats.PoisonedFacets, and the app shuts down cleanly.
func TestApp_PoisonedFacet_DoesNotCrashRun(t *testing.T) {
	restoreHooks(t)
	setupRaceAppHooks()

	hook := &poisonedFrameHook{}
	var rtCh = make(chan *runtime.Runtime, 1)
	runRuntime = func(rt *runtime.Runtime) error {
		rtCh <- rt
		defer rt.Shutdown()
		return rt.Run()
	}

	done := make(chan error, 1)
	cfg := DefaultConfig("poison", 320, 240)
	cfg.Render = RenderBackendSoftware
	cfg.Runtime.DiagnosticsHook = hook
	go func() {
		done <- Run(cfg, func(BuildContext) facet.FacetImpl {
			return newPanicProjectionTree()
		})
	}()

	var rt *runtime.Runtime
	select {
	case rt = <-rtCh:
	case <-time.After(10 * time.Second):
		t.Fatal("runtime not started")
	}

	deadline := time.Now().Add(10 * time.Second)
	for hook.poisonedCount() == 0 && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	if hook.poisonedCount() == 0 {
		t.Fatal("FrameStats.PoisonedFacets never reached 1")
	}
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d, want 1", got)
	}

	rt.Shutdown()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("Run returned error: %v", err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("Run did not return after Shutdown")
	}
}
