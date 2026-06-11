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

// --- Integration test: composed chart ---

type chartFixture struct {
	barStore *store.CollectionStore[barItem]
	ptStore  *store.CollectionStore[scatterItem]
	barMark  *Bar[barItem]
	ptMark   *Point[scatterItem]
	axisMark *Axis
	yDomain  *store.ValueStore[[2]float64]
	yRange   *store.ValueStore[[2]float64]
	bounds   gfx.Rect
}

func newChartFixture(t *testing.T) *chartFixture {
	f := &chartFixture{}

	f.barStore = store.NewCollectionStore(barID)
	f.ptStore = store.NewCollectionStore(scatterID)

	f.yDomain = store.NewValueStore([2]float64{0, 100})
	f.yRange = store.NewValueStore([2]float64{0, 300})
	yScale := reactive.NewLinearReactive(f.yDomain, f.yRange)

	f.barMark = NewBar(f.barStore,
		func(i barItem) string { return i.cat },
		func(i barItem) float64 { return i.val },
		yScale,
	)
	f.barMark.Padding = marks.Const[float32](0.2)
	f.barMark.Color = gfx.Color{R: 0.2, G: 0.5, B: 0.8, A: 1}

	f.ptMark = NewPoint(f.ptStore,
		func(i scatterItem) float64 { return i.x },
		func(i scatterItem) float64 { return i.y },
		reactive.NewLinearReactive(
			store.NewValueStore([2]float64{0, 10}),
			store.NewValueStore([2]float64{0, 300}),
		),
		yScale,
	)
	f.ptMark.Radius = marks.Const[float32](4)
	f.ptMark.Color = gfx.Color{R: 0.9, G: 0.2, B: 0.2, A: 1}

	fonts := (axisGoldenRuntime{}).FontRegistry()
	f.axisMark = NewAxis(yScale, marks.Const(AxisBottom), fonts)
	f.axisMark.TickCount = marks.Const(6)

	f.bounds = gfx.RectFromXYWH(30, 20, 240, 280)

	return f
}

func (f *chartFixture) attach() {
	rt := vizRuntimeStub{}
	f.barMark.OnAttach(facet.AttachContext{Runtime: rt})
	f.ptMark.OnAttach(facet.AttachContext{Runtime: rt})
	f.axisMark.OnAttach(facet.AttachContext{Runtime: rt})
}

func (f *chartFixture) arrange() {
	f.barMark.Layout.Arrange(facet.ArrangeContext{}, f.bounds)
	f.ptMark.Layout.Arrange(facet.ArrangeContext{}, f.bounds)
	f.axisMark.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(30, 310, 240, 30))
}

func (f *chartFixture) collectCommands() []gfx.Command {
	var cmds []gfx.Command
	ctx := facet.ProjectionContext{Bounds: f.bounds, ContentScale: 1}

	if list := f.barMark.Projection.Project(ctx); list != nil {
		cmds = append(cmds, list.Commands...)
	}
	if list := f.ptMark.Projection.Project(ctx); list != nil {
		cmds = append(cmds, list.Commands...)
	}
	if list := f.axisMark.Projection.Project(facet.ProjectionContext{
		Bounds:       gfx.RectFromXYWH(30, 310, 240, 30),
		ContentScale: 1,
	}); list != nil {
		cmds = append(cmds, list.Commands...)
	}
	return cmds
}

func TestChartGoldenComposed(t *testing.T) {
	f := newChartFixture(t)

	f.barStore.Insert(barItem{id: 1, cat: "Q1", val: 25})
	f.barStore.Insert(barItem{id: 2, cat: "Q2", val: 60})
	f.barStore.Insert(barItem{id: 3, cat: "Q3", val: 45})
	f.barStore.Insert(barItem{id: 4, cat: "Q4", val: 90})

	f.ptStore.Insert(scatterItem{id: 1, x: 1, y: 10})
	f.ptStore.Insert(scatterItem{id: 2, x: 3, y: 40})
	f.ptStore.Insert(scatterItem{id: 3, x: 5, y: 65})
	f.ptStore.Insert(scatterItem{id: 4, x: 7, y: 85})
	f.ptStore.Insert(scatterItem{id: 5, x: 9, y: 50})
	f.ptStore.Insert(scatterItem{id: 6, x: 11, y: 95})

	f.attach()
	f.arrange()

	cmds := f.collectCommands()
	if len(cmds) == 0 {
		t.Fatal("expected chart commands")
	}

	surface := renderAxisGolden(t, cmds, f.bounds, 340, 360)
	testkit.AssertGolden(t, surface, "chart_composed")
}

func TestChartLiveDataUpdate(t *testing.T) {
	f := newChartFixture(t)

	f.barStore.Insert(barItem{id: 1, cat: "A", val: 50})
	f.attach()
	f.arrange()

	cmdsBefore := f.collectCommands()
	if len(cmdsBefore) == 0 {
		t.Fatal("expected commands before update")
	}

	f.barStore.Insert(barItem{id: 2, cat: "B", val: 80})
	f.arrange()

	cmdsAfter := f.collectCommands()
	if len(cmdsAfter) <= len(cmdsBefore) {
		t.Fatal("expected more commands after data insert")
	}
}

func TestChartSemanticZoom(t *testing.T) {
	f := newChartFixture(t)

	f.barStore.Insert(barItem{id: 1, cat: "A", val: 50})
	f.attach()
	f.arrange()
	f.collectCommands() // primes bar rects

	// Hit at the first bar's center
	r := f.barMark.barRects[0]
	center := gfx.Point{X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
	_, ok := f.barMark.HitMember(center)
	if !ok {
		t.Fatal("expected hit on bar before zoom")
	}

	// Zoom in by narrowing domain — bar should be taller
	f.yDomain.Set([2]float64{25, 75})
	f.arrange()
	f.collectCommands()

	r2 := f.barMark.barRects[0]
	if r2.Height() <= r.Height() {
		t.Fatal("expected bar to be taller after zoom in")
	}
}

func TestChartHitTestBar(t *testing.T) {
	f := newChartFixture(t)

	f.barStore.Insert(barItem{id: 1, cat: "Q1", val: 60})
	f.barStore.Insert(barItem{id: 2, cat: "Q2", val: 80})
	f.attach()
	f.arrange()
	f.collectCommands() // primes hit rects

	r := f.barMark.barRects[0]
	center := gfx.Point{X: (r.Min.X + r.Max.X) / 2, Y: (r.Min.Y + r.Max.Y) / 2}
	member, ok := f.barMark.HitMember(center)
	if !ok {
		t.Fatal("expected hit on Q1 bar at its center")
	}
	if member != "Q1" {
		t.Fatalf("hit member = %q, want %q", member, "Q1")
	}
}
