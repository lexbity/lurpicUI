package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type incrementalLayoutRoot struct {
	facet.Facet
	layout facet.LayoutRole
	left   *layoutCountLeaf
	right  *layoutCountLeaf

	measureCount int
	arrangeCount int
}

func (r *incrementalLayoutRoot) Base() *facet.Facet {
	r.BindImpl(r)
	return &r.Facet
}

func newIncrementalLayoutRoot(left, right *layoutCountLeaf) *incrementalLayoutRoot {
	root := &incrementalLayoutRoot{
		Facet: facet.NewFacet(),
		left:  left,
		right: right,
	}
	root.layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	root.layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear,
	}
	root.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		root.measureCount++
		leftSize := left.layout.Measure(ctx, c).Size
		rightSize := right.layout.Measure(ctx, c).Size
		return facet.MeasureResult{
			Size: gfx.Size{
				W: leftSize.W + rightSize.W,
				H: maxFloat32(leftSize.H, rightSize.H),
			},
		}
	}
	root.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		root.arrangeCount++
		root.layout.ArrangedBounds = bounds
		half := bounds.Width() / 2
		left.layout.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, half, bounds.Height()))
		right.layout.Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X+half, bounds.Min.Y, bounds.Width()-half, bounds.Height()))
	}
	root.AddRole(&root.layout)
	root.AddChild(left.Base())
	root.AddChild(right.Base())
	return root
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func TestRuntimeRunLayoutPassPrunesCleanSiblingCaches(t *testing.T) {
	left := newLayoutCountLeaf(gfx.Size{W: 10, H: 12})
	right := newLayoutCountLeaf(gfx.Size{W: 20, H: 18})
	root := newIncrementalLayoutRoot(left, right)
	rt := mustRuntimeTree(t, root)

	rt.markTreeDirty(root, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 200, H: 100})
	if root.measureCount != 1 || root.arrangeCount != 1 {
		t.Fatalf("root counts = %d/%d, want 1/1", root.measureCount, root.arrangeCount)
	}
	if left.measureCount != 1 || left.arrangeCount != 1 {
		t.Fatalf("left counts = %d/%d, want 1/1", left.measureCount, left.arrangeCount)
	}
	if right.measureCount != 1 || right.arrangeCount != 1 {
		t.Fatalf("right counts = %d/%d, want 1/1", right.measureCount, right.arrangeCount)
	}

	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	root.Base().InvalidateWithSource(facet.DirtyLayout, "test")
	left.Base().InvalidateWithSource(facet.DirtyLayout, "test")
	rt.dirtyFacets[root.ID()] = facet.DirtyLayout
	rt.dirtyFacets[left.ID()] = facet.DirtyLayout

	rt.runLayoutPass(gfx.Size{W: 200, H: 100})
	if root.measureCount != 2 || root.arrangeCount != 2 {
		t.Fatalf("root counts = %d/%d, want 2/2", root.measureCount, root.arrangeCount)
	}
	if left.measureCount != 2 || left.arrangeCount != 2 {
		t.Fatalf("left counts = %d/%d, want 2/2", left.measureCount, left.arrangeCount)
	}
	if right.measureCount != 1 || right.arrangeCount != 1 {
		t.Fatalf("right counts = %d/%d, want 1/1 after sibling prune", right.measureCount, right.arrangeCount)
	}
}

// gatingHost arranges its single child to gfx.Rect{}, mirroring a Stage that
// hides inactive exhibits. It is used to exercise the F-layout-root-fallback
// re-gate: when the child is independently re-arranged it must stay empty
// rather than fall back to the full window.
type gatingHost struct {
	facet.Facet
	layout facet.LayoutRole
	child  facet.FacetImpl
}

func (h *gatingHost) Base() *facet.Facet {
	h.BindImpl(h)
	return &h.Facet
}

func newGatingHost(child facet.FacetImpl) *gatingHost {
	h := &gatingHost{Facet: facet.NewFacet(), child: child}
	h.layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	h.layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear,
	}
	h.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	h.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
		// Gate the child to empty bounds — the gating parent contract.
		if role := child.Base().LayoutRole(); role != nil {
			role.Arrange(ctx, gfx.Rect{})
		}
	}
	h.AddRole(&h.layout)
	h.AddChild(child.Base())
	return h
}

// TestRuntimeRunLayoutPassReGatesChildWhenGatingParentEmpty pins
// F-layout-root-fallback: a child gated to empty bounds by its parent must
// stay empty when the runtime independently re-arranges it, rather than being
// resurrected with the full window bounds.
func TestRuntimeRunLayoutPassReGatesChildWhenGatingParentEmpty(t *testing.T) {
	leaf := newLayoutCountLeaf(gfx.Size{W: 50, H: 50})
	host := newGatingHost(leaf)
	rt := mustRuntimeTree(t, host)

	// First pass: arrange the host at window bounds; it gates the leaf empty.
	rt.markTreeDirty(host, facet.DirtyLayout)
	rt.runLayoutPass(gfx.Size{W: 400, H: 300})
	if got := leaf.layout.ArrangedBounds; !got.IsEmpty() {
		t.Fatalf("leaf bounds after gating pass = %v, want empty", got)
	}

	// Now the leaf is marked DirtyLayout on its own (e.g. a store subscription),
	// without the host — so selectedLayoutRoots selects the leaf as an
	// independent root whose own ArrangedBounds is empty.
	leaf.Base().InvalidateWithSource(facet.DirtyLayout, "test")
	rt.dirtyFacets = map[facet.FacetID]facet.DirtyFlags{leaf.Base().ID(): facet.DirtyLayout}
	rt.runLayoutPass(gfx.Size{W: 400, H: 300})

	// Pre-fix the leaf would have been resurrected to the full window (400x300);
	// the re-gate must keep it empty because its gating parent is empty.
	if got := leaf.layout.ArrangedBounds; !got.IsEmpty() {
		t.Fatalf("leaf bounds after independent re-arrange = %v, want empty (gating parent re-gate)", got)
	}
}
