package runtime

import (
	"errors"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
)

type pendingSignal struct{ deliver func() }

type Runtime struct {
	config Config

	projectionSystem *projection.System
	inputSystem      *input.System
	focusManager     *facet.FocusManager
	jobPool          *job.Pool
	renderPipeline   *RenderPipeline
	layerRegistry    *layout.LayerRegistry
	assetManager     assets.Manager

	platformApp    platform.App
	window         platform.Window
	windowBindings map[string]platform.Window
	root           facet.FacetImpl

	frameNumber  uint64
	frameTimer   *FrameTimer
	contentScale float32

	dirtyFacets      map[facet.FacetID]facet.DirtyFlags
	dirtySources     map[facet.FacetID]string
	childAttachments map[facet.FacetID]facet.Attachment
	anchorCaches     map[facet.FacetID]*layout.AnchorPositionCache
	projectionLayers map[facet.FacetID]facet.ProjectionLayer
	lastHitTrace     diagnostics.HitTestTrace
	hitTraceEnabled  bool
	pendingEvents    []platform.Event
	signalQueue      []pendingSignal
	lastWindowFrames map[string]*render.Frame
	diagMu           sync.RWMutex
	diag             DiagnosticsHook
	rootStyleContext any
	rootStyleSubs    signal.Subscriptions
	phase1HooksMu    sync.RWMutex
	phase1Hooks      []func(time.Duration)
	shutdownHooksMu  sync.RWMutex
	shutdownHooks    []func()
	lifecycleMu      sync.Mutex
	lifecycleCond    *sync.Cond
	paused           bool
	lifecycleBound   bool
	surfaceReady     bool
	imeVisible       bool

	shutdownCh chan struct{}
	doneCh     chan struct{}

	lastStats diagnostics.FrameStats
	log       Logger

	startOnce  sync.Once
	shutdownMu sync.Mutex
	started    bool
	stopping   bool

	projectionInProgress atomic.Bool
}

func New(config Config, platformApp platform.App, window platform.Window, backend render.Backend, root facet.FacetImpl) (*Runtime, error) {
	if config.TargetFPS <= 0 {
		return nil, errors.New("runtime: TargetFPS must be greater than zero")
	}
	if config.FontRegistry == nil {
		return nil, errors.New("runtime: FontRegistry is required")
	}
	if config.LayerRegistry == nil {
		return nil, errors.New("runtime: LayerRegistry is required")
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
		projectionSystem: projection.NewSystem(),
		inputSystem:      input.NewSystem(config.GestureConfig),
		focusManager:     facet.NewFocusManager(),
		jobPool:          job.NewPool(config.WorkerCount),
		renderPipeline:   newRenderPipeline(backend),
		layerRegistry:    config.LayerRegistry,
		assetManager:     config.AssetManager,
		platformApp:      platformApp,
		window:           window,
		windowBindings:   copyWindowBindings(config.WindowBindings, window),
		root:             root,
		frameTimer:       NewFrameTimer(config.TargetFPS),
		dirtyFacets:      make(map[facet.FacetID]facet.DirtyFlags),
		dirtySources:     make(map[facet.FacetID]string),
		childAttachments: make(map[facet.FacetID]facet.Attachment),
		anchorCaches:     make(map[facet.FacetID]*layout.AnchorPositionCache),
		projectionLayers: make(map[facet.FacetID]facet.ProjectionLayer),
		lastWindowFrames: make(map[string]*render.Frame),
		shutdownCh:       make(chan struct{}),
		doneCh:           make(chan struct{}),
		log:              config.Logger,
		diag:             config.DiagnosticsHook,
		surfaceReady:     true,
	}
	rt.lifecycleCond = sync.NewCond(&rt.lifecycleMu)
	if rt.rootStyleContext == nil {
		theme.NewRootStyleContext(rt, theme.DefaultTokens(), nil)
	}
	rt.inputSystem.SetFocusManager(rt.focusManager)
	if cap, ok := platform.PointerCapableOf(platformApp); ok {
		rt.inputSystem.SetHoverSupported(cap.SupportsHover())
	}
	if err := rt.validateWindowBindings(); err != nil {
		return nil, err
	}
	store.SetProjectionActiveCheck(func() bool {
		return rt.projectionInProgress.Load()
	})
	return rt, nil
}

func (rt *Runtime) CommandRegistry() *CommandRegistry {
	return rt.config.CommandRegistry
}

func convertFrame(frame *projection.FrameOutput) *render.Frame {
	return assembleFrameWithLayers(frame, nil, nil)
}

func (rt *Runtime) assembleFrame(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) *render.Frame {
	return assembleFrameWithLayers(output, dirtySnapshot, rt)
}

func (rt *Runtime) WindowFrames() map[string]*render.Frame {
	if len(rt.lastWindowFrames) == 0 {
		return nil
	}
	out := make(map[string]*render.Frame, len(rt.lastWindowFrames))
	for k, v := range rt.lastWindowFrames {
		out[k] = v
	}
	return out
}

type frameLayerResolver interface {
	ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool)
	ResolveChildAttachment(id facet.FacetID) (facet.Attachment, bool)
	ResolveWindowBinding(id facet.FacetID) (layout.WindowBinding, bool)
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
				item.z = int(attachment.ZPriority)
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

func (rt *Runtime) assembleWindowFrames(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) map[string]*render.Frame {
	if output == nil {
		return nil
	}
	grouped := make(map[string]*projection.FrameOutput)
	order := make([]string, 0)
	for _, po := range output.RenderBatchs {
		key := windowBindingKey(layout.WindowBinding{Kind: layout.WindowBindingPrimary})
		if rt != nil {
			if binding, ok := rt.resolveWindowBindingForFacet(po.FacetID); ok {
				key = windowBindingKey(binding)
			}
		}
		group, ok := grouped[key]
		if !ok {
			group = &projection.FrameOutput{SelectionGeometries: make(map[facet.FacetID]*projection.SelectionGeometry)}
			grouped[key] = group
			order = append(order, key)
		}
		group.RenderBatchs = append(group.RenderBatchs, po)
		if sel := output.SelectionGeometries[po.FacetID]; sel != nil {
			if group.SelectionGeometries == nil {
				group.SelectionGeometries = make(map[facet.FacetID]*projection.SelectionGeometry)
			}
			group.SelectionGeometries[po.FacetID] = sel
		}
	}
	out := make(map[string]*render.Frame, len(grouped))
	for _, key := range order {
		group := grouped[key]
		if group == nil {
			continue
		}
		out[key] = assembleFrameWithLayers(group, dirtySnapshot, rt)
	}
	return out
}

func (rt *Runtime) resolveWindowBindingForFacet(id facet.FacetID) (layout.WindowBinding, bool) {
	if id == 0 || rt.layerRegistry == nil {
		return layout.WindowBinding{}, false
	}
	if layer, ok := rt.projectionLayers[id]; ok {
		if desc, ok := rt.layerRegistry.Lookup(layout.LayerID(layer.LayerID)); ok {
			return desc.WindowBinding, true
		}
	}
	return layout.WindowBinding{}, false
}

func windowBindingKey(binding layout.WindowBinding) string {
	switch binding.Kind {
	case layout.WindowBindingNamed:
		return binding.Name
	default:
		return "__primary__"
	}
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

func (rt *Runtime) Schedule(j job.AnyJob) {
	if j == nil || rt.jobPool == nil {
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
	if rt.jobPool == nil {
		return
	}
	rt.jobPool.CancelJob(id)
}

func (rt *Runtime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {

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

func (rt *Runtime) MarkTreeDirty(root facet.FacetImpl, flags facet.DirtyFlags) {
	if root == nil {
		return
	}
	rt.queueSignal(func() {
		rt.markTreeDirty(root, flags)
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
	})
}

func (rt *Runtime) RequestFrame() {
	if rt.frameTimer == nil {
		return
	}
	rt.frameTimer.RequestFrame()
}
