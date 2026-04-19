package diagnostics

import "time"

// FrameStats summarizes one runtime frame.
type FrameStats struct {
	FrameNumber     uint64
	DirtyFacets     int
	ProjectedFacets int
	CacheHits       int
	LayerCount      int
	JobsCommitted   int
	JobsDiscarded   int
	LayoutDuration  time.Duration
	ProjectDuration time.Duration
	RenderDuration  time.Duration
}
