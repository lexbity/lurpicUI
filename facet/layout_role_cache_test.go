package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type cacheTestGroupPolicy struct {
	variant int
}

func (cacheTestGroupPolicy) Kind() GroupLayoutKind { return GroupLayoutGrid }
func (p cacheTestGroupPolicy) MeasureGroup(ctx GroupMeasureContext, children []GroupChild) (GroupMeasureResult, error) {
	return GroupMeasureResult{}, nil
}
func (p cacheTestGroupPolicy) ArrangeGroup(ctx GroupArrangeContext, children []GroupChild) ([]ArrangedGroupChild, error) {
	return nil, nil
}

type cacheTestGroupSource struct {
	variant int
}

func (cacheTestGroupSource) Children() []GroupChild { return nil }

func cacheTestIntrinsicA(ctx MeasureContext, c Constraints) IntrinsicSize {
	return IntrinsicSize{Min: gfx.Size{W: 1, H: 2}}
}

func cacheTestIntrinsicB(ctx MeasureContext, c Constraints) IntrinsicSize {
	return IntrinsicSize{Min: gfx.Size{W: 3, H: 4}}
}

func TestConstraintsEquals(t *testing.T) {
	a := Constraints{
		MinSize: gfx.Size{W: 1, H: 2},
		MaxSize: gfx.Size{W: 3, H: 4},
	}
	b := Constraints{
		MinSize: gfx.Size{W: 1, H: 2},
		MaxSize: gfx.Size{W: 3, H: 4},
	}
	if !a.Equals(b) {
		t.Fatalf("expected constraints to be equal: %#v vs %#v", a, b)
	}
	if a.Equals(Constraints{MinSize: gfx.Size{W: 1, H: 2}, MaxSize: gfx.Size{W: 3, H: 5}}) {
		t.Fatalf("expected constraints with different max size to differ")
	}
}

func TestGroupParentContractEquals(t *testing.T) {
	a := GroupParentContract{
		Kind:     GroupLayoutGrid,
		Policy:   cacheTestGroupPolicy{variant: 1},
		Overflow: OverflowClip,
		Clipping: GroupClipBounds,
		Children: cacheTestGroupSource{variant: 2},
	}
	b := GroupParentContract{
		Kind:     GroupLayoutGrid,
		Policy:   cacheTestGroupPolicy{variant: 1},
		Overflow: OverflowClip,
		Clipping: GroupClipBounds,
		Children: cacheTestGroupSource{variant: 2},
	}
	if !a.Equals(b) {
		t.Fatalf("expected parent contracts to be equal: %#v vs %#v", a, b)
	}

	if a.Equals(GroupParentContract{
		Kind:     GroupLayoutGrid,
		Policy:   cacheTestGroupPolicy{variant: 9},
		Overflow: OverflowClip,
		Clipping: GroupClipBounds,
		Children: cacheTestGroupSource{variant: 2},
	}) {
		t.Fatalf("expected different policies to be unequal")
	}

	if a.Equals(GroupParentContract{
		Kind:     GroupLayoutGrid,
		Policy:   cacheTestGroupPolicy{variant: 1},
		Overflow: OverflowVisible,
		Clipping: GroupClipBounds,
		Children: cacheTestGroupSource{variant: 2},
	}) {
		t.Fatalf("expected different overflow policies to be unequal")
	}
}

func TestGroupChildContractEquals(t *testing.T) {
	a := GroupChildContract{
		SupportedPlacement: SupportsGrid | SupportsLinear,
		Intrinsic:          cacheTestIntrinsicA,
		Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
		Stretch:            StretchPolicy{Width: StretchAlways},
		Baseline:           BaselineNone,
	}
	b := GroupChildContract{
		SupportedPlacement: SupportsGrid | SupportsLinear,
		Intrinsic:          cacheTestIntrinsicA,
		Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
		Stretch:            StretchPolicy{Width: StretchAlways},
		Baseline:           BaselineNone,
	}
	if !a.Equals(b) {
		t.Fatalf("expected child contracts to be equal: %#v vs %#v", a, b)
	}
	if a.Equals(GroupChildContract{
		SupportedPlacement: SupportsGrid | SupportsLinear,
		Intrinsic:          cacheTestIntrinsicB,
		Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
		Stretch:            StretchPolicy{Width: StretchAlways},
		Baseline:           BaselineNone,
	}) {
		t.Fatalf("expected different intrinsic callbacks to be unequal")
	}
	if a.Equals(GroupChildContract{
		SupportedPlacement: SupportsGrid,
		Intrinsic:          cacheTestIntrinsicA,
		Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
		Stretch:            StretchPolicy{Width: StretchAlways},
		Baseline:           BaselineNone,
	}) {
		t.Fatalf("expected different supported placements to be unequal")
	}
}

func TestLayoutRoleMeasureCachesIdenticalCalls(t *testing.T) {
	calls := 0
	role := &LayoutRole{
		Parent: GroupParentContract{
			Kind:     GroupLayoutGrid,
			Policy:   cacheTestGroupPolicy{variant: 1},
			Overflow: OverflowClip,
			Clipping: GroupClipBounds,
			Children: cacheTestGroupSource{variant: 2},
		},
		Child: GroupChildContract{
			SupportedPlacement: SupportsGrid | SupportsLinear,
			Intrinsic:          cacheTestIntrinsicA,
			Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
			Stretch:            StretchPolicy{Width: StretchAlways},
			Baseline:           BaselineNone,
		},
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult {
			calls++
			return MeasureResult{Size: gfx.Size{W: float32(10 * calls), H: float32(20 * calls)}}
		},
	}

	constraints := Constraints{
		MinSize: gfx.Size{W: 5, H: 6},
		MaxSize: gfx.Size{W: 7, H: 8},
	}
	got1 := role.Measure(MeasureContext{}, constraints)
	got2 := role.Measure(MeasureContext{}, constraints)
	if calls != 1 {
		t.Fatalf("expected OnMeasure to run once, got %d calls", calls)
	}
	if got1 != got2 {
		t.Fatalf("expected cached measure result to be reused: %#v vs %#v", got1, got2)
	}
	if role.MeasuredSize != got1.Size {
		t.Fatalf("cached size = %#v, want %#v", role.MeasuredSize, got1.Size)
	}
}

func TestLayoutRoleArrangeCachesIdenticalCalls(t *testing.T) {
	calls := 0
	role := &LayoutRole{
		Parent: GroupParentContract{
			Kind:     GroupLayoutGrid,
			Policy:   cacheTestGroupPolicy{variant: 1},
			Overflow: OverflowClip,
			Clipping: GroupClipBounds,
			Children: cacheTestGroupSource{variant: 2},
		},
		Child: GroupChildContract{
			SupportedPlacement: SupportsGrid | SupportsLinear,
			Intrinsic:          cacheTestIntrinsicA,
			Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
			Stretch:            StretchPolicy{Width: StretchAlways},
			Baseline:           BaselineNone,
		},
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult {
			return MeasureResult{Size: gfx.Size{W: 10, H: 20}}
		},
		OnArrange: func(ctx ArrangeContext, bounds gfx.Rect) {
			calls++
		},
	}

	role.Measure(MeasureContext{}, Constraints{
		MinSize: gfx.Size{W: 5, H: 6},
		MaxSize: gfx.Size{W: 7, H: 8},
	})
	ctx := ArrangeContext{
		Placement: Placement{
			Mode: PlacementLinear,
			Linear: LinearPlacement{
				Order: 1,
			},
		},
	}
	bounds := gfx.RectFromXYWH(1, 2, 3, 4)
	role.Arrange(ctx, bounds)
	role.Arrange(ctx, bounds)
	if calls != 1 {
		t.Fatalf("expected OnArrange to run once, got %d calls", calls)
	}
	if role.ArrangedBounds != bounds {
		t.Fatalf("arranged bounds = %#v, want %#v", role.ArrangedBounds, bounds)
	}
}

func TestLayoutRoleMeasureRecomputesOnChangedConstraints(t *testing.T) {
	calls := 0
	role := &LayoutRole{
		Parent: GroupParentContract{
			Kind:     GroupLayoutGrid,
			Policy:   cacheTestGroupPolicy{variant: 1},
			Overflow: OverflowClip,
			Clipping: GroupClipBounds,
			Children: cacheTestGroupSource{variant: 2},
		},
		Child: GroupChildContract{
			SupportedPlacement: SupportsGrid | SupportsLinear,
			Intrinsic:          cacheTestIntrinsicA,
			Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
			Stretch:            StretchPolicy{Width: StretchAlways},
			Baseline:           BaselineNone,
		},
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult {
			calls++
			return MeasureResult{Size: gfx.Size{W: float32(10 * calls), H: float32(20 * calls)}}
		},
	}

	c1 := Constraints{
		MinSize: gfx.Size{W: 5, H: 6},
		MaxSize: gfx.Size{W: 7, H: 8},
	}
	gotA := role.Measure(MeasureContext{}, c1)
	_ = role.Measure(MeasureContext{}, c1)
	if calls != 1 {
		t.Fatalf("identical input recomputed: calls=%d, want 1", calls)
	}

	c2 := Constraints{
		MinSize: gfx.Size{W: 10, H: 12},
		MaxSize: gfx.Size{W: 14, H: 16},
	}
	gotB := role.Measure(MeasureContext{}, c2)
	if calls != 2 {
		t.Fatalf("changed constraints did not recompute: calls=%d, want 2", calls)
	}
	if gotB == gotA {
		t.Fatal("returned stale cached result for new constraints")
	}
}

func TestLayoutRoleArrangeRecomputesOnChangedBounds(t *testing.T) {
	calls := 0
	role := &LayoutRole{
		Parent: GroupParentContract{
			Kind:     GroupLayoutGrid,
			Policy:   cacheTestGroupPolicy{variant: 1},
			Overflow: OverflowClip,
			Clipping: GroupClipBounds,
			Children: cacheTestGroupSource{variant: 2},
		},
		Child: GroupChildContract{
			SupportedPlacement: SupportsGrid | SupportsLinear,
			Intrinsic:          cacheTestIntrinsicA,
			Constraints:        ConstraintPolicy{BelowMinWidth: CompressionWrap},
			Stretch:            StretchPolicy{Width: StretchAlways},
			Baseline:           BaselineNone,
		},
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult {
			return MeasureResult{Size: gfx.Size{W: 10, H: 20}}
		},
		OnArrange: func(ctx ArrangeContext, bounds gfx.Rect) {
			calls++
		},
	}

	role.Measure(MeasureContext{}, Constraints{
		MinSize: gfx.Size{W: 5, H: 6},
		MaxSize: gfx.Size{W: 7, H: 8},
	})
	ctx := ArrangeContext{
		Placement: Placement{
			Mode: PlacementLinear,
			Linear: LinearPlacement{
				Order: 1,
			},
		},
	}
	b1 := gfx.RectFromXYWH(1, 2, 3, 4)
	role.Arrange(ctx, b1)
	role.Arrange(ctx, b1)
	if calls != 1 {
		t.Fatalf("identical input recomputed: calls=%d, want 1", calls)
	}
	if role.ArrangedBounds != b1 {
		t.Fatalf("arranged bounds = %#v, want %#v", role.ArrangedBounds, b1)
	}

	b2 := gfx.RectFromXYWH(5, 6, 7, 8)
	role.Arrange(ctx, b2)
	if calls != 2 {
		t.Fatalf("changed bounds did not recompute: calls=%d, want 2", calls)
	}
	if role.ArrangedBounds != b2 {
		t.Fatalf("arranged bounds = %#v, want %#v", role.ArrangedBounds, b2)
	}
}
