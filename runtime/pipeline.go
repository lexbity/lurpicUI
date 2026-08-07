package runtime

import (
	"errors"
	"fmt"
	"sync"

	"codeburg.org/lexbit/lurpicui/render"
)

type frameHandoff struct {
	frame  *render.Frame
	doneCh chan struct{}
}

// RenderPipeline hands frames to the render backend with capacity-1 backpressure.
//
// Slice 10 (Q9/FR-12): on a *render.ErrGPUFatal from Submit, the render thread
// destroys the GPU backend and swaps in a software backend bound to the current
// platform surface for the rest of the session (one-shot, no auto-retry). The
// `mu` guards `backend`/`uploadQueue` against the runtime thread's lifecycle
// reads (device generation, surface lost/created, eviction).
type RenderPipeline struct {
	mu          sync.RWMutex
	backend     render.Backend
	uploadQueue *render.UploadQueue
	handoffCh   chan frameHandoff
	fatalCh     chan error
	destroyOnce sync.Once

	// fromKind names the initially-selected backend for the fallback diagnostic
	// (e.g. "*vulkan.Backend").
	fromKind string
	// fallbackFactory lazily builds a software backend bound to the current
	// platform surface. Set by the runtime constructor; nil disables fallback.
	fallbackFactory func() (render.Backend, error)
	// onFallback is invoked on the render thread after a successful GPU→software
	// swap (may be nil). The runtime uses it to log, emit the OnBackendFallback
	// diagnostic (config.go's backendFallbackSink), and re-wire the asset
	// texture backend.
	onFallback func(from, reason string)
}

type renderThread struct {
	pipeline *RenderPipeline
}

func newRenderPipeline(backend render.Backend) *RenderPipeline {
	p := &RenderPipeline{
		backend:   backend,
		handoffCh: make(chan frameHandoff, 1),
		fatalCh:   make(chan error, 1),
		fromKind:  backendKindName(backend),
	}
	if tb, ok := backend.(render.TextureBackend); ok {
		p.uploadQueue = render.NewUploadQueue(tb, 1024)
	}
	return p
}

// backendKindName names a backend for the fallback diagnostic without importing
// the concrete packages: the runtime sees only the type name.
func backendKindName(b render.Backend) string {
	if b == nil {
		return "unknown"
	}
	return fmt.Sprintf("%T", b)
}

// Backend returns the current backend (nil when none / mid-swap). Locked read
// for the runtime thread; the render thread's swap serializes against it.
func (p *RenderPipeline) Backend() render.Backend {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.backend
}

// UploadQueue exposes the pipeline's upload queue for wiring into the asset
// manager's uploader bridge. Returns nil when the backend does not support
// textures or the pipeline has not been fully initialised.
func (p *RenderPipeline) UploadQueue() *render.UploadQueue {
	if p == nil {
		return nil
	}
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.uploadQueue
}

// Submit sends a frame to the render backend pipeline.
func (p *RenderPipeline) Submit(frame *render.Frame) {
	if p == nil || p.Backend() == nil {
		return
	}
	p.handoffCh <- frameHandoff{frame: frame}
}

// SubmitAndWait hands a frame to the renderer and waits for completion.
func (p *RenderPipeline) SubmitAndWait(frame *render.Frame) {
	if p == nil || p.Backend() == nil {
		return
	}
	done := make(chan struct{})
	p.handoffCh <- frameHandoff{frame: frame, doneCh: done}
	<-done
}

// swapToSoftwareFallback tears down the failing GPU backend and installs a
// software backend (Q9): one-shot, no auto-retry. Called from the render thread
// after an ErrGPUFatal; the frame in flight is dropped (one dropped frame is
// acceptable). If the software backend cannot be built, the failure surfaces on
// fatalCh (the runtime shuts down) rather than silently stalling.
func (p *RenderPipeline) swapToSoftwareFallback(reason error) {
	p.mu.Lock()
	old := p.backend
	if old == nil || p.fallbackFactory == nil {
		p.mu.Unlock()
		return
	}
	// Hold the lock across the teardown + swap so the runtime thread's backend
	// reads never observe a half-swapped state.
	old.Destroy()
	backend, err := p.fallbackFactory()
	if err != nil {
		p.mu.Unlock()
		select {
		case p.fatalCh <- fmt.Errorf("runtime: software fallback after GPU fatal (%w): %w", reason, err):
		default:
		}
		return
	}
	p.backend = backend
	if tb, ok := backend.(render.TextureBackend); ok {
		p.uploadQueue = render.NewUploadQueue(tb, 1024)
	}
	fromKind := p.fromKind
	p.mu.Unlock()

	if p.onFallback != nil {
		p.onFallback(fromKind, reason.Error())
	}
}

// destroyBackend destroys the current backend under the pipeline lock (used at
// shutdown and on surface loss). The backend object is kept so a subsequent
// surface-created event can re-initialize or recreate it (the lifecycle
// contract); a GPU-fatal fallback replaces it wholesale.
func (p *RenderPipeline) destroyBackend() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.backend != nil {
		p.backend.Destroy()
	}
}

// recreateOrInitialize rebuilds the backend against a new platform surface under
// the pipeline lock (preferring the lighter Recreate path when available).
func (p *RenderPipeline) recreateOrInitialize(surface render.Surface) error {
	if p == nil || surface == nil {
		return nil
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.backend == nil {
		return errors.New("runtime: no render backend to reinitialize")
	}
	if recreatable, ok := p.backend.(render.RecreatableBackend); ok {
		return recreatable.Recreate(surface)
	}
	return p.backend.Initialize(surface)
}

// evictCaches forwards to a CacheEvictor backend under the pipeline lock.
func (p *RenderPipeline) evictCaches() {
	if p == nil {
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	if evictor, ok := p.backend.(render.CacheEvictor); ok {
		evictor.EvictCaches()
	}
}

func (rt *renderThread) run() {
	for handoff := range rt.pipeline.handoffCh {
		backend := rt.pipeline.Backend()
		if backend == nil || handoff.frame == nil {
			if handoff.doneCh != nil {
				close(handoff.doneCh)
			}
			continue
		}
		if q := rt.pipeline.UploadQueue(); q != nil {
			q.DrainBudget()
		}
		if err := backend.Submit(handoff.frame); err != nil {
			var fatal *render.ErrGPUFatal
			if errors.As(err, &fatal) {
				// GPU-fatal: one-shot swap to software (Q9). The frame in flight
				// is dropped; the next frame renders on the software backend.
				rt.pipeline.swapToSoftwareFallback(err)
			} else {
				select {
				case rt.pipeline.fatalCh <- err:
				default:
				}
			}
		}
		if handoff.doneCh != nil {
			close(handoff.doneCh)
		}
	}
}

func (p *RenderPipeline) destroy() {
	p.destroyOnce.Do(func() {
		close(p.handoffCh)
	})
}
