package projection

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

var _ = text.GlyphRun{}
var _ = signal.Fired
var _ store.Version

type ProjectionCacheKey uint64

type ProjectionOutput struct {
	FacetID           facet.FacetID
	Bounds            gfx.Rect
	Transform         gfx.Transform
	Commands          gfx.CommandList
	HitRegions        []HitRegion
	ChildContext      *ChildProjectionContext
	SelectionGeometry *SelectionGeometry
	CacheKey          ProjectionCacheKey
}

type HitRegion struct {
	Bounds      gfx.Rect
	Shape       *gfx.Path
	MarkID      facet.MarkID
	Cursor      facet.CursorShape
	PassThrough bool
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

type LayerOutput struct {
	FacetID   facet.FacetID
	Commands  gfx.CommandList
	Bounds    gfx.Rect
	Transform gfx.Transform
	Opacity   float32
}

type FrameOutput struct {
	Layers              []LayerOutput
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

// CurrentHitMap returns the hit map computed during the most recent run.
func (s *System) CurrentHitMap() *HitMap {
	if s == nil {
		return nil
	}
	return s.currentHitMap
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
	for _, child := range base.Children() {
		childNode := buildProjectionTree(child)
		if childNode == nil {
			continue
		}
		childNode.parent = node
		node.children = append(node.children, childNode)
	}
	return node
}

func (s *System) walkNode(node *projectionNode, parentTransform gfx.Transform, parentChildCtx *ChildProjectionContext, dirty map[facet.FacetID]facet.DirtyFlags) {
	if node == nil || node.base == nil || node.impl == nil {
		return
	}
	base := node.base
	facetID := base.ID()
	resolvedTransform := parentTransform
	if viewport := base.ViewportRole(); viewport != nil {
		resolvedTransform = resolvedTransform.Multiply(viewport.Transform)
	}
	bounds := gfx.Rect{}
	if layout := base.LayoutRole(); layout != nil {
		bounds = layout.ArrangedBounds
	}

	cacheKey := s.computeCacheKey(node.impl, resolvedTransform, parentChildCtx)
	output := s.outputCache[facetID]
	if output == nil || output.CacheKey != cacheKey || s.isDirtyWithMap(facetID, dirty) {
		output = s.project(node.impl, resolvedTransform, bounds, parentChildCtx, cacheKey)
		s.outputCache[facetID] = output
		s.ProjectedFacets++
	} else {
		s.CacheHits++
	}
	s.frameOutputs = append(s.frameOutputs, output)
	childCtx := output.ChildContext
	for _, child := range node.children {
		s.walkNode(child, resolvedTransform, childCtx, dirty)
	}
}

func (s *System) project(
	impl facet.FacetImpl,
	resolvedTransform gfx.Transform,
	bounds gfx.Rect,
	parentChildCtx *ChildProjectionContext,
	cacheKey ProjectionCacheKey,
) *ProjectionOutput {
	base := impl.Base()
	output := &ProjectionOutput{
		FacetID:   base.ID(),
		Bounds:    bounds,
		Transform: resolvedTransform,
		CacheKey:  cacheKey,
	}
	if pr := base.ProjectionRole(); pr != nil && pr.OnProject != nil {
		if cmds := pr.Project(facet.ProjectionContext{
			Bounds:   bounds,
			Viewport: base.ViewportRole(),
		}); cmds != nil {
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
	if base.ViewportRole() != nil || len(base.Children()) > 0 {
		childCtx := &ChildProjectionContext{
			Transform:   resolvedTransform,
			WorldBounds: resolvedTransform.TransformRect(bounds),
		}
		if !bounds.IsEmpty() {
			b := bounds
			childCtx.ClipBounds = &b
		}
		output.ChildContext = childCtx
	} else if parentChildCtx != nil {
		clone := *parentChildCtx
		output.ChildContext = &clone
	}
	if output.Commands.Len() == 0 && len(output.HitRegions) == 0 {
		output.SelectionGeometry = nil
	}
	return output
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
		out.Layers = append(out.Layers, LayerOutput{
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
		h.WriteUint64(uint64(render.LayerID))
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

type hitEntry struct {
	facetID   facet.FacetID
	transform gfx.Transform
	regions   []HitRegion
}

// HitMapEntry is a synthetic hit-map entry used by tests and input routing helpers.
type HitMapEntry struct {
	FacetID   facet.FacetID
	Transform gfx.Transform
	Regions   []HitRegion
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
		out.entries = append(out.entries, hitEntry{
			facetID:   entry.FacetID,
			transform: entry.Transform,
			regions:   regions,
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
			FacetID:   entry.facetID,
			Transform: entry.transform,
			Regions:   regions,
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
		for _, region := range entry.regions {
			if HitRegionContains(region, local) {
				if region.PassThrough {
					continue
				}
				return &HitTestResult{
					FacetID: entry.facetID,
					MarkID:  region.MarkID,
					Cursor:  region.Cursor,
				}
			}
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
		entries = append(entries, hitEntry{
			facetID:   po.FacetID,
			transform: po.Transform,
			regions:   regions,
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
	var walk func(*projectionNode)
	walk = func(n *projectionNode) {
		if n == nil || n.base == nil {
			return
		}
		flags := n.base.DirtyFlags()
		if s != nil && s.isDirty(n.base.ID()) {
			flags |= facet.DirtyAll
		}
		if flags != 0 {
			dirty[n.base.ID()] |= flags
		}
		for _, child := range n.children {
			walk(child)
		}
	}
	walk(node)
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
		var walk func(*projectionNode)
		walk = func(n *projectionNode) {
			if n == nil || n.base == nil {
				return
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
			for _, child := range n.children {
				walk(child)
			}
		}
		walk(node)
	}
}

func (s *System) markSubtreeDirty(node *projectionNode, flags facet.DirtyFlags, dirty map[facet.FacetID]facet.DirtyFlags) bool {
	if node == nil || node.base == nil {
		return false
	}
	changed := mergeDirtyFlags(dirty, node.base.ID(), flags)
	for _, child := range node.children {
		if s.markSubtreeDirty(child, flags, dirty) {
			changed = true
		}
	}
	return changed
}

func (s *System) clearTreeDirty(node *projectionNode) {
	if node == nil || node.base == nil {
		return
	}
	if flags := node.base.DirtyFlags(); flags != 0 {
		node.base.ClearDirty(flags)
	}
	for _, child := range node.children {
		s.clearTreeDirty(child)
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
