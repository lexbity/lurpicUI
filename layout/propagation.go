package layout

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

// Content-vs-structure rule (RX-1 FR-3 boundary):
//
//   - Content is a change that leaves the facet's child set, child order, and
//     hosting topology untouched — a store write, a binding value change, a
//     label that grows longer. Content changes MUST be routed through
//     PropagateContentDirty so the facet re-measures, re-arranges through its
//     ancestor policies, and re-projects within one frame, with no author-
//     written invalidation routing.
//   - Structure is a change to the child set, child order, or pane/mount
//     topology — adding/removing/re-hosting a pane, swapping the active
//     exhibit, flipping a layer's mount state. Structural changes remain
//     explicit host code: the host mutates the tree and routes its own layout
//     root.
//
// Marks route content changes automatically (marks.Core); bespoke hosts use
// this package's entry point for the same contract.
//
// NearestLayoutRoot returns the nearest ancestor (or f itself) that declares a
// GroupParentContract — a facet whose LayoutRole.Parent.Kind is non-zero — or
// the app root (a facet with no parent) when no such ancestor exists. It is
// the scope within which a content change re-lays: routing the nearest layout
// root re-measures and re-arranges the facet through the policies that arrange
// it, rather than laying the facet out in isolation.
func NearestLayoutRoot(f facet.FacetImpl) facet.FacetImpl {
	if f == nil {
		return nil
	}
	current := f
	for {
		base := current.Base()
		if base == nil {
			return current
		}
		if lr := base.LayoutRole(); lr != nil && lr.Parent.Kind != facet.GroupLayoutNone {
			return current
		}
		parent := base.Parent()
		if parent == nil {
			// App root: no gating parent, so it is the layout root of its own
			// subtree. A root with empty arranged bounds means "not yet
			// arranged", not "hidden" (F-inactive-layer-child excludes the
			// root for the same reason).
			return current
		}
		if impl := parent.Impl(); impl != nil {
			current = impl
		} else {
			current = parent
		}
	}
}

// PropagateContentDirty routes a content invalidation at f through the RX-1
// FR-3 mechanism. It must run on the runtime thread and is idempotent within a
// frame (rt.Invalidate OR-flags per facet id).
//
// The declared flags decide whether the change affects geometry:
//
//   - flags include DirtyLayout: the facet re-measures and re-arranges through
//     its nearest layout root's policies — the root is routed DirtyLayout so
//     the runtime's layout pass re-runs, and the facet's local flags are
//     DirtyLayout|DirtyProjection.
//   - flags are projection-only: the facet re-projects (local DirtyProjection
//     plus a runtime-map entry so the frame's dirty regions are correct) but
//     does NOT re-lay — a data-value change with fixed geometry (a chart rule
//     value, a color) must not re-measure an ancestor layout root.
//
// The facet is always entered in the runtime dirty map so the frame's
// dirty-region computation covers its re-projected pixels. Local flags are set
// even when no runtime is attached so standalone projections still re-run; the
// layout route is a no-op through a nil runtime.
func PropagateContentDirty(f facet.FacetImpl, rt facet.RuntimeServices, source string, flags facet.DirtyFlags) {
	if f == nil || f.Base() == nil {
		return
	}
	base := f.Base()
	if flags&facet.DirtyLayout != 0 {
		base.InvalidateWithSource(facet.DirtyLayout|facet.DirtyProjection, source)
	} else {
		base.InvalidateWithSource(flags|facet.DirtyProjection, source)
	}
	if rt == nil {
		return
	}
	rt.Invalidate(base.ID(), flags|facet.DirtyProjection, source)
	if flags&facet.DirtyLayout == 0 {
		return
	}
	if root := NearestLayoutRoot(f); root != nil && root.Base() != nil && root.Base().ID() != base.ID() {
		rt.Invalidate(root.Base().ID(), facet.DirtyLayout, source)
	}
}
