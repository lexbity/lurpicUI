package runtime

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

const debugDisableSurfaceCreated = false

func (rt *Runtime) RunOneFrame() {
	rt.RunOneFrameAt(rt.clock.Now())
}

// RunOneFrameAt runs one frame observing the supplied instant. It is the
// headless-harness seam: a test owns frame time completely (dt = now − the
// previous frame's now). In Virtual mode the parameter is ignored — the
// virtual clock is the only time authority (QP-5).
func (rt *Runtime) RunOneFrameAt(now time.Time) {
	_ = rt.start()
	rt.driveDrain()
	if rt.clock.Mode() == ClockVirtual {
		if rt.bootstrapFramePending {
			rt.runBootstrapFrame(true)
			return
		}
		// Parked reads/control work consumes no app frame. Explicit frame
		// tokens are consumed once, through the same scheduler as Run.
		rt.runScheduledVirtualFrame(true)
		return
	}
	if rt.drivePort != nil && len(rt.drivePort.frameTokens) != 0 {
		rt.runScheduledVirtualFrame(true)
		return
	}
	// RunOneFrameAt is the headless scheduler boundary: in Wall mode its
	// caller supplies the engine instant for this frame. Run() selects the
	// same instant from rt.clock.Now() after wall pacing.
	rt.runFrame(now, true, false)
	if rt.lastRenderCompletion.Frame == rt.frameNumber {
		rt.driveHandleRenderCompletion(rt.lastRenderCompletion)
	}
}

func (rt *Runtime) runBootstrapFrame(waitForRender bool) bool {
	if !rt.bootstrapFramePending {
		return false
	}
	reserved := false
	if !waitForRender && rt.renderPipeline != nil && rt.isSurfaceReady() {
		reserved = rt.renderPipeline.TryReserveFrame()
		if !reserved {
			return false
		}
	}
	rt.bootstrapFramePending = false
	rt.bootstrapFrameAwaiting = true
	rt.runFrame(rt.clock.Now(), waitForRender, reserved)
	if waitForRender && rt.lastRenderCompletion.Frame == rt.frameNumber {
		rt.driveHandleRenderCompletion(rt.lastRenderCompletion)
	}
	return true
}

func (rt *Runtime) Run() error {

	if err := rt.start(); err != nil {
		return err
	}
	for {
		if !rt.waitIfPausedOrStopped() {
			return nil
		}
		if runtimeTraceActive() {
			runtimeTracef("Run loop pre-wait frame=%d shutdown=%v", rt.frameNumber, isChannelClosed(rt.shutdownCh))
		}
		select {
		case <-rt.shutdownCh:
			<-rt.doneCh
			return nil
		case err := <-rt.renderPipeline.fatalCh:
			if err != nil {
				select {
				case <-rt.shutdownCh:
				default:
					close(rt.shutdownCh)
				}
				<-rt.doneCh
				return fmt.Errorf("runtime: render failure: %w", err)
			}
		default:
		}
		// Close the gap between the non-blocking select above and
		// frameTimer.Wait: if shutdown was signaled in that window, drain to
		// doneCh and exit instead of running a useless frame against a tree
		// that superviseShutdown may already be disposing.
		if isChannelClosed(rt.shutdownCh) {
			<-rt.doneCh
			return nil
		}
		if rt.clock.Mode() == ClockVirtual {
			// Virtual mode parks between explicit frame tokens. Control reads
			// and render acknowledgments remain serviceable without a frame.
			if !rt.virtualParkStep() {
				return nil
			}
			continue
		}
		pacingNow := rt.frameTimer.Wait() // wall pacing only; the body reads engine time.
		if runtimeTraceActive() {
			runtimeTracef("Run loop got frame tick frame=%d pace=%s", rt.frameNumber+1, pacingNow.Format(time.RFC3339Nano))
		}
		// LDP cut (QP-6): drain between frames, before this frame's body.
		rt.driveDrain()
		if rt.drivePort != nil && len(rt.drivePort.frameTokens) != 0 {
			if !rt.runScheduledVirtualFrame(false) && rt.clock.Mode() == ClockVirtual {
				continue
			}
			continue
		}
		if rt.clock.Mode() == ClockVirtual {
			continue
		}
		reserved := false
		if rt.renderPipeline != nil && rt.isSurfaceReady() {
			reserved = rt.renderPipeline.TryReserveFrame()
			if !reserved {
				// Keep serving control work while the bounded render window is
				// full. The next frame tick or control wake retries admission.
				continue
			}
		}
		rt.runFrame(rt.clock.Now(), false, reserved)
	}
}

// virtualParkStep blocks until a drive command (or shutdown) wakes the
// runtime, then drains the inbox at the cut between frames. It renders
// nothing: in Virtual mode frames happen only inside time.advance (via
// top-level scheduler). Returns false when shutdown was signaled.
func (rt *Runtime) virtualParkStep() bool {
	var wake <-chan struct{}
	if rt.drivePort != nil {
		wake = rt.drivePort.wake
	}
	for {
		if rt.bootstrapFramePending {
			if rt.runBootstrapFrame(false) {
				return true
			}
		}
		if rt.drivePort != nil && len(rt.drivePort.frameTokens) > 0 && rt.runScheduledVirtualFrame(false) {
			return true
		}
		var completions <-chan RenderCompletion
		if rt.renderPipeline != nil {
			completions = rt.renderPipeline.Completions()
		}
		var jobWake <-chan struct{}
		if rt.jobPool != nil {
			jobWake = rt.jobPool.FrontierWake()
		}
		var leaseTimer *time.Timer
		var leaseWake <-chan time.Time
		if rt.drivePort != nil && rt.drivePort.timeOwnerSession == "" && !rt.drivePort.bootstrapTimeUntil.IsZero() {
			delay := rt.drivePort.bootstrapTimeUntil.Sub(rt.clock.WallNow())
			if delay < 0 {
				delay = 0
			}
			leaseTimer = time.NewTimer(delay)
			leaseWake = leaseTimer.C
		}
		select {
		case <-rt.shutdownCh:
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			return false
		case completion := <-completions:
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			rt.driveHandleRenderCompletion(completion)
		case <-jobWake:
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			// Recheck the frozen frontier below; worker completion never
			// commits app state outside phase 1.
		case <-wake:
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			rt.driveDrain()
		case <-leaseWake:
			rt.drivePort.expireBootstrapLease()
		}
		if !rt.waitIfPausedOrStopped() {
			return false
		}
		if rt.drivePort != nil && len(rt.drivePort.frameTokens) > 0 {
			if rt.runScheduledVirtualFrame(false) {
				return true
			}
			// Credit is returned by a render completion. Stay responsive to
			// control and shutdown while waiting for it.
			continue
		}
		if rt.clock.Mode() != ClockVirtual {
			return true
		}
		// Control wakeups can be spurious/coalesced; with no work, park again.
	}
}

func (rt *Runtime) Shutdown() {

	rt.shutdownMu.Lock()
	if rt.stopping {
		rt.shutdownMu.Unlock()
		<-rt.doneCh
		return
	}
	rt.stopping = true
	started := rt.started
	rt.shutdownMu.Unlock()

	if !started {
		_ = rt.shutdown()
		close(rt.doneCh)
		return
	}
	rt.lifecycleMu.Lock()
	rt.paused = false
	if rt.lifecycleCond != nil {
		rt.lifecycleCond.Broadcast()
	}
	rt.lifecycleMu.Unlock()
	select {
	case <-rt.shutdownCh:
	default:
		close(rt.shutdownCh)
	}
	<-rt.doneCh
}

func (rt *Runtime) start() error {

	var startErr error
	rt.startOnce.Do(func() {
		if runtimeTraceActive() {
			runtimeTracef("start begin runtimeThread=%v", syncutil.OnRuntimeThread())
		}
		if !syncutil.OnRuntimeThread() {
			syncutil.RegisterRuntimeThread()
		}
		store.SetSignalQueueHook(rt.queueSignal)
		rt.installRecoveryHooks()
		go (&renderThread{pipeline: rt.renderPipeline}).run()
		rt.bindLifecycleCallbacks()
		rt.contentScale = rt.effectiveContentScale()
		rt.attachTree(rt.root)
		rt.activateTree(rt.root)
		rt.markTreeDirty(rt.root, facet.DirtyAll)
		go rt.superviseShutdown()
		rt.shutdownMu.Lock()
		rt.started = true
		rt.shutdownMu.Unlock()
		if runtimeTraceActive() {
			runtimeTracef("start complete")
		}
	})
	return startErr
}

// installRecoveryHooks registers the input and facet callback recovery hooks,
// both delegating to the runtime's guardedCallback. They are cleared in
// shutdown() so no stale runtime closure outlives its runtime.
func (rt *Runtime) installRecoveryHooks() {
	input.SetRecoveryHook(rt.guardedCallback)
	facet.SetCallbackRecoveryHook(rt.guardedCallback)
}

// guardedCallback runs a callback under the runtime's per-facet quarantine. It
// reports whether the callback ran to completion (false when the facet was
// already quarantined or was quarantined by the call itself). Its signature
// satisfies both input.RecoveryHook and facet.CallbackRecoveryHook.
func (rt *Runtime) guardedCallback(id facet.FacetID, role string, cb func()) bool {
	ran := true
	rt.guardedInvoke(id, role, func() {
		cb()
	})
	if rt.isPoisoned(id) {
		ran = false
	}
	return ran
}

func (rt *Runtime) bindLifecycleCallbacks() {
	if rt.platformApp == nil {
		return
	}
	lc, ok := platform.LifecycleCapableOf(rt.platformApp)
	if !ok || lc == nil {
		return
	}
	rt.lifecycleMu.Lock()
	if rt.lifecycleBound {
		rt.lifecycleMu.Unlock()
		return
	}
	rt.lifecycleBound = true
	rt.lifecycleMu.Unlock()

	lc.OnPause(func() {
		rt.handlePlatformPause()
	})
	lc.OnResume(func() {
		rt.handlePlatformResume()
	})
	lc.OnLowMemory(func() {
		rt.handlePlatformLowMemory()
	})
	if !debugDisableSurfaceCreated {
		lc.OnSurfaceLost(func() {
			rt.handleSurfaceLost()
		})
		lc.OnSurfaceCreated(func(surface platform.Surface) {
			rt.handleSurfaceCreated(surface)
		})
	}
}

func (rt *Runtime) superviseShutdown() {
	defer close(rt.doneCh)
	<-rt.shutdownCh
	_ = rt.shutdown()
}

func (rt *Runtime) shutdown() error {

	rt.jobPool.CancelAll()
	rt.runShutdownHooks()
	rt.clearShutdownHooks()
	rt.disposeTree(rt.root)
	rt.jobPool.Shutdown()
	rt.clearPhase1TickHooks()
	rt.rootStyleSubs.Release()
	if rt.renderPipeline != nil {
		rt.renderPipeline.destroy()
	}
	rt.renderPipeline.destroyBackend()
	store.SetSignalQueueHook(nil)
	input.ClearRecoveryHook()
	facet.ClearCallbackRecoveryHook()
	syncutil.ResetRuntimeThreadForTest()
	return nil
}

func (rt *Runtime) waitIfPausedOrStopped() bool {

	rt.lifecycleMu.Lock()
	for rt.paused {
		rt.lifecycleCond.Wait()
		select {
		case <-rt.shutdownCh:
			rt.lifecycleMu.Unlock()
			<-rt.doneCh
			return false
		default:
		}
	}
	rt.lifecycleMu.Unlock()
	return true
}

func (rt *Runtime) isPaused() bool {

	rt.lifecycleMu.Lock()
	paused := rt.paused
	rt.lifecycleMu.Unlock()
	return paused
}

func (rt *Runtime) isSurfaceReady() bool {

	rt.lifecycleMu.Lock()
	ready := rt.surfaceReady
	rt.lifecycleMu.Unlock()
	return ready
}

func (rt *Runtime) AddFacet(parent, child facet.FacetImpl, attachment facet.Attachment) {
	if parent == nil || child == nil {
		return
	}
	parentBase := parent.Base()
	childBase := child.Base()
	if parentBase == nil || childBase == nil {
		return
	}
	parentBase.AddChildRuntime(childBase)
	if rt.childAttachments == nil {
		rt.childAttachments = make(map[facet.FacetID]facet.Attachment)
	}
	rt.childAttachments[childBase.ID()] = attachment
	if parentBase.State() == facet.StateCreated {
		return
	}
	rt.attachTree(child)
	rt.activateTree(child)
	if rt.projectionLayers == nil {
		rt.projectionLayers = make(map[facet.FacetID]facet.ProjectionLayer)
	}
	parentBase.InvalidateWithSource(facet.DirtyLayout, "runtime.AddFacet")
	rt.dirtyFacets[parentBase.ID()] |= facet.DirtyLayout
	rt.dirtySources[parentBase.ID()] = "runtime.AddFacet"
	rt.markTreeDirty(child, facet.DirtyAll)
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) RemoveFacet(child facet.FacetImpl) {
	if child == nil || child.Base() == nil {
		return
	}
	parent := child.Base().Parent()
	if parent == nil {
		return
	}
	parent.RemoveChild(child.Base())
	if rt.childAttachments != nil {
		delete(rt.childAttachments, child.Base().ID())
	}
	if rt.projectionLayers != nil {
		delete(rt.projectionLayers, child.Base().ID())
	}
	parent.InvalidateWithSource(facet.DirtyLayout, "runtime.RemoveFacet")
	rt.dirtyFacets[parent.ID()] |= facet.DirtyLayout
	rt.dirtySources[parent.ID()] = "runtime.RemoveFacet"
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) UpdateChildAttachment(child facet.FacetImpl, attachment facet.Attachment) {
	if child == nil || child.Base() == nil {
		return
	}
	if rt.childAttachments == nil {
		rt.childAttachments = make(map[facet.FacetID]facet.Attachment)
	}
	rt.childAttachments[child.Base().ID()] = attachment
	if parent := child.Base().Parent(); parent != nil {
		parent.InvalidateWithSource(facet.DirtyLayout, "runtime.UpdateChildAttachment")
		rt.dirtyFacets[parent.ID()] |= facet.DirtyLayout
		rt.dirtySources[parent.ID()] = "runtime.UpdateChildAttachment"
	}
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) initiateShutdown() {

	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
	select {
	case <-rt.shutdownCh:
	default:
		close(rt.shutdownCh)
	}
}
