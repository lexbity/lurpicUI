package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

type vizRuntimeStub struct{}

func (vizRuntimeStub) Schedule(j job.AnyJob)                  {}
func (vizRuntimeStub) CancelJob(id job.JobID)                 {}
func (vizRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

func TestRule_horizontal_at_value(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(50.0), RuleHorizontal, rs)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	stroke, ok := cmds.Commands[0].(gfx.StrokePath)
	if !ok {
		t.Fatalf("expected StrokePath, got %T", cmds.Commands[0])
	}
	if len(stroke.Path.Segments) != 2 {
		t.Fatalf("expected 2 path segments, got %d", len(stroke.Path.Segments))
	}
	// Horizontal rule at Y = Map(50) = 100
	if stroke.Path.Segments[0].Pts[0].Y != 100 {
		t.Fatalf("expected Y=100, got %f", stroke.Path.Segments[0].Pts[0].Y)
	}
}

func TestRule_vertical_at_value(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(25.0), RuleVertical, rs)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	stroke, ok := cmds.Commands[0].(gfx.StrokePath)
	if !ok {
		t.Fatalf("expected StrokePath, got %T", cmds.Commands[0])
	}
	// Vertical rule at X = Map(25) = 75
	if stroke.Path.Segments[0].Pts[0].X != 75 {
		t.Fatalf("expected X=75, got %f", stroke.Path.Segments[0].Pts[0].X)
	}
}

func TestRule_at_lower_bound(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(0.0), RuleHorizontal, rs)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(10, 20, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	stroke := r.Projection.Project(facet.ProjectionContext{Bounds: bounds}).Commands[0].(gfx.StrokePath)
	if stroke.Path.Segments[0].Pts[0].Y != 20 {
		t.Fatalf("rule at lower bound: expected Y=20, got %f", stroke.Path.Segments[0].Pts[0].Y)
	}
}

func TestRule_at_upper_bound(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(100.0), RuleHorizontal, rs)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(10, 20, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	stroke := r.Projection.Project(facet.ProjectionContext{Bounds: bounds}).Commands[0].(gfx.StrokePath)
	if stroke.Path.Segments[0].Pts[0].Y != 220 {
		t.Fatalf("rule at upper bound: expected Y=220, got %f", stroke.Path.Segments[0].Pts[0].Y)
	}
}

func TestRule_out_of_bounds_clamps(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	r := NewRule(marks.Const(200.0), RuleVertical, rs)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(10, 20, 300, 200)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil && len(cmds.Commands) > 0 {
		// Value beyond domain still maps to a pixel (scale clamps or extends)
		stroke := cmds.Commands[0].(gfx.StrokePath)
		if stroke.Path.Segments[0].Pts[0].X < bounds.Min.X {
			t.Fatal("expected rule to stay at or beyond bounds")
		}
	}
}

func TestRule_nil_scale_returns_nil(t *testing.T) {
	r := NewRule(marks.Const(50.0), RuleHorizontal, nil)
	facet.Attach(r, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	r.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := r.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for nil scale")
	}
}
