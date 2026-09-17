package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
)

// recordingRuntime records RuntimeServices.Invalidate calls so a test can
// assert what a propagation routed. It implements facet.RuntimeServices.
type recordingRuntime struct {
	invalidated map[facet.FacetID]facet.DirtyFlags
}

func (r *recordingRuntime) Schedule(j job.AnyJob)  {}
func (r *recordingRuntime) CancelJob(id job.JobID) {}
func (r *recordingRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if r.invalidated == nil {
		r.invalidated = make(map[facet.FacetID]facet.DirtyFlags)
	}
	r.invalidated[id] |= flags
}

// groupHost is a minimal group-parent host (declares Parent.Kind) that arranges
// two leaves side by side.
type groupHost struct {
	facet.Facet
	layout facet.LayoutRole
	left   *leaf
	right  *leaf
}

func newGroupHost() *groupHost {
	h := &groupHost{
		Facet: facet.NewFacet(),
		left:  newLeaf("left"),
		right: newLeaf("right"),
	}
	h.AddChild(h.left.Base())
	h.AddChild(h.right.Base())
	h.layout = facet.LayoutRole{
		Parent: facet.GroupParentContract{
			Kind:     facet.GroupLayoutLinearHorizontal,
			Policy:   nopGroupPolicy{},
			Children: h,
		},
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			h.layout.ArrangedBounds = bounds
			half := bounds.Width() / 2
			h.left.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, half, bounds.Height()))
			h.right.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X+half, bounds.Min.Y, bounds.Width()-half, bounds.Height()))
		},
	}
	h.layout.Child = facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear}
	h.AddRole(&h.layout)
	return h
}

func (h *groupHost) Children() []facet.GroupChild   { return nil }
func (h *groupHost) Base() *facet.Facet             { h.BindImpl(h); return &h.Facet }
func (h *groupHost) OnAttach(_ facet.AttachContext) {}
func (h *groupHost) OnDetach()                      {}
func (h *groupHost) OnActivate()                    {}
func (h *groupHost) OnDeactivate()                  {}

type nopGroupPolicy struct{}

func (nopGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (nopGroupPolicy) MeasureGroup(facet.GroupMeasureContext, []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (nopGroupPolicy) ArrangeGroup(facet.GroupArrangeContext, []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}

// leaf is a content leaf whose measure result is derived from a bound value.
type leaf struct {
	facet.Facet
	layout facet.LayoutRole
	name   string
}

func newLeaf(name string) *leaf {
	l := &leaf{Facet: facet.NewFacet(), name: name}
	l.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		w := float32(9)
		if name == "left" {
			w = 9 // "9" digits wide
		}
		return facet.MeasureResult{Size: gfx.Size{W: w, H: 12}}
	}
	l.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.layout.ArrangedBounds = bounds
	}
	l.AddRole(&l.layout)
	return l
}

func (l *leaf) Base() *facet.Facet             { l.BindImpl(l); return &l.Facet }
func (l *leaf) OnAttach(_ facet.AttachContext) {}
func (l *leaf) OnDetach()                      {}
func (l *leaf) OnActivate()                    {}
func (l *leaf) OnDeactivate()                  {}

func TestNearestLayoutRoot_returnsNearestGroupAncestor(t *testing.T) {
	host := newGroupHost()
	attachAll(t, host)

	// The leaf's nearest layout root is the host (its only group ancestor).
	if got := NearestLayoutRoot(host.left); got == nil || got.Base().ID() != host.ID() {
		t.Fatalf("NearestLayoutRoot(left) = %v, want the host", got)
	}
	// The host itself declares a group contract, so it is its own layout root.
	if got := NearestLayoutRoot(host); got == nil || got.Base().ID() != host.ID() {
		t.Fatalf("NearestLayoutRoot(host) = %v, want the host", got)
	}
}

func TestNearestLayoutRoot_returnsAppRootWhenNoGroupAncestor(t *testing.T) {
	leaf := newLeaf("orphan")
	attachAll(t, leaf)
	// A facet with no group-contract ancestor resolves to itself (the app
	// root of its subtree) so propagation always has a routing target.
	if got := NearestLayoutRoot(leaf); got == nil || got.Base().ID() != leaf.ID() {
		t.Fatalf("NearestLayoutRoot(orphan) = %v, want the orphan itself", got)
	}
}

func TestPropagateContentDirty_layoutFlaggedRoutesNearestRoot(t *testing.T) {
	host := newGroupHost()
	attachAll(t, host)
	rt := &recordingRuntime{}

	PropagateContentDirty(host.left, rt, "test", facet.DirtyLayout|facet.DirtyProjection)

	// The leaf re-measures/re-projects locally ...
	flags := host.left.Base().DirtyFlags()
	if flags&facet.DirtyLayout == 0 || flags&facet.DirtyProjection == 0 {
		t.Fatalf("leaf dirty flags = %v, want DirtyLayout|DirtyProjection", flags)
	}
	// ... and the nearest layout root is routed DirtyLayout for the layout pass.
	if rt.invalidated[host.left.ID()]&facet.DirtyProjection == 0 {
		t.Fatalf("leaf not entered in the runtime map: %v", rt.invalidated[host.left.ID()])
	}
	if rt.invalidated[host.ID()]&facet.DirtyLayout == 0 {
		t.Fatalf("host (nearest layout root) not routed DirtyLayout: %v", rt.invalidated[host.ID()])
	}
}

func TestPropagateContentDirty_projectionOnlyDoesNotRouteLayout(t *testing.T) {
	host := newGroupHost()
	attachAll(t, host)
	rt := &recordingRuntime{}

	PropagateContentDirty(host.right, rt, "test", facet.DirtyProjection)

	flags := host.right.Base().DirtyFlags()
	if flags&facet.DirtyLayout != 0 {
		t.Fatal("projection-only content change must not re-measure (FR-3 flag-honoring)")
	}
	if flags&facet.DirtyProjection == 0 {
		t.Fatal("expected DirtyProjection after projection-only content change")
	}
	if _, ok := rt.invalidated[host.ID()]; ok {
		t.Fatalf("projection-only change routed the layout root: %v", rt.invalidated)
	}
}

// TestPropagateContentDirty_idempotentRoutesSameRoot proves double propagation
// in one frame routes the SAME nearest layout root — the runtime's OR-flag
// semantics then dedup it into a single layout pass (RX-1 Q3 idempotence).
func TestPropagateContentDirty_idempotentRoutesSameRoot(t *testing.T) {
	host := newGroupHost()
	attachAll(t, host)
	rt := &recordingRuntime{}

	PropagateContentDirty(host.left, rt, "a", facet.DirtyLayout|facet.DirtyProjection)
	PropagateContentDirty(host.left, rt, "b", facet.DirtyLayout|facet.DirtyProjection)

	// Both propagations route the same root id; the runtime OR-flags it, so the
	// layout pass runs once. The recording runtime coalesces into one entry.
	if got := rt.invalidated[host.ID()] & facet.DirtyLayout; got == 0 {
		t.Fatalf("layout root routed %v, want DirtyLayout", rt.invalidated[host.ID()])
	}
}

func attachAll(t *testing.T, impl facet.FacetImpl) {
	t.Helper()
	facet.Attach(impl, facet.AttachContext{})
}
