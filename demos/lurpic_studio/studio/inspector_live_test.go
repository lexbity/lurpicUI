package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func testLiveState() *state.AppState {
	rows := []dataset.Row{
		{Revenue: 1000, Users: 100, Region: "NA"},
		{Revenue: 2000, Users: 200, Region: "EU"},
	}
	return state.NewAppState(rows)
}

func TestTextFieldMutatesChartTitle(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.TextField().Value.Set("New Chart Title")
	if s.ChartTitle.Get() != "New Chart Title" {
		t.Errorf("expected ChartTitle 'New Chart Title', got %q", s.ChartTitle.Get())
	}
}

func TestNumberFieldMutatesYAxisMax(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.NumberField().Value.Set(99999)
	if s.YAxisMax.Get() != 99999 {
		t.Errorf("expected YAxisMax 99999, got %f", s.YAxisMax.Get())
	}
}

func TestColorPickerMutatesSeriesColor(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	red := gfx.Color{R: 1, G: 0, B: 0, A: 1}
	p.ColorPicker().ColorChanged.Emit(red)
	c := s.SeriesColor.Get()
	if c.R != 1 || c.G != 0 || c.B != 0 || c.A != 1 {
		t.Errorf("expected SeriesColor red, got %v", c)
	}
}

func TestCheckboxMutatesShowGrid(t *testing.T) {
	s := testLiveState()
	s.ShowGrid.Set(true)
	p := NewInspectorPanel(s)
	p.Checkbox().Value.Set(0)
	if s.ShowGrid.Get() != false {
		t.Error("expected ShowGrid false after checkbox off")
	}
	p.Checkbox().Value.Set(1)
	if s.ShowGrid.Get() != true {
		t.Error("expected ShowGrid true after checkbox on")
	}
}

func TestRadioGroupMutatesChartType(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.RadioGroup().Value.Set("bar")
	if s.ChartType.Get() != state.ChartBar {
		t.Errorf("expected ChartType bar, got %v", s.ChartType.Get())
	}
	p.RadioGroup().Value.Set("area")
	if s.ChartType.Get() != state.ChartArea {
		t.Errorf("expected ChartType area, got %v", s.ChartType.Get())
	}
}

func TestSliderMutatesOpacity(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.Slider().Value.Set(0.85)
	if s.Opacity.Get() != 0.85 {
		t.Errorf("expected Opacity 0.85, got %f", s.Opacity.Get())
	}
	p.Slider().Value.Set(0.15)
	if s.Opacity.Get() != 0.15 {
		t.Errorf("expected Opacity 0.15, got %f", s.Opacity.Get())
	}
}

func TestSwitchMutatesLive(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.Switch().Value.Set(true)
	if s.Live.Get() != true {
		t.Error("expected Live true")
	}
	p.Switch().Value.Set(false)
	if s.Live.Get() != false {
		t.Error("expected Live false")
	}
}

func TestDropdownMutatesAggregation(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.DropdownSelect().Value.Set("sum")
	if s.Aggregation.Get() != state.AggSum {
		t.Errorf("expected Aggregation sum, got %v", s.Aggregation.Get())
	}
	p.DropdownSelect().Value.Set("avg")
	if s.Aggregation.Get() != state.AggAvg {
		t.Errorf("expected Aggregation avg, got %v", s.Aggregation.Get())
	}
	p.DropdownSelect().Value.Set("none")
	if s.Aggregation.Get() != state.AggNone {
		t.Errorf("expected Aggregation none, got %v", s.Aggregation.Get())
	}
}

func TestButtonGroupMutatesTimeRange(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.ButtonGroup().Value.Set([]string{"7d"})
	if s.TimeRange.Get() != state.TimeRange7d {
		t.Errorf("expected TimeRange 7d, got %v", s.TimeRange.Get())
	}
	p.ButtonGroup().Value.Set([]string{"30d"})
	if s.TimeRange.Get() != state.TimeRange30d {
		t.Errorf("expected TimeRange 30d, got %v", s.TimeRange.Get())
	}
	p.ButtonGroup().Value.Set([]string{"all"})
	if s.TimeRange.Get() != state.TimeRangeAll {
		t.Errorf("expected TimeRange all, got %v", s.TimeRange.Get())
	}
}

func TestTurnDialMutatesSmoothing(t *testing.T) {
	s := testLiveState()
	p := NewInspectorPanel(s)
	p.TurnDial().Value.Set(75)
	if s.Smoothing.Get() != 75 {
		t.Errorf("expected Smoothing 75, got %f", s.Smoothing.Get())
	}
	p.TurnDial().Value.Set(25)
	if s.Smoothing.Get() != 25 {
		t.Errorf("expected Smoothing 25, got %f", s.Smoothing.Get())
	}
}

func TestChartTypeChangeDoesNotAffectVisibleRows(t *testing.T) {
	s := testLiveState()
	initial := s.VisibleRows.Get()
	s.ChartType.Set(state.ChartBar)
	after := s.VisibleRows.Get()
	if len(after) != len(initial) {
		t.Errorf("VisibleRows should remain unchanged after ChartType change: initial %d, after %d", len(initial), len(after))
	}
}

func TestOpacityChangeDoesNotAffectSeriesColorDirectly(t *testing.T) {
	s := testLiveState()
	s.SeriesColor.Set(gfx.Color{R: 1, G: 0, B: 0, A: 1})
	_ = NewInspectorPanel(s)
	s.Opacity.Set(0.9)
	col := s.SeriesColor.Get()
	if col.A != 1 {
		t.Errorf("SeriesColor alpha should remain 1 after Opacity change (chart canvas own alpha), got %f", col.A)
	}
}
