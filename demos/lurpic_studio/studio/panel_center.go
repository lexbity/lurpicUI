package studio

import (
	"fmt"
	"strconv"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/navigation"
	marksstruct "codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
)

type CenterPanel struct {
	facet.Facet
	layout       facet.LayoutRole
	tabs         *navigation.Tabs
	table        *marksstruct.Table
	pagination   *navigation.Pagination
	chartCanvas  *ChartCanvas
	appState     *state.AppState

	sourceSub signal.SubscriptionID
	rowsSub   signal.SubscriptionID
	pageSub   signal.SubscriptionID
	tabSub    signal.SubscriptionID
}

func rowsToTableColumns() []marksstruct.TableColumn {
	return []marksstruct.TableColumn{
		{Key: "date", Label: "Date", Width: 110, Align: facet.AlignStart, Sortable: true},
		{Key: "revenue", Label: "Revenue", Width: 110, Align: facet.AlignEnd, Sortable: true},
		{Key: "users", Label: "Users", Width: 90, Align: facet.AlignEnd, Sortable: true},
		{Key: "region", Label: "Region", Width: 90, Align: facet.AlignStart, Sortable: true},
	}
}

func rowsToTableData(rows []dataset.Row) marksstruct.TableData {
	cols := rowsToTableColumns()
	tableRows := make([]marksstruct.TableRow, len(rows))
	for i, r := range rows {
		tableRows[i] = marksstruct.TableRow{
			Key: fmt.Sprintf("%s-%s", r.Date.Format("2006-01-02"), r.Region),
			Cells: []string{
				r.Date.Format("2006-01-02"),
				fmt.Sprintf("%.2f", r.Revenue),
				fmt.Sprintf("%.0f", r.Users),
				r.Region,
			},
		}
	}
	return marksstruct.TableData{Columns: cols, Rows: tableRows}
}

func computeTotalPages(appState *state.AppState) int {
	appState.VisibleRows.Get()
	allRows := appState.Rows.All()
	totalRows := len(allRows)
	if totalRows == 0 {
		return 1
	}
	totalPages := (totalRows + state.PageSize - 1) / state.PageSize
	if totalPages < 1 {
		totalPages = 1
	}
	return totalPages
}

func buildPaginationItems(totalPages int) []navigation.PaginationItem {
	items := make([]navigation.PaginationItem, totalPages)
	for i := 0; i < totalPages; i++ {
		items[i] = navigation.PaginationItem{
			Key:   strconv.Itoa(i + 1),
			Label: strconv.Itoa(i + 1),
		}
	}
	return items
}

func NewCenterPanel(appState *state.AppState, fonts *text.FontRegistry) *CenterPanel {
	p := &CenterPanel{appState: appState}
	p.Facet = facet.NewFacet()

	p.tabs = navigation.NewTabs("Center View", []navigation.TabItem{
		{Key: "data", Label: "Data", IconRef: "table"},
		{Key: "chart", Label: "Chart", IconRef: "bar_chart"},
	})
	p.tabs.Activated.Subscribe(func(index int) {
		switch index {
		case 0:
			appState.ActiveTab.Set(state.TabData)
		case 1:
			appState.ActiveTab.Set(state.TabChart)
		}
	})

	initialData := rowsToTableData(appState.VisibleRows.Get())
	p.table = marksstruct.NewTable("Data Table", initialData)

	totalPages := computeTotalPages(appState)
	p.pagination = navigation.NewPagination("Data Pages", buildPaginationItems(totalPages))
	p.pagination.Activated.Subscribe(func(index int) {
		appState.Page.Set(index + 1)
	})

	p.chartCanvas = NewChartCanvas(appState, fonts)

	p.sourceSub = appState.SelectedSource.OnChange.Subscribe(func(c signal.Change[string]) {
		p.refreshTable()
		p.refreshPagination()
	})
	p.rowsSub = appState.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		p.refreshTable()
	})
	p.pageSub = appState.Page.OnChange.Subscribe(func(c signal.Change[int]) {
		p.pagination.CurrentIndex = marks.Const(c.New - 1)
		p.pagination.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})
	p.tabSub = appState.ActiveTab.OnChange.Subscribe(func(c signal.Change[state.TabID]) {
		p.pagination.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	})

	innerLayout := layout.NewColumnLayout()
	innerLayout.Add(layout.Flexible(p.tabs, 0))
	innerLayout.Add(layout.Flexible(p.table, 1))
	innerLayout.Add(layout.Flexible(p.pagination, 0))
	chartCard := marksstruct.NewCard("Chart View")
	chartCard.LayoutMode = marks.Const(marksstruct.CardLayoutVertical)
	chartCard.ChildrenContent = []marksstruct.CardChild{
		{Key: "chart", Facet: p.chartCanvas, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
	}
	chartCard.Base().AddChild(p.chartCanvas.Base())
	innerLayout.Add(layout.Flexible(chartCard, 1))

	p.Facet.AddChild(innerLayout.Base())

	p.layout = facet.LayoutRole{
		OnMeasure: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
			width := constraints.MaxSize.W
			height := constraints.MaxSize.H
			if width <= 0 {
				width = 600
			}
			if height <= 0 {
				height = 400
			}
			innerLayout.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: width, H: height}})
			return facet.MeasureResult{Size: gfx.Size{W: width, H: height}}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			innerLayout.Base().LayoutRole().Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, bounds)
		},
		Child: facet.GroupChildContract{
			SupportedPlacement: facet.SupportsFree | facet.SupportsGrid | facet.SupportsLinear,
			Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
				return facet.IntrinsicSize{
					Min:       gfx.Size{W: 300, H: 200},
					Preferred: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
					Max:       gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H},
				}
			},
			Stretch: facet.StretchPolicy{Width: facet.StretchAlways, Height: facet.StretchAlways},
		},
	}
	p.AddRole(&p.layout)
	return p
}

func (p *CenterPanel) Base() *facet.Facet { p.Facet.BindImpl(p); return &p.Facet }
func (p *CenterPanel) OnAttach(ctx facet.AttachContext) {}
func (p *CenterPanel) OnDetach() {
	p.appState.SelectedSource.OnChange.Unsubscribe(p.sourceSub)
	p.appState.VisibleRows.OnChange.Unsubscribe(p.rowsSub)
	p.appState.Page.OnChange.Unsubscribe(p.pageSub)
	p.appState.ActiveTab.OnChange.Unsubscribe(p.tabSub)
}
func (p *CenterPanel) OnActivate()   {}
func (p *CenterPanel) OnDeactivate() {}

func (p *CenterPanel) refreshTable() {
	rows := p.appState.VisibleRows.Get()
	data := rowsToTableData(rows)
	p.table.Data.Set(data)
}

func (p *CenterPanel) refreshPagination() {
	totalPages := computeTotalPages(p.appState)
	p.pagination.SetItems(buildPaginationItems(totalPages))
	p.pagination.CurrentIndex = marks.Const(p.appState.Page.Get() - 1)
	p.pagination.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

func (p *CenterPanel) Tabs() *navigation.Tabs              { return p.tabs }
func (p *CenterPanel) Table() *marksstruct.Table             { return p.table }
func (p *CenterPanel) Pagination() *navigation.Pagination    { return p.pagination }
func (p *CenterPanel) ActiveTab() state.TabID                { return p.appState.ActiveTab.Get() }
