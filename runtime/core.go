package runtime

import (
	"errors"
	"fmt"
	"runtime"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
	"unicode/utf8"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/drive/readstore"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/input"
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

var ErrNoReadyFrame = errors.New("runtime: no primary ready frame")

// defaultTeardownGrace returns the configured shutdown grace or the 5-second default.
func defaultTeardownGrace(config Config) time.Duration {
	if config.ShutdownGrace > 0 {
		return config.ShutdownGrace
	}
	return 5 * time.Second
}

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
	frameStorage  *frameStorage

	platformApp    platform.App
	window         platform.Window
	windowBindings map[string]platform.Window
	root           facet.FacetImpl
	liveIndex      *liveIndex

	frameNumber        uint64
	frameTimer         *FrameTimer
	contentScale       float32
	projectionModality facet.InputModality
	// clock is the engine time service (QP-5): Wall reads the system clock,
	// Virtual single-steps on driver advances. Runtime-thread owned.
	clock                  *Clock
	bootstrapFramePending  bool
	bootstrapFrameAwaiting bool

	dirtyFacets       map[facet.FacetID]facet.DirtyFlags
	dirtySources      map[facet.FacetID]string
	nextDirtyFacets   map[facet.FacetID]facet.DirtyFlags
	nextDirtySources  map[facet.FacetID]string
	dirtyCaptured     bool
	childAttachments  map[facet.FacetID]facet.Attachment
	anchorCaches      map[facet.FacetID]*layout.AnchorPositionCache
	projectionLayers  map[facet.FacetID]facet.ProjectionLayer
	lastHitTrace      diagnostics.HitTestTrace
	hitTraceEnabled   bool
	pendingEvents     []platform.Event
	lastWindowFrames  map[string]*render.Frame
	pendingFrameViews map[uint64]*render.FrameGroup
	lastReadyFrame    uint64
	frameViewMu       sync.RWMutex
	diagMu            sync.RWMutex
	diag              DiagnosticsHook
	rootStyleContext  any
	rootStyleSubs     signal.Subscriptions
	phase1HooksMu     sync.RWMutex
	phase1Hooks       []phase1HookSlot
	phase1HookActive  []int
	phase1HookFree    []int
	phase1HookSeq     uint64
	phase1Dispatches  int
	shutdownHooksMu   sync.RWMutex
	shutdownHooks     []func()
	lifecycleMu       sync.Mutex
	lifecycleCond     *sync.Cond
	paused            bool
	lifecycleBound    bool
	// surfaceReady gates render submission after a lifecycle-driven surface
	// loss. The runtime may keep ticking and processing input while the
	// platform surface is absent, but the backend must not receive frames until
	// a fresh surface has been created successfully.
	surfaceReady               bool
	surfaceLifecycleGeneration uint64
	lastDeviceGeneration       uint64
	hasDeviceGeneration        bool
	imeVisible                 bool

	shutdownCh  chan struct{}
	startupDone chan struct{}
	doneCh      chan struct{}

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

	// Teardown ownership (D7): single-owner claim and bounded-grace fallback.
	teardownClaim    atomic.Uint32 // 0 = open, 1 = owner claimed
	teardownGrace    time.Duration // default 5s, bounded supervisor fallback
	loopStopped      chan struct{} // closed when Run() loop exits
	runtimeGoroutine uint64        // goroutine ID of the runtime thread (D7)
	doneChClosed     atomic.Bool   // guard to prevent double-close of doneCh
}

func New(config Config, platformApp platform.App, window platform.Window, backend render.Backend, root facet.FacetImpl) (*Runtime, error) {
	if len(config.WindowBindings) >= render.MaxFrameWindows {
		return nil, fmt.Errorf("runtime: too many named windows (maximum total %d)", render.MaxFrameWindows)
	}
	for name, binding := range config.WindowBindings {
		if name == "" || name == render.PrimaryWindow || len(name) > 256 || !utf8.ValidString(name) || strings.ContainsRune(name, 0) || binding == nil {
			return nil, fmt.Errorf("runtime: invalid named window binding %q", name)
		}
	}

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
	storage, err := newFrameStorage(config.Resources.Plan.Caps[membudget.Frame], framePacketCapacity(config.Resources.Plan.Profile))
	if err != nil {
		return nil, fmt.Errorf("runtime: frame wallet: %w", err)
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
		pendingFrameViews:     make(map[uint64]*render.FrameGroup),
		frameStorage:          storage,
		shutdownCh:            make(chan struct{}),
		startupDone:           make(chan struct{}),
		doneCh:                make(chan struct{}),
		log:                   config.Logger,
		diag:                  config.DiagnosticsHook,
		surfaceReady:          true,
		poisoned:              make(map[facet.FacetID]struct{}),
		poisonReports:         make(map[facet.FacetID]*poisonReport),
		controlWatchers:       make(map[facet.FacetID][]controlWatcher),
		recoveryDisabled:      config.RecoveryDisabled,
		teardownGrace:         defaultTeardownGrace(config),
		loopStopped:           make(chan struct{}),
	}
	if err := rt.validateWindowBindings(); err != nil {
		return nil, err
	}
	if err := rt.configureRenderWindows(backend); err != nil {
		return nil, err
	}
	rt.liveIndex.registerTree(root)
	if err := rt.validateFacetStoreScopes(root); err != nil {
		return nil, err
	}
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
	rt.renderPipeline.fallbackFactory = config.SoftwareBackendFactory
	targets := rt.renderWindowTargets(rt.currentSurface())
	if len(targets) > 0 {
		rt.renderPipeline.targetCount = len(targets)
		copy(rt.renderPipeline.targets[:], targets)
	}
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
	config.StoreScope.Bind(config.StoreIdentity)
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
	// Create jobs wallet from the resource policy
	jobsWallet, err := membudget.NewWallet(config.Resources.Plan.Caps[membudget.Jobs])
	if err != nil {
		return nil, fmt.Errorf("failed to create jobs wallet: %w", err)
	}
	
	// Create the job pool with proper configuration.
	// Per D9: pool construction comes from the ResourcePlan.
	// Use ProfilePoolConfig to get the proper bounds for the profile.
	profile := config.Resources.Plan.Profile
	poolConfig := job.ProfilePoolConfig(profile.String())
	poolConfig.JobsWallet = jobsWallet
	poolConfig.WorkerCount = config.WorkerCount
	
	rt.jobPool = job.NewPoolWithConfig(poolConfig)
	// Also store in config so the app layer can use it for asset manager.
	// Per D9: the application owns pool construction; the runtime and asset manager consume it.
	config.JobPool = rt.jobPool
	
	rt.projectionSystem.SetRuntime(rt)

	return rt, nil
}

func (rt *Runtime) CommandRegistry() *CommandRegistry {
	return rt.config.CommandRegistry
}

// RetainLastPrimaryFrame returns an owned lease for the latest acknowledged
// primary-window frame. The caller must Release it after encoding or reading.
func (rt *Runtime) RetainLastPrimaryFrame() (*render.Frame, error) {
	if rt == nil {
		return nil, ErrNoReadyFrame
	}
	rt.frameViewMu.RLock()
	defer rt.frameViewMu.RUnlock()
	if rt.lastWindowFrames["__primary__"] == nil {
		return nil, ErrNoReadyFrame
	}
	return rt.lastWindowFrames["__primary__"].Retain()
}

// Clock returns the runtime's time service (QP-5).
func (rt *Runtime) Clock() *Clock { return rt.clock }

// Now returns the current engine time (RuntimeServices accessor). Called on
// the runtime thread; marks read the clock so Virtual-mode replay is
// deterministic.
func (rt *Runtime) Now() time.Time { return rt.clock.Now() }

func (rt *Runtime) sealFrameGroup(inputs map[string]render.FrameInput, cleanup bool) (*render.FrameGroup, error) {
	if len(inputs) == 0 || len(inputs) > render.MaxFrameWindows {
		return nil, fmt.Errorf("runtime: invalid frame window set size %d", len(inputs))
	}
	scratchCharge, err := rt.frameStorage.scratch.Reserve(frameSealScratchBytes)
	if err != nil {
		return nil, fmt.Errorf("runtime: reserve frame seal scratch: %w", err)
	}
	defer func() { _ = scratchCharge.Cancel() }()
	var scratch frameSealScratch
	defer func() { clear(scratch.keys[:]); clear(scratch.frames[:]) }()
	keys := scratch.keys[:0]
	for key := range inputs {
		keys = append(keys, key)
	}
	slices.SortFunc(keys, func(a, b string) int {
		if a == b {
			return 0
		}
		if a == render.PrimaryWindow {
			return -1
		}
		if b == render.PrimaryWindow {
			return 1
		}
		return strings.Compare(a, b)
	})
	sizes := scratch.sizes[:len(keys)]
	var total uint64
	for i, key := range keys {
		input := inputs[key]
		input.WindowID = key
		bytes, err := render.EstimateFrameBytes(input)
		if err != nil {
			return nil, fmt.Errorf("runtime: estimate window %q: %w", key, err)
		}
		if bytes > rt.frameStorage.packetCap-total {
			return nil, &membudget.ResourceLimit{Requested: bytes, Used: total, Capacity: rt.frameStorage.packetCap}
		}
		total += bytes
		sizes[i] = bytes
	}
	reservations, err := rt.frameStorage.reserve(sizes, cleanup)
	if err != nil {
		return nil, fmt.Errorf("runtime: reserve frame window set: %w", err)
	}
	frames := scratch.frames[:len(keys)]
	for i, key := range keys {
		input := inputs[key]
		input.WindowID = key
		frames[i], err = render.SealFrame(input, reservations[i])
		if err != nil {
			for j := range reservations {
				if frames[j] != nil {
					_ = frames[j].Release()
				} else {
					_ = reservations[j].Cancel()
				}
			}
			return nil, fmt.Errorf("runtime: seal window %q: %w", key, err)
		}
	}
	group, err := render.TakeFrameGroup(frames)
	if err != nil {
		for _, frame := range frames {
			_ = frame.Release()
		}
		return nil, err
	}
	return group, nil
}

// RetainWindowFrames returns owned leases for all latest acknowledged windows.
// The caller must Release every returned frame.
func (rt *Runtime) RetainWindowFrames() (map[string]*render.Frame, error) {
	if rt == nil {
		return map[string]*render.Frame{}, nil
	}
	rt.frameViewMu.RLock()
	defer rt.frameViewMu.RUnlock()
	if len(rt.lastWindowFrames) == 0 {
		return map[string]*render.Frame{}, nil
	}
	out := make(map[string]*render.Frame, len(rt.lastWindowFrames))
	for k, v := range rt.lastWindowFrames {
		retained, err := v.Retain()
		if err != nil {
			for _, frame := range out {
				_ = frame.Release()
			}
			return nil, fmt.Errorf("runtime: retain window %q frame: %w", k, err)
		}
		out[k] = retained
	}
	return out, nil
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
	batch render.BatchInput
}

func assembleFrameInputWithLayers(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags, resolver frameLayerResolver) render.FrameInput {
	if output == nil {
		return render.FrameInput{}
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
		rb := render.BatchInput{
			ID:       render.RenderBatchID(RenderBatch.FacetID),
			Bounds:   RenderBatch.Bounds,
			Opacity:  RenderBatch.Opacity,
			Commands: cmds,
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
	out := render.FrameInput{Batches: make([]render.BatchInput, 0, len(items)), Layers: make([]render.LayerInput, 0, len(items))}
	hasLayer := false
	var currentOrder int
	var currentClip gfx.Rect
	for _, item := range items {
		batchIndex := len(out.Batches)
		out.Batches = append(out.Batches, item.batch)
		if !hasLayer || currentOrder != item.order || currentClip != item.clip {
			out.Layers = append(out.Layers, render.LayerInput{
				RenderOrder: item.order,
				ClipRect:    item.clip,
				FirstBatch:  batchIndex,
				BatchCount:  1,
			})
			currentOrder = item.order
			currentClip = item.clip
			hasLayer = true
			continue
		}
		last := len(out.Layers) - 1
		out.Layers[last].BatchCount++
	}
	out.DirtyRegions = computeDirtyRegions(output, dirtySnapshot)
	return out
}

func (rt *Runtime) assembleWindowFrames(output *projection.FrameOutput, dirtySnapshot map[facet.FacetID]facet.DirtyFlags) map[string]render.FrameInput {
	if output == nil {
		return nil
	}
	grouped := make(map[string]*projection.FrameOutput)
	order := []string{render.PrimaryWindow}
	grouped[render.PrimaryWindow] = &projection.FrameOutput{}
	for key := range rt.windowBindings {
		if key != render.PrimaryWindow {
			order = append(order, key)
			grouped[key] = &projection.FrameOutput{}
		}
	}
	sort.Strings(order[1:])
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
	out := make(map[string]render.FrameInput, len(grouped))
	for _, key := range order {
		group := grouped[key]
		if group == nil {
			continue
		}
		input := assembleFrameInputWithLayers(group, dirtySnapshot, rt)
		input.WindowID = key
		if key == render.PrimaryWindow {
			input.SurfaceGeneration = rt.surfaceLifecycleGeneration
		} else {
			input.SurfaceGeneration = 1
		}
		out[key] = input
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

// closeDoneChOnce closes doneCh exactly once, preventing double-close panics.
func (rt *Runtime) closeDoneChOnce() {
	if rt.doneChClosed.CompareAndSwap(false, true) {
		close(rt.doneCh)
	}
}

// currentGoroutineID returns the current goroutine ID using runtime.Stack.
func currentGoroutineID() uint64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// Stack output format: "goroutine <id> [<state>]: ..."
	// We parse the numeric ID at the start.
	for i := 0; i < n; i++ {
		if buf[i] == ' ' {
			start := i + 1
			for j := start; j < n; j++ {
				if buf[j] < '0' || buf[j] > '9' {
					if id, err := strconv.ParseUint(string(buf[start:j]), 10, 64); err == nil {
						return id
					}
					break
				}
			}
		}
	}
	return 0
}

// onRuntimeThread reports whether the current goroutine is the runtime thread.
// The runtime thread is the one that calls start(), which sets runtimeGoroutine.
// Before start() is called, no goroutine is the runtime thread.
func (rt *Runtime) onRuntimeThread() bool {
	if rt.runtimeGoroutine == 0 {
		return false
	}
	return currentGoroutineID() == rt.runtimeGoroutine
}
