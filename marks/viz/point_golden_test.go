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

func TestPointGoldenEmpty(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	p.Radius = marks.Const[float32](5)
	p.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}

	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	proj := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	var cmdList []gfx.Command
	if proj != nil {
		cmdList = proj.Commands
	}
	surface := renderAxisGolden(t, cmdList, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_blank")
}

func TestPointGoldenSingleDatum(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	p.Radius = marks.Const[float32](5)
	p.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}

	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "point_single")
}

func TestPointGoldenDegenerateDomain(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{5, 5})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{5, 5})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	p.Radius = marks.Const[float32](5)
	p.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}

	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "point_degenerate_domain")
}

func TestPointGoldenScatter(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	p := NewPoint(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	p.Radius = marks.Const[float32](5)
	p.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 1}

	facet.Attach(p, facet.AttachContext{Runtime: vizRuntimeStub{}})
	p.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 1, y: 1})
	s.Insert(scatterItem{id: 2, x: 2, y: 3})
	s.Insert(scatterItem{id: 3, x: 4, y: 2})
	s.Insert(scatterItem{id: 4, x: 5, y: 7})
	s.Insert(scatterItem{id: 5, x: 7, y: 5})
	s.Insert(scatterItem{id: 6, x: 8, y: 9})
	s.Insert(scatterItem{id: 7, x: 9, y: 4})
	s.Insert(scatterItem{id: 8, x: 3, y: 8})
	s.Insert(scatterItem{id: 9, x: 6, y: 6})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	p.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := p.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "point_scatter")
}
