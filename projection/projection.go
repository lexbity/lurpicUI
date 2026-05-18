package projection

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

var _ = text.GlyphRun{}
var _ = signal.Fired
var _ store.Version

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
	ZPriority   int32
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
}

type FrameInfo struct {
	Number    uint64
	DeltaTime time.Duration
	WallTime  time.Time
}

type System struct {
	outputCache     map[facet.FacetID]*ProjectionOutput
	frameOutputs    []*ProjectionOutput
	dirtySet        map[facet.FacetID]struct{}
	currentHitMap   *HitMap
	ProjectedFacets int
	CacheHits       int
	runtime         facet.RuntimeServices
	layerResolver   LayerResolver
}

type runtimeStateSource interface {
	CurrentContentScale() float32
	CurrentInputModality() facet.InputModality
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
}

func NewSystem() *System {
	return &System{
		outputCache: make(map[facet.FacetID]*ProjectionOutput),
		dirtySet:    make(map[facet.FacetID]struct{}),
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
}

func (s *System) MarkDirty(id facet.FacetID) {
	if s == nil || id == 0 {
		return
	}
	if s.dirtySet == nil {
		s.dirtySet = make(map[facet.FacetID]struct{})
	}
	s.dirtySet[id] = struct{}{}
}

func (s *System) Run(root facet.FacetImpl, frame FrameInfo) *FrameOutput {
	if s == nil {
		return &FrameOutput{}
	}
	if s.outputCache == nil {
		s.outputCache = make(map[facet.FacetID]*ProjectionOutput)
	}
	if s.dirtySet == nil {
		s.dirtySet = make(map[facet.FacetID]struct{})
	}
	s.ProjectedFacets = 0
	s.CacheHits = 0
	s.frameOutputs = s.frameOutputs[:0]
	rootNode := buildProjectionTree(root)
	if rootNode != nil {
		dirty := s.collectDirtyFlags(rootNode)
		s.propagateDirty(rootNode, dirty)
		s.walkNode(rootNode, gfx.Identity(), nil, dirty)
		s.clearTreeDirty(rootNode)
	}
	out := s.assembleFrameOutput()
	s.currentHitMap = out.HitMap
	s.dirtySet = make(map[facet.FacetID]struct{})
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
	s.dirtySet = make(map[facet.FacetID]struct{})
	s.currentHitMap = nil
	s.ProjectedFacets = 0
	s.CacheHits = 0
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

// SetCurrentHitMap replaces the cached hit map. It is used by runtime tests and
// allows callers to inject a precomputed traversal map.
func (s *System) SetCurrentHitMap(m *HitMap) {
	if s == nil {
		return
	}
	s.currentHitMap = m
}

func buildProjectionTree(root facet.FacetImpl) *projectionNode {
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
		children := frame.node.base.Children()
		if len(children) == 0 {
			continue
		}
		frame.node.children = make([]*projectionNode, 0, len(children))
		for i := 0; i < len(children); i++ {
			child := children[i]
			if child == nil || child.Base() == nil {
				continue
			}
			childNode := &projectionNode{
				impl:   child,
				base:   child.Base(),
				parent: frame.node,
			}
			frame.node.children = append(frame.node.children, childNode)
		}
		for i := len(frame.node.children) - 1; i >= 0; i-- {
			stack = append(stack, buildFrame{impl: frame.node.children[i].impl, node: frame.node.children[i]})
		}
	}
	return node
}

func (s *System) walkNode(node *projectionNode, parentTransform gfx.Transform, parentChildCtx *ChildProjectionContext, dirty map[facet.FacetID]facet.DirtyFlags) {
	if node == nil || node.base == nil || node.impl == nil {
		return
	}
	type walkFrame struct {
		node            *projectionNode
		parentTransform gfx.Transform
		parentChildCtx  *ChildProjectionContext
	}
	stack := []walkFrame{{node: node, parentTransform: parentTransform, parentChildCtx: parentChildCtx}}
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

		cacheKey := s.computeCacheKey(frame.node.impl, resolvedTransform, frame.parentChildCtx, layerCtx, hasLayer)
		output := s.outputCache[facetID]
		if output == nil || output.CacheKey != cacheKey || s.isDirtyWithMap(facetID, dirty) {
			output = s.project(frame.node.impl, resolvedTransform, bounds, frame.parentChildCtx, cacheKey, layerCtx, hasLayer)
			s.outputCache[facetID] = output
			s.ProjectedFacets++
		} else {
			s.CacheHits++
		}
		s.frameOutputs = append(s.frameOutputs, output)
		childCtx := output.ChildContext
		children := frame.node.children
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, walkFrame{
				node:            children[i],
				parentTransform: resolvedTransform,
				parentChildCtx:  childCtx,
			})
		}
	}
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
		if cmds := pr.Project(ctx); cmds != nil {
			output.Commands = *cmds
		}
	} else if rr := base.RenderRole(); rr != nil && rr.OnCollect != nil {
		if cmds := rr.Collect(bounds); cmds != nil {
			output.Commands = *cmds
		}
	}
	if hr := base.HitRole(); hr != nil && hr.OnHitTest != nil {
		output.HitRegions = []HitRegion{{
			Bounds: bounds,
			MarkID: 0,
			Cursor: facet.CursorDefault,
		}}
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
	if base.ViewportRole() != nil || len(base.Children()) > 0 {
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

func (s *System) isDirty(id facet.FacetID) bool {
	if s == nil || id == 0 {
		return false
	}
	_, ok := s.dirtySet[id]
	return ok
}

func (s *System) isDirtyWithMap(id facet.FacetID, dirty map[facet.FacetID]facet.DirtyFlags) bool {
	if s.isDirty(id) {
		return true
	}
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
		} else if base.ViewportRole() != nil || len(base.Children()) > 0 {
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
	zPriority  int32
	clipRect   gfx.Rect
	transform  gfx.Transform
	regions    []HitRegion
}

// HitMapEntry is a synthetic hit-map entry used by tests and input routing helpers.
type HitMapEntry struct {
	FacetID    facet.FacetID
	LayerID    facet.LayerID
	LayerOrder int
	Placement  facet.PlacementMode
	HitPolicy  facet.HitPolicy
	ClipPolicy facet.ClipPolicy
	ZPriority  int32
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
			zPriority:  entry.ZPriority,
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
		out = append(out, HitMapEntry{
			FacetID:    entry.facetID,
			LayerID:    entry.layerID,
			LayerOrder: entry.layerOrder,
			Placement:  entry.placement,
			HitPolicy:  entry.hitPolicy,
			ClipPolicy: entry.clipPolicy,
			ZPriority:  entry.zPriority,
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
				if region.FacetID != 0 {
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
				if hitRegion.FacetID != 0 {
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
		regions := make([]HitRegion, len(po.HitRegions))
		copy(regions, po.HitRegions)
		for i := range regions {
			regions[i].FacetID = po.FacetID
			regions[i].LayerID = po.LayerID
			regions[i].Placement = po.Placement
			regions[i].HitPolicy = po.LayerHitPolicy
			regions[i].ClipPolicy = po.LayerClipPolicy
		}
		entries = append(entries, hitEntry{
			facetID:    po.FacetID,
			layerID:    po.LayerID,
			placement:  po.Placement,
			hitPolicy:  po.LayerHitPolicy,
			clipPolicy: po.LayerClipPolicy,
			transform:  po.Transform,
			regions:    regions,
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

func (s *System) collectDirtyFlags(node *projectionNode) map[facet.FacetID]facet.DirtyFlags {
	dirty := make(map[facet.FacetID]facet.DirtyFlags)
	stack := []*projectionNode{node}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || n.base == nil {
			continue
		}
		flags := n.base.DirtyFlags()
		if s != nil && s.isDirty(n.base.ID()) {
			flags |= facet.DirtyAll
		}
		if flags != 0 {
			dirty[n.base.ID()] |= flags
		}
		children := n.children
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	if len(dirty) == 0 {
		return nil
	}
	return dirty
}

func (s *System) propagateDirty(node *projectionNode, dirty map[facet.FacetID]facet.DirtyFlags) {
	if node == nil || node.base == nil || dirty == nil {
		return
	}
	changed := true
	for changed {
		changed = false
		stack := []*projectionNode{node}
		for len(stack) > 0 {
			n := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if n == nil || n.base == nil {
				continue
			}
			id := n.base.ID()
			flags := dirty[id]
			if flags&facet.DirtyLayout != 0 {
				if parent := n.parent; parent != nil {
					if mergeDirtyFlags(dirty, parent.base.ID(), facet.DirtyLayout) {
						changed = true
					}
				}
				if s.markSubtreeDirty(n, facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit, dirty) {
					changed = true
				}
			}
			if flags&facet.DirtyProjection != 0 {
				if s.markSubtreeDirty(n, facet.DirtyProjection|facet.DirtyHit, dirty) {
					changed = true
				}
			}
			children := n.children
			for i := len(children) - 1; i >= 0; i-- {
				stack = append(stack, children[i])
			}
		}
	}
}

func (s *System) markSubtreeDirty(node *projectionNode, flags facet.DirtyFlags, dirty map[facet.FacetID]facet.DirtyFlags) bool {
	if node == nil || node.base == nil {
		return false
	}
	changed := false
	stack := []*projectionNode{node}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || n.base == nil {
			continue
		}
		if mergeDirtyFlags(dirty, n.base.ID(), flags) {
			changed = true
		}
		children := n.children
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return changed
}

func (s *System) clearTreeDirty(node *projectionNode) {
	if node == nil || node.base == nil {
		return
	}
	stack := []*projectionNode{node}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if n == nil || n.base == nil {
			continue
		}
		if flags := n.base.DirtyFlags(); flags != 0 {
			n.base.ClearDirty(flags)
		}
		children := n.children
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
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
