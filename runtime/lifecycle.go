package runtime

import (
	"fmt"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

const debugDisableSurfaceCreated = false

func (rt *Runtime) RunOneFrame() {

	_ = rt.start()
	rt.runFrame(time.Now(), true)
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
		now := rt.frameTimer.Wait()
		if runtimeTraceActive() {
			runtimeTracef("Run loop got frame tick frame=%d now=%s", rt.frameNumber+1, now.Format(time.RFC3339Nano))
		}
		rt.runFrame(now, false)
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
	if rt.renderPipeline != nil && rt.renderPipeline.backend != nil {
		rt.renderPipeline.backend.Destroy()
	}
	store.SetSignalQueueHook(nil)
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
