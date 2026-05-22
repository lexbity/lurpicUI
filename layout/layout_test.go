package layout

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

type testLeaf struct {
	facet.Facet
	layout facet.LayoutRole

	measuredSize    gfx.Size
	lastConstraints Constraints
	arrangedBounds  gfx.Rect
	measureCount    int
	arrangeCount    int
	measureFn       func(Constraints) gfx.Size
}

func newTestLeaf(size gfx.Size) *testLeaf {
	return newTestLeafWithMeasure(size, nil)
}

func newTestLeafWithMeasure(size gfx.Size, fn func(Constraints) gfx.Size) *testLeaf {
	l := &testLeaf{
		Facet:        facet.NewFacet(),
		measuredSize: size,
		measureFn:    fn,
	}
	l.layout.OnMeasure = func(ctx facet.MeasureContext, c Constraints) facet.MeasureResult {
		l.measureCount++
		l.lastConstraints = c
		if l.measureFn != nil {
			return facet.MeasureResult{Size: l.measureFn(c)}
		}
		return facet.MeasureResult{Size: l.measuredSize}
	}
	l.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		l.arrangeCount++
		l.arrangedBounds = bounds
	}
	l.AddRole(&l.layout)
	return l
}

func (l *testLeaf) Base() *facet.Facet {
	l.Facet.BindImpl(l)
	return &l.Facet
}
func (l *testLeaf) OnAttach(ctx facet.AttachContext) {}
func (l *testLeaf) OnDetach()                        {}
func (l *testLeaf) OnActivate()                      {}
func (l *testLeaf) OnDeactivate()                    {}

func runLayout(root facet.FacetImpl, windowSize gfx.Size) {
	sys := NewSystem()
	sys.MarkDirty(root)
	sys.Run(windowSize)
}

func TestConstraints_tight_min_equals_max(t *testing.T) {
	s := gfx.Size{W: 12, H: 34}
	c := Tight(s)
	if !c.IsTight() {
		t.Fatal("expected tight constraints")
	}
	if c.MinSize != s || c.MaxSize != s {
		t.Fatalf("unexpected constraints: %#v", c)
	}
}

func TestConstraints_constrain_clamps(t *testing.T) {
	c := Constraints{
		MinSize: gfx.Size{W: 10, H: 20},
		MaxSize: gfx.Size{W: 30, H: 40},
	}
	got := c.Constrain(gfx.Size{W: 5, H: 50})
	if got.W != 10 || got.H != 40 {
		t.Fatalf("unexpected constrained size: %#v", got)
	}
}

func TestConstraints_loose_no_minimum(t *testing.T) {
	c := Loose(gfx.Size{W: 100, H: 200})
	if c.MinSize != (gfx.Size{}) {
		t.Fatalf("expected zero min, got %#v", c.MinSize)
	}
}

func TestConstraints_unconstrained_no_limits(t *testing.T) {
	c := Unconstrained()
	if c.MinSize != (gfx.Size{}) || c.MaxSize != (gfx.Size{}) {
		t.Fatalf("expected unconstrained, got %#v", c)
	}
}

func TestStackLayout_size_is_largest_child(t *testing.T) {
	root := NewStackLayout(AlignTopLeft)
	a := newTestLeaf(gfx.Size{W: 40, H: 20})
	b := newTestLeaf(gfx.Size{W: 60, H: 35})
	root.AddChild(a)
	root.AddChild(b)

	runLayout(root, gfx.Size{W: 200, H: 200})

	if got := root.Base().LayoutRole().MeasuredSize; got != (gfx.Size{W: 60, H: 35}) {
		t.Fatalf("unexpected stack size: %#v", got)
	}
}

func TestStackLayout_alignment_topleft(t *testing.T) {
	root := NewStackLayout(AlignTopLeft)
	child := newTestLeaf(gfx.Size{W: 20, H: 10})
	root.AddChild(child)

	runLayout(root, gfx.Size{W: 100, H: 100})

	if child.arrangedBounds.Min != (gfx.Point{}) {
		t.Fatalf("unexpected bounds: %#v", child.arrangedBounds)
	}
}

func TestStackLayout_alignment_center(t *testing.T) {
	root := NewStackLayout(AlignCenter)
	child := newTestLeaf(gfx.Size{W: 20, H: 10})
	root.AddChild(child)

	runLayout(root, gfx.Size{W: 100, H: 100})

	if child.arrangedBounds.Min != (gfx.Point{X: 40, Y: 45}) {
		t.Fatalf("unexpected bounds: %#v", child.arrangedBounds)
	}
}

func TestRowLayout_fixed_children(t *testing.T) {
	row := NewRowLayout()
	a := newTestLeaf(gfx.Size{W: 100, H: 20})
	b := newTestLeaf(gfx.Size{W: 60, H: 20})
	row.Add(Fixed(a))
	row.Add(Fixed(b))

	runLayout(row, gfx.Size{W: 400, H: 50})

	if got := row.Base().LayoutRole().MeasuredSize.W; got != 160 {
		t.Fatalf("unexpected width: %v", got)
	}
}

func TestRowLayout_flex_child_gets_remainder(t *testing.T) {
	row := NewRowLayout()
	fixed := newTestLeaf(gfx.Size{W: 100, H: 20})
	flex := newTestLeafWithMeasure(gfx.Size{}, func(c Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 20}
	})
	row.Add(Fixed(fixed))
	row.Add(Flexible(flex, 1))

	runLayout(row, gfx.Size{W: 400, H: 50})

	if got := flex.arrangedBounds.Width(); got != 300 {
		t.Fatalf("unexpected flex width: %v", got)
	}
}

func TestRowLayout_multiple_flex_children_proportional(t *testing.T) {
	row := NewRowLayout()
	a := newTestLeafWithMeasure(gfx.Size{}, func(c Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 20}
	})
	b := newTestLeafWithMeasure(gfx.Size{}, func(c Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 20}
	})
	row.Add(Flexible(a, 1))
	row.Add(Flexible(b, 2))

	runLayout(row, gfx.Size{W: 300, H: 50})

	if got := a.arrangedBounds.Width(); got != 100 {
		t.Fatalf("unexpected first flex width: %v", got)
	}
	if got := b.arrangedBounds.Width(); got != 200 {
		t.Fatalf("unexpected second flex width: %v", got)
	}
}

func TestRowLayout_gap_included_in_total(t *testing.T) {
	row := NewRowLayout()
	row.Gap = 10
	a := newTestLeaf(gfx.Size{W: 50, H: 20})
	b := newTestLeaf(gfx.Size{W: 60, H: 20})
	row.Add(Fixed(a))
	row.Add(Fixed(b))

	runLayout(row, gfx.Size{W: 400, H: 50})

	if got := row.Base().LayoutRole().MeasuredSize.W; got != 120 {
		t.Fatalf("unexpected row width: %v", got)
	}
}

func TestRowLayout_padding_reduces_available_space(t *testing.T) {
	row := NewRowLayout()
	row.Padding = gfx.Insets{Top: 5, Right: 10, Bottom: 5, Left: 10}
	child := newTestLeaf(gfx.Size{W: 50, H: 20})
	row.Add(Fixed(child))

	runLayout(row, gfx.Size{W: 200, H: 100})

	if child.arrangedBounds.Min.X != 10 || child.arrangedBounds.Min.Y != 5 {
		t.Fatalf("unexpected child bounds: %#v", child.arrangedBounds)
	}
}

func TestColumnLayout_vertical_equivalent_to_row(t *testing.T) {
	col := NewColumnLayout()
	a := newTestLeaf(gfx.Size{W: 20, H: 100})
	b := newTestLeafWithMeasure(gfx.Size{}, func(c Constraints) gfx.Size {
		return gfx.Size{W: 20, H: c.MaxSize.H}
	})
	col.Add(Fixed(a))
	col.Add(Flexible(b, 1))

	runLayout(col, gfx.Size{W: 50, H: 400})

	if got := b.arrangedBounds.Height(); got != 300 {
		t.Fatalf("unexpected flex height: %v", got)
	}
}

func TestPaddingLayout_adds_insets(t *testing.T) {
	child := newTestLeaf(gfx.Size{W: 40, H: 20})
	pad := NewPaddingLayout(child, gfx.Insets{Top: 5, Right: 7, Bottom: 3, Left: 9})

	runLayout(pad, gfx.Size{W: 200, H: 100})

	if got := pad.Base().LayoutRole().MeasuredSize; got != (gfx.Size{W: 56, H: 28}) {
		t.Fatalf("unexpected padded size: %#v", got)
	}
}

func TestPaddingLayout_deflates_constraints(t *testing.T) {
	child := newTestLeafWithMeasure(gfx.Size{W: 10, H: 10}, func(c Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: c.MaxSize.H}
	})
	pad := NewPaddingLayout(child, gfx.Insets{Top: 5, Right: 7, Bottom: 3, Left: 9})

	runLayout(pad, gfx.Size{W: 100, H: 80})

	if child.lastConstraints.MaxSize.W != 84 || child.lastConstraints.MaxSize.H != 72 {
		t.Fatalf("unexpected deflated constraints: %#v", child.lastConstraints)
	}
}

func TestSystem_clearLayoutDirtyIterative(t *testing.T) {
	const depth = 2048
	root := newTestLeaf(gfx.Size{W: 10, H: 10})
	current := root
	for i := 1; i < depth; i++ {
		child := newTestLeaf(gfx.Size{W: 10, H: 10})
		current.AddChild(&child.Facet)
		current = child
	}

	root.Base().Invalidate(facet.DirtyLayout)
	current.Base().Invalidate(facet.DirtyLayout)
	runLayout(root, gfx.Size{W: 100, H: 100})

	if got := root.Base().DirtyFlags(); got&facet.DirtyLayout != 0 {
		t.Fatalf("expected root layout dirtiness to be cleared, got %#v", got)
	}
	if got := current.Base().DirtyFlags(); got != 0 {
		t.Fatalf("expected deep leaf dirtiness to be cleared, got %#v", got)
	}
}

func TestSizedBox_forces_width_and_height(t *testing.T) {
	child := newTestLeafWithMeasure(gfx.Size{}, func(c Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: c.MaxSize.H}
	})
	box := NewSizedBox(100, 50, child)

	runLayout(box, gfx.Size{W: 200, H: 200})

	if child.lastConstraints.MinSize != (gfx.Size{W: 100, H: 50}) || child.lastConstraints.MaxSize != (gfx.Size{W: 100, H: 50}) {
		t.Fatalf("unexpected sized box constraints: %#v", child.lastConstraints)
	}
}

func TestSizedBox_nil_child_is_empty_space(t *testing.T) {
	box := NewSizedBox(100, 50, nil)

	runLayout(box, gfx.Size{W: 200, H: 200})

	if got := box.Base().LayoutRole().MeasuredSize; got != (gfx.Size{W: 100, H: 50}) {
		t.Fatalf("unexpected box size: %#v", got)
	}
}

func TestSplitLayout_horizontal_split_fraction(t *testing.T) {
	split := NewSplitLayout(SplitHorizontal, 0.3)
	first := newTestLeaf(gfx.Size{W: 10, H: 20})
	second := newTestLeaf(gfx.Size{W: 10, H: 20})
	split.SetFirst(first)
	split.SetSecond(second)

	runLayout(split, gfx.Size{W: 400, H: 100})

	if got := first.arrangedBounds.Width(); got != 120 {
		t.Fatalf("unexpected first width: %v", got)
	}
	if got := second.arrangedBounds.Width(); got != 280 {
		t.Fatalf("unexpected second width: %v", got)
	}
}

func TestSplitLayout_divider_width_respected(t *testing.T) {
	split := NewSplitLayout(SplitHorizontal, 0.5)
	split.DividerWidth = 4
	first := newTestLeaf(gfx.Size{W: 10, H: 20})
	second := newTestLeaf(gfx.Size{W: 10, H: 20})
	split.SetFirst(first)
	split.SetSecond(second)

	runLayout(split, gfx.Size{W: 400, H: 100})

	if got := first.arrangedBounds.Width(); got != 198 {
		t.Fatalf("unexpected first width: %v", got)
	}
	if got := second.arrangedBounds.Width(); got != 198 {
		t.Fatalf("unexpected second width: %v", got)
	}
}

func TestScrollLayout_child_unconstrained_on_scroll_axis(t *testing.T) {
	child := newTestLeafWithMeasure(gfx.Size{W: 50, H: 50}, func(c Constraints) gfx.Size {
		return gfx.Size{W: 50, H: 50}
	})
	scroll := NewScrollLayout(ScrollVertical, child)

	runLayout(scroll, gfx.Size{W: 100, H: 100})

	if child.lastConstraints.MaxSize.H != 0 {
		t.Fatalf("expected unconstrained height, got %#v", child.lastConstraints)
	}
}

func TestScrollLayout_scrolloffset_positions_child(t *testing.T) {
	child := newTestLeaf(gfx.Size{W: 50, H: 200})
	scroll := NewScrollLayout(ScrollVertical, child)
	scroll.ScrollOffset = gfx.Point{X: 0, Y: 100}

	runLayout(scroll, gfx.Size{W: 100, H: 100})

	if got := child.arrangedBounds.Min.Y; got != -100 {
		t.Fatalf("unexpected child y: %v", got)
	}
}

func TestLayoutSystem_only_dirty_subtrees_remeasure(t *testing.T) {
	root := NewStackLayout(AlignTopLeft)
	left := NewRowLayout()
	right := NewRowLayout()
	leftLeaf := newTestLeaf(gfx.Size{W: 10, H: 10})
	rightLeaf := newTestLeaf(gfx.Size{W: 10, H: 10})
	left.Add(Fixed(leftLeaf))
	right.Add(Fixed(rightLeaf))
	root.AddChild(left)
	root.AddChild(right)

	sys := NewSystem()
	sys.MarkDirty(left)
	sys.Run(gfx.Size{W: 200, H: 200})

	if leftLeaf.measureCount == 0 {
		t.Fatal("expected left subtree to measure")
	}
	if rightLeaf.measureCount != 0 {
		t.Fatalf("expected right subtree to stay clean, got %d", rightLeaf.measureCount)
	}
}

func TestLayoutSystem_run_clears_dirty(t *testing.T) {
	root := NewStackLayout(AlignTopLeft)
	child := newTestLeaf(gfx.Size{W: 10, H: 10})
	root.AddChild(child)

	sys := NewSystem()
	sys.MarkDirty(root)
	sys.Run(gfx.Size{W: 100, H: 100})

	if got := root.Base().DirtyFlags() & facet.DirtyLayout; got != 0 {
		t.Fatalf("expected dirty cleared, got %v", got)
	}
	if got := child.Base().DirtyFlags() & facet.DirtyLayout; got != 0 {
		t.Fatalf("expected child dirty cleared, got %v", got)
	}
}

func TestLayoutSystem_selectedRoots_prunes_dirty_descendants(t *testing.T) {
	root := NewStackLayout(AlignTopLeft)
	mid := NewStackLayout(AlignTopLeft)
	leaf := newTestLeaf(gfx.Size{W: 10, H: 10})
	root.AddChild(mid)
	mid.AddChild(leaf)

	sys := NewSystem()
	sys.dirtyRoots[root.ID()] = root
	sys.dirtyRoots[leaf.ID()] = leaf

	roots := sys.selectedRoots()
	if len(roots) != 1 || roots[0] != root {
		t.Fatalf("selected roots = %#v", roots)
	}
}

func TestAlignedOrigin_all_nine_alignments(t *testing.T) {
	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	child := gfx.Size{W: 20, H: 10}
	seen := make(map[gfx.Point]struct{})
	for _, a := range []Alignment{
		AlignTopLeft,
		AlignTopCenter,
		AlignTopRight,
		AlignCenterLeft,
		AlignCenter,
		AlignCenterRight,
		AlignBottomLeft,
		AlignBottomCenter,
		AlignBottomRight,
	} {
		p := alignedOrigin(child, bounds, a)
		seen[p] = struct{}{}
	}
	if len(seen) != 9 {
		t.Fatalf("expected 9 unique alignments, got %d", len(seen))
	}
}
