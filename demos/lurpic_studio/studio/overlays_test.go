package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestNewOverlays_createsAll(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	if ov == nil {
		t.Fatal("newOverlays returned nil")
	}
	if ov.dialog == nil {
		t.Fatal("no dialog")
	}
	if ov.exportToast == nil {
		t.Fatal("no export toast")
	}
	if ov.tooltip == nil {
		t.Fatal("no tooltip")
	}
	if ov.commandPalette == nil {
		t.Fatal("no command palette")
	}
	if ov.popupPalette == nil {
		t.Fatal("no popup palette")
	}
	if ov.navDrawer == nil {
		t.Fatal("no nav drawer")
	}
	if ov.commandReg == nil {
		t.Fatal("no command registry")
	}
}

func TestOverlays_allStartClosed(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	if ov.dialog.Open.Get() != false {
		t.Fatal("dialog should start closed")
	}
	if ov.exportToast.Open.Get() != false {
		t.Fatal("export toast should start closed")
	}
	if ov.tooltip.Open.Get() != false {
		t.Fatal("tooltip should start closed")
	}
	if ov.commandPalette.Open != false {
		t.Fatal("command palette should start closed")
	}
	if ov.popupPalette.Open.Get() != false {
		t.Fatal("popup palette should start closed")
	}
	if ov.navDrawer.Open.Get() != false {
		t.Fatal("nav drawer should start closed")
	}
}

func TestCommandRegistry_hasRegisteredCommands(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	cmds := ov.commandReg.Snapshot()
	if len(cmds) < 8 {
		t.Fatalf("expected at least 8 registered commands, got %d", len(cmds))
	}
}

func TestCommandRegistry_executeSwitchesChartType(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	_ = ov.commandReg

	found := false
	for _, cmd := range ov.commandReg.Snapshot() {
		if cmd.ID == "chart-bar" {
			found = true
			cmd.Execute()
			break
		}
	}
	if !found {
		t.Fatal("chart-bar command not found")
	}
	if as.ChartType.Get() != state.ChartBar {
		t.Fatalf("expected ChartBar after executing chart-bar command, got %q", as.ChartType.Get())
	}
}

func TestCommandRegistry_executeResetsFilters(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	as.SelectedSource.Set("NA")
	as.Aggregation.Set(state.AggSum)

	for _, cmd := range ov.commandReg.Snapshot() {
		if cmd.ID == "reset-all" {
			cmd.Execute()
			break
		}
	}
	if as.SelectedSource.Get() != "" {
		t.Fatal("SelectedSource should be empty after reset")
	}
	if as.Aggregation.Get() != state.AggNone {
		t.Fatal("Aggregation should be None after reset")
	}
}

func TestDialog_hasTwoActions(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	actions := ov.dialog.Actions.Get()
	if len(actions) != 2 {
		t.Fatalf("expected 2 dialog actions, got %d", len(actions))
	}
	if actions[0].Label != "Cancel" {
		t.Fatalf("first action expected 'Cancel', got %q", actions[0].Label)
	}
	if actions[1].Label != "Delete" {
		t.Fatalf("second action expected 'Delete', got %q", actions[1].Label)
	}
}

func TestNavDrawer_hasSection(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	if len(ov.navDrawer.Sections) != 1 {
		t.Fatalf("expected 1 nav drawer section, got %d", len(ov.navDrawer.Sections))
	}
	if len(ov.navDrawer.Sections[0].Items) != 4 {
		t.Fatalf("expected 4 nav drawer items, got %d", len(ov.navDrawer.Sections[0].Items))
	}
}

func TestDialog_typeCheck(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	d := marks.Describe(ov.dialog)
	if d.TypeName != "dialog" {
		t.Fatalf("expected dialog type, got %q", d.TypeName)
	}
}

func TestCommandPalette_typeCheck(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	d := marks.Describe(ov.commandPalette)
	if d.TypeName != "command_palette" {
		t.Fatalf("expected command_palette type, got %q", d.TypeName)
	}
}

func TestNavDrawer_typeCheck(t *testing.T) {
	as := state.NewAppState(nil)
	ov := newOverlays(as)
	d := marks.Describe(ov.navDrawer)
	if d.TypeName != "nav_drawer" {
		t.Fatalf("expected nav_drawer type, got %q", d.TypeName)
	}
}

func TestRootHasOverlays(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)
	if root.overlays == nil {
		t.Fatal("root has no overlays")
	}
	if root.overlays.commandReg == nil {
		t.Fatal("root has no command registry")
	}
}
