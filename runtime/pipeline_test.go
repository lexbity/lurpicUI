package runtime

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
)

type pipelineSurface struct {
	mu     sync.Mutex
	w, h   int
	stride int
	buffer []byte
}

func newPipelineSurface(w, h int) *pipelineSurface {
	stride := w * 4
	return &pipelineSurface{w: w, h: h, stride: stride, buffer: make([]byte, stride*h)}
}
func (s *pipelineSurface) Size() (int, int) { return s.w, s.h }
func (s *pipelineSurface) Resize(w, h int) {
	s.mu.Lock()
	s.w, s.h, s.stride = w, h, w*4
	s.buffer = make([]byte, s.stride*h)
	s.mu.Unlock()
}
func (s *pipelineSurface) Buffer() []byte          { return s.buffer }
func (s *pipelineSurface) Stride() int             { return s.stride }
func (s *pipelineSurface) Lock() error             { s.mu.Lock(); return nil }
func (s *pipelineSurface) Unlock([]gfx.Rect) error { s.mu.Unlock(); return nil }

func TestSoftwareSurfaceReadyAcknowledgment(t *testing.T) {
	surface := newPipelineSurface(8, 8)
	backend := software.NewSoftwareRenderer()
	if err := backend.Initialize(surface); err != nil {
		t.Fatal(err)
	}
	pipeline := newRenderPipeline(backend)
	renderDone := make(chan struct{})
	go func() {
		(&renderThread{pipeline: pipeline}).run()
		close(renderDone)
	}()
	completion := pipeline.SubmitAndWait(testframe.Group(testframe.Seal(render.FrameInput{})), 41)
	if !pipeline.TryReserveFrame() {
		t.Fatal("frame credit unavailable")
	}
	_ = pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 42)
	asyncCompletion := <-pipeline.Completions()
	pipeline.destroy()
	<-renderDone
	if completion.Frame != 41 || !completion.Ready || completion.Err != nil {
		t.Fatalf("completion = %+v, want ready software frame 41", completion)
	}
	if asyncCompletion.Frame != 42 || !asyncCompletion.Ready || asyncCompletion.Err != nil {
		t.Fatalf("async completion = %+v, want ready software frame 42", asyncCompletion)
	}
}

func TestRenderCompletionAcknowledgesReleasedGenerationOnFailure(t *testing.T) {
	failure := errors.New("backend rejected frame")
	backend := &fakeBackend{submitErr: failure}
	pipeline := newRenderPipeline(backend)
	pipeline.start()
	wallet, err := membudget.NewWallet(4096)
	if err != nil {
		t.Fatal(err)
	}
	input := render.FrameInput{}
	bytes, err := render.EstimateFrameBytes(input)
	if err != nil {
		t.Fatal(err)
	}
	reservation, err := wallet.Reserve(bytes + 1)
	if err != nil {
		t.Fatal(err)
	}
	frame, err := render.SealFrame(input, reservation)
	if err != nil {
		t.Fatal(err)
	}
	generation := frame.Generation()
	completion := pipeline.SubmitAndWait(testframe.Group(frame), 73)
	if completion.Frame != 73 || completion.ResourceGeneration != generation || completion.Ready || !errors.Is(completion.Err, failure) {
		t.Fatalf("completion=%+v, want failed acknowledgment for generation %d", completion, generation)
	}
	if frame.BatchCount() != 0 {
		t.Fatal("pipeline retained its transferred frame lease after failed submit")
	}
	_, reserved, live, retiring := wallet.Snapshot()
	if reserved != 0 || live != 0 || retiring != 0 {
		t.Fatalf("failed frame charge remained: reserved=%d live=%d retiring=%d", reserved, live, retiring)
	}
	if !pipeline.shutdown(time.Second) {
		t.Fatal("render worker did not stop")
	}
}

func TestRenderCreditWindowIsBoundedAndNonblocking(t *testing.T) {
	backend := &delayedCommandBackend{
		started: make(chan struct{}, 1),
		release: make(chan struct{}, 1),
	}
	backend.delay.Store(true)
	pipeline := newRenderPipeline(backend)
	done := make(chan struct{})
	go func() {
		(&renderThread{pipeline: pipeline}).run()
		close(done)
	}()
	t.Cleanup(func() {
		select {
		case backend.release <- struct{}{}:
		default:
		}
		pipeline.destroy()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
			t.Error("render worker did not stop")
		}
	})

	if !pipeline.TryReserveFrame() || !pipeline.TryReserveFrame() {
		t.Fatal("the two configured frame credits were not available")
	}
	if pipeline.TryReserveFrame() {
		pipeline.ReleaseFrameReservation()
		t.Fatal("a third frame reserved credit beyond the configured window")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 1)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("render worker did not start frame one")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 2)
	if pipeline.TryReserveFrame() {
		pipeline.ReleaseFrameReservation()
		t.Fatal("a frame reserved credit while both frame groups were outstanding")
	}
	backend.delay.Store(false)
	backend.release <- struct{}{}
	for want := uint64(1); want <= 2; want++ {
		select {
		case completion := <-pipeline.Completions():
			if completion.Frame != want || !completion.Ready || completion.Err != nil {
				t.Fatalf("completion = %+v, want ready frame %d", completion, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("frame %d did not complete", want)
		}
	}
	for i := 0; i < 2; i++ {
		if !pipeline.TryReserveFrame() {
			t.Fatalf("credit %d was not returned after completion", i+1)
		}
	}
	pipeline.ReleaseFrameReservation()
	pipeline.ReleaseFrameReservation()
}

func TestCleanupCreditSurvivesBlockedRenderer(t *testing.T) {
	backend := &delayedCommandBackend{started: make(chan struct{}, 1), release: make(chan struct{}, 1)}
	backend.delay.Store(true)
	pipeline := newRenderPipeline(backend)
	pipeline.start()
	t.Cleanup(func() {
		backend.delay.Store(false)
		select {
		case backend.release <- struct{}{}:
		default:
		}
		if !pipeline.shutdown(5 * time.Second) {
			t.Error("render worker did not stop")
		}
	})
	if !pipeline.TryReserveFrame() || !pipeline.TryReserveFrame() {
		t.Fatal("ordinary render groups were not available")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 1)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("renderer did not block on the first ordinary frame")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 2)
	if pipeline.TryReserveFrame() {
		pipeline.ReleaseFrameReservation()
		t.Fatal("ordinary credit became available while renderer was blocked")
	}
	if !pipeline.TryReserveCleanupFrame() {
		t.Fatal("reserved cleanup group was unavailable while ordinary groups were full")
	}
	pipeline.SubmitCleanupReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 3)
	backend.delay.Store(false)
	backend.release <- struct{}{}
	for want := uint64(1); want <= 3; want++ {
		select {
		case completion := <-pipeline.Completions():
			if completion.Frame != want || !completion.Ready {
				t.Fatalf("completion=%+v, want ready frame %d", completion, want)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("frame %d did not complete", want)
		}
	}
	if !pipeline.TryReserveFrame() || !pipeline.TryReserveFrame() || !pipeline.TryReserveCleanupFrame() {
		t.Fatal("completion did not return all owner-specific groups")
	}
	pipeline.ReleaseFrameReservation()
	pipeline.ReleaseFrameReservation()
	pipeline.ReleaseCleanupFrameReservation()
}

type lifecycleOrderingBackend struct {
	started     chan struct{}
	release     chan struct{}
	active      atomic.Int32
	destroyed   atomic.Int32
	recreated   atomic.Int32
	initialized atomic.Int32
	concurrent  atomic.Bool
}

func (b *lifecycleOrderingBackend) Initialize(render.Surface) error {
	if b.active.Load() != 0 {
		b.concurrent.Store(true)
	}
	b.initialized.Add(1)
	return nil
}
func (b *lifecycleOrderingBackend) Submit(*render.Frame) error {
	b.active.Add(1)
	defer b.active.Add(-1)
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
	return nil
}
func (b *lifecycleOrderingBackend) Resize(int, int) error { return nil }
func (b *lifecycleOrderingBackend) Destroy() {
	if b.active.Load() != 0 {
		b.concurrent.Store(true)
	}
	b.destroyed.Add(1)
}
func (b *lifecycleOrderingBackend) Recreate(render.Surface) error {
	if b.active.Load() != 0 {
		b.concurrent.Store(true)
	}
	b.recreated.Add(1)
	return nil
}

func TestRenderLifecycleOperationsAreOrderedWithFrames(t *testing.T) {
	backend := &lifecycleOrderingBackend{started: make(chan struct{}, 1), release: make(chan struct{}, 1)}
	pipeline := newRenderPipeline(backend)
	pipeline.start()
	t.Cleanup(func() {
		backend.release <- struct{}{}
		if !pipeline.shutdown(5 * time.Second) {
			t.Error("render worker did not stop")
		}
	})
	if !pipeline.TryReserveFrame() {
		t.Fatal("ordinary frame reservation unavailable")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 17)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("render submission did not start")
	}
	if err := pipeline.QueueSurfaceLost(8); err != nil {
		t.Fatalf("queue surface loss: %v", err)
	}
	if err := pipeline.QueueSurfaceCreated(&pipelineSurface{w: 8, h: 8}, 9, nil); err != nil {
		t.Fatalf("queue surface creation: %v", err)
	}
	if backend.destroyed.Load() != 0 || backend.recreated.Load() != 0 {
		t.Fatal("runtime control called backend lifecycle while a frame was active")
	}
	select {
	case completion := <-pipeline.Completions():
		t.Fatalf("frame/lifecycle completion published before blocked Submit returned: %+v", completion)
	default:
	}
	backend.release <- struct{}{}
	for want := 0; want < 3; want++ {
		select {
		case completion := <-pipeline.Completions():
			if want == 0 && (completion.Frame != 17 || completion.Lifecycle != 0) {
				t.Fatalf("completion 0 = %+v, want preceding frame", completion)
			}
			if want == 1 && (completion.Lifecycle != renderLifecycleSurfaceLost || completion.LifecycleGeneration != 8) {
				t.Fatalf("completion 1 = %+v, want surface loss generation 8", completion)
			}
			if want == 2 && (completion.Lifecycle != renderLifecycleSurfaceCreated || completion.LifecycleGeneration != 9 || !completion.Ready) {
				t.Fatalf("completion 2 = %+v, want ready surface generation 9", completion)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("completion %d did not arrive", want)
		}
	}
	if backend.concurrent.Load() || backend.destroyed.Load() != 1 || backend.recreated.Load() != 0 || backend.initialized.Load() != 1 {
		t.Fatalf("lifecycle calls overlapped submission or counts were wrong: concurrent=%v destroyed=%d recreated=%d", backend.concurrent.Load(), backend.destroyed.Load(), backend.recreated.Load())
	}
}

type saturatedLifecycleBackend struct {
	fakeBackend
	started chan struct{}
	release chan struct{}
}

func (b *saturatedLifecycleBackend) EvictCaches() {
	select {
	case b.started <- struct{}{}:
	default:
	}
	<-b.release
}

func TestRenderPipelineBuffersMaximumOutstandingAcknowledgments(t *testing.T) {
	backend := &saturatedLifecycleBackend{started: make(chan struct{}, 1), release: make(chan struct{})}
	pipeline := newRenderPipeline(backend)
	pipeline.start()
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(backend.release) }) }
	t.Cleanup(func() {
		unblock()
		if !pipeline.shutdown(5 * time.Second) {
			t.Error("render owner did not stop after saturation test cleanup")
		}
	})

	if err := pipeline.QueueCacheEviction(1); err != nil {
		t.Fatal(err)
	}
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("render owner did not begin the first lifecycle operation")
	}
	// The first request is executing; fill all queued lifecycle slots.
	for i := 0; i < maxRenderLifecycleRequests; i++ {
		if err := pipeline.QueueCacheEviction(uint64(i + 2)); err != nil {
			t.Fatalf("queue lifecycle request %d: %v", i+2, err)
		}
	}
	if !pipeline.TryReserveFrame() || !pipeline.TryReserveFrame() {
		t.Fatal("ordinary frame reservations unavailable")
	}
	if !pipeline.TryReserveCleanupFrame() {
		t.Fatal("cleanup frame reservation unavailable")
	}
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 1)
	pipeline.SubmitReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 2)
	pipeline.SubmitCleanupReserved(testframe.Group(testframe.Seal(render.FrameInput{})), 3)

	unblock()
	if !pipeline.shutdown(5 * time.Second) {
		t.Fatal("render owner stalled while publishing the maximum outstanding acknowledgments")
	}

	want := maxRenderLifecycleRequests + 1 + maxOutstandingFrames
	if got := len(pipeline.Completions()); got != want {
		t.Fatalf("buffered acknowledgments=%d, want %d", got, want)
	}
	for i := 0; i < want; i++ {
		<-pipeline.Completions()
	}
}

func TestReservedFrameRejectedAfterShutdownReleasesStorage(t *testing.T) {
	pipeline := newRenderPipeline(&fakeBackend{})
	if !pipeline.TryReserveFrame() {
		t.Fatal("frame credit unavailable")
	}
	frame := testframe.Seal(render.FrameInput{})
	group := testframe.Group(frame)
	if !pipeline.shutdown(time.Second) {
		t.Fatal("pipeline did not stop")
	}
	if err := pipeline.SubmitReserved(group, 1); err == nil {
		t.Fatal("stopped pipeline accepted a reserved frame")
	}
	if frame.Generation() != 0 || group.WindowCount() != 0 {
		t.Fatal("rejected handoff retained its transferred storage")
	}
	if pipeline.TryReserveFrame() {
		t.Fatal("stopped pipeline admitted more work")
	}
}

func TestRenderPipeline_destroy_is_idempotent(t *testing.T) {
	fb := &fakeBackend{}
	p := newRenderPipeline(fb)

	p.destroy()
	if fb.destroyCount.Load() != 0 {
		t.Fatalf("backend Destroy called on pipeline destroy: count=%d", fb.destroyCount.Load())
	}

	p.destroy()
	if fb.destroyCount.Load() != 0 {
		t.Fatalf("backend Destroy called on double pipeline destroy: count=%d", fb.destroyCount.Load())
	}
}

func TestRenderPipeline_destroy_backend_destroyed_on_shutdown(t *testing.T) {
	fb := &fakeBackend{}
	root := &runtimeTestFacet{Facet: facet.NewFacet()}
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, fb, root)
	if err != nil {
		t.Fatalf("New: %v", err)
	}

	rt.Shutdown()

	if fb.destroyCount.Load() != 1 {
		t.Fatalf("backend Destroy called %d times, want 1", fb.destroyCount.Load())
	}
}
