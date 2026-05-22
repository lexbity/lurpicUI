package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
)

type testGroupSource struct{}

func (testGroupSource) Children() []GroupChild { return nil }

type testGroupPolicy struct{}

func (testGroupPolicy) Kind() GroupLayoutKind { return GroupLayoutGrid }
func (testGroupPolicy) MeasureGroup(ctx GroupMeasureContext, children []GroupChild) (GroupMeasureResult, error) {
	return GroupMeasureResult{}, nil
}
func (testGroupPolicy) ArrangeGroup(ctx GroupArrangeContext, children []GroupChild) ([]ArrangedGroupChild, error) {
	return nil, nil
}

func TestLayoutRole_measure_populates_parent_and_child_contexts(t *testing.T) {
	role := &LayoutRole{
		Parent: GroupParentContract{
			Kind:     GroupLayoutGrid,
			Policy:   testGroupPolicy{},
			Children: testGroupSource{},
		},
		Child: GroupChildContract{SupportedPlacement: SupportsGrid | SupportsLinear},
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult {
			if ctx.ParentGroup.Kind != GroupLayoutGrid {
				t.Fatalf("parent group kind = %v, want grid", ctx.ParentGroup.Kind)
			}
			if ctx.ChildGroup.SupportedPlacement != (SupportsGrid | SupportsLinear) {
				t.Fatalf("child placement = %v, want grid|linear", ctx.ChildGroup.SupportedPlacement)
			}
			return MeasureResult{Size: gfx.Size{W: 7, H: 9}}
		},
	}

	got := role.Measure(MeasureContext{}, Constraints{})
	if got.Size != (gfx.Size{W: 7, H: 9}) {
		t.Fatalf("Measure = %#v, want 7x9", got.Size)
	}
	if role.MeasuredResult.Size != (gfx.Size{W: 7, H: 9}) {
		t.Fatalf("MeasuredResult = %#v, want cached measure result", role.MeasuredResult)
	}
}

func TestLayoutRole_attach_rejects_invalid_parent_child_contracts(t *testing.T) {
	missingPolicy := &LayoutRole{
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult { return MeasureResult{} },
		Parent:    GroupParentContract{Kind: GroupLayoutGrid},
	}
	missingChild := &LayoutRole{
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult { return MeasureResult{} },
		Parent:    GroupParentContract{Kind: GroupLayoutGrid, Policy: testGroupPolicy{}, Children: testGroupSource{}},
	}

	f := &Facet{state: StateCreated}
	f.roles = []Role{missingPolicy}
	mustPanic(t, func() { Attach(f, AttachContext{}) })

	f = &Facet{state: StateCreated}
	f.roles = []Role{missingChild}
	mustPanic(t, func() { Attach(f, AttachContext{}) })
}

func TestLayoutRole_arrange_rejects_unsupported_placement(t *testing.T) {
	role := &LayoutRole{
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult { return MeasureResult{} },
		OnArrange: func(ctx ArrangeContext, bounds gfx.Rect) {
			t.Fatal("unexpected arrange callback for unsupported placement")
		},
		Child: GroupChildContract{SupportedPlacement: SupportsGrid | SupportsAnchor},
	}

	mustPanic(t, func() {
		role.Arrange(ArrangeContext{
			Placement: Placement{Mode: PlacementLinear},
		}, gfx.RectFromXYWH(0, 0, 10, 10))
	})
}

func TestLayoutRole_arrange_accepts_radial_placement(t *testing.T) {
	called := false
	role := &LayoutRole{
		OnMeasure: func(ctx MeasureContext, c Constraints) MeasureResult { return MeasureResult{} },
		OnArrange: func(ctx ArrangeContext, bounds gfx.Rect) {
			called = true
		},
		Child: GroupChildContract{SupportedPlacement: SupportsRadial},
	}

	role.Arrange(ArrangeContext{
		Placement: Placement{Mode: PlacementRadial, Radial: RadialPlacement{Angle: 0}},
	}, gfx.RectFromXYWH(0, 0, 10, 10))
	if !called {
		t.Fatal("expected arrange callback to run for radial placement")
	}
}
