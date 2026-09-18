package diagnostics

import (
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
)

// FrameStats summarizes one runtime frame.
type FrameStats struct {
	FrameNumber     uint64
	DirtyFacets     int
	ProjectedFacets int
	CacheHits       int
	// ProjectionEmptyBoundsSkips counts non-layer facets gated by the RX-1
	// FR-1 empty-bounds gate in the most recent frame: facets arranged to empty
	// bounds that were pruned without projecting (an inactive stage exhibit, a
	// hidden overlay host). A nonzero count is expected whenever the tree holds
	// gated content; a spike alongside stale pixels is the A-1/A-2/A-3 failure
	// signature this counter exists to make observable.
	ProjectionEmptyBoundsSkips int
	// ProjectionCacheMissesByBounds counts projection cache misses whose stale
	// cached entry carried different arranged/layer bounds than the current
	// frame — a bounds change invalidating a cache entry (FR-1 freshness).
	ProjectionCacheMissesByBounds int
	// LayersUnmountedSkips counts layer-attached facets skipped by the layer
	// system in the most recent frame because their Mount store read false
	// (RX-1 Q4 visibility by mount state). An unmounted layer is skipped
	// entirely — no measure, no arrange, no projection layer, no hit — and is
	// independently mount-gated by the projection walk so a stale arranged
	// bounds can never resurrect it (NFR-8). Nonzero whenever an overlay is
	// closed (the command palette, a modal scrim, a tooltip); a nonzero count
	// alongside stale pixels is the A-4/A-5 failure signature.
	LayersUnmountedSkips      int
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

	// RX-1 P5 hot-path counters. All are frame-scoped (reset each frame):
	// zero in the quiet steady state (AC-6), nonzero on the frame that reacts
	// to a change.
	//
	// DerivedEvaluated / DerivedRecomputed report the store's derived flush
	// activity: how many derived Get calls ran and how many recomputed.
	DerivedEvaluated     int
	DerivedRecomputed    int
	DerivedFlushDuration time.Duration
	// ArrangeCount counts layout arrangements (host OnArrange invocations).
	ArrangeCount int
	// OverflowClampedCount counts OverflowGrow arrange clamps in the most
	// recent frame (RX-1 Q5 / NFR-8): a Grow mark arranged below its measured
	// min-content was clamped and flagged. Nonzero is a layout-contract
	// violation indicator, not a supported mode of operation.
	OverflowClampedCount int
	// CollectCount counts facet collect callbacks (OnCollect) in projection.
	CollectCount int
	// MaterializeCount counts materialized (rendered) output nodes.
	MaterializeCount int
	// HitTestCount counts hit-test invocations during the frame's hit phase.
	HitTestCount int
	// LayerResolveCount counts layer groups resolved by the layer tree pass.
	LayerResolveCount int
	// PruneCount counts projection nodes pruned by the empty-bounds gate or a
	// poisoned subtree skip.
	PruneCount int
	// GateCount counts projection nodes evaluated against the empty-bounds
	// gate (both kept and pruned).
	GateCount int

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
