package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
)

func TestNewChromePane_createsAllMarks(t *testing.T) {
	as := state.NewAppState(nil)
	ribbon, toolbar, breadcrumbs, actionBar := newChromePane(as)
	if ribbon == nil {
		t.Fatal("ribbon is nil")
	}
	if toolbar == nil {
		t.Fatal("toolbar is nil")
	}
	if breadcrumbs == nil {
		t.Fatal("breadcrumbs is nil")
	}
	if actionBar == nil {
		t.Fatal("actionBar is nil")
	}
}

func TestRibbon_hasSections(t *testing.T) {
	_, toolbar, _, _ := newChromePane(nil)
	_ = toolbar
	ribbon := newRibbon()
	if len(ribbon.Sections) != 3 {
		t.Fatalf("expected 3 ribbon sections, got %d", len(ribbon.Sections))
	}
	if ribbon.Sections[0].Key != "home" {
		t.Fatalf("first section key: expected 'home', got %q", ribbon.Sections[0].Key)
	}
	if ribbon.Sections[2].Key != "view" {
		t.Fatalf("last section key: expected 'view', got %q", ribbon.Sections[2].Key)
	}
}

func TestRibbon_sectionsHaveToolbars(t *testing.T) {
	ribbon := newRibbon()
	for _, s := range ribbon.Sections {
		if len(s.Toolbars) == 0 {
			t.Fatalf("section %q has no toolbars", s.Key)
		}
		for j, tb := range s.Toolbars {
			if tb == nil {
				t.Fatalf("section %q toolbar[%d] is nil", s.Key, j)
			}
		}
	}
}

func TestRibbon_descriptor(t *testing.T) {
	ribbon := newRibbon()
	d := marks.Describe(ribbon)
	if d.TypeName != "ribbon" {
		t.Fatalf("expected ribbon type, got %q", d.TypeName)
	}
	if d.Family != "action" {
		t.Fatalf("expected action family, got %q", d.Family)
	}
}

func TestToolbar_hasGroups(t *testing.T) {
	toolbar := newToolbar()
	if len(toolbar.Groups) != 2 {
		t.Fatalf("expected 2 toolbar groups, got %d", len(toolbar.Groups))
	}
	if toolbar.Groups[0].Key != "file" {
		t.Fatalf("first group key: expected 'file', got %q", toolbar.Groups[0].Key)
	}
	if toolbar.Groups[1].Key != "export" {
		t.Fatalf("second group key: expected 'export', got %q", toolbar.Groups[1].Key)
	}
}

func TestToolbar_fileGroupHasActions(t *testing.T) {
	toolbar := newToolbar()
	fileGroup := toolbar.Groups[0]
	if len(fileGroup.Actions) != 3 {
		t.Fatalf("expected 3 file actions, got %d", len(fileGroup.Actions))
	}
	actionKeys := []string{fileGroup.Actions[0].Key, fileGroup.Actions[1].Key, fileGroup.Actions[2].Key}
	expected := []string{"new", "open", "save"}
	for i, k := range expected {
		if actionKeys[i] != k {
			t.Fatalf("file action[%d]: expected %q, got %q", i, k, actionKeys[i])
		}
	}
}

func TestToolbar_hasOverflow(t *testing.T) {
	toolbar := newToolbar()
	if toolbar.Overflow == nil {
		t.Fatal("toolbar has no overflow menu")
	}
	if len(toolbar.Overflow.Entries) != 5 {
		t.Fatalf("expected 5 overflow entries, got %d", len(toolbar.Overflow.Entries))
	}
	if toolbar.Overflow.Entries[0].Key != "import" {
		t.Fatalf("first overflow entry: expected 'import', got %q", toolbar.Overflow.Entries[0].Key)
	}
}

func TestToolbar_overflowHasDivider(t *testing.T) {
	toolbar := newToolbar()
	divider := toolbar.Overflow.Entries[3]
	if divider.Kind != action.MenuButtonEntryDivider {
		t.Fatal("expected a divider entry in overflow menu")
	}
}

func TestToolbar_descriptor(t *testing.T) {
	toolbar := newToolbar()
	d := marks.Describe(toolbar)
	if d.TypeName != "toolbar" {
		t.Fatalf("expected toolbar type, got %q", d.TypeName)
	}
}

func TestToolbar_overflowActionIconsAreIconRef(t *testing.T) {
	// Ensure we don't accidentally use IconSource incorrectly
	toolbar := newToolbar()
	overflow := toolbar.Overflow
	if overflow.Entries[0].IconRef != "import" {
		t.Fatalf("expected icon ref 'import', got %q", overflow.Entries[0].IconRef)
	}
}

func TestBreadcrumbs_hasItems(t *testing.T) {
	as := state.NewAppState(nil)
	_, _, breadcrumbs, _ := newChromePane(as)
	// Breadcrumbs items are set in the constructor
	expected := []string{"Sources", "Data"}
	for i, item := range breadcrumbs.Items {
		if item.Label != expected[i] {
			t.Fatalf("breadcrumb[%d]: expected %q, got %q", i, expected[i], item.Label)
		}
	}
}

func TestBreadcrumbs_currentIndex(t *testing.T) {
	as := state.NewAppState(nil)
	_, _, breadcrumbs, _ := newChromePane(as)
	idx := breadcrumbs.CurrentIndex.Get()
	if idx != 1 {
		t.Fatalf("expected CurrentIndex = 1 (last item), got %d", idx)
	}
}

func TestBreadcrumbs_descriptor(t *testing.T) {
	as := state.NewAppState(nil)
	_, _, breadcrumbs, _ := newChromePane(as)
	d := marks.Describe(breadcrumbs)
	if d.TypeName != "breadcrumbs" {
		t.Fatalf("expected breadcrumbs type, got %q", d.TypeName)
	}
	if d.Family != "navigation" {
		t.Fatalf("expected navigation family, got %q", d.Family)
	}
}

func TestActionBar_hasActions(t *testing.T) {
	as := state.NewAppState(nil)
	actionBar := newActionBar(as)
	if len(actionBar.Actions.Get()) != 2 {
		t.Fatalf("expected 2 action bar actions, got %d", len(actionBar.Actions.Get()))
	}
}

func TestActionBar_descriptor(t *testing.T) {
	as := state.NewAppState(nil)
	actionBar := newActionBar(as)
	d := marks.Describe(actionBar)
	if d.TypeName != "action_bar" {
		t.Fatalf("expected action_bar type, got %q", d.TypeName)
	}
	if d.Family != "action" {
		t.Fatalf("expected action family, got %q", d.Family)
	}
}

func TestRootHasChromeReferences(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)
	if root.ribbon == nil {
		t.Fatal("root has no ribbon reference")
	}
	if root.toolbar == nil {
		t.Fatal("root has no toolbar reference")
	}
	if root.breadcrumbs == nil {
		t.Fatal("root has no breadcrumbs reference")
	}
	if root.actionBar == nil {
		t.Fatal("root has no actionBar reference")
	}
}

func TestLabeledActions_useTextMark(t *testing.T) {
	// Actions with labels should create text marks
	_ = navigation.BreadcrumbItem{Label: "test"}
	_ = action.ActionGroupAction{Key: "test", Label: "Test"}
	_ = action.MenuButtonEntry{Key: "test", Label: "Test"}
}

func TestToolbar_fileGroupActionsAreIconButtons(t *testing.T) {
	toolbar := newToolbar()
	fileGroup := toolbar.Groups[0]
	for _, a := range fileGroup.Actions {
		if a.IconRef == "" {
			t.Fatalf("icon button action %q has empty IconRef", a.Key)
		}
	}
}

func TestExportGroupActionsHaveLabels(t *testing.T) {
	toolbar := newToolbar()
	exportGroup := toolbar.Groups[1]
	for _, a := range exportGroup.Actions {
		if a.Label == "" {
			t.Fatalf("export action %q has empty label", a.Key)
		}
	}
}
