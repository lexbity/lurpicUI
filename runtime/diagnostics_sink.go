package runtime

import "codeburg.org/lexbit/lurpicui/facet"

// DirtySnapshot is a point-in-time copy of one frame's dirty set and the
// per-facet invalidation source recorded when each facet was last marked dirty
// (F-dirtysources). It is produced at the frame snapshot point — the same
// place the runtime computes its own layout/projection dirty set — so it
// carries exactly what the current frame will re-layout and re-project.
type DirtySnapshot struct {
	// FrameNumber is the runtime frame that produced this snapshot.
	FrameNumber uint64
	// Dirty maps each facet that the frame must re-layout/re-project to its
	// dirty flags. Nil when no facet is dirty.
	Dirty map[facet.FacetID]facet.DirtyFlags
	// Sources maps each dirty facet to the invalidation source string
	// recorded when it was last marked dirty (the runtime's dirtySources).
	Sources map[facet.FacetID]string
}

// DirtySnapshotSink is an optional capability a DiagnosticsHook implementer
// may add (structurally) to receive one DirtySnapshot per frame, delivered on
// the runtime thread at the dirty-set snapshot point. The runtime discovers it
// by type assertion, mirroring the poisoningSink / backendFallbackSink /
// overlayInjector patterns — the DiagnosticsHook interface itself is not
// widened.
//
// The sink MUST NOT render synchronously: it should stage the snapshot for a
// later, on-thread consumer so the observed frame's critical path is
// unchanged (NFR-introspect-neutral).
type DirtySnapshotSink interface {
	OnDirtySnapshot(DirtySnapshot)
}

// dirtySnapshotSinkOf returns the diagnostics hook as a DirtySnapshotSink, or
// nil when it does not implement the interface.
func dirtySnapshotSinkOf(hook DiagnosticsHook) DirtySnapshotSink {
	if hook == nil {
		return nil
	}
	if sink, ok := hook.(DirtySnapshotSink); ok {
		return sink
	}
	return nil
}

// copyDirtySources returns a shallow copy of the runtime's dirtySources map so
// a retained snapshot does not alias the live map (which the next frame
// mutates).
func (rt *Runtime) copyDirtySources() map[facet.FacetID]string {
	if len(rt.dirtySources) == 0 {
		return nil
	}
	out := make(map[facet.FacetID]string, len(rt.dirtySources))
	for id, src := range rt.dirtySources {
		out[id] = src
	}
	return out
}

// buildDirtySnapshot assembles the frame's dirty set. The runtime's own
// tracker (rt.dirtyFacets) covers invalidations routed through runtime
// services (rt.Invalidate, jobs, markTreeDirty); facets invalidated through
// store-bound bindings carry only local dirty bits, so the tree walk merges
// those in — this is exactly the per-frame set the layout and projection
// passes consume. Sources prefer the runtime's dirtySources, falling back to
// each facet's locally-recorded invalidation source.
func (rt *Runtime) buildDirtySnapshot(frameNumber uint64) DirtySnapshot {
	snap := DirtySnapshot{
		FrameNumber: frameNumber,
		Dirty:       rt.copyDirtyFacets(),
		Sources:     rt.copyDirtySources(),
	}
	rt.walkDirtyTree(rt.root, &snap)
	return snap
}

func (rt *Runtime) walkDirtyTree(root facet.FacetImpl, snap *DirtySnapshot) {
	if root == nil || root.Base() == nil {
		return
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		base := node.Base()
		if flags := base.DirtyFlags(); flags != 0 {
			if snap.Dirty == nil {
				snap.Dirty = make(map[facet.FacetID]facet.DirtyFlags)
			}
			snap.Dirty[base.ID()] |= flags
			if src := base.LastInvalidatedBy(); src != "" {
				if snap.Sources == nil {
					snap.Sources = make(map[facet.FacetID]string)
				}
				if _, ok := snap.Sources[base.ID()]; !ok {
					snap.Sources[base.ID()] = src
				}
			}
		}
		for _, child := range base.Children() {
			stack = append(stack, child)
		}
	}
}
