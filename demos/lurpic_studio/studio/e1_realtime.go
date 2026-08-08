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
// streaming feed (snapshot→work→commit), the sliding live-tail window, the
// editable spreadsheet (CollectionBinder composition), a read-only table
// legend, and a control strip. Slice P5 was read-only (part A); P6 adds the
// editable grid + linked brushing (part B).
type Realtime struct {
	facet.Facet
	layout facet.LayoutRole
	tick   facet.TickRole

	appState *state.AppState
	canvas   *ChartCanvas
	feed     *Feed
	grid     *EditableGrid
	table    *structure.Table
	brush    BrushStores

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

// gridHeight is the spreadsheet's fixed height inside E1.
const gridHeight float32 = 180

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
		brush:       NewBrushStores(),
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
		Hover:       e.brush.Hover,
		HoverRegion: e.brush.HoverRegion,
		Selection:   e.brush.Selection,
	})

	e.grid = NewEditableGrid(appState.Rows, fonts, themeCtx, e.brush)
	e.table = structure.NewTable("Latest", latestTableData(appState.Rows), nil)
	e.buildControls()
	e.AddChild(e.canvas.Base())
	e.AddChild(e.grid.Base())
	e.AddChild(e.controls.Base())
	e.AddChild(e.table.Base())

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

// Grid returns the editable spreadsheet.
func (e *Realtime) Grid() *EditableGrid { return e.grid }

// Brush returns the shared linked-brushing channels.
func (e *Realtime) Brush() BrushStores { return e.brush }

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
	if role := e.grid.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	// The bottom strip (controls + table) is content-height, so measure it
	// with an unbounded height instead of letting it flex-fill the stage.
	content := facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}}
	if role := e.controls.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	if role := e.table.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
}

func (e *Realtime) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	controlsH := e.controls.Base().LayoutRole().MeasuredSize.H
	tableH := e.table.Base().LayoutRole().MeasuredSize.H
	bottomH := controlsH
	if tableH > bottomH {
		bottomH = tableH
	}
	if bottomH < 1 {
		bottomH = 1
	}
	canvasH := bounds.Height() - gridHeight - bottomH
	if canvasH < 1 {
		canvasH = 1
	}
	e.canvas.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), canvasH))
	e.grid.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y+canvasH, bounds.Width(), gridHeight))

	// Bottom strip: the controls card on the left, the table legend on the
	// right.
	bottomY := bounds.Min.Y + canvasH + gridHeight
	controlsW := bounds.Width() * 0.62
	e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bottomY, controlsW, bottomH))
	e.table.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X+controlsW, bottomY, bounds.Width()-controlsW, bottomH))
}

// latestTableData builds a compact read-only table snapshot from the newest
// rows (the table mark's honest coverage role: a non-editable legend).
func latestTableData(rows *store.CollectionStore[dataset.Row]) structure.TableData {
	all := rows.All()
	n := 5
	if len(all) < n {
		n = len(all)
	}
	out := structure.TableData{
		Columns: []structure.TableColumn{
			{Key: "time", Label: "Time"},
			{Key: "value", Label: "Value"},
			{Key: "region", Label: "Region"},
		},
		Rows: make([]structure.TableRow, 0, n),
	}
	for i := len(all) - n; i < len(all); i++ {
		r := all[i]
		out.Rows = append(out.Rows, structure.TableRow{
			Key:   strconv.Itoa(i),
			Cells: []string{r.Time.Format("01-02 15:04:05"), strconv.FormatFloat(r.Value, 'f', 1, 64), r.Region},
		})
	}
	return out
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
