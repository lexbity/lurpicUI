package viz

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
)

type scatterItem struct {
	id store.ItemID
	x  float64
	y  float64
}

func scatterID(i scatterItem) store.ItemID { return i.id }

func TestPoint_produces_draw_points(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 200})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 200})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 1, y: 2})
	s.Insert(scatterItem{id: 2, x: 5, y: 5})
	s.Insert(scatterItem{id: 3, x: 9, y: 8})

	bounds := gfx.RectFromXYWH(10, 10, 200, 200)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	dp, ok := cmds.Commands[0].(gfx.DrawPoints)
	if !ok {
		t.Fatalf("expected DrawPoints, got %T", cmds.Commands[0])
	}
	if len(dp.Points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(dp.Points))
	}
}

func TestPoint_positions_through_scales(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 200})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 200})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 0, y: 0})
	s.Insert(scatterItem{id: 2, x: 10, y: 10})

	bounds := gfx.RectFromXYWH(0, 0, 200, 200)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	dp := p.Projection.Project(facet.ProjectionContext{Bounds: bounds}).Commands[0].(gfx.DrawPoints)

	// Item 1: x=0 → Map(0)=0, y=0 → Map(0)=0 (Y range is [0,200] not inverted)
	if dp.Points[0].X != 0 || dp.Points[0].Y != 0 {
		t.Fatalf("point[0] = (%f,%f), want (0,0)", dp.Points[0].X, dp.Points[0].Y)
	}
	// Item 2: x=10 → Map(10)=200, y=10 → Map(10)=200
	if dp.Points[1].X != 200 || dp.Points[1].Y != 200 {
		t.Fatalf("point[1] = (%f,%f), want (200,200)", dp.Points[1].X, dp.Points[1].Y)
	}
}

func TestPoint_empty_store_no_points(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xScale := reactive.NewLinearReactive(
		store.NewValueStore([2]float64{0, 1}),
		store.NewValueStore([2]float64{0, 100}),
	)
	yScale := reactive.NewLinearReactive(
		store.NewValueStore([2]float64{0, 1}),
		store.NewValueStore([2]float64{0, 100}),
	)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds != nil {
		t.Fatal("expected nil commands for empty store")
	}
}
