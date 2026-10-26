package runtime

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/platform"
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
		parkedEvents := rt.collectPlatformEvents()
		if len(parkedEvents) != 0 {
			human := false
			for _, event := range parkedEvents {
				if isHumanInput(event) {
					human = true
					break
				}
			}
			if human {
				rt.preemptDriveForHuman()
				kept := parkedEvents[:0]
				for _, event := range parkedEvents {
					if eventSource(event) != platform.SourceDrive {
						kept = append(kept, event)
					}
				}
				rt.pendingEvents = append(rt.pendingEvents, kept...)
				if rt.drivePort != nil && len(rt.drivePort.frameTokens) != 0 {
					rt.runScheduledVirtualFrame(true)
				}
				return
			}
			rt.pendingEvents = append(rt.pendingEvents, parkedEvents...)
		}
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
	rt.drivePrepareInputFrame()
	// RunOneFrameAt is the headless scheduler boundary: in Wall mode its
	// caller supplies the engine instant for this frame. Run() selects the
	// same instant from rt.clock.Now() after wall pacing.
	rt.runFrame(now, true, false, false)
	if rt.lastRenderCompletion.Frame == rt.frameNumber {
		rt.driveHandleRenderCompletion(rt.lastRenderCompletion)
	}
}

// RunControlTurn services queued drive control and completed render
// acknowledgments without authorizing or executing an application frame. It is
// the scheduler seam for headless clients that need to await a control reply.
// A caller that needs frame work must authorize it separately through the
// normal scheduler.
func (rt *Runtime) RunControlTurn() error {
	if rt == nil {
		return fmt.Errorf("runtime: control turn on nil runtime")
	}
	if err := rt.start(); err != nil {
		return err
	}
	rt.driveDrain()
	rt.driveCollectRenderCompletions()
	return nil
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
	rt.runFrame(rt.clock.Now(), waitForRender, reserved, false)
	if waitForRender && rt.lastRenderCompletion.Frame == rt.frameNumber {
		rt.driveHandleRenderCompletion(rt.lastRenderCompletion)
	}
	return true
}

func (rt *Runtime) Run() error {

	if err := rt.start(); err != nil {
		return err
	}
	defer rt.setPlatformFrameDemand(false)
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
			rt.setPlatformFrameDemand(false)
			// Virtual mode parks between explicit frame tokens. Control reads
			// and render acknowledgments remain serviceable without a frame.
			if !rt.virtualParkStep() {
				return nil
			}
			continue
		}
		if !rt.hasWallFrameWork() {
			rt.setPlatformFrameDemand(false)
			if !rt.waitForWallWork() {
				return nil
			}
			continue
		}
		rt.setPlatformFrameDemand(true)
		pacingNow, frameEligible := rt.frameTimer.WaitWithWake(rt.platformEventWake(), rt.drainPlatformEvents, rt.hasWallFrameWork)
		if !frameEligible {
			rt.setPlatformFrameDemand(false)
			continue
		}
		rt.wallWakePending.Store(false)
		if runtimeTraceActive() {
			runtimeTracef("Run loop got frame tick frame=%d pace=%s", rt.frameNumber+1, pacingNow.Format(time.RFC3339Nano))
		}
		// LDP cut (QP-6): drain between frames, before this frame's body.
		rt.driveDrain()
		// Render acknowledgments release bounded frame credits even when the
		// drive seam is disabled. Drain them at the runtime control turn before
		// attempting another ordinary Wall frame.
		rt.driveCollectRenderCompletions()
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
		cleanupReserved := false
		if rt.renderPipeline != nil && rt.isSurfaceReady() {
			reserved = rt.renderPipeline.TryReserveFrame()
			if !reserved && rt.hasPendingHumanInput() {
				cleanupReserved = rt.renderPipeline.TryReserveCleanupFrame()
			}
			if !reserved && !cleanupReserved {
				// Keep serving control work while the bounded render window is
				// full. The next frame tick or control wake retries admission.
				continue
			}
		}
		rt.drivePrepareInputFrame()
		rt.runFrame(rt.clock.Now(), false, reserved, cleanupReserved)
	}
}

func (rt *Runtime) hasWallFrameWork() bool {
	if rt.wallWakePending.Load() || rt.bootstrapFramePending || len(rt.pendingEvents) != 0 ||
		len(rt.dirtyFacets) != 0 || len(rt.activeTickFacets) != 0 {
		return true
	}
	if step := rt.driveInputStep; step != nil && !step.preempted && step.completed < len(step.events) {
		return true
	}
	if rt.drivePort != nil && len(rt.drivePort.frameTokens) != 0 {
		return true
	}
	if rt.frameTimer != nil && len(rt.frameTimer.requestCh) != 0 {
		return true
	}
	return false
}

func (rt *Runtime) hasPendingHumanInput() bool {
	for _, event := range rt.pendingEvents {
		if isHumanInput(event) {
			return true
		}
	}
	return false
}

func (rt *Runtime) setPlatformFrameDemand(demand bool) {
	if rt != nil && rt.platformApp != nil {
		rt.platformApp.SetFrameDemand(demand)
	}
}

func (rt *Runtime) platformEventWake() <-chan struct{} {
	if rt == nil || rt.platformApp == nil || rt.platformApp.Events() == nil {
		return nil
	}
	return rt.platformApp.Events().Wake()
}

func (rt *Runtime) drainPlatformEvents() {
	events := rt.handleWindowEvents(rt.collectPlatformEvents())
	if len(events) != 0 {
		rt.pendingEvents = append(rt.pendingEvents, events...)
	}
}

func (rt *Runtime) waitForWallWork() bool {
	requestWake, vsyncWake := rt.frameTimer.idleChannels()
	var jobWake <-chan struct{}
	if rt.jobPool != nil {
		jobWake = rt.jobPool.FrontierWake()
	}
	var driveWake <-chan struct{}
	if rt.drivePort != nil {
		driveWake = rt.drivePort.wake
	}
	var renderWake <-chan RenderCompletion
	if rt.renderPipeline != nil {
		renderWake = rt.renderPipeline.Completions()
	}
	eventWake := rt.platformEventWake()
	var hoverTimer *time.Timer
	var hoverWake <-chan time.Time
	if deadline, ok := rt.inputSystem.NextHoverDeadline(); ok {
		delay := deadline.Sub(rt.clock.Now())
		if delay < 0 {
			delay = 0
		}
		hoverTimer = time.NewTimer(delay)
		hoverWake = hoverTimer.C
	}
	stopHoverTimer := func() {
		if hoverTimer != nil {
			hoverTimer.Stop()
		}
	}
	select {
	case <-rt.shutdownCh:
		stopHoverTimer()
		return false
	case <-requestWake:
		rt.wallWakePending.Store(true)
	case <-vsyncWake:
		// A pacing callback is only an opportunity to present already-eligible
		// work; it does not create application work by itself.
	case <-jobWake:
		rt.wallWakePending.Store(true)
	case completion := <-renderWake:
		rt.driveHandleRenderCompletion(completion)
	case <-eventWake:
		rt.drainPlatformEvents()
		if len(rt.pendingEvents) != 0 {
			rt.wallWakePending.Store(true)
		}
	case <-driveWake:
		rt.driveDrain()
	case <-hoverWake:
		rt.wallWakePending.Store(true)
	}
	stopHoverTimer()
	return !isChannelClosed(rt.shutdownCh)
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
		// Platform queues are polled before consuming an authorized frame token:
		// a parked controller must yield even when several advances are queued.
		parkedEvents := rt.collectPlatformEvents()
		if len(parkedEvents) != 0 {
			human := false
			for _, event := range parkedEvents {
				if isHumanInput(event) {
					human = true
					break
				}
			}
			if human {
				rt.preemptDriveForHuman()
				kept := parkedEvents[:0]
				for _, event := range parkedEvents {
					if eventSource(event) != platform.SourceDrive {
						kept = append(kept, event)
					}
				}
				rt.pendingEvents = append(rt.pendingEvents, kept...)
				return true
			}
			rt.pendingEvents = append(rt.pendingEvents, parkedEvents...)
		}
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
		platformWake := time.NewTimer(10 * time.Millisecond)
		select {
		case <-rt.shutdownCh:
			platformWake.Stop()
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			return false
		case completion := <-completions:
			platformWake.Stop()
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			rt.driveHandleRenderCompletion(completion)
		case <-jobWake:
			platformWake.Stop()
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			// Recheck the frozen frontier below; worker completion never
			// commits app state outside phase 1.
		case <-wake:
			platformWake.Stop()
			if leaseTimer != nil {
				leaseTimer.Stop()
			}
			rt.driveDrain()
		case <-leaseWake:
			platformWake.Stop()
			rt.drivePort.expireBootstrapLease()
		case <-platformWake.C:
			events := rt.collectPlatformEvents()
			human := false
			for _, event := range events {
				if isHumanInput(event) {
					human = true
					break
				}
			}
			if human {
				if rt.driveInputStep != nil {
					rt.preemptInputStep()
				}
				kept := events[:0]
				for _, event := range events {
					if eventSource(event) != platform.SourceDrive {
						kept = append(kept, event)
					}
				}
				rt.pendingEvents = append(rt.pendingEvents, kept...)
				if rt.drivePort != nil {
					rt.drivePort.releaseTimeOwner(true)
				}
				return true
			}
			if len(events) != 0 {
				rt.pendingEvents = append(rt.pendingEvents, events...)
				return true
			}
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
		rt.installRecoveryHooks()
		rt.renderPipeline.start()
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
	if rt.focusManager != nil {
		rt.focusManager.Reset()
	}
	rt.jobPool.Shutdown()
	rt.clearPhase1TickHooks()
	rt.rootStyleSubs.Release()
	if rt.renderPipeline != nil {
		// Native submission may be blocked. The render owner retains its page
		// lease and performs backend destruction when it eventually returns;
		// control shutdown remains bounded while that charge stays live.
		rt.renderPipeline.shutdown(2 * time.Second)
	}
	rt.releaseFrameViews()
	rt.config.StoreScope.Close()
	if rt.config.TextService != nil {
		_ = rt.config.TextService.Close()
	}
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
	if err := rt.liveIndex.validateNewTree(child); err != nil {
		panic(err)
	}
	if parentBase.State() != facet.StateCreated && !rt.liveIndex.contains(parentBase.ID()) {
		panic(fmt.Sprintf("runtime: parent facet %d is not live in this runtime", parentBase.ID()))
	}
	parentBase.AddChildRuntime(childBase)
	rt.projectionSystem.InvalidateTopology()
	rt.focusDirty = true
	rt.layerTreeDirty = true
	if rt.childAttachments == nil {
		rt.childAttachments = make(map[facet.FacetID]facet.Attachment)
	}
	rt.childAttachments[childBase.ID()] = attachment
	if parentBase.State() == facet.StateCreated {
		return
	}
	func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				delete(rt.childAttachments, childBase.ID())
				if childBase.Parent() == parentBase {
					parentBase.RemoveChild(childBase)
				} else if childBase.State() != facet.StateDisposed {
					facet.Dispose(child)
				}
				panic(recovered)
			}
		}()
		rt.attachTree(child)
		rt.activateTree(child)
	}()
	if rt.projectionLayers == nil {
		rt.projectionLayers = make(map[facet.FacetID]facet.ProjectionLayer)
	}
	parentBase.InvalidateWithSource(facet.DirtyLayout, "runtime.AddFacet")
	rt.recordInvalidation(parentBase.ID(), facet.DirtyLayout, "runtime.AddFacet")
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
	rt.projectionSystem.InvalidateTopology()
	rt.focusDirty = true
	rt.layerTreeDirty = true
	rt.invalidateDriveObservation()
	if rt.childAttachments != nil {
		delete(rt.childAttachments, child.Base().ID())
	}
	if rt.projectionLayers != nil {
		delete(rt.projectionLayers, child.Base().ID())
	}
	parent.InvalidateWithSource(facet.DirtyLayout, "runtime.RemoveFacet")
	rt.recordInvalidation(parent.ID(), facet.DirtyLayout, "runtime.RemoveFacet")
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
	rt.invalidateDriveObservation()
	rt.childAttachments[child.Base().ID()] = attachment
	rt.projectionSystem.InvalidateTopology()
	rt.focusDirty = true
	rt.layerTreeDirty = true
	if parent := child.Base().Parent(); parent != nil {
		parent.InvalidateWithSource(facet.DirtyLayout, "runtime.UpdateChildAttachment")
		rt.recordInvalidation(parent.ID(), facet.DirtyLayout, "runtime.UpdateChildAttachment")
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
