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

func TestLineGoldenEmpty(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	l.StrokeWidth = marks.Const[float32](2)
	l.Color = gfx.Color{R: 0.1, G: 0.3, B: 0.7, A: 1}

	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	proj := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	var cmdList []gfx.Command
	if proj != nil {
		cmdList = proj.Commands
	}
	surface := renderAxisGolden(t, cmdList, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_empty")
}

func TestLineGoldenSingleDatum(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	l.StrokeWidth = marks.Const[float32](2)
	l.Color = gfx.Color{R: 0.1, G: 0.3, B: 0.7, A: 1}

	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_single")
}

func TestLineGoldenDegenerateDomain(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{5, 5})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{5, 5})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	l.StrokeWidth = marks.Const[float32](2)
	l.Color = gfx.Color{R: 0.1, G: 0.3, B: 0.7, A: 1}

	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_degenerate_domain")
}

func TestLineGoldenBasic(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	l := NewLine(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	l.StrokeWidth = marks.Const[float32](2)
	l.Color = gfx.Color{R: 0.1, G: 0.3, B: 0.7, A: 1}

	facet.Attach(l, facet.AttachContext{Runtime: vizRuntimeStub{}})
	l.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 0, y: 1})
	s.Insert(scatterItem{id: 2, x: 2, y: 5})
	s.Insert(scatterItem{id: 3, x: 4, y: 3})
	s.Insert(scatterItem{id: 4, x: 6, y: 8})
	s.Insert(scatterItem{id: 5, x: 8, y: 4})
	s.Insert(scatterItem{id: 6, x: 10, y: 9})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	l.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := l.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "line_basic")
}

func TestAreaGoldenEmpty(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	a.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 0.3}

	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})
	a.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	proj := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	var cmdList []gfx.Command
	if proj != nil {
		cmdList = proj.Commands
	}
	surface := renderAxisGolden(t, cmdList, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "area_empty")
}

func TestAreaGoldenDegenerateDomain(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{5, 5})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{5, 5})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	a.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 0.3}

	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})
	a.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 5, y: 5})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "area_degenerate_domain")
}

func TestAreaGoldenBasic(t *testing.T) {
	s := store.NewCollectionStore(scatterID)
	xDom := store.NewValueStore([2]float64{0, 10})
	xRng := store.NewValueStore([2]float64{0, 300})
	yDom := store.NewValueStore([2]float64{0, 10})
	yRng := store.NewValueStore([2]float64{0, 300})
	xScale := reactive.NewLinearReactive(xDom, xRng)
	yScale := reactive.NewLinearReactive(yDom, yRng)

	a := NewArea(s,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		xScale, yScale,
	)
	a.Color = gfx.Color{R: 0.2, G: 0.4, B: 0.8, A: 0.3}

	facet.Attach(a, facet.AttachContext{Runtime: vizRuntimeStub{}})
	a.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(scatterItem{id: 1, x: 0, y: 1})
	s.Insert(scatterItem{id: 2, x: 2, y: 5})
	s.Insert(scatterItem{id: 3, x: 4, y: 3})
	s.Insert(scatterItem{id: 4, x: 6, y: 8})
	s.Insert(scatterItem{id: 5, x: 8, y: 4})
	s.Insert(scatterItem{id: 6, x: 10, y: 9})

	bounds := gfx.RectFromXYWH(20, 20, 300, 300)
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := a.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 340, 340)
	testkit.AssertGolden(t, surface, "area_basic")
}
