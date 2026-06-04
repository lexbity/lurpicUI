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

func TestBarGoldenEmpty(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	b.Padding = marks.Const[float32](0.2)
	b.Color = gfx.Color{R: 0.2, G: 0.5, B: 0.8, A: 1}

	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	bounds := gfx.RectFromXYWH(30, 20, 240, 280)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)

	proj := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	var cmdList []gfx.Command
	if proj != nil {
		cmdList = proj.Commands
	}
	surface := renderAxisGolden(t, cmdList, bounds, 300, 320)
	testkit.AssertGolden(t, surface, "bar_empty")
}

func TestBarGoldenSingleDatum(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	b.Padding = marks.Const[float32](0.2)
	b.Color = gfx.Color{R: 0.2, G: 0.5, B: 0.8, A: 1}

	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "Single", val: 50})

	bounds := gfx.RectFromXYWH(30, 20, 240, 280)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 300, 320)
	testkit.AssertGolden(t, surface, "bar_single")
}

func TestBarGoldenBasic(t *testing.T) {
	s := store.NewCollectionStore(barID)
	yDom := store.NewValueStore([2]float64{0, 100})
	yRng := store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(yDom, yRng)

	b := NewBar(s,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	b.Padding = marks.Const[float32](0.2)
	b.Color = gfx.Color{R: 0.2, G: 0.5, B: 0.8, A: 1}

	facet.Attach(b, facet.AttachContext{Runtime: vizRuntimeStub{}})
	b.OnAttach(facet.AttachContext{Runtime: vizRuntimeStub{}})

	s.Insert(barItem{id: 1, cat: "Q1", val: 25})
	s.Insert(barItem{id: 2, cat: "Q2", val: 60})
	s.Insert(barItem{id: 3, cat: "Q3", val: 45})
	s.Insert(barItem{id: 4, cat: "Q4", val: 90})

	bounds := gfx.RectFromXYWH(30, 20, 240, 280)
	b.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := b.Projection.Project(facet.ProjectionContext{Bounds: bounds})
	surface := renderAxisGolden(t, cmds.Commands, bounds, 300, 320)
	testkit.AssertGolden(t, surface, "bar_basic")
}
