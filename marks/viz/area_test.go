package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestArea_produces_fill_path(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	s := store.NewCollectionStore(scope, scatterID)
	xDom := store.NewValueStore(scope, [2]float64{0, 10})
	xRng := store.NewValueStore(scope, [2]float64{0, 200})
	yDom := store.NewValueStore(scope, [2]float64{0, 10})
	yRng := store.NewValueStore(scope, [2]float64{0, 200})
	xScale := reactive.NewLinearReactive(scope, xDom, xRng)
	yScale := reactive.NewLinearReactive(scope, yDom, yRng)

	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(a, facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})
	a.OnAttach(facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})

	s.Insert(scatterItem{id: 1, x: 0, y: 5})
	s.Insert(scatterItem{id: 2, x: 5, y: 8})
	s.Insert(scatterItem{id: 3, x: 10, y: 2})

	bounds := gfx.RectFromXYWH(10, 10, 200, 200)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	fp, ok := cmds.Commands[0].(gfx.FillPath)
	if !ok {
		t.Fatalf("expected FillPath, got %T", cmds.Commands[0])
	}
	if len(fp.Path.Segments) < 3 {
		t.Fatalf("expected at least 3 path segments, got %d", len(fp.Path.Segments))
	}
}

func TestArea_path_forms_closed_polygon(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	s := store.NewCollectionStore(scope, scatterID)
	xDom := store.NewValueStore(scope, [2]float64{0, 10})
	xRng := store.NewValueStore(scope, [2]float64{0, 200})
	yDom := store.NewValueStore(scope, [2]float64{0, 10})
	yRng := store.NewValueStore(scope, [2]float64{0, 200})
	xScale := reactive.NewLinearReactive(scope, xDom, xRng)
	yScale := reactive.NewLinearReactive(scope, yDom, yRng)

	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(a, facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})
	a.OnAttach(facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})

	s.Insert(scatterItem{id: 1, x: 0, y: 5})
	s.Insert(scatterItem{id: 2, x: 10, y: 5})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	fp := a.Projection.Project(facet.ProjectionContext{Bounds: bounds}).Commands[0].(gfx.FillPath)
	segs := fp.Path.Segments

	// Path should be: MoveTo(0,100) → LineTo(200,100) → LineTo(200,200) → LineTo(0,200) → Close
	if len(segs) < 5 {
		t.Fatalf("expected 5+ segments, got %d", len(segs))
	}
	if segs[0].Verb != gfx.PathMoveTo {
		t.Fatal("first segment must be MoveTo")
	}
	if segs[1].Verb != gfx.PathLineTo {
		t.Fatal("second segment must be LineTo")
	}
	if segs[len(segs)-1].Verb != gfx.PathClose {
		t.Fatal("last segment must be Close")
	}
}

func TestArea_empty_store(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	s := store.NewCollectionStore(scope, scatterID)
	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		reactive.NewLinearReactive(scope, store.NewValueStore(scope, [2]float64{0, 1}),
			store.NewValueStore(scope, [2]float64{0, 100}),
		),
		reactive.NewLinearReactive(scope, store.NewValueStore(scope, [2]float64{0, 1}),
			store.NewValueStore(scope, [2]float64{0, 100}),
		),
	)
	facet.Attach(a, facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})
	a.OnAttach(facet.AttachContext{Runtime: newAxisGoldenRuntime(t)})

	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for empty store")
	}
}

func TestArea_Contract_ScaleInvalidates(t *testing.T) {
	scope := store.NewScope()
	t.Cleanup(scope.Close)
	contracttest.AssertScaleInvalidates(t,
		func(_ *store.Scope, scale *reactive.ReactiveScale) facet.FacetImpl {
			return NewArea(store.NewCollectionStore(scope, scatterID),
				func(i scatterItem) float64 { return i.x },
				func(i scatterItem) float64 { return i.y },
				scale, nil,
			)
		},
		func(domain *store.ValueStore[[2]float64]) {
			domain.Set([2]float64{0, 50})
		},
	)
}
