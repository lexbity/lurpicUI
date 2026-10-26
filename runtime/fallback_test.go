package runtime

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testframe"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
)

// swSurface is an in-memory render.SoftwareSurface used to bind the software
// fallback backend in tests. The render thread writes the buffer between
// Lock/Unlock; the test reads after SubmitAndWait (which synchronizes), so no
// mutex is needed.
type swSurface struct {
	width     int
	height    int
	buf       []byte
	blitCount int
}

func (s *swSurface) Size() (int, int) { return s.width, s.height }
func (s *swSurface) Resize(width, height int) {
	s.width = width
	s.height = height
	s.buf = make([]byte, width*height*4)
}
func (s *swSurface) Buffer() []byte { return s.buf }
func (s *swSurface) Stride() int    { return s.width * 4 }
func (s *swSurface) Lock() error    { return nil }
func (s *swSurface) Unlock([]gfx.Rect) error {
	s.blitCount++
	return nil
}

func (s *swSurface) blits() int { return s.blitCount }

// fatalOnceBackend is a GPU stand-in whose Submit always fails with
// *render.ErrGPUFatal, driving the render thread's fallback swap.
type fatalOnceBackend struct {
	submits   atomic.Int32
	destroyed atomic.Bool
}

func (b *fatalOnceBackend) Initialize(render.Surface) error { return nil }
func (b *fatalOnceBackend) Submit(*render.Frame) error {
	b.submits.Add(1)
	return &render.ErrGPUFatal{Err: errors.New("test device lost")}
}
func (b *fatalOnceBackend) Resize(int, int) error { return nil }
func (b *fatalOnceBackend) Destroy() {
	b.destroyed.Store(true)
}

var _ render.Backend = (*fatalOnceBackend)(nil)

// TestRenderPipeline_GPUFatalFallsBackToSoftware is the Slice 10 fallback test
// (FR-12/Q9): the render thread catches a *render.ErrGPUFatal from Submit,
// destroys the GPU backend, swaps in a software backend bound to the platform
// surface, and continues rendering on software — with the OnBackendFallback
// diagnostic emitted. The frame in flight is dropped; the next frame renders on
// software.
func TestRenderPipeline_GPUFatalFallsBackToSoftware(t *testing.T) {
	surface := &swSurface{}
	surface.Resize(64, 64)

	gpu := &fatalOnceBackend{}
	p := newRenderPipeline(gpu)
	p.fallbackFactory = func(render.Surface) (render.Backend, error) {
		r := software.NewSoftwareRenderer()
		if err := r.Initialize(surface); err != nil {
			return nil, err
		}
		return r, nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		(&renderThread{pipeline: p}).run()
	}()

	frame := testframe.Seal(render.FrameInput{Batches: []render.BatchInput{{
		ID:      1,
		Bounds:  gfx.RectFromXYWH(0, 0, 64, 64),
		Opacity: 1,
		Commands: gfx.CommandList{Commands: []gfx.Command{
			gfx.FillRect{Rect: gfx.RectFromXYWH(4, 4, 40, 40), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(30, 60, 200, 255))},
		}},
	}}})

	// Frame 1 triggers the GPU-fatal swap (the frame itself is dropped).
	first, err := frame.Retain()
	if err != nil {
		t.Fatal(err)
	}
	completion := p.SubmitAndWait(testframe.Group(first), 1)
	if !gpu.destroyed.Load() {
		t.Fatal("GPU backend must be destroyed on ErrGPUFatal")
	}
	if completion.FallbackFrom == "" || completion.FallbackReason == "" {
		t.Fatal("fallback completion notice was missing")
	}

	// Frame 2 renders on the software backend: the surface receives a blit.
	before := surface.blits()
	p.SubmitAndWait(testframe.Group(frame), 2)
	if surface.blits() <= before {
		t.Fatal("software backend must render frames after the fallback")
	}
	pixel := surface.Buffer()[8*surface.Stride()+8*4:]
	if pixel[0] != 30 || pixel[1] != 60 || pixel[2] != 200 || pixel[3] != 255 {
		t.Fatalf("fallback did not draw the held immutable packet: %v", pixel[:4])
	}

	p.destroy()
	<-done
}

// TestRenderPipeline_GPUFatalDoesNotFatalShutdown verifies a non-GPU-fatal
// submit error still surfaces on fatalCh (shutdown) rather than triggering the
// fallback swap.
func TestRenderPipeline_GPUFatalDoesNotFatalShutdown(t *testing.T) {
	surface := &swSurface{}
	surface.Resize(64, 64)

	gpu := &fakeBackend{submitErr: errors.New("packet parse error")}
	p := newRenderPipeline(gpu)
	p.fallbackFactory = func(render.Surface) (render.Backend, error) {
		return software.NewSoftwareRenderer(), nil
	}

	done := make(chan struct{})
	go func() {
		defer close(done)
		(&renderThread{pipeline: p}).run()
	}()

	frame := testframe.Seal(render.FrameInput{})
	p.SubmitAndWait(testframe.Group(frame), 3)

	select {
	case err := <-p.fatalCh:
		if err == nil {
			t.Fatal("expected a non-nil fatal error")
		}
	default:
		t.Fatal("a non-GPU-fatal submit error must surface on fatalCh")
	}
	// The GPU backend was not destroyed (no fallback swap).
	if gpu.destroyCount.Load() != 0 {
		t.Fatalf("GPU backend must not be destroyed for a non-fatal submit error, destroyCount=%d", gpu.destroyCount.Load())
	}

	p.destroy()
	<-done
}

// backendFallbackRecordingHook records OnBackendFallback events; used to verify
// the runtime emits the diagnostic through a structurally-opted-in hook.
type backendFallbackRecordingHook struct {
	mu     sync.Mutex
	events []diagnostics.BackendFallback
}

func (h *backendFallbackRecordingHook) OnFrame(diagnostics.FrameStats) {}
func (h *backendFallbackRecordingHook) OnBackendFallback(ev diagnostics.BackendFallback) {
	h.mu.Lock()
	h.events = append(h.events, ev)
	h.mu.Unlock()
}

func (h *backendFallbackRecordingHook) last() (diagnostics.BackendFallback, bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.events) == 0 {
		return diagnostics.BackendFallback{}, false
	}
	return h.events[len(h.events)-1], true
}

// TestRuntime_WiresFallbackAndEmitsDiagnostic verifies the Runtime constructor
// wires the render thread's fallback (Q9) and that notifyBackendFallback emits
// OnBackendFallback to a DiagnosticsHook that structurally opts in.
func TestRuntime_WiresFallbackAndEmitsDiagnostic(t *testing.T) {
	root := facet.NewFacet()
	root.BindImpl(&root)
	hook := &backendFallbackRecordingHook{}
	cfg := testConfig(t)
	cfg.LayerRegistry = testLayerRegistry(t)
	cfg.DiagnosticsHook = hook
	cfg.SoftwareBackendFactory = func(surface render.Surface) (render.Backend, error) {
		backend := software.NewSoftwareRenderer()
		if err := backend.Initialize(surface); err != nil {
			return nil, err
		}
		return backend, nil
	}
	rt, err := New(cfg, nil, nil, &fakeBackend{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if rt.renderPipeline.fallbackFactory == nil {
		t.Fatal("Runtime must wire the software fallback factory (Q9)")
	}
	defer rt.Shutdown()
	group, err := rt.sealFrameGroup(map[string]render.FrameInput{render.PrimaryWindow: {}}, false)
	if err != nil {
		t.Fatal(err)
	}
	rt.pendingFrameViews[1] = group
	notice := RenderCompletion{Frame: 1, ResourceGeneration: group.Generation(), Err: errors.New("device lost"), FallbackFrom: "*vulkan.Backend", FallbackReason: "device lost"}
	rt.driveHandleRenderCompletion(notice)
	rt.driveHandleRenderCompletion(notice)
	if len(hook.events) != 1 {
		t.Fatal("fallback notice was not applied exactly once at the runtime boundary")
	}
	ev, ok := hook.last()
	if !ok {
		t.Fatal("OnBackendFallback diagnostic must be emitted")
	}
	if ev.From != "*vulkan.Backend" || ev.To != "software" || ev.Reason != "device lost" {
		t.Fatalf("fallback event mismatch: %+v", ev)
	}
}
