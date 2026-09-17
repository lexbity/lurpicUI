package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/store"
)

// storeSizedLeaf is a leaf whose measured width follows a store — the AC-4
// "label grows longer" shape. Routing a content change at it must re-measure
// it through its host's layout root.
type storeSizedLeaf struct {
	facet.Facet
	lrole facet.LayoutRole
	extra *store.ValueStore[float32]
	base  float32
}

func newStoreSizedLeaf(base float32, extra *store.ValueStore[float32]) *storeSizedLeaf {
	l := &storeSizedLeaf{Facet: facet.NewFacet(), base: base, extra: extra}
	l.lrole.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: l.base + l.extra.Get(), H: 12}}
	}
	l.lrole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.lrole.ArrangedBounds = bounds
	}
	l.AddRole(&l.lrole)
	return l
}

func (l *storeSizedLeaf) Base() *facet.Facet             { l.BindImpl(l); return &l.Facet }
func (l *storeSizedLeaf) OnAttach(_ facet.AttachContext) {}
func (l *storeSizedLeaf) OnDetach()                      {}
func (l *storeSizedLeaf) OnActivate()                    {}
func (l *storeSizedLeaf) OnDeactivate()                  {}

// reactiveGroupHost arranges a left and right leaf side by side from their
// measured widths, so a left growth pushes the right — no overlap.
type reactiveGroupHost struct {
	facet.Facet
	lrole facet.LayoutRole
	left  *storeSizedLeaf
	right *storeSizedLeaf
}

func newReactiveGroupHost(left, right *storeSizedLeaf) *reactiveGroupHost {
	h := &reactiveGroupHost{Facet: facet.NewFacet(), left: left, right: right}
	h.AddChild(left.Base())
	h.AddChild(right.Base())
	h.lrole = facet.LayoutRole{
		Parent: facet.GroupParentContract{
			Kind:     facet.GroupLayoutLinearHorizontal,
			Policy:   nopRuntimeGroupPolicy{},
			Children: h,
		},
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			h.left.lrole.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
			h.right.lrole.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			h.lrole.ArrangedBounds = bounds
			leftW := h.left.lrole.MeasuredSize.W
			h.left.lrole.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, leftW, bounds.Height()))
			rightW := h.right.lrole.MeasuredSize.W
			h.right.lrole.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X+leftW, bounds.Min.Y, rightW, bounds.Height()))
		},
	}
	h.lrole.Child = facet.GroupChildContract{SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear}
	h.AddRole(&h.lrole)
	return h
}

func (h *reactiveGroupHost) Children() []facet.GroupChild   { return nil }
func (h *reactiveGroupHost) Base() *facet.Facet             { h.BindImpl(h); return &h.Facet }
func (h *reactiveGroupHost) OnAttach(_ facet.AttachContext) {}
func (h *reactiveGroupHost) OnDetach()                      {}
func (h *reactiveGroupHost) OnActivate()                    {}
func (h *reactiveGroupHost) OnDeactivate()                  {}

type nopRuntimeGroupPolicy struct{}

func (nopRuntimeGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (nopRuntimeGroupPolicy) MeasureGroup(facet.GroupMeasureContext, []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (nopRuntimeGroupPolicy) ArrangeGroup(facet.GroupArrangeContext, []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}

// TestLayoutDirtyRoots_resolvesMidTreeFacetToNearestRoot proves the RX-1
// selectedLayoutRoots extension: a mid-tree facet routed DirtyLayout selects
// its nearest layout root (the host), not the facet itself, so the re-laid
// host re-measures the whole subtree instead of re-arranging the facet in
// isolation.
func TestLayoutDirtyRoots_resolvesMidTreeFacetToNearestRoot(t *testing.T) {
	left := newStoreSizedLeaf(10, store.NewValueStore[float32](0))
	right := newStoreSizedLeaf(20, store.NewValueStore[float32](0))
	host := newReactiveGroupHost(left, right)
	rt := mustRuntimeTree(t, host)

	// Mark everything dirty and run one layout pass so the tree settles.
	rt.markTreeDirty(host, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 400, H: 100})

	// Route the LEFT leaf as dirty — as if a content change hit it.
	rt.dirtyFacets = map[facet.FacetID]facet.DirtyFlags{left.ID(): facet.DirtyLayout}
	left.lrole.InvalidateCache()
	roots := rt.layoutDirtyRoots()

	if len(roots) != 1 || roots[0].Base().ID() != host.ID() {
		t.Fatalf("layoutDirtyRoots = %d roots, want exactly the host", len(roots))
	}
}

// TestLayoutReactivity_leafGrowsRelaysParentPushesSibling proves AC-4: a store
// write to a label that makes it grow longer re-measures the parent group and
// re-arranges it within one frame, pushing the sibling so the two never
// overlap.
func TestLayoutReactivity_leafGrowsRelaysParentPushesSibling(t *testing.T) {
	extra := store.NewValueStore[float32](0)
	left := newStoreSizedLeaf(9, extra)
	right := newStoreSizedLeaf(30, store.NewValueStore[float32](0))
	host := newReactiveGroupHost(left, right)
	rt := mustRuntimeTree(t, host)

	rt.markTreeDirty(host, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 400, H: 100})

	before := right.lrole.ArrangedBounds.Min.X

	// The label grows "9" → "1024": widen the leaf by 40px and route the
	// content change through the FR-3 propagation.
	extra.Set(40)
	layout.PropagateContentDirty(left, rt, "ac4", facet.DirtyLayout|facet.DirtyProjection)
	rt.runLayoutPass(gfx.Size{W: 400, H: 100})

	if got := left.lrole.MeasuredSize.W; got != 49 {
		t.Fatalf("left measured width = %v, want 49 (re-measured)", got)
	}
	if got := right.lrole.ArrangedBounds.Min.X; got <= before {
		t.Fatalf("right was not pushed: x %v -> %v", before, got)
	}
	if right.lrole.ArrangedBounds.Min.X < left.lrole.ArrangedBounds.Max.X {
		t.Fatalf("overlap: right.Min.X %v < left.Max.X %v", right.lrole.ArrangedBounds.Min.X, left.lrole.ArrangedBounds.Max.X)
	}
}
