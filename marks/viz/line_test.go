package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestLine_produces_polyline(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 200})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 200})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 0, y: 0})
	s.Insert(scatterItem{id: 2, x: 5, y: 5})
	s.Insert(scatterItem{id: 3, x: 10, y: 10})

	bounds := gfx.RectFromXYWH(10, 10, 200, 200)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	pl, ok := cmds.Commands[0].(gfx.DrawPolyline)
	if !ok {
		t.Fatalf("expected DrawPolyline, got %T", cmds.Commands[0])
	}
	if len(pl.Points) != 3 {
		t.Fatalf("expected 3 polyline points, got %d", len(pl.Points))
	}
}

func TestLine_positions_ordered(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 200})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 200})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 0, y: 0})
	s.Insert(scatterItem{id: 2, x: 5, y: 5})
	s.Insert(scatterItem{id: 3, x: 10, y: 10})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	pl := cmds.Commands[0].(gfx.DrawPolyline)

	if pl.Points[0].X != 0 || pl.Points[0].Y != 0 {
		t.Fatalf("point[0] = (%f,%f), want (0,0)", pl.Points[0].X, pl.Points[0].Y)
	}
	if pl.Points[1].X != 100 || pl.Points[1].Y != 100 {
		t.Fatalf("point[1] = (%f,%f), want (100,100)", pl.Points[1].X, pl.Points[1].Y)
	}
	if pl.Points[2].X != 200 || pl.Points[2].Y != 200 {
		t.Fatalf("point[2] = (%f,%f), want (200,200)", pl.Points[2].X, pl.Points[2].Y)
	}
}

func TestLine_empty_store(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 1}),
			store.NewValueStore([2]float64{0, 100}),
		),
	)
	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for empty store")
	}
}
