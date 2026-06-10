package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

func NewInspectorPanel(appState *state.AppState) *InspectorPanel {
	p := &InspectorPanel{appState: appState}
	p.Facet = facet.NewFacet()

	p.textField = input.NewTextField("Chart Title", uiinput.TextInputOutlined)
	p.textField.Value = appState.ChartTitle

	p.numberField = input.NewNumberField("Y-Axis Max")
	p.numberField.Value = appState.YAxisMax

	p.colorPicker = input.NewColorPicker("Series Color")
	p.colorPicker.SelectedColor = appState.SeriesColor.Get()
	p.colorPicker.ColorChanged.Subscribe(func(c gfx.Color) {
		appState.SeriesColor.Set(c)
	})

	showGrid := appState.ShowGrid.Get()
	gridState := selection.CheckboxStateOff
	if showGrid {
		gridState = selection.CheckboxStateOn
	}
	p.checkbox = selection.NewCheckbox("Show Grid")
	p.checkbox.Value = store.NewValueStore(gridState)
	p.checkbox.Value.OnChange.Subscribe(func(c signal.Change[selection.CheckboxState]) {
		appState.ShowGrid.Set(c.New == selection.CheckboxStateOn)
	})

	p.radioGroup = selection.NewRadioGroup("Chart Type", []selection.RadioOption{
		{Value: "line", Label: "Line"},
		{Value: "area", Label: "Area"},
		{Value: "scatter", Label: "Scatter"},
		{Value: "bar", Label: "Bar"},
	})
	chartTypeVal := chartTypeToStr(appState.ChartType.Get())
	p.radioGroup.Value = store.NewValueStore(chartTypeVal)
	p.radioGroup.Value.OnChange.Subscribe(func(c signal.Change[string]) {
		appState.ChartType.Set(strToChartType(c.New))
	})

	p.slider = selection.NewSlider("Opacity", 0, 1, 0.05)
	p.slider.Value = appState.Opacity

	p.switchCtrl = selection.NewSwitch("Live Refresh")
	p.switchCtrl.Value = appState.Live

	p.dropdownSelect = selection.NewDropdownSelect("Aggregation", []selection.DropdownOption{
		{Value: "none", Label: "None"},
		{Value: "sum", Label: "Sum"},
		{Value: "avg", Label: "Average"},
	})
	aggVal := aggModeToStr(appState.Aggregation.Get())
	p.dropdownSelect.Value = store.NewValueStore(aggVal)
	p.dropdownSelect.Value.OnChange.Subscribe(func(c signal.Change[string]) {
		appState.Aggregation.Set(strToAggMode(c.New))
	})

	p.buttonGroup = selection.NewButtonGroup("Time Range", []selection.ButtonGroupOption{
		{Key: "7d", Label: "7d"},
		{Key: "30d", Label: "30d"},
		{Key: "all", Label: "All"},
	})
	timeVal := timeRangeToStr(appState.TimeRange.Get())
	p.buttonGroup.Value = store.NewValueStore([]string{timeVal})
	p.buttonGroup.Value.OnChange.Subscribe(func(c signal.Change[[]string]) {
		if len(c.New) > 0 {
			appState.TimeRange.Set(strToTimeRange(c.New[0]))
		}
	})

	p.actionGroup = action.NewActionGroup(
		marks.Const("Actions"),
		marks.Const([]action.ActionGroupAction{
			{Key: "align_left", Label: "Left", IconRef: "align_horizontal_left"},
			{Key: "align_center", Label: "Center", IconRef: "align_horizontal_center"},
			{Key: "align_right", Label: "Right", IconRef: "align_horizontal_right"},
		}),
	)

	p.turnDial = selection.NewTurnDial("Smoothing", 0, 100, 5)
	p.turnDial.Value = appState.Smoothing

	p.scrollRegion = structure.NewScrollRegion("Inspector")
	controls := []facet.FacetImpl{
		p.textField,
		p.numberField,
		p.colorPicker,
		p.checkbox,
		p.radioGroup,
		p.slider,
		p.switchCtrl,
		p.dropdownSelect,
		p.buttonGroup,
		p.actionGroup,
		p.turnDial,
	}
	p.controlFacets = controls

	children := make([]structure.ScrollRegionChild, len(controls))
	for i, ctrl := range controls {
		children[i] = structure.ScrollRegionChild{
			Facet:     ctrl,
			MarkID:    facet.MarkID(i + 2),
			Placement: facet.Placement{Mode: facet.PlacementGrid},
		}
	}
	p.scrollRegion.SetChildren(children)

	p.Facet.AddChild(p.scrollRegion.Base())
	for _, ctrl := range controls {
		p.scrollRegion.Base().AddChild(ctrl.Base())
	}

	p.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			width := constraints.MaxSize.W
			height := constraints.MaxSize.H
			if width <= 0 { width = 280 }
			if height <= 0 { height = 600 }
			p.scrollRegion.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: height}})
			for _, ctrl := range controls {
				ctrl.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width - 8, H: 60}})
			}
			return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			width := bounds.Width()
			height := bounds.Height()
			p.scrollRegion.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, width, height))
			p.scrollRegion.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, width, height)
			y := bounds.Min.Y + 4
			for _, ctrl := range controls {
				ctrl.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(bounds.Min.X+4, y, width-8, 52))
				ctrl.Base().LayoutRole().ArrangedBounds = gfx.RectFromXYWH(bounds.Min.X+4, y, width-8, 52)
				y += 56
			}
		},
		Child: facet.GroupChildContract{
			SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear,
			Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
				return facet.IntrinsicSize{
					Min: gfx.Size{W: 260, H: 200},
					Preferred: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
					Max: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
				}
			},
			Stretch: facet.StretchPolicy{Width: facet.StretchAlways, Height: facet.StretchAlways},
		},
	}
	p.AddRole(&p.layout)
	return p
}

type InspectorPanel struct {
	facet.Facet
	layout        facet.LayoutRole
	scrollRegion  *structure.ScrollRegion
	appState      *state.AppState

	textField     *input.TextField
	numberField   *input.NumberField
	colorPicker   *input.ColorPicker
	checkbox      *selection.Checkbox
	radioGroup    *selection.RadioGroup
	slider        *selection.Slider
	switchCtrl    *selection.Switch
	dropdownSelect *selection.DropdownSelect
	buttonGroup   *selection.ButtonGroup
	actionGroup   *action.ActionGroup
	turnDial      *selection.TurnDial

	controlFacets []facet.FacetImpl
}

func (p *InspectorPanel) Base() *facet.Facet { p.Facet.BindImpl(p); return &p.Facet }
func (p *InspectorPanel) OnAttach(ctx facet.AttachContext)  {}
func (p *InspectorPanel) OnDetach()                         {}
func (p *InspectorPanel) OnActivate()                       {}
func (p *InspectorPanel) OnDeactivate()                     {}

func (p *InspectorPanel) TextField() *input.TextField              { return p.textField }
func (p *InspectorPanel) NumberField() *input.NumberField          { return p.numberField }
func (p *InspectorPanel) ColorPicker() *input.ColorPicker          { return p.colorPicker }
func (p *InspectorPanel) Checkbox() *selection.Checkbox             { return p.checkbox }
func (p *InspectorPanel) RadioGroup() *selection.RadioGroup         { return p.radioGroup }
func (p *InspectorPanel) Slider() *selection.Slider                 { return p.slider }
func (p *InspectorPanel) Switch() *selection.Switch                 { return p.switchCtrl }
func (p *InspectorPanel) DropdownSelect() *selection.DropdownSelect { return p.dropdownSelect }
func (p *InspectorPanel) ButtonGroup() *selection.ButtonGroup       { return p.buttonGroup }
func (p *InspectorPanel) ActionGroup() *action.ActionGroup          { return p.actionGroup }
func (p *InspectorPanel) TurnDial() *selection.TurnDial             { return p.turnDial }
func (p *InspectorPanel) ControlCount() int                         { return len(p.controlFacets) }

func chartTypeToStr(ct state.ChartType) string {
	switch ct {
	case state.ChartArea: return "area"
	case state.ChartPoint: return "scatter"
	case state.ChartBar: return "bar"
	default: return "line"
	}
}

func strToChartType(s string) state.ChartType {
	switch s {
	case "area": return state.ChartArea
	case "scatter": return state.ChartPoint
	case "bar": return state.ChartBar
	default: return state.ChartLine
	}
}

func aggModeToStr(a state.AggMode) string {
	switch a {
	case state.AggSum: return "sum"
	case state.AggAvg: return "avg"
	default: return "none"
	}
}

func strToAggMode(s string) state.AggMode {
	switch s {
	case "sum": return state.AggSum
	case "avg": return state.AggAvg
	default: return state.AggNone
	}
}

func timeRangeToStr(t state.TimeRangeMode) string {
	switch t {
	case state.TimeRange7d: return "7d"
	case state.TimeRange30d: return "30d"
	default: return "all"
	}
}

func strToTimeRange(s string) state.TimeRangeMode {
	switch s {
	case "7d": return state.TimeRange7d
	case "30d": return state.TimeRange30d
	default: return state.TimeRangeAll
	}
}
