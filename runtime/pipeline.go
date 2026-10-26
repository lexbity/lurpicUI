package runtime

import (
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/render"
)

type frameHandoff struct {
	frame              *render.Frame
	frameID            uint64
	resourceGeneration uint64
	doneCh             chan RenderCompletion
	async              bool
	reserved           bool
	cleanup            bool
}

// RenderCompletion is the immutable acknowledgment returned to the runtime
// after the render worker finishes its submission call. Software backends
// draw synchronously in Submit, so Ready identifies an available surface.
type RenderCompletion struct {
	Frame              uint64
	ResourceGeneration uint64
	Backend            string
	Ready              bool
	Err                error
}

// RenderPipeline hands frames to the render backend with capacity-1 backpressure.
//
// Slice 10 (Q9/FR-12): on a *render.ErrGPUFatal from Submit, the render thread
// destroys the GPU backend and swaps in a software backend bound to the current
// platform surface for the rest of the session (one-shot, no auto-retry). The
// `mu` guards `backend`/`uploadQueue` against the runtime thread's lifecycle
// reads (device generation, surface lost/created, eviction).
type RenderPipeline struct {
	mu                sync.RWMutex
	backend           render.Backend
	uploadQueue       *render.UploadQueue
	handoffCh         chan frameHandoff
	cleanupHandoffCh  chan frameHandoff
	fatalCh           chan error
	completedCh       chan RenderCompletion
	frameCredit       chan struct{}
	cleanupCredit     chan struct{}
	destroyOnce       sync.Once
	doneCh            chan struct{}
	shutdownRequested bool
	workerStarted     atomic.Bool

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
		backend:          backend,
		handoffCh:        make(chan frameHandoff, 1),
		cleanupHandoffCh: make(chan frameHandoff, 1),
		fatalCh:          make(chan error, 1),
		// At most one handoff is queued while another frame is in the
		// backend. Two slots retain every completion until the runtime's next
		// control turn without making render completion block the runtime.
		completedCh:   make(chan RenderCompletion, 3),
		frameCredit:   make(chan struct{}, 2),
		cleanupCredit: make(chan struct{}, 1),
		doneCh:        make(chan struct{}),
		fromKind:      backendKindName(backend),
	}
	p.frameCredit <- struct{}{}
	p.frameCredit <- struct{}{}
	p.cleanupCredit <- struct{}{}
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

func (p *RenderPipeline) start() {
	if p == nil || !p.workerStarted.CompareAndSwap(false, true) {
		return
	}
	go (&renderThread{pipeline: p}).run()
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
func (p *RenderPipeline) Submit(frame *render.Frame, frameID uint64) {
	if p == nil || p.Backend() == nil {
		if frame != nil {
			_ = frame.Release()
		}
		return
	}
	p.reserveFrame()
	p.SubmitReserved(frame, frameID)
}

// TryReserveFrame reserves one of the pipeline's two bounded in-flight frame
// groups before the runtime begins a frame. It never waits for the renderer.
func (p *RenderPipeline) TryReserveFrame() bool {
	if p == nil || p.Backend() == nil {
		return false
	}
	select {
	case <-p.frameCredit:
		return true
	default:
		return false
	}
}

// SubmitReserved transfers a previously reserved frame group to the render
// worker. The capacity reservation guarantees this handoff cannot block.
func (p *RenderPipeline) SubmitReserved(frame *render.Frame, frameID uint64) {
	if p == nil {
		return
	}
	p.handoffCh <- frameHandoff{frame: frame, frameID: frameID, resourceGeneration: frame.Generation(), async: true, reserved: true}
}

// ReleaseFrameReservation returns a frame group when a reserved frame exits
// before renderer submission.
func (p *RenderPipeline) ReleaseFrameReservation() {
	if p == nil {
		return
	}
	p.frameCredit <- struct{}{}
}

// TryReserveCleanupFrame claims the independent cleanup group even when both
// ordinary frame groups are held by a delayed renderer.
func (p *RenderPipeline) TryReserveCleanupFrame() bool {
	if p == nil || p.Backend() == nil {
		return false
	}
	select {
	case <-p.cleanupCredit:
		return true
	default:
		return false
	}
}

func (p *RenderPipeline) SubmitCleanupReserved(frame *render.Frame, frameID uint64) {
	if p == nil {
		return
	}
	p.cleanupHandoffCh <- frameHandoff{frame: frame, frameID: frameID, resourceGeneration: frame.Generation(), async: true, reserved: true, cleanup: true}
}

func (p *RenderPipeline) ReleaseCleanupFrameReservation() {
	if p == nil {
		return
	}
	p.cleanupCredit <- struct{}{}
}

func (p *RenderPipeline) reserveFrame() {
	<-p.frameCredit
}

// Completions exposes immutable render acknowledgments to the runtime
// control loop. The channel is bounded by the pipeline's two in-flight
// frame groups.
func (p *RenderPipeline) Completions() <-chan RenderCompletion {
	if p == nil {
		return nil
	}
	return p.completedCh
}

// SubmitAndWait hands a frame to the renderer and waits for completion.
func (p *RenderPipeline) SubmitAndWait(frame *render.Frame, frameID uint64) RenderCompletion {
	if p == nil || p.Backend() == nil {
		generation := frame.Generation()
		if frame != nil {
			_ = frame.Release()
		}
		return RenderCompletion{Frame: frameID, ResourceGeneration: generation, Err: errors.New("runtime: render backend unavailable")}
	}
	done := make(chan RenderCompletion, 1)
	p.reserveFrame()
	p.handoffCh <- frameHandoff{frame: frame, frameID: frameID, resourceGeneration: frame.Generation(), doneCh: done, reserved: true}
	return <-done
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
	defer func() {
		rt.pipeline.mu.Lock()
		if rt.pipeline.shutdownRequested && rt.pipeline.backend != nil {
			rt.pipeline.backend.Destroy()
		}
		rt.pipeline.mu.Unlock()
		close(rt.pipeline.doneCh)
	}()
	ordinary := rt.pipeline.handoffCh
	cleanup := rt.pipeline.cleanupHandoffCh
	var ordinaryPending, cleanupPending *frameHandoff
	for ordinary != nil || cleanup != nil {
		if ordinaryPending == nil && ordinary != nil {
			select {
			case handoff, ok := <-ordinary:
				if ok {
					ordinaryPending = &handoff
				} else {
					ordinary = nil
				}
			default:
			}
		}
		if cleanupPending == nil && cleanup != nil {
			select {
			case handoff, ok := <-cleanup:
				if ok {
					cleanupPending = &handoff
				} else {
					cleanup = nil
				}
			default:
			}
		}
		if ordinaryPending != nil || cleanupPending != nil {
			if ordinaryPending == nil || cleanupPending != nil && cleanupPending.frameID < ordinaryPending.frameID {
				rt.process(*cleanupPending)
				cleanupPending = nil
			} else {
				rt.process(*ordinaryPending)
				ordinaryPending = nil
			}
			continue
		}
		if ordinary == nil && cleanup == nil {
			break
		}
		select {
		case handoff, ok := <-cleanup:
			if ok {
				cleanupPending = &handoff
			} else {
				cleanup = nil
			}
		case handoff, ok := <-ordinary:
			if ok {
				ordinaryPending = &handoff
			} else {
				ordinary = nil
			}
		}
	}
	if ordinaryPending != nil {
		rt.process(*ordinaryPending)
	}
	if cleanupPending != nil {
		rt.process(*cleanupPending)
	}
}

func (rt *renderThread) process(handoff frameHandoff) {
	backend := rt.pipeline.Backend()
	var submitErr error
	if backend == nil || handoff.frame == nil {
		submitErr = errors.New("runtime: render backend unavailable")
	} else {
		submitErr = func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("runtime: render backend panic: %v", recovered)
				}
			}()
			if q := rt.pipeline.UploadQueue(); q != nil {
				q.DrainBudget()
			}
			return backend.Submit(handoff.frame)
		}()
	}
	// Submit is the backend's immutable-page consumption boundary. Backends
	// that retain data beyond this call must hold a separate frame lease. The
	// transferred lease is released for both ordinary errors and panics.
	if handoff.frame != nil {
		if releaseErr := handoff.frame.Release(); submitErr == nil && releaseErr != nil {
			submitErr = releaseErr
		}
	}
	if submitErr != nil {
		var fatal *render.ErrGPUFatal
		if errors.As(submitErr, &fatal) {
			// GPU-fatal: one-shot swap to software (Q9). The frame in flight
			// is dropped; the next frame renders on the software backend.
			rt.pipeline.swapToSoftwareFallback(submitErr)
		} else {
			select {
			case rt.pipeline.fatalCh <- submitErr:
			default:
			}
		}
	}
	backendName := "unknown"
	if backend != nil {
		backendName = backendKindName(backend)
	}
	rt.publishCompletion(handoff, RenderCompletion{
		Frame: handoff.frameID, ResourceGeneration: handoff.resourceGeneration, Backend: backendName,
		Ready: submitErr == nil, Err: submitErr,
	})
}

func (rt *renderThread) publishCompletion(handoff frameHandoff, completion RenderCompletion) {
	if handoff.doneCh != nil {
		handoff.doneCh <- completion
		close(handoff.doneCh)
		if handoff.reserved {
			if handoff.cleanup {
				rt.pipeline.ReleaseCleanupFrameReservation()
			} else {
				rt.pipeline.ReleaseFrameReservation()
			}
		}
		return
	}
	if handoff.async {
		rt.pipeline.completedCh <- completion
	}
	if handoff.reserved {
		if handoff.cleanup {
			rt.pipeline.ReleaseCleanupFrameReservation()
		} else {
			rt.pipeline.ReleaseFrameReservation()
		}
	}
}

func (p *RenderPipeline) destroy() {
	p.destroyOnce.Do(func() {
		close(p.handoffCh)
		close(p.cleanupHandoffCh)
	})
}

func (p *RenderPipeline) shutdown(timeout time.Duration) bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	p.shutdownRequested = true
	if !p.workerStarted.Load() {
		if p.backend != nil {
			p.backend.Destroy()
		}
		p.mu.Unlock()
		p.destroy()
		close(p.doneCh)
		return true
	}
	p.mu.Unlock()
	p.destroy()
	return p.waitStopped(timeout)
}

func (p *RenderPipeline) waitStopped(timeout time.Duration) bool {
	if p == nil {
		return true
	}
	if !p.workerStarted.Load() {
		return true
	}
	if timeout <= 0 {
		<-p.doneCh
		return true
	}
	select {
	case <-p.doneCh:
		return true
	case <-time.After(timeout):
		return false
	}
}
