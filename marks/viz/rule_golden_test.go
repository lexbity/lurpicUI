package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRuleGoldenHorizontal(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(50.0), RuleHorizontal, rs)
	r.Color = gfx.Color{R: 0.8, G: 0.2, B: 0.2, A: 1}
	r.StrokeWidth = 2
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(10, 10, 280, 180)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 300, 200)
	testkit.AssertGolden(t, surface, "rule_horizontal")
}

func TestRuleGoldenVertical(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(30.0), RuleVertical, rs)
	r.Color = gfx.Color{R: 0.2, G: 0.6, B: 0.2, A: 1}
	r.StrokeWidth = 1.5
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(10, 10, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 320, 220)
	testkit.AssertGolden(t, surface, "rule_vertical")
}
