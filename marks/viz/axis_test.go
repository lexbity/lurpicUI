package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestAxis_computes_entries_from_ticks(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisBottom), nil)
	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 200, 30)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	if len(a.entries) == 0 {
		t.Fatal("expected tick entries")
	}
	if a.entries[0].Label == "" {
		t.Fatal("expected non-empty label")
	}
	if a.entries[0].Pixel < 0 {
		t.Fatal("expected non-negative pixel position")
	}
}

func TestAxis_bottom_ticks_are_stroked(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisBottom), nil)
	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 200, 30)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	stroke, ok := cmds.Commands[0].(gfx.StrokePath)
	if !ok {
		t.Fatalf("expected StrokePath, got %T", cmds.Commands[0])
	}
	if len(stroke.Path.Segments) != 2 {
		t.Fatalf("expected 2 path segments per tick, got %d", len(stroke.Path.Segments))
	}
}

func TestAxis_nil_scale_returns_nil(t *testing.T) {
	a := NewAxis(nil, marks.Const(AxisBottom), nil)
	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 200, 30)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for nil scale")
	}
}

func TestAxis_left_orientation_ticks(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisLeft), nil)
	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 40, 200)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands for left axis")
	}

	stroke, ok := cmds.Commands[0].(gfx.StrokePath)
	if !ok {
		t.Fatalf("expected StrokePath, got %T", cmds.Commands[0])
	}
	// Left axis ticks go from (maxX - tickLen) to maxX
	if stroke.Path.Segments[0].Pts[0].X >= bounds.Max.X {
		t.Fatal("expected left axis tick to start left of maxX")
	}
}

func TestAxis_all_orientations_emit_ticks_and_labels(t *testing.T) {
	fonts := (axisGoldenRuntime{}).FontRegistry()

	for _, orient := range []AxisOrientation{AxisBottom, AxisTop, AxisLeft, AxisRight} {
		domain := store.NewValueStore([2]float64{0, 100})
		rng := store.NewValueStore([2]float64{0, 200})
		rs := reactive.NewLinearReactive(domain, rng)

		a := NewAxis(rs, marks.Const(orient), fonts)
		facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

		var bounds gfx.Rect
		switch orient {
		case AxisBottom, AxisTop:
			bounds = gfx.RectFromXYWH(0, 0, 200, 30)
		case AxisLeft, AxisRight:
			bounds = gfx.RectFromXYWH(0, 0, 40, 200)
		}
		a.Layout.Arrange(facet.ArrangeContext{}, bounds)

		cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
		if cmds == nil || len(cmds.Commands) == 0 {
			t.Fatalf("orientation %v: expected projection commands", orient)
		}

		hasTicks := false
		hasLabels := false
		for _, c := range cmds.Commands {
			switch c.(type) {
			case gfx.StrokePath:
				hasTicks = true
			case gfx.DrawGlyphRun:
				hasLabels = true
			}
		}
		if !hasTicks {
			t.Errorf("orientation %v: no tick strokes", orient)
		}
		if !hasLabels {
			t.Errorf("orientation %v: no label glyph runs", orient)
		}
	}
}

func TestAxis_label_collision_skips_overlapping(t *testing.T) {
	fonts := (axisGoldenRuntime{}).FontRegistry()

	// A narrow axis with many ticks ensures labels overlap.
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 60})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisBottom), fonts)
	a.TickCount = marks.Const(20)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(0, 0, 60, 30)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil {
		t.Fatal("expected commands")
	}

	labelCount := 0
	for _, c := range cmds.Commands {
		if _, ok := c.(gfx.DrawGlyphRun); ok {
			labelCount++
		}
	}
	if labelCount >= 20 {
		t.Fatalf("expected collision to skip labels, got %d labels for 20 ticks", labelCount)
	}
	if labelCount == 0 {
		t.Fatal("expected at least one non-colliding label")
	}
}

// TestAxis_left_label_column_measured_from_widest_tick proves RX-1 FR-15: the
// left/right axis's label column is sized to the widest formatted tick label
// measured via text metrics, not a constant estimate — the constant clipped
// formatted tick text and caused the A-10 dash-clipping class.
func TestAxis_left_label_column_measured_from_widest_tick(t *testing.T) {
	fonts := (axisGoldenRuntime{}).FontRegistry()
	domain := store.NewValueStore([2]float64{0, 1000})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisLeft), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	a.Layout.Measure(facet.MeasureContext{ContentScale: 1}, facet.Constraints{MaxSize: gfx.Size{W: 120, H: 240}})
	size := a.Layout.MeasuredSize
	if size.W <= 0 {
		t.Fatal("left axis measured size has no width")
	}

	// The widest formatted tick label (e.g. "1000") measured with the same
	// style the axis uses at projection.
	s := a.Scale.Get()
	ticker, ok := s.(scale.Ticker)
	if !ok {
		t.Fatal("linear scale is not a Ticker")
	}
	shaper := text.NewShaper(fonts)
	style := text.TextStyle{Size: 11}
	widest := float32(0)
	for _, tk := range ticker.Ticks(a.TickCount.Get()) {
		if tk.Label == "" {
			continue
		}
		shaped := shaper.ShapeSimple(tk.Label, style)
		if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
			continue
		}
		if w := shaped.Lines[0].Runs[0].Bounds.Width(); w > widest {
			widest = w
		}
	}
	if widest <= 0 {
		t.Fatal("no tick labels shaped; the fixture is degenerate")
	}
	// The label column (the measured width minus the tick length and the
	// trailing padding) must fit the widest label.
	tickLen := float32(6)
	if got := size.W - tickLen - 6; got < widest {
		t.Fatalf("left axis label column %.1f < widest tick label %.1f (FR-15)", got, widest)
	}
}

func TestAxis_tick_count_respected(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	a := NewAxis(rs, marks.Const(AxisBottom), nil)
	a.TickCount = marks.Const(3)
	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 200, 30)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	if len(a.entries) == 0 {
		t.Fatal("expected entries with TickCount=3")
	}
}

func TestAxis_Contract_ScaleInvalidates(t *testing.T) {
	contracttest.AssertScaleInvalidates(t,
		func(scale *reactive.ReactiveScale) facet.FacetImpl {
			return NewAxis(scale, marks.Const(AxisBottom), nil)
		},
		func(domain *store.ValueStore[[2]float64]) {
			domain.Set([2]float64{0, 50})
		},
	)
}
