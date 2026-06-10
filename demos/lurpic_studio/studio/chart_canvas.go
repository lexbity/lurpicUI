package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

type ChartCanvas struct {
	facet.Facet
	layout       facet.LayoutRole
	appState     *state.AppState

	xDomain      *store.ValueStore[[2]float64]
	yDomainStore *store.ValueStore[[2]float64]
	xRange       *store.ValueStore[[2]float64]
	yRange       *store.ValueStore[[2]float64]

	xScale       *reactive.ReactiveScale
	yScale       *reactive.ReactiveScale

	xAxis        *viz.Axis
	yAxis        *viz.Axis
	rule         *viz.Rule
	lineMark     *viz.Line[dataset.Row]
	areaMark     *viz.Area[dataset.Row]
	pointMark    *viz.Point[dataset.Row]
	barMark      *viz.Bar[state.BarBucket]

	chartData    *store.CollectionStore[dataset.Row]
	barData      *store.CollectionStore[state.BarBucket]

	visibleSub   signal.SubscriptionID
	bucketSub    signal.SubscriptionID
	colorSub     signal.SubscriptionID
	opacitySub   signal.SubscriptionID
	chartTypeSub signal.SubscriptionID
}

func chartRowID(r dataset.Row) store.ItemID {
	h := uint64(r.Date.Unix())
	for i := 0; i < len(r.Region); i++ {
		h = h*31 + uint64(r.Region[i])
	}
	return store.ItemID(h)
}

func xAccessor(r dataset.Row) float64   { return float64(r.Date.Unix()) }
func yAccessor(r dataset.Row) float64   { return r.Revenue }
func barCat(b state.BarBucket) string   { return b.Region }
func barVal(b state.BarBucket) float64  { return b.Value }

func NewChartCanvas(appState *state.AppState, fonts *text.FontRegistry) *ChartCanvas {
	c := &ChartCanvas{appState: appState}
	c.Facet = facet.NewFacet()

	c.chartData = store.NewCollectionStore[dataset.Row](chartRowID)
	c.chartData.Replace(appState.VisibleRows.Get())

	c.barData = store.NewCollectionStore[state.BarBucket](chartBucketID)
	c.barData.Replace(appState.BarBuckets.Get())

	xDomainDerived := reactive.DomainFromCollection(c.chartData, xAccessor)
	c.xDomain = bridgeFloatDomain(xDomainDerived)
	c.yDomainStore = bridgeFloatDomain(appState.YDomain)

	c.xRange = store.NewValueStore[[2]float64]([2]float64{0, 500})
	c.yRange = store.NewValueStore[[2]float64]([2]float64{400, 0})

	c.xScale = reactive.NewTimeReactive(c.xDomain, c.xRange)
	c.yScale = reactive.NewLinearReactive(c.yDomainStore, c.yRange)

	c.xAxis = viz.NewAxis(c.xScale, marks.Const(viz.AxisBottom), fonts)
	c.yAxis = viz.NewAxis(c.yScale, marks.Const(viz.AxisLeft), fonts)

	c.rule = viz.NewRule(
		marks.FromStore(appState.Threshold, facet.DirtyProjection),
		viz.RuleHorizontal, c.yScale,
	)

	c.lineMark = viz.NewLine(c.chartData, xAccessor, yAccessor, c.xScale, c.yScale)
	c.areaMark = viz.NewArea(c.chartData, xAccessor, yAccessor, c.xScale, c.yScale)
	c.pointMark = viz.NewPoint(c.chartData, xAccessor, yAccessor, c.xScale, c.yScale)
	c.barMark = viz.NewBar(c.barData, barCat, barVal, c.yScale)

	c.Facet.AddChild(c.xAxis.Base())
	c.Facet.AddChild(c.yAxis.Base())
	c.Facet.AddChild(c.rule.Base())
	c.Facet.AddChild(c.lineMark.Base())
	c.Facet.AddChild(c.areaMark.Base())
	c.Facet.AddChild(c.pointMark.Base())
	c.Facet.AddChild(c.barMark.Base())

	c.visibleSub = appState.VisibleRows.OnChange.Subscribe(func(cg signal.Change[[]dataset.Row]) {
		c.chartData.Replace(cg.New)
	})
	c.bucketSub = appState.BarBuckets.OnChange.Subscribe(func(cg signal.Change[[]state.BarBucket]) {
		c.barData.Replace(cg.New)
	})
	c.colorSub = appState.SeriesColor.OnChange.Subscribe(func(cg signal.Change[gfx.Color]) {
		col := cg.New
		c.lineMark.Color = col
		op := float32(appState.Opacity.Get())
		c.areaMark.Color = gfx.Color{R: col.R, G: col.G, B: col.B, A: op}
		c.pointMark.Color = col
		c.barMark.Color = col
		c.Invalidate(facet.DirtyProjection)
	})
	c.opacitySub = appState.Opacity.OnChange.Subscribe(func(cg signal.Change[float64]) {
		col := appState.SeriesColor.Get()
		c.areaMark.Color = gfx.Color{R: col.R, G: col.G, B: col.B, A: float32(cg.New)}
		c.Invalidate(facet.DirtyProjection)
	})
	c.chartTypeSub = appState.ChartType.OnChange.Subscribe(func(cg signal.Change[state.ChartType]) {
		c.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})

	c.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			width := constraints.MaxSize.W
			height := constraints.MaxSize.H
			if width <= 0 { width = 400 }
			if height <= 0 { height = 300 }

			marginL := float32(50)
			marginR := float32(10)
			marginT := float32(10)
			marginB := float32(30)
			plotW := width - marginL - marginR
			plotH := height - marginT - marginB
			if plotW < 50 { plotW = 50 }
			if plotH < 50 { plotH = 50 }

			c.xRange.Set([2]float64{0, float64(plotW)})
			c.yRange.Set([2]float64{0, float64(plotH)})

			c.xAxis.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: marginB}})
			c.yAxis.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: marginL, H: plotH}})
			c.rule.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: plotH}})
			c.lineMark.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: plotH}})
			c.areaMark.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: plotH}})
			c.pointMark.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: plotH}})
			c.barMark.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: plotW, H: plotH}})

			return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			marginL := float32(50)
			marginR := float32(10)
			marginT := float32(10)
			marginB := float32(30)
			plotW := bounds.Width() - marginL - marginR
			plotH := bounds.Height() - marginT - marginB
			if plotW < 50 { plotW = 50 }
			if plotH < 50 { plotH = 50 }

			plotRect := gfx.RectFromXYWH(bounds.Min.X+marginL, bounds.Min.Y+marginT, plotW, plotH)

			c.xAxis.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(plotRect.Min.X, plotRect.Max.Y, plotW, marginB))
			c.yAxis.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(bounds.Min.X, plotRect.Min.Y, marginL, plotH))
			c.xAxis.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(plotRect.Min.X, plotRect.Max.Y, plotW, marginB)
			c.yAxis.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, plotRect.Min.Y, marginL, plotH)

			c.rule.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, plotRect)
			c.rule.Base().LayoutRole().ArrangedBounds = plotRect

			ct := appState.ChartType.Get()
			switch ct {
			case state.ChartLine:
				c.lineMark.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, plotRect)
				c.lineMark.Base().LayoutRole().ArrangedBounds = plotRect
				c.areaMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.pointMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.barMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
			case state.ChartArea:
				c.areaMark.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, plotRect)
				c.areaMark.Base().LayoutRole().ArrangedBounds = plotRect
				c.lineMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.pointMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.barMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
			case state.ChartPoint:
				c.pointMark.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, plotRect)
				c.pointMark.Base().LayoutRole().ArrangedBounds = plotRect
				c.lineMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.areaMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.barMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
			case state.ChartBar:
				c.barMark.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, plotRect)
				c.barMark.Base().LayoutRole().ArrangedBounds = plotRect
				c.lineMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.areaMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
				c.pointMark.Base().LayoutRole().ArrangedBounds = gfx.Rect{}
			}
		},
	}
	c.AddRole(&c.layout)
	return c
}

func (c *ChartCanvas) Base() *facet.Facet { c.Facet.BindImpl(c); return &c.Facet }
func (c *ChartCanvas) OnAttach(ctx facet.AttachContext) {}
func (c *ChartCanvas) OnDetach() {
	c.appState.VisibleRows.OnChange.Unsubscribe(c.visibleSub)
	c.appState.BarBuckets.OnChange.Unsubscribe(c.bucketSub)
	c.appState.SeriesColor.OnChange.Unsubscribe(c.colorSub)
	c.appState.Opacity.OnChange.Unsubscribe(c.opacitySub)
	c.appState.ChartType.OnChange.Unsubscribe(c.chartTypeSub)
}
func (c *ChartCanvas) OnActivate()   {}
func (c *ChartCanvas) OnDeactivate() {}

func chartBucketID(b state.BarBucket) store.ItemID {
	var h uint64
	for i := 0; i < len(b.Region); i++ {
		h = h*31 + uint64(b.Region[i])
	}
	return store.ItemID(h)
}

func bridgeFloatDomain(d *store.Derived[[2]float64]) *store.ValueStore[[2]float64] {
	vs := store.NewValueStore(d.Get())
	d.OnChange.Subscribe(func(c signal.Change[[2]float64]) {
		vs.Set(c.New)
	})
	return vs
}

func (c *ChartCanvas) XScale() *reactive.ReactiveScale   { return c.xScale }
func (c *ChartCanvas) YScale() *reactive.ReactiveScale   { return c.yScale }
func (c *ChartCanvas) XAxis() *viz.Axis                   { return c.xAxis }
func (c *ChartCanvas) YAxis() *viz.Axis                   { return c.yAxis }
func (c *ChartCanvas) Rule() *viz.Rule                    { return c.rule }
func (c *ChartCanvas) Line() *viz.Line[dataset.Row]       { return c.lineMark }
func (c *ChartCanvas) Area() *viz.Area[dataset.Row]       { return c.areaMark }
func (c *ChartCanvas) Point() *viz.Point[dataset.Row]     { return c.pointMark }
func (c *ChartCanvas) Bar() *viz.Bar[state.BarBucket]      { return c.barMark }

func (c *ChartCanvas) ScalePoint(row dataset.Row) (float64, float64) {
	xs := c.xScale.Get()
	ys := c.yScale.Get()
	return xs.Map(xAccessor(row)), ys.Map(yAccessor(row))
}

func (c *ChartCanvas) PlotBounds() gfx.Rect {
	marginL := float32(50)
	marginR := float32(10)
	marginT := float32(10)
	marginB := float32(30)
	bounds := c.layout.ArrangedBounds
	plotW := bounds.Width() - marginL - marginR
	plotH := bounds.Height() - marginT - marginB
	if plotW < 50 { plotW = 50 }
	if plotH < 50 { plotH = 50 }
	return gfx.RectFromXYWH(bounds.Min.X+marginL, bounds.Min.Y+marginT, plotW, plotH)
}


