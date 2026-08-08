package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// vizRowTime is the chart's x accessor: the row timestamp as fractional unix
// seconds (sub-second precision so a sub-second cadence slides the live tail).
func vizRowTime(r dataset.Row) float64 { return float64(r.Time.UnixNano()) / 1e9 }

// vizRowValue is the chart's y accessor.
func vizRowValue(r dataset.Row) float64 { return r.Value }

// vizRowRegion is the bar chart's categorical accessor.
func vizRowRegion(r dataset.Row) string { return r.Region }

// vizRowID identifies a row by its stamped counter (monotonic, stable across
// edits — see dataset.Row.ID).
func vizRowID(r dataset.Row) store.ItemID { return store.ItemID(r.ID) }

// chartMargins is the canvas geometry around the plot.
type chartMargins struct {
	left, top, right, bottom float32
}

// bridgeDerived mirrors the reactive package's internal bridge: it keeps a
// ValueStore in sync with a Derived's recomputes so a Derived domain can feed
// a scale constructor that consumes ValueStores. Consumers must Get() the
// Derived for its OnChange to fire (lazy recompute, F-derived-independence).
func bridgeDerived(d *store.Derived[[2]float64]) *store.ValueStore[[2]float64] {
	vs := store.NewValueStore(d.Get())
	d.OnChange.Subscribe(func(c signal.Change[[2]float64]) {
		vs.Set(c.New)
	})
	return vs
}

// ChartType selects the active series mark.
type ChartType uint8

const (
	ChartLine ChartType = iota
	ChartArea
	ChartPoint
	ChartBar
)

// String returns the canonical chart-type name (also the radio_group option
// value).
func (t ChartType) String() string {
	switch t {
	case ChartArea:
		return "area"
	case ChartPoint:
		return "point"
	case ChartBar:
		return "bar"
	default:
		return "line"
	}
}

// chartTypeFromString parses a chart-type name.
func chartTypeFromString(s string) ChartType {
	switch s {
	case "area":
		return ChartArea
	case "point":
		return ChartPoint
	case "bar":
		return ChartBar
	default:
		return ChartLine
	}
}

// ChartConfig wires a ChartCanvas to caller-owned stores.
type ChartConfig struct {
	Fonts *text.FontRegistry
	Theme theme.ResolvedContext

	Rows    *store.CollectionStore[dataset.Row]
	XDomain *store.ValueStore[[2]float64]
	XRange  *store.ValueStore[[2]float64]
	YDomain *store.Derived[[2]float64]
	YRange  *store.ValueStore[[2]float64]

	// Paused, when set, is flipped true on a pan/zoom gesture (the live-tail
	// pause); nil for a plain canvas.
	Paused *store.ValueStore[bool]

	ChartType   *store.ValueStore[string]
	SeriesColor *store.ValueStore[gfx.Color]
	Opacity     *store.ValueStore[float64]
	ShowGrid    *store.ValueStore[bool]
	RuleValue   *store.ValueStore[float64]

	// Brush stores (linked with the spreadsheet). When Hover is set, the
	// canvas hit-tests the active series on pointer move and publishes the
	// hovered row/region; when Selection is set, a pointer press on a point
	// selects it.
	Hover       *store.ValueStore[*store.ItemID]
	HoverRegion *store.ValueStore[string]
	Selection   *store.ValueStore[store.ItemID]
}

// ChartCanvas is the gallery's chart canvas (promoted from the P3 viz probe):
// the time x-scale, a Derived-driven y-scale, a swappable series mark
// (line/area/point/bar), a reference rule, and data-domain pan/zoom. It hosts
// all four series and visibility-gates the active one per ChartType.
type ChartCanvas struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	hit      facet.HitRole
	input    facet.InputRole
	viewport facet.ViewportRole

	rows    *store.CollectionStore[dataset.Row]
	xDomain *store.ValueStore[[2]float64]
	xRange  *store.ValueStore[[2]float64]
	yDomain *store.Derived[[2]float64]
	yRange  *store.ValueStore[[2]float64]
	paused  *store.ValueStore[bool]

	xScale *reactive.ReactiveScale
	yScale *reactive.ReactiveScale
	zoom   *reactive.ZoomController

	chartType   *store.ValueStore[string]
	seriesColor *store.ValueStore[gfx.Color]
	opacity     *store.ValueStore[float64]
	showGrid    *store.ValueStore[bool]
	ruleValue   *store.ValueStore[float64]
	effective   *store.ValueStore[gfx.Color]
	hover       *store.ValueStore[*store.ItemID]
	hoverRegion *store.ValueStore[string]
	selection   *store.ValueStore[store.ItemID]

	line  *viz.Line[dataset.Row]
	area  *viz.Area[dataset.Row]
	point *viz.Point[dataset.Row]
	bar   *viz.Bar[dataset.Row]
	xAxis *viz.Axis
	yAxis *viz.Axis
	rule  *viz.Rule

	margins chartMargins
	plot    gfx.Rect

	canvasColor    gfx.Color
	borderColor    gfx.Color
	gridColor      gfx.Color
	selectionColor gfx.Color

	rt        facet.RuntimeServices
	dragStart gfx.Point
	dragging  bool
	panning   bool
	cleanup   func()
}

// dragThreshold is the pointer-move distance that turns a press into a pan
// gesture. A press without enough movement is a selection click and must not
// pause the live tail (FR-window: only pan/zoom gestures set Paused).
const dragThreshold = 4

// NewChartCanvas builds a chart canvas over the caller's stores. Missing
// optional stores default to sensible values (line series, theme color, full
// opacity, grid off, rule at 0).
func NewChartCanvas(cfg ChartConfig) *ChartCanvas {
	c := &ChartCanvas{
		rows:           cfg.Rows,
		xDomain:        cfg.XDomain,
		xRange:         cfg.XRange,
		yDomain:        cfg.YDomain,
		yRange:         cfg.YRange,
		paused:         cfg.Paused,
		chartType:      cfg.ChartType,
		seriesColor:    cfg.SeriesColor,
		opacity:        cfg.Opacity,
		showGrid:       cfg.ShowGrid,
		ruleValue:      cfg.RuleValue,
		hover:          cfg.Hover,
		hoverRegion:    cfg.HoverRegion,
		selection:      cfg.Selection,
		margins:        chartMargins{top: 8, right: 8},
		canvasColor:    cfg.Theme.Color(theme.ColorSurface),
		borderColor:    cfg.Theme.Color(theme.ColorBorder),
		gridColor:      cfg.Theme.Color(theme.ColorBorder),
		selectionColor: gfx.ColorFromRGBA8(220, 60, 120, 220),
	}
	if c.chartType == nil {
		c.chartType = store.NewValueStore(ChartLine.String())
	}
	if c.seriesColor == nil {
		c.seriesColor = store.NewValueStore(gfx.Color{})
	}
	if c.opacity == nil {
		c.opacity = store.NewValueStore(1.0)
	}
	if c.showGrid == nil {
		c.showGrid = store.NewValueStore(false)
	}
	if c.ruleValue == nil {
		c.ruleValue = store.NewValueStore(0.0)
	}
	c.effective = store.NewValueStore(c.effectiveColorValue())

	c.xScale = reactive.NewTimeReactive(c.xDomain, c.xRange)
	c.yScale = reactive.NewLinearReactive(bridgeDerived(c.yDomain), c.yRange)
	c.zoom = reactive.NewZoomController(c.xDomain)

	c.line = viz.NewLine(c.rows, vizRowTime, vizRowValue, c.xScale, c.yScale)
	c.area = viz.NewArea(c.rows, vizRowTime, vizRowValue, c.xScale, c.yScale)
	c.point = viz.NewPoint(c.rows, vizRowTime, vizRowValue, c.xScale, c.yScale)
	c.bar = viz.NewBar(c.rows, vizRowRegion, vizRowValue, c.yScale)
	colorBinding := marks.FromStore(c.effective, facet.DirtyProjection)
	c.line.Color = colorBinding
	c.area.Color = colorBinding
	c.point.Color = colorBinding
	c.bar.Color = colorBinding

	c.xAxis = viz.NewAxis(c.xScale, marks.Const(viz.AxisBottom), cfg.Fonts)
	c.yAxis = viz.NewAxis(c.yScale, marks.Const(viz.AxisLeft), cfg.Fonts)
	c.rule = viz.NewRule(marks.FromStore(c.ruleValue, facet.DirtyProjection), viz.RuleHorizontal, c.yScale)

	c.Facet = facet.NewFacet()
	c.AddChild(c.line.Base())
	c.AddChild(c.area.Base())
	c.AddChild(c.point.Base())
	c.AddChild(c.bar.Base())
	c.AddChild(c.xAxis.Base())
	c.AddChild(c.yAxis.Base())
	c.AddChild(c.rule.Base())

	c.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke chart canvas host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			return c.measure(ctx, constraints)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			c.arrange(ctx, bounds)
		},
	}
	c.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(c.canvasColor)})
			list.Commands = append(list.Commands, c.gridCommands()...)
			list.Commands = append(list.Commands, c.selectionHighlightCommands()...)
			if !c.plot.IsEmpty() {
				list.Add(gfx.StrokeRect{Rect: c.plot, Stroke: gfx.StrokeStyle{Width: 1}, Brush: gfx.SolidBrush(c.borderColor)})
			}
		},
	}
	c.hit = facet.HitRole{
		OnHitTest: func(pt gfx.Point) facet.HitResult {
			if !c.plot.IsEmpty() && c.plot.Contains(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorCrosshair}
			}
			return facet.HitResult{}
		},
	}
	c.input = facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool { return c.onPointer(e) },
		OnScroll:  func(e facet.ScrollEvent) bool { return c.onScroll(e) },
	}
	c.viewport.Transform = gfx.Identity()

	c.AddRole(&c.layout)
	c.AddRole(&c.render)
	c.AddRole(&c.hit)
	c.AddRole(&c.input)
	c.AddRole(&c.viewport)
	return c
}

// Rows returns the canvas's collection.
func (c *ChartCanvas) Rows() *store.CollectionStore[dataset.Row] { return c.rows }

// XDomain returns the zoom-mutable x-domain store.
func (c *ChartCanvas) XDomain() *store.ValueStore[[2]float64] { return c.xDomain }

// YDomain returns the value-extent derived store.
func (c *ChartCanvas) YDomain() *store.Derived[[2]float64] { return c.yDomain }

// XScale returns the time x-scale.
func (c *ChartCanvas) XScale() *reactive.ReactiveScale { return c.xScale }

// YScale returns the linear y-scale.
func (c *ChartCanvas) YScale() *reactive.ReactiveScale { return c.yScale }

// ZoomController returns the x-domain pan/zoom controller.
func (c *ChartCanvas) ZoomController() *reactive.ZoomController { return c.zoom }

// Line returns the line series mark.
func (c *ChartCanvas) Line() *viz.Line[dataset.Row] { return c.line }

// Area returns the area series mark.
func (c *ChartCanvas) Area() *viz.Area[dataset.Row] { return c.area }

// Point returns the point series mark.
func (c *ChartCanvas) Point() *viz.Point[dataset.Row] { return c.point }

// Bar returns the bar series mark.
func (c *ChartCanvas) Bar() *viz.Bar[dataset.Row] { return c.bar }

// Rule returns the reference rule mark.
func (c *ChartCanvas) Rule() *viz.Rule { return c.rule }

// RuleValue returns the reference rule's value store.
func (c *ChartCanvas) RuleValue() *store.ValueStore[float64] { return c.ruleValue }

// XAxis returns the bottom time axis.
func (c *ChartCanvas) XAxis() *viz.Axis { return c.xAxis }

// YAxis returns the left value axis.
func (c *ChartCanvas) YAxis() *viz.Axis { return c.yAxis }

// PlotRect returns the plot area in window space (valid after arrange).
func (c *ChartCanvas) PlotRect() gfx.Rect { return c.plot }

// ChartTypeStore returns the series-selection store.
func (c *ChartCanvas) ChartTypeStore() *store.ValueStore[string] { return c.chartType }

// SelectionColor returns the chart-side selection highlight color.
func (c *ChartCanvas) SelectionColor() gfx.Color { return c.selectionColor }

// SeriesColorStore returns the series color store.
func (c *ChartCanvas) SeriesColorStore() *store.ValueStore[gfx.Color] { return c.seriesColor }

// OpacityStore returns the series opacity store.
func (c *ChartCanvas) OpacityStore() *store.ValueStore[float64] { return c.opacity }

// ShowGridStore returns the grid-overlay store.
func (c *ChartCanvas) ShowGridStore() *store.ValueStore[bool] { return c.showGrid }

// ResetDomain sets the x-domain (used by jump-to-live) and, when a Paused
// store is wired, clears the pause so the live tail resumes.
func (c *ChartCanvas) ResetDomain(domain [2]float64) {
	c.xDomain.Set(domain)
	if c.paused != nil {
		c.paused.Set(false)
	}
}

func (c *ChartCanvas) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	for _, child := range []facet.FacetImpl{c.xAxis, c.yAxis} {
		if role := child.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: constraints.MaxSize})
		}
	}
	c.margins.left = c.yAxis.Base().LayoutRole().MeasuredSize.W
	c.margins.bottom = c.xAxis.Base().LayoutRole().MeasuredSize.H
	return facet.MeasureResult{Size: constraints.Constrain(constraints.MaxSize)}
}

func (c *ChartCanvas) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	plot := gfx.RectFromXYWH(
		bounds.Min.X+c.margins.left,
		bounds.Min.Y+c.margins.top,
		bounds.Width()-c.margins.left-c.margins.right,
		bounds.Height()-c.margins.top-c.margins.bottom,
	)
	if plot.Width() < 1 || plot.Height() < 1 {
		return
	}
	c.plot = plot
	c.xRange.Set([2]float64{0, float64(plot.Width())})
	c.yRange.Set([2]float64{float64(plot.Height()), 0})
	c.yDomain.Get() // force the derived recompute so the bridged y-scale stays live

	active := chartTypeFromString(c.chartType.Get())
	for _, s := range []struct {
		t ChartType
		m facet.FacetImpl
	}{{ChartLine, c.line}, {ChartArea, c.area}, {ChartPoint, c.point}, {ChartBar, c.bar}} {
		if s.t == active {
			s.m.Base().LayoutRole().Arrange(ctx, plot)
		} else {
			s.m.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		}
	}
	// The bar's band x-scale owns the axis, so the time x-axis is hidden for
	// the bar chart type.
	xAxisBounds := gfx.Rect{}
	if active != ChartBar {
		xAxisBounds = gfx.RectFromXYWH(plot.Min.X, plot.Max.Y, plot.Width(), c.margins.bottom)
	}
	c.xAxis.Base().LayoutRole().Arrange(ctx, xAxisBounds)
	c.yAxis.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, plot.Min.Y, c.margins.left, plot.Height()))
	c.rule.Base().LayoutRole().Arrange(ctx, plot)
}

func (c *ChartCanvas) gridCommands() []gfx.Command {
	if !c.showGrid.Get() || c.plot.IsEmpty() || c.yScale == nil {
		return nil
	}
	s := c.yScale.Get()
	ticker, ok := s.(scale.Ticker)
	if !ok {
		return nil
	}
	cmds := make([]gfx.Command, 0, 5)
	for _, t := range ticker.Ticks(5) {
		y := c.plot.Min.Y + float32(s.Map(t.Value))
		cmds = append(cmds, gfx.StrokePath{
			Path: gfx.Path{Segments: []gfx.PathSegment{
				{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: c.plot.Min.X, Y: y}}},
				{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: c.plot.Max.X, Y: y}}},
			}},
			Stroke: gfx.StrokeStyle{Width: 1},
			Brush:  gfx.SolidBrush(c.gridColor),
		})
	}
	return cmds
}

// selectionHighlightCommands renders the chart-side highlight for the selected
// row (FR-brush: a table row click or a chart point press publishes Selection;
// the chart reacts by drawing the marker). For the temporal series it draws a
// ring around the selected data point; for the bar chart it outlines the
// selected row's region band.
func (c *ChartCanvas) selectionHighlightCommands() []gfx.Command {
	if c.selection == nil || c.plot.IsEmpty() {
		return nil
	}
	id := c.selection.Get()
	if id == 0 {
		return nil
	}
	if chartTypeFromString(c.chartType.Get()) == ChartBar {
		row, ok := c.rowByID(id)
		if !ok {
			return nil
		}
		band := c.bar.BandRect(row.Region)
		if band.IsEmpty() {
			return nil
		}
		return []gfx.Command{gfx.StrokeRect{
			Rect:   band,
			Stroke: gfx.StrokeStyle{Width: 2},
			Brush:  gfx.SolidBrush(c.selectionColor),
		}}
	}
	row, ok := c.rowByID(id)
	if !ok {
		return nil
	}
	px := c.plot.Min.X + float32(c.xScale.Get().Map(vizRowTime(row)))
	py := c.plot.Min.Y + float32(c.yScale.Get().Map(vizRowValue(row)))
	ring := gfx.RectFromXYWH(px-6, py-6, 12, 12)
	return []gfx.Command{
		gfx.FillRect{Rect: ring, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(220, 60, 120, 40))},
		gfx.StrokePath{Path: gfx.CirclePath(gfx.Point{X: px, Y: py}, 6), Stroke: gfx.StrokeStyle{Width: 2}, Brush: gfx.SolidBrush(c.selectionColor)},
	}
}

// rowByID returns the row with the given collection id.
func rowByID(rows *store.CollectionStore[dataset.Row], id store.ItemID) (dataset.Row, bool) {
	for _, r := range rows.All() {
		if rows.Identify(r) == id {
			return r, true
		}
	}
	return dataset.Row{}, false
}

func (c *ChartCanvas) rowByID(id store.ItemID) (dataset.Row, bool) {
	return rowByID(c.rows, id)
}

func (c *ChartCanvas) effectiveColorValue() gfx.Color {
	base := c.seriesColor.Get()
	if base == (gfx.Color{}) {
		return gfx.Color{}
	}
	base.A = float32(c.opacity.Get())
	return base
}

func (c *ChartCanvas) onPointer(e facet.PointerEvent) bool {
	switch e.Kind {
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			c.dragging = true
			c.panning = false
			c.dragStart = e.Position
			// A press alone is a selection candidate, not a pan: the live tail
			// is NOT paused here so a selectAt click never freezes the feed
			// (FR-window: only an actual pan/zoom gesture sets Paused).
			if c.selection != nil {
				c.selectAt(e.Position)
			}
		}
	case platform.PointerMove:
		if c.dragging {
			if !c.panning {
				// A drag only becomes a pan once the pointer moves beyond the
				// threshold, so a jittery selection click does not pause the
				// feed either.
				dx := e.Position.X - c.dragStart.X
				dy := e.Position.Y - c.dragStart.Y
				if dx*dx+dy*dy < dragThreshold*dragThreshold {
					return true
				}
				c.panning = true
				c.pauseLive()
			}
			c.panPixels(e.Position.X - c.dragStart.X)
			c.dragStart = e.Position
		} else if c.hover != nil {
			c.brushAt(e.Position)
		}
	case platform.PointerRelease:
		c.dragging = false
		c.panning = false
	}
	return true
}

// pauseLive sets Paused when a pan/zoom gesture starts, so the next feed tick's
// AnchorLiveWindow does not overwrite the gesture's domain change (FR-window).
// It is idempotent: Paused is only written on the false→true transition so a
// long drag does not re-emit the store signal every move.
func (c *ChartCanvas) pauseLive() {
	if c.paused != nil && !c.paused.Get() {
		c.paused.Set(true)
	}
}

// brushAt publishes the hovered row/region from the active series at the given
// canvas-local point (the linked-brushing feed from chart → grid).
func (c *ChartCanvas) brushAt(pt gfx.Point) {
	switch chartTypeFromString(c.chartType.Get()) {
	case ChartPoint:
		c.brushPoint(pt)
	case ChartBar:
		c.brushBar(pt)
	default:
		c.setHover(nil, "")
	}
}

// selectAt publishes the selected row when a point is pressed (chart → grid).
func (c *ChartCanvas) selectAt(pt gfx.Point) {
	if chartTypeFromString(c.chartType.Get()) != ChartPoint {
		return
	}
	if id, ok := c.nearestPoint(pt); ok {
		c.selection.Set(id)
	}
}

// brushPoint finds the nearest point within a hit radius and publishes it.
func (c *ChartCanvas) brushPoint(pt gfx.Point) {
	if id, ok := c.nearestPoint(pt); ok {
		c.setHover(&id, "")
	} else {
		c.setHover(nil, "")
	}
}

// nearestPoint returns the id of the point nearest the cursor within a hit
// radius, in the canvas's plot space.
func (c *ChartCanvas) nearestPoint(pt gfx.Point) (store.ItemID, bool) {
	xs := c.xScale.Get()
	ys := c.yScale.Get()
	rows := c.rows.All()
	const hitRadius float32 = 8
	best := -1
	bestDist := float32(hitRadius*hitRadius + 1)
	for i := range rows {
		px := c.plot.Min.X + float32(xs.Map(vizRowTime(rows[i])))
		py := c.plot.Min.Y + float32(ys.Map(vizRowValue(rows[i])))
		dx, dy := pt.X-px, pt.Y-py
		d := dx*dx + dy*dy
		if d <= hitRadius*hitRadius && d < bestDist {
			bestDist = d
			best = i
		}
	}
	if best < 0 {
		return 0, false
	}
	return c.rows.Identify(rows[best]), true
}

// brushBar publishes a region-group hover for the bar band under the cursor.
// The canvas receives pointer positions in its own space (the series marks
// project with absolute coordinates), so the band is hit-tested directly.
func (c *ChartCanvas) brushBar(pt gfx.Point) {
	if region, ok := c.bar.HitMember(pt); ok {
		id := regionHoverSentinel
		c.setHover(&id, region)
	} else {
		c.setHover(nil, "")
	}
}

func (c *ChartCanvas) setHover(id *store.ItemID, region string) {
	if c.hover != nil {
		c.hover.Set(id)
	}
	if c.hoverRegion != nil {
		c.hoverRegion.Set(region)
	}
}

func (c *ChartCanvas) onScroll(e facet.ScrollEvent) bool {
	factor := 1.0 - float64(e.DeltaY)*0.01
	if factor <= 0 {
		factor = 0.1
	}
	// FR-window: a wheel zoom is a zoom gesture; pause the live tail so the
	// next tick's AnchorLiveWindow does not overwrite the zoom.
	c.pauseLive()
	c.zoomAt(float64(e.Position.X-c.plot.Min.X), factor)
	return true
}

// panPixels pans the x-domain by a pixel drag. Dragging right moves the view
// window toward earlier data (the data appears to move right).
func (c *ChartCanvas) panPixels(dx float32) {
	if c.plot.Width() <= 0 {
		return
	}
	lo, hi := c.xDomain.Get()[0], c.xDomain.Get()[1]
	dataPerPixel := (hi - lo) / float64(c.plot.Width())
	c.zoom.Pan(-float64(dx) * dataPerPixel)
}

// zoomAt zooms the x-domain around a focal pixel (factor > 1 zooms in).
func (c *ChartCanvas) zoomAt(focalPx, factor float64) {
	if c.plot.Width() <= 0 {
		return
	}
	if inv, ok := c.xScale.Get().(scale.InvertibleScale); ok {
		c.zoom.Zoom(inv.Invert(focalPx), factor)
	}
}

func (c *ChartCanvas) OnAttach(ctx facet.AttachContext) {
	c.rt = ctx.Runtime
	cleanups := make([]func(), 0, 2)
	cleanups = append(cleanups, subscribeEffectiveColor(c))
	// The active series' arrangement and the axis visibility change with the
	// chart type, so a switch must re-layout the canvas (F-charttype-
	// subscription).
	idType := c.chartType.OnChange.Subscribe(func(signal.Change[string]) {
		invalidateLayout(c, c.rt, "chart_canvas.chartType")
		c.Invalidate(facet.DirtyProjection | facet.DirtyHit)
	})
	cleanups = append(cleanups, func() {
		c.chartType.OnChange.Unsubscribe(idType)
	})
	// The selection highlight is chart-side state: a selection (from a chart
	// press or a grid row click) must re-project the canvas so the marker
	// appears (FR-brush).
	if c.selection != nil {
		idSel := c.selection.OnChange.Subscribe(func(signal.Change[store.ItemID]) {
			c.Invalidate(facet.DirtyProjection)
		})
		cleanups = append(cleanups, func() {
			c.selection.OnChange.Unsubscribe(idSel)
		})
	}
	c.cleanup = func() {
		for _, fn := range cleanups {
			if fn != nil {
				fn()
			}
		}
	}
}

func (c *ChartCanvas) OnDetach() {
	if c.cleanup != nil {
		c.cleanup()
		c.cleanup = nil
	}
}

func (c *ChartCanvas) Base() *facet.Facet { c.BindImpl(c); return &c.Facet }
func (c *ChartCanvas) OnActivate()        {}
func (c *ChartCanvas) OnDeactivate()      {}

// subscribeEffectiveColor keeps the marks' color binding in sync with the
// SeriesColor × Opacity stores.
func subscribeEffectiveColor(c *ChartCanvas) func() {
	idColor := c.seriesColor.OnChange.Subscribe(func(signal.Change[gfx.Color]) { c.syncEffective() })
	idOpacity := c.opacity.OnChange.Subscribe(func(signal.Change[float64]) { c.syncEffective() })
	return func() {
		c.seriesColor.OnChange.Unsubscribe(idColor)
		c.opacity.OnChange.Unsubscribe(idOpacity)
	}
}

func (c *ChartCanvas) syncEffective() {
	c.effective.Set(c.effectiveColorValue())
}
