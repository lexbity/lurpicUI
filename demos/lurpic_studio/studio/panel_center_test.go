package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/text"
)

func testCenterFonts(t *testing.T) *text.FontRegistry {
	t.Helper()
	return testkit.TestFontRegistry(t)
}

func testRowsForCenter() []dataset.Row {
	rows := make([]dataset.Row, 12)
	for i := 0; i < 12; i++ {
		rows[i] = dataset.Row{
			Revenue: float64(1000 + i*100),
			Users:   float64(100 + i*10),
			Region:  []string{"NA", "EU", "APAC", "LATAM"}[i%4],
		}
	}
	return rows
}

func TestCenterPanelConstructs(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	if p == nil {
		t.Fatal("NewCenterPanel returned nil")
	}
	if p.Tabs() == nil {
		t.Fatal("CenterPanel has no Tabs")
	}
	if p.Table() == nil {
		t.Fatal("CenterPanel has no Table")
	}
	if p.Pagination() == nil {
		t.Fatal("CenterPanel has no Pagination")
	}
}

func TestCenterPanelTabsItems(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	tabs := p.Tabs()
	if len(tabs.Items) != 2 {
		t.Fatalf("expected 2 tab items, got %d", len(tabs.Items))
	}
	if tabs.Items[0].Key != "data" || tabs.Items[1].Key != "chart" {
		t.Errorf("expected tabs 'data' and 'chart', got %q and %q", tabs.Items[0].Key, tabs.Items[1].Key)
	}
}

func TestCenterPanelTableHasColumns(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	data := p.Table().Data.Get()
	if len(data.Columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(data.Columns))
	}
	expected := []string{"date", "revenue", "users", "region"}
	for i, col := range data.Columns {
		if col.Key != expected[i] {
			t.Errorf("column %d: expected key %q, got %q", i, expected[i], col.Key)
		}
	}
}

func TestCenterPanelTableHasRows(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	data := p.Table().Data.Get()
	if len(data.Rows) != 10 {
		t.Fatalf("expected 10 rows (page 1, PageSize=10), got %d", len(data.Rows))
	}
}

func TestCenterPanelPaginationHasItems(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	_ = s.VisibleRows.Get()
	p := NewCenterPanel(s, testCenterFonts(t))
	pag := p.Pagination()
	if len(pag.Items) != 2 {
		t.Fatalf("expected 2 pagination items (12 rows / 10 per page = ceil(1.2) = 2), got %d", len(pag.Items))
	}
}

func TestCenterPanelTabSwitchSetsActiveTab(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	if s.ActiveTab.Get() != state.TabData {
		t.Fatalf("expected initial ActiveTab Data, got %v", s.ActiveTab.Get())
	}
	p.Tabs().Activated.Emit(1)
	if s.ActiveTab.Get() != state.TabChart {
		t.Errorf("expected ActiveTab Chart after clicking second tab, got %v", s.ActiveTab.Get())
	}
	p.Tabs().Activated.Emit(0)
	if s.ActiveTab.Get() != state.TabData {
		t.Errorf("expected ActiveTab Data after clicking first tab, got %v", s.ActiveTab.Get())
	}
}

func TestCenterPanelPaginationNext(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	if s.Page.Get() != 1 {
		t.Fatalf("expected initial Page 1, got %d", s.Page.Get())
	}
	p := NewCenterPanel(s, testCenterFonts(t))
	p.Pagination().Activated.Emit(1)
	if s.Page.Get() != 2 {
		t.Errorf("expected Page 2 after pagination activated with index 1, got %d", s.Page.Get())
	}
}

func TestCenterPanelSourceChangeUpdatesTable(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))

	s.SelectedSource.Set("NA")
	data := p.Table().Data.Get()
	for _, row := range data.Rows {
		if len(row.Cells) < 4 || row.Cells[3] != "NA" {
			t.Errorf("expected all rows to be NA after selecting NA source, got one with region %v", row.Cells)
		}
	}
}

func TestCenterPanelMeasures(t *testing.T) {
	s := state.NewAppState(testRowsForCenter())
	p := NewCenterPanel(s, testCenterFonts(t))
	result := p.Base().LayoutRole().Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 800, H: 600}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Errorf("expected measurable size, got %v", result.Size)
	}
}
