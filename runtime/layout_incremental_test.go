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
	r.Facet.BindImpl(r)
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
