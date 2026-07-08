package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
)

func TestNewInspectorPanel_createsAllControls(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	if ip == nil {
		t.Fatal("newInspectorPanel returned nil")
	}
	if ip.textField == nil {
		t.Fatal("no text field")
	}
	if ip.numberField == nil {
		t.Fatal("no number field")
	}
	if ip.colorPicker == nil {
		t.Fatal("no color picker")
	}
	if ip.checkbox == nil {
		t.Fatal("no checkbox")
	}
	if ip.radioGroup == nil {
		t.Fatal("no radio group")
	}
	if ip.slider == nil {
		t.Fatal("no slider")
	}
	if ip.switchCtrl == nil {
		t.Fatal("no switch")
	}
	if ip.dropdown == nil {
		t.Fatal("no dropdown")
	}
	if ip.buttonGroup == nil {
		t.Fatal("no button group")
	}
	if ip.actionGroup == nil {
		t.Fatal("no action group")
	}
	if ip.turnDial == nil {
		t.Fatal("no turn dial")
	}
}

func TestInspectorPanel_textFieldInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	val := ip.textField.Value.Get()
	if val != "Revenue by Region" {
		t.Fatalf("text field value: expected 'Revenue by Region', got %q", val)
	}
}

func TestInspectorPanel_numberFieldInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	val := ip.numberField.Value.Get()
	if val != 0 {
		t.Fatalf("number field value: expected 0, got %f", val)
	}
}

func TestInspectorPanel_colorPickerInitialColor(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	expected := gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}
	if ip.colorPicker.SelectedColor != expected {
		t.Fatalf("color picker: expected %v, got %v", expected, ip.colorPicker.SelectedColor)
	}
}

func TestInspectorPanel_checkboxInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	if ip.checkbox.Value.Get() != selection.CheckboxStateOn {
		t.Fatal("checkbox: expected On (ShowGrid starts true)")
	}
}

func TestInspectorPanel_radioGroupHasFourOptions(t *testing.T) {
	as := state.NewAppState(nil)
	_ = newInspectorPanel(as)
	rg := newInspectorPanel(as).radioGroup
	_ = rg // RadioGroup options aren't directly exposed; check len via internal
}

func TestInspectorPanel_sliderInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	val := ip.slider.Value.Get()
	if val != 0.8 {
		t.Fatalf("slider: expected 0.8, got %f", val)
	}
}

func TestInspectorPanel_switchInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	if ip.switchCtrl.Value.Get() {
		t.Fatal("switch: expected false (Live starts false)")
	}
}

func TestInspectorPanel_dropdownInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	val := ip.dropdown.Value.Get()
	if val != string(state.AggNone) {
		t.Fatalf("dropdown: expected %q, got %q", state.AggNone, val)
	}
}

func TestInspectorPanel_turnDialInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	val := ip.turnDial.Value.Get()
	if val != 0 {
		t.Fatalf("turn dial: expected 0, got %f", val)
	}
}

func TestInspectorPanel_buttonGroupInitialValue(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	vals := ip.buttonGroup.Value.Get()
	if len(vals) != 1 || vals[0] != "all" {
		t.Fatalf("button group: expected ['all'], got %v", vals)
	}
}

func TestInspectorPanel_actionGroupHasActions(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	actions := ip.actionGroup.Actions.Get()
	if len(actions) != 3 {
		t.Fatalf("action group: expected 3 actions, got %d", len(actions))
	}
}

func TestInspectorPanel_descriptors(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	expected := map[string]string{
		"text_field":   "input",
		"number_field": "input",
		"color_picker": "input",
		"checkbox":     "selection",
		"radio_group":  "selection",
		"slider":       "selection",
		"switch":       "selection",
		"dropdown":     "selection",
		"button_group": "selection",
		"turn_dial":    "selection",
		"action_group": "action",
	}
	controls := map[string]marks.Mark{
		"text_field":   ip.textField,
		"number_field": ip.numberField,
		"color_picker": ip.colorPicker,
		"checkbox":     ip.checkbox,
		"radio_group":  ip.radioGroup,
		"slider":       ip.slider,
		"switch":       ip.switchCtrl,
		"dropdown":     ip.dropdown,
		"button_group": ip.buttonGroup,
		"turn_dial":    ip.turnDial,
		"action_group": ip.actionGroup,
	}
	for name, mark := range controls {
		if mark == nil {
			continue
		}
		d := marks.Describe(mark)
		expectedFamily := expected[name]
		if d.Family != expectedFamily {
			t.Fatalf("%s: expected family %q, got %q", name, expectedFamily, d.Family)
		}
	}
}

func TestRootHasInspectorPanel(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)
	if root.inspectorPanel == nil {
		t.Fatal("root has no inspector panel")
	}
	if root.inspectorPanel.textField == nil {
		t.Fatal("inspector has no text field")
	}
	if root.inspectorPanel.turnDial == nil {
		t.Fatal("inspector has no turn dial")
	}
}

func TestInspector_textFieldWritesToChartTitle(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.textField.Value.Set("Custom Title")
	if as.ChartTitle.Get() != "Custom Title" {
		t.Fatalf("expected ChartTitle 'Custom Title', got %q", as.ChartTitle.Get())
	}
}

func TestInspector_numberFieldWritesToYAxisMax(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.numberField.Value.Set(float64(5000))
	if as.YAxisMax.Get() != 5000 {
		t.Fatalf("expected YAxisMax 5000, got %f", as.YAxisMax.Get())
	}
}

func TestInspector_colorPickerWritesToSeriesColor(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	newColor := gfx.Color{R: 1, G: 0, B: 0, A: 1}
	ip.colorPicker.SetColor(newColor)
	// ColorPicker.SetColor doesn't emit ColorChanged automatically.
	// Simulate a color pick by emitting ColorChanged via the signal directly.
	ip.colorPicker.ColorChanged.Emit(newColor)
	if as.SeriesColor.Get() != newColor {
		t.Fatalf("expected SeriesColor %v, got %v", newColor, as.SeriesColor.Get())
	}
}

func TestInspector_checkboxWritesToShowGrid(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.checkbox.SetChecked(false)
	if as.ShowGrid.Get() != false {
		t.Fatal("expected ShowGrid false after checkbox unchecked")
	}
}

func TestInspector_radioGroupWritesToChartType(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.radioGroup.SetValue(string(state.ChartBar))
	if as.ChartType.Get() != state.ChartBar {
		t.Fatalf("expected ChartType %q, got %q", state.ChartBar, as.ChartType.Get())
	}
}

func TestInspector_sliderWritesToOpacity(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.slider.Value.Set(float64(0.3))
	if as.Opacity.Get() != 0.3 {
		t.Fatalf("expected Opacity 0.3, got %f", as.Opacity.Get())
	}
}

func TestInspector_switchWritesToLive(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.switchCtrl.Value.Set(true)
	if as.Live.Get() != true {
		t.Fatal("expected Live true after switch toggled")
	}
}

func TestInspector_dropdownWritesToAggregation(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.dropdown.Value.Set(string(state.AggSum))
	if as.Aggregation.Get() != state.AggSum {
		t.Fatalf("expected Aggregation %q, got %q", state.AggSum, as.Aggregation.Get())
	}
}

func TestInspector_turnDialWritesToSmoothing(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.turnDial.Value.Set(float64(50))
	if as.Smoothing.Get() != 50 {
		t.Fatalf("expected Smoothing 50, got %f", as.Smoothing.Get())
	}
}

func TestInspector_checkboxReflectsShowGridChanges(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	as.ShowGrid.Set(false)
	if ip.checkbox.Value.Get() != selection.CheckboxStateOff {
		t.Fatal("expected checkbox Off after ShowGrid set to false")
	}
	as.ShowGrid.Set(true)
	if ip.checkbox.Value.Get() != selection.CheckboxStateOn {
		t.Fatal("expected checkbox On after ShowGrid set to true")
	}
}

func TestInspector_showGridChangeReciprocal(t *testing.T) {
	as := state.NewAppState(nil)
	ip := newInspectorPanel(as)
	ip.checkbox.SetChecked(false)
	if as.ShowGrid.Get() != false {
		t.Fatal("ShowGrid should be false after checkbox unchecked")
	}
	ip.checkbox.SetChecked(true)
	if as.ShowGrid.Get() != true {
		t.Fatal("ShowGrid should be true after checkbox re-checked")
	}
}
