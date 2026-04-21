package runtime

import (
	"fmt"
	"sort"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/layout"
	projectedpolicy "codeburg.org/lexbit/lurpicui/layout/projected"
)

type layoutPhaseStats struct {
	specResolution        time.Duration
	anchorExport          time.Duration
	structuralMeasure     time.Duration
	layerBoundsResolution time.Duration
	arrange               time.Duration
}

type layerChildRef struct {
	base       *facet.Facet
	attachment layout.ChildAttachment
}

func (rt *Runtime) resolveLayerTree() layoutPhaseStats {
	if rt == nil || rt.root == nil {
		return layoutPhaseStats{}
	}
	rt.projectionLayers = make(map[facet.FacetID]facet.ProjectionLayer)
	return rt.walkLayerTree(rt.root, gfx.Identity())
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
