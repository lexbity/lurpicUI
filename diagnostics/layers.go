package diagnostics

import (
	"fmt"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// LayerSnapshot describes one resolved layer for diagnostics.
type LayerSnapshot struct {
	LayerID            layout.LayerID
	Placement          layout.PlacementMode
	Measurement        layout.MeasurementMode
	CoordSpace         layout.CoordSpace
	RenderOrder        int
	HitPolicy          layout.LayerHitPolicy
	Bounds             gfx.Rect
	ClipRect           gfx.Rect
	Transform          gfx.Transform
	ChildCount         int
	AnchorCacheVersion uint64
	AnchorCacheCount   int
}

// AnchorSnapshot describes the exported anchors for one parent facet.
type AnchorSnapshot struct {
	ParentID facet.FacetID
	Version  uint64
	Entries  []AnchorSnapshotEntry
}

// AnchorSnapshotEntry describes one exported anchor.
type AnchorSnapshotEntry struct {
	ID       layout.AnchorID
	Position gfx.Point
	Children []facet.FacetID
	Changed  bool
}

// LayerSource provides layer snapshots for diagnostics.
type LayerSource interface {
	LayerSnapshots(parent facet.FacetID) []LayerSnapshot
}

// AnchorSource provides anchor snapshots for diagnostics.
type AnchorSource interface {
	AnchorSnapshot(parent facet.FacetID) (AnchorSnapshot, bool)
}

// HitTraceSource provides the most recent hit traversal trace.
type HitTraceSource interface {
	HitTrace() HitTestTrace
}

// HitTestTrace records the hit traversal that produced a result.
type HitTestTrace struct {
	TestedLayers []LayerHitTrace
	Result       facet.FacetID
}

// LayerHitTrace captures one tested layer during hit traversal.
type LayerHitTrace struct {
	ParentID    facet.FacetID
	LayerID     layout.LayerID
	HitPolicy   layout.LayerHitPolicy
	TestedCount int
	HitFacetID  facet.FacetID
	StoppedHere bool
}

func (s LayerSnapshot) String() string {
	return fmt.Sprintf("LayerID=%d Placement=%d Measurement=%d CoordSpace=%d RenderOrder=%d HitPolicy=%d Bounds=%v ClipRect=%v Children=%d AnchorCache=%d@v%d",
		s.LayerID, s.Placement, s.Measurement, s.CoordSpace, s.RenderOrder, s.HitPolicy, s.Bounds, s.ClipRect, s.ChildCount, s.AnchorCacheCount, s.AnchorCacheVersion)
}

// String returns a human-readable anchor snapshot summary.
func (s AnchorSnapshot) String() string {
	if len(s.Entries) == 0 {
		return fmt.Sprintf("ParentID=%d Version=%d Anchors=0", s.ParentID, s.Version)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "ParentID=%d Version=%d Anchors=%d", s.ParentID, s.Version, len(s.Entries))
	for _, entry := range s.Entries {
		fmt.Fprintf(&b, "\n  %s at %v children=%v", entry.ID, entry.Position, entry.Children)
	}
	return b.String()
}

// String returns a human-readable hit traversal summary.
func (t HitTestTrace) String() string {
	if len(t.TestedLayers) == 0 {
		return fmt.Sprintf("Result=%d Layers=0", t.Result)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "Result=%d Layers=%d", t.Result, len(t.TestedLayers))
	for _, layer := range t.TestedLayers {
		fmt.Fprintf(&b, "\n  %s", layer.String())
	}
	return b.String()
}

// String returns a human-readable description of one tested layer.
func (t LayerHitTrace) String() string {
	return fmt.Sprintf("ParentID=%d LayerID=%d HitPolicy=%d Tested=%d HitFacetID=%d StoppedHere=%t",
		t.ParentID, t.LayerID, t.HitPolicy, t.TestedCount, t.HitFacetID, t.StoppedHere)
}
