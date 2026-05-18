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
	LayerName          string
	WindowBinding      string
	Placement          layout.PlacementMode
	Measurement        layout.MeasurementMode
	CoordSpace         layout.CoordSpace
	RenderOrder        int
	HitPolicy          layout.LayerHitPolicy
	RootPolicyKind     string
	RecipeVersion      uint64
	Materialized       bool
	CommandCount       int
	HitRegionCount     int
	Bounds             gfx.Rect
	ClipRect           gfx.Rect
	Transform          gfx.Transform
	ChildCount         int
	ArrangedChildren   []ArrangedChildSnapshot
	AnchorCacheVersion uint64
	AnchorCacheCount   int
}

// LayerFrame describes the resolved spatial frame for one layer.
type LayerFrame struct {
	LayerID        layout.LayerID
	LayerName      string
	WindowBinding  string
	CoordSpace     layout.CoordSpace
	Bounds         gfx.Rect
	ClipRect       gfx.Rect
	Transform      gfx.Transform
	RenderOrder    int
	HitPolicy      layout.LayerHitPolicy
	RootPolicyKind string
	RecipeVersion  uint64
	Materialized   bool
	CommandCount   int
	HitRegionCount int
}

// ArrangedChildSnapshot describes one child arranged inside a layer.
type ArrangedChildSnapshot struct {
	FacetID       facet.FacetID
	LayerID       layout.LayerID
	WindowBinding string
	Placement     facet.PlacementMode
	HitPolicy     facet.HitPolicy
	ClipPolicy    facet.ClipPolicy
	ZPriority     int32
	Bounds        gfx.Rect
	ClipRect      gfx.Rect
	Materialized  bool
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
	LayerOrder  int
	CoordSpace  layout.CoordSpace
	RenderOrder int
	HitPolicy   layout.LayerHitPolicy
	Placement   facet.PlacementMode
	ClipPolicy  facet.ClipPolicy
	ZPriority   int32
	Bounds      gfx.Rect
	ClipRect    gfx.Rect
	Transform   gfx.Transform
	TestedCount int
	HitFacetID  facet.FacetID
	StoppedHere bool
}

func (s LayerSnapshot) String() string {
	frame := s.Frame()
	return fmt.Sprintf("Frame=%s Placement=%d Measurement=%d Children=%d Arranged=%d Materialized=%t Cmds=%d Hits=%d AnchorCache=%d@v%d",
		frame.String(), s.Placement, s.Measurement, s.ChildCount, len(s.ArrangedChildren), s.Materialized, s.CommandCount, s.HitRegionCount, s.AnchorCacheCount, s.AnchorCacheVersion)
}

// Frame returns the resolved spatial frame for this snapshot.
func (s LayerSnapshot) Frame() LayerFrame {
	return LayerFrame{
		LayerID:        s.LayerID,
		LayerName:      s.LayerName,
		WindowBinding:  s.WindowBinding,
		CoordSpace:     s.CoordSpace,
		Bounds:         s.Bounds,
		ClipRect:       s.ClipRect,
		Transform:      s.Transform,
		RenderOrder:    s.RenderOrder,
		HitPolicy:      s.HitPolicy,
		RootPolicyKind: s.RootPolicyKind,
		RecipeVersion:  s.RecipeVersion,
		Materialized:   s.Materialized,
		CommandCount:   s.CommandCount,
		HitRegionCount: s.HitRegionCount,
	}
}

// String returns a human-readable description of the resolved layer frame.
func (f LayerFrame) String() string {
	return fmt.Sprintf("LayerID=%d LayerName=%q WindowBinding=%q CoordSpace=%d Bounds=%v ClipRect=%v Transform=%v RenderOrder=%d HitPolicy=%d RootPolicyKind=%s RecipeVersion=%d Materialized=%t Cmds=%d Hits=%d",
		f.LayerID, f.LayerName, f.WindowBinding, f.CoordSpace, f.Bounds, f.ClipRect, f.Transform, f.RenderOrder, f.HitPolicy, f.RootPolicyKind, f.RecipeVersion, f.Materialized, f.CommandCount, f.HitRegionCount)
}

// String returns a human-readable description of one arranged child snapshot.
func (s ArrangedChildSnapshot) String() string {
	return fmt.Sprintf("FacetID=%d LayerID=%d WindowBinding=%q Placement=%d HitPolicy=%d ClipPolicy=%d ZPriority=%d Bounds=%v ClipRect=%v Materialized=%t",
		s.FacetID, s.LayerID, s.WindowBinding, s.Placement, s.HitPolicy, s.ClipPolicy, s.ZPriority, s.Bounds, s.ClipRect, s.Materialized)
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
	return fmt.Sprintf("ParentID=%d LayerID=%d LayerOrder=%d CoordSpace=%d Bounds=%v ClipRect=%v Transform=%v RenderOrder=%d HitPolicy=%d Placement=%d ClipPolicy=%d ZPriority=%d Tested=%d HitFacetID=%d StoppedHere=%t",
		t.ParentID, t.LayerID, t.LayerOrder, t.CoordSpace, t.Bounds, t.ClipRect, t.Transform, t.RenderOrder, t.HitPolicy, t.Placement, t.ClipPolicy, t.ZPriority, t.TestedCount, t.HitFacetID, t.StoppedHere)
}
