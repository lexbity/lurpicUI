package runtime

import (
	"fmt"
	"sort"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/theme"
)

type layoutPhaseStats struct {
	specResolution        time.Duration
	anchorExport          time.Duration
	structuralMeasure     time.Duration
	layerBoundsResolution time.Duration
	arrange               time.Duration
	// groups counts the layer groups resolved this frame (RX-1 P5
	// layer-resolve-count).
	groups int
}

func (rt *Runtime) resolveLayerTree() layoutPhaseStats {
	if rt.root == nil {
		return layoutPhaseStats{}
	}
	rt.projectionLayers = make(map[facet.FacetID]facet.ProjectionLayer)
	// Resolve every anchor exporter's cache first so anchored layers resolve
	// from the current (pre-walk) positions. CAUTION first-integrator wiring
	// (F-anchor-runtime): this reads exporter ArrangedBounds from the last
	// layout/arrange pass, so an anchor-cache change re-arranges its anchored
	// children during this same walk.
	rt.resolveAnchorExports()
	return rt.walkLayerTree(rt.root, gfx.Identity())
}

// EnableDiagnostics replaces the runtime diagnostics hook.
func (rt *Runtime) EnableDiagnostics(d DiagnosticsHook) {
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
	if fn == nil {
		return
	}
	inspector := diagnostics.NewInspector(rt.root)
	inspector.SetLayerSource(rt)
	inspector.SetAnchorSource(rt)
	inspector.SetHitTraceSource(rt)
	fn(inspector)
}

func (rt *Runtime) diagnosticsHook() DiagnosticsHook {
	rt.diagMu.RLock()
	defer rt.diagMu.RUnlock()
	return rt.diag
}

func (rt *Runtime) findFacetByID(root facet.FacetImpl, id facet.FacetID) facet.FacetImpl {
	if root == nil || root.Base() == nil {
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
	if rt.root == nil {
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
	if node == nil || node.Base() == nil {
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
	if parent == nil || parent.Base() == nil {
		return
	}
	parentID := parent.Base().ID()
	cache := rt.anchorCaches[parentID]
	anchorChildren := make(map[layout.AnchorID][]facet.FacetID)
	exporters := make([]facet.FacetImpl, 0)
	for _, childBase := range parent.Base().Children() {
		if childBase == nil {
			continue
		}
		attachment, ok := rt.childAttachments[childBase.ID()]
		if ok && attachment.LayerID != 0 && attachment.Placement.Anchor.AnchorRef != "" {
			anchorID := layout.AnchorID(attachment.Placement.Anchor.AnchorRef)
			anchorChildren[anchorID] = append(anchorChildren[anchorID], childBase.ID())
		}
		child := rt.findFacetByID(rt.root, childBase.ID())
		if child == nil {
			continue
		}
		if _, ok := child.(layout.AnchorExporter); ok {
			if attachment, ok := rt.childAttachments[childBase.ID()]; ok && attachment.LayerID != 0 {
				exporters = append(exporters, child)
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
		if !ok || attachment.LayerID == 0 {
			continue
		}
		if rt.layerRegistry == nil {
			continue
		}
		if _, ok := rt.layerRegistry.Lookup(layout.LayerID(attachment.LayerID)); !ok {
			continue
		}
		resolved := layout.ResolvedLayer{LayerID: layout.LayerID(attachment.LayerID)}
		if lr := exporterFacet.Base().LayoutRole(); lr != nil {
			resolved.Bounds = lr.ArrangedBounds
		}
		if viewport := exporterFacet.Base().ViewportRole(); viewport != nil {
			resolved.Transform = viewport.Transform
			if !viewport.WorldBounds.IsEmpty() {
				resolved.ClipRect = viewport.WorldBounds
			}
		}
		if resolved.ClipRect.IsEmpty() {
			resolved.ClipRect = resolved.Bounds
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
	childIDs := make([]facet.FacetID, 0, len(parent.Base().Children()))
	for _, childBase := range parent.Base().Children() {
		if childBase == nil {
			continue
		}
		if attachment, ok := rt.childAttachments[childBase.ID()]; ok && attachment.LayerID != 0 {
			childIDs = append(childIDs, childBase.ID())
		}
	}
	rt.markFacetGroupDirty(childIDs, facet.DirtyLayout, anchorChangeSource(changes[0]))
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
	if len(anchorChildren) == 0 {
		return
	}
	ids := make([]facet.FacetID, 0)
	for _, refs := range anchorChildren {
		ids = append(ids, refs...)
	}
	rt.markFacetGroupDirty(ids, facet.DirtyLayout, source)
}

func (rt *Runtime) markFacetGroupDirty(ids []facet.FacetID, flags facet.DirtyFlags, source string) {
	for _, id := range ids {
		rt.markFacetDirtyByID(id, flags, source)
	}
}

func (rt *Runtime) markFacetDirtyByID(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if id == 0 {
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
	if node == nil || node.Base() == nil {
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
		stats = stats.add(rt.resolveAttachedLayers(frame.node, current))
		children := frame.node.Base().Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, layerFrame{node: children[i], transform: current})
		}
	}
	return stats
}

func (s layoutPhaseStats) add(other layoutPhaseStats) layoutPhaseStats {
	s.specResolution += other.specResolution
	s.anchorExport += other.anchorExport
	s.structuralMeasure += other.structuralMeasure
	s.layerBoundsResolution += other.layerBoundsResolution
	s.arrange += other.arrange
	s.groups += other.groups
	return s
}

func (rt *Runtime) resolveAttachedLayers(parent facet.FacetImpl, accumulated gfx.Transform) layoutPhaseStats {
	if parent == nil || parent.Base() == nil {
		return layoutPhaseStats{}
	}
	parentBounds := gfx.Rect{}
	// parentGatedEmpty records that a non-root host has been gated to empty
	// bounds (e.g. a Stage that arranges inactive exhibits to gfx.Rect{}). When
	// true, every layer child of this host is cascaded to empty below regardless
	// of the parentBounds fallback, so a hidden host's overlays are never
	// resurrected by the layer pass (F-inactive-layer-child). The runtime root
	// is excluded: it has no gating parent, so empty bounds there means "not yet
	// arranged" or a first frame, not "hidden".
	parentGatedEmpty := parent.Base().Parent() != nil
	if lr := parent.Base().LayoutRole(); lr != nil {
		if !lr.ArrangedBounds.IsEmpty() {
			parentBounds = lr.ArrangedBounds
			parentGatedEmpty = false
		} else if lr.MeasuredSize.W > 0 || lr.MeasuredSize.H > 0 {
			parentBounds = gfx.RectFromXYWH(0, 0, lr.MeasuredSize.W, lr.MeasuredSize.H)
		} else if w, h := rt.windowSize(); w > 0 || h > 0 {
			parentBounds = gfx.RectFromXYWH(0, 0, float32(w), float32(h))
		}
	} else if w, h := rt.windowSize(); w > 0 || h > 0 {
		parentBounds = gfx.RectFromXYWH(0, 0, float32(w), float32(h))
	}
	if rt.layerRegistry == nil {
		return layoutPhaseStats{}
	}
	type layerGroup struct {
		desc     layout.LayerDescriptor
		children []layout.LayerChild
		band     facet.ZBand
	}
	groupMap := make(map[facet.LayerID]*layerGroup)
	ordered := make([]facet.LayerID, 0)
	for _, childBase := range parent.Base().Children() {
		if childBase == nil {
			continue
		}
		att := childBase.LayerAttachment()
		attachment, ok := rt.childAttachments[childBase.ID()]
		// A child is a layer when it was AttachLayer'd (band contract) or the
		// host pinned a LayerID via AddFacet/UpdateChildAttachment.
		isLayer := childBase.IsLayer()
		if !isLayer && (!ok || attachment.LayerID == 0) {
			continue
		}
		if isLayer && att.Mount != nil && !att.Mount.Get() {
			// Unmounted: clear the child's arranged bounds so its projection is
			// gated and it registers no hit (RX-1 Q4 visibility by mount state).
			if role := childBase.LayoutRole(); role != nil {
				role.Arrange(facet.ArrangeContext{
					Runtime: rt,
					Theme:   rt.themeContext(parentBounds),
				}, gfx.Rect{})
			}
			continue
		}
		// Resolve the registry layer ID: a pinned attachment (the host set a
		// LayerID) wins; otherwise the band maps to a standard layer.
		if !ok || attachment.LayerID == 0 {
			// A band-mapped layer must be content-bearing (it has a layout role);
			// inert placeholder surfaces (the marks' bare surfaceChild facets) stay
			// unresolved as before — resolving them changes projection state and
			// breaks the revisit-byte-identical contract (RX-1 AC-1).
			if childBase.LayoutRole() == nil {
				continue
			}
			lid, mapped := standardLayerIDForBand(att.Band)
			if !mapped {
				continue
			}
			attachment = facet.Attachment{Placement: attachment.Placement}
			attachment.LayerID = lid
			attachment.ZOrder = att.Order
			attachment.Band = att.Band
			rt.childAttachments[childBase.ID()] = attachment
		}
		band := attachment.Band
		if band == 0 && isLayer {
			band = att.Band
		}
		if band == 0 {
			band = facet.ZBandContent
		}
		desc, ok := rt.layerRegistry.Lookup(layout.LayerID(attachment.LayerID))
		if !ok {
			continue
		}
		layerID := facet.LayerID(desc.ID)
		group := groupMap[layerID]
		if group == nil {
			group = &layerGroup{desc: desc, band: band}
			groupMap[layerID] = group
			ordered = append(ordered, layerID)
		}
		layoutRole := childBase.LayoutRole()
		var contract facet.GroupChildContract
		if layoutRole != nil {
			contract = layoutRole.Child
		}
		group.children = append(group.children, layout.LayerChild{
			FacetID:    childBase.ID(),
			Attachment: attachment,
			Layout:     layoutRole,
			Descriptor: contract,
		})
	}
	if len(ordered) == 0 {
		return layoutPhaseStats{}
	}
	// Layers paint in band order (RX-1 Q4 named z-bands); the registry order
	// and layer id break ties within a band.
	sort.SliceStable(ordered, func(i, j int) bool {
		left := groupMap[ordered[i]]
		right := groupMap[ordered[j]]
		if left.band != right.band {
			return left.band < right.band
		}
		if left.desc.Order != right.desc.Order {
			return left.desc.Order < right.desc.Order
		}
		return left.desc.ID < right.desc.ID
	})
	// F-inactive-layer-child: a host gated to empty bounds (its own
	// ArrangedBounds is empty) must keep every layer-attached child inert.
	// The policies below would otherwise position the children from the
	// parentBounds fallback — free placement lands at bounds.Min + offset,
	// which with a zero/empty rect resurrects a hidden host's overlay at its
	// seed position and steals hits. Bypass the policies and cascade empty
	// bounds (and an empty projection layer) to each child instead.
	if parentGatedEmpty {
		emptyLayer := facet.ProjectionLayer{}
		for _, layerID := range ordered {
			group := groupMap[layerID]
			if group == nil {
				continue
			}
			for _, child := range group.children {
				if child.Layout != nil {
					child.Layout.Arrange(facet.ArrangeContext{
						Runtime:     rt,
						Theme:       rt.themeContext(parentBounds),
						ParentGroup: child.Layout.Parent,
						ChildGroup:  child.Layout.Child,
						Placement:   child.Attachment.Placement,
					}, gfx.Rect{})
				}
				rt.projectionLayers[child.FacetID] = emptyLayer
			}
		}
		return layoutPhaseStats{}
	}
	stats := layoutPhaseStats{groups: len(ordered)}
	parentViewport := parent.Base().ViewportRole()
	cache := rt.anchorCaches[parent.Base().ID()]
	for _, layerID := range ordered {
		group := groupMap[layerID]
		if group == nil {
			continue
		}
		recipeStart := time.Now()
		recipe := rt.resolveLayerRecipe(group.desc, parentBounds)
		if override := groupRecipeOverride(rt, group.children); override != nil {
			recipe = *override
		}
		policy := layout.ResolveLayerLayoutPolicy(recipe)
		stats.specResolution += time.Since(recipeStart)
		layerCtx := facet.LayerContext{
			ID:         facet.LayerID(group.desc.ID),
			HitPolicy:  facet.HitPolicy(group.desc.HitPolicy),
			ClipPolicy: facet.ClipPolicy(group.desc.ClipPolicy),
			Dismissal: facet.DismissalScope{
				Enabled: group.desc.Dismissal.Enabled,
				BehindOrders: facet.OrderRange{
					Min: group.desc.Dismissal.BehindOrders.Min,
					Max: group.desc.Dismissal.BehindOrders.Max,
				},
				Triggers: facet.DismissalTriggerSet(group.desc.Dismissal.Triggers),
			},
			FocusTrap:    group.desc.FocusTrap,
			FocusRestore: group.desc.FocusRestore,
			Order:        int32(group.desc.Order),
		}
		measureCtx := layout.LayerMeasureContext{
			Runtime:          rt,
			Theme:            rt.themeContext(parentBounds),
			Layer:            layerCtx,
			Bounds:           parentBounds,
			Recipe:           recipe,
			ContentScale:     rt.contentScale,
			WritingDirection: facet.WritingDirectionLTR,
			AnchorCache:      cache,
		}
		measureStart := time.Now()
		measureResult, err := policy.MeasureLayer(measureCtx, group.children)
		if err != nil {
			panic(err)
		}
		stats.structuralMeasure += time.Since(measureStart)
		layerBounds := parentBounds
		if measureResult.Size.W > 0 || measureResult.Size.H > 0 {
			layerBounds = gfx.RectFromXYWH(parentBounds.Min.X, parentBounds.Min.Y, measureResult.Size.W, measureResult.Size.H)
		}
		if layerBounds.IsEmpty() {
			layerBounds = parentBounds
		}
		layerFrame := rt.resolveLayerFrame(layerBounds, group.desc, recipe, accumulated)
		layerFrame.Bounds = layerBounds
		if parentViewport != nil && !parentViewport.WorldBounds.IsEmpty() {
			layerFrame.ClipRect = accumulated.TransformRect(parentViewport.WorldBounds)
		} else if group.desc.ClipPolicy != layout.ClipNone {
			layerFrame.ClipRect = layerBounds
		}
		arrangeCtx := layout.LayerArrangeContext{
			LayerMeasureContext: measureCtx,
			ClipRect:            layerFrame.ClipRect,
		}
		arrangeStart := time.Now()
		arranged, err := policy.ArrangeLayer(arrangeCtx, group.children)
		if err != nil {
			panic(err)
		}
		stats.arrange += time.Since(arrangeStart)
		if len(arranged) == 0 {
			for _, child := range group.children {
				if child.Layout != nil {
					child.Layout.Arrange(facet.ArrangeContext{
						Runtime:     rt,
						Theme:       measureCtx.Theme,
						Layer:       layerCtx,
						ParentGroup: child.Layout.Parent,
						ChildGroup:  child.Layout.Child,
						Placement:   child.Attachment.Placement,
					}, layerBounds)
				}
				childLayer := layerFrame
				childLayer.Bounds = layerBounds
				rt.projectionLayers[child.FacetID] = childLayer
			}
			continue
		}
		for _, arrangedChild := range arranged {
			child, ok := findLayerChild(group.children, arrangedChild.FacetID)
			if !ok {
				continue
			}
			if child.Layout != nil {
				child.Layout.Arrange(facet.ArrangeContext{
					Runtime:     rt,
					Theme:       measureCtx.Theme,
					Layer:       layerCtx,
					ParentGroup: child.Layout.Parent,
					ChildGroup:  child.Layout.Child,
					Placement:   child.Attachment.Placement,
				}, arrangedChild.Bounds)
			}
			childLayer := layerFrame
			childLayer.Bounds = arrangedChild.Bounds
			rt.projectionLayers[arrangedChild.FacetID] = childLayer
		}
	}
	return stats
}

func findLayerChild(children []layout.LayerChild, id facet.FacetID) (layout.LayerChild, bool) {
	for i := range children {
		if children[i].FacetID == id {
			return children[i], true
		}
	}
	return layout.LayerChild{}, false
}

// standardLayerIDForBand maps a facet ZBand to a standard registry layer ID.
// The band's position in the enum gives the paint order; the registry layer
// provides the descriptor (hit policy, clip, recipe). Layers that resolve to
// the same band share the same tier.
func standardLayerIDForBand(band facet.ZBand) (facet.LayerID, bool) {
	switch band {
	case facet.ZBandBase:
		return facet.LayerID(layout.StandardLayerIDBase), true
	case facet.ZBandContent:
		return facet.LayerID(layout.StandardLayerIDForeground), true
	case facet.ZBandPopover:
		return facet.LayerID(layout.StandardLayerIDFloating), true
	case facet.ZBandModal:
		return facet.LayerID(layout.StandardLayerIDModal), true
	case facet.ZBandTooltip:
		return facet.LayerID(layout.StandardLayerIDOverlay), true
	case facet.ZBandToast:
		return facet.LayerID(layout.StandardLayerIDStatus), true
	default:
		return 0, false
	}
}

func (rt *Runtime) themeContext(parentBounds gfx.Rect) theme.ResolvedContext {
	ctx := theme.DefaultResolvedContext()
	if rt != nil && rt.config.ThemeResolver != nil {
		ctx = ctx.WithResolver(rt.config.ThemeResolver)
	}
	ctx = ctx.WithViewport(gfx.Size{W: parentBounds.Width(), H: parentBounds.Height()})
	if rt != nil {
		ctx.ContentScale = rt.contentScale
	}
	return ctx
}

func (rt *Runtime) resolveLayerRecipe(desc layout.LayerDescriptor, parentBounds gfx.Rect) layout.ResolvedLayerLayoutRecipe {
	if rt != nil && rt.config.ThemeResolver != nil && (desc.LayoutRecipe.Family != "" || desc.LayoutRecipe.Name != "") {
		ctx := rt.themeContext(parentBounds)
		if recipe, ok := ctx.ResolveLayerLayoutRecipe(desc.LayoutRecipe); ok {
			return recipe
		}
	}
	return layout.DefaultLayerLayoutRecipe()
}

// groupRecipeOverride resolves a layer-attachment recipe override for the
// group's children. A child that declares Recipe.Name == "modal" is arranged
// by the modal recipe: a single cell that fills the parent so the child
// centers itself within it (RX-1 Q4; the command palette's centered surface is
// the canonical consumer).
func groupRecipeOverride(rt *Runtime, children []layout.LayerChild) *layout.ResolvedLayerLayoutRecipe {
	if rt == nil || len(children) == 0 {
		return nil
	}
	for _, child := range children {
		att := child.Attachment
		if att.Band != facet.ZBandModal {
			continue
		}
		impl := rt.findFacetByID(rt.root, child.FacetID)
		if impl == nil || impl.Base() == nil {
			continue
		}
		if impl.Base().LayerAttachment().Recipe.Name == "modal" {
			modal := layout.DefaultLayerLayoutRecipe()
			modal.Grid = layout.ResolvedGridConfig{Columns: 1, Rows: 1}
			modal.PolicyKind = layout.LayerLayoutGrid
			return &modal
		}
	}
	return nil
}

func (rt *Runtime) resolveLayerFrame(parentBounds gfx.Rect, desc layout.LayerDescriptor, recipe layout.ResolvedLayerLayoutRecipe, accumulated gfx.Transform) facet.ProjectionLayer {
	layer := facet.ProjectionLayer{}
	layer.LayerID = facet.LayerID(desc.ID)
	layer.CoordSpace = uint8(desc.CoordSpace)
	if desc.CoordSpace == layout.CoordScreenAligned {
		layer.Transform = gfx.Identity()
	} else {
		layer.Transform = accumulated
	}
	layer.RenderOrder = int(desc.Order)
	if desc.HitPolicy != layout.HitNormal {
		layer.HitPolicy = uint8(desc.HitPolicy)
	}
	if desc.ClipPolicy != layout.ClipNone {
		layer.ClipRect = parentBounds
	}
	layer.ClipPolicy = facet.ClipPolicy(desc.ClipPolicy)
	layer.Dismissal = facet.DismissalScope{
		Enabled: desc.Dismissal.Enabled,
		BehindOrders: facet.OrderRange{
			Min: desc.Dismissal.BehindOrders.Min,
			Max: desc.Dismissal.BehindOrders.Max,
		},
		Triggers: facet.DismissalTriggerSet(desc.Dismissal.Triggers),
	}
	layer.FocusTrap = desc.FocusTrap
	layer.FocusRestore = desc.FocusRestore
	layer.RecipeVersion = layerRecipeVersion(desc, recipe)
	return layer
}

func layerRecipeVersion(desc layout.LayerDescriptor, recipe layout.ResolvedLayerLayoutRecipe) uint64 {
	h := hashutil.NewCacheKeyBuilder()
	h.WriteString(string(desc.Name))
	h.WriteUint64(uint64(desc.ID))
	h.WriteUint64(uint64(desc.Order)) //nolint:gosec // integer overflow conversion
	h.WriteString(desc.LayoutRecipe.Family)
	h.WriteString(desc.LayoutRecipe.Name)
	if desc.FocusTrap {
		h.WriteUint8(1)
	} else {
		h.WriteUint8(0)
	}
	h.WriteUint8(uint8(desc.FocusRestore))
	h.WriteUint8(uint8(recipe.PolicyKind))
	h.WriteUint8(uint8(recipe.Clip))
	h.WriteUint64(uint64(recipe.Grid.Columns)) //nolint:gosec // integer overflow conversion
	h.WriteUint64(uint64(recipe.Grid.Rows))    //nolint:gosec // integer overflow conversion
	h.WriteFloat32(float32(recipe.Grid.ColumnGap))
	h.WriteFloat32(float32(recipe.Grid.RowGap))
	h.WriteFloat32(recipe.Grid.Margin.Top)
	h.WriteFloat32(recipe.Grid.Margin.Right)
	h.WriteFloat32(recipe.Grid.Margin.Bottom)
	h.WriteFloat32(recipe.Grid.Margin.Left)
	h.WriteFloat32(float32(recipe.Anchor.Gap))
	h.WriteFloat32(float32(recipe.Anchor.OffsetX))
	h.WriteFloat32(float32(recipe.Anchor.OffsetY))
	h.WriteFloat32(float32(recipe.Free.X))
	h.WriteFloat32(float32(recipe.Free.Y))
	h.WriteFloat32(float32(recipe.Free.Width.Value))
	if recipe.Free.Width.Valid {
		h.WriteUint8(1)
	} else {
		h.WriteUint8(0)
	}
	h.WriteFloat32(float32(recipe.Free.Height.Value))
	if recipe.Free.Height.Valid {
		h.WriteUint8(1)
	} else {
		h.WriteUint8(0)
	}
	h.WriteFloat32(float32(recipe.Insets.Top))
	h.WriteFloat32(float32(recipe.Insets.Right))
	h.WriteFloat32(float32(recipe.Insets.Bottom))
	h.WriteFloat32(float32(recipe.Insets.Left))
	return h.Sum()
}

// ResolveProjectionLayer returns the current projection-layer snapshot for a facet.
func (rt *Runtime) ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool) {
	if id == 0 {
		return facet.ProjectionLayer{}, false
	}
	layer, ok := rt.projectionLayers[id]
	return layer, ok
}

// ResolveChildAttachment returns the stored attachment metadata for a child facet.
func (rt *Runtime) ResolveChildAttachment(id facet.FacetID) (facet.Attachment, bool) {
	if id == 0 || rt.childAttachments == nil {
		return facet.Attachment{}, false
	}
	attachment, ok := rt.childAttachments[id]
	return attachment, ok
}

// ResolveWindowBinding returns the registered window binding for a facet, if any.
func (rt *Runtime) ResolveWindowBinding(id facet.FacetID) (layout.WindowBinding, bool) {
	if id == 0 || rt.layerRegistry == nil {
		return layout.WindowBinding{}, false
	}
	layer, ok := rt.projectionLayers[id]
	if !ok {
		return layout.WindowBinding{}, false
	}
	desc, ok := rt.layerRegistry.Lookup(layout.LayerID(layer.LayerID))
	if !ok {
		return layout.WindowBinding{}, false
	}
	return desc.WindowBinding, true
}

// CurrentContentScale returns the runtime's effective content scale.
func (rt *Runtime) CurrentContentScale() float32 {
	return rt.contentScale
}

// CurrentInputModality returns the runtime's current input modality snapshot.
func (rt *Runtime) CurrentInputModality() facet.InputModality {
	if rt.inputSystem == nil {
		return facet.InputModalityUnknown
	}
	return rt.inputSystem.CurrentInputModality()
}

// LayerSnapshots returns diagnostics layer summaries for a parent facet.
func (rt *Runtime) LayerSnapshots(parent facet.FacetID) []diagnostics.LayerSnapshot {
	if parent == 0 {
		return nil
	}
	if rt.layerRegistry == nil {
		return nil
	}
	parentFacet := rt.findFacetByID(rt.root, parent)
	if parentFacet == nil || parentFacet.Base() == nil {
		return nil
	}
	parentLayer := rt.projectionLayers[parent]
	outputs := make(map[facet.FacetID]projection.OutputSnapshot)
	if rt.projectionSystem != nil {
		for _, snap := range rt.projectionSystem.OutputSnapshots() {
			outputs[snap.FacetID] = snap
		}
	}
	type orderedSnapshot struct {
		order int
		id    facet.FacetID
		snap  diagnostics.LayerSnapshot
	}
	snapshots := make([]orderedSnapshot, 0, len(parentFacet.Base().Children()))
	cache := rt.anchorCaches[parent]
	for _, childBase := range parentFacet.Base().Children() {
		if childBase == nil {
			continue
		}
		attachment, ok := rt.childAttachments[childBase.ID()]
		if !ok || attachment.LayerID == 0 {
			continue
		}
		desc, ok := rt.layerRegistry.Lookup(layout.LayerID(attachment.LayerID))
		if !ok {
			continue
		}
		layer := rt.projectionLayers[childBase.ID()]
		layerSnap := outputs[childBase.ID()]
		var placement layout.PlacementMode
		switch attachment.Placement.Mode {
		case facet.PlacementAnchor:
			placement = layout.PlacementAnchor
		case facet.PlacementFree:
			placement = layout.PlacementFree
		case facet.PlacementLinear:
			placement = layout.PlacementSplit
		default:
			placement = layout.PlacementGrid
		}
		arrangedChildren := make([]diagnostics.ArrangedChildSnapshot, 0, len(childBase.Children()))
		for _, grandChildBase := range childBase.Children() {
			if grandChildBase == nil {
				continue
			}
			grandAttachment, ok := rt.childAttachments[grandChildBase.ID()]
			if !ok || grandAttachment.LayerID == 0 {
				continue
			}
			grandLayer := rt.projectionLayers[grandChildBase.ID()]
			grandDesc, _ := rt.layerRegistry.Lookup(layout.LayerID(grandAttachment.LayerID))
			arrangedChildren = append(arrangedChildren, diagnostics.ArrangedChildSnapshot{
				FacetID:       grandChildBase.ID(),
				LayerID:       layout.LayerID(grandAttachment.LayerID),
				WindowBinding: windowBindingString(grandDesc.WindowBinding),
				Placement:     grandAttachment.Placement.Mode,
				HitPolicy:     facet.HitPolicy(grandLayer.HitPolicy),
				ClipPolicy:    grandLayer.ClipPolicy,
				ZOrder:        grandAttachment.ZOrder,
				Bounds:        grandLayer.Bounds,
				ClipRect:      grandLayer.ClipRect,
				Materialized:  grandLayer.LayerID != 0,
			})
		}
		sort.SliceStable(arrangedChildren, func(i, j int) bool {
			if arrangedChildren[i].LayerID != arrangedChildren[j].LayerID {
				return arrangedChildren[i].LayerID < arrangedChildren[j].LayerID
			}
			return arrangedChildren[i].FacetID < arrangedChildren[j].FacetID
		})
		resolvedRecipe := rt.resolveLayerRecipe(desc, parentLayer.Bounds)
		snapshots = append(snapshots, orderedSnapshot{
			order: int(desc.Order),
			id:    childBase.ID(),
			snap: diagnostics.LayerSnapshot{
				LayerID:          desc.ID,
				LayerName:        string(desc.Name),
				WindowBinding:    windowBindingString(desc.WindowBinding),
				Placement:        placement,
				Measurement:      layout.MeasureNonStructural,
				CoordSpace:       desc.CoordSpace,
				RenderOrder:      int(desc.Order),
				HitPolicy:        desc.HitPolicy,
				FocusTrap:        desc.FocusTrap,
				FocusRestore:     desc.FocusRestore,
				RootPolicyKind:   resolvedRecipe.PolicyKind.String(),
				RecipeVersion:    layerSnap.LayerRecipeVersion,
				Materialized:     layerSnap.Materialized,
				CommandCount:     layerSnap.CommandCount,
				HitRegionCount:   layerSnap.HitRegionCount,
				Bounds:           layer.Bounds,
				ClipRect:         layer.ClipRect,
				Transform:        layer.Transform,
				ChildCount:       len(childBase.Children()),
				ArrangedChildren: arrangedChildren,
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
			},
		})
	}
	sort.SliceStable(snapshots, func(i, j int) bool {
		if snapshots[i].order != snapshots[j].order {
			return snapshots[i].order < snapshots[j].order
		}
		return snapshots[i].id < snapshots[j].id
	})
	out := make([]diagnostics.LayerSnapshot, 0, len(snapshots))
	for _, item := range snapshots {
		out = append(out, item.snap)
	}
	return out
}

func windowBindingString(binding layout.WindowBinding) string {
	switch binding.Kind {
	case layout.WindowBindingNamed:
		if binding.Name != "" {
			return binding.Name
		}
		return "named"
	case layout.WindowBindingPrimary:
		fallthrough
	default:
		return "primary"
	}
}

// AnchorSnapshot returns diagnostics data for a parent's anchor cache.
func (rt *Runtime) AnchorSnapshot(parent facet.FacetID) (diagnostics.AnchorSnapshot, bool) {
	if parent == 0 {
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
		if !ok || attachment.Placement.Anchor.AnchorRef == "" {
			continue
		}
		anchorID := layout.AnchorID(attachment.Placement.Anchor.AnchorRef)
		childRefs[anchorID] = append(childRefs[anchorID], childBase.ID())
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
