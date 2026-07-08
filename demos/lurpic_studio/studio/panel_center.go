package studio

import (
	"fmt"
	"strconv"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
)

type centerPanel struct {
	col         *layout.ColumnLayout
	tabs        *navigation.Tabs
	dataView    *layout.ColumnLayout
	tableScroll *structure.ScrollRegion
	table       *structure.Table
	pagination  *navigation.Pagination
	card        *structure.Card
	chart       *chartCanvas
	body        *tabbedBody
}

type tabbedBody struct {
	facet.Facet
	col      *layout.ColumnLayout
	appState *state.AppState
	dataView *layout.ColumnLayout
	chart    *chartCanvas
}

func (b *tabbedBody) Base() *facet.Facet               { return &b.Facet }
func (b *tabbedBody) OnAttach(ctx facet.AttachContext) {}
func (b *tabbedBody) OnDetach()                        {}
func (b *tabbedBody) OnActivate()                      {}
func (b *tabbedBody) OnDeactivate()                    {}

func newTabbedBody(as *state.AppState, dv *layout.ColumnLayout, cc *chartCanvas) *tabbedBody {
	b := &tabbedBody{appState: as, dataView: dv, chart: cc}

	// Use a ColumnLayout as the container for tab switching.
	// The active tab's child is made visible while the other is collapsed
	// by setting zero bounds on the inactive one.
	b.col = layout.NewColumnLayout()
	b.col.Gap = 0
	b.col.Add(layout.Flexible(dv, 1))
	b.col.Add(layout.Flexible(cc, 1))

	b.Facet.AddChild(b.col.Base())

	// React to tab changes by invalidating layout
	as.ActiveTab.OnChange.Subscribe(func(c signal.Change[state.TabID]) {
		b.Invalidate(facet.DirtyLayout)
	})

	return b
}

func newCenterPanel(as *state.AppState) *centerPanel {
	cp := &centerPanel{}

	cp.tabs = navigation.NewTabs("Center Tabs", []navigation.TabItem{
		{Key: string(state.TabData), Label: "Data"},
		{Key: string(state.TabChart), Label: "Chart"},
	})

	cp.table = structure.NewTable("Data Table", emptyTableData())
	cp.tableScroll = structure.NewScrollRegion("Table scroll")
	cp.tableScroll.Gap = marks.Const(float32(0))
	cp.tableScroll.SetChildren([]structure.ScrollRegionChild{
		{Facet: cp.table},
	})

	cp.pagination = navigation.NewPagination("Table pages", []navigation.PaginationItem{
		{Key: "page-0", Label: "1"},
	})

	cp.dataView = layout.NewColumnLayout()
	cp.dataView.Gap = 0
	cp.dataView.Add(layout.Flexible(cp.tableScroll, 1))
	cp.dataView.Add(layout.Fixed(cp.pagination))

	cp.chart = newChartCanvas(as)

	cp.card = structure.NewCard("Chart Card")
	cp.card.LayoutMode = marks.Const(structure.CardLayoutVertical)
	cp.card.ChildrenContent = []structure.CardChild{
		{Key: "canvas", Facet: cp.chart},
	}

	cp.body = newTabbedBody(as, cp.dataView, cp.chart)

	cp.col = layout.NewColumnLayout()
	cp.col.Gap = 0
	cp.col.Add(layout.Fixed(cp.tabs))
	cp.col.Add(layout.Flexible(cp.body, 1))

	wireTabs(as, cp.tabs)
	wireTableData(as, cp.table)
	wirePagination(as, cp.pagination)

	return cp
}

func wireTabs(as *state.AppState, tabs *navigation.Tabs) {
	tabs.Activated.Subscribe(func(idx int) {
		if idx >= 0 && idx < len(tabs.Items) {
			as.ActiveTab.Set(state.TabID(tabs.Items[idx].Key))
		}
	})
	as.ActiveTab.OnChange.Subscribe(func(c signal.Change[state.TabID]) {
		for i, item := range tabs.Items {
			if item.Key == string(c.New) {
				tabs.ActiveIndex = marks.Const(i)
				return
			}
		}
	})
}

func wireTableData(as *state.AppState, table *structure.Table) {
	refreshTable := func() {
		table.Data.Set(rowsToTableData(as, as.VisibleRows.Get()))
	}
	as.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		refreshTable()
	})
	as.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		refreshTable()
	})
}

func wirePagination(as *state.AppState, pagination *navigation.Pagination) {
	updatePageItems := func() {
		total := computeTotalPages(as)
		pagination.SetItems(makePageItems(total))
	}
	updatePageItems()
	as.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		updatePageItems()
	})
	pagination.Activated.Subscribe(func(idx int) {
		as.Page.Set(idx)
	})
	as.Page.OnChange.Subscribe(func(c signal.Change[int]) {
		if c.New >= 0 && c.New < len(pagination.Items) {
			pagination.CurrentIndex = marks.Const(c.New)
		}
	})
}

func computeTotalPages(as *state.AppState) int {
	rows := as.FilteredRows.Get()
	total := len(rows) / state.PageSize
	if len(rows)%state.PageSize != 0 {
		total++
	}
	if total < 1 {
		total = 1
	}
	return total
}

func makePageItems(count int) []navigation.PaginationItem {
	items := make([]navigation.PaginationItem, count)
	for i := range items {
		items[i] = navigation.PaginationItem{
			Key:   fmt.Sprintf("page-%d", i),
			Label: fmt.Sprintf("%d", i+1),
		}
	}
	return items
}

func emptyTableData() structure.TableData {
	return structure.TableData{
		Columns: []structure.TableColumn{
			{Key: "date", Label: "Date", Width: 120, Sortable: true},
			{Key: "revenue", Label: "Revenue", Width: 100, Sortable: true},
			{Key: "users", Label: "Users", Width: 80, Sortable: true},
			{Key: "region", Label: "Region", Width: 80, Sortable: true},
		},
		Rows: []structure.TableRow{},
	}
}

func rowsToTableData(as *state.AppState, filteredRows []dataset.Row) structure.TableData {
	td := emptyTableData()
	for i, r := range filteredRows {
		td.Rows = append(td.Rows, structure.TableRow{
			Key:   "row-" + strconv.Itoa(i),
			Cells: []string{r.Date.Format("2006-01-02"), fmt.Sprintf("%.1f", r.Revenue), fmt.Sprintf("%.0f", r.Users), r.Region},
		})
	}
	return td
}
