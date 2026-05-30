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

	// Asset system diagnostics — populated when an asset manager is configured.
	AssetTotalEntries       int
	AssetLoadingEntries     int
	AssetReadyEntries       int
	AssetPartialEntries     int
	AssetFailedEntries      int
	AssetCPUUsedBytes       int64
	AssetCPUBudgetBytes     int64
	AssetGPUUsedBytes       int64
	AssetGPUBudgetBytes     int64
	AssetEvictionsThisFrame int
	AssetUploadsThisFrame   int
	AssetJobsInFlight       int
	AssetCacheHitRate       float64
}
