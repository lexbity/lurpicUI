package diagnostics

import (
	"sort"

	engdiag "codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Adapter bridges engine diagnostics to the app-facing abstraction.
// It provides stable access to diagnostic data without exposing engine internals.
type Adapter struct {
	inspector  *engdiag.Inspector
	hitProbe   *engdiag.HitProbe
	frameStats *FrameStatsHistory
	scene      SceneCapabilitySummary
	overlays   ActiveOverlays
}

// NewAdapter creates a new diagnostics adapter.
func NewAdapter() *Adapter {
	return &Adapter{
		frameStats: NewFrameStatsHistory(120),
		overlays:   NewActiveOverlays(""),
	}
}

// SetInspector sets the tree inspector used for summary generation.
func (a *Adapter) SetInspector(inspector *engdiag.Inspector) {
	if a == nil {
		return
	}
	a.inspector = inspector
}

// SetHitProbe sets the hit probe used for hit-region summaries.
func (a *Adapter) SetHitProbe(probe *engdiag.HitProbe) {
	if a == nil {
		return
	}
	a.hitProbe = probe
}

// SetSceneSummary stores the current scene capability summary.
func (a *Adapter) SetSceneSummary(summary SceneCapabilitySummary) {
	if a == nil {
		return
	}
	a.scene = summary
	a.overlays.SceneID = summary.SceneID
}

// SetActiveOverlays stores the current overlay state.
func (a *Adapter) SetActiveOverlays(overlays ActiveOverlays) {
	if a == nil {
		return
	}
	a.overlays = overlays
}

// UpdateFrameStats adds a new frame stats entry from engine data.
func (a *Adapter) UpdateFrameStats(engineStats engdiag.FrameStats) {
	if a == nil || a.frameStats == nil {
		return
	}

	view := FrameStatsView{
		FrameNumber:         engineStats.FrameNumber,
		TotalDuration:       engineStats.LayoutDuration + engineStats.ProjectDuration + engineStats.RenderDuration,
		LayoutDuration:      engineStats.LayoutDuration,
		ProjectDuration:     engineStats.ProjectDuration,
		RenderDuration:      engineStats.RenderDuration,
		DirtyFacetCount:     engineStats.DirtyFacets,
		ProjectedFacetCount: engineStats.ProjectedFacets,
		CacheHitCount:       engineStats.CacheHits,
		RenderBatchCount:    engineStats.RenderBatchCount,
		JobsCommitted:       engineStats.JobsCommitted,
		JobsDiscarded:       engineStats.JobsDiscarded,
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

// Snapshot captures the adapter's current stable state.
func (a *Adapter) Snapshot() Snapshot {
	if a == nil {
		return Snapshot{}
	}
	out := Snapshot{
		Scene:        a.GetSceneSummary(),
		Overlays:     a.GetActiveOverlays(),
		Focus:        a.GetFocusSummary(),
		Hit:          a.GetHitSummary(),
		Invalidation: a.GetInvalidationSummary(),
		Anchor:       a.GetAnchorSummary(),
		Render:       a.GetRenderBatchSummary(),
	}
	if a.frameStats != nil {
		out.Frames = append(out.Frames, a.frameStats.GetAll()...)
	}
	return out
}

// GetSceneSummary returns the current scene capability summary.
func (a *Adapter) GetSceneSummary() SceneCapabilitySummary {
	if a == nil {
		return SceneCapabilitySummary{}
	}
	return a.scene
}

// GetActiveOverlays returns the current overlay state.
func (a *Adapter) GetActiveOverlays() ActiveOverlays {
	if a == nil {
		return NewActiveOverlays("")
	}
	return a.overlays
}

// GetHitSummary returns a summary of hit test information.
func (a *Adapter) GetHitSummary() HitSummary {
	summary := HitSummary{
		RegionsByType: make(map[string]int),
	}
	if a == nil {
		return summary
	}

	if entries := a.hitProbeEntries(); len(entries) > 0 {
		for _, entry := range entries {
			if entry.FacetType != "" {
				summary.RegionsByType[entry.FacetType] += len(entry.Regions)
			}
			for _, region := range entry.Regions {
				summary.TotalRegions++
				rect := entry.Transform.TransformRect(region.Bounds)
				summary.ScreenSpaceBounds = unionRect(summary.ScreenSpaceBounds, rect)
				summary.RecentHits = append(summary.RecentHits, HitResult{
					Point:       rectCenter(rect),
					FacetID:     entry.FacetID,
					FacetType:   entry.FacetType,
					MarkID:      region.MarkID,
					PassThrough: region.PassThrough,
					Chain: []HitChainEntry{{
						FacetID:   entry.FacetID,
						FacetType: entry.FacetType,
						Region:    rect,
					}},
				})
			}
		}
		return summary
	}

	a.walkInspector(func(_ int, info facetInfoView) {
		if info.HitRegions == 0 {
			return
		}
		summary.TotalRegions += info.HitRegions
		if info.TypeName != "" {
			summary.RegionsByType[info.TypeName] += info.HitRegions
		}
		if !info.Bounds.IsEmpty() {
			summary.ScreenSpaceBounds = unionRect(summary.ScreenSpaceBounds, info.Bounds)
		}
		for i := 0; i < info.HitRegions; i++ {
			summary.RecentHits = append(summary.RecentHits, HitResult{
				Point:     rectCenter(info.Bounds),
				FacetID:   info.ID,
				FacetType: info.TypeName,
				Chain: []HitChainEntry{{
					FacetID:   info.ID,
					FacetType: info.TypeName,
					Region:    info.Bounds,
				}},
			})
		}
	})

	return summary
}

// GetFocusSummary returns focus state information.
func (a *Adapter) GetFocusSummary() FocusSummary {
	summary := FocusSummary{
		TabOrder: make([]FocusableInfo, 0, 8),
	}
	if a == nil {
		return summary
	}

	var found bool
	a.walkInspector(func(_ int, info facetInfoView) {
		if info.Focusable {
			summary.TabOrder = append(summary.TabOrder, FocusableInfo{
				FacetID:   info.ID,
				FacetType: info.TypeName,
				Bounds:    info.Bounds,
			})
		}
		if found || !info.Focusable {
			return
		}
		found = true
		summary.ActiveFocusOwner = info.ID
		summary.FocusOwnerType = info.TypeName
		summary.FocusChain = append([]facet.FacetID(nil), info.Path...)
	})
	summary.HasFocus = found
	return summary
}

// GetInvalidationSummary returns dirty/invalidation state.
func (a *Adapter) GetInvalidationSummary() InvalidationSummary {
	summary := InvalidationSummary{
		ByFlag:   make(map[facet.DirtyFlags]int),
		BySource: make(map[string]int),
	}
	if a == nil {
		return summary
	}

	a.walkInspector(func(_ int, info facetInfoView) {
		if info.DirtyFlags == 0 {
			return
		}
		summary.TotalDirtyFacets++
		summary.ByFlag[info.DirtyFlags]++
		if info.LastInvalidatedBy != "" {
			summary.BySource[info.LastInvalidatedBy]++
		}
		summary.RecentInvalidations = append(summary.RecentInvalidations, InvalidationEntry{
			FacetID:   info.ID,
			FacetType: info.TypeName,
			Flags:     info.DirtyFlags,
			Source:    info.LastInvalidatedBy,
			Timestamp: 0,
		})
	})

	sort.SliceStable(summary.RecentInvalidations, func(i, j int) bool {
		if summary.RecentInvalidations[i].FacetType != summary.RecentInvalidations[j].FacetType {
			return summary.RecentInvalidations[i].FacetType < summary.RecentInvalidations[j].FacetType
		}
		return summary.RecentInvalidations[i].FacetID < summary.RecentInvalidations[j].FacetID
	})

	return summary
}

// GetAnchorSummary returns anchor state information.
func (a *Adapter) GetAnchorSummary() AnchorSummary {
	summary := AnchorSummary{
		ByParent: make(map[facet.FacetID]int),
	}
	if a == nil {
		return summary
	}

	a.walkInspector(func(_ int, info facetInfoView) {
		if info.ChildCount == 0 {
			return
		}
		summary.TotalAnchors += info.ChildCount
		summary.ByParent[info.ID] = info.ChildCount
		entry := AnchorEntry{
			ParentID:   info.ID,
			AnchorID:   info.TypeName,
			Position:   rectCenter(info.Bounds),
			ChildCount: info.ChildCount,
			Changed:    info.DirtyFlags != 0,
		}
		summary.HotAnchors = append(summary.HotAnchors, entry)
		if entry.Changed {
			summary.ChangedAnchors = append(summary.ChangedAnchors, entry)
		}
	})

	return summary
}

// GetRenderBatchSummary returns render batch information.
func (a *Adapter) GetRenderBatchSummary() RenderBatchSummary {
	summary := RenderBatchSummary{
		BatchesByLayer: make(map[int]int),
		CommandsByType: make(map[string]int),
	}
	if a == nil {
		return summary
	}

	a.walkInspector(func(_ int, info facetInfoView) {
		if !info.Renderable && info.LayerCount == 0 {
			return
		}
		if info.Renderable {
			summary.TotalBatches++
			summary.CommandsByType[info.TypeName]++
		}
		if info.LayerCount > 0 {
			summary.TotalBatches += info.LayerCount
			for _, layer := range info.Layers {
				summary.BatchesByLayer[layer.RenderOrder]++
			}
		} else {
			summary.BatchesByLayer[info.Depth]++
		}
		summary.TotalCommands += max(1, info.LayerCount)
	})

	return summary
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

// Snapshot is a stable diagnostics capture suitable for export.
type Snapshot struct {
	Scene        SceneCapabilitySummary
	Overlays     ActiveOverlays
	Focus        FocusSummary
	Hit          HitSummary
	Invalidation InvalidationSummary
	Anchor       AnchorSummary
	Render       RenderBatchSummary
	Frames       []FrameStatsView
}

type facetInfoView struct {
	ID                facet.FacetID
	TypeName          string
	Path              []facet.FacetID
	Depth             int
	Bounds            gfx.Rect
	DirtyFlags        facet.DirtyFlags
	ChildCount        int
	LastInvalidatedBy string
	Focusable         bool
	Renderable        bool
	LayerCount        int
	Layers            []engdiag.LayerSnapshot
	HitRegions        int
}

func (a *Adapter) walkInspector(fn func(depth int, info facetInfoView)) {
	if a == nil || a.inspector == nil || fn == nil {
		return
	}

	path := make([]facet.FacetID, 0, 16)
	a.inspector.Walk(func(depth int, info engdiag.FacetInfo) {
		for len(path) > depth {
			path = path[:len(path)-1]
		}
		path = append(path, info.ID)

		view := facetInfoView{
			ID:                info.ID,
			TypeName:          info.TypeName,
			Path:              append([]facet.FacetID(nil), path...),
			Depth:             depth,
			Bounds:            info.ArrangedBounds,
			DirtyFlags:        info.DirtyFlags,
			ChildCount:        info.ChildCount,
			LastInvalidatedBy: info.LastInvalidatedBy,
			Focusable:         containsRole(info.Roles, "FocusRole"),
			Renderable:        containsRole(info.Roles, "RenderRole") || containsRole(info.Roles, "ProjectionRole"),
			LayerCount:        len(info.Layers),
			Layers:            info.Layers,
			HitRegions:        0,
		}
		if containsRole(info.Roles, "HitRole") {
			view.HitRegions = 1
		}
		fn(depth, view)
	})
}

func (a *Adapter) hitProbeEntries() []engdiag.HitProbeEntry {
	if a == nil || a.hitProbe == nil {
		return nil
	}
	return a.hitProbe.Entries()
}

func containsRole(roles []string, want string) bool {
	for _, role := range roles {
		if role == want {
			return true
		}
	}
	return false
}

func unionRect(a, b gfx.Rect) gfx.Rect {
	if a.IsEmpty() {
		return b
	}
	if b.IsEmpty() {
		return a
	}
	if b.Min.X < a.Min.X {
		a.Min.X = b.Min.X
	}
	if b.Min.Y < a.Min.Y {
		a.Min.Y = b.Min.Y
	}
	if b.Max.X > a.Max.X {
		a.Max.X = b.Max.X
	}
	if b.Max.Y > a.Max.Y {
		a.Max.Y = b.Max.Y
	}
	return a
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func rectCenter(r gfx.Rect) gfx.Point {
	return gfx.Point{
		X: r.Min.X + r.Width()/2,
		Y: r.Min.Y + r.Height()/2,
	}
}
