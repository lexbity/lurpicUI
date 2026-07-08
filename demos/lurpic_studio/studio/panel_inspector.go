package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type inspectorPanel struct {
	col         *layout.ColumnLayout
	textField   *input.TextField
	numberField *input.NumberField
	colorPicker *input.ColorPicker
	checkbox    *selection.Checkbox
	radioGroup  *selection.RadioGroup
	slider      *selection.Slider
	switchCtrl  *selection.Switch
	dropdown    *selection.DropdownSelect
	buttonGroup *selection.ButtonGroup
	actionGroup *action.ActionGroup
	turnDial    *selection.TurnDial
}

func newInspectorPanel(as *state.AppState) *inspectorPanel {
	ip := &inspectorPanel{}

	ip.textField = input.NewTextField("Chart Title", uiinput.TextInputOutlined)
	ip.textField.Value = as.ChartTitle
	allowLinear(ip.textField)

	ip.numberField = input.NewNumberField("Y-Axis Max")
	ip.numberField.Value = as.YAxisMax
	allowLinear(ip.numberField)

	ip.colorPicker = input.NewColorPicker("Series Color")
	ip.colorPicker.SetColor(as.SeriesColor.Get())
	ip.colorPicker.ColorChanged.Subscribe(func(c gfx.Color) {
		as.SeriesColor.Set(c)
	})
	allowLinear(ip.colorPicker)

	ip.checkbox = selection.NewCheckbox("Show Grid")
	if as.ShowGrid.Get() {
		ip.checkbox.SetChecked(true)
	} else {
		ip.checkbox.SetChecked(false)
	}
	ip.checkbox.Value.OnChange.Subscribe(func(c signal.Change[selection.CheckboxState]) {
		as.ShowGrid.Set(c.New == selection.CheckboxStateOn)
	})
	as.ShowGrid.OnChange.Subscribe(func(c signal.Change[bool]) {
		if c.New {
			ip.checkbox.SetChecked(true)
		} else {
			ip.checkbox.SetChecked(false)
		}
	})
	allowLinear(ip.checkbox)

	ip.radioGroup = selection.NewRadioGroup("Chart Type", []selection.RadioOption{
		{Value: string(state.ChartLine), Label: "Line"},
		{Value: string(state.ChartArea), Label: "Area"},
		{Value: string(state.ChartBar), Label: "Bar"},
		{Value: string(state.ChartScatter), Label: "Scatter"},
	})
	ip.radioGroup.Value.OnChange.Subscribe(func(c signal.Change[string]) {
		as.ChartType.Set(state.ChartType(c.New))
	})
	allowLinear(ip.radioGroup)

	ip.slider = selection.NewSlider("Opacity", 0, 1, 0.05)
	ip.slider.Value = as.Opacity
	allowLinear(ip.slider)

	ip.switchCtrl = selection.NewSwitch("Live Refresh")
	ip.switchCtrl.Value = as.Live
	allowLinear(ip.switchCtrl)

	ip.dropdown = selection.NewDropdownSelect("Aggregation", []selection.DropdownOption{
		{Value: string(state.AggNone), Label: "None"},
		{Value: string(state.AggSum), Label: "Sum"},
		{Value: string(state.AggAvg), Label: "Average"},
	})
	ip.dropdown.Value.Set(string(as.Aggregation.Get()))
	ip.dropdown.Value.OnChange.Subscribe(func(c signal.Change[string]) {
		as.Aggregation.Set(state.AggMode(c.New))
	})
	allowLinear(ip.dropdown)

	ip.buttonGroup = selection.NewButtonGroup("Time Range", []selection.ButtonGroupOption{
		{Key: "7d", Label: "7d"},
		{Key: "30d", Label: "30d"},
		{Key: "all", Label: "All"},
	})
	ip.buttonGroup.SetSelectedKeys("all")
	allowLinear(ip.buttonGroup)

	ip.actionGroup = action.NewActionGroup(marks.Const("Align"), marks.Const([]action.ActionGroupAction{
		{Key: "left", Label: "Left", Active: true},
		{Key: "center", Label: "Center"},
		{Key: "right", Label: "Right"},
	}))
	allowLinear(ip.actionGroup)

	ip.turnDial = selection.NewTurnDial("Smoothing", 0, 100, 1)
	ip.turnDial.Value = as.Smoothing
	allowLinear(ip.turnDial)

	ip.col = layout.NewColumnLayout()
	ip.col.Gap = 4
	ip.col.Add(layout.Fixed(ip.textField))
	ip.col.Add(layout.Fixed(ip.numberField))
	ip.col.Add(layout.Fixed(wrapInCard("Color", ip.colorPicker)))
	ip.col.Add(layout.Fixed(ip.checkbox))
	ip.col.Add(layout.Fixed(wrapInCard("Chart Type", ip.radioGroup)))
	ip.col.Add(layout.Fixed(ip.slider))
	ip.col.Add(layout.Fixed(ip.switchCtrl))
	ip.col.Add(layout.Fixed(wrapInCard("Aggregation", ip.dropdown)))
	ip.col.Add(layout.Fixed(wrapInCard("Time Range", ip.buttonGroup)))
	ip.col.Add(layout.Fixed(wrapInCard("Align", ip.actionGroup)))
	ip.col.Add(layout.Fixed(wrapInCard("Smoothing", ip.turnDial)))

	return ip
}

func wrapInCard(label string, content marks.Mark) *structure.Card {
	card := structure.NewCard(label)
	card.LayoutMode = marks.Const(structure.CardLayoutVertical)
	card.ChildrenContent = []structure.CardChild{
		{Key: label, Facet: content},
	}
	return card
}

var _ = gfx.Color{}
var _ = signal.Change[string]{}
var _ = store.NewValueStore[int](0)
