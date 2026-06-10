package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func testInspectorState() *state.AppState {
	rows := []dataset.Row{{Revenue: 100, Users: 10, Region: "NA"}}
	s := state.NewAppState(rows)
	s.ChartTitle.Set("Test Chart")
	s.YAxisMax.Set(50000)
	s.SeriesColor.Set(gfx.Color{R: 1, G: 0, B: 0, A: 1})
	s.ShowGrid.Set(true)
	s.ChartType.Set(state.ChartBar)
	s.Opacity.Set(0.75)
	s.Live.Set(true)
	s.Aggregation.Set(state.AggAvg)
	s.Smoothing.Set(50)
	return s
}

func TestInspectorPanelConstructs(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p == nil {
		t.Fatal("NewInspectorPanel returned nil")
	}
}

func TestInspectorPanelControlCount(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.ControlCount() != 11 {
		t.Fatalf("expected 11 controls, got %d", p.ControlCount())
	}
}

func TestInspectorTextFieldInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.TextField().Value.Get() != "Test Chart" {
		t.Errorf("expected TextField value 'Test Chart', got %q", p.TextField().Value.Get())
	}
}

func TestInspectorNumberFieldInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.NumberField().Value.Get() != 50000 {
		t.Errorf("expected NumberField value 50000, got %f", p.NumberField().Value.Get())
	}
}

func TestInspectorColorPickerInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	c := p.ColorPicker().SelectedColor
	if c.R != 1 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("expected ColorPicker SelectedColor red, got %v", c)
	}
}

func TestInspectorCheckboxInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.Checkbox().Value.Get() != 1 {
		t.Errorf("expected Checkbox On (1), got %v", p.Checkbox().Value.Get())
	}
}

func TestInspectorRadioGroupInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.RadioGroup().Value.Get() != "bar" {
		t.Errorf("expected RadioGroup value 'bar', got %q", p.RadioGroup().Value.Get())
	}
}

func TestInspectorSliderInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.Slider().Value.Get() != 0.75 {
		t.Errorf("expected Slider value 0.75, got %f", p.Slider().Value.Get())
	}
}

func TestInspectorSwitchInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if !p.Switch().Value.Get() {
		t.Error("expected Switch value true")
	}
}

func TestInspectorDropdownInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.DropdownSelect().Value.Get() != "avg" {
		t.Errorf("expected DropdownSelect value 'avg', got %q", p.DropdownSelect().Value.Get())
	}
}

func TestInspectorButtonGroupInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	vals := p.ButtonGroup().Value.Get()
	if len(vals) != 1 || vals[0] != "all" {
		t.Errorf("expected ButtonGroup value ['all'], got %v", vals)
	}
}

func TestInspectorActionGroupHasActions(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	actions := p.ActionGroup().Actions.Get()
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	if actions[0].Key != "align_left" || actions[2].Key != "align_right" {
		t.Errorf("unexpected action keys: %v", actions)
	}
}

func TestInspectorTurnDialInitialValue(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)
	if p.TurnDial().Value.Get() != 50 {
		t.Errorf("expected TurnDial value 50, got %f", p.TurnDial().Value.Get())
	}
}

func TestInspectorControlsMatchAppState(t *testing.T) {
	s := testInspectorState()
	p := NewInspectorPanel(s)

	if p.TextField().Value.Get() != s.ChartTitle.Get() {
		t.Error("TextField doesn't match ChartTitle")
	}
	if p.NumberField().Value.Get() != s.YAxisMax.Get() {
		t.Error("NumberField doesn't match YAxisMax")
	}
	if p.Slider().Value.Get() != s.Opacity.Get() {
		t.Error("Slider doesn't match Opacity")
	}
	if p.TurnDial().Value.Get() != s.Smoothing.Get() {
		t.Error("TurnDial doesn't match Smoothing")
	}
	if p.Switch().Value.Get() != s.Live.Get() {
		t.Error("Switch doesn't match Live")
	}
}
