package studio

import (
	"hash/fnv"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	marksdata "codeburg.org/lexbit/lurpicui/marks/data"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

var cnX = marks.Channel{Name: "x"}
var cnY = marks.Channel{Name: "y"}

type chartCanvas struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	appState *state.AppState

	xScale *reactive.ReactiveScale
	yScale *reactive.ReactiveScale

	xDomain *store.ValueStore[[2]float64]
	xRange  *store.ValueStore[[2]float64]
	yRange  *store.ValueStore[[2]float64]

	chartData *store.CollectionStore[dataset.Row]
	barData   *store.CollectionStore[state.BarBucket]

	xAxis    *viz.Axis
	yAxis    *viz.Axis
	ruleLine *viz.Rule
	lineMark *viz.Line[dataset.Row]
	areaMark *viz.Area[dataset.Row]
	scatter  *dataScatter
	barMark  *viz.Bar[state.BarBucket]
}

func newChartCanvas(as *state.AppState) *chartCanvas {
	cc := &chartCanvas{
		appState: as,
		layout:   facet.LayoutRole{}, // Explicit zero-initialization
		render:   facet.RenderRole{}, // Explicit zero-initialization
	}

	chartRows := store.NewCollectionStore(identifyChartRow)
	as.FilteredRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		chartRows.Replace(c.New)
	})
	rows := as.FilteredRows.Get()
	if len(rows) > 0 {
		chartRows.Replace(rows)
	}
	cc.chartData = chartRows

	barColl := store.NewCollectionStore(identifyBarBucket)
	as.BarBuckets.OnChange.Subscribe(func(c signal.Change[[]state.BarBucket]) {
		barColl.Replace(c.New)
	})
	bb := as.BarBuckets.Get()
	if len(bb) > 0 {
		barColl.Replace(bb)
	}
	cc.barData = barColl

	cc.xDomain = store.NewValueStore([2]float64{0, 1})
	cc.xRange = store.NewValueStore([2]float64{0, 100})
	cc.yRange = store.NewValueStore([2]float64{100, 0})

	cc.xScale = reactive.NewTimeReactive(cc.xDomain, cc.xRange)
	yDomainVs := store.NewValueStore([2]float64{0, 100})
	cc.yScale = reactive.NewLinearReactive(yDomainVs, cc.yRange)
	as.YDomain.OnChange.Subscribe(func(c signal.Change[[2]float64]) {
		yDomainVs.Set(c.New)
	})

	cc.xAxis = viz.NewAxis(cc.xScale, marks.Const(viz.AxisBottom), nil)
	cc.yAxis = viz.NewAxis(cc.yScale, marks.Const(viz.AxisLeft), nil)

	cc.ruleLine = viz.NewRule(marks.FromStore(as.Threshold, 0), viz.RuleHorizontal, cc.yScale)
	cc.ruleLine.Color = gfx.Color{R: 0.8, G: 0.2, B: 0.2, A: 0.6}
	cc.ruleLine.StrokeWidth = 1

	accent := gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}
	accentFill := gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 0.2}

	cc.lineMark = viz.NewLine(cc.chartData,
		func(r dataset.Row) float64 { return float64(r.Date.UnixMilli()) },
		func(r dataset.Row) float64 { return r.Revenue },
		cc.xScale, cc.yScale,
	)
	cc.lineMark.Color = accent
	cc.lineMark.StrokeWidth = marks.Const[float32](2)

	cc.areaMark = viz.NewArea(cc.chartData,
		func(r dataset.Row) float64 { return float64(r.Date.UnixMilli()) },
		func(r dataset.Row) float64 { return r.Revenue },
		cc.xScale, cc.yScale,
	)
	cc.areaMark.Color = accentFill

	cc.scatter = newDataScatter(cc.chartData, cc.xScale, cc.yScale, accent, 4)

	cc.barMark = viz.NewBar(cc.barData,
		func(b state.BarBucket) string { return b.Region },
		func(b state.BarBucket) float64 { return b.Revenue },
		cc.yScale,
	)
	cc.barMark.Color = accent

	cc.Facet.AddChild(cc.yAxis.Base())
	cc.Facet.AddChild(cc.xAxis.Base())
	cc.Facet.AddChild(cc.ruleLine.Base())
	cc.Facet.AddChild(cc.lineMark.Base())
	cc.Facet.AddChild(cc.areaMark.Base())
	cc.Facet.AddChild(cc.scatter.Base())
	cc.Facet.AddChild(cc.barMark.Base())

	cc.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult { //lurpiclint:ignore LL001 -- canvas measures to max so marks resolve intrinsics
		return facet.MeasureResult{Size: c.MaxSize}
	}
	cc.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) { //lurpiclint:ignore LL001 -- canvas positions axes/plot/marks at computed chart coordinates
		cc.layout.ArrangedBounds = bounds

		margin := float32(8)
		yAxisW := float32(50)
		xAxisH := float32(30)

		plotX := bounds.Min.X + margin + yAxisW
		plotY := bounds.Min.Y + margin
		plotW := bounds.Width() - margin*2 - yAxisW
		plotH := bounds.Height() - margin*2 - xAxisH
		if plotW < 10 {
			plotW = 10
		}
		if plotH < 10 {
			plotH = 10
		}

		cc.yRange.Set([2]float64{float64(plotY + plotH), float64(plotY)})
		cc.xRange.Set([2]float64{float64(plotX), float64(plotX + plotW)})

		cc.yAxis.Base().LayoutRole().OnArrange(ctx, gfx.Rect{
			Min: gfx.Point{X: bounds.Min.X + margin, Y: plotY},
			Max: gfx.Point{X: bounds.Min.X + margin + yAxisW, Y: plotY + plotH},
		})
		cc.xAxis.Base().LayoutRole().OnArrange(ctx, gfx.Rect{
			Min: gfx.Point{X: plotX, Y: plotY + plotH},
			Max: gfx.Point{X: plotX + plotW, Y: bounds.Max.Y - margin},
		})

		plotBounds := gfx.Rect{
			Min: gfx.Point{X: plotX, Y: plotY},
			Max: gfx.Point{X: plotX + plotW, Y: plotY + plotH},
		}

		cc.lineMark.Base().LayoutRole().OnArrange(ctx, plotBounds)
		cc.areaMark.Base().LayoutRole().OnArrange(ctx, plotBounds)
		cc.scatter.Base().LayoutRole().OnArrange(ctx, plotBounds)
		cc.barMark.Base().LayoutRole().OnArrange(ctx, plotBounds)
		cc.ruleLine.Base().LayoutRole().OnArrange(ctx, plotBounds)
	}
	cc.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(gfx.Color{R: 0.1, G: 0.11, B: 0.14, A: 1})})
	}

	cc.Facet.AddRole(&cc.layout)
	cc.Facet.AddRole(&cc.render)

	updateXDomain(as, cc.xDomain)

	as.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		updateXDomain(as, cc.xDomain)
	})
	as.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		updateXDomain(as, cc.xDomain)
	})

	return cc
}

// dataScatter wraps marks/data.DataMark to create one child facet per data row.
// Each child renders a circle at the data point's mapped position.
// This exercises marks/data: DataMark, CollectionBinder, factory, and
// MapPosition for position encoding.
type dataScatter struct {
	facet.Facet
	marks.Core
	dm     *marksdata.DataMark[dataset.Row]
	color  gfx.Color
	radius float32
}

func newDataScatter(
	store *store.CollectionStore[dataset.Row],
	xScale, yScale *reactive.ReactiveScale,
	color gfx.Color,
	radius float32,
) *dataScatter {
	s := &dataScatter{color: color, radius: radius}
	s.Facet = facet.NewFacet()

	scales := map[marks.Channel]*reactive.ReactiveScale{
		cnX: xScale,
		cnY: yScale,
	}

	s.dm = marksdata.NewDataMark(
		&s.Facet,
		store,
		func(row dataset.Row) facet.FacetImpl {
			return newScatterDot(
				row,
				float64(row.Date.UnixMilli()),
				row.Revenue,
				color,
				radius,
			)
		},
		scales,
		nil,
	)

	s.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.Layout.ArrangedBounds = bounds
		for _, child := range s.dm.Binder.Children() {
			dot := child.(*scatterDot)
			px := s.dm.MapPosition(cnX, dot.xVal)
			py := s.dm.MapPosition(cnY, dot.yVal)
			cb := gfx.Rect{
				Min: gfx.Point{X: float32(px), Y: float32(py)},
				Max: gfx.Point{X: float32(px) + 1, Y: float32(py) + 1},
			}
			if lr := child.Base().LayoutRole(); lr != nil {
				lr.ArrangedBounds = cb
			}
		}
	}
	s.Core.RegisterRoles()
	s.Facet.AddRole(&s.Layout)
	return s
}

func (s *dataScatter) Base() *facet.Facet               { return &s.Facet }
func (s *dataScatter) OnAttach(ctx facet.AttachContext) { s.Core.OnAttach(); s.dm.Binder.OnAttach(ctx) }
func (s *dataScatter) OnDetach()                        { s.Core.OnDetach(); s.dm.Binder.OnDetach() }
func (s *dataScatter) OnActivate()                      { s.Core.OnActivate() }
func (s *dataScatter) OnDeactivate()                    { s.Core.OnDeactivate() }
func (s *dataScatter) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "datascatter"}
}
func (s *dataScatter) AccessibleName() string    { return "Data scatter" }
func (s *dataScatter) AccessibilityRole() string { return "img" }
func (s *dataScatter) ChildMarks() []marks.Mark {
	children := s.dm.Binder.Children()
	out := make([]marks.Mark, len(children))
	for i, c := range children {
		out[i] = c.(marks.Mark)
	}
	return out
}

var _ marks.Composite = (*dataScatter)(nil)

type scatterDot struct {
	marks.Core
	row   dataset.Row
	xVal  float64
	yVal  float64
	color gfx.Color
	r     float32
}

func newScatterDot(row dataset.Row, xVal, yVal float64, color gfx.Color, r float32) *scatterDot {
	d := &scatterDot{row: row, xVal: xVal, yVal: yVal, color: color, r: r}
	d.Facet = facet.NewFacet()
	d.Core.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return drawDot(d)
	}
	d.Core.RegisterRoles()
	return d
}

func (d *scatterDot) Base() *facet.Facet               { return &d.Facet }
func (d *scatterDot) OnAttach(ctx facet.AttachContext) { d.Core.OnAttach() }
func (d *scatterDot) OnDetach()                        { d.Core.OnDetach() }
func (d *scatterDot) OnActivate()                      { d.Core.OnActivate() }
func (d *scatterDot) OnDeactivate()                    { d.Core.OnDeactivate() }

func drawDot(d *scatterDot) []gfx.Command {
	bounds := d.Layout.ArrangedBounds
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return nil
	}
	center := gfx.Point{
		X: bounds.Min.X + bounds.Width()*0.5,
		Y: bounds.Min.Y + bounds.Height()*0.5,
	}
	path := gfx.CirclePath(center, d.r)
	return []gfx.Command{
		gfx.FillPath{Path: path, Brush: gfx.SolidBrush(d.color)},
	}
}
func (d *scatterDot) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "viz", TypeName: "scatterdot"}
}
func (d *scatterDot) Encoding() marks.Encoding { return d }

func (d *scatterDot) Channels() []marks.Channel {
	return []marks.Channel{cnX, cnY}
}

func (d *scatterDot) BuildCommands(ctx facet.ProjectionContext) []gfx.Command {
	bounds := d.Layout.ArrangedBounds
	if bounds.Width() <= 0 || bounds.Height() <= 0 {
		return nil
	}
	center := gfx.Point{
		X: bounds.Min.X + bounds.Width()*0.5,
		Y: bounds.Min.Y + bounds.Height()*0.5,
	}
	path := gfx.CirclePath(center, d.r)
	return []gfx.Command{
		gfx.FillPath{Path: path, Brush: gfx.SolidBrush(d.color)},
	}
}

var _ facet.FacetImpl = (*scatterDot)(nil)

func updateXDomain(as *state.AppState, xd *store.ValueStore[[2]float64]) {
	rows := as.VisibleRows.Get()
	if len(rows) == 0 {
		xd.Set([2]float64{0, 1})
		return
	}
	min := float64(rows[0].Date.UnixMilli())
	max := min
	for _, r := range rows {
		t := float64(r.Date.UnixMilli())
		if t < min {
			min = t
		}
		if t > max {
			max = t
		}
	}
	if max == min {
		max = min + float64(time.Hour.Milliseconds()*24)
	}
	xd.Set([2]float64{min, max})
}

func identifyChartRow(r dataset.Row) store.ItemID {
	h := fnv.New64a()
	h.Write([]byte(r.Date.Format("2006-01-02")))
	h.Write([]byte(r.Region))
	return store.ItemID(h.Sum64())
}

func identifyBarBucket(b state.BarBucket) store.ItemID {
	h := fnv.New64a()
	h.Write([]byte(b.Region))
	return store.ItemID(h.Sum64())
}
