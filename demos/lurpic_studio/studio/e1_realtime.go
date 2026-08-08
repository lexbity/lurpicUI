package studio

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/runtime"
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
	legend   *structure.List
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
	reshape  *action.RadialMenu
	jump     *action.IconButton

	// tip is the chart-side anchored tooltip (FR-brush: a selection — from a
	// chart point press or a grid row click — shows the selected row's details
	// anchored to the chart).
	tip     *feedback.Tooltip
	tipOpen *store.ValueStore[bool]
	tipText *store.ValueStore[string]

	reshapeUnsub []func() //lurpiclint:ignore LL012 -- subscription cleanup handles are structural lifecycle state (F-lint-hosts)
	cleanupFns   []func() //lurpiclint:ignore LL012 -- teardown handles are structural lifecycle state (F-lint-hosts)
	rt           facet.RuntimeServices
	cleanup      func()
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
		Fonts:        fonts,
		Theme:        themeCtx,
		Rows:         appState.Rows,
		XDomain:      appState.LiveWindow, // the live-tail window is the x-domain
		XRange:       store.NewValueStore([2]float64{0, 1}),
		YDomain:      appState.YDomain,
		YRange:       store.NewValueStore([2]float64{1, 0}),
		WindowedRows: appState.VisibleRows, // FR-viz: series plot the live-window view
		Paused:       appState.Paused,
		ChartType:    e.chartType,
		SeriesColor:  e.seriesColor,
		Opacity:      e.opacity,
		ShowGrid:     e.showGrid,
		RuleValue:    e.ruleValue,
		Hover:        e.brush.Hover,
		HoverRegion:  e.brush.HoverRegion,
		Selection:    e.brush.Selection,
	})

	e.grid = NewEditableGrid(appState.Rows, fonts, themeCtx, e.brush)
	e.table = structure.NewTable("Latest", latestTableData(appState.Rows), nil)
	e.legend = structure.NewList("Feed legend", bucketLegendEntries(appState.BarBuckets.Get()))
	e.tipOpen = store.NewValueStore(false)
	e.tipText = store.NewValueStore("")
	e.tip = feedback.NewTooltip("", e.tipOpen)
	e.tip.Content = marks.FromStore(e.tipText, facet.DirtyProjection)
	e.tip.Placement = facet.AnchorPlacement{Side: facet.AnchorAbove}
	e.buildControls()
	e.buildReshapeDial()
	e.buildJumpButton()
	e.AddChild(e.canvas.Base())   //lurpiclint:ignore LL021 -- E1 hosts its canvas as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.grid.Base())     //lurpiclint:ignore LL021 -- E1 hosts its grid as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.controls.Base()) //lurpiclint:ignore LL021 -- E1 hosts its controls as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.reshape.Base())  //lurpiclint:ignore LL021 -- E1 hosts its reshape dial as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.table.Base())    //lurpiclint:ignore LL021 -- E1 hosts its table as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.legend.Base())   //lurpiclint:ignore LL021 -- E1 hosts its legend as a regular child, not an overlay (LL021 over-fires on any field ref)
	e.AddChild(e.jump.Base())     //lurpiclint:ignore LL021 -- E1 hosts the jump-to-live button as a regular child, not an overlay (LL021 over-fires)
	e.AddChild(e.tip.Base())      //lurpiclint:ignore LL021 -- E1 hosts its anchored tooltip as a regular child; the mark self-mounts its layered surface (LL021 over-fires)

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
	e.tick = facet.TickRole{OnTick: func(dt time.Duration) {
		e.feed.OnTick(dt)
	}}
	e.AddRole(&e.layout)
	e.AddRole(&e.tick)
	return e
}

// Canvas returns the chart canvas.
func (e *Realtime) Canvas() *ChartCanvas { return e.canvas }

// Reshape returns the radial chart-reshape dial.
func (e *Realtime) Reshape() *action.RadialMenu { return e.reshape }

// Jump returns the jump-to-live button.
func (e *Realtime) Jump() *action.IconButton { return e.jump }

// TipOpen returns the anchored tooltip's visibility store.
func (e *Realtime) TipOpen() *store.ValueStore[bool] { return e.tipOpen }

// TipText returns the anchored tooltip's content store.
func (e *Realtime) TipText() *store.ValueStore[string] { return e.tipText }

// Tip returns the anchored tooltip mark.
func (e *Realtime) Tip() *feedback.Tooltip { return e.tip }

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

// buildReshapeDial builds the radial_menu chart-reshape control (the §3.3 E1
// placement: radial_menu · chart reshape · radial layout). Its four radial
// children are icon buttons, one per chart type; clicking one writes ChartType,
// re-projecting the canvas series. icon_button children are used because the
// radial policy arranges its children with Radial placement, which the plain
// button mark's child contract does not declare.
func (e *Realtime) buildReshapeDial() {
	types := []struct {
		key  string
		icon string
	}{
		{ChartLine.String(), iconChartLine},
		{ChartArea.String(), iconChartArea},
		{ChartPoint.String(), iconChartPoint},
		{ChartBar.String(), iconChartBar},
	}
	children := make([]action.RadialChild, 0, len(types))
	for i, t := range types {
		btn := action.NewIconButton(primitive.IconSVG(t.icon))
		key := t.key
		id := btn.Activated.Subscribe(func(signal.Unit) {
			if e.chartType.Get() != key {
				e.chartType.Set(key)
			}
		})
		e.reshapeUnsub = append(e.reshapeUnsub, func() { btn.Activated.Unsubscribe(id) })
		children = append(children, action.RadialChild{
			Child:     btn,
			Placement: facet.RadialPlacement{Angle: float64(i) * 2 * math.Pi / float64(len(types)), RadiusTrack: reshapeDialRadius},
		})
	}
	center := action.NewIconButton(primitive.IconSVG(iconRealtime))
	e.reshape = action.NewRadialMenu("Chart type", center, children)
	e.reshape.DefaultTrackRadius = reshapeDialRadius
}

// reshapeDialRadius is the radial_menu's track radius (a compact dial that
// fits the E1 bottom strip).
const reshapeDialRadius float32 = 44

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

// buildJumpButton wires the jump-to-live affordance (FR-window): an
// icon_button floating over the chart's top-right that resets the x-domain to
// [now-W, now] and clears Paused. It is a direct child (not inside the controls
// Card, whose content is self-projected and not hit-testable — F-card-content)
// so the button is a real, clickable UI affordance.
func (e *Realtime) buildJumpButton() {
	e.jump = action.NewIconButton(primitive.IconSVG(iconJumpLive))
	idJump := e.jump.Activated.Subscribe(func(signal.Unit) {
		e.jumpToLive()
	})
	e.reshapeUnsub = append(e.reshapeUnsub, func() { e.jump.Activated.Unsubscribe(idJump) })
}

// jumpToLive resets the x-domain to [now-W, now] and clears Paused (the live
// tail resumes). It is the UI affordance behind FR-window's "jump to live".
func (e *Realtime) jumpToLive() {
	rows := e.appState.Rows.All()
	if len(rows) == 0 {
		return
	}
	hi := float64(rows[len(rows)-1].Time.Unix())
	e.canvas.ResetDomain([2]float64{hi - e.appState.WindowSeconds.Get(), hi})
}

func (e *Realtime) measure(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
	if role := e.canvas.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	if role := e.grid.Base().LayoutRole(); role != nil {
		role.Measure(ctx, facet.Constraints{MaxSize: c.MaxSize})
	}
	// The bottom strip (controls + reshape dial + table) is content-height, so
	// measure it with an unbounded height instead of letting it flex-fill the
	// stage.
	content := facet.Constraints{MaxSize: gfx.Size{W: c.MaxSize.W}}
	if role := e.controls.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	if role := e.reshape.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	if role := e.legend.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	if role := e.table.Base().LayoutRole(); role != nil {
		role.Measure(ctx, content)
	}
	if role := e.tip.Base().LayoutRole(); role != nil {
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

	// Bottom strip: the controls card on the left, the jump-to-live button and
	// the radial reshape dial in the middle, and the table + feed legend
	// stacked on the right.
	bottomY := bounds.Min.Y + canvasH + gridHeight
	controlsW := bounds.Width() * 0.5
	e.controls.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bottomY, controlsW, bottomH))
	jumpW := float32(32)
	jumpX := bounds.Min.X + controlsW
	jumpH := float32(28)
	if jumpH > bottomH {
		jumpH = bottomH
	}
	e.jump.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(jumpX, bottomY+(bottomH-jumpH)*0.5, jumpW, jumpH))
	reshapeX := jumpX + jumpW
	reshapeW := e.reshape.Base().LayoutRole().MeasuredSize.W
	if reshapeW < 1 || reshapeW > bounds.Width()*0.22 {
		reshapeW = bounds.Width() * 0.22
	}
	e.reshape.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(reshapeX, bottomY, reshapeW, bottomH))
	rightX := reshapeX + reshapeW
	rightW := bounds.Max.X - rightX
	right := gfx.RectFromXYWH(rightX, bottomY, rightW, bottomH)
	legendH := e.legend.Base().LayoutRole().MeasuredSize.H
	if legendH > bottomH*0.6 {
		legendH = bottomH * 0.6
	}
	if legendH < 1 {
		legendH = 1
	}
	e.legend.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(right.Min.X, right.Min.Y, right.Width(), legendH))
	e.table.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(right.Min.X, right.Min.Y+legendH, right.Width(), right.Height()-legendH))

	// The anchored tooltip bubble sits at the plot's top-center: the facet is
	// arranged to a small rect so its (bounds-derived) hit region does not
	// claim the chart (a passive anchored tooltip must not block chart hover /
	// brush; the hit map resolves regions by bounds, not OnHitTest).
	if plot := e.canvas.PlotRect(); !plot.IsEmpty() {
		tipW := e.tip.Base().LayoutRole().MeasuredSize.W
		tipH := e.tip.Base().LayoutRole().MeasuredSize.H
		if tipW < 20 || tipW > plot.Width()*0.6 {
			tipW = 200
		}
		if tipH < 16 || tipH > plot.Height()*0.5 {
			tipH = 44
		}
		tipX := plot.Min.X + (plot.Width()-tipW)*0.5
		e.tip.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(tipX, plot.Min.Y, tipW, tipH))
	} else {
		e.tip.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}
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

// bucketLegendEntries builds the feed-legend list from the windowed
// BarBuckets: one entry per region with its live-window aggregate (the
// structure.list mark's honest role: the categorical feed legend that the bar
// chart's band scale reads, fed by the same BarBuckets derived).
func bucketLegendEntries(buckets []state.RegionBucket) []structure.ListEntry {
	entries := make([]structure.ListEntry, 0, len(buckets))
	for _, b := range buckets {
		entries = append(entries, structure.ListEntry{
			Key:            b.Region,
			Label:          b.Region,
			SupportingText: strconv.FormatFloat(b.Value, 'f', 1, 64),
			Selected:       false,
		})
	}
	return entries
}

func (e *Realtime) OnAttach(ctx facet.AttachContext) {
	e.rt = ctx.Runtime
	e.feed.SetRuntime(ctx.Runtime)
	// Arm the streaming tick (AC-1): the framework's TickRole runs only when
	// armed, and tickFacets resets it after each tick, so a phase-1 hook
	// re-arms it every frame — the runtime's own rearmTicks pattern. The real
	// app's frame loop seeds the frame clock (FrameTimer.Wait), so each frame
	// carries a real delta and the feed ticks at its cadence.
	e.tick.RequestTick()
	if rt, ok := ctx.Runtime.(*runtime.Runtime); ok {
		unarm := rt.RegisterPhase1TickHook(func(time.Duration) {
			e.tick.RequestTick()
		})
		e.cleanupLater(unarm)
	}

	unsubInsert := e.appState.Rows.OnInsertSubscribe(func(ev store.CollectionInsertEvent[dataset.Row]) {
		e.ruleValue.Set(ev.Item.Value)
	})
	// The windowed deriveds (VisibleRows, BarBuckets) are lazy: a source change
	// marks them dirty but a Get() is required to recompute and fire OnChange.
	// Flushing on the row signals + the window keeps the windowed series and
	// the feed legend in the same frame as any data or window change
	// (F-derived-range; NFR-edit-latency).
	unsubEdit := e.appState.Rows.OnUpdateSubscribe(func(store.CollectionUpdateEvent[dataset.Row]) {
		e.flushWindowedDeriveds()
	})
	unsubEvict := e.appState.Rows.OnRemoveSubscribe(func(store.CollectionRemoveEvent[dataset.Row]) {
		e.flushWindowedDeriveds()
	})
	winID := e.appState.LiveWindow.OnChange.Subscribe(func(signal.Change[[2]float64]) {
		e.flushWindowedDeriveds()
	})
	legendID := e.appState.BarBuckets.OnChange.Subscribe(func(signal.Change[[]state.RegionBucket]) {
		e.legend.Data.Set(bucketLegendEntries(e.appState.BarBuckets.Get()))
	})
	gridID := e.gridState.OnChange.Subscribe(func(c signal.Change[selection.CheckboxState]) {
		e.showGrid.Set(c.New == selection.CheckboxStateOn)
	})
	rangeID := e.timeRange.OnChange.Subscribe(func(c signal.Change[[]string]) {
		e.applyTimeRange(c.New)
	})
	selID := e.brush.Selection.OnChange.Subscribe(func(c signal.Change[store.ItemID]) {
		e.updateSelectionTip(c.New)
	})
	e.cleanup = func() {
		for _, fn := range e.cleanupFns {
			if fn != nil {
				fn()
			}
		}
		e.cleanupFns = nil
		unsubInsert()
		unsubEdit()
		unsubEvict()
		e.appState.LiveWindow.OnChange.Unsubscribe(winID)
		e.gridState.OnChange.Unsubscribe(gridID)
		e.timeRange.OnChange.Unsubscribe(rangeID)
		e.brush.Selection.OnChange.Unsubscribe(selID)
		e.appState.BarBuckets.OnChange.Unsubscribe(legendID)
	}
}

// cleanupLater appends a teardown function to the exhibit's cleanup list.
func (e *Realtime) cleanupLater(fn func()) {
	if fn == nil {
		return
	}
	e.cleanupFns = append(e.cleanupFns, fn)
}

// flushWindowedDeriveds forces the lazy windowed deriveds (VisibleRows,
// BarBuckets) to recompute so their OnChange consumers stay in sync. The
// Derived contract is lazy: a source change marks it dirty but a consumer must
// Get() to recompute and fire OnChange (F-derived-range). The tick runs before
// deliverSignals in the frame, so the recompute-triggered OnChange is delivered
// in the same frame and the windowed series + legend re-project with it.
func (e *Realtime) flushWindowedDeriveds() {
	e.appState.VisibleRows.Get()
	e.appState.BarBuckets.Get()
}

// updateSelectionTip reflects a selection (chart point press or grid row click)
// into the chart-side tooltip: the selected row's time/region/value, anchored
// to the chart. A zero selection clears the tooltip.
func (e *Realtime) updateSelectionTip(id store.ItemID) {
	if id == 0 {
		e.tipText.Set("")
		e.tipOpen.Set(false)
		return
	}
	row, ok := rowByID(e.appState.Rows, id)
	if !ok {
		e.tipText.Set("")
		e.tipOpen.Set(false)
		return
	}
	e.tipText.Set(fmt.Sprintf("%s · %s · %.1f", row.Time.Format("15:04:05"), row.Region, row.Value))
	e.tipOpen.Set(true)
	// The tooltip was measured/arranged while closed, so its projection caches
	// are empty; the mark's Open binding invalidates only its local dirty bits,
	// which the runtime's layout pass does not read (F-dirtylayout-routing).
	// Routing a layout pass through the runtime re-measures and re-arranges the
	// tooltip so its bubble actually renders.
	invalidateLayout(e.Base(), e.rt, "E1.updateSelectionTip")
}

func (e *Realtime) OnDetach() {
	for _, unsub := range e.reshapeUnsub {
		if unsub != nil {
			unsub()
		}
	}
	e.reshapeUnsub = nil
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
