package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/runtime"
)

func testOverlayState() *state.AppState {
	return state.NewAppState([]dataset.Row{{Revenue: 100, Users: 10, Region: "NA"}})
}

func TestDialogConstructs(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	if o.Dialog() == nil {
		t.Fatal("OverlayHost has no Dialog")
	}
	if o.Dialog().Actions.Get() == nil {
		t.Fatal("Dialog has no actions")
	}
	actions := o.Dialog().Actions.Get()
	if len(actions) != 2 {
		t.Fatalf("expected 2 dialog actions, got %d", len(actions))
	}
	if actions[0].Label != "Cancel" || actions[1].Label != "Delete" {
		t.Errorf("unexpected dialog action labels: %q, %q", actions[0].Label, actions[1].Label)
	}
}

func TestDialogCancelDoesNotMutateState(t *testing.T) {
	s := testOverlayState()
	s.SelectedSource.Set("NA")
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	o.Dialog().Actioned.Emit(0)
	if s.SelectedSource.Get() != "NA" {
		t.Error("SelectedSource should remain NA after Cancel")
	}
}

func TestDialogConfirmClearsSelection(t *testing.T) {
	s := testOverlayState()
	s.SelectedSource.Set("NA")
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	o.Dialog().Actioned.Emit(1)
	if s.SelectedSource.Get() != "" {
		t.Errorf("expected empty SelectedSource after Delete, got %q", s.SelectedSource.Get())
	}
}

func TestDialogDismissClearsOverlayState(t *testing.T) {
	s := testOverlayState()
	s.OverlayState.Set(state.OverlayDialog)
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	o.Dialog().Dismissed.Emit(struct{}{})
	if s.OverlayState.Get() != state.OverlayNone {
		t.Errorf("expected OverlayNone after dismiss, got %v", s.OverlayState.Get())
	}
}

func TestNotificationConstructs(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	if o.Notification() == nil {
		t.Fatal("OverlayHost has no Notification")
	}
	if o.Notification().Title.Get() != "Export Complete" {
		t.Errorf("expected notification title 'Export Complete', got %q", o.Notification().Title.Get())
	}
}

func TestShowNotification(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	o.ShowNotification()
	if !o.Notification().Open.Get() {
		t.Error("Notification should be open after ShowNotification")
	}
}

func TestCommandPaletteConstructs(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	if o.CommandPalette() == nil {
		t.Fatal("OverlayHost has no CommandPalette")
	}
}

func TestCommandRegistration(t *testing.T) {
	reg := runtime.NewCommandRegistry()
	s := testOverlayState()
	registerCommands(reg, s)
	snapshot := reg.Snapshot()
	if len(snapshot) != 7 {
		t.Fatalf("expected 7 registered commands, got %d", len(snapshot))
	}
}

func TestCommandPaletteRunsBarCommand(t *testing.T) {
	reg := runtime.NewCommandRegistry()
	s := testOverlayState()
	registerCommands(reg, s)
	reg.Execute("chart_bar")
	if s.ChartType.Get() != state.ChartBar {
		t.Errorf("expected ChartType bar after executing chart_bar command, got %v", s.ChartType.Get())
	}
}

func TestCommandPaletteRunsLineCommand(t *testing.T) {
	reg := runtime.NewCommandRegistry()
	s := testOverlayState()
	s.ChartType.Set(state.ChartBar)
	registerCommands(reg, s)
	reg.Execute("chart_line")
	if s.ChartType.Get() != state.ChartLine {
		t.Errorf("expected ChartType line after executing chart_line command, got %v", s.ChartType.Get())
	}
}

func TestCommandPaletteResetView(t *testing.T) {
	reg := runtime.NewCommandRegistry()
	s := testOverlayState()
	s.SelectedSource.Set("NA")
	s.Page.Set(3)
	registerCommands(reg, s)
	reg.Execute("reset_view")
	if s.SelectedSource.Get() != "" {
		t.Errorf("expected empty SelectedSource after reset, got %q", s.SelectedSource.Get())
	}
	if s.Page.Get() != 1 {
		t.Errorf("expected Page 1 after reset, got %d", s.Page.Get())
	}
}

func TestCommandPaletteToggleGrid(t *testing.T) {
	reg := runtime.NewCommandRegistry()
	s := testOverlayState()
	s.ShowGrid.Set(false)
	registerCommands(reg, s)
	reg.Execute("show_grid")
	if !s.ShowGrid.Get() {
		t.Error("expected ShowGrid true after toggle")
	}
	reg.Execute("show_grid")
	if s.ShowGrid.Get() {
		t.Error("expected ShowGrid false after second toggle")
	}
}

func TestNavDrawerConstructs(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	if o.NavDrawer() == nil {
		t.Fatal("OverlayHost has no NavDrawer")
	}
	if len(o.NavDrawer().Sections) != 1 {
		t.Fatalf("expected 1 nav drawer section, got %d", len(o.NavDrawer().Sections))
	}
	if len(o.NavDrawer().Sections[0].Items) != 5 {
		t.Fatalf("expected 5 nav drawer items, got %d", len(o.NavDrawer().Sections[0].Items))
	}
}

func TestOverlayHostMeasures(t *testing.T) {
	s := testOverlayState()
	reg := runtime.NewCommandRegistry()
	o := NewOverlayHost(s, reg)
	result := o.Base().LayoutRole().Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if result.Size.W != 1280 || result.Size.H != 800 {
		t.Errorf("expected 1280x800, got %v", result.Size)
	}
}
