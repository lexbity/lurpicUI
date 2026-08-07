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

// vizRowTime is the chart's x accessor: the row timestamp as unix seconds
// (the second-precision synthetic clock).
func vizRowTime(r dataset.Row) float64 { return float64(r.Time.Unix()) }

// vizRowValue is the chart's y accessor.
func vizRowValue(r dataset.Row) float64 { return r.Value }

// vizRowID identifies a row by its stamped counter (monotonic, stable across
// edits — see dataset.Row.ID).
func vizRowID(r dataset.Row) store.ItemID { return store.ItemID(r.ID) }

// chartMargins is the canvas geometry around the plot.
type chartMargins struct {
	left, top, right, bottom float32
}

// VizProbe is the isolated prove-viz-first chart: one line series with a
// bottom x-axis, a left y-axis, and a reference rule over real seed data,
// with data-domain pan/zoom. It is the first production consumer of the viz
// marks and the reactive scales together (F-unconsumed); Slice P5 promotes
// this wiring into the gallery.
type VizProbe struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	hit      facet.HitRole
	input    facet.InputRole
	viewport facet.ViewportRole

	rows *store.CollectionStore[dataset.Row]

	// xDomain is zoom-mutable (the ZoomController's domain); xRange is the
	// pixel range pushed on every arrange.
	xDomain *store.ValueStore[[2]float64]
	xRange  *store.ValueStore[[2]float64]
	xScale  *reactive.ReactiveScale

	// yDomain is a Derived of the value extent (auto-extent, never zoomed);
	// yRange is the inverted pixel range pushed on every arrange.
	yDomain *store.Derived[[2]float64]
	yRange  *store.ValueStore[[2]float64]
	yScale  *reactive.ReactiveScale

	zoom *reactive.ZoomController

	line  *viz.Line[dataset.Row]
	xAxis *viz.Axis
	yAxis *viz.Axis
	rule  *viz.Rule

	// ruleValue drives the reference rule (exposed for tests).
	ruleValue *store.ValueStore[float64]

	margins chartMargins
	plot    gfx.Rect

	canvasColor gfx.Color
	borderColor gfx.Color

	dragStart gfx.Point
	dragging  bool
}

// NewVizProbe builds the standalone chart over the given seed rows. fonts is
// used by the axes' label shaper; themeCtx supplies the canvas tokens.
func NewVizProbe(seed []dataset.Row, fonts *text.FontRegistry, themeCtx theme.ResolvedContext) *VizProbe {
	rows := store.NewCollectionStore(vizRowID)
	for i := range seed {
		seed[i].ID = uint64(i + 1)
		rows.Insert(seed[i])
	}

	p := &VizProbe{
		rows:        rows,
		xRange:      store.NewValueStore([2]float64{0, 1}),
		yRange:      store.NewValueStore([2]float64{1, 0}),
		ruleValue:   store.NewValueStore(0.0),
		margins:     chartMargins{top: 8, right: 8},
		canvasColor: themeCtx.Color(theme.ColorSurface),
		borderColor: themeCtx.Color(theme.ColorBorder),
	}

	// x-domain: the full seed extent. DomainFromCollection (never-consumed)
	// computes it; the value is seeded into the zoom-mutable ValueStore.
	xExtent := reactive.DomainFromCollection(rows, vizRowTime)
	p.xDomain = store.NewValueStore(xExtent.Get())
	// y-domain: a Derived of the value extent.
	p.yDomain = reactive.DomainFromCollection(rows, vizRowValue)

	// The scales are wired with explicit ValueStores (the spec's shape): the
	// y-domain Derived is bridged into a ValueStore. The FromDerived scale
	// constructors are not used for the range because they bridge derived
	// ranges lazily — a directly-set pixel range would go stale until the
	// wrapped derived is re-Get()'d (F-derived-range).
	p.xScale = reactive.NewTimeReactive(p.xDomain, p.xRange)
	p.yScale = reactive.NewLinearReactive(bridgeDerived(p.yDomain), p.yRange)
	p.zoom = reactive.NewZoomController(p.xDomain)

	p.ruleValue.Set(meanValue(seed))

	p.line = viz.NewLine(rows, vizRowTime, vizRowValue, p.xScale, p.yScale)
	p.xAxis = viz.NewAxis(p.xScale, marks.Const(viz.AxisBottom), fonts)
	p.yAxis = viz.NewAxis(p.yScale, marks.Const(viz.AxisLeft), fonts)
	p.rule = viz.NewRule(marks.FromStore(p.ruleValue, facet.DirtyProjection), viz.RuleHorizontal, p.yScale)

	p.Facet = facet.NewFacet()
	p.AddChild(p.line.Base())
	p.AddChild(p.xAxis.Base())
	p.AddChild(p.yAxis.Base())
	p.AddChild(p.rule.Base())

	p.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke chart canvas host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return p.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			p.arrange(ctx, bounds)
		},
	}
	p.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(p.canvasColor)})
			if !p.plot.IsEmpty() {
				list.Add(gfx.StrokeRect{Rect: p.plot, Stroke: gfx.StrokeStyle{Width: 1}, Brush: gfx.SolidBrush(p.borderColor)})
			}
		},
	}
	p.hit = facet.HitRole{
		OnHitTest: func(pt gfx.Point) facet.HitResult {
			if !p.plot.IsEmpty() && p.plot.Contains(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorCrosshair}
			}
			return facet.HitResult{}
		},
	}
	p.input = facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool { return p.onPointer(e) },
		OnScroll:  func(e facet.ScrollEvent) bool { return p.onScroll(e) },
	}
	// The view is data-domain (ZoomController mutates the x-domain); the
	// ViewportRole declares the canvas as a viewport for hit/input routing.
	p.viewport.Transform = gfx.Identity()

	p.AddRole(&p.layout)
	p.AddRole(&p.render)
	p.AddRole(&p.hit)
	p.AddRole(&p.input)
	p.AddRole(&p.viewport)
	return p
}

// Rows returns the probe's collection.
func (p *VizProbe) Rows() *store.CollectionStore[dataset.Row] { return p.rows }

// XDomain returns the zoom-mutable x-domain store.
func (p *VizProbe) XDomain() *store.ValueStore[[2]float64] { return p.xDomain }

// YDomain returns the value-extent derived store.
func (p *VizProbe) YDomain() *store.Derived[[2]float64] { return p.yDomain }

// XScale returns the time x-scale.
func (p *VizProbe) XScale() *reactive.ReactiveScale { return p.xScale }

// YScale returns the linear y-scale.
func (p *VizProbe) YScale() *reactive.ReactiveScale { return p.yScale }

// ZoomController returns the x-domain pan/zoom controller.
func (p *VizProbe) ZoomController() *reactive.ZoomController { return p.zoom }

// Line returns the line series mark.
func (p *VizProbe) Line() *viz.Line[dataset.Row] { return p.line }

// Rule returns the reference rule mark.
func (p *VizProbe) Rule() *viz.Rule { return p.rule }

// XAxis returns the bottom time axis.
func (p *VizProbe) XAxis() *viz.Axis { return p.xAxis }

// YAxis returns the left value axis.
func (p *VizProbe) YAxis() *viz.Axis { return p.yAxis }

// RuleValue returns the reference rule's value store.
func (p *VizProbe) RuleValue() *store.ValueStore[float64] { return p.ruleValue }

// PlotRect returns the plot area in window space (valid after arrange).
func (p *VizProbe) PlotRect() gfx.Rect { return p.plot }

func (p *VizProbe) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	for _, child := range []facet.FacetImpl{p.line, p.xAxis, p.yAxis, p.rule} {
		if role := child.Base().LayoutRole(); role != nil {
			role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
		}
	}
	p.margins.left = p.yAxis.Base().LayoutRole().MeasuredSize.W
	p.margins.bottom = p.xAxis.Base().LayoutRole().MeasuredSize.H
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (p *VizProbe) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	plot := gfx.RectFromXYWH(
		bounds.Min.X+p.margins.left,
		bounds.Min.Y+p.margins.top,
		bounds.Width()-p.margins.left-p.margins.right,
		bounds.Height()-p.margins.top-p.margins.bottom,
	)
	if plot.Width() < 1 || plot.Height() < 1 {
		return
	}
	p.plot = plot

	// Push the pixel ranges into the scales (screen y-down).
	p.xRange.Set([2]float64{0, float64(plot.Width())})
	p.yRange.Set([2]float64{float64(plot.Height()), 0})

	p.line.Base().LayoutRole().Arrange(ctx, plot)
	p.rule.Base().LayoutRole().Arrange(ctx, plot)
	p.xAxis.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(plot.Min.X, plot.Max.Y, plot.Width(), p.margins.bottom))
	p.yAxis.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, plot.Min.Y, p.margins.left, plot.Height()))
}

func (p *VizProbe) onPointer(e facet.PointerEvent) bool {
	switch e.Kind {
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			p.dragging = true
			p.dragStart = e.Position
		}
	case platform.PointerMove:
		if p.dragging {
			p.panPixels(e.Position.X - p.dragStart.X)
			p.dragStart = e.Position
		}
	case platform.PointerRelease:
		p.dragging = false
	}
	return true
}

func (p *VizProbe) onScroll(e facet.ScrollEvent) bool {
	focal := float64(e.Position.X - p.plot.Min.X)
	factor := 1.0 - float64(e.DeltaY)*0.01
	if factor <= 0 {
		factor = 0.1
	}
	p.zoomAt(focal, factor)
	return true
}

// panPixels pans the x-domain by a pixel drag. Dragging right moves the view
// window toward earlier data (the data appears to move right).
func (p *VizProbe) panPixels(dx float32) {
	if p.plot.Width() <= 0 {
		return
	}
	lo, hi := p.xDomain.Get()[0], p.xDomain.Get()[1]
	dataPerPixel := (hi - lo) / float64(p.plot.Width())
	p.zoom.Pan(-float64(dx) * dataPerPixel)
}

// zoomAt zooms the x-domain around a focal pixel (factor > 1 zooms in).
func (p *VizProbe) zoomAt(focalPx, factor float64) {
	if p.plot.Width() <= 0 {
		return
	}
	if inv, ok := p.xScale.Get().(scale.InvertibleScale); ok {
		p.zoom.Zoom(inv.Invert(focalPx), factor)
	}
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

func meanValue(rows []dataset.Row) float64 {
	if len(rows) == 0 {
		return 0
	}
	sum := 0.0
	for _, r := range rows {
		sum += r.Value
	}
	return sum / float64(len(rows))
}

func (p *VizProbe) Base() *facet.Facet             { p.BindImpl(p); return &p.Facet }
func (p *VizProbe) OnAttach(_ facet.AttachContext) {}
func (p *VizProbe) OnDetach()                      {}
func (p *VizProbe) OnActivate()                    {}
func (p *VizProbe) OnDeactivate()                  {}
