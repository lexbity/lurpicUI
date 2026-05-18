package diagnostics

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/projection"
)

// HitProbeResult is a fully expanded hit-map entry for inspection.
type HitProbeResult struct {
	FacetID       facet.FacetID
	FacetType     string
	LayerID       facet.LayerID
	LayerOrder    int
	Placement     facet.PlacementMode
	HitPolicy     facet.HitPolicy
	ClipPolicy    facet.ClipPolicy
	ZPriority     int32
	MarkID        facet.MarkID
	Bounds        gfx.Rect
	EffectiveClip gfx.Rect
	PassThrough   bool
}

// HitProbe exposes the complete set of hit results for a point.
type HitProbe struct {
	hitMap    *projection.HitMap
	typeNames map[facet.FacetID]string
}

// HitProbeEntry is a stable snapshot of one hit-map entry.
type HitProbeEntry struct {
	FacetID    facet.FacetID
	FacetType  string
	LayerID    facet.LayerID
	LayerOrder int
	Placement  facet.PlacementMode
	HitPolicy  facet.HitPolicy
	ClipPolicy facet.ClipPolicy
	ZPriority  int32
	Transform  gfx.Transform
	ClipRect   gfx.Rect
	Regions    []projection.HitRegion
}

// HitProbeSource provides a current hit-probe snapshot.
type HitProbeSource interface {
	HitProbe() *HitProbe
}

// NewHitProbe constructs a probe from the current hit map and optional tree root.
func NewHitProbe(root facet.FacetImpl, hitMap *projection.HitMap) *HitProbe {
	probe := &HitProbe{hitMap: hitMap}
	if root != nil {
		probe.typeNames = make(map[facet.FacetID]string)
		var walk func(facet.FacetImpl)
		walk = func(node facet.FacetImpl) {
			if node == nil || node.Base() == nil {
				return
			}
			probe.typeNames[node.Base().ID()] = typeName(node)
			for _, child := range node.Base().Children() {
				walk(child)
			}
		}
		walk(root)
	}
	return probe
}

// At returns all hit results at screenPoint in front-to-back order.
func (p *HitProbe) At(screenPoint gfx.Point) []HitProbeResult {
	if p == nil || p.hitMap == nil {
		return nil
	}
	entries := p.hitMap.Entries()
	if len(entries) == 0 {
		return nil
	}
	out := make([]HitProbeResult, 0, len(entries))
	for _, entry := range entries {
		local := screenPoint
		if inv, ok := entry.Transform.Inverse(); ok {
			local = inv.TransformPoint(screenPoint)
		}
		clip := entry.ClipRect
		if !clip.IsEmpty() {
			if inv, ok := entry.Transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				continue
			}
		}
		for _, region := range entry.Regions {
			if !projection.HitRegionContains(region, local) {
				continue
			}
			out = append(out, HitProbeResult{
				FacetID:       entry.FacetID,
				FacetType:     p.typeNames[entry.FacetID],
				LayerID:       entry.LayerID,
				LayerOrder:    entry.LayerOrder,
				Placement:     entry.Placement,
				HitPolicy:     entry.HitPolicy,
				ClipPolicy:    entry.ClipPolicy,
				ZPriority:     entry.ZPriority,
				MarkID:        region.MarkID,
				Bounds:        region.Bounds,
				EffectiveClip: clip,
				PassThrough:   region.PassThrough,
			})
		}
	}
	return out
}

// Entries returns a stable snapshot of the current hit-map entries.
func (p *HitProbe) Entries() []HitProbeEntry {
	if p == nil || p.hitMap == nil {
		return nil
	}
	entries := p.hitMap.Entries()
	if len(entries) == 0 {
		return nil
	}
	out := make([]HitProbeEntry, 0, len(entries))
	for _, entry := range entries {
		regions := make([]projection.HitRegion, len(entry.Regions))
		copy(regions, entry.Regions)
		out = append(out, HitProbeEntry{
			FacetID:    entry.FacetID,
			FacetType:  p.typeNames[entry.FacetID],
			LayerID:    entry.LayerID,
			LayerOrder: entry.LayerOrder,
			Placement:  entry.Placement,
			HitPolicy:  entry.HitPolicy,
			ClipPolicy: entry.ClipPolicy,
			ZPriority:  entry.ZPriority,
			Transform:  entry.Transform,
			ClipRect:   entry.ClipRect,
			Regions:    regions,
		})
	}
	return out
}
