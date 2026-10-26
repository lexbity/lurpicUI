package runtime

import (
	"errors"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/drive/readstore"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/internal/membudget"
	"codeburg.org/lexbit/lurpicui/internal/renderutil"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

type Runtime struct {
	config Config

	projectionSystem     *projection.System
	inputSystem          *input.System
	focusManager         *facet.FocusManager
	jobPool              *job.Pool
	renderPipeline       *RenderPipeline
	lastRenderCompletion RenderCompletion
	layerRegistry        *layout.LayerRegistry
	assetManager         assets.Manager
	// assetUploader tracks the pipeline's upload adapter so a GPU→software
	// backend swap can close the old adapter and re-wire the asset manager
	// (Slice 10, FR-12).
	assetUploader *assetUploader

	platformApp    platform.App
	window         platform.Window
	windowBindings map[string]platform.Window
	root           facet.FacetImpl
	liveIndex      *liveIndex

	frameNumber  uint64
	frameTimer   *FrameTimer
	contentScale float32
	// clock is the engine time service (QP-5): Wall reads the system clock,
	// Virtual single-steps on driver advances. Runtime-thread owned.
	clock                  *Clock
	bootstrapFramePending  bool
	bootstrapFrameAwaiting bool

	dirtyFacets      map[facet.FacetID]facet.DirtyFlags
	dirtySources     map[facet.FacetID]string
	nextDirtyFacets  map[facet.FacetID]facet.DirtyFlags
	nextDirtySources map[facet.FacetID]string
	dirtyCaptured    bool
	childAttachments map[facet.FacetID]facet.Attachment
	anchorCaches     map[facet.FacetID]*layout.AnchorPositionCache
	projectionLayers map[facet.FacetID]facet.ProjectionLayer
	lastHitTrace     diagnostics.HitTestTrace
	hitTraceEnabled  bool
	pendingEvents    []platform.Event
	lastWindowFrames map[string]*render.Frame
	diagMu           sync.RWMutex
	diag             DiagnosticsHook
	rootStyleContext any
	rootStyleSubs    signal.Subscriptions
	phase1HooksMu    sync.RWMutex
	phase1Hooks      []phase1HookSlot
	phase1HookFree   []int
	phase1HookSeq    uint64
	shutdownHooksMu  sync.RWMutex
	shutdownHooks    []func()
	lifecycleMu      sync.Mutex
	lifecycleCond    *sync.Cond
	paused           bool
	lifecycleBound   bool
	// surfaceReady gates render submission after a lifecycle-driven surface
	// loss. The runtime may keep ticking and processing input while the
	// platform surface is absent, but the backend must not receive frames until
	// a fresh surface has been created successfully.
	surfaceReady bool
	imeVisible   bool

	shutdownCh chan struct{}
	doneCh     chan struct{}

	lastStats         diagnostics.FrameStats
	lastDirtySnapshot map[facet.FacetID]facet.DirtyFlags
	// lastArrangeCount is the number of arrangements performed by the most
	// recent layout pass (0 when the pass was skipped). RX-1 P5: arrange-count
	// is 0 in the quiet steady state.
	lastArrangeCount int
	log              log.Logger

	startOnce  sync.Once
	shutdownMu sync.Mutex
	started    bool
	stopping   bool

	// frameMu serializes an in-flight frame against tree disposal. runFrame
	// takes a read lock for the frame body; disposeTree takes a write lock.
	// This enforces the single-driver-thread invariant: the facet tree is
	// never read and mutated concurrently, even across the shutdown handshake.
	frameMu sync.RWMutex

	// Per-facet panic quarantine. A facet whose callback panics is marked
	// poisoned and its subtree is skipped on subsequent frames; see
	// guardedInvoke and poison.
	poisonMu         sync.RWMutex
	poisoned         map[facet.FacetID]struct{}
	poisonReports    map[facet.FacetID]*poisonReport
	recoveryDisabled bool
	focusDirty       bool
	focusRebuilds    uint64
	layerTreeDirty   bool
	activeTickFacets []facet.FacetImpl
	activeTickByID   map[facet.FacetID]int
	tickWork         []facet.FacetImpl

	// projectionInProgress is a runtime-thread guard used by store mutations.
	// The store package consults this so writes that would invalidate the
	// current projection phase fail loudly instead of corrupting frame order.
	projectionInProgress atomic.Bool
	wallWakePending      atomic.Bool

	onFrameSubmitted func() // test hook; set only in tests to signal frame completion

	// drivePort is non-nil only when Config.Drive.Enabled. Its bounded
	// inbox is admitted at a cut on this thread; see drive_port.go.
	drivePort *drivePort
	// currentDriveJobScope is set only while a socket-originated application
	// command callback is executing on the runtime thread.
	currentDriveJobScope *commandJobScope

	// liveTestIDs is the author-identity registry (QP-7): TestID → facet ID
	// for attached facets. Installed as the facet-package registry in New.
	liveTestIDs liveTestIDs

	// namedStores is the LDP named-store registry (QP-8 path b).
	namedStores           *readstore.Registry
	namedExportOwners     map[string]facet.FacetID
	facetReadExports      map[facet.FacetID]map[string]store.ReadExport
	readExportsFrozen     bool
	treeSnapshots         map[string]treeSnapshot
	nextTreeSnapshot      uint64
	treeSnapshotBytes     int
	storeReadSnapshots    map[string]storeReadSnapshot
	nextStoreSnapshot     uint64
	storeSnapshotBytes    int
	storeSnapshotReserved int
	driveObservation      *driveObservationIndex

	// driveInputStep is the in-flight drive input command (QP-11); nil when
	// no step owns input.
	driveInputStep   *driveInputStep
	controlWatchers  map[facet.FacetID][]controlWatcher
	controlWatchSeq  uint64
	nextGestureID    uint64
	nextDriveTouchID uint64
}

func New(config Config, platformApp platform.App, window platform.Window, backend render.Backend, root facet.FacetImpl) (*Runtime, error) {
	if config.TargetFPS <= 0 {
		return nil, errors.New("runtime: TargetFPS must be greater than zero")
	}
	if config.Drive.Enabled {
		if config.Drive.ObservationNodeCapacity == 0 {
			config.Drive.ObservationNodeCapacity = driveObservationNodeLimit
		}
		if config.Drive.ObservationByteCapacity == 0 {
			config.Drive.ObservationByteCapacity = driveObservationByteLimit
		}
		if config.Drive.ObservationNodeCapacity < 1 || config.Drive.ObservationNodeCapacity > 262144 {
			return nil, errors.New("runtime: drive observation node capacity must be in [1,262144]")
		}
		if config.Drive.ObservationByteCapacity < 1 || config.Drive.ObservationByteCapacity > 64<<20 {
			return nil, errors.New("runtime: drive observation byte capacity must be in [1,64 MiB]")
		}
	}
	if config.FontRegistry == nil {
		return nil, errors.New("runtime: FontRegistry is required")
	}
	if err := config.Resources.Validate(); err != nil {
		return nil, err
	}
	if got, want := config.FontRegistry.WalletCapacity(), config.Resources.Plan.Caps[membudget.Text]; got != want {
		return nil, fmt.Errorf("runtime: font registry text wallet capacity %d does not match resource plan %d", got, want)
	}
	if config.TextService != nil && config.TextService.Registry() != config.FontRegistry {
		return nil, errors.New("runtime: TextService must use Config.FontRegistry")
	}
	if config.LayerRegistry == nil {
		return nil, errors.New("runtime: LayerRegistry is required")
	}
	if root == nil {
		return nil, errors.New("runtime: root facet is required")
	}
	if config.StoreScope == nil {
		return nil, errors.New("runtime: StoreScope is required")
	}
	if config.StoreIdentity == nil {
		return nil, errors.New("runtime: StoreIdentity is required")
	}
	if config.HostedChildren == nil {
		return nil, errors.New("runtime: HostedChildren capability is required")
	}
	if backend == nil {
		return nil, errors.New("runtime: backend is required")
	}
	preparedClock := config.PreparedClock
	if preparedClock == nil {
		var err error
		preparedClock, err = PrepareClock(config.Clock)
		if err != nil {
			return nil, err
		}
	}
	if preparedClock.clock == nil {
		return nil, errors.New("runtime: prepared clock is invalid")
	}
	config.PreparedClock = preparedClock
	if config.WorkerCount <= 0 {
		config.WorkerCount = DefaultConfig().WorkerCount
	}
	if config.Logger == nil {
		config.Logger = log.NopLogger{}
	}
	rt := &Runtime{
		config:                config,
		clock:                 preparedClock.clock,
		bootstrapFramePending: preparedClock.bootstrap,
		projectionSystem:      projection.NewSystem(),
		inputSystem:           input.NewSystem(config.GestureConfig),
		focusManager:          facet.NewFocusManager(),
		focusDirty:            true,
		layerTreeDirty:        true,
		activeTickByID:        make(map[facet.FacetID]int),
		jobPool:               job.NewPool(config.WorkerCount),
		layerRegistry:         config.LayerRegistry,
		assetManager:          config.AssetManager,
		platformApp:           platformApp,
		window:                window,
		windowBindings:        copyWindowBindings(config.WindowBindings, window),
		root:                  root,
		liveIndex:             newLiveIndex(),
		frameTimer:            NewFrameTimer(config.TargetFPS),
		dirtyFacets:           make(map[facet.FacetID]facet.DirtyFlags),
		dirtySources:          make(map[facet.FacetID]string),
		nextDirtyFacets:       make(map[facet.FacetID]facet.DirtyFlags),
		nextDirtySources:      make(map[facet.FacetID]string),
		childAttachments:      make(map[facet.FacetID]facet.Attachment),
		anchorCaches:          make(map[facet.FacetID]*layout.AnchorPositionCache),
		projectionLayers:      make(map[facet.FacetID]facet.ProjectionLayer),
		lastWindowFrames:      make(map[string]*render.Frame),
		shutdownCh:            make(chan struct{}),
		doneCh:                make(chan struct{}),
		log:                   config.Logger,
		diag:                  config.DiagnosticsHook,
		surfaceReady:          true,
		poisoned:              make(map[facet.FacetID]struct{}),
		poisonReports:         make(map[facet.FacetID]*poisonReport),
		controlWatchers:       make(map[facet.FacetID][]controlWatcher),
		recoveryDisabled:      config.RecoveryDisabled,
	}
	rt.liveIndex.registerTree(root)
	if err := rt.validateFacetStoreScopes(root); err != nil {
		return nil, err
	}
	config.StoreScope.Bind(config.StoreIdentity)
	rt.liveTestIDs = make(liveTestIDs)
	rt.facetReadExports = make(map[facet.FacetID]map[string]store.ReadExport)
	rt.namedExportOwners = make(map[string]facet.FacetID)
	if err := rt.initNamedRegistry(); err != nil {
		return nil, err
	}
	if err := rt.registerTreeReadExports(root); err != nil {
		return nil, err
	}
	rt.inputSystem.SetTimeSource(rt.clock.Now)
	rt.clock.SetGestureGuard(rt.inputSystem.GestureActive)
	rt.frameTimer.SetPacingClock(rt.clock.WallNow)
	rt.renderPipeline = newRenderPipeline(backend)
	// Slice 10 (Q9/FR-12): the render thread swaps a fatal GPU backend for the
	// software backend bound to the current surface; the swap notifies the
	// runtime so it can log, emit OnBackendFallback, and re-wire the asset
	// texture releaser/uploader to the software backend.
	rt.renderPipeline.fallbackFactory = rt.softwareFallbackBackend
	rt.renderPipeline.onFallback = rt.notifyBackendFallback
	rt.lifecycleCond = sync.NewCond(&rt.lifecycleMu)
	if config.Drive.Enabled {
		if err := rt.installDrivePort(); err != nil {
			return nil, err
		}
	}
	if rt.rootStyleContext == nil {
		theme.NewRootStyleContext(config.StoreScope, rt, theme.DefaultTokens(), nil)
	}
	rt.inputSystem.SetFocusManager(rt.focusManager)
	if cap, ok := platform.PointerCapableOf(platformApp); ok {
		rt.inputSystem.SetHoverSupported(cap.SupportsHover())
	}
	if err := rt.validateWindowBindings(); err != nil {
		return nil, err
	}
	if err := rt.validateLayerRecipes(); err != nil {
		return nil, err
	}
	createdTextService := false
	if config.TextService == nil {
		service, err := text.NewService(config.FontRegistry, config.Resources.TextLimits)
		if err != nil {
			return nil, err
		}
		config.TextService = service
		rt.config.TextService = service
		createdTextService = true
	}
	if err := config.TextService.BindRuntime(); err != nil {
		if createdTextService {
			_ = config.TextService.Close()
		}
		return nil, err
	}
	store.SetProjectionActiveCheck(func() bool {
		return rt.projectionInProgress.Load()
	})
	facet.SetCollectPhaseCheck(func() bool {
		return rt.projectionInProgress.Load()
	})

	// Wire the GPU texture releaser and uploader bridges when the backend
	// supports textures. This keeps the asset cache and the renderer aligned:
	// eviction must release backend textures, and decoded pixels must flow
	// through the renderer's per-frame upload budget.
	if tb, ok := backend.(render.TextureBackend); ok && tb.UploadBudgetBytesPerFrame() > 0 {
		if m, ok := rt.assetManager.(*assets.ManagerImpl); ok {
			m.SetTextureReleaser(&assetTextureReleaser{backend: tb})
			if q := rt.renderPipeline.UploadQueue(); q != nil {
				rt.assetUploader = newAssetUploader(q)
				m.SetUploader(rt.assetUploader)
			}
			// Wire the device-loss re-upload callback so that ResolveDrawable
			// can trigger a lazy re-upload when a GPU LOD was invalidated but
			// the CPU data survives.
			assets.SetGPUReuploadCallback(func(id assets.AssetID, lod int) {
				m.RequestUpload(id, lod)
			})
		}
	}
	rt.projectionSystem.SetRuntime(rt)

	return rt, nil
}

func (rt *Runtime) CommandRegistry() *CommandRegistry {
	return rt.config.CommandRegistry
}

// LastPrimaryFrame returns the most recent submitted primary-window frame
// (nil before the first submit).
func (rt *Runtime) LastPrimaryFrame() *render.Frame {
	return rt.lastWindowFrames["__primary__"]
}

// Clock returns the runtime's time service (QP-5).
func (rt *Runtime) Clock() *Clock { return rt.clock }

// Now returns the current engine time (RuntimeServices accessor). Called on
// the runtime thread; marks read the clock so Virtual-mode replay is
// deterministic.
func (rt *Runtime) Now() time.Time { return rt.clock.Now() }

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
				item.z = int(attachment.ZOrder)
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
	scope := rt.currentDriveJobScope
	if scope != nil && !rt.drivePort.registerCommandJob(scope, j.JobID()) {
		return
	}
	ownerID := facet.FacetID(j.OwnerID())
	afterCommit := func(result job.AnyResult) {
		f := rt.liveFacet(ownerID)
		if f == nil || f.Base() == nil {
			return
		}
		pr := f.Base().ProjectionRole()
		if pr == nil || pr.OnJobResult == nil {
			return
		}
		rt.guardedInvoke(f.Base().ID(), "job", func() {
			pr.OnJobResult(result)
		})
		f.Base().InvalidateWithSource(facet.DirtyProjection, "job.OnJobResult")
		rt.recordInvalidation(ownerID, facet.DirtyProjection, "job.OnJobResult")
		if rt.frameTimer != nil {
			rt.frameTimer.RequestFrame()
		}
	}
	var err error
	if scope != nil {
		err = rt.jobPool.SubmitAnyGuarded(j, scope.active.Load, afterCommit)
		if err != nil {
			rt.drivePort.rollbackCommandJob(scope, j.JobID(), err)
		}
		return
	}
	_ = rt.jobPool.SubmitAny(j, afterCommit)
}

func (rt *Runtime) CancelJob(id job.JobID) {
	if rt.jobPool == nil {
		return
	}
	rt.jobPool.CancelJob(id)
}

func (rt *Runtime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {

	rt.config.StoreScope.QueueSignal(func() {
		rt.recordInvalidation(id, flags, source)
		rt.markFocusDirty(id)
		if rt.root != nil && rt.root.Base() != nil && rt.root.Base().ID() == id {
			rt.root.Base().InvalidateWithSource(flags, source)
		}
	})
}

func (rt *Runtime) MarkTreeDirty(root facet.FacetImpl, flags facet.DirtyFlags) {
	if root == nil {
		return
	}
	rt.config.StoreScope.QueueSignal(func() {
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

// SetFrameClock seeds the simulated frame clock: the next frame's delta time
// is measured from the given instant. It is a test seam — headless harnesses
// never call FrameTimer.Wait, so dt stays zero and TickRole cadence cannot be
// exercised. Seeding the clock lets a test drive the real frame-tick path end
// to end (AC-1). The clock owns frame deltas (QP-5), so the seed goes there.
func (rt *Runtime) SetFrameClock(now time.Time) {
	if rt == nil || rt.clock == nil {
		return
	}
	rt.clock.SeedLastFrame(now)
}
