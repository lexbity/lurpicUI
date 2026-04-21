package runtime

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	projectedpolicy "codeburg.org/lexbit/lurpicui/layout/projected"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/store"
)

type pendingSignal struct{ deliver func() }

// Runtime drives the engine pipeline.
type Runtime struct {
	config Config

	layoutSystem     *layout.System
	projectionSystem *projection.System
	inputSystem      *input.System
	focusManager     *facet.FocusManager
	jobPool          *job.Pool
	renderPipeline   *RenderPipeline
	policyRegistry   *PolicyRegistry

	platformApp platform.App
	window      platform.Window
	root        facet.FacetImpl

	frameNumber  uint64
	frameTimer   *FrameTimer
	contentScale float32

	dirtyFacets      map[facet.FacetID]facet.DirtyFlags
	dirtySources     map[facet.FacetID]string
	childAttachments map[facet.FacetID]layout.ChildAttachment
	layerStates      map[facet.FacetID]*resolvedLayerSet
	anchorCaches     map[facet.FacetID]*layout.AnchorPositionCache
	projectionLayers map[facet.FacetID]facet.ProjectionLayer
	lastHitTrace     diagnostics.HitTestTrace
	hitTraceEnabled  bool
	pendingEvents    []platform.Event
	signalQueue      []pendingSignal
	diagMu           sync.RWMutex
	diag             DiagnosticsHook
	rootStyleContext any
	phase1HooksMu    sync.RWMutex
	phase1Hooks      []func(time.Duration)

	shutdownCh chan struct{}
	doneCh     chan struct{}

	lastStats diagnostics.FrameStats
	log       Logger

	startOnce  sync.Once
	shutdownMu sync.Mutex
	started    bool
	stopping   bool
}

// New constructs a runtime with the provided config and roots.
func New(config Config, platformApp platform.App, window platform.Window, backend render.Backend, root facet.FacetImpl) (*Runtime, error) {
	if config.TargetFPS <= 0 {
		return nil, errors.New("runtime: TargetFPS must be greater than zero")
	}
	if config.FontRegistry == nil {
		return nil, errors.New("runtime: FontRegistry is required")
	}
	if root == nil {
		return nil, errors.New("runtime: root facet is required")
	}
	if backend == nil {
		return nil, errors.New("runtime: backend is required")
	}
	if config.WorkerCount <= 0 {
		config.WorkerCount = DefaultConfig().WorkerCount
	}
	if config.Logger == nil {
		config.Logger = NopLogger{}
	}
	rt := &Runtime{
		config:           config,
		layoutSystem:     layout.NewSystem(),
		projectionSystem: projection.NewSystem(),
		inputSystem:      input.NewSystem(config.GestureConfig),
		focusManager:     facet.NewFocusManager(),
		jobPool:          job.NewPool(config.WorkerCount),
		renderPipeline:   newRenderPipeline(backend),
		policyRegistry:   DefaultRegistry(),
		platformApp:      platformApp,
		window:           window,
		root:             root,
		frameTimer:       NewFrameTimer(config.TargetFPS),
		dirtyFacets:      make(map[facet.FacetID]facet.DirtyFlags),
		dirtySources:     make(map[facet.FacetID]string),
		childAttachments: make(map[facet.FacetID]layout.ChildAttachment),
		layerStates:      make(map[facet.FacetID]*resolvedLayerSet),
		anchorCaches:     make(map[facet.FacetID]*layout.AnchorPositionCache),
		projectionLayers: make(map[facet.FacetID]facet.ProjectionLayer),
		shutdownCh:       make(chan struct{}),
		doneCh:           make(chan struct{}),
		log:              config.Logger,
		diag:             config.DiagnosticsHook,
	}
	rt.inputSystem.SetFocusManager(rt.focusManager)
	return rt, nil
}

// RunOneFrame executes a single frame pass synchronously.
func (rt *Runtime) RunOneFrame() {
	if rt == nil {
		return
	}
	_ = rt.start()
	rt.runFrame(time.Now(), true)
}

// Run executes frames until Shutdown is requested.
func (rt *Runtime) Run() error {
	if rt == nil {
		return nil
	}
	if err := rt.start(); err != nil {
		return err
	}
	for {
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
		rt.runFrame(now, false)
	}
}

// Shutdown stops the runtime and waits for cleanup.
func (rt *Runtime) Shutdown() {
	if rt == nil {
		return
	}
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
	select {
	case <-rt.shutdownCh:
	default:
		close(rt.shutdownCh)
	}
	<-rt.doneCh
}

// LastFrameStats reports the most recent frame summary.
func (rt *Runtime) LastFrameStats() diagnostics.FrameStats {
	if rt == nil {
		return diagnostics.FrameStats{}
	}
	return rt.lastStats
}

func (rt *Runtime) start() error {
	if rt == nil {
		return nil
	}
	var startErr error
	rt.startOnce.Do(func() {
		if !syncutil.OnRuntimeThread() {
			syncutil.RegisterRuntimeThread()
		}
		store.SetSignalQueueHook(rt.queueSignal)
		go (&renderThread{pipeline: rt.renderPipeline}).run()
		rt.jobPool.Start()
		rt.contentScale = rt.effectiveContentScale()
		rt.attachTree(rt.root)
		rt.activateTree(rt.root)
		rt.markTreeDirty(rt.root, facet.DirtyAll)
		go rt.superviseShutdown()
		rt.shutdownMu.Lock()
		rt.started = true
		rt.shutdownMu.Unlock()
	})
	return startErr
}

func (rt *Runtime) superviseShutdown() {
	defer close(rt.doneCh)
	<-rt.shutdownCh
	_ = rt.shutdown()
}

func (rt *Runtime) shutdown() error {
	if rt == nil {
		return nil
	}
	rt.jobPool.CancelAll()
	rt.disposeTree(rt.root)
	rt.jobPool.Shutdown()
	rt.clearPhase1TickHooks()
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

func (rt *Runtime) runFrame(now time.Time, waitForRender bool) {
	if rt == nil {
		return
	}
	rt.frameNumber++
	stats := diagnostics.FrameStats{FrameNumber: rt.frameNumber}

	committed, discarded := rt.drainJobResults()
	stats.JobsCommitted = committed
	stats.JobsDiscarded = discarded

	newEvents := rt.collectPlatformEvents()
	rt.pendingEvents = append(rt.pendingEvents, newEvents...)
	rt.pendingEvents = rt.handleWindowEvents(rt.pendingEvents)

	hoverEvents := rt.inputSystem.TickHover(now)
	dt := time.Duration(0)
	if rt.frameTimer != nil && !rt.frameTimer.lastFrame.IsZero() {
		dt = now.Sub(rt.frameTimer.lastFrame)
	}
	rt.runPhase1TickHooks(dt)
	rt.tickFacets(dt)

	currentHitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	routedEvents := rt.inputSystem.Process(rt.pendingEvents, currentHitMap, rt.root)
	rt.pendingEvents = rt.pendingEvents[:0]
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
	layoutStart := time.Now()
	if rt.hasLayoutDirty() {
		rt.markLayoutDirtyFacets()
		w, h := rt.windowSize()
		rt.layoutSystem.Run(gfx.Size{W: float32(w), H: float32(h)})
	}
	stats.LayoutDuration = time.Since(layoutStart)
	stats.DirtyFacets = len(dirtySnapshot)

	layerStart := time.Now()
	phaseStats := rt.resolveLayerTree()
	stats.LayoutResolveDuration = time.Since(layerStart)
	stats.LayoutDuration += stats.LayoutResolveDuration
	stats.LayerSpecDuration = phaseStats.specResolution
	stats.AnchorExportDuration = phaseStats.anchorExport
	stats.StructuralMeasureDuration = phaseStats.structuralMeasure
	stats.LayerBoundsDuration = phaseStats.layerBoundsResolution
	stats.ArrangeDuration = phaseStats.arrange

	projStart := time.Now()
	rt.projectionSystem.SetRuntime(rt)
	frameOut := rt.projectionSystem.Run(rt.root, projection.FrameInfo{
		Number:    rt.frameNumber,
		DeltaTime: now.Sub(rt.frameTimer.lastFrame),
		WallTime:  now,
	})
	if rt.focusManager != nil {
		rt.focusManager.RebuildTabOrder(rt.root)
	}
	stats.ProjectDuration = time.Since(projStart)
	stats.ProjectedFacets = rt.projectionSystem.ProjectedFacets
	stats.CacheHits = rt.projectionSystem.CacheHits
	if frameOut != nil {
		stats.RenderBatchCount = len(frameOut.RenderBatchs)
	}

	renderStart := time.Now()
	if rt.renderPipeline != nil && frameOut != nil {
		frame := rt.assembleFrame(frameOut, dirtySnapshot)
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
	}
	stats.RenderDuration = time.Since(renderStart)
	rt.updateIMECursorRect()

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

func (rt *Runtime) collectPlatformEvents() []platform.Event {
	if rt == nil || rt.platformApp == nil || rt.platformApp.Events() == nil {
		return nil
	}
	return rt.platformApp.Events().Poll()
}

func (rt *Runtime) handleWindowEvents(events []platform.Event) []platform.Event {
	if rt == nil || len(events) == 0 {
		return events[:0]
	}
	out := events[:0]
	for _, ev := range events {
		switch e := ev.(type) {
		case platform.EventWindowClose:
			rt.initiateShutdown()
		case platform.EventWindowResize:
			rt.handleResize(e.Width, e.Height)
		case platform.EventWindowFocus:
			if !e.Focused {
				rt.inputSystem.ClearPointerState()
				if rt.focusManager != nil {
					rt.focusManager.ClearFocus()
				}
				rt.inputSystem.ClearFocus()
			}
		default:
			out = append(out, ev)
		}
	}
	return out
}

func (rt *Runtime) handleResize(w, h int) {
	if rt == nil {
		return
	}
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
	if rt == nil || rt.window == nil || rt.focusManager == nil {
		return
	}
	focused := rt.focusManager.FocusedImpl()
	if focused == nil || focused.Base() == nil {
		return
	}
	tr := focused.Base().TextRole()
	if tr == nil || !tr.CaretVisible || tr.Layout == nil {
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

// AddFacet attaches a new child facet at runtime with layer metadata.
func (rt *Runtime) AddFacet(parent, child facet.FacetImpl, attachment layout.ChildAttachment) {
	if rt == nil || parent == nil || child == nil {
		return
	}
	parentBase := parent.Base()
	childBase := child.Base()
	if parentBase == nil || childBase == nil {
		return
	}
	parentBase.AddChildRuntime(childBase)
	if rt.childAttachments == nil {
		rt.childAttachments = make(map[facet.FacetID]layout.ChildAttachment)
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

// RemoveFacet detaches and disposes a runtime child facet.
func (rt *Runtime) RemoveFacet(child facet.FacetImpl) {
	if rt == nil || child == nil || child.Base() == nil {
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

func (rt *Runtime) initiateShutdown() {
	if rt == nil {
		return
	}
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
	select {
	case <-rt.shutdownCh:
	default:
		close(rt.shutdownCh)
	}
}

func (rt *Runtime) drainJobResults() (committed int, discarded int) {
	if rt == nil || rt.jobPool == nil {
		return 0, 0
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

func (rt *Runtime) deliverSignals() {
	const maxIterations = 16
	for i := 0; i < maxIterations && len(rt.signalQueue) > 0; i++ {
		batch := rt.signalQueue
		rt.signalQueue = rt.signalQueue[:0]
		for _, s := range batch {
			if s.deliver != nil {
				s.deliver()
			}
		}
	}
}

func (rt *Runtime) queueSignal(deliver func()) {
	if rt == nil || deliver == nil {
		return
	}
	rt.signalQueue = append(rt.signalQueue, pendingSignal{deliver: deliver})
}

func (rt *Runtime) copyDirtyFacets() map[facet.FacetID]facet.DirtyFlags {
	if rt == nil || len(rt.dirtyFacets) == 0 {
		return nil
	}
	out := make(map[facet.FacetID]facet.DirtyFlags, len(rt.dirtyFacets))
	for id, flags := range rt.dirtyFacets {
		if flags != 0 {
			out[id] = flags
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (rt *Runtime) attachTree(root facet.FacetImpl) {
	if rt == nil || root == nil {
		return
	}
	facet.Attach(root, facet.AttachContext{Runtime: rt})
}

func (rt *Runtime) activateTree(root facet.FacetImpl) {
	if rt == nil || root == nil {
		return
	}
	facet.Activate(root)
}

func (rt *Runtime) disposeTree(root facet.FacetImpl) {
	if rt == nil || root == nil {
		return
	}
	facet.Dispose(root)
}

func (rt *Runtime) markTreeDirty(root facet.FacetImpl, flags facet.DirtyFlags) {
	if rt == nil || root == nil {
		return
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		base := node.Base()
		base.InvalidateWithSource(flags, "runtime.markTreeDirty")
		rt.dirtyFacets[base.ID()] = flags
		rt.dirtySources[base.ID()] = "runtime.markTreeDirty"
		children := base.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
}

func (rt *Runtime) hasLayoutDirty() bool {
	for _, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout != 0 {
			return true
		}
	}
	return false
}

func (rt *Runtime) markLayoutDirtyFacets() {
	if rt == nil || rt.layoutSystem == nil {
		return
	}
	for id, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout == 0 {
			continue
		}
		if f := rt.findFacetByID(rt.root, id); f != nil {
			rt.layoutSystem.MarkDirty(f)
		}
	}
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
	if rt == nil || root == nil || fn == nil {
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
	if rt == nil || rt.window == nil {
		return 0, 0
	}
	return rt.window.Size()
}

func (rt *Runtime) effectiveContentScale() float32 {
	if rt == nil {
		return 1
	}
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

func convertFrame(frame *projection.FrameOutput) *render.Frame {
	return assembleFrameWithLayers(frame, nil, nil)
}

func (rt *Runtime) assembleFrame(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) *render.Frame {
	return assembleFrameWithLayers(output, dirtySnapshot, rt)
}

type frameLayerResolver interface {
	ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool)
	ResolveChildAttachment(id facet.FacetID) (layout.ChildAttachment, bool)
}

type layoutPhaseStats struct {
	specResolution        time.Duration
	anchorExport          time.Duration
	structuralMeasure     time.Duration
	layerBoundsResolution time.Duration
	arrange               time.Duration
}

type frameBatchItem struct {
	order int
	z     int
	clip  gfx.Rect
	index int
	batch render.RenderBatch
}

func assembleFrameWithLayers(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags, resolver frameLayerResolver) *render.Frame {
	if output == nil {
		return &render.Frame{}
	}
	items := make([]frameBatchItem, 0, len(output.RenderBatchs))
	for i, RenderBatch := range output.RenderBatchs {
		cmds := gfx.CommandList{}
		if !RenderBatch.Transform.IsIdentity() {
			cmds.Add(gfx.PushTransform{Matrix: RenderBatch.Transform})
		}
		for _, cmd := range RenderBatch.Commands.Commands {
			cmds.Add(cmd)
		}
		if sel := output.SelectionGeometries[RenderBatch.FacetID]; sel != nil {
			selectionCmd := gfx.DrawSelectionRects{
				Rects: append([]gfx.Rect(nil), sel.SelectionRects...),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(64, 128, 255, 96)),
			}
			if sel.CaretVisible {
				selectionCmd.Rects = append(selectionCmd.Rects, sel.CaretRect)
			}
			if len(selectionCmd.Rects) > 0 {
				cmds.Add(selectionCmd)
			}
		}
		if !RenderBatch.Transform.IsIdentity() {
			cmds.Add(gfx.PopTransform{})
		}
		rb := render.RenderBatch{
			ID:          render.RenderBatchID(RenderBatch.FacetID),
			Bounds:      RenderBatch.Bounds,
			Opacity:     RenderBatch.Opacity,
			Commands:    cmds,
			CommandHash: hashutil.HashCommandList(cmds),
		}
		item := frameBatchItem{index: i, batch: rb}
		if resolver != nil {
			if layer, ok := resolver.ResolveProjectionLayer(RenderBatch.FacetID); ok {
				item.order = layer.RenderOrder
				item.clip = layer.ClipRect
			}
			if attachment, ok := resolver.ResolveChildAttachment(RenderBatch.FacetID); ok {
				item.z = attachment.ZPriority
			}
		}
		items = append(items, item)
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order != items[j].order {
			return items[i].order < items[j].order
		}
		if items[i].z != items[j].z {
			return items[i].z < items[j].z
		}
		return items[i].index < items[j].index
	})
	out := &render.Frame{
		RenderBatchs: make([]render.RenderBatch, 0, len(items)),
		FramePacket: render.FramePacket{
			Layers: make([]render.LayeredBatch, 0, len(items)),
		},
	}
	hasLayer := false
	var currentOrder int
	var currentClip gfx.Rect
	for _, item := range items {
		out.RenderBatchs = append(out.RenderBatchs, item.batch)
		if !hasLayer || currentOrder != item.order || currentClip != item.clip {
			out.Layers = append(out.Layers, render.LayeredBatch{
				RenderOrder: item.order,
				ClipRect:    item.clip,
				Batches:     []render.RenderBatch{item.batch},
			})
			currentOrder = item.order
			currentClip = item.clip
			hasLayer = true
			continue
		}
		last := len(out.Layers) - 1
		out.Layers[last].Batches = append(out.Layers[last].Batches, item.batch)
	}
	out.DirtyRegions = computeDirtyRegions(output, dirtySnapshot)
	return out
}

func computeDirtyRegions(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) []gfx.Rect {
	if output == nil || len(output.RenderBatchs) == 0 {
		return nil
	}
	rects := make([]gfx.Rect, 0, len(output.RenderBatchs))
	for _, RenderBatch := range output.RenderBatchs {
		if dirtySnapshot != nil {
			if flags := dirtySnapshot[RenderBatch.FacetID]; flags == 0 {
				continue
			}
		}
		rects = append(rects, RenderBatch.Bounds)
	}
	if len(rects) == 0 {
		return nil
	}
	rects = renderutil.MergeRects(rects, 0.25)
	rects = renderutil.RemoveContained(rects)
	return rects
}

// RuntimeServices hooks used by facets during attach.
func (rt *Runtime) Schedule(j job.AnyJob) {
	if rt == nil || j == nil || rt.jobPool == nil {
		return
	}
	ownerID := facet.FacetID(j.OwnerID())
	_ = rt.jobPool.SubmitAny(j, func(result job.AnyResult) {
		f := rt.findFacetByID(rt.root, ownerID)
		if f == nil || f.Base() == nil {
			return
		}
		pr := f.Base().ProjectionRole()
		if pr == nil || pr.OnJobResult == nil {
			return
		}
		pr.OnJobResult(result)
		f.Base().InvalidateWithSource(facet.DirtyProjection, "job.OnJobResult")
		rt.dirtyFacets[ownerID] |= facet.DirtyProjection
		rt.dirtySources[ownerID] = "job.OnJobResult"
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
	})
}

func (rt *Runtime) CancelJob(id job.JobID) {
	if rt == nil || rt.jobPool == nil {
		return
	}
	rt.jobPool.CancelJob(id)
}

func (rt *Runtime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if rt == nil {
		return
	}
	rt.queueSignal(func() {
		rt.dirtyFacets[id] |= flags
		if source != "" {
			rt.dirtySources[id] = source
		}
		if rt.root != nil && rt.root.Base() != nil && rt.root.Base().ID() == id {
			rt.root.Base().InvalidateWithSource(flags, source)
		}
	})
}

// EnableDiagnostics replaces the runtime diagnostics hook.
func (rt *Runtime) EnableDiagnostics(d DiagnosticsHook) {
	if rt == nil {
		return
	}
	rt.diagMu.Lock()
	rt.diag = d
	rt.diagMu.Unlock()
	if hook, ok := d.(*diagnostics.Hook); ok {
		if hook.Inspector == nil {
			hook.Inspector = diagnostics.NewInspector(rt.root)
		}
		if hook.Inspector != nil {
			hook.Inspector.SetLayerSource(rt)
			hook.Inspector.SetAnchorSource(rt)
			hook.Inspector.SetHitTraceSource(rt)
		}
		hook.HitProbeSource = rt
	}
	rt.hitTraceEnabled = true
}

// Inspect runs fn with a synchronous inspector snapshot.
func (rt *Runtime) Inspect(fn func(*diagnostics.Inspector)) {
	if rt == nil || fn == nil {
		return
	}
	inspector := diagnostics.NewInspector(rt.root)
	inspector.SetLayerSource(rt)
	inspector.SetAnchorSource(rt)
	inspector.SetHitTraceSource(rt)
	fn(inspector)
}

func (rt *Runtime) diagnosticsHook() DiagnosticsHook {
	if rt == nil {
		return nil
	}
	rt.diagMu.RLock()
	defer rt.diagMu.RUnlock()
	return rt.diag
}

func (rt *Runtime) findFacetByID(root facet.FacetImpl, id facet.FacetID) facet.FacetImpl {
	if rt == nil || root == nil || root.Base() == nil {
		return nil
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		if node.Base().ID() == id {
			return node
		}
		children := node.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			child := children[i]
			next := facet.FacetImpl(child)
			if impl := child.Impl(); impl != nil {
				next = impl
			}
			stack = append(stack, next)
		}
	}
	return nil
}

func (rt *Runtime) resolveLayerTree() layoutPhaseStats {
	if rt == nil || rt.root == nil {
		return layoutPhaseStats{}
	}
	rt.projectionLayers = make(map[facet.FacetID]facet.ProjectionLayer)
	return rt.walkLayerTree(rt.root, gfx.Identity())
}

func (rt *Runtime) resolveAnchorExports() {
	if rt == nil || rt.root == nil {
		return
	}
	endGuard := syncutil.BeginAnchorExport()
	defer endGuard()
	visited := make(map[facet.FacetID]struct{})
	rt.walkAnchorExportTree(rt.root, visited)
	for parentID := range rt.anchorCaches {
		if _, ok := visited[parentID]; !ok {
			delete(rt.anchorCaches, parentID)
		}
	}
}

func (rt *Runtime) walkAnchorExportTree(node facet.FacetImpl, visited map[facet.FacetID]struct{}) {
	if rt == nil || node == nil || node.Base() == nil {
		return
	}
	stack := []facet.FacetImpl{node}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == nil || current.Base() == nil {
			continue
		}
		if visited != nil {
			visited[current.Base().ID()] = struct{}{}
		}
		rt.reconcileAnchorExports(current)
		children := current.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
}

func (rt *Runtime) reconcileAnchorExports(parent facet.FacetImpl) {
	if rt == nil || parent == nil || parent.Base() == nil {
		return
	}
	parentID := parent.Base().ID()
	state := rt.layerStates[parentID]
	cache := rt.anchorCaches[parentID]
	if state == nil && cache == nil {
		return
	}
	var layerKinds map[layout.LayerID]layout.PlacementMode
	if state != nil {
		layerKinds = make(map[layout.LayerID]layout.PlacementMode, len(state.specs))
		for _, spec := range state.specs {
			layerKinds[spec.ID] = spec.Placement
		}
	}
	anchorChildren := make(map[layout.AnchorID][]facet.FacetID)
	exporters := make([]facet.FacetImpl, 0)
	for _, childBase := range parent.Base().Children() {
		if childBase == nil {
			continue
		}
		attachment, ok := rt.childAttachments[childBase.ID()]
		if ok {
			layerID := attachment.LayerID
			if layerID == 0 && state != nil && len(state.specs) > 0 {
				layerID = state.specs[0].ID
			}
			if attachment.Placement.AnchorRef != "" && layerKinds != nil && layerKinds[layerID] == layout.PlacementAnchor {
				anchorChildren[attachment.Placement.AnchorRef] = append(anchorChildren[attachment.Placement.AnchorRef], childBase.ID())
			}
		}
		child := rt.findFacetByID(rt.root, childBase.ID())
		if child == nil {
			continue
		}
		if _, ok := child.(layout.AnchorExporter); ok {
			if attachment, ok := rt.childAttachments[childBase.ID()]; ok && state != nil {
				layerID := attachment.LayerID
				if layerID == 0 && len(state.specs) > 0 {
					layerID = state.specs[0].ID
				}
				if _, ok := state.resolvedLayer(layerID); ok {
					exporters = append(exporters, child)
				}
			}
		}
	}
	if len(anchorChildren) == 0 {
		if cache != nil && cache.Len() > 0 {
			_ = cache.Reset()
		}
		delete(rt.anchorCaches, parentID)
		return
	}
	if cache == nil {
		cache = layout.NewAnchorPositionCache()
		rt.anchorCaches[parentID] = cache
	}
	if len(exporters) == 0 {
		if cache.Len() > 0 {
			_ = cache.Reset()
			rt.markAnchorChildrenDirty(anchorChildren, "anchor exporter detached")
			if rt.frameTimer != nil {
				rt.frameTimer.RequestFrame()
			}
		}
		return
	}
	combined := make(layout.AnchorSet)
	for _, exporterFacet := range exporters {
		exporter := exporterFacet.(layout.AnchorExporter)
		attachment, ok := rt.childAttachments[exporterFacet.Base().ID()]
		if !ok {
			continue
		}
		layerID := attachment.LayerID
		if layerID == 0 && len(state.specs) > 0 {
			layerID = state.specs[0].ID
		}
		resolved, ok := state.resolvedLayer(layerID)
		if !ok {
			continue
		}
		ctx := layout.AnchorExportContext{
			ResolvedLayer: resolved,
			Viewport: layout.Viewport{
				Transform:   resolved.Transform,
				WorldBounds: resolved.Bounds,
			},
		}
		for id, pos := range exporter.ExportAnchors(ctx) {
			combined[id] = pos
		}
	}
	changes := cache.Replace(combined)
	if len(changes) == 0 {
		return
	}
	for _, change := range changes {
		if refs := anchorChildren[change.ID]; len(refs) > 0 {
			rt.markFacetGroupDirty(refs, facet.DirtyLayout, anchorChangeSource(change))
		}
	}
	if rt.frameTimer != nil {
		rt.frameTimer.RequestFrame()
	}
}

func anchorChangeSource(change layout.AnchorChange) string {
	if change.Removed {
		return fmt.Sprintf("anchor %q removed from %v", change.ID, change.Old)
	}
	if change.Old == (gfx.Point{}) {
		return fmt.Sprintf("anchor %q added at %v", change.ID, change.New)
	}
	return fmt.Sprintf("anchor %q moved %v -> %v", change.ID, change.Old, change.New)
}

func (rt *Runtime) markAnchorChildrenDirty(anchorChildren map[layout.AnchorID][]facet.FacetID, source string) {
	if rt == nil || len(anchorChildren) == 0 {
		return
	}
	ids := make([]facet.FacetID, 0)
	for _, refs := range anchorChildren {
		ids = append(ids, refs...)
	}
	rt.markFacetGroupDirty(ids, facet.DirtyLayout, source)
}

func (rt *Runtime) markFacetGroupDirty(ids []facet.FacetID, flags facet.DirtyFlags, source string) {
	if rt == nil {
		return
	}
	for _, id := range ids {
		rt.markFacetDirtyByID(id, flags, source)
	}
}

func (rt *Runtime) markFacetDirtyByID(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if rt == nil || id == 0 {
		return
	}
	rt.dirtyFacets[id] |= flags
	if source != "" {
		rt.dirtySources[id] = source
	}
	if f := rt.findFacetByID(rt.root, id); f != nil && f.Base() != nil {
		f.Base().InvalidateWithSource(flags, source)
	}
}

func (rt *Runtime) walkLayerTree(node facet.FacetImpl, accumulated gfx.Transform) layoutPhaseStats {
	if rt == nil || node == nil || node.Base() == nil {
		return layoutPhaseStats{}
	}
	var stats layoutPhaseStats
	type layerFrame struct {
		node      facet.FacetImpl
		transform gfx.Transform
	}
	stack := []layerFrame{{node: node, transform: accumulated}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.node == nil || frame.node.Base() == nil {
			continue
		}
		current := frame.transform
		if viewport := frame.node.Base().ViewportRole(); viewport != nil {
			current = current.Multiply(viewport.Transform)
		}
		if composer, ok := frame.node.(LayerComposer); ok {
			childStats := rt.resolveComposedLayers(frame.node, composer, current)
			stats.specResolution += childStats.specResolution
			stats.anchorExport += childStats.anchorExport
			stats.structuralMeasure += childStats.structuralMeasure
			stats.layerBoundsResolution += childStats.layerBoundsResolution
			stats.arrange += childStats.arrange
		}
		children := frame.node.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, layerFrame{node: children[i], transform: current})
		}
	}
	return stats
}

func (rt *Runtime) resolveComposedLayers(parent facet.FacetImpl, composer LayerComposer, accumulated gfx.Transform) layoutPhaseStats {
	if rt == nil || parent == nil || parent.Base() == nil || composer == nil {
		return layoutPhaseStats{}
	}
	var stats layoutPhaseStats
	specStart := time.Now()
	specs := composer.OnLayerSpecs()
	stats.specResolution += time.Since(specStart)
	if len(specs) == 0 {
		delete(rt.layerStates, parent.Base().ID())
		return stats
	}
	for _, spec := range specs {
		if err := layout.ValidateLayerSpec(spec); err != nil {
			panic(err)
		}
	}
	parentID := parent.Base().ID()
	state := rt.layerStates[parentID]
	if state == nil {
		state = &resolvedLayerSet{}
		rt.layerStates[parentID] = state
	}
	state.specs = append(state.specs[:0], specs...)
	state.layers = state.layers[:0]
	state.childCounts = state.childCounts[:0]
	parentBounds := gfx.Rect{}
	if lr := parent.Base().LayoutRole(); lr != nil {
		if !lr.ArrangedBounds.IsEmpty() {
			parentBounds = lr.ArrangedBounds
		} else if lr.MeasuredSize.W > 0 || lr.MeasuredSize.H > 0 {
			parentBounds = gfx.RectFromXYWH(0, 0, lr.MeasuredSize.W, lr.MeasuredSize.H)
		}
	} else if w, h := rt.windowSize(); w > 0 || h > 0 {
		parentBounds = gfx.RectFromXYWH(0, 0, float32(w), float32(h))
	}
	if parentBounds.IsEmpty() {
		return stats
	}
	childrenByLayer := make(map[layout.LayerID][]layerChildRef, len(parent.Base().Children()))
	for _, childBase := range parent.Base().Children() {
		if childBase == nil {
			continue
		}
		child := childBase
		attachment := rt.childAttachments[child.ID()]
		layerID := attachment.LayerID
		if layerID == 0 && len(specs) > 0 {
			layerID = specs[0].ID
		}
		childrenByLayer[layerID] = append(childrenByLayer[layerID], layerChildRef{
			base:       child,
			attachment: attachment,
		})
	}
	layerBoundsStart := time.Now()
	for _, spec := range specs {
		resolved := layout.ResolvedLayer{
			LayerID:     spec.ID,
			Bounds:      resolveLayerBounds(parentBounds, spec),
			CoordLimits: spec.CoordLimits,
			HitPolicy:   spec.HitPolicy,
			RenderOrder: spec.RenderOrder,
			CoordSpace:  spec.CoordSpace,
		}
		resolved.Transform = resolveLayerTransform(accumulated, spec)
		resolved.ClipRect = resolveClipRect(resolved, spec, parent.Base(), accumulated)
		state.layers = append(state.layers, resolved)
		state.childCounts = append(state.childCounts, len(childrenByLayer[spec.ID]))
	}
	stats.layerBoundsResolution += time.Since(layerBoundsStart)
	anchorStart := time.Now()
	rt.reconcileAnchorExports(parent)
	stats.anchorExport += time.Since(anchorStart)
	cache := rt.anchorCaches[parentID]
	for i := range state.layers {
		if state.specs[i].Placement == layout.PlacementAnchor {
			state.layers[i].AnchorCache = cache
		}
	}
	arrangeStart := time.Now()
	for i, spec := range state.specs {
		layerChildren := childrenByLayer[spec.ID]
		policy := rt.policyRegistry.MustPolicy(spec.Placement)
		nodes := make([]layout.ChildNode, 0, len(layerChildren))
		handles := make([]layout.ChildArrangeHandle, len(layerChildren))
		resolved := state.layers[i]
		for j := range layerChildren {
			ref := layerChildren[j]
			intrinsic := gfx.Size{}
			node := layout.ChildNode{
				FacetID:       ref.base.ID(),
				Attachment:    ref.attachment,
				IntrinsicSize: intrinsic,
			}
			if spec.Placement == layout.PlacementProjected {
				if childImpl := rt.findFacetByID(rt.root, ref.base.ID()); childImpl != nil {
					if wp, ok := childImpl.(projectedpolicy.WorldPositioned); ok {
						node.WorldPosition = wp.WorldPosition()
						node.WorldSize = wp.WorldSize()
						node.HasWorldSpace = true
					} else if rt.log != nil {
						rt.log.Warn("layout/projected missing WorldPositioned", "facetID", ref.base.ID(), "layerID", spec.ID)
					}
				}
			} else if lr := ref.base.LayoutRole(); lr != nil {
				intrinsic = lr.Measure(layout.Loose(gfx.Size{
					W: parentBounds.Width(),
					H: parentBounds.Height(),
				}))
				node.IntrinsicSize = intrinsic
			}
			node.AttachArrangeHandle(&handles[j])
			nodes = append(nodes, node)
		}
		if spec.Measurement == layout.MeasureStructural {
			measureStart := time.Now()
			_ = policy.Measure(nodes, gfx.Size{W: resolved.Bounds.Width(), H: resolved.Bounds.Height()})
			stats.structuralMeasure += time.Since(measureStart)
		}
		policy.Arrange(nodes, resolved)
		for j := range layerChildren {
			ref := layerChildren[j]
			arranged := gfx.Rect{}
			if bounds, ok := handles[j].Bounds(); ok {
				arranged = bounds
				if lr := ref.base.LayoutRole(); lr != nil {
					lr.Arrange(bounds)
				}
			}
			rt.projectionLayers[ref.base.ID()] = facet.ProjectionLayer{
				Bounds:      arranged,
				Transform:   resolved.Transform,
				ClipRect:    resolved.ClipRect,
				RenderOrder: resolved.RenderOrder,
				HitPolicy:   uint8(resolved.HitPolicy),
			}
		}
		state.layers[i] = resolved
	}
	stats.arrange += time.Since(arrangeStart)
	return stats
}

type layerChildRef struct {
	base       *facet.Facet
	attachment layout.ChildAttachment
}

func resolveLayerBounds(parentBounds gfx.Rect, spec layout.LayerSpec) gfx.Rect {
	if !spec.CoordLimits.Bounds.IsEmpty() {
		return spec.CoordLimits.Bounds
	}
	return parentBounds
}

func resolveLayerTransform(accumulated gfx.Transform, spec layout.LayerSpec) gfx.Transform {
	switch spec.CoordSpace {
	case layout.CoordViewport:
		return accumulated
	case layout.CoordScreenAligned:
		return gfx.Identity()
	case layout.CoordParentLayout, layout.CoordContent:
		return gfx.Identity()
	default:
		return gfx.Identity()
	}
}

func resolveClipRect(layer layout.ResolvedLayer, spec layout.LayerSpec, parent *facet.Facet, accumulated gfx.Transform) gfx.Rect {
	switch spec.ClipPolicy {
	case layout.ClipNone:
		return gfx.Rect{}
	case layout.ClipToParent, layout.ClipToContent, layout.ClipToViewport:
		if spec.ClipPolicy == layout.ClipToViewport && parent != nil {
			if vr := parent.ViewportRole(); vr != nil && !vr.WorldBounds.IsEmpty() {
				return accumulated.TransformRect(vr.WorldBounds)
			}
		}
		return layer.Bounds
	default:
		return layer.Bounds
	}
}

// ResolveProjectionLayer returns the current projection-layer snapshot for a facet.
func (rt *Runtime) ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool) {
	if rt == nil || id == 0 {
		return facet.ProjectionLayer{}, false
	}
	layer, ok := rt.projectionLayers[id]
	return layer, ok
}

// ResolveChildAttachment returns the stored attachment metadata for a child facet.
func (rt *Runtime) ResolveChildAttachment(id facet.FacetID) (layout.ChildAttachment, bool) {
	if rt == nil || id == 0 || rt.childAttachments == nil {
		return layout.ChildAttachment{}, false
	}
	attachment, ok := rt.childAttachments[id]
	return attachment, ok
}

// LayerSnapshots returns diagnostics layer summaries for a parent facet.
func (rt *Runtime) LayerSnapshots(parent facet.FacetID) []diagnostics.LayerSnapshot {
	if rt == nil || parent == 0 {
		return nil
	}
	state := rt.layerStates[parent]
	if state == nil || len(state.specs) == 0 {
		return nil
	}
	snapshots := make([]diagnostics.LayerSnapshot, 0, len(state.specs))
	cache := rt.anchorCaches[parent]
	for i := range state.specs {
		spec := state.specs[i]
		layer := layout.ResolvedLayer{}
		if i < len(state.layers) {
			layer = state.layers[i]
		}
		snapshots = append(snapshots, diagnostics.LayerSnapshot{
			LayerID:     spec.ID,
			Placement:   spec.Placement,
			Measurement: spec.Measurement,
			CoordSpace:  spec.CoordSpace,
			RenderOrder: spec.RenderOrder,
			HitPolicy:   spec.HitPolicy,
			Bounds:      layer.Bounds,
			ClipRect:    layer.ClipRect,
			Transform:   layer.Transform,
			ChildCount:  0,
			AnchorCacheVersion: func() uint64 {
				if cache == nil {
					return 0
				}
				return cache.Version()
			}(),
			AnchorCacheCount: func() int {
				if cache == nil {
					return 0
				}
				return cache.Len()
			}(),
		})
		if i < len(state.childCounts) {
			snapshots[i].ChildCount = state.childCounts[i]
		}
	}
	return snapshots
}

// AnchorSnapshot returns diagnostics data for a parent's anchor cache.
func (rt *Runtime) AnchorSnapshot(parent facet.FacetID) (diagnostics.AnchorSnapshot, bool) {
	if rt == nil || parent == 0 {
		return diagnostics.AnchorSnapshot{}, false
	}
	cache := rt.anchorCaches[parent]
	if cache == nil {
		return diagnostics.AnchorSnapshot{}, false
	}
	parentFacet := rt.findFacetByID(rt.root, parent)
	if parentFacet == nil || parentFacet.Base() == nil {
		return diagnostics.AnchorSnapshot{}, false
	}
	childRefs := make(map[layout.AnchorID][]facet.FacetID)
	for _, childBase := range parentFacet.Base().Children() {
		if childBase == nil {
			continue
		}
		attachment, ok := rt.childAttachments[childBase.ID()]
		if !ok || attachment.Placement.AnchorRef == "" {
			continue
		}
		childRefs[attachment.Placement.AnchorRef] = append(childRefs[attachment.Placement.AnchorRef], childBase.ID())
	}
	snapshot := diagnostics.AnchorSnapshot{
		ParentID: parent,
		Version:  cache.Version(),
	}
	positions := cache.Snapshot()
	if len(positions) == 0 {
		return snapshot, true
	}
	for id, pos := range positions {
		snapshot.Entries = append(snapshot.Entries, diagnostics.AnchorSnapshotEntry{
			ID:       id,
			Position: pos,
			Children: append([]facet.FacetID(nil), childRefs[id]...),
		})
	}
	sort.SliceStable(snapshot.Entries, func(i, j int) bool {
		return snapshot.Entries[i].ID < snapshot.Entries[j].ID
	})
	return snapshot, true
}
