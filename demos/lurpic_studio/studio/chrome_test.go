package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestChromeRibbonSections(t *testing.T) {
	ribbon := newChromeRibbon()
	desc := marks.Describe(ribbon)
	if desc.Family != "action" || desc.TypeName != "ribbon" {
		t.Errorf("expected action/ribbon, got %s/%s", desc.Family, desc.TypeName)
	}
	if len(ribbon.Sections) != 4 {
		t.Fatalf("expected 4 ribbon sections, got %d", len(ribbon.Sections))
	}
	expected := []string{"home", "insert", "chart", "view"}
	for i, s := range ribbon.Sections {
		if s.Key != expected[i] {
			t.Errorf("section %d: expected key %q, got %q", i, expected[i], s.Key)
		}
		if len(s.Toolbars) == 0 {
			t.Errorf("section %q has no toolbars", s.Key)
		}
	}
}

func TestChromeToolbarGroups(t *testing.T) {
	toolbar := newChromeToolbar()
	if len(toolbar.Groups) != 2 {
		t.Fatalf("expected 2 toolbar groups, got %d", len(toolbar.Groups))
	}
	if toolbar.Overflow == nil {
		t.Fatal("toolbar should have overflow menu")
	}
	if len(toolbar.Overflow.Entries) != 3 {
		t.Errorf("expected 3 overflow entries, got %d", len(toolbar.Overflow.Entries))
	}
}

func TestChromeSplitButtonItems(t *testing.T) {
	split := newChromeSplitButton()
	desc := marks.Describe(split)
	if desc.TypeName != "split_button" {
		t.Errorf("expected split_button, got %s", desc.TypeName)
	}
	items := split.Items.Get()
	if len(items) != 3 {
		t.Fatalf("expected 3 split button items, got %d", len(items))
	}
	if items[0].Key != "export_csv" {
		t.Errorf("first item key: expected export_csv, got %q", items[0].Key)
	}
}

func TestChromeMenuButtonEntries(t *testing.T) {
	menu := newChromeMenuButton()
	if len(menu.Entries) != 3 {
		t.Fatalf("expected 3 menu entries, got %d", len(menu.Entries))
	}
	if !menu.Entries[2].Destructive {
		t.Error("delete entry should be destructive")
	}
}

func TestChromeBreadcrumbsItems(t *testing.T) {
	bread := newChromeBreadcrumbs()
	if len(bread.Items) != 3 {
		t.Fatalf("expected 3 breadcrumb items, got %d", len(bread.Items))
	}
	if bread.Items[0].Label != "Sources" {
		t.Errorf("first breadcrumb: expected Sources, got %q", bread.Items[0].Label)
	}
}

func TestChromeActionBarActions(t *testing.T) {
	bar := newChromeActionBar()
	desc := marks.Describe(bar)
	if desc.TypeName != "action_bar" {
		t.Errorf("expected action_bar, got %s", desc.TypeName)
	}
	if bar.Actions.Get() == nil {
		t.Fatal("Actions binding returned nil")
	}
	actions := bar.Actions.Get()
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
}

func TestChromeIconButtonsCount(t *testing.T) {
	btns := newChromeIconButtons()
	if len(btns) != 3 {
		t.Fatalf("expected 3 icon buttons, got %d", len(btns))
	}
}

func TestNewChromeBar(t *testing.T) {
	s := state.NewAppState(nil)
	chrome := NewChromeBar(s, gfx.Size{W: 1280, H: 800})
	if chrome == nil {
		t.Fatal("NewChromeBar returned nil")
	}
	if chrome.ribbon == nil {
		t.Fatal("ChromeBar has no ribbon")
	}
	if chrome.toolbar == nil {
		t.Fatal("ChromeBar has no toolbar")
	}
}

func TestChromeBarTotalHeight(t *testing.T) {
	s := state.NewAppState(nil)
	chrome := NewChromeBar(s, gfx.Size{W: 1280, H: 800})
	h := chrome.TotalHeight()
	if h <= 0 {
		t.Errorf("expected positive total height, got %f", h)
	}
	if h < 80 || h > 120 {
		t.Errorf("expected total height around 88, got %f", h)
	}
}

func TestChromeRibbonActivated(t *testing.T) {
	ribbon := newChromeRibbon()
	activatedCount := 0
	activatedIndex := -1
	ribbon.Activated.Subscribe(func(index int) {
		activatedCount++
		activatedIndex = index
	})
	ribbon.Activated.Emit(2)
	if activatedCount != 1 {
		t.Errorf("expected 1 activation, got %d", activatedCount)
	}
	if activatedIndex != 2 {
		t.Errorf("expected activation index 2, got %d", activatedIndex)
	}
	ribbon.Activated.Emit(0)
	if activatedCount != 2 {
		t.Errorf("expected 2 activations after second emit, got %d", activatedCount)
	}
	if activatedIndex != 0 {
		t.Errorf("expected activation index 0 after second emit, got %d", activatedIndex)
	}
}

func TestChromeSplitButtonItemsAreExportActions(t *testing.T) {
	split := newChromeSplitButton()
	items := split.Items.Get()
	exportKeys := map[string]bool{"export_csv": true, "export_png": true, "export_pdf": true}
	for _, item := range items {
		if !exportKeys[item.Key] {
			t.Errorf("unexpected item key: %q", item.Key)
		}
	}
}
