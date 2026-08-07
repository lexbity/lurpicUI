package studio

import (
	"strconv"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// RealtimeData is the E1 exhibit descriptor; Build constructs the facet over
// the shared app state.
type RealtimeData struct {
	fonts *text.FontRegistry
	theme theme.ResolvedContext
}

// NewRealtimeData builds the E1 exhibit for the given font registry and theme.
func NewRealtimeData(fonts *text.FontRegistry, themeCtx theme.ResolvedContext) *RealtimeData {
	return &RealtimeData{fonts: fonts, theme: themeCtx}
}

func (e *RealtimeData) ID() ExhibitID { return ExhibitRealtime }
func (e *RealtimeData) Title() string { return "Realtime Data" }
func (e *RealtimeData) Build(appState *state.AppState) facet.FacetImpl {
	return NewRealtimeFacet(appState, e.fonts, e.theme)
}

// timeRangeSeconds maps a TimeRange button key to the live-window width W.
func timeRangeSeconds(key string) float64 {
	key = strings.TrimSuffix(key, "s")
	if v, err := strconv.ParseFloat(key, 64); err == nil && v > 0 {
		return v
	}
	return 60
}

// Realtime is the E1 flagship facet: the live chart (ChartCanvas), the
// streaming feed (snapshot→work→commit), the sliding live-tail window, and a
// control strip. Slice P5 is read-only (part A); the editable grid and linked
// brushing land in P6.
type Realtime struct {
	facet.Facet
	layout facet.LayoutRole
	tick   facet.TickRole

	appState *state.AppState
	canvas   *ChartCanvas
	feed     *Feed

	// Control stores (the control marks write these).
	chartType   *store.ValueStore[string]
	seriesColor *store.ValueStore[gfx.Color]
	opacity     *store.ValueStore[float64]
	showGrid    *store.ValueStore[bool]
	ruleValue   *store.ValueStore[float64]
	gridState   *store.ValueStore[selection.CheckboxState]
	timeRange   *store.ValueStore[[]string]

	controls *structure.Card

	rt      facet.RuntimeServices
	cleanup func()
}

// NewRealtimeFacet builds the E1 facet over the shared app state.
func NewRealtimeFacet(appState *state.AppState, fonts *text.FontRegistry, themeCtx theme.ResolvedContext) *Realtime {
	e := &Realtime{
		appState:    appState,
		chartType:   store.NewValueStore(ChartLine.String()),
		seriesColor: store.NewValueStore(gfx.Color{}),
		opacity:     store.NewValueStore(1.0),
		showGrid:    store.NewValueStore(false),
		ruleValue:   store.NewValueStore(0.0),
		gridState:   store.NewValueStore(selection.CheckboxStateOff),
		timeRange:   store.NewValueStore([]string{"60s"}),
	}
	e.Facet = facet.NewFacet()
	e.feed = NewFeed(appState, uint64(e.Facet.ID()))

	e.canvas = NewChartCanvas(ChartConfig{
		Fonts:       fonts,
		Theme:       themeCtx,
		Rows:        appState.Rows,
		XDomain:     appState.LiveWindow, // the live-tail window is the x-domain
		XRange:      store.NewValueStore([2]float64{0, 1}),
		YDomain:     appState.YDomain,
		YRange:      store.NewValueStore([2]float64{1, 0}),
		Paused:      appState.Paused,
		ChartType:   e.chartType,
		SeriesColor: e.seriesColor,
		Opacity:     e.opacity,
		ShowGrid:    e.showGrid,
		RuleValue:   e.ruleValue,
	})

	e.buildControls()
	e.AddChild(e.canvas.Base())
	e.AddChild(e.controls.Base())

	e.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke exhibit host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return e.measure(ctx, c)
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			e.arrange(ctx, bounds)
		},
	}
	e.layout.Child = linearChildContract(facet.StretchPolicy{
		Width:  facet.StretchAlways,
		Height: facet.StretchAlways,
	})
	e.tick = facet.TickRole{OnTick: func(dt time.Duration) { e.feed.OnTick(dt) }}
	e.AddRole(&e.layout)
	e.AddRole(&e.tick)
	return e
}

// Canvas returns the chart canvas.
func (e *Realtime) Canvas() *ChartCanvas { return e.canvas }

// Feed returns the streaming feed.
func (e *Realtime) Feed() *Feed { return e.feed }

// ChartType returns the series-selection store.
func (e *Realtime) ChartType() *store.ValueStore[string] { return e.chartType }

// SeriesColor returns the series color store.
func (e *Realtime) SeriesColor() *store.ValueStore[gfx.Color] { return e.seriesColor }

// Opacity returns the series opacity store.
func (e *Realtime) Opacity() *store.ValueStore[float64] { return e.opacity }

// ShowGrid returns the grid-overlay store.
func (e *Realtime) ShowGrid() *store.ValueStore[bool] { return e.showGrid }

// RuleValue returns the reference-rule value store.
func (e *Realtime) RuleValue() *store.ValueStore[float64] { return e.ruleValue }

// TimeRange returns the selected time-range keys (the button group's store).
func (e *Realtime) TimeRange() *store.ValueStore[[]string] { return e.timeRange }

func (e *Realtime) buildControls() {
	liveSwitch := selection.NewSwitch("Live", e.feed.Live())
	chartRadio := selection.NewRadioGroup("Chart type", []selection.RadioOption{
		{Value: ChartLine.String(), Label: "Line"},
		{Value: ChartArea.String(), Label: "Area"},
		{Value: ChartPoint.String(), Label: "Points"},
		{Value: ChartBar.String(), Label: "Bars"},
	}, e.chartType)
	seriesPicker := input.NewColorPicker("Series color", e.seriesColor)
	opacitySlider := selection.NewSlider("Opacity", 0.1, 1, 0.05, e.opacity)
	gridCheck := selection.NewCheckbox("Grid", e.gridState)
	maxField := input.NewNumberField("Y max", e.appState.YAxisMax)
	rangeButtons := selection.NewButtonGroup("Time range", []selection.ButtonGroupOption{
		{Key: "30s", Label: "30s"},
		{Key: "60s", Label: "60s"},
		{Key: "120s", Label: "120s"},
	}, e.timeRange)

	e.controls = structure.NewCard("Chart controls")
	e.controls.GridColumns = marks.Const(3)
	e.controls.GridRows = marks.Const(3)
	e.controls.ChildrenContent = []structure.CardChild{
		{Key: "live", Facet: liveSwitch, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "chart", Facet: chartRadio, Grid: facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 2, RowSpan: 1}},
		{Key: "color", Facet: seriesPicker, Grid: facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "opacity", Facet: opacitySlider, Grid: facet.GridPlacement{ColStart: 1, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "grid", Facet: gridCheck, Grid: facet.GridPlacement{ColStart: 2, RowStart: 1, ColSpan: 1, RowSpan: 1}},
		{Key: "max", Facet: maxField, Grid: facet.GridPlacement{ColStart: 0, RowStart: 2, ColSpan: 1, RowSpan: 1}},
		{Key: "range", Facet: rangeButtons, Grid: facet.GridPlacement{ColStart: 1, RowStart: 2, ColSpan: 2, RowSpan: 1}},
	}
}

func (e *Realtime) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := e.canvas.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	// Measure the controls with an unbounded height so the control card
	// reports its content height instead of flex-filling the whole stage.
	if role := e.controls.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}})
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (e *Realtime) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	controlsH := e.controls.Base().LayoutRole().MeasuredSize.H
	canvasH := bounds.Height() - controlsH
	if canvasH < 1 {
		canvasH = 1
	}
	e.canvas.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), canvasH))
	e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+canvasH, bounds.Width(), controlsH))
}

func (e *Realtime) OnAttach(ctx facet.AttachContext) {
	e.rt = ctx.Runtime
	e.feed.SetRuntime(ctx.Runtime)

	unsubInsert := e.appState.Rows.OnInsertSubscribe(func(ev store.CollectionInsertEvent[dataset.Row]) {
		e.ruleValue.Set(ev.Item.Value)
	})
	gridID := e.gridState.OnChange.Subscribe(func(c signal.Change[selection.CheckboxState]) {
		e.showGrid.Set(c.New == selection.CheckboxStateOn)
	})
	rangeID := e.timeRange.OnChange.Subscribe(func(c signal.Change[[]string]) {
		e.applyTimeRange(c.New)
	})
	e.cleanup = func() {
		unsubInsert()
		e.gridState.OnChange.Unsubscribe(gridID)
		e.timeRange.OnChange.Unsubscribe(rangeID)
	}
}

func (e *Realtime) OnDetach() {
	if e.cleanup != nil {
		e.cleanup()
		e.cleanup = nil
	}
}

// applyTimeRange maps the selected button-group keys to the live-window width.
func (e *Realtime) applyTimeRange(keys []string) {
	w := 60.0
	if len(keys) > 0 {
		w = timeRangeSeconds(keys[len(keys)-1])
	}
	e.appState.WindowSeconds.Set(w)
}

func (e *Realtime) Base() *facet.Facet { e.BindImpl(e); return &e.Facet }
func (e *Realtime) OnActivate()        {}
func (e *Realtime) OnDeactivate()      {}
