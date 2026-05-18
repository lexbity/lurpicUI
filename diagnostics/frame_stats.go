package diagnostics

import "time"

// FrameStats summarizes one runtime frame.
type FrameStats struct {
	FrameNumber               uint64
	DirtyFacets               int
	ProjectedFacets           int
	CacheHits                 int
	RenderBatchCount          int
	JobsCommitted             int
	JobsDiscarded             int
	LayoutDuration            time.Duration
	LayoutResolveDuration     time.Duration
	LayerResolutionDuration   time.Duration
	AnchorExportDuration      time.Duration
	StructuralMeasureDuration time.Duration
	LayerBoundsDuration       time.Duration
	ArrangeDuration           time.Duration
	ProjectDuration           time.Duration
	RenderDuration            time.Duration
}
