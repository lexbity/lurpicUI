package runtime

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/assets"
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

// HitProbe returns a fresh hit probe for the current hit map.
func (rt *Runtime) HitProbe() *diagnostics.HitProbe {
	if rt == nil || rt.projectionSystem == nil {
		return nil
	}
	return diagnostics.NewHitProbe(rt.root, rt.layeredHitMap(rt.projectionSystem.CurrentHitMap()))
}

// HitTrace returns the most recent hit traversal trace.
func (rt *Runtime) HitTrace() diagnostics.HitTestTrace {
	if rt == nil {
		return diagnostics.HitTestTrace{}
	}
	return rt.lastHitTrace
}

// FacetByID returns the facet implementation with the given ID, if present.
func (rt *Runtime) FacetByID(id facet.FacetID) facet.FacetImpl {
	if rt == nil {
		return nil
	}
	return rt.findFacetByID(rt.root, id)
}

// RootStyleContext returns the application root style context, if one has been installed.
func (rt *Runtime) RootStyleContext() any {
	if rt == nil {
		return nil
	}
	return rt.rootStyleContext
}

// IconResolver exposes the configured icon resolver to mark implementations.
func (rt *Runtime) IconResolver() IconResolver {
	if rt == nil {
		return nil
	}
	return rt.config.IconResolver
}

// ResolveIcon resolves an icon asset through the configured runtime resolver.
func (rt *Runtime) ResolveIcon(ref string) (IconAsset, bool) {
	if rt == nil || rt.config.IconResolver == nil {
		return IconAsset{}, false
	}
	asset, ok := rt.config.IconResolver.ResolveIcon(ref)
	if !ok {
		return IconAsset{}, false
	}
	return asset.Clone(), true
}

// AssetManager exposes the configured runtime asset manager, if any.
func (rt *Runtime) AssetManager() assets.Manager {
	if rt == nil {
		return nil
	}
	return rt.assetManager
}

// AssetRegistry exposes the configured asset registry, if any.
func (rt *Runtime) AssetRegistry() *assets.AssetRegistryStore {
	if rt == nil {
		return nil
	}
	return rt.config.AssetRegistry
}

// SetRootStyleContext installs the root style context object used by tree helpers.
func (rt *Runtime) SetRootStyleContext(ctx any) {
	if rt == nil {
		return
	}
	rt.rootStyleSubs.Release()
	rt.rootStyleContext = ctx
	if store, ok := ctx.(*theme.StyleContextStore); ok && store != nil {
		signal.Track(&rt.rootStyleSubs, &store.OnChange, func(change signal.Change[theme.StyleContext]) {
			_ = change
			if rt.root == nil {
				return
			}
			rt.markTreeDirty(rt.root, facet.DirtyLayout|facet.DirtyProjection)
			if rt.frameTimer != nil {
				rt.frameTimer.RequestFrame()
			}
		})
	}
}

// EnableHitTrace toggles capture of the most recent hit traversal trace.
func (rt *Runtime) EnableHitTrace(enabled bool) {
	if rt == nil {
		return
	}
	rt.hitTraceEnabled = enabled
	if !enabled {
		rt.lastHitTrace = diagnostics.HitTestTrace{}
	}
}

// HitTest resolves the topmost facet hit at the given screen point.
func (rt *Runtime) HitTest(screenPos gfx.Point) facet.FacetID {
	if rt == nil || rt.projectionSystem == nil {
		if rt != nil && rt.hitTraceEnabled {
			rt.lastHitTrace = diagnostics.HitTestTrace{}
		}
		return 0
	}
	hitMap := rt.layeredHitMap(rt.projectionSystem.CurrentHitMap())
	if hitMap == nil {
		if rt.hitTraceEnabled {
			rt.lastHitTrace = diagnostics.HitTestTrace{}
		}
		return 0
	}
	id, trace := rt.hitTestWithMap(hitMap, screenPos)
	if rt.hitTraceEnabled {
		rt.lastHitTrace = trace
	}
	return id
}

func (rt *Runtime) layeredHitMap(hitMap *projection.HitMap) *projection.HitMap {
	if rt == nil || hitMap == nil {
		return hitMap
	}
	entries := hitMap.Entries()
	if len(entries) == 0 {
		return hitMap
	}
	type hitLayerEntry struct {
		entry projection.HitMapEntry
		order int
		z     int
	}
	items := make([]hitLayerEntry, 0, len(entries))
	for _, entry := range entries {
		order := entry.LayerOrder
		if rt.layerRegistry != nil && entry.LayerID != 0 {
			if desc, ok := rt.layerRegistry.Lookup(layout.LayerID(entry.LayerID)); ok {
				order = int(desc.Order)
			}
		}
		z := int(entry.ZPriority)
		if z == 0 {
			if attachment, ok := rt.childAttachments[entry.FacetID]; ok {
				z = int(attachment.ZPriority)
			}
		}
		layer, ok := rt.projectionLayers[entry.FacetID]
		if ok {
			entry.LayerID = layer.LayerID
			entry.HitPolicy = facet.HitPolicy(layer.HitPolicy)
			entry.ClipPolicy = layer.ClipPolicy
			entry.ClipRect = layer.ClipRect
		}
		if attachment, ok := rt.childAttachments[entry.FacetID]; ok {
			entry.Placement = attachment.Placement.Mode
			if z == 0 {
				z = int(attachment.ZPriority)
			}
		}
		entry.LayerOrder = order
		entry.ZPriority = int32(z)
		items = append(items, hitLayerEntry{entry: entry, order: order, z: z})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order != items[j].order {
			return items[i].order > items[j].order
		}
		if items[i].z != items[j].z {
			return items[i].z > items[j].z
		}
		return false
	})
	sorted := make([]projection.HitMapEntry, 0, len(items))
	for _, item := range items {
		sorted = append(sorted, item.entry)
	}
	return projection.NewHitMap(sorted...)
}

func (rt *Runtime) hitTestWithMap(hitMap *projection.HitMap, screenPos gfx.Point) (facet.FacetID, diagnostics.HitTestTrace) {
	if rt == nil || hitMap == nil {
		return 0, diagnostics.HitTestTrace{}
	}
	entries := hitMap.Entries()
	if len(entries) == 0 {
		return 0, diagnostics.HitTestTrace{}
	}
	trace := diagnostics.HitTestTrace{TestedLayers: make([]diagnostics.LayerHitTrace, 0, len(entries))}
	var passthrough facet.FacetID
	for _, entry := range entries {
		layer, hasLayer := rt.projectionLayers[entry.FacetID]
		policy := layout.LayerHitPolicy(entry.HitPolicy)
		if hasLayer {
			policy = layout.LayerHitPolicy(layer.HitPolicy)
		}
		parentID := facet.FacetID(0)
		if child := rt.findFacetByID(rt.root, entry.FacetID); child != nil && child.Base() != nil {
			if parent := child.Base().Parent(); parent != nil {
				parentID = parent.ID()
			}
		}
		local := screenPos
		if inv, ok := entry.Transform.Inverse(); ok {
			local = inv.TransformPoint(screenPos)
		}
		clipRect := entry.ClipRect
		layerBounds := gfx.Rect{}
		layerTransform := entry.Transform
		layerCoordSpace := layout.CoordSpace(0)
		if hasLayer {
			if clipRect.IsEmpty() {
				clipRect = layer.ClipRect
			}
			layerBounds = layer.Bounds
			layerTransform = layer.Transform
			layerCoordSpace = layout.CoordSpace(layer.CoordSpace)
		}
		if !clipRect.IsEmpty() {
			clip := clipRect
			if inv, ok := entry.Transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				traceLayer := diagnostics.LayerHitTrace{
					ParentID:    parentID,
					LayerID:     layout.LayerID(entry.LayerID),
					LayerOrder:  entry.LayerOrder,
					RenderOrder: entry.LayerOrder,
					CoordSpace:  layerCoordSpace,
					HitPolicy:   policy,
					Bounds:      layerBounds,
					ClipRect:    clipRect,
					Transform:   layerTransform,
					Placement:   entry.Placement,
					ClipPolicy:  entry.ClipPolicy,
					ZPriority:   entry.ZPriority,
					TestedCount: 0,
					StoppedHere: policy == layout.HitBlockBelow,
				}
				trace.TestedLayers = append(trace.TestedLayers, traceLayer)
				if policy == layout.HitBlockBelow {
					trace.Result = 0
					return 0, trace
				}
				continue
			}
		}
		hit := false
		hitFacetID := entry.FacetID
		tested := 0
		for _, region := range entry.Regions {
			tested++
			if projection.HitRegionContains(region, local) {
				hit = true
				if region.FacetID != 0 {
					hitFacetID = region.FacetID
				}
				break
			}
		}
		traceLayer := diagnostics.LayerHitTrace{
			ParentID:    parentID,
			LayerID:     layout.LayerID(entry.LayerID),
			LayerOrder:  entry.LayerOrder,
			RenderOrder: entry.LayerOrder,
			CoordSpace:  layerCoordSpace,
			HitPolicy:   policy,
			Bounds:      layerBounds,
			ClipRect:    clipRect,
			Transform:   layerTransform,
			Placement:   entry.Placement,
			ClipPolicy:  entry.ClipPolicy,
			ZPriority:   entry.ZPriority,
			TestedCount: tested,
		}
		if !hit {
			traceLayer.StoppedHere = policy == layout.HitBlockBelow
			trace.TestedLayers = append(trace.TestedLayers, traceLayer)
			if policy == layout.HitBlockBelow {
				trace.Result = 0
				return 0, trace
			}
			continue
		}
		traceLayer.HitFacetID = hitFacetID
		switch policy {
		case layout.HitPassThrough:
			passthrough = hitFacetID
			trace.TestedLayers = append(trace.TestedLayers, traceLayer)
		case layout.HitNormal, layout.HitBlockBelow:
			traceLayer.StoppedHere = true
			trace.TestedLayers = append(trace.TestedLayers, traceLayer)
			trace.Result = hitFacetID
			return hitFacetID, trace
		}
	}
	trace.Result = passthrough
	return passthrough, trace
}

// overlayInjector is satisfied by diagnostics.Hook when an Overlay is set.
type overlayInjector interface {
	InjectOverlay(frame *render.Frame, stats diagnostics.FrameStats)
}

var _ facet.RuntimeServices = (*Runtime)(nil)
