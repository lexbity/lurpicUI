// Package canvas provides GraphCanvasFacet, a reference application-layer facet
// that renders a large node graph using a background-built R*-tree spatial index
// with level-of-detail rendering.
package canvas

import (
	"sync/atomic"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/graph/index"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// indexJobID is the fixed job ID used for all spatial index builds.
// Reusing the same ID ensures the pool automatically cancels a previous build
// when a new one is scheduled.
const indexJobID job.JobID = 1

// GraphCanvasFacet renders a graph of nodes with pan/zoom and background LOD indexing.
type GraphCanvasFacet struct {
	facet.Facet

	layout     facet.LayoutRole
	renderRole facet.RenderRole
	hitRole    facet.HitRole
	inputRole  facet.InputRole
	projRole   facet.ProjectionRole

	// Application stores — injected before OnAttach.
	graphStore    *store.CollectionStore[GraphNode]
	edgeStore     *store.CollectionStore[GraphEdge]
	viewportStore *store.ValueStore[ViewportState]

	// Runtime services (set in OnAttach).
	rt   facet.RuntimeServices
	subs signal.Subscriptions

	// Background index build pool (1 worker; owned by this facet).
	pool *job.Pool

	// nodeIndex is nil until the first build completes.
	// Written only inside onCommit (runtime thread) or when pool is nil (tests).
	nodeIndex index.LODIndex

	// buildCount is incremented each time scheduleIndexRebuild is called.
	// Exported via BuildCount() for tests.
	buildCount atomic.Int32
}

// NewGraphCanvasFacet constructs a canvas facet with the provided stores.
// All store pointers must be non-nil.
func NewGraphCanvasFacet(
	graphStore *store.CollectionStore[GraphNode],
	edgeStore *store.CollectionStore[GraphEdge],
	viewportStore *store.ValueStore[ViewportState],
) *GraphCanvasFacet {
	f := &GraphCanvasFacet{
		Facet:         facet.NewFacet(),
		graphStore:    graphStore,
		edgeStore:     edgeStore,
		viewportStore: viewportStore,
	}

	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return c.MaxSize
	}

	f.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		// RenderRole is unused; projection handles rendering.
	}

	f.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return f.hitTest(p)
	}

	f.projRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		return f.project(ctx)
	}

	f.AddRole(&f.layout)
	f.AddRole(&f.renderRole)
	f.AddRole(&f.hitRole)
	f.AddRole(&f.inputRole)
	f.AddRole(&f.projRole)

	return f
}

// Base satisfies facet.FacetImpl.
func (f *GraphCanvasFacet) Base() *facet.Facet { return &f.Facet }

// SetOnPointer sets the pointer event handler for testing.
func (f *GraphCanvasFacet) SetOnPointer(fn func(e facet.PointerEvent) bool) {
	f.inputRole.OnPointer = fn
}

// OnAttach subscribes to store signals and starts the background worker pool.
func (f *GraphCanvasFacet) OnAttach(ctx facet.AttachContext) {
	f.rt = ctx.Runtime
	f.pool = job.NewPool(1)
	sub := facet.Subscribe(f)

	invalidate := func() {
		f.rt.Invalidate(f.Base().ID(), facet.DirtyProjection, "graphStore.OnReplace")
	}

	// Subscribe to node store replacements → rebuild index.
	sub.Collect(f.graphStore.OnReplaceSubscribe(func(signal.Unit) {
		f.scheduleIndexRebuild()
		invalidate()
	}))

	// Subscribe to viewport changes → re-project only (no rebuild).
	if f.viewportStore != nil {
		facet.To(sub, &f.viewportStore.OnChange, func(signal.Change[ViewportState]) {
			invalidate()
		})
	}
}

// OnDetach cleans up subscriptions and shuts down the worker pool.
func (f *GraphCanvasFacet) OnDetach() {
	f.subs.Release()
	if f.pool != nil {
		f.pool.Shutdown()
		f.pool = nil
	}
}

func (f *GraphCanvasFacet) OnActivate()   {}
func (f *GraphCanvasFacet) OnDeactivate() {}

// BuildCount returns the number of index rebuilds scheduled.
// Used by tests to verify rebuild triggering without pool internals.
func (f *GraphCanvasFacet) BuildCount() int {
	return int(f.buildCount.Load())
}

// NodeIndex returns the current spatial index (nil before first build).
// Used by tests.
func (f *GraphCanvasFacet) NodeIndex() index.LODIndex {
	return f.nodeIndex
}
