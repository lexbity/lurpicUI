package diagnostics

import (
	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
)

// Adapter bridges engine diagnostics to the app-facing abstraction.
// It provides stable access to diagnostic data without exposing engine internals.
type Adapter struct {
	inspector  *diagnostics.Inspector
	hitProbe   *diagnostics.HitProbe
	frameStats *FrameStatsHistory
}

// NewAdapter creates a new diagnostics adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		frameStats: NewFrameStatsHistory(120), // Keep 2 seconds at 60fps
	}
}

// SetInspector sets the engine inspector for facet tree queries.
func (a *Adapter) SetInspector(inspector *diagnostics.Inspector) {
	a.inspector = inspector
}

// SetHitProbe sets the engine hit probe for hit testing queries.
func (a *Adapter) SetHitProbe(probe *diagnostics.HitProbe) {
	a.hitProbe = probe
}

// UpdateFrameStats adds a new frame stats entry from engine data.
func (a *Adapter) UpdateFrameStats(engineStats diagnostics.FrameStats) {
	if a == nil || a.frameStats == nil {
		return
	}

	view := FrameStatsView{
		FrameNumber:       engineStats.FrameNumber,
		TotalDuration:     engineStats.LayoutDuration + engineStats.ProjectDuration + engineStats.RenderDuration,
		LayoutDuration:    engineStats.LayoutDuration,
		ProjectDuration:   engineStats.ProjectDuration,
		RenderDuration:    engineStats.RenderDuration,
		DirtyFacetCount:   engineStats.DirtyFacets,
		ProjectedFacetCount: engineStats.ProjectedFacets,
		CacheHitCount:     engineStats.CacheHits,
		RenderBatchCount:  engineStats.RenderBatchCount,
		JobsCommitted:     engineStats.JobsCommitted,
		JobsDiscarded:     engineStats.JobsDiscarded,
	}

	a.frameStats.Add(view)
}

// GetFrameStats returns the current frame stats history.
func (a *Adapter) GetFrameStats() *FrameStatsHistory {
	if a == nil {
		return nil
	}
	return a.frameStats
}

// GetHitSummary returns a summary of hit test information.
func (a *Adapter) GetHitSummary() HitSummary {
	if a == nil || a.hitProbe == nil {
		return HitSummary{}
	}

	// Note: hit probe doesn't expose raw regions directly,
	// so we return basic info. Full implementation would use
	// engine HitMap if available.
	return HitSummary{
		TotalRegions:  0, // Would count from hitMap if exposed
		RegionsByType: make(map[string]int),
		RecentHits:    nil,
	}
}

// GetFocusSummary returns focus state information.
func (a *Adapter) GetFocusSummary() FocusSummary {
	// Focus state is tracked by the runtime FocusManager
	// This is a placeholder for the full implementation
	return FocusSummary{
		ActiveFocusOwner: 0,
		HasFocus:         false,
		FocusChain:       nil,
		TabOrder:         nil,
	}
}

// GetInvalidationSummary returns dirty/invalidation state.
func (a *Adapter) GetInvalidationSummary() InvalidationSummary {
	if a == nil || a.inspector == nil {
		return InvalidationSummary{
			ByFlag:   make(map[facet.DirtyFlags]int),
			BySource: make(map[string]int),
		}
	}

	summary := InvalidationSummary{
		ByFlag:   make(map[facet.DirtyFlags]int),
		BySource: make(map[string]int),
	}

	dirtySet := a.inspector.DirtySet()
	summary.TotalDirtyFacets = len(dirtySet)

	for _, flags := range dirtySet {
		summary.ByFlag[flags]++
	}

	return summary
}

// GetAnchorSummary returns anchor state information.
func (a *Adapter) GetAnchorSummary() AnchorSummary {
	if a == nil || a.inspector == nil {
		return AnchorSummary{
			ByParent: make(map[facet.FacetID]int),
		}
	}

	// Anchor data would be collected by walking the tree
	// and querying anchor snapshots
	return AnchorSummary{
		TotalAnchors: 0,
		ByParent:     make(map[facet.FacetID]int),
		ChangedAnchors: nil,
		HotAnchors:     nil,
	}
}

// GetRenderBatchSummary returns render batch information.
// This requires access to the render frame data.
func (a *Adapter) GetRenderBatchSummary() RenderBatchSummary {
	// Would need access to render frame to populate this
	return RenderBatchSummary{
		BatchesByLayer: make(map[int]int),
		CommandsByType: make(map[string]int),
	}
}

// IsAvailable reports whether the adapter has access to engine diagnostics.
func (a *Adapter) IsAvailable() bool {
	return a != nil && (a.inspector != nil || a.hitProbe != nil)
}

// IsInspectorAvailable reports whether facet tree inspection is available.
func (a *Adapter) IsInspectorAvailable() bool {
	return a != nil && a.inspector != nil
}

// IsHitProbeAvailable reports whether hit testing is available.
func (a *Adapter) IsHitProbeAvailable() bool {
	return a != nil && a.hitProbe != nil
}
