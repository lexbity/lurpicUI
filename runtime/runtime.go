package runtime

import (
	"errors"
	"fmt"
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
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
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

	platformApp platform.App
	window      platform.Window
	root        facet.FacetImpl

	frameNumber  uint64
	frameTimer   *FrameTimer
	contentScale float32

	dirtyFacets   map[facet.FacetID]facet.DirtyFlags
	dirtySources  map[facet.FacetID]string
	pendingEvents []platform.Event
	signalQueue   []pendingSignal
	diagMu        sync.RWMutex
	diag          DiagnosticsHook

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
		platformApp:      platformApp,
		window:           window,
		root:             root,
		frameTimer:       NewFrameTimer(config.TargetFPS),
		dirtyFacets:      make(map[facet.FacetID]facet.DirtyFlags),
		dirtySources:     make(map[facet.FacetID]string),
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
	rt.tickFacets(dt)

	currentHitMap := rt.projectionSystem.CurrentHitMap()
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
		stats.LayerCount = len(frameOut.Layers)
	}

	renderStart := time.Now()
	if rt.renderPipeline != nil && frameOut != nil {
		frame := rt.assembleFrame(frameOut, dirtySnapshot)
		if diag := rt.diagnosticsHook(); diag != nil {
			if oi, ok := diag.(overlayInjector); ok {
				oi.InjectOverlay(frame, rt.lastStats)
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

// AddFacet attaches a new child facet at runtime.
func (rt *Runtime) AddFacet(parent, child facet.FacetImpl) {
	if rt == nil || parent == nil || child == nil {
		return
	}
	parentBase := parent.Base()
	childBase := child.Base()
	if parentBase == nil || childBase == nil {
		return
	}
	parentBase.AddChildRuntime(childBase)
	rt.attachTree(child)
	rt.activateTree(child)
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
	root.Base().InvalidateWithSource(flags, "runtime.markTreeDirty")
	rt.dirtyFacets[root.Base().ID()] = flags
	rt.dirtySources[root.Base().ID()] = "runtime.markTreeDirty"
	for _, child := range root.Base().Children() {
		rt.markTreeDirty(child, flags)
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
	if root.Base() != nil && root.Base().State() == facet.StateActive {
		fn(root)
	}
	for _, child := range root.Base().Children() {
		rt.walkActive(child, fn)
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
	return assembleFrame(frame, nil)
}

func (rt *Runtime) assembleFrame(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) *render.Frame {
	return assembleFrame(output, dirtySnapshot)
}

func assembleFrame(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) *render.Frame {
	if output == nil {
		return &render.Frame{}
	}
	out := &render.Frame{
		Layers: make([]render.Layer, 0, len(output.Layers)),
	}
	for _, layer := range output.Layers {
		cmds := gfx.CommandList{}
		if !layer.Transform.IsIdentity() {
			cmds.Add(gfx.PushTransform{Matrix: layer.Transform})
		}
		for _, cmd := range layer.Commands.Commands {
			cmds.Add(cmd)
		}
		if sel := output.SelectionGeometries[layer.FacetID]; sel != nil {
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
		if !layer.Transform.IsIdentity() {
			cmds.Add(gfx.PopTransform{})
		}
		out.Layers = append(out.Layers, render.Layer{
			ID:          render.LayerID(layer.FacetID),
			Bounds:      layer.Bounds,
			Opacity:     layer.Opacity,
			Commands:    cmds,
			CommandHash: hashutil.HashCommandList(cmds),
		})
	}
	out.DirtyRegions = computeDirtyRegions(output, dirtySnapshot)
	return out
}

func computeDirtyRegions(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) []gfx.Rect {
	if output == nil || len(output.Layers) == 0 {
		return nil
	}
	rects := make([]gfx.Rect, 0, len(output.Layers))
	for _, layer := range output.Layers {
		if dirtySnapshot != nil {
			if flags := dirtySnapshot[layer.FacetID]; flags == 0 {
				continue
			}
		}
		rects = append(rects, layer.Bounds)
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
}

// Inspect runs fn with a synchronous inspector snapshot.
func (rt *Runtime) Inspect(fn func(*diagnostics.Inspector)) {
	if rt == nil || fn == nil {
		return
	}
	fn(diagnostics.NewInspector(rt.root))
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
	if root.Base().ID() == id {
		return root
	}
	for _, child := range root.Base().Children() {
		if found := rt.findFacetByID(child, id); found != nil {
			return found
		}
	}
	return nil
}

// overlayInjector is satisfied by diagnostics.Hook when an Overlay is set.
type overlayInjector interface {
	InjectOverlay(frame *render.Frame, stats diagnostics.FrameStats)
}

var _ facet.RuntimeServices = (*Runtime)(nil)

// Legacy constructor compatibility for the test harness during the transition.
func NewLegacy(root facet.FacetImpl, backend render.Backend, windowSize gfx.Size) (*Runtime, error) {
	cfg := DefaultConfig()
	reg, _ := text.NewFontRegistry()
	cfg.FontRegistry = reg
	return New(cfg, nil, nil, backend, root)
}
