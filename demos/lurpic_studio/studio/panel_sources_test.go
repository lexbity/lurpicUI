package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func testAppState() *state.AppState {
	rows := []dataset.Row{
		{Revenue: 100, Users: 10, Region: "NA"},
		{Revenue: 200, Users: 20, Region: "EU"},
		{Revenue: 300, Users: 30, Region: "APAC"},
		{Revenue: 400, Users: 40, Region: "LATAM"},
	}
	return state.NewAppState(rows)
}

func TestSourcesPanelConstructs(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	if p == nil {
		t.Fatal("NewSourcesPanel returned nil")
	}
	if p.NavRail() == nil {
		t.Fatal("SourcesPanel has no NavRail")
	}
	if p.TreeNav() == nil {
		t.Fatal("SourcesPanel has no TreeNavigator")
	}
	if len(p.ListItems()) != 4 {
		t.Fatalf("expected 4 list items, got %d", len(p.ListItems()))
	}
}

func TestSourcesPanelNavRailItems(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	rail := p.NavRail()
	if len(rail.Items) != 3 {
		t.Fatalf("expected 3 nav rail items, got %d", len(rail.Items))
	}
	if rail.Items[0].Key != "sources" {
		t.Errorf("expected first nav item key 'sources', got %q", rail.Items[0].Key)
	}
}

func TestSourcesPanelTreeNodes(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	tree := p.TreeNav()
	data := tree.Data.Get()
	if len(data) != 4 {
		t.Fatalf("expected 4 tree nodes, got %d", len(data))
	}
	expectedLabels := map[string]string{
		"NA":   "North America",
		"EU":   "Europe",
		"APAC": "APAC",
		"LATAM": "LATAM",
	}
	for _, node := range data {
		expected, ok := expectedLabels[node.Key]
		if !ok {
			t.Errorf("unexpected tree node key: %q", node.Key)
			continue
		}
		if node.Label != expected {
			t.Errorf("node %q: expected label %q, got %q", node.Key, expected, node.Label)
		}
	}
}

func TestSourcesPanelListItems(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	items := p.ListItems()
	if len(items) != 4 {
		t.Fatalf("expected 4 list items, got %d", len(items))
	}
	expectedLabels := []string{"North America", "Europe", "APAC", "LATAM"}
	for i, item := range items {
		if item.Label.Get() != expectedLabels[i] {
			t.Errorf("list item %d: expected label %q, got %q", i, expectedLabels[i], item.Label.Get())
		}
	}
}

func TestSourcesPanelListItemClickSetsSelectedSource(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	items := p.ListItems()
	if s.SelectedSource.Get() != "" {
		t.Fatalf("expected empty SelectedSource initially, got %q", s.SelectedSource.Get())
	}
	items[0].Activated.Emit(struct{}{})
	if s.SelectedSource.Get() != "NA" {
		t.Errorf("expected SelectedSource 'NA' after clicking first item, got %q", s.SelectedSource.Get())
	}
}

func TestSourcesPanelListItemClickSetsEU(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	items := p.ListItems()
	items[1].Activated.Emit(struct{}{})
	if s.SelectedSource.Get() != "EU" {
		t.Errorf("expected SelectedSource 'EU' after clicking second item, got %q", s.SelectedSource.Get())
	}
}

func TestSourcesPanelStatusLightConnected(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	s.Connection.Set(state.ConnConnected)
	if p.StatusLight().Label.Get() != "Connected" {
		t.Errorf("expected 'Connected', got %q", p.StatusLight().Label.Get())
	}
}

func TestSourcesPanelStatusLightDisconnected(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	s.Connection.Set(state.ConnDisconnected)
	if p.StatusLight().Label.Get() != "Disconnected" {
		t.Errorf("expected 'Disconnected', got %q", p.StatusLight().Label.Get())
	}
}

func TestSourcesPanelStatusLightConnecting(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	s.Connection.Set(state.ConnConnecting)
	if p.StatusLight().Label.Get() != "Connecting..." {
		t.Errorf("expected 'Connecting...', got %q", p.StatusLight().Label.Get())
	}
}

func TestSourcesPanelUpdateBadge(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	p.UpdateBadge("NA")
	if p.Badge().Label.Get() != "1" {
		t.Errorf("expected badge '1' for NA, got %q", p.Badge().Label.Get())
	}
	p.UpdateBadge("")
	if p.Badge().Label.Get() != "0" {
		t.Errorf("expected badge '0' for empty source, got %q", p.Badge().Label.Get())
	}
}

func TestSourcesPanelUpdateSelection(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	p.UpdateSelection("NA")
	for _, li := range p.ListItems() {
		if li.Label.Get() == "North America" && !li.Selected.Get() {
			t.Error("North America should be selected")
		}
		if li.Label.Get() != "North America" && li.Selected.Get() {
			t.Errorf("%s should not be selected", li.Label.Get())
		}
	}
}

func TestSourcesPanelMeasures(t *testing.T) {
	s := testAppState()
	p := NewSourcesPanel(s)
	role := p.Base().LayoutRole()
	if role == nil {
		t.Fatal("SourcesPanel has no LayoutRole")
	}
	result := role.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 600}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Errorf("expected measurable size, got %v", result.Size)
	}
}
