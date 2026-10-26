package runtime

import (
	"image/color"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

type frameDemandRecordingApp struct {
	queue  *queue
	demand chan bool
}

func (a *frameDemandRecordingApp) Events() platform.EventQueue { return a.queue }
func (a *frameDemandRecordingApp) SetFrameDemand(demand bool) {
	select {
	case a.demand <- demand:
	default:
	}
}
func (a *frameDemandRecordingApp) Destroy() {}

func TestWallRuntimeParksAfterBootstrapUntilFrameRequest(t *testing.T) {
	rt := mustRuntime(t)
	backend := rt.renderPipeline.backend.(*backendFixture)
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	deadline := time.Now().Add(time.Second)
	for backend.submitCount.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if backend.submitCount.Load() < 2 {
		t.Fatalf("bootstrap and dirty follow-up frames submitted = %d, want at least 2", backend.submitCount.Load())
	}
	settled := backend.submitCount.Load()
	time.Sleep(75 * time.Millisecond)
	if got := backend.submitCount.Load(); got != settled {
		t.Fatalf("idle runtime submissions changed from %d to %d", settled, got)
	}

	rt.RequestFrame()
	deadline = time.Now().Add(time.Second)
	for backend.submitCount.Load() <= settled && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if got := backend.submitCount.Load(); got <= settled {
		t.Fatalf("explicit frame request did not wake idle runtime: baseline=%d current=%d", settled, got)
	}
	rt.Shutdown()
	select {
	case err := <-done:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop after shutdown")
	}
}

func TestWallPlatformEventWakesParkedRuntimeAndRestoresFrameDemand(t *testing.T) {
	rt := mustRuntime(t)
	backend := rt.renderPipeline.backend.(*backendFixture)
	queue := &queue{wake: make(chan struct{}, 1)}
	app := &frameDemandRecordingApp{queue: queue, demand: make(chan bool, 8)}
	rt.platformApp = app
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	waitSubmissions := func(target int32) bool {
		deadline := time.Now().Add(time.Second)
		for backend.submitCount.Load() < target && time.Now().Before(deadline) {
			time.Sleep(time.Millisecond)
		}
		return backend.submitCount.Load() >= target
	}
	if !waitSubmissions(2) {
		t.Fatalf("bootstrap submissions = %d, want at least 2", backend.submitCount.Load())
	}
	settled := backend.submitCount.Load()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case demand := <-app.demand:
			if !demand {
				goto idle
			}
		default:
			time.Sleep(time.Millisecond)
		}
	}
	t.Fatal("runtime did not disable frame demand after settling")

idle:
	queue.Push(platform.EventWindowResize{Width: 640, Height: 480})
	if !waitSubmissions(settled + 1) {
		t.Fatalf("platform event did not wake runtime: baseline=%d current=%d", settled, backend.submitCount.Load())
	}
	seenActive := false
	deadline = time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		select {
		case demand := <-app.demand:
			if demand {
				seenActive = true
			} else if seenActive {
				rt.Shutdown()
				select {
				case <-done:
					return
				case <-time.After(2 * time.Second):
					t.Fatal("runtime did not stop")
				}
			}
		default:
			time.Sleep(time.Millisecond)
		}
	}
	rt.Shutdown()
	t.Fatal("runtime did not restore idle frame demand after platform event")
}

func TestWallFacetInvalidationSchedulesFrameFromRuntimeCallback(t *testing.T) {
	root := newRuntimeRenderFacet("invalidate", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	rt := mustRuntimeTree(t, root)
	backend := rt.renderPipeline.backend.(*backendFixture)
	queue := &queue{wake: make(chan struct{}, 1)}
	app := &frameDemandRecordingApp{queue: queue, demand: make(chan bool, 8)}
	rt.platformApp = app
	var invalidate atomic.Bool
	var collects atomic.Int32
	collect := root.render.OnCollect
	root.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		collects.Add(1)
		collect(list, bounds)
	}
	release := rt.RegisterPhase1TickHook(func(time.Duration) {
		if invalidate.Swap(false) {
			root.Invalidate(facet.DirtyProjection)
		}
	})
	defer release()
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	deadline := time.Now().Add(time.Second)
	for backend.submitCount.Load() < 2 && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if backend.submitCount.Load() < 2 {
		t.Fatalf("bootstrap submissions = %d, want at least 2", backend.submitCount.Load())
	}
	baseline := backend.submitCount.Load()
	collectBaseline := collects.Load()
	invalidate.Store(true)
	rt.frameTimer.RequestFrame()
	deadline = time.Now().Add(time.Second)
	for backend.submitCount.Load() <= baseline && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if backend.submitCount.Load() <= baseline {
		t.Fatal("runtime-owned facet invalidation did not schedule a Wall frame")
	}
	if got := collects.Load(); got <= collectBaseline {
		t.Fatal("facet invalidation did not reproject on the runtime owner")
	}
	rt.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop")
	}
}

func TestPendingHoverDeadlineWakesIdle(t *testing.T) {
	root := newRuntimeRenderFacet("hover-deadline", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	hit := facet.HitRole{OnHitTest: func(gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, MarkID: 19}
	}}
	root.Base().AddRole(&hit)
	scope := store.NewScope()
	cfg := testConfigForScope(t, scope)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.GestureConfig.HoverDelay = 40 * time.Millisecond
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, &backendFixture{}, root)
	if err != nil {
		scope.Close()
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(rt.Shutdown)
	rt.inputSystem.SetHoverSupported(true)
	backend := rt.renderPipeline.backend.(*backendFixture)
	queue := &queue{wake: make(chan struct{}, 1)}
	rt.platformApp = &frameDemandRecordingApp{queue: queue, demand: make(chan bool, 8)}
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	if !waitForSubmissions(backend, 2, time.Second) {
		t.Fatalf("bootstrap submissions = %d, want at least 2", backend.submitCount.Load())
	}
	queue.Push(platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 20, Y: 20},
		Source:   platform.SourcePlatform,
	})
	if !waitForSubmissions(backend, backend.submitCount.Load()+1, time.Second) {
		t.Fatal("pointer event did not produce its authorized frame")
	}
	baseline := backend.submitCount.Load()
	if !waitForSubmissions(backend, baseline+1, time.Second) {
		t.Fatal("pending hover deadline did not wake an idle Wall runtime")
	}
	rt.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop")
	}
}

func TestReadyWallJobRequestsFrame(t *testing.T) {
	root := newRuntimeRenderFacet("wall-job", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	var resultCalls atomic.Int32
	projectionRole := facet.ProjectionRole{
		OnProject:   func(facet.ProjectionContext) *gfx.CommandList { return nil },
		OnJobResult: func(job.AnyResult) { resultCalls.Add(1) },
	}
	root.Base().AddRole(&projectionRole)
	rt := mustRuntimeTree(t, root)
	backend := rt.renderPipeline.backend.(*backendFixture)
	queue := &queue{wake: make(chan struct{}, 1)}
	rt.platformApp = &frameDemandRecordingApp{queue: queue, demand: make(chan bool, 8)}
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	if !waitForSubmissions(backend, 2, time.Second) {
		t.Fatalf("bootstrap submissions = %d, want at least 2", backend.submitCount.Load())
	}
	workRelease := make(chan struct{})
	jobID := job.JobID(91001)
	work := job.Job[int, int]{
		ID:       jobID,
		Snapshot: job.NewSnapshot(1),
		Work: func(snapshot job.Snapshot[int], _ *job.CancelToken) (int, error) {
			<-workRelease
			return snapshot.Data + 1, nil
		},
	}
	rt.Schedule(job.BindJob(uint64(root.Base().ID()), work, func(int) {}))
	baseline := backend.submitCount.Load()
	close(workRelease)
	if !waitForAtomic(&resultCalls, 1, time.Second) {
		t.Fatal("ready Wall-mode job result did not commit")
	}
	if !waitForSubmissions(backend, baseline+1, time.Second) {
		t.Fatal("ready Wall-mode job did not request its normal frame")
	}
	rt.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop")
	}
}

func TestIdleVirtualJobDoesNotAdvance(t *testing.T) {
	root := newRuntimeRenderFacet("virtual-job", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	var resultCalls atomic.Int32
	projectionRole := facet.ProjectionRole{
		OnProject:   func(facet.ProjectionContext) *gfx.CommandList { return nil },
		OnJobResult: func(job.AnyResult) { resultCalls.Add(1) },
	}
	root.Base().AddRole(&projectionRole)
	scope := store.NewScope()
	cfg := testConfigForScope(t, scope)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.Clock = ClockConfig{InitialMode: ClockVirtual, EpochNS: 1_800_000_000_000_000_000}
	backend := &backendFixture{}
	rt, err := New(cfg, nil, &testWindow{width: 800, height: 600}, backend, root)
	if err != nil {
		scope.Close()
		t.Fatalf("new runtime: %v", err)
	}
	t.Cleanup(rt.Shutdown)
	done := make(chan error, 1)
	go func() { done <- rt.Run() }()
	if !waitForSubmissions(backend, 1, time.Second) {
		t.Fatal("Virtual bootstrap frame did not submit")
	}
	baseline := backend.submitCount.Load()
	workRelease := make(chan struct{})
	work := job.Job[int, int]{
		ID:       91002,
		Snapshot: job.NewSnapshot(1),
		Work: func(snapshot job.Snapshot[int], _ *job.CancelToken) (int, error) {
			<-workRelease
			return snapshot.Data + 1, nil
		},
	}
	rt.Schedule(job.BindJob(uint64(root.Base().ID()), work, func(int) {}))
	close(workRelease)
	frontier := rt.jobPool.Frontier()
	deadline := time.Now().Add(time.Second)
	for !rt.jobPool.ReadyThrough(frontier) && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if !rt.jobPool.ReadyThrough(frontier) {
		t.Fatal("Virtual job did not reach the worker frontier")
	}
	time.Sleep(40 * time.Millisecond)
	if got := backend.submitCount.Load(); got != baseline {
		t.Fatalf("job completion invented a Virtual frame: submissions=%d, want %d", got, baseline)
	}
	if got := resultCalls.Load(); got != 0 {
		t.Fatalf("Virtual job committed without an authorized frame: calls=%d", got)
	}
	rt.Shutdown()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("runtime did not stop")
	}
}

func waitForSubmissions(backend *backendFixture, target int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for backend.submitCount.Load() < target && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	return backend.submitCount.Load() >= target
}

func waitForAtomic(value *atomic.Int32, target int32, timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for value.Load() < target && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	return value.Load() >= target
}
