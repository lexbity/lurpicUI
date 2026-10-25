package runtime

import (
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
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
	completion := pipeline.SubmitAndWait(&render.Frame{}, 41)
	pipeline.Submit(&render.Frame{}, 42)
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
	pipeline.SubmitReserved(&render.Frame{}, 1)
	select {
	case <-backend.started:
	case <-time.After(5 * time.Second):
		t.Fatal("render worker did not start frame one")
	}
	pipeline.SubmitReserved(&render.Frame{}, 2)
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
	cfg := DefaultConfig()
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
