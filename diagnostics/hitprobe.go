package diagnostics

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/projection"
)

// HitProbeResult is a fully expanded hit-map entry for inspection.
type HitProbeResult struct {
	FacetID     facet.FacetID
	FacetType   string
	MarkID      facet.MarkID
	Region      gfx.Rect
	PassThrough bool
}

// HitProbe exposes the complete set of hit results for a point.
type HitProbe struct {
	hitMap    *projection.HitMap
	typeNames map[facet.FacetID]string
}

// NewHitProbe constructs a probe from the current hit map and optional tree root.
func NewHitProbe(root facet.FacetImpl, hitMap *projection.HitMap) *HitProbe {
	probe := &HitProbe{hitMap: hitMap}
	if root != nil {
		probe.typeNames = make(map[facet.FacetID]string)
		walkFacet(root, 0, func(_ int, info FacetInfo) {
			probe.typeNames[info.ID] = info.TypeName
		})
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
		for _, region := range entry.Regions {
			if !projection.HitRegionContains(region, local) {
				continue
			}
			out = append(out, HitProbeResult{
				FacetID:     entry.FacetID,
				FacetType:   p.typeNames[entry.FacetID],
				MarkID:      region.MarkID,
				Region:      region.Bounds,
				PassThrough: region.PassThrough,
			})
		}
	}
	return out
}
