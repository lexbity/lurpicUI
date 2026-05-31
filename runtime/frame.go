package runtime

import (
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
)

func (rt *Runtime) runFrame(now time.Time, waitForRender bool) {

	if rt.isPaused() {
		return
	}
	rt.frameNumber++
	stats := diagnostics.FrameStats{FrameNumber: rt.frameNumber}

	committed, discarded := rt.drainJobResults()
	stats.JobsCommitted = committed
	stats.JobsDiscarded = discarded

	// Drain completed GPU upload results from the previous frame.
	// UploadQueue.DrainBudget runs on the render thread (triggered by
	// frame submission), so results become available here on the next frame.
	rt.drainGPUUploadResults()

	newEvents := rt.collectPlatformEvents()
	rt.pendingEvents = append(rt.pendingEvents, newEvents...)
	rt.pendingEvents = rt.handleWindowEvents(rt.pendingEvents)
	if drainer, ok := rt.assetManager.(interface{ DrainInvalidations() int }); ok {
		if count := drainer.DrainInvalidations(); count > 0 && rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
	}

	hoverEvents := rt.inputSystem.TickHover(now)
	dt := time.Duration(0)
	if rt.frameTimer != nil && !rt.frameTimer.lastFrame.IsZero() {
		dt = now.Sub(rt.frameTimer.lastFrame)
	}
	rt.runPhase1TickHooks(dt)
	rt.tickFacets(dt)

	currentHitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	dismissalEvents := rt.dismissalEventsForPointerPresses(rt.pendingEvents, currentHitMap)
	routedEvents := rt.inputSystem.Process(rt.pendingEvents, currentHitMap, rt.root)
	rt.pendingEvents = rt.pendingEvents[:0]
	routedEvents = append(dismissalEvents, routedEvents...)
	routedEvents = append(routedEvents, hoverEvents...)
	for _, re := range routedEvents {
		_ = input.Deliver(re, rt.root)
	}
	rt.deliverSignals()
	select {
	case <-rt.shutdownCh:
		rt.lastStats = stats
		return
	default:
	}

	dirtySnapshot := rt.copyDirtyFacets()
	stats.DirtyFacets = len(dirtySnapshot)

	layoutStart := time.Now()
	if rt.hasLayoutDirty() {
		w, h := rt.windowSize()
		rt.runLayoutPass(gfx.Size{W: float32(w), H: float32(h)})
	}
	stats.LayoutDuration = time.Since(layoutStart)

	layerStart := time.Now()
	phaseStats := rt.resolveLayerTree()
	stats.LayoutResolveDuration = time.Since(layerStart)
	stats.LayoutDuration += stats.LayoutResolveDuration
	stats.LayerResolutionDuration = phaseStats.specResolution
	stats.AnchorExportDuration = phaseStats.anchorExport
	stats.StructuralMeasureDuration = phaseStats.structuralMeasure
	stats.LayerBoundsDuration = phaseStats.layerBoundsResolution
	stats.ArrangeDuration = phaseStats.arrange

	projStart := time.Now()
	if rt.focusManager != nil {
		rt.focusManager.RebuildTabOrder(rt.root)
	}
	rt.syncFocusTraps()
	rt.projectionSystem.SetRuntime(rt)

	rt.projectionInProgress.Store(true)
	frameOut := rt.projectionSystem.Run(rt.root, projection.FrameInfo{
		Number:    rt.frameNumber,
		DeltaTime: now.Sub(rt.frameTimer.lastFrame),
		WallTime:  now,
	})
	rt.projectionInProgress.Store(false)

	stats.ProjectDuration = time.Since(projStart)
	stats.ProjectedFacets = rt.projectionSystem.ProjectedFacets
	stats.CacheHits = rt.projectionSystem.CacheHits
	if frameOut != nil {
		stats.RenderBatchCount = len(frameOut.RenderBatchs)
	}

	renderStart := time.Now()
	if rt.renderPipeline != nil && frameOut != nil && rt.isSurfaceReady() {
		windowFrames := rt.assembleWindowFrames(frameOut, dirtySnapshot)
		rt.lastWindowFrames = windowFrames
		frame := windowFrames[windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary})]
		if frame == nil {
			frame = &render.Frame{}
		}
		if diag := rt.diagnosticsHook(); diag != nil {
			if oi, ok := diag.(overlayInjector); ok {
				oi.InjectOverlay(frame, stats)
			}
		}
		if waitForRender {
			rt.renderPipeline.SubmitAndWait(frame)
		} else {
			rt.renderPipeline.Submit(frame)
		}
		// First-frame marker: CI smoke tests grep logcat for this string to
		// verify the app launched and rendered at least one frame. The marker
		// fires exactly once (frameNumber 1) and is emitted regardless of
		// platform so it works on both Android (logcat) and desktop (stderr).
		if rt.frameNumber == 1 {
			rt.log.Info("LURPIC_FIRST_FRAME",
				"frame", rt.frameNumber,
				"batches", stats.RenderBatchCount,
			)
		}
	}
	stats.RenderDuration = time.Since(renderStart)
	rt.updateIMECursorRect()

	// Check for device generation change (device lost + recreate).
	// When the generation bumps, all GPU-cached texture IDs become invalid
	// and are lazily re-uploaded on next use.
	rt.checkDeviceGeneration()

	// Collect asset manager diagnostics for the frame.
	if rt.assetManager != nil {
		astats := rt.assetManager.Stats()
		stats.AssetTotalEntries = astats.TotalEntries
		stats.AssetLoadingEntries = astats.LoadingEntries
		stats.AssetReadyEntries = astats.ReadyEntries
		stats.AssetPartialEntries = astats.PartialEntries
		stats.AssetFailedEntries = astats.FailedEntries
		stats.AssetCPUUsedBytes = astats.CPUUsedBytes
		stats.AssetCPUBudgetBytes = astats.CPUBudgetBytes
		stats.AssetGPUUsedBytes = astats.GPUUsedBytes
		stats.AssetGPUBudgetBytes = astats.GPUBudgetBytes
		stats.AssetEvictionsThisFrame = astats.EvictionsThisFrame
		stats.AssetUploadsThisFrame = astats.UploadsThisFrame
		stats.AssetJobsInFlight = astats.JobsInFlight
		stats.AssetCacheHitRate = astats.CacheHitRate
	}

	rt.lastStats = stats
	if diag := rt.diagnosticsHook(); diag != nil {
		diag.OnFrame(stats)
	}
	if len(dirtySnapshot) > 0 && rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	rt.dirtySources = make(map[facet.FacetID]string)
}

func (rt *Runtime) withDismissalEvents(routed []input.RoutedEvent, hitMap *projection.HitMap) []input.RoutedEvent {
	if hitMap == nil || len(routed) == 0 {
		return routed
	}
	out := make([]input.RoutedEvent, 0, len(routed)*2)
	for _, re := range routed {
		if press, ok := re.Event.(input.PointerPressEvent); ok {
			out = append(out, rt.dismissalEventsForPress(re.Target, press, hitMap)...)
		}
		out = append(out, re)
	}
	return out
}

func (rt *Runtime) dismissalEventsForPointerPresses(events []platform.Event, hitMap *projection.HitMap) []input.RoutedEvent {
	if hitMap == nil || len(events) == 0 {
		return nil
	}
	out := make([]input.RoutedEvent, 0)
	for _, ev := range events {
		press, ok := ev.(platform.EventPointer)
		if !ok || press.Kind != platform.PointerPress {
			continue
		}
		targetID := facet.FacetID(0)
		markID := facet.MarkID(0)
		if hit := hitMap.HitTest(gfx.Point{X: press.Position.X, Y: press.Position.Y}); hit != nil {
			targetID = hit.FacetID
			markID = hit.MarkID
		}
		out = append(out, rt.dismissalEventsForPress(targetID, input.PointerPressEvent{
			Position:  gfx.Point{X: press.Position.X, Y: press.Position.Y},
			ScreenPos: gfx.Point{X: press.Position.X, Y: press.Position.Y},
			Button:    press.Button,
			Modifiers: press.Modifiers,
			MarkID:    markID,
		}, hitMap)...)
	}
	return out
}

func (rt *Runtime) dismissalEventsForPress(target facet.FacetID, press input.PointerPressEvent, hitMap *projection.HitMap) []input.RoutedEvent {
	if hitMap == nil {
		return nil
	}
	entries := hitMap.Entries()
	if len(entries) == 0 {
		return nil
	}
	targetOrder := int32(0)
	targetLayerID := facet.LayerID(0)
	for _, entry := range entries {
		if entry.FacetID == target {
			targetOrder = int32(entry.LayerOrder)
			targetLayerID = facet.LayerID(entry.LayerID)
			break
		}
	}
	out := make([]input.RoutedEvent, 0, len(entries))
	for _, entry := range entries {
		layer, ok := rt.projectionLayers[entry.FacetID]
		if !ok {
			continue
		}
		scope := layer.Dismissal
		if !scope.Enabled || !dismissalTriggerEnabled(scope, facet.DismissalTriggerPointer) {
			continue
		}
		if entry.FacetID == target {
			continue
		}
		if !dismissalBehindContains(scope.BehindOrders, targetOrder) {
			continue
		}
		out = append(out, input.RoutedEvent{
			Target: entry.FacetID,
			Event: input.DismissEvent{
				Trigger:    facet.DismissalTriggerPointer,
				ScreenPos:  press.ScreenPos,
				HitFacetID: target,
				HitMarkID:  press.MarkID,
				HitLayerID: targetLayerID,
				HitOrder:   int(targetOrder),
			},
		})
	}
	return out
}

func dismissalTriggerEnabled(scope facet.DismissalScope, trigger facet.DismissalTrigger) bool {
	if !scope.Enabled {
		return false
	}
	if scope.Triggers == 0 {
		return trigger == facet.DismissalTriggerPointer
	}
	switch trigger {
	case facet.DismissalTriggerPointer:
		return scope.Triggers&facet.DismissalTriggerSetPointer != 0
	case facet.DismissalTriggerKey:
		return scope.Triggers&facet.DismissalTriggerSetKey != 0
	case facet.DismissalTriggerFocusLoss:
		return scope.Triggers&facet.DismissalTriggerSetFocusLoss != 0
	default:
		return false
	}
}

func dismissalBehindContains(r facet.OrderRange, order int32) bool {
	return order >= r.Min && order < r.Max
}

func (rt *Runtime) handleResize(w, h int) {

	if rt.window != nil {
		if resizable, ok := rt.window.(interface{ Resize(int, int) }); ok {
			resizable.Resize(w, h)
		}
	}
	rt.markTreeDirty(rt.root, facet.DirtyAll)
	if newScale := rt.effectiveContentScale(); newScale != rt.contentScale {
		rt.contentScale = newScale
		rt.markTreeDirty(rt.root, facet.DirtyAll)
	}
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func (rt *Runtime) updateIMECursorRect() {
	if rt.focusManager == nil {
		rt.setIMEVisible(false)
		return
	}
	focused := rt.focusManager.FocusedImpl()
	if focused == nil || focused.Base() == nil {
		rt.setIMEVisible(false)
		return
	}
	tr := focused.Base().TextRole()
	if tr == nil || !tr.IMEEnabled {
		rt.setIMEVisible(false)
		return
	}
	rt.setIMEVisible(true)
	if rt.window == nil {
		return
	}
	if !tr.CaretVisible || tr.Layout == nil {
		rt.window.SetIMECursorRect(gfx.Rect{})
		return
	}
	caret := tr.Layout.CaretRect(tr.CaretPosition)
	offset := gfx.Point{}
	if lr := focused.Base().LayoutRole(); lr != nil {
		offset = lr.ArrangedBounds.Min
	}
	rt.window.SetIMECursorRect(gfx.RectFromXYWH(
		caret.Min.X+offset.X,
		caret.Min.Y+offset.Y,
		caret.Width(),
		caret.Height(),
	))
}

func (rt *Runtime) setIMEVisible(visible bool) {

	if visible == rt.imeVisible {
		return
	}
	if cap, ok := platform.IMECapableOf(rt.platformApp); ok && cap != nil {
		if visible {
			cap.ShowSoftKeyboard()
		} else {
			cap.HideSoftKeyboard()
		}
	}
	rt.imeVisible = visible
}

func (rt *Runtime) drainGPUUploadResults() {
	if rt.assetManager == nil {
		return
	}
	if d, ok := rt.assetManager.(interface{ DrainUploadResults() int }); ok {
		d.DrainUploadResults()
	}
}

func (rt *Runtime) drainJobResults() (committed int, discarded int) {

	if rt.assetManager != nil {
		if count := rt.assetManager.DrainCompleted(); count > 0 {
			committed += count
			if rt.frameTimer != nil {
				rt.frameTimer.RequestFrame()
			}
		}
	}
	if rt.jobPool == nil {
		return committed, 0
	}
	results := rt.jobPool.Drain()
	for _, r := range results {
		if r.Cancelled() {
			discarded++
			continue
		}
		if err := r.Err(); err != nil {
			rt.log.Debug("job result error", "jobID", r.JobID(), "err", err)
			discarded++
			continue
		}
		committed++
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
	}
	return committed, discarded
}

func (rt *Runtime) tickFacets(dt time.Duration) {
	rt.walkActive(rt.root, func(f facet.FacetImpl) {
		tr := f.Base().TickRole()
		if tr == nil || !tr.IsActive() {
			return
		}
		if tr.OnTick != nil {
			tr.OnTick(dt)
		}
		tr.Reset()
	})
}

func (rt *Runtime) walkActive(root facet.FacetImpl, fn func(facet.FacetImpl)) {
	if root == nil || fn == nil {
		return
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		if node.Base().State() == facet.StateActive {
			fn(node)
		}
		children := node.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
}

func (rt *Runtime) windowSize() (int, int) {
	if rt.window == nil {
		return 0, 0
	}
	return rt.window.Size()
}

func (rt *Runtime) effectiveContentScale() float32 {

	if rt.config.ContentScale > 0 {
		return rt.config.ContentScale
	}
	if rt.window != nil {
		if scale := rt.window.ContentScale(); scale > 0 {
			return scale
		}
	}
	return 1
}
