package projection

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/layout"
)

type ProjectionCacheKey uint64

type ProjectionOutput struct {
	LayerID            facet.LayerID
	LayerRecipeVersion uint64
	InputModality      facet.InputModality
	ContentScale       float32
	LayerClipPolicy    facet.ClipPolicy
	LayerHitPolicy     facet.HitPolicy
	Placement          facet.PlacementMode
	FacetID            facet.FacetID
	Bounds             gfx.Rect
	ClipRect           gfx.Rect
	Transform          gfx.Transform
	Commands           gfx.CommandList
	HitRegions         []HitRegion
	ChildContext       *ChildProjectionContext
	SelectionGeometry  *SelectionGeometry
	CacheKey           ProjectionCacheKey
}

// OutputSnapshot is a read-only summary of one projected facet in the current frame.
type OutputSnapshot struct {
	FacetID            facet.FacetID
	LayerID            facet.LayerID
	LayerRecipeVersion uint64
	InputModality      facet.InputModality
	ContentScale       float32
	LayerClipPolicy    facet.ClipPolicy
	LayerHitPolicy     facet.HitPolicy
	Placement          facet.PlacementMode
	Bounds             gfx.Rect
	ClipRect           gfx.Rect
	Transform          gfx.Transform
	CommandCount       int
	HitRegionCount     int
	Materialized       bool
}

type HitRegion struct {
	Bounds      gfx.Rect
	Shape       *gfx.Path
	MarkID      facet.MarkID
	FacetID     facet.FacetID
	Cursor      facet.CursorShape
	PassThrough bool
	LayerID     facet.LayerID
	Placement   facet.PlacementMode
	HitPolicy   facet.HitPolicy
	ClipPolicy  facet.ClipPolicy
	ZOrder      int32
}

type ChildProjectionContext struct {
	Transform   gfx.Transform
	ClipBounds  *gfx.Rect
	WorldBounds gfx.Rect
}

type SelectionGeometry struct {
	CaretRect      gfx.Rect
	SelectionRects []gfx.Rect
	CaretVisible   bool
}

type RenderBatchOutput struct {
	FacetID   facet.FacetID
	Commands  gfx.CommandList
	Bounds    gfx.Rect
	Transform gfx.Transform
	Opacity   float32
}

type FrameOutput struct {
	RenderBatchs        []RenderBatchOutput
	HitMap              *HitMap
	SelectionGeometries map[facet.FacetID]*SelectionGeometry
	// Reprojected lists the facets whose projection output was recomputed
	// this frame (cache miss or dirty-forced re-projection). Assembly derives
	// the frame's declared damage regions from this set: a facet that
	// re-projected — including a pure arranged-bounds move with no dirty
	// flag — must have its tiles recomposited even when its batch-ID list
	// per tile is unchanged. Nil on quiet frames (zero re-projections).
	Reprojected map[facet.FacetID]struct{}
}

type FrameInfo struct {
	Number    uint64
	DeltaTime time.Duration
	WallTime  time.Time
}

type System struct {
	outputCache     map[facet.FacetID]*ProjectionOutput
	frameOutputs    []*ProjectionOutput
	dirtyCurrent    map[facet.FacetID]facet.DirtyFlags
	dirtyNext       map[facet.FacetID]facet.DirtyFlags
	dirtyCaptured   bool
	nodesByID       map[facet.FacetID]*projectionNode
	currentHitMap   *HitMap
	ProjectedFacets int
	CacheHits       int
	// EmptyBoundsSkips counts facets gated by the RX-1 FR-1 empty-bounds gate
	// during the most recent Run: non-layer facets arranged to empty bounds
	// that were pruned without projecting.
	EmptyBoundsSkips int
	// CacheMissesByBounds counts projection cache misses whose stale cached
	// output carried different arranged/layer bounds than the current frame —
	// a bounds change invalidating an entry (FR-1 freshness).
	CacheMissesByBounds int
	// RX-1 P5 hot-path counters (frame-scoped; zero in the quiet steady state).
	// GateCount counts projection nodes evaluated against the empty-bounds
	// gate; PruneCount counts nodes pruned by the gate or a poisoned skip.
	// CollectCount counts OnCollect invocations; MaterializeCount counts
	// outputs that produced commands; HitTestCount counts hit roles tested.
	GateCount        int
	PruneCount       int
	CollectCount     int
	MaterializeCount int
	HitTestCount     int
	runtime          facet.RuntimeServices
	layerResolver    LayerResolver
	recoverySurface  facetRecovery
	// nodeList is the retained pre-order node buffer used to aggregate
	// subtreeHasLayer when lifecycle invalidates projection topology.
	nodeList         []*projectionNode
	topologyRoot     facet.FacetImpl
	topologyRootNode *projectionNode
	topologyDirty    bool
	TopologyRebuilds uint64
	walkStack        []projectionWalkFrame
	// reprojectedScratch collects the facet IDs re-projected during the
	// current walk (reused buffer; FrameOutput.Reprojected is the per-frame
	// published view for assembly's declared damage).
	reprojectedScratch []facet.FacetID
	// frameNumber is the most recent Run's frame number (for LURPIC_TRACE=gates
	// gate-decision tracing, NFR-8).
	frameNumber uint64
	// gatesTraceLines counts gate-trace lines emitted in the current frame,
	// capped at gateTraceLineCap.
	gatesTraceLines int
}

type runtimeStateSource interface {
	CurrentContentScale() float32
	CurrentInputModality() facet.InputModality
}

// facetRecovery is the runtime-internal per-facet panic-recovery surface. The
// real *runtime.Runtime satisfies this; test doubles and stubs do not, and for
// them recovery is a no-op (their facets are trusted). Projection reaches it
// through the same type-assertion escape hatch as runtimeStateSource and
// LayerResolver, so the public facet.RuntimeServices interface is not widened.
type facetRecovery interface {
	// GuardedProject runs a projection callback with facet quarantine. It
	// returns true when fn ran to completion, false when the facet was already
	// quarantined or was quarantined by the call itself.
	GuardedProject(id facet.FacetID, role string, fn func()) bool
	// IsPoisoned reports whether a facet has been quarantined.
	IsPoisoned(id facet.FacetID) bool
}

// recovery returns the runtime's facet-recovery surface, or nil when the
// runtime does not expose one (e.g. projection tests driven by stub runtimes).
// The surface is resolved once in SetRuntime so the hot path is a field read.
func (s *System) recovery() facetRecovery {
	if s == nil {
		return nil
	}
	return s.recoverySurface
}

// runGuarded invokes a projection callback through the runtime's facet
// recovery surface when one is available; with a stub runtime it invokes the
// callback directly, preserving the pre-recovery behavior.
func (s *System) runGuarded(id facet.FacetID, role string, fn func()) {
	if rec := s.recovery(); rec != nil {
		rec.GuardedProject(id, role, fn)
		return
	}
	fn()
}

// LayerResolver provides resolved layer snapshots to the projection pass.
type LayerResolver interface {
	ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool)
}

type projectionNode struct {
	impl     facet.FacetImpl
	base     *facet.Facet
	parent   *projectionNode
	children []*projectionNode
	// subtreeHasLayer records whether this node or any descendant is a
	// layer-attached facet (RX-1 Q1). The empty-bounds gate must descend into
	// gated subtrees that contain a layer: a layer resolves its own bounds via
	// layerCtx and may legitimately paint outside a zero-arranged host.
	subtreeHasLayer bool
}

type projectionWalkFrame struct {
	node            *projectionNode
	parentTransform gfx.Transform
	parentChildCtx  *ChildProjectionContext
}

func NewSystem() *System {
	return &System{
		outputCache:   make(map[facet.FacetID]*ProjectionOutput),
		dirtyCurrent:  make(map[facet.FacetID]facet.DirtyFlags),
		dirtyNext:     make(map[facet.FacetID]facet.DirtyFlags),
		nodesByID:     make(map[facet.FacetID]*projectionNode),
		topologyDirty: true,
	}
}

// MarkDirtyFlags records an explicit runtime invalidation in the generation
// that can still be consumed safely.
func (s *System) MarkDirtyFlags(id facet.FacetID, flags facet.DirtyFlags) {
	if s == nil || id == 0 || flags == 0 {
		return
	}
	if s.dirtyCurrent == nil {
		s.dirtyCurrent = make(map[facet.FacetID]facet.DirtyFlags)
	}
	if s.dirtyNext == nil {
		s.dirtyNext = make(map[facet.FacetID]facet.DirtyFlags)
	}
	dirty := s.dirtyCurrent
	if s.dirtyCaptured {
		dirty = s.dirtyNext
	}
	dirty[id] |= flags
}

// BeginDirtyGeneration seals the current invalidation cut before phases that
// may publish work for the next frame.
func (s *System) BeginDirtyGeneration() {
	if s == nil {
		return
	}
	s.dirtyCaptured = true
	clear(s.dirtyNext)
}

// SealDirtyGeneration retires consumed invalidations and promotes any work
// published after the frame cut.
func (s *System) SealDirtyGeneration() {
	if s == nil || !s.dirtyCaptured {
		return
	}
	clear(s.dirtyCurrent)
	s.dirtyCurrent, s.dirtyNext = s.dirtyNext, s.dirtyCurrent
	clear(s.dirtyNext)
	s.dirtyCaptured = false
}

// InvalidateTopology tells the runtime-owned projection graph that lifecycle
// or hosted-child publication changed. It is called only at those canonical
// mutation seams; unchanged frames reuse the retained graph.
func (s *System) InvalidateTopology() {
	if s != nil {
		s.topologyDirty = true
	}
}

// SetRuntime provides the runtime services exposed to ProjectionRole callbacks.
func (s *System) SetRuntime(rt facet.RuntimeServices) {
	if s == nil {
		return
	}
	s.runtime = rt
	if lr, ok := rt.(LayerResolver); ok {
		s.layerResolver = lr
	} else {
		s.layerResolver = nil
	}
	if rec, ok := rt.(facetRecovery); ok {
		s.recoverySurface = rec
	} else {
		s.recoverySurface = nil
	}
}

func (s *System) MarkDirty(id facet.FacetID) {
	if s == nil || id == 0 {
		return
	}
	s.MarkDirtyFlags(id, facet.DirtyAll)
}

func (s *System) Run(root facet.FacetImpl, frame FrameInfo) *FrameOutput {
	if s == nil {
		return &FrameOutput{}
	}
	if s.outputCache == nil {
		s.outputCache = make(map[facet.FacetID]*ProjectionOutput)
	}
	s.ProjectedFacets = 0
	s.CacheHits = 0
	s.EmptyBoundsSkips = 0
	s.CacheMissesByBounds = 0
	s.GateCount = 0
	s.PruneCount = 0
	s.CollectCount = 0
	s.MaterializeCount = 0
	s.HitTestCount = 0
	s.frameNumber = frame.Number
	s.gatesTraceLines = 0
	if s.topologyRoot != root {
		s.topologyRoot = root
		s.topologyDirty = true
	}
	topologyRebuilt := s.topologyDirty
	if s.topologyDirty {
		s.topologyRootNode = s.buildProjectionTree(root)
		s.topologyDirty = false
		s.TopologyRebuilds++
	}
	rootNode := s.topologyRootNode
	dirty := s.dirtyCurrent
	if !topologyRebuilt && len(dirty) == 0 {
		s.CacheHits = len(s.frameOutputs)
		s.reprojectedScratch = s.reprojectedScratch[:0]
	} else {
		clear(s.frameOutputs)
		s.frameOutputs = s.frameOutputs[:0]
		s.reprojectedScratch = s.reprojectedScratch[:0]
		if rootNode != nil {
			s.propagateProjectionDirty(dirty)
			s.walkNode(rootNode, gfx.Identity(), nil, dirty)
			s.clearDirtySnapshot(dirty)
		}
	}
	out := s.assembleFrameOutput()
	s.currentHitMap = out.HitMap
	return out
}

// Reset clears cached projection outputs and invalidation state.
// It is used by low-memory handling to shed recoverable projection caches.
func (s *System) Reset() {
	if s == nil {
		return
	}
	s.outputCache = make(map[facet.FacetID]*ProjectionOutput)
	s.frameOutputs = nil
	clear(s.dirtyCurrent)
	clear(s.dirtyNext)
	s.dirtyCurrent = make(map[facet.FacetID]facet.DirtyFlags)
	s.dirtyNext = make(map[facet.FacetID]facet.DirtyFlags)
	s.dirtyCaptured = false
	s.currentHitMap = nil
	clear(s.nodeList)
	s.nodeList = s.nodeList[:0]
	clear(s.walkStack[:cap(s.walkStack)])
	s.walkStack = nil
	s.topologyRoot = nil
	s.topologyRootNode = nil
	s.topologyDirty = true
	clear(s.nodesByID)
	s.nodesByID = make(map[facet.FacetID]*projectionNode)
	s.ProjectedFacets = 0
	s.CacheHits = 0
	s.EmptyBoundsSkips = 0
	s.CacheMissesByBounds = 0
	s.GateCount = 0
	s.PruneCount = 0
	s.CollectCount = 0
	s.MaterializeCount = 0
	s.HitTestCount = 0
}

// ForgetFacet releases projection output retained for a disposed lifetime.
func (s *System) ForgetFacet(id facet.FacetID) {
	if s == nil || id == 0 {
		return
	}
	delete(s.outputCache, id)
	delete(s.nodesByID, id)
	delete(s.dirtyCurrent, id)
	delete(s.dirtyNext, id)
	s.topologyDirty = true
}

// CurrentHitMap returns the hit map computed during the most recent run.
func (s *System) CurrentHitMap() *HitMap {
	if s == nil {
		return nil
	}
	return s.currentHitMap
}

// OutputSnapshots returns a stable snapshot of the current frame outputs.
func (s *System) OutputSnapshots() []OutputSnapshot {
	if s == nil || len(s.frameOutputs) == 0 {
		return nil
	}
	out := make([]OutputSnapshot, 0, len(s.frameOutputs))
	for _, po := range s.frameOutputs {
		if po == nil {
			continue
		}
		out = append(out, OutputSnapshot{
			FacetID:            po.FacetID,
			LayerID:            po.LayerID,
			LayerRecipeVersion: po.LayerRecipeVersion,
			InputModality:      po.InputModality,
			ContentScale:       po.ContentScale,
			LayerClipPolicy:    po.LayerClipPolicy,
			LayerHitPolicy:     po.LayerHitPolicy,
			Placement:          po.Placement,
			Bounds:             po.Bounds,
			ClipRect:           po.ClipRect,
			Transform:          po.Transform,
			CommandCount:       po.Commands.Len(),
			HitRegionCount:     len(po.HitRegions),
			Materialized:       true,
		})
	}
	return out
}

// LastOutputCommands returns the command list projected for a facet in the most
// recent frame, or nil when the facet produced no output. It lets tests and
// diagnostics inspect retained projection output without re-invoking projection
// callbacks outside the phase (RX-1 P5: project output retained).
func (s *System) LastOutputCommands(id facet.FacetID) []gfx.Command {
	if s == nil {
		return nil
	}
	for _, po := range s.frameOutputs {
		if po != nil && po.FacetID == id {
			return po.Commands.Commands
		}
	}
	return nil
}

// SetCurrentHitMap replaces the cached hit map. It is used by runtime tests and
// allows callers to inject a precomputed traversal map.
func (s *System) SetCurrentHitMap(m *HitMap) {
	if s == nil {
		return
	}
	s.currentHitMap = m
}

func (s *System) buildProjectionTree(root facet.FacetImpl) *projectionNode {
	if root == nil {
		return nil
	}
	base := root.Base()
	if base == nil {
		return nil
	}
	node := &projectionNode{
		impl: root,
		base: base,
	}
	if s.nodesByID == nil {
		s.nodesByID = make(map[facet.FacetID]*projectionNode)
	}
	clear(s.nodesByID)
	clear(s.nodeList)
	s.nodeList = s.nodeList[:0]
	type buildFrame struct {
		impl facet.FacetImpl
		node *projectionNode
	}
	stack := []buildFrame{{impl: root, node: node}}
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.impl == nil || frame.node == nil || frame.node.base == nil {
			continue
		}
		s.nodeList = append(s.nodeList, frame.node)
		s.nodesByID[frame.node.base.ID()] = frame.node
		if !frame.node.base.HasChildren() {
			continue
		}
		frame.node.children = frame.node.children[:0]
		frame.node.base.VisitChildren(func(child facet.FacetImpl) bool {
			if child == nil || child.Base() == nil {
				return true
			}
			childNode := &projectionNode{
				impl:   child,
				base:   child.Base(),
				parent: frame.node,
			}
			frame.node.children = append(frame.node.children, childNode)
			return true
		})
		for i := len(frame.node.children) - 1; i >= 0; i-- {
			stack = append(stack, buildFrame{impl: frame.node.children[i].impl, node: frame.node.children[i]})
		}
	}
	// Aggregate subtreeHasLayer bottom-up. The node list is in pre-order, so a
	// parent always precedes its descendants; iterating in reverse visits every
	// descendant before its parent, which is exactly the order a post-order
	// aggregation needs — without recursion or a second tree walk.
	for i := len(s.nodeList) - 1; i >= 0; i-- {
		n := s.nodeList[i]
		has := s.isLayer(n.base.ID())
		for _, child := range n.children {
			if child != nil && child.subtreeHasLayer {
				has = true
			}
		}
		n.subtreeHasLayer = has
	}
	return node
}

// isLayer reports whether the runtime resolved a projection layer for the
// facet. With no layer resolver installed (isolated projection tests) no facet
// is a layer.
func (s *System) isLayer(id facet.FacetID) bool {
	if s == nil || s.layerResolver == nil {
		return false
	}
	_, ok := s.layerResolver.ResolveProjectionLayer(id)
	return ok
}

// Safety: walkNode is called from the runtime goroutine during the projection
// phase (phase 8 of the frame pipeline). The facet tree is read-only during
// this phase — signal delivery (phase 6) and layout (phase 7) both complete
// before projection begins. The runtime sets projectionInProgress to true at
// the start of phase 8 and false at the end; any store mutation during
// projection panics via store.SetProjectionActiveCheck. Individual facet
// callbacks (OnProject/OnCollect/OnHitTest registration) are recovered by the
// runtime's facetRecovery surface so a panicking mark quarantines only its own
// subtree instead of terminating the run.
func (s *System) walkNode(node *projectionNode, parentTransform gfx.Transform, parentChildCtx *ChildProjectionContext, dirty map[facet.FacetID]facet.DirtyFlags) {
	if node == nil || node.base == nil || node.impl == nil {
		return
	}
	rec := s.recovery()
	stack := s.walkStack[:0]
	stack = append(stack, projectionWalkFrame{node: node, parentTransform: parentTransform, parentChildCtx: parentChildCtx})
	for len(stack) > 0 {
		frame := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if frame.node == nil || frame.node.base == nil || frame.node.impl == nil {
			continue
		}
		base := frame.node.base
		facetID := base.ID()
		resolvedTransform := frame.parentTransform
		bounds := gfx.Rect{}
		layerCtx, hasLayer := s.resolveLayerContext(frame.node.impl, frame.parentTransform)
		if hasLayer {
			resolvedTransform = layerCtx.Transform
			bounds = layerCtx.Bounds
		} else {
			if viewport := base.ViewportRole(); viewport != nil {
				resolvedTransform = resolvedTransform.Multiply(viewport.Transform)
			}
			if layoutRole := base.LayoutRole(); layoutRole != nil {
				bounds = layoutRole.ArrangedBounds
			}
		}

		// RX-1 FR-1 empty-bounds gate. A non-layer facet with a LayoutRole
		// arranged to empty bounds presents nothing: emit an empty output and
		// do not project it. Without the gate an unguarded mark's OnProject
		// would re-run at empty bounds (the cache key already carries bounds,
		// so the gated re-arrange is a cache miss) and contribute stale
		// commands from a hidden host (A-1/A-2/A-3). The empty output still
		// participates in pre-order output bookkeeping; zero bounds contribute
		// no commands, no hit regions, and
		// no dirty region. Facets without a LayoutRole have no arranged bounds
		// to be gated by (the stage/host gating model arranges hosts to empty);
		// they keep their current projection behavior. Layer facets are exempt
		// from the empty-bounds gate — they resolve their own bounds via
		// layerCtx — and are instead gated by mount state (RX-1 Q4): a layer
		// mounted via AttachLayer whose Mount store reads false is skipped
		// entirely, no projection and no hit. The runtime's layer pass also
		// skips unmounted layers (no measure/arrange/projection-layer entry, so
		// hasLayer is false here), so the mount gate is what keeps a stale
		// arranged bounds from a previous mount from resurrecting pixels.
		// mountGated is true only for layer-attached facets whose Mount store
		// reads false. The non-layer hot path is a single cheap IsLayer() read;
		// the LayerAttachment copy is taken only for real layer facets.
		mountGated := false
		if base.IsLayer() {
			if att := base.LayerAttachment(); att.Mount != nil && !att.Mount.Get() {
				mountGated = true
			}
		}
		gated := mountGated || (!hasLayer && base.LayoutRole() != nil && bounds.IsEmpty())
		s.GateCount++
		var output *ProjectionOutput
		if gated {
			if mountGated {
				// Mount-gated layers are counted by the runtime layer pass
				// (FrameStats.LayersUnmountedSkips, NFR-8); they are not FR-1
				// empty-bounds skips, so they do not inflate EmptyBoundsSkips.
				s.traceGate(facetID, "mount")
			} else {
				s.addEmptyBoundsSkip()
				s.traceGate(facetID, "empty-bounds")
			}
			s.PruneCount++
			output = &ProjectionOutput{FacetID: facetID, Bounds: gfx.Rect{}}
		} else {
			cacheKey := s.computeCacheKey(frame.node.impl, resolvedTransform, frame.parentChildCtx, layerCtx, hasLayer)
			output = s.loadCachedOutput(facetID)
			if output == nil || output.CacheKey != cacheKey || s.isDirtyWithMap(facetID, dirty) {
				if output != nil && output.CacheKey != cacheKey && output.Bounds != bounds {
					s.addCacheMissByBounds()
				}
				output = s.project(frame.node.impl, resolvedTransform, bounds, frame.parentChildCtx, cacheKey, layerCtx, hasLayer)
				s.storeCachedOutput(facetID, output)
				s.addProjectedFacet()
				s.reprojectedScratch = append(s.reprojectedScratch, facetID)
			} else {
				s.addCacheHit()
			}
		}
		s.frameOutputs = append(s.frameOutputs, output)
		children := frame.node.children
		if len(children) == 0 {
			continue
		}
		if gated && !frame.node.subtreeHasLayer {
			// Full prune: a gated subtree with no layer facet is skipped
			// wholesale (FR-1). A gated subtree that DOES hold a layer is
			// descended into so the layer can resolve itself; its non-layer
			// descendants are themselves empty-arranged and gate individually.
			continue
		}
		childCtx := output.ChildContext
		for i := len(children) - 1; i >= 0; i-- {
			if rec != nil && children[i] != nil && children[i].base != nil && rec.IsPoisoned(children[i].base.ID()) {
				// Do not descend into a quarantined subtree.
				continue
			}
			stack = append(stack, projectionWalkFrame{
				node:            children[i],
				parentTransform: resolvedTransform,
				parentChildCtx:  childCtx,
			})
		}
	}
	clear(stack[:cap(stack)])
	s.walkStack = stack[:0]
}

func (s *System) loadCachedOutput(id facet.FacetID) *ProjectionOutput {
	if s == nil || id == 0 {
		return nil
	}
	return s.outputCache[id]
}

func (s *System) storeCachedOutput(id facet.FacetID, output *ProjectionOutput) {
	if s == nil || id == 0 || output == nil {
		return
	}
	s.outputCache[id] = output
}

func (s *System) addProjectedFacet() {
	if s == nil {
		return
	}
	s.ProjectedFacets++
}

func (s *System) addCacheHit() {
	if s == nil {
		return
	}
	s.CacheHits++
}

func (s *System) addEmptyBoundsSkip() {
	if s == nil {
		return
	}
	s.EmptyBoundsSkips++
}

func (s *System) addCacheMissByBounds() {
	if s == nil {
		return
	}
	s.CacheMissesByBounds++
}

func (s *System) project(
	impl facet.FacetImpl,
	resolvedTransform gfx.Transform,
	bounds gfx.Rect,
	parentChildCtx *ChildProjectionContext,
	cacheKey ProjectionCacheKey,
	layerCtx facet.ProjectionLayer,
	hasLayer bool,
) *ProjectionOutput {
	base := impl.Base()
	output := &ProjectionOutput{
		FacetID:   base.ID(),
		Bounds:    bounds,
		Transform: resolvedTransform,
		CacheKey:  cacheKey,
	}
	if rec := s.recovery(); rec != nil && rec.IsPoisoned(base.ID()) {
		// The facet (or one of its ancestors) has been quarantined: contribute
		// nothing to the frame. The walker already skips poisoned subtrees, so
		// this only fires when a facet is re-projected while poisoned.
		return output
	}
	if pr := base.ProjectionRole(); pr != nil && pr.OnProject != nil {
		ctx := facet.ProjectionContext{
			Bounds:        bounds,
			Viewport:      base.ViewportRole(),
			Runtime:       s.runtime,
			ContentScale:  s.currentContentScale(),
			InputModality: s.currentInputModality(),
		}
		if hasLayer {
			ctx.Layer = layerCtx
		}
		s.runGuarded(base.ID(), "project", func() {
			if cmds := pr.Project(ctx); cmds != nil {
				output.Commands = *cmds
			}
		})
	} else if rr := base.RenderRole(); rr != nil && rr.OnCollect != nil {
		s.CollectCount++
		s.runGuarded(base.ID(), "collect", func() {
			if cmds := rr.Collect(bounds); cmds != nil {
				output.Commands = *cmds
			}
		})
	}
	if len(output.Commands.Commands) > 0 {
		s.MaterializeCount++
	}
	if hr := base.HitRole(); hr != nil && hr.OnHitTest != nil {
		s.HitTestCount++
		if rec := s.recovery(); rec == nil || !rec.IsPoisoned(base.ID()) {
			output.HitRegions = []HitRegion{{
				Bounds: bounds,
				MarkID: 0,
				Cursor: facet.CursorDefault,
			}}
		}
	}
	if tr := base.TextRole(); tr != nil {
		output.SelectionGeometry = selectionGeometryFromTextRole(tr)
	}
	worldBounds := resolvedTransform.TransformRect(bounds)
	effectiveClip, hasClip := s.resolveEffectiveClip(base, resolvedTransform, bounds, worldBounds, parentChildCtx, layerCtx, hasLayer)
	if hasLayer {
		output.LayerID = layerCtx.LayerID
		output.LayerRecipeVersion = layerCtx.RecipeVersion
		output.LayerClipPolicy = layerCtx.ClipPolicy
		output.LayerHitPolicy = facet.HitPolicy(layerCtx.HitPolicy)
	}
	if hasClip {
		output.ClipRect = effectiveClip
	}
	output.ContentScale = s.currentContentScale()
	output.InputModality = s.currentInputModality()
	if base.ViewportRole() != nil || base.HasChildren() {
		childCtx := &ChildProjectionContext{
			Transform:   resolvedTransform,
			WorldBounds: resolvedTransform.TransformRect(bounds),
		}
		if hasClip {
			clip := effectiveClip
			childCtx.ClipBounds = &clip
		}
		output.ChildContext = childCtx
	} else if parentChildCtx != nil {
		clone := *parentChildCtx
		output.ChildContext = &clone
	}
	if hasClip && output.Commands.Len() > 0 {
		localClip, ok := clipToLocal(effectiveClip, resolvedTransform)
		if ok && !localClip.IsEmpty() {
			output.Commands = wrapCommandsWithClip(output.Commands, localClip)
		}
	}
	if hasClip && len(output.HitRegions) > 0 {
		localClip, ok := clipToLocal(effectiveClip, resolvedTransform)
		if ok && !localClip.IsEmpty() {
			for i := range output.HitRegions {
				if output.HitRegions[i].Shape != nil {
					continue
				}
				output.HitRegions[i].Bounds = intersectRects(output.HitRegions[i].Bounds, localClip)
			}
		}
	}
	if output.Commands.Len() == 0 && len(output.HitRegions) == 0 {
		output.SelectionGeometry = nil
	}
	return output
}

func selectionGeometryFromTextRole(role *facet.TextRole) *SelectionGeometry {
	if role == nil {
		return nil
	}
	geom := role.CollectSelectionGeometry()
	if geom == nil {
		return nil
	}
	out := &SelectionGeometry{
		CaretRect:    geom.CaretRect,
		CaretVisible: geom.CaretVisible,
	}
	if len(geom.SelectionRects) > 0 {
		out.SelectionRects = make([]gfx.Rect, len(geom.SelectionRects))
		copy(out.SelectionRects, geom.SelectionRects)
	}
	return out
}

func (s *System) assembleFrameOutput() *FrameOutput {
	out := &FrameOutput{
		SelectionGeometries: collectSelectionGeometries(s.frameOutputs),
		HitMap:              buildHitMap(s.frameOutputs),
	}
	if len(s.reprojectedScratch) > 0 {
		reprojected := make(map[facet.FacetID]struct{}, len(s.reprojectedScratch))
		for _, id := range s.reprojectedScratch {
			reprojected[id] = struct{}{}
		}
		out.Reprojected = reprojected
	}
	for _, po := range s.frameOutputs {
		if po == nil {
			continue
		}
		if po.Commands.Len() == 0 {
			continue
		}
		out.RenderBatchs = append(out.RenderBatchs, RenderBatchOutput{
			FacetID:   po.FacetID,
			Commands:  po.Commands,
			Bounds:    po.Bounds,
			Transform: po.Transform,
			Opacity:   1,
		})
	}
	return out
}

func (s *System) isDirtyWithMap(id facet.FacetID, dirty map[facet.FacetID]facet.DirtyFlags) bool {
	if dirty == nil || id == 0 {
		return false
	}
	return dirty[id] != 0
}

func (s *System) computeCacheKey(
	f facet.FacetImpl,
	resolvedTransform gfx.Transform,
	parentChildCtx *ChildProjectionContext,
	layerCtx facet.ProjectionLayer,
	hasLayer bool,
) ProjectionCacheKey {
	base := f.Base()
	if base == nil {
		return 0
	}
	h := hashutil.NewCacheKeyBuilder()
	h.WriteUint64(uint64(base.ID()))
	hashTransform(&h, resolvedTransform)
	if parentChildCtx == nil {
		h.WriteUint8(0)
	} else {
		h.WriteUint8(1)
		hashTransform(&h, parentChildCtx.Transform)
		if parentChildCtx.ClipBounds == nil {
			h.WriteUint8(0)
		} else {
			h.WriteUint8(1)
			hashRect(&h, *parentChildCtx.ClipBounds)
		}
		hashRect(&h, parentChildCtx.WorldBounds)
	}
	if hasLayer {
		h.WriteUint8(1)
		h.WriteUint64(uint64(layerCtx.LayerID))
		hashRect(&h, layerCtx.Bounds)
		hashTransform(&h, layerCtx.Transform)
		hashRect(&h, layerCtx.ClipRect)
		h.WriteUint8(uint8(layerCtx.ClipPolicy))
		h.WriteUint64(layerCtx.RecipeVersion)
	} else {
		h.WriteUint8(0)
	}
	h.WriteFloat32(s.currentContentScale())
	h.WriteUint8(uint8(s.currentInputModality()))
	if layout := base.LayoutRole(); layout != nil {
		h.WriteUint8(1)
		hashRect(&h, layout.ArrangedBounds)
		h.WriteFloat32(layout.Constraints.MinSize.W)
		h.WriteFloat32(layout.Constraints.MinSize.H)
		h.WriteFloat32(layout.Constraints.MaxSize.W)
		h.WriteFloat32(layout.Constraints.MaxSize.H)
	} else {
		h.WriteUint8(0)
	}
	if render := base.RenderRole(); render != nil {
		h.WriteUint8(1)
		h.WriteUint64(uint64(render.RenderBatchID))
	} else {
		h.WriteUint8(0)
	}
	if hit := base.HitRole(); hit != nil {
		h.WriteUint8(1)
	} else {
		h.WriteUint8(0)
	}
	if viewport := base.ViewportRole(); viewport != nil {
		h.WriteUint8(1)
		hashTransform(&h, viewport.Transform)
		hashRect(&h, viewport.WorldBounds)
	} else {
		h.WriteUint8(0)
	}
	versions := base.SubscribedVersions()
	h.WriteUint64(uint64(len(versions)))
	for _, v := range versions {
		h.WriteUint64(uint64(v))
	}
	return ProjectionCacheKey(h.Sum())
}

func (s *System) resolveLayerContext(impl facet.FacetImpl, parentTransform gfx.Transform) (facet.ProjectionLayer, bool) {
	if s == nil || impl == nil || impl.Base() == nil {
		return facet.ProjectionLayer{}, false
	}
	if s.layerResolver != nil {
		if layer, ok := s.layerResolver.ResolveProjectionLayer(impl.Base().ID()); ok {
			return layer, true
		}
	}
	base := impl.Base()
	layer := facet.ProjectionLayer{}
	if layoutRole := base.LayoutRole(); layoutRole != nil {
		layer.Bounds = layoutRole.ArrangedBounds
	}
	if viewport := base.ViewportRole(); viewport != nil {
		layer.Transform = parentTransform.Multiply(viewport.Transform)
	} else {
		layer.Transform = parentTransform
	}
	if !layer.Bounds.IsEmpty() {
		layer.ClipRect = layer.Transform.TransformRect(layer.Bounds)
	}
	return layer, false
}

func (s *System) currentContentScale() float32 {
	if s == nil || s.runtime == nil {
		return 0
	}
	if src, ok := s.runtime.(runtimeStateSource); ok {
		return src.CurrentContentScale()
	}
	return 0
}

func (s *System) currentInputModality() facet.InputModality {
	if s == nil || s.runtime == nil {
		return facet.InputModalityUnknown
	}
	if src, ok := s.runtime.(runtimeStateSource); ok {
		return src.CurrentInputModality()
	}
	return facet.InputModalityUnknown
}

func (s *System) resolveEffectiveClip(
	base *facet.Facet,
	resolvedTransform gfx.Transform,
	bounds gfx.Rect,
	worldBounds gfx.Rect,
	parentChildCtx *ChildProjectionContext,
	layerCtx facet.ProjectionLayer,
	hasLayer bool,
) (gfx.Rect, bool) {
	var clip gfx.Rect
	hasClip := false
	if parentChildCtx != nil && parentChildCtx.ClipBounds != nil && !parentChildCtx.ClipBounds.IsEmpty() {
		clip = *parentChildCtx.ClipBounds
		hasClip = true
	}
	if hasLayer && !layerCtx.ClipRect.IsEmpty() {
		clip, hasClip = layout.IntersectClipRects(clip, hasClip, layerCtx.ClipRect)
	}
	if base != nil {
		if layoutRole := base.LayoutRole(); layoutRole != nil {
			if groupClip, ok := layout.GroupClipRect(worldBounds, layoutRole.Parent); ok {
				clip, hasClip = layout.IntersectClipRects(clip, hasClip, groupClip)
			}
		} else if base.ViewportRole() != nil || base.HasChildren() {
			clip, hasClip = layout.IntersectClipRects(clip, hasClip, worldBounds)
		}
	}
	if hasClip {
		return clip, true
	}
	_ = resolvedTransform
	_ = bounds
	return gfx.Rect{}, false
}

func clipToLocal(clipWorld gfx.Rect, transform gfx.Transform) (gfx.Rect, bool) {
	if clipWorld.IsEmpty() {
		return gfx.Rect{}, false
	}
	inv, ok := transform.Inverse()
	if !ok {
		return clipWorld, true
	}
	return inv.TransformRect(clipWorld), true
}

func wrapCommandsWithClip(cmds gfx.CommandList, clip gfx.Rect) gfx.CommandList {
	if clip.IsEmpty() || cmds.Len() == 0 {
		return cmds
	}
	wrapped := gfx.CommandList{}
	wrapped.Add(gfx.PushClipRect{Rect: clip})
	for _, cmd := range cmds.Commands {
		wrapped.Add(cmd)
	}
	wrapped.Add(gfx.PopClip{})
	return wrapped
}

func intersectRects(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	minX := a.Min.X
	if b.Min.X > minX {
		minX = b.Min.X
	}
	minY := a.Min.Y
	if b.Min.Y > minY {
		minY = b.Min.Y
	}
	maxX := a.Max.X
	if b.Max.X < maxX {
		maxX = b.Max.X
	}
	maxY := a.Max.Y
	if b.Max.Y < maxY {
		maxY = b.Max.Y
	}
	rect := gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
	if rect.IsEmpty() {
		return gfx.Rect{}
	}
	return rect
}

type hitEntry struct {
	facetID    facet.FacetID
	layerID    facet.LayerID
	layerOrder int
	placement  facet.PlacementMode
	hitPolicy  facet.HitPolicy
	clipPolicy facet.ClipPolicy
	zOrder     int32
	clipRect   gfx.Rect
	transform  gfx.Transform
	regions    []HitRegion
	projected  bool
}

// HitMapEntry is a synthetic hit-map entry used by tests and input routing helpers.
type HitMapEntry struct {
	FacetID    facet.FacetID
	LayerID    facet.LayerID
	LayerOrder int
	Placement  facet.PlacementMode
	HitPolicy  facet.HitPolicy
	ClipPolicy facet.ClipPolicy
	ZOrder     int32
	ClipRect   gfx.Rect
	Transform  gfx.Transform
	Regions    []HitRegion
}

type HitMap struct {
	entries []hitEntry
}

type HitTestResult struct {
	FacetID facet.FacetID
	MarkID  facet.MarkID
	Cursor  facet.CursorShape
}

// NewHitMap constructs a hit map from explicit entries.
func NewHitMap(entries ...HitMapEntry) *HitMap {
	if len(entries) == 0 {
		return &HitMap{}
	}
	out := &HitMap{entries: make([]hitEntry, 0, len(entries))}
	for _, entry := range entries {
		regions := make([]HitRegion, len(entry.Regions))
		copy(regions, entry.Regions)
		for i := range regions {
			if regions[i].FacetID == 0 {
				regions[i].FacetID = entry.FacetID
			}
		}
		out.entries = append(out.entries, hitEntry{
			facetID:    entry.FacetID,
			layerID:    entry.LayerID,
			layerOrder: entry.LayerOrder,
			placement:  entry.Placement,
			hitPolicy:  entry.HitPolicy,
			clipPolicy: entry.ClipPolicy,
			zOrder:     entry.ZOrder,
			clipRect:   entry.ClipRect,
			transform:  entry.Transform,
			regions:    regions,
		})
	}
	return out
}

// TransformFor returns the transform associated with a facet ID.
func (m *HitMap) TransformFor(facetID facet.FacetID) (gfx.Transform, bool) {
	if m == nil {
		return gfx.Transform{}, false
	}
	for _, entry := range m.entries {
		if entry.facetID == facetID {
			return entry.transform, true
		}
	}
	return gfx.Transform{}, false
}

// Entries returns a copy of the hit-map entries in front-to-back order.
func (m *HitMap) Entries() []HitMapEntry {
	if m == nil || len(m.entries) == 0 {
		return nil
	}
	out := make([]HitMapEntry, 0, len(m.entries))
	for _, entry := range m.entries {
		regions := make([]HitRegion, len(entry.regions))
		copy(regions, entry.regions)
		if entry.projected {
			for i := range regions {
				regions[i].FacetID = entry.facetID
				regions[i].LayerID = entry.layerID
				regions[i].Placement = entry.placement
				regions[i].HitPolicy = entry.hitPolicy
				regions[i].ClipPolicy = entry.clipPolicy
			}
		}
		out = append(out, HitMapEntry{
			FacetID:    entry.facetID,
			LayerID:    entry.layerID,
			LayerOrder: entry.layerOrder,
			Placement:  entry.placement,
			HitPolicy:  entry.hitPolicy,
			ClipPolicy: entry.clipPolicy,
			ZOrder:     entry.zOrder,
			ClipRect:   entry.clipRect,
			Transform:  entry.transform,
			Regions:    regions,
		})
	}
	return out
}

func (m *HitMap) HitTest(screenPoint gfx.Point) *HitTestResult {
	if m == nil || len(m.entries) == 0 {
		return nil
	}
	for _, entry := range m.entries {
		local := screenPoint
		if inv, ok := entry.transform.Inverse(); ok {
			local = inv.TransformPoint(screenPoint)
		}
		if !entry.clipRect.IsEmpty() {
			clip := entry.clipRect
			if inv, ok := entry.transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				if entry.hitPolicy == facet.HitBlockBelow {
					return nil
				}
				continue
			}
		}
		hit := false
		var hitRegion *HitRegion
		for _, region := range entry.regions {
			if HitRegionContains(region, local) {
				hit = true
				regionCopy := region
				hitRegion = &regionCopy
				if region.PassThrough || entry.hitPolicy == facet.HitPassThrough {
					continue
				}
				facetID := entry.facetID
				if !entry.projected && region.FacetID != 0 {
					facetID = region.FacetID
				}
				return &HitTestResult{
					FacetID: facetID,
					MarkID:  region.MarkID,
					Cursor:  region.Cursor,
				}
			}
		}
		if hit && entry.hitPolicy == facet.HitBlockBelow {
			if hitRegion != nil {
				facetID := entry.facetID
				if !entry.projected && hitRegion.FacetID != 0 {
					facetID = hitRegion.FacetID
				}
				return &HitTestResult{
					FacetID: facetID,
					MarkID:  hitRegion.MarkID,
					Cursor:  hitRegion.Cursor,
				}
			}
			return &HitTestResult{FacetID: entry.facetID}
		}
		if !hit && entry.hitPolicy == facet.HitBlockBelow {
			return nil
		}
	}
	return nil
}

func buildHitMap(outputs []*ProjectionOutput) *HitMap {
	if len(outputs) == 0 {
		return &HitMap{}
	}
	entries := make([]hitEntry, 0, len(outputs))
	for i := len(outputs) - 1; i >= 0; i-- {
		po := outputs[i]
		if po == nil || len(po.HitRegions) == 0 {
			continue
		}
		entries = append(entries, hitEntry{
			facetID:    po.FacetID,
			layerID:    po.LayerID,
			placement:  po.Placement,
			hitPolicy:  po.LayerHitPolicy,
			clipPolicy: po.LayerClipPolicy,
			transform:  po.Transform,
			regions:    po.HitRegions,
			projected:  true,
		})
	}
	return &HitMap{entries: entries}
}

func collectSelectionGeometries(outputs []*ProjectionOutput) map[facet.FacetID]*SelectionGeometry {
	if len(outputs) == 0 {
		return nil
	}
	out := make(map[facet.FacetID]*SelectionGeometry)
	for _, po := range outputs {
		if po == nil || po.SelectionGeometry == nil {
			continue
		}
		out[po.FacetID] = po.SelectionGeometry
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// propagateProjectionDirty adds hit invalidation for each changed projection.
// Descendants are evaluated through their complete immutable parent-context
// cache keys during the required walk, so unchanged child context does not
// force author projection callbacks. Changed geometry or clips miss those
// keys without inventing an authored dependency.
func (s *System) propagateProjectionDirty(dirty map[facet.FacetID]facet.DirtyFlags) {
	if len(dirty) == 0 {
		return
	}
	for id, flags := range dirty {
		if flags&facet.DirtyProjection == 0 {
			continue
		}
		mergeDirtyFlags(dirty, id, facet.DirtyHit)
	}
}

func (s *System) clearDirtySnapshot(dirty map[facet.FacetID]facet.DirtyFlags) {
	for id, flags := range dirty {
		if node := s.nodesByID[id]; node != nil && node.base != nil {
			node.base.ClearDirty(flags)
		}
	}
}

func mergeDirtyFlags(dirty map[facet.FacetID]facet.DirtyFlags, id facet.FacetID, flags facet.DirtyFlags) bool {
	if dirty == nil || id == 0 || flags == 0 {
		return false
	}
	current := dirty[id]
	next := current | flags
	if next == current {
		return false
	}
	dirty[id] = next
	return true
}

// HitRegionContains reports whether p lies within region.
func HitRegionContains(region HitRegion, p gfx.Point) bool {
	if region.Shape != nil {
		// Shape-aware hit testing is deferred to later phases; use the bounds.
	}
	return region.Bounds.Contains(p)
}

func hashTransform(b *hashutil.CacheKeyBuilder, t gfx.Transform) {
	b.WriteFloat32(t.A)
	b.WriteFloat32(t.B)
	b.WriteFloat32(t.C)
	b.WriteFloat32(t.D)
	b.WriteFloat32(t.TX)
	b.WriteFloat32(t.TY)
}

func hashRect(b *hashutil.CacheKeyBuilder, r gfx.Rect) {
	b.WriteFloat32(r.Min.X)
	b.WriteFloat32(r.Min.Y)
	b.WriteFloat32(r.Max.X)
	b.WriteFloat32(r.Max.Y)
}
