package runtime

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
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

// SetRootStyleContext installs the root style context object used by tree helpers.
func (rt *Runtime) SetRootStyleContext(ctx any) {
	if rt == nil {
		return
	}
	rt.rootStyleContext = ctx
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
		layer, ok := rt.projectionLayers[entry.FacetID]
		order := 0
		if ok {
			order = layer.RenderOrder
		}
		z := 0
		if attachment, ok := rt.childAttachments[entry.FacetID]; ok {
			z = attachment.ZPriority
		}
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
	type orderedEntry struct {
		entry projection.HitMapEntry
		order int
		z     int
	}
	items := make([]orderedEntry, 0, len(entries))
	for _, entry := range entries {
		layer, ok := rt.projectionLayers[entry.FacetID]
		order := 0
		policy := layout.HitNormal
		if ok {
			order = layer.RenderOrder
			policy = layout.LayerHitPolicy(layer.HitPolicy)
		}
		z := 0
		if attachment, ok := rt.childAttachments[entry.FacetID]; ok {
			z = attachment.ZPriority
		}
		if policy == layout.HitDisabled {
			continue
		}
		items = append(items, orderedEntry{entry: entry, order: order, z: z})
	}
	sort.SliceStable(items, func(i, j int) bool {
		if items[i].order != items[j].order {
			return items[i].order > items[j].order
		}
		if items[i].z != items[j].z {
			return items[i].z > items[j].z
		}
		return i < j
	})
	trace := diagnostics.HitTestTrace{TestedLayers: make([]diagnostics.LayerHitTrace, 0, len(items))}
	var passthrough facet.FacetID
	for _, item := range items {
		layer, ok := rt.projectionLayers[item.entry.FacetID]
		policy := layout.HitNormal
		if ok {
			policy = layout.LayerHitPolicy(layer.HitPolicy)
		}
		parentID := facet.FacetID(0)
		layerID := layout.LayerID(0)
		if child := rt.findFacetByID(rt.root, item.entry.FacetID); child != nil && child.Base() != nil {
			if parent := child.Base().Parent(); parent != nil {
				parentID = parent.ID()
			}
		}
		if attachment, ok := rt.childAttachments[item.entry.FacetID]; ok {
			layerID = attachment.LayerID
		}
		local := screenPos
		if inv, ok := item.entry.Transform.Inverse(); ok {
			local = inv.TransformPoint(screenPos)
		}
		if !layer.ClipRect.IsEmpty() {
			clip := layer.ClipRect
			if inv, ok := item.entry.Transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				traceLayer := diagnostics.LayerHitTrace{
					ParentID:    parentID,
					LayerID:     layerID,
					CoordSpace:  layout.CoordSpace(layer.CoordSpace),
					RenderOrder: layer.RenderOrder,
					HitPolicy:   policy,
					Bounds:      layer.Bounds,
					ClipRect:    layer.ClipRect,
					Transform:   layer.Transform,
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
		tested := 0
		for _, region := range item.entry.Regions {
			tested++
			if projection.HitRegionContains(region, local) {
				hit = true
				break
			}
		}
		traceLayer := diagnostics.LayerHitTrace{
			ParentID:    parentID,
			LayerID:     layerID,
			CoordSpace:  layout.CoordSpace(layer.CoordSpace),
			RenderOrder: layer.RenderOrder,
			HitPolicy:   policy,
			Bounds:      layer.Bounds,
			ClipRect:    layer.ClipRect,
			Transform:   layer.Transform,
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
		traceLayer.HitFacetID = item.entry.FacetID
		switch policy {
		case layout.HitPassThrough:
			passthrough = item.entry.FacetID
			trace.TestedLayers = append(trace.TestedLayers, traceLayer)
		case layout.HitNormal, layout.HitBlockBelow:
			traceLayer.StoppedHere = true
			trace.TestedLayers = append(trace.TestedLayers, traceLayer)
			trace.Result = item.entry.FacetID
			return item.entry.FacetID, trace
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
