package grid

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// TrackSizing selects how a track gets its size.
type TrackSizing uint8

const (
	TrackFixed TrackSizing = iota
	TrackIntrinsic
	TrackFlex
)

// TrackDef describes a single row or column track.
type TrackDef struct {
	Sizing TrackSizing
	Value  float32
	Min    float32
	Max    float32
}

// AutoPlacementMode determines the order auto-placed children follow.
type AutoPlacementMode uint8

const (
	AutoRowFirst AutoPlacementMode = iota
	AutoColumnFirst
)

// Config configures the grid policy.
type Config struct {
	Columns       []TrackDef
	Rows          []TrackDef
	ColumnGap     float32
	RowGap        float32
	AutoPlacement AutoPlacementMode
}

// Placement describes line-based cell placement.
type Placement struct {
	ColStart int
	RowStart int
	ColSpan  int
	RowSpan  int
}

// Child is the narrow view of a child facet participating in a grid policy.
type Child struct {
	FacetID    facet.FacetID
	Attachment facet.Attachment
	Layout     *facet.LayoutRole
	Contract   facet.GroupChildContract
}

// ArrangedChild captures a child arranged by a grid policy.
type ArrangedChild struct {
	FacetID   facet.FacetID
	Bounds    gfx.Rect
	Placement Placement
	ZPriority int32
	Contract  facet.GroupChildContract
}

// Policy arranges children in a 2D track grid.
type Policy struct {
	cfg Config
}

// New constructs a grid policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

func defaultFlexTracks(count int) []TrackDef {
	if count <= 0 {
		count = 1
	}
	out := make([]TrackDef, count)
	for i := range out {
		out[i] = TrackDef{Sizing: TrackFlex, Value: 1}
	}
	return out
}

func clampFloat(value, min, max float32) float32 {
	if value < min {
		value = min
	}
	if max > 0 && value > max {
		value = max
	}
	return value
}
