package diagnostics

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// HitSummary provides a stable view of hit test information.
type HitSummary struct {
	// TotalRegions is the total number of hit regions registered.
	TotalRegions int

	// Regions by facet type for quick overview
	RegionsByType map[string]int

	// ScreenSpaceBounds is the total hit-testable area in screen space.
	ScreenSpaceBounds gfx.Rect

	// RecentHits contains the most recent hit test results.
	RecentHits []HitResult

	// CapturedAt is when this summary was generated.
	CapturedAt int64 // Unix timestamp for comparison
}

// HitResult represents a single hit test at a point in time.
type HitResult struct {
	// Point in screen space where the hit occurred.
	Point gfx.Point

	// FacetID of the hit facet.
	FacetID facet.FacetID

	// FacetType is the human-readable type name.
	FacetType string

	// MarkID identifies the specific mark within the facet.
	MarkID facet.MarkID

	// PassThrough indicates if the region allows hit-through.
	PassThrough bool

	// Chain contains the hit chain from front to back.
	Chain []HitChainEntry
}

// HitChainEntry represents one level in a hit chain.
type HitChainEntry struct {
	FacetID   facet.FacetID
	FacetType string
	Region    gfx.Rect
}

// IsEmpty reports whether the hit summary contains no regions.
func (h HitSummary) IsEmpty() bool {
	return h.TotalRegions == 0
}

// FocusSummary provides a stable view of focus state.
type FocusSummary struct {
	// ActiveFocusOwner is the currently focused facet ID, or 0 if none.
	ActiveFocusOwner facet.FacetID

	// FocusOwnerType is the human-readable type of the focused facet.
	FocusOwnerType string

	// FocusChain contains the path from root to focused facet.
	FocusChain []facet.FacetID

	// TabOrder contains the focusable facets in tab order.
	TabOrder []FocusableInfo

	// HasFocus reports whether any facet currently has focus.
	HasFocus bool
}

// FocusableInfo describes one focusable facet.
type FocusableInfo struct {
	FacetID   facet.FacetID
	FacetType string
	Bounds    gfx.Rect
}

// FocusDepth returns how many levels deep the current focus is.
func (f FocusSummary) FocusDepth() int {
	return len(f.FocusChain)
}

// IsInChain reports whether the given facet ID is in the focus chain.
func (f FocusSummary) IsInChain(id facet.FacetID) bool {
	for _, chainID := range f.FocusChain {
		if chainID == id {
			return true
		}
	}
	return false
}

// InvalidationSummary provides a stable view of dirty/invalidation state.
type InvalidationSummary struct {
	// TotalDirtyFacets is the count of currently dirty facets.
	TotalDirtyFacets int

	// ByFlag breaks down dirty facets by flag type.
	ByFlag map[facet.DirtyFlags]int

	// BySource tracks which systems triggered invalidations.
	BySource map[string]int

	// RecentInvalidations contains the most recent invalidation events.
	RecentInvalidations []InvalidationEntry
}

// InvalidationEntry represents a single invalidation event.
type InvalidationEntry struct {
	FacetID   facet.FacetID
	FacetType string
	Flags     facet.DirtyFlags
	Source    string
	Timestamp int64
}

// IsClean reports whether there are no dirty facets.
func (i InvalidationSummary) IsClean() bool {
	return i.TotalDirtyFacets == 0
}

// DirtyFlagNames returns human-readable names for dirty flags.
func (i InvalidationSummary) DirtyFlagNames(flags facet.DirtyFlags) []string {
	var names []string
	if flags&facet.DirtyLayout != 0 {
		names = append(names, "Layout")
	}
	if flags&facet.DirtyProjection != 0 {
		names = append(names, "Projection")
	}
	if flags&facet.DirtyHit != 0 {
		names = append(names, "Hit")
	}
	if len(names) == 0 {
		return []string{"None"}
	}
	return names
}

// RenderBatchSummary provides a stable view of render batch information.
type RenderBatchSummary struct {
	// TotalBatches is the total number of render batches.
	TotalBatches int

	// BatchesByLayer counts batches per logical layer.
	BatchesByLayer map[int]int

	// TotalCommands is the total number of graphics commands.
	TotalCommands int

	// CommandsByType breaks down commands by type.
	CommandsByType map[string]int
}

// IsEmpty reports whether there are no render batches.
func (r RenderBatchSummary) IsEmpty() bool {
	return r.TotalBatches == 0
}

// AnchorSummary provides a stable view of anchor state.
type AnchorSummary struct {
	// TotalAnchors is the total number of exported anchors.
	TotalAnchors int

	// ByParent counts anchors per parent facet.
	ByParent map[facet.FacetID]int

	// ChangedAnchors contains anchors that changed since last frame.
	ChangedAnchors []AnchorEntry

	// HotAnchors are anchors with recent activity.
	HotAnchors []AnchorEntry
}

// AnchorEntry describes one anchor point.
type AnchorEntry struct {
	ParentID   facet.FacetID
	AnchorID   string
	Position   gfx.Point
	ChildCount int
	Changed    bool
}

// IsEmpty reports whether there are no anchors.
func (a AnchorSummary) IsEmpty() bool {
	return a.TotalAnchors == 0
}
