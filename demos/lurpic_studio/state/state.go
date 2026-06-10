package state

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/store"
)

type TabID int

const (
	TabData TabID = iota
	TabChart
)

type ChartType int

const (
	ChartLine ChartType = iota
	ChartArea
	ChartPoint
	ChartBar
)

type AggMode int

const (
	AggNone AggMode = iota
	AggSum
	AggAvg
)

type LayoutMode int

const (
	LayoutWide LayoutMode = iota
	LayoutNarrow
)

type ConnState int

const (
	ConnDisconnected ConnState = iota
	ConnConnecting
	ConnConnected
)

type TimeRangeMode int

const (
	TimeRange7d TimeRangeMode = iota
	TimeRange30d
	TimeRangeAll
)

func (m TimeRangeMode) String() string {
	switch m {
	case TimeRange7d:
		return "7d"
	case TimeRange30d:
		return "30d"
	default:
		return "all"
	}
}

type OverlayKind int

const (
	OverlayNone OverlayKind = iota
	OverlayDialog
	OverlayNotification
	OverlayCommandPalette
	OverlayPopupPalette
	OverlayNavDrawer
)

type BarBucket struct {
	Region string
	Value  float64
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
	Rotation       *store.ValueStore[float64]
	Smoothing      *store.ValueStore[float64]
	ShowGrid       *store.ValueStore[bool]
	Live           *store.ValueStore[bool]
	Aggregation    *store.ValueStore[AggMode]
	Page           *store.ValueStore[int]
	LayoutMode     *store.ValueStore[LayoutMode]
	JobProgress    *store.ValueStore[float32]
	Connection     *store.ValueStore[ConnState]
	Threshold      *store.ValueStore[float64]
	OverlayState   *store.ValueStore[OverlayKind]
	TimeRange      *store.ValueStore[TimeRangeMode]

	VisibleRows *store.Derived[[]dataset.Row]
	YDomain     *store.Derived[[2]float64]
	BarBuckets  *store.Derived[[]BarBucket]
}

func NewAppState(rows []dataset.Row) *AppState {
	s := &AppState{
		Rows:           store.NewCollectionStore[dataset.Row](identifyRow),
		SelectedSource: store.NewValueStore(""),
		ActiveTab:      store.NewValueStore(TabData),
		ChartType:      store.NewValueStore(ChartLine),
		SeriesColor:    store.NewValueStore(gfx.Color{R: 0.20, G: 0.40, B: 0.80, A: 1}),
		ChartTitle:     store.NewValueStore("Revenue Over Time"),
		YAxisMax:       store.NewValueStore[float64](0),
		Opacity:        store.NewValueStore[float64](0.3),
		Rotation:       store.NewValueStore[float64](0),
		Smoothing:      store.NewValueStore[float64](0),
		ShowGrid:       store.NewValueStore(true),
		Live:           store.NewValueStore(false),
		Aggregation:    store.NewValueStore(AggNone),
		Page:           store.NewValueStore(1),
		LayoutMode:     store.NewValueStore(LayoutWide),
		JobProgress:    store.NewValueStore[float32](0),
		Connection:     store.NewValueStore(ConnDisconnected),
		Threshold:      store.NewValueStore[float64](15000),
		OverlayState:   store.NewValueStore(OverlayNone),
		TimeRange:      store.NewValueStore(TimeRangeAll),
	}
	s.Rows.Replace(rows)

	baseSources := []store.Invalidatable{s.Rows, s.SelectedSource, s.Aggregation, s.Page}

	s.VisibleRows = store.NewDerived(
		func() []dataset.Row {
			return computeVisibleRows(s.Rows.All(), s.SelectedSource.Get(), s.Aggregation.Get(), s.Page.Get())
		},
		baseSources...,
	)

	yDomainSources := make([]store.Invalidatable, len(baseSources)+1)
	copy(yDomainSources, baseSources)
	yDomainSources[len(baseSources)] = s.YAxisMax
	s.YDomain = store.NewDerived(
		func() [2]float64 {
			return computeYDomain(
				computeVisibleRows(s.Rows.All(), s.SelectedSource.Get(), s.Aggregation.Get(), s.Page.Get()),
				s.YAxisMax.Get(),
			)
		},
		yDomainSources...,
	)

	s.BarBuckets = store.NewDerived(
		func() []BarBucket {
			return computeBarBuckets(
				computeVisibleRows(s.Rows.All(), s.SelectedSource.Get(), s.Aggregation.Get(), s.Page.Get()),
			)
		},
		baseSources...,
	)

	return s
}

func identifyRow(r dataset.Row) store.ItemID {
	h := uint64(r.Date.Unix())
	for i := 0; i < len(r.Region); i++ {
		h = h*31 + uint64(r.Region[i])
	}
	return store.ItemID(h)
}
