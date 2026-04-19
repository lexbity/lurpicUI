package canvas

import (
	"errors"

	"codeburg.org/lexbit/lurpicui/facet"
	gindex "codeburg.org/lexbit/lurpicui/graph/index"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/store"
)

// scheduleIndexRebuild submits a background index build to the facet's pool.
// If a previous build is in flight with the same ID, the pool cancels it automatically.
func (f *GraphCanvasFacet) scheduleIndexRebuild() {
	if f.pool == nil {
		return
	}
	f.buildCount.Add(1)

	nodes := f.graphStore.All()
	nodeVer := f.graphStore.Version()

	snap := job.NewSnapshot(nodes, nodeVer)
	snap = job.BindCurrentVersions(snap, func() []store.Version {
		return []store.Version{f.graphStore.Version()}
	})

	_ = job.Schedule(f.pool, job.Job[[]GraphNode, gindex.LODIndex]{
		ID:       indexJobID,
		Priority: job.PriorityBackground,
		Snapshot: snap,
		Work: func(s job.Snapshot[[]GraphNode], cancel *job.CancelToken) (gindex.LODIndex, error) {
			return buildNodeIndex(s.Data, cancel)
		},
	}, func(idx gindex.LODIndex) {
		f.nodeIndex = idx
		f.rt.Invalidate(f.Base().ID(), facet.DirtyProjection, "indexReady")
	})
}

// drainPool commits any completed index build results.
// Must be called on the runtime thread (inside project()).
func (f *GraphCanvasFacet) drainPool() {
	if f.pool == nil {
		return
	}
	f.pool.Drain()
}

// buildNodeIndex constructs a LODIndex from a node slice.
// Cancellation is checked every 1000 nodes.
func buildNodeIndex(nodes []GraphNode, cancel *job.CancelToken) (gindex.LODIndex, error) {
	b := gindex.NewRStarIndexBuilder(len(nodes))
	for i, node := range nodes {
		if cancel != nil && i%1000 == 0 && cancel.Cancelled() {
			return nil, errors.New("cancelled")
		}
		b.Add(gindex.EntityID(node.ID), node.Bounds)
	}
	return b.BuildWithCancel(cancel), nil
}
