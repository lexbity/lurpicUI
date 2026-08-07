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

// BackendFallback describes a GPU→software render backend swap after a fatal
// GPU error (FR-12/Q9). Delivered to a DiagnosticsHook implementer that opts in
// by also implementing OnBackendFallback(diagnostics.BackendFallback) — the
// hook interface itself is not widened.
type BackendFallback struct {
	// From is the failing backend kind (e.g. "*vulkan.Backend").
	From string
	// To is the backend the runtime swapped to (always software for a fallback).
	To string
	// Reason is the GPU-fatal error that triggered the swap.
	Reason string
}
