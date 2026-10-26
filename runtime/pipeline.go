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
	frame              *render.FrameGroup
	frameID            uint64
	resourceGeneration uint64
	sequence           uint64
	doneCh             chan RenderCompletion
	async              bool
	reserved           bool
	cleanup            bool
}

type renderLifecycleKind uint8

const (
	renderLifecycleSurfaceLost renderLifecycleKind = iota + 1
	renderLifecycleSurfaceCreated
	renderLifecycleEvictCaches
)

type renderLifecycleRequest struct {
	targets     [render.MaxFrameWindows]render.WindowTarget
	targetCount int
	kind        renderLifecycleKind
	surface     render.Surface
	generation  uint64
	sequence    uint64
}

const (
	maxRenderLifecycleRequests = 16
	maxOutstandingFrames       = 3 // two ordinary groups and one cleanup group
)

// RenderCompletion is the immutable acknowledgment returned to the runtime
// after the render worker finishes its submission call. Software backends
// draw synchronously in Submit, so Ready identifies an available surface.
type WindowCompletion struct {
	render.WindowDescriptor
	Ready bool
}

type RenderCompletion struct {
	FallbackFrom        string
	FallbackReason      string
	WindowCount         int
	Windows             [render.MaxFrameWindows]WindowCompletion
	Frame               uint64
	ResourceGeneration  uint64
	Lifecycle           renderLifecycleKind
	LifecycleGeneration uint64
	DeviceGeneration    uint64
	HasDeviceGeneration bool
	Backend             string
	Ready               bool
	Err                 error
}

// RenderPipeline hands frames to the render backend through two prepaid
// ordinary groups plus an independent cleanup group.
//
// Slice 10 (Q9/FR-12): on a *render.ErrGPUFatal from Submit, the render thread
// destroys the GPU backend and swaps in a software backend bound to the current
// platform surface for the rest of the session (one-shot, no auto-retry). The
// render thread owns every backend call, including lifecycle changes and
// device-generation queries. `mu` protects pointer publication and fallback
// replacement for capability readers.
type RenderPipeline struct {
	mu                sync.RWMutex
	backend           render.Backend
	uploadQueue       *render.UploadQueue
	handoffCh         chan frameHandoff
	cleanupHandoffCh  chan frameHandoff
	lifecycleCh       chan renderLifecycleRequest
	fatalCh           chan error
	completedCh       chan RenderCompletion
	frameCredit       chan struct{}
	cleanupCredit     chan struct{}
	destroyOnce       sync.Once
	doneCh            chan struct{}
	shutdownRequested bool
	workerStarted     atomic.Bool
	sequence          atomic.Uint64
	orderMu           sync.Mutex

	// fromKind names the initially-selected backend for the fallback diagnostic
	// (e.g. "*vulkan.Backend").
	fromKind string
	// Startup/lifecycle capture supplies native targets. The render worker
	// never calls a runtime getter or an authored callback during fallback.
	targets         [render.MaxFrameWindows]render.WindowTarget
	targetCount     int
	fallbackFactory func(render.Surface) (render.Backend, error)
}

type renderThread struct {
	surfaceDestroyed bool
	pipeline         *RenderPipeline
}

func newRenderPipeline(backend render.Backend) *RenderPipeline {
	p := &RenderPipeline{
		backend:          backend,
		handoffCh:        make(chan frameHandoff, 2),
		cleanupHandoffCh: make(chan frameHandoff, 1),
		lifecycleCh:      make(chan renderLifecycleRequest, maxRenderLifecycleRequests),
		fatalCh:          make(chan error, 1),
		// Keep every possible acknowledgment buffered: the lifecycle queue may
		// hold its full capacity while one more request is executing, alongside
		// both ordinary frame groups and the reserved cleanup group. Completion
		// publication must not strand the render owner during shutdown.
		completedCh:   make(chan RenderCompletion, maxRenderLifecycleRequests+1+maxOutstandingFrames),
		frameCredit:   make(chan struct{}, 2),
		cleanupCredit: make(chan struct{}, 1),
		doneCh:        make(chan struct{}),
		fromKind:      backendKindName(backend),
		targetCount:   1,
		targets:       [render.MaxFrameWindows]render.WindowTarget{{ID: render.PrimaryWindow}},
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

// Backend returns the current backend pointer for capability wiring. Callers
// must not invoke backend methods; all such calls belong to the render owner.
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

// TryReserveFrame reserves one of the pipeline's two bounded in-flight frame
// groups before the runtime begins a frame. It never waits for the renderer.
func (p *RenderPipeline) TryReserveFrame() bool {
	if p == nil {
		return false
	}
	p.mu.RLock()
	available := p.backend != nil && !p.shutdownRequested
	p.mu.RUnlock()
	if !available {
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
func (p *RenderPipeline) SubmitReserved(frame *render.FrameGroup, frameID uint64) error {
	return p.enqueueFrame(frameHandoff{frame: frame, frameID: frameID, resourceGeneration: frame.Generation(), async: true, reserved: true})
}

func (p *RenderPipeline) enqueueFrame(handoff frameHandoff) error {
	p.orderMu.Lock()
	defer p.orderMu.Unlock()
	p.mu.RLock()
	stopping := p.shutdownRequested
	p.mu.RUnlock()
	sequence, sequenceErr := p.nextSequenceLocked()
	if stopping || sequenceErr != nil {
		if handoff.frame != nil {
			_ = handoff.frame.Release()
		}
		if handoff.reserved {
			if handoff.cleanup {
				p.ReleaseCleanupFrameReservation()
			} else {
				p.ReleaseFrameReservation()
			}
		}
		if sequenceErr != nil {
			return sequenceErr
		}
		return errors.New("runtime: render pipeline is shutting down")
	}
	handoff.sequence = sequence
	if handoff.cleanup {
		p.cleanupHandoffCh <- handoff
	} else {
		p.handoffCh <- handoff
	}
	return nil
}

// ReleaseFrameReservation returns a frame group when a reserved frame exits
// before renderer submission.
func (p *RenderPipeline) ReleaseFrameReservation() {
	if p == nil {
		return
	}
	select {
	case p.frameCredit <- struct{}{}:
	default:
		panic("runtime: duplicate ordinary frame credit release")
	}
}

// TryReserveCleanupFrame claims the independent cleanup group even when both
// ordinary frame groups are held by a delayed renderer.
func (p *RenderPipeline) TryReserveCleanupFrame() bool {
	if p == nil {
		return false
	}
	p.mu.RLock()
	available := p.backend != nil && !p.shutdownRequested
	p.mu.RUnlock()
	if !available {
		return false
	}
	select {
	case <-p.cleanupCredit:
		return true
	default:
		return false
	}
}

func (p *RenderPipeline) SubmitCleanupReserved(frame *render.FrameGroup, frameID uint64) error {
	return p.enqueueFrame(frameHandoff{frame: frame, frameID: frameID, resourceGeneration: frame.Generation(), async: true, reserved: true, cleanup: true})
}

func (p *RenderPipeline) ReleaseCleanupFrameReservation() {
	if p == nil {
		return
	}
	select {
	case p.cleanupCredit <- struct{}{}:
	default:
		panic("runtime: duplicate cleanup frame credit release")
	}
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
func (p *RenderPipeline) SubmitAndWait(frame *render.FrameGroup, frameID uint64) RenderCompletion {
	if p == nil || p.Backend() == nil {
		generation := frame.Generation()
		if frame != nil {
			_ = frame.Release()
		}
		return RenderCompletion{Frame: frameID, ResourceGeneration: generation, Err: errors.New("runtime: render backend unavailable")}
	}
	done := make(chan RenderCompletion, 1)
	p.reserveFrame()
	generation := frame.Generation()
	if err := p.enqueueFrame(frameHandoff{frame: frame, frameID: frameID, resourceGeneration: generation, doneCh: done, reserved: true}); err != nil {
		return RenderCompletion{Frame: frameID, ResourceGeneration: generation, Err: err}
	}
	return <-done
}

func (p *RenderPipeline) requestLifecycle(kind renderLifecycleKind, surface render.Surface, generation uint64, targets []render.WindowTarget) error {
	if p == nil {
		return errors.New("runtime: render pipeline unavailable")
	}
	p.orderMu.Lock()
	defer p.orderMu.Unlock()
	p.mu.RLock()
	stopping := p.shutdownRequested
	p.mu.RUnlock()
	if stopping {
		return errors.New("runtime: render pipeline is shutting down")
	}
	if len(targets) > render.MaxFrameWindows {
		return errors.New("runtime: too many lifecycle windows")
	}
	sequence, err := p.nextSequenceLocked()
	if err != nil {
		return err
	}
	request := renderLifecycleRequest{kind: kind, surface: surface, generation: generation, sequence: sequence, targetCount: len(targets)}
	copy(request.targets[:], targets)
	select {
	case p.lifecycleCh <- request:
		return nil
	default:
		return errors.New("runtime: render lifecycle request capacity exhausted")
	}
}

// nextSequenceLocked is called only under orderMu, before publication.
func (p *RenderPipeline) nextSequenceLocked() (uint64, error) {
	if p.sequence.Load() == ^uint64(0) {
		return 0, errors.New("runtime: render submission sequence exhausted")
	}
	return p.sequence.Add(1), nil
}

// swapToSoftwareFallback tears down the failing GPU backend and installs a
// software backend (Q9): one-shot, no auto-retry. Called from the render thread
// after an ErrGPUFatal; the frame in flight is dropped (one dropped frame is
// acceptable). If the software backend cannot be built, the failure surfaces on
// fatalCh (the runtime shuts down) rather than silently stalling.
func (p *RenderPipeline) swapToSoftwareFallback(reason error) (string, error) {
	p.mu.RLock()
	old, factory, from := p.backend, p.fallbackFactory, p.fromKind
	p.mu.RUnlock()
	if old == nil || factory == nil {
		return "", fmt.Errorf("runtime: software fallback unavailable after %w", reason)
	}
	var primary render.Surface
	for _, target := range p.targets[:p.targetCount] {
		if target.ID == render.PrimaryWindow {
			primary = target.Surface
		}
	}
	old.Destroy()
	backend, err := factory(primary)
	if err != nil {
		return "", fmt.Errorf("runtime: software fallback after %w: %w", reason, err)
	}
	if p.targetCount > 1 {
		multi, ok := backend.(render.WindowBackend)
		if !ok {
			backend.Destroy()
			return "", errors.New("runtime: fallback backend lacks window-set contract")
		}
		if err := multi.ConfigureWindows(p.targets[:p.targetCount]); err != nil {
			backend.Destroy()
			return "", err
		}
	}
	var uploads *render.UploadQueue
	if textures, ok := backend.(render.TextureBackend); ok {
		uploads = render.NewUploadQueue(textures, 1024)
	}
	p.mu.Lock()
	p.backend, p.uploadQueue = backend, uploads
	p.mu.Unlock()
	return from, nil
}

// QueueSurfaceLost serializes backend teardown after all previously accepted
// submissions and before any later submission.
func (p *RenderPipeline) QueueSurfaceLost(generation uint64) error {
	return p.requestLifecycle(renderLifecycleSurfaceLost, nil, generation, nil)
}

// QueueSurfaceCreated serializes surface recreation on the render owner. The
// returned readiness is published only by the matching completion.
func (p *RenderPipeline) QueueSurfaceCreated(surface render.Surface, generation uint64, targets []render.WindowTarget) error {
	return p.requestLifecycle(renderLifecycleSurfaceCreated, surface, generation, targets)
}

// QueueCacheEviction asks the render owner to release its reconstructible
// caches without making runtime control wait for a backend lock or GPU fence.
func (p *RenderPipeline) QueueCacheEviction(generation uint64) error {
	return p.requestLifecycle(renderLifecycleEvictCaches, nil, generation, nil)
}

func (rt *renderThread) run() {
	defer func() {
		rt.pipeline.mu.RLock()
		stopping, backend := rt.pipeline.shutdownRequested, rt.pipeline.backend
		rt.pipeline.mu.RUnlock()
		if stopping && backend != nil {
			backend.Destroy()
		}
		close(rt.pipeline.doneCh)
	}()
	ordinary := rt.pipeline.handoffCh
	cleanup := rt.pipeline.cleanupHandoffCh
	lifecycle := rt.pipeline.lifecycleCh
	var ordinaryPending, cleanupPending *frameHandoff
	var lifecyclePending *renderLifecycleRequest
	for ordinary != nil || cleanup != nil || lifecycle != nil {
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
		if lifecyclePending == nil && lifecycle != nil {
			select {
			case request, ok := <-lifecycle:
				if ok {
					lifecyclePending = &request
				} else {
					lifecycle = nil
				}
			default:
			}
		}
		if ordinaryPending != nil || cleanupPending != nil || lifecyclePending != nil {
			ordinarySeq, cleanupSeq, lifecycleSeq := ^uint64(0), ^uint64(0), ^uint64(0)
			if ordinaryPending != nil {
				ordinarySeq = ordinaryPending.sequence
			}
			if cleanupPending != nil {
				cleanupSeq = cleanupPending.sequence
			}
			if lifecyclePending != nil {
				lifecycleSeq = lifecyclePending.sequence
			}
			switch {
			case lifecycleSeq < ordinarySeq && lifecycleSeq < cleanupSeq:
				rt.processLifecycle(*lifecyclePending)
				lifecyclePending = nil
			case cleanupSeq < ordinarySeq:
				rt.process(*cleanupPending)
				cleanupPending = nil
			default:
				rt.process(*ordinaryPending)
				ordinaryPending = nil
			}
			continue
		}
		if ordinary == nil && cleanup == nil && lifecycle == nil {
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
		case request, ok := <-lifecycle:
			if ok {
				lifecyclePending = &request
			} else {
				lifecycle = nil
			}
		}
	}
	if ordinaryPending != nil {
		rt.process(*ordinaryPending)
	}
	if cleanupPending != nil {
		rt.process(*cleanupPending)
	}
	if lifecyclePending != nil {
		rt.processLifecycle(*lifecyclePending)
	}
}

func (rt *renderThread) processLifecycle(request renderLifecycleRequest) {
	p := rt.pipeline
	p.mu.RLock()
	backend := p.backend
	p.mu.RUnlock()
	var err error
	if backend == nil {
		err = errors.New("runtime: render backend unavailable for lifecycle operation")
	} else {
		func() {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("runtime: render lifecycle backend panic: %v", recovered)
				}
			}()
			switch request.kind {
			case renderLifecycleSurfaceLost:
				backend.Destroy()
				rt.surfaceDestroyed = true
			case renderLifecycleSurfaceCreated:
				if request.surface == nil {
					err = errors.New("runtime: nil surface in render lifecycle request")
					return
				}
				if recreatable, ok := backend.(render.RecreatableBackend); ok && !rt.surfaceDestroyed {
					err = recreatable.Recreate(request.surface)
				} else {
					err = backend.Initialize(request.surface)
				}
				if err == nil && request.targetCount > 1 {
					if multi, ok := backend.(render.WindowBackend); ok {
						err = multi.ConfigureWindows(request.targets[:request.targetCount])
					} else {
						err = errors.New("runtime: recreated backend lacks window-set contract")
					}
				}
				if err == nil {
					rt.surfaceDestroyed = false
					p.targets = [render.MaxFrameWindows]render.WindowTarget{}
					p.targetCount = request.targetCount
					copy(p.targets[:], request.targets[:request.targetCount])
					if p.targetCount == 0 {
						p.targetCount = 1
						p.targets[0] = render.WindowTarget{ID: render.PrimaryWindow, Surface: request.surface}
					}
				}
			case renderLifecycleEvictCaches:
				if evictor, ok := backend.(render.CacheEvictor); ok {
					evictor.EvictCaches()
				}
			default:
				err = fmt.Errorf("runtime: unknown render lifecycle operation %d", request.kind)
			}
		}()
	}
	completion := RenderCompletion{
		Lifecycle: request.kind, LifecycleGeneration: request.generation,
		Backend: backendKindName(backend), Ready: err == nil, Err: err,
	}
	if backend != nil {
		completion.DeviceGeneration, completion.HasDeviceGeneration, err = queryDeviceGeneration(backend)
		if err != nil && completion.Err == nil {
			completion.Err = err
			completion.Ready = false
		}
	}
	p.completedCh <- completion
}

func (rt *renderThread) process(handoff frameHandoff) {
	backend := rt.pipeline.Backend()
	completion := RenderCompletion{Frame: handoff.frameID, ResourceGeneration: handoff.resourceGeneration, Backend: backendKindName(backend)}
	if backend == nil || handoff.frame == nil {
		completion.Err = errors.New("runtime: render backend unavailable")
	} else {
		completion.WindowCount = handoff.frame.WindowCount()
		for i := 0; i < completion.WindowCount; i++ {
			descriptor, _ := handoff.frame.WindowAt(i)
			completion.Windows[i].WindowDescriptor = descriptor
		}
		completion.Err = func() (err error) {
			defer func() {
				if recovered := recover(); recovered != nil {
					err = fmt.Errorf("runtime: render backend panic: %v", recovered)
				}
			}()
			if q := rt.pipeline.UploadQueue(); q != nil {
				q.DrainBudget()
			}
			for i := 0; i < completion.WindowCount; i++ {
				frame, retainErr := handoff.frame.RetainWindow(i)
				if retainErr != nil {
					return retainErr
				}
				submitErr := func() (err error) {
					defer func() {
						releaseErr := frame.Release()
						if err == nil {
							err = releaseErr
						}
					}()
					if multi, ok := backend.(render.WindowBackend); ok {
						return multi.SubmitWindow(completion.Windows[i].ID, frame)
					}
					if completion.WindowCount != 1 || completion.Windows[i].ID != render.PrimaryWindow {
						return errors.New("runtime: backend lacks required window-set rendering contract")
					}
					return backend.Submit(frame)
				}()
				if submitErr != nil {
					return fmt.Errorf("runtime: render window %q: %w", completion.Windows[i].ID, submitErr)
				}
				completion.Windows[i].Ready = true
			}
			return nil
		}()
	}
	if handoff.frame != nil {
		if err := handoff.frame.Release(); completion.Err == nil {
			completion.Err = err
		}
	}
	if completion.Err != nil {
		var fatal *render.ErrGPUFatal
		if errors.As(completion.Err, &fatal) {
			from, err := rt.pipeline.swapToSoftwareFallback(completion.Err)
			if err != nil {
				select {
				case rt.pipeline.fatalCh <- err:
				default:
				}
			} else {
				completion.FallbackFrom = from
				completion.FallbackReason = completion.Err.Error()
			}
		} else {
			select {
			case rt.pipeline.fatalCh <- completion.Err:
			default:
			}
		}
	}
	if backend != nil {
		completion.DeviceGeneration, completion.HasDeviceGeneration, completion.Err = mergeDeviceGeneration(backend, completion.Err)
	}
	completion.Ready = completion.Err == nil
	rt.publishCompletion(handoff, completion)
}

func mergeDeviceGeneration(backend render.Backend, currentErr error) (uint64, bool, error) {
	generation, ok, err := queryDeviceGeneration(backend)
	if currentErr == nil {
		currentErr = err
	}
	return generation, ok, currentErr
}

func queryDeviceGeneration(backend render.Backend) (generation uint64, ok bool, err error) {
	provider, ok := backend.(render.DeviceGenerationProvider)
	if !ok {
		return 0, false, nil
	}
	defer func() {
		if recovered := recover(); recovered != nil {
			err = fmt.Errorf("runtime: render backend device-generation panic: %v", recovered)
		}
	}()
	return provider.DeviceGeneration(), true, nil
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
		p.orderMu.Lock()
		p.mu.Lock()
		p.shutdownRequested = true
		p.mu.Unlock()
		close(p.handoffCh)
		close(p.cleanupHandoffCh)
		close(p.lifecycleCh)
		p.orderMu.Unlock()
	})
}

func (p *RenderPipeline) shutdown(timeout time.Duration) bool {
	if p == nil {
		return true
	}
	p.mu.Lock()
	p.shutdownRequested = true
	startWorker := !p.workerStarted.Load()
	p.mu.Unlock()
	if startWorker {
		p.start()
	}
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
