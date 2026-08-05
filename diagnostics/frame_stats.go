package diagnostics

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
)

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

	// PoisonedFacets is the number of distinct facets currently quarantined
	// after a callback panic (FR-8). Zero on healthy runs; a monitoring hook
	// or test can detect non-zero poison without parsing logs.
	PoisonedFacets int
}

// PoisonReport describes the first failure of a quarantined facet. It is
// delivered to a DiagnosticsHook implementer that opts in by also implementing
// OnFacetPoisoned(diagnostics.PoisonReport) — the hook interface itself is not
// widened.
type PoisonReport struct {
	FacetID   facet.FacetID
	MarkType  string
	Role      string
	Panic     string
	Stack     string
	FirstSeen time.Time
}
