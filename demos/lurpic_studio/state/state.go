package state

import (
	"hash/fnv"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

type TabID string

const (
	TabData  TabID = "data"
	TabChart TabID = "chart"
)

type ChartType string

const (
	ChartLine    ChartType = "line"
	ChartArea    ChartType = "area"
	ChartBar     ChartType = "bar"
	ChartScatter ChartType = "scatter"
)

type AggMode string

const (
	AggNone AggMode = "none"
	AggSum  AggMode = "sum"
	AggAvg  AggMode = "avg"
)

type LayoutMode string

const (
	LayoutWide   LayoutMode = "wide"
	LayoutNarrow LayoutMode = "narrow"
)

type ConnState string

const (
	ConnDisconnected ConnState = "disconnected"
	ConnConnecting   ConnState = "connecting"
	ConnConnected    ConnState = "connected"
	ConnError        ConnState = "error"
)

type OverlayKind string

const (
	OverlayNone           OverlayKind = ""
	OverlayDialog         OverlayKind = "dialog"
	OverlayCommandPalette OverlayKind = "command_palette"
	OverlayPopupPalette   OverlayKind = "popup_palette"
	OverlayNavDrawer      OverlayKind = "nav_drawer"
	OverlayBottomSheet    OverlayKind = "bottom_sheet"
)

type BarBucket struct {
	Region  string
	Revenue float64
	Users   float64
}

const PageSize = 10

type AppState struct {
	Rows           *store.CollectionStore[dataset.Row]
	SelectedSource *store.ValueStore[string]
	ActiveTab      *store.ValueStore[TabID]
	ChartType      *store.ValueStore[ChartType]
	SeriesColor    *store.ValueStore[gfx.Color]
	ChartTitle     *store.ValueStore[string]
	YAxisMax       *store.ValueStore[float64]
	Opacity        *store.ValueStore[float64]
	Smoothing      *store.ValueStore[float64]
	ShowGrid       *store.ValueStore[bool]
	Live           *store.ValueStore[bool]
	Aggregation    *store.ValueStore[AggMode]
	Page           *store.ValueStore[int]
	LayoutMode     *store.ValueStore[LayoutMode]
	JobProgress    *store.ValueStore[float64]
	Connection     *store.ValueStore[ConnState]
	Threshold      *store.ValueStore[float64]
	OverlayState   *store.ValueStore[OverlayKind]

	FilteredRows *store.Derived[[]dataset.Row]
	VisibleRows  *store.Derived[[]dataset.Row]
	YDomain      *store.Derived[[2]float64]
	BarBuckets   *store.Derived[[]BarBucket]
}

func NewAppState(rows []dataset.Row) *AppState {
	s := &AppState{
		Rows:           store.NewCollectionStore(identifyRow),
		SelectedSource: store.NewValueStore(""),
		ActiveTab:      store.NewValueStore(TabData),
		ChartType:      store.NewValueStore(ChartLine),
		SeriesColor:    store.NewValueStore(gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}),
		ChartTitle:     store.NewValueStore("Revenue by Region"),
		YAxisMax:       store.NewValueStore(float64(0)),
		Opacity:        store.NewValueStore(0.8),
		Smoothing:      store.NewValueStore(float64(0)),
		ShowGrid:       store.NewValueStore(true),
		Live:           store.NewValueStore(false),
		Aggregation:    store.NewValueStore(AggNone),
		Page:           store.NewValueStore(0),
		LayoutMode:     store.NewValueStore(LayoutWide),
		JobProgress:    store.NewValueStore(float64(0)),
		Connection:     store.NewValueStore(ConnConnected),
		Threshold:      store.NewValueStore(float64(10000)),
		OverlayState:   store.NewValueStore(OverlayNone),
	}
	for _, r := range rows {
		s.Rows.Insert(r)
	}
	s.initDeriveds()
	return s
}

func (s *AppState) initDeriveds() {
	s.FilteredRows = store.NewDerived(
		func() []dataset.Row { return computeFilteredRows(s) },
		s.Rows,
		s.SelectedSource,
		s.Aggregation,
	)
	s.VisibleRows = store.NewDerived(
		func() []dataset.Row { return computeVisibleRows(s) },
		s.FilteredRows,
		s.Page,
	)
	s.YDomain = store.NewDerived(
		func() [2]float64 { return computeYDomain(s) },
		s.FilteredRows,
		s.YAxisMax,
	)
	s.BarBuckets = store.NewDerived(
		func() []BarBucket { return computeBarBuckets(s) },
		s.FilteredRows,
	)
}

func identifyRow(r dataset.Row) store.ItemID {
	h := fnv.New64a()
	h.Write([]byte(r.Date.Format("2006-01-02")))
	h.Write([]byte(r.Region))
	return store.ItemID(h.Sum64())
}
