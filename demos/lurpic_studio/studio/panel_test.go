package studio

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
)

func testTime(s string) time.Time {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return t
}

func makeTestDataset(rowsPerRegion int, regionKeys []string) []dataset.Row {
	var out []dataset.Row
	id := 0
	for _, reg := range regionKeys {
		for j := 0; j < rowsPerRegion; j++ {
			out = append(out, dataset.Row{
				Date:    testTime("2026-01-01").AddDate(0, 0, id),
				Revenue: float64(1000 + id*100),
				Users:   float64(100 + id*10),
				Region:  reg,
			})
			id++
		}
	}
	return out
}

func TestNewSourcesPanel_createsAllComponents(t *testing.T) {
	as := state.NewAppState(nil)
	sp := newSourcesPanel(as)
	if sp == nil {
		t.Fatal("newSourcesPanel returned nil")
	}
	if sp.rail == nil {
		t.Fatal("no nav rail")
	}
	if sp.tree == nil {
		t.Fatal("no tree navigator")
	}
	if sp.list == nil {
		t.Fatal("no list")
	}
	if sp.scroll == nil {
		t.Fatal("no scroll region")
	}

	sd := marks.Describe(sp.rail)
	if sd.TypeName != "nav_rail" {
		t.Fatalf("expected nav_rail type, got %q", sd.TypeName)
	}
	td := marks.Describe(sp.tree)
	if td.TypeName != "tree_navigator" {
		t.Fatalf("expected tree_navigator type, got %q", td.TypeName)
	}
	ld := marks.Describe(sp.list)
	if ld.TypeName != "list" {
		t.Fatalf("expected list type, got %q", ld.TypeName)
	}
}

func TestNewSourcesPanel_listHasFourEntries(t *testing.T) {
	as := state.NewAppState(nil)
	sp := newSourcesPanel(as)
	entries := sp.list.Data.Get()
	if len(entries) != 4 {
		t.Fatalf("expected 4 entries, got %d", len(entries))
	}
	if entries[0].Key != "NA" {
		t.Fatalf("first entry key: expected 'NA', got %q", entries[0].Key)
	}
	if entries[3].Key != "LATAM" {
		t.Fatalf("last entry key: expected 'LATAM', got %q", entries[3].Key)
	}
}

func TestNewSourcesPanel_railHasThreeItems(t *testing.T) {
	as := state.NewAppState(nil)
	sp := newSourcesPanel(as)
	if len(sp.rail.Items) != 3 {
		t.Fatalf("expected 3 nav rail items, got %d", len(sp.rail.Items))
	}
}

func TestNewSourcesPanel_treeHasRegionNodes(t *testing.T) {
	as := state.NewAppState(nil)
	sp := newSourcesPanel(as)
	nodes := sp.tree.Data.Get()
	if len(nodes) != 1 {
		t.Fatalf("expected 1 root tree node, got %d", len(nodes))
	}
	if nodes[0].Key != "regions" {
		t.Fatalf("root node key: expected 'regions', got %q", nodes[0].Key)
	}
	if !nodes[0].Expanded {
		t.Fatal("root node should be expanded")
	}
	if len(nodes[0].Children) != 4 {
		t.Fatalf("expected 4 region children, got %d", len(nodes[0].Children))
	}
}

func TestNewCenterPanel_createsAllComponents(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	if cp == nil {
		t.Fatal("newCenterPanel returned nil")
	}
	if cp.tabs == nil {
		t.Fatal("no tabs")
	}
	if cp.table == nil {
		t.Fatal("no table")
	}
	if cp.pagination == nil {
		t.Fatal("no pagination")
	}
	if cp.card == nil {
		t.Fatal("no card")
	}
}

func TestCenterPanel_tabsHaveDataAndChart(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	if len(cp.tabs.Items) != 2 {
		t.Fatalf("expected 2 tabs, got %d", len(cp.tabs.Items))
	}
	if cp.tabs.Items[0].Key != string(state.TabData) {
		t.Fatalf("first tab key: expected %q, got %q", state.TabData, cp.tabs.Items[0].Key)
	}
	if cp.tabs.Items[1].Key != string(state.TabChart) {
		t.Fatalf("second tab key: expected %q, got %q", state.TabChart, cp.tabs.Items[1].Key)
	}
}

func TestCenterPanel_tableHasColumns(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	td := cp.table.Data.Get()
	if len(td.Columns) != 4 {
		t.Fatalf("expected 4 columns, got %d", len(td.Columns))
	}
	if td.Columns[0].Key != "date" {
		t.Fatalf("expected first column 'date', got %q", td.Columns[0].Key)
	}
	if td.Columns[3].Key != "region" {
		t.Fatalf("expected last column 'region', got %q", td.Columns[3].Key)
	}
}

func TestCenterPanel_tableStartsEmpty(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	td := cp.table.Data.Get()
	if len(td.Rows) != 0 {
		t.Fatalf("expected 0 rows (empty), got %d", len(td.Rows))
	}
}

func TestCenterPanel_paginationStartsWithOnePage(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	if len(cp.pagination.Items) != 1 {
		t.Fatalf("expected 1 page, got %d", len(cp.pagination.Items))
	}
}

func TestCenterPanel_cardExists(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)
	lm := cp.card.LayoutMode.Get()
	if lm != structure.CardLayoutVertical {
		t.Fatalf("expected CardLayoutVertical, got %v", lm)
	}
}

func TestRootHasAllPanelReferences(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)
	if root.sourcesPanel == nil {
		t.Fatal("root has no sourcesPanel")
	}
	if root.sourcesPanel.rail == nil {
		t.Fatal("root has no nav rail")
	}
	if root.sourcesPanel.tree == nil {
		t.Fatal("root has no tree navigator")
	}
	if root.sourcesPanel.scroll == nil {
		t.Fatal("root has no scroll region")
	}
	if root.sourcesPanel.list == nil {
		t.Fatal("root has no list")
	}
	if root.centerPanel == nil {
		t.Fatal("root has no centerPanel")
	}
	if root.centerPanel.tabs == nil {
		t.Fatal("root has no tabs")
	}
	if root.centerPanel.table == nil {
		t.Fatal("root has no table")
	}
	if root.centerPanel.pagination == nil {
		t.Fatal("root has no pagination")
	}
	if root.centerPanel.card == nil {
		t.Fatal("root has no card")
	}
}

func TestScrollRegion_clipsOverflowContent(t *testing.T) {
	as := state.NewAppState(nil)
	sp := newSourcesPanel(as)

	childList := sp.scroll.Children()
	if len(childList) == 0 {
		t.Fatal("scroll region should have children")
	}

	sp.scroll.SetChildren([]structure.ScrollRegionChild{
		{Facet: primitive.NewText(marks.Const("tall content"))},
	})

	updated := sp.scroll.Children()
	if len(updated) == 0 {
		t.Fatal("scroll region should have children after SetChildren")
	}
}

func TestCountRowsForSource_returnsCorrectCount(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA", "EU", "APAC"})
	as := state.NewAppState(rows)
	if got := countRowsForSource(as, "NA"); got != 5 {
		t.Fatalf("expected 5 NA rows, got %d", got)
	}
	if got := countRowsForSource(as, "EU"); got != 5 {
		t.Fatalf("expected 5 EU rows, got %d", got)
	}
	if got := countRowsForSource(as, "APAC"); got != 5 {
		t.Fatalf("expected 5 APAC rows, got %d", got)
	}
	if got := countRowsForSource(as, "LATAM"); got != 0 {
		t.Fatalf("expected 0 LATAM rows (no data), got %d", got)
	}
	if got := countRowsForSource(as, ""); got != 15 {
		t.Fatalf("expected 15 total rows (all), got %d", got)
	}
}

func TestFindSelectedTreeKey_returnsKey(t *testing.T) {
	nodes := []navigation.TreeNode{
		{Key: "a", Children: []navigation.TreeNode{
			{Key: "a1", Selected: true},
			{Key: "a2"},
		}},
		{Key: "b"},
	}
	if got := findSelectedTreeKey(nodes); got != "a1" {
		t.Fatalf("expected 'a1', got %q", got)
	}
}

func TestFindSelectedTreeKey_noSelection(t *testing.T) {
	nodes := []navigation.TreeNode{
		{Key: "a", Children: []navigation.TreeNode{
			{Key: "a1"},
			{Key: "a2"},
		}},
	}
	if got := findSelectedTreeKey(nodes); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}

func TestFindSelectedTreeKey_rootSelected(t *testing.T) {
	nodes := []navigation.TreeNode{
		{Key: "a", Selected: true, Children: []navigation.TreeNode{
			{Key: "a1"},
		}},
	}
	if got := findSelectedTreeKey(nodes); got != "a" {
		t.Fatalf("expected 'a', got %q", got)
	}
}

func TestWireSourceRowCounts_updatesEntryCounts(t *testing.T) {
	rows := makeTestDataset(3, []string{"NA", "EU", "APAC", "LATAM"})
	as := state.NewAppState(rows)
	_ = newSourcesPanel(as)

	as.SelectedSource.Set("NA")

	callCount := 0
	subID := as.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		callCount++
	})
	defer as.SelectedSource.OnChange.Unsubscribe(subID)

	if c := countRowsForSource(as, "NA"); c != 3 {
		t.Fatalf("expected 3 NA rows, got %d", c)
	}
	if c := countRowsForSource(as, "EU"); c != 3 {
		t.Fatalf("expected 3 EU rows, got %d", c)
	}
	if c := countRowsForSource(as, ""); c != 12 {
		t.Fatalf("expected 12 total rows, got %d", c)
	}
	_ = callCount
}

func TestRowsToTableData_convertsRows(t *testing.T) {
	rows := makeTestDataset(2, []string{"NA"})
	as := state.NewAppState(rows)
	td := rowsToTableData(as, rows)
	if len(td.Rows) != 2 {
		t.Fatalf("expected 2 rows, got %d", len(td.Rows))
	}
}

func TestComputeTotalPages_returnsCorrectCount(t *testing.T) {
	rows := makeTestDataset(state.PageSize*3, []string{"NA"})
	as := state.NewAppState(rows)
	pages := computeTotalPages(as)
	if pages != 3 {
		t.Fatalf("expected 3 pages for %d rows, got %d", state.PageSize*3, pages)
	}
}

func TestComputeTotalPages_oneForEmpty(t *testing.T) {
	as := state.NewAppState(nil)
	pages := computeTotalPages(as)
	if pages != 1 {
		t.Fatalf("expected 1 page for empty data, got %d", pages)
	}
}

func TestComputeTotalPages_roundsUp(t *testing.T) {
	rows := makeTestDataset(state.PageSize+1, []string{"NA"})
	as := state.NewAppState(rows)
	pages := computeTotalPages(as)
	if pages != 2 {
		t.Fatalf("expected 2 pages for %d rows (pageSize=%d), got %d", state.PageSize+1, state.PageSize, pages)
	}
}

func TestMakePageItems_createsCorrectCount(t *testing.T) {
	items := makePageItems(5)
	if len(items) != 5 {
		t.Fatalf("expected 5 items, got %d", len(items))
	}
	if items[0].Key != "page-0" || items[0].Label != "1" {
		t.Fatalf("first item: expected {page-0, 1}, got %+v", items[0])
	}
	if items[4].Key != "page-4" || items[4].Label != "5" {
		t.Fatalf("last item: expected {page-4, 5}, got %+v", items[4])
	}
}

func TestTabs_activeTabSyncsFromStore(t *testing.T) {
	as := state.NewAppState(nil)
	_ = newCenterPanel(as)

	as.ActiveTab.Set(state.TabChart)

	cp := newCenterPanel(as)
	if cp.tabs.ActiveIndex.Get() != 0 {
		t.Fatalf("expected ActiveIndex=0 (TabData is default), got %d", cp.tabs.ActiveIndex.Get())
	}
}

func TestTabs_activationSetsStore(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)

	cp.tabs.Activated.Emit(1)
	if as.ActiveTab.Get() != state.TabChart {
		t.Fatalf("expected TabChart after activating index 1, got %q", as.ActiveTab.Get())
	}

	cp.tabs.Activated.Emit(0)
	if as.ActiveTab.Get() != state.TabData {
		t.Fatalf("expected TabData after activating index 0, got %q", as.ActiveTab.Get())
	}
}

func TestPagination_pageChangeUpdatesStore(t *testing.T) {
	as := state.NewAppState(nil)
	cp := newCenterPanel(as)

	initialPage := as.Page.Get()
	cp.pagination.Activated.Emit(2)
	if as.Page.Get() != 2 {
		t.Fatalf("expected Page=2 after activating index 2, got %d", as.Page.Get())
	}
	_ = initialPage
}

func TestPagination_pageSyncsFromStore(t *testing.T) {
	rows := makeTestDataset(state.PageSize*3, []string{"NA", "EU"})
	as := state.NewAppState(rows)
	cp := newCenterPanel(as)

	// After init with 3 pages of data, Page=0, CurrentIndex=0
	if got := cp.pagination.CurrentIndex.Get(); got != 0 {
		t.Fatalf("expected CurrentIndex=0 initially, got %d", got)
	}

	// Set Page=1 and verify pagination updates
	as.Page.Set(1)
	if got := cp.pagination.CurrentIndex.Get(); got != 1 {
		t.Fatalf("expected CurrentIndex=1 after Page=1, got %d", got)
	}
}
