package state

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
)

func makeTestRows() []dataset.Row {
	return []dataset.Row{
		{Date: tm("2026-01-01"), Revenue: 1000, Users: 100, Region: "NA"},
		{Date: tm("2026-01-02"), Revenue: 2000, Users: 200, Region: "EU"},
		{Date: tm("2026-01-03"), Revenue: 3000, Users: 300, Region: "APAC"},
		{Date: tm("2026-01-04"), Revenue: 1500, Users: 150, Region: "NA"},
		{Date: tm("2026-01-05"), Revenue: 2500, Users: 250, Region: "EU"},
		{Date: tm("2026-01-06"), Revenue: 3500, Users: 350, Region: "APAC"},
		{Date: tm("2026-01-07"), Revenue: 1200, Users: 120, Region: "NA"},
		{Date: tm("2026-01-08"), Revenue: 2200, Users: 220, Region: "EU"},
		{Date: tm("2026-01-09"), Revenue: 3200, Users: 320, Region: "APAC"},
		{Date: tm("2026-01-10"), Revenue: 1800, Users: 180, Region: "NA"},
		{Date: tm("2026-01-11"), Revenue: 2800, Users: 280, Region: "EU"},
		{Date: tm("2026-01-12"), Revenue: 3800, Users: 380, Region: "APAC"},
		{Date: tm("2026-01-13"), Revenue: 1100, Users: 110, Region: "NA"},
		{Date: tm("2026-01-14"), Revenue: 2100, Users: 210, Region: "EU"},
		{Date: tm("2026-01-15"), Revenue: 3100, Users: 310, Region: "APAC"},
	}
}

func tm(s string) time.Time {
	tm, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return tm
}

func TestNewAppState_initialValues(t *testing.T) {
	s := NewAppState(makeTestRows())
	if got := s.SelectedSource.Get(); got != "" {
		t.Fatalf("SelectedSource: %q", got)
	}
	if got := s.ActiveTab.Get(); got != TabData {
		t.Fatalf("ActiveTab: %q", got)
	}
	if got := s.ChartType.Get(); got != ChartLine {
		t.Fatalf("ChartType: %q", got)
	}
	if got := s.SeriesColor.Get(); got != (gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}) {
		t.Fatalf("SeriesColor: %v", got)
	}
	if got := s.Aggregation.Get(); got != AggNone {
		t.Fatalf("Aggregation: %q", got)
	}
	if got := s.Page.Get(); got != 0 {
		t.Fatalf("Page: %d", got)
	}
	if got := s.LayoutMode.Get(); got != LayoutWide {
		t.Fatalf("LayoutMode: %q", got)
	}
	if got := s.Connection.Get(); got != ConnConnected {
		t.Fatalf("Connection: %q", got)
	}
}

func TestNewAppState_rowsLoaded(t *testing.T) {
	s := NewAppState(makeTestRows())
	if got := s.Rows.Len(); got != 15 {
		t.Fatalf("Rows.Len: expected 15, got %d", got)
	}
}

func TestVisibleRows_all_noFilter(t *testing.T) {
	s := NewAppState(makeTestRows())
	rows := s.VisibleRows.Get()
	if len(rows) != 10 {
		t.Fatalf("expected 10 (first page), got %d", len(rows))
	}
}

func TestVisibleRows_filterBySource(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.SelectedSource.Set("NA")
	rows := s.VisibleRows.Get()

	for _, r := range rows {
		if r.Region != "NA" {
			t.Fatalf("expected all rows to be NA, got %q", r.Region)
		}
	}
}

func TestVisibleRows_pagination(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.SelectedSource.Set("EU")
	rows0 := s.VisibleRows.Get()
	if len(rows0) != 5 {
		t.Fatalf("page 0: expected 5 EU rows, got %d", len(rows0))
	}

	s.Page.Set(1)
	rows1 := s.VisibleRows.Get()
	if len(rows1) != 0 {
		t.Fatalf("page 1: expected 0 rows (past end), got %d", len(rows1))
	}
}

func TestVisibleRows_outOfRangePage(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.Page.Set(99)
	rows := s.VisibleRows.Get()
	if len(rows) != 0 {
		t.Fatalf("expected empty for out-of-range page, got %d", len(rows))
	}
}

func TestVisibleRows_aggregationSum(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.Aggregation.Set(AggSum)
	rows := s.VisibleRows.Get()

	naSum := 1000 + 1500 + 1200 + 1800 + 1100
	naUsers := 100 + 150 + 120 + 180 + 110

	found := false
	for _, r := range rows {
		if r.Region == "NA" {
			found = true
			if r.Revenue != float64(naSum) {
				t.Fatalf("NA sum revenue: expected %.0f, got %.0f", float64(naSum), r.Revenue)
			}
			if r.Users != float64(naUsers) {
				t.Fatalf("NA sum users: expected %.0f, got %.0f", float64(naUsers), r.Users)
			}
		}
	}
	if !found {
		t.Fatal("NA region not found in aggregated rows")
	}
}

func TestVisibleRows_aggregationAvg(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.Aggregation.Set(AggAvg)
	rows := s.VisibleRows.Get()

	naAvg := (1000 + 1500 + 1200 + 1800 + 1100) / 5.0
	naAvgUsers := (100 + 150 + 120 + 180 + 110) / 5.0

	found := false
	for _, r := range rows {
		if r.Region == "NA" {
			found = true
			if r.Revenue != naAvg {
				t.Fatalf("NA avg revenue: expected %.2f, got %.2f", naAvg, r.Revenue)
			}
			if r.Users != naAvgUsers {
				t.Fatalf("NA avg users: expected %.2f, got %.2f", naAvgUsers, r.Users)
			}
		}
	}
	if !found {
		t.Fatal("NA region not found in aggregated rows")
	}
}

func TestVisibleRows_aggregationOrdered(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.Aggregation.Set(AggSum)
	rows := s.VisibleRows.Get()

	if len(rows) != 3 {
		t.Fatalf("expected 3 aggregated rows (NA, APAC, EU), got %d", len(rows))
	}
	if rows[0].Region != "APAC" {
		t.Fatalf("expected first region APAC, got %q", rows[0].Region)
	}
	if rows[1].Region != "EU" {
		t.Fatalf("expected second region EU, got %q", rows[1].Region)
	}
	if rows[2].Region != "NA" {
		t.Fatalf("expected third region NA, got %q", rows[2].Region)
	}
}

func TestYDomain_fromRevenue(t *testing.T) {
	s := NewAppState(makeTestRows())
	yd := s.YDomain.Get()
	if yd[0] != 0 {
		t.Fatalf("expected min 0, got %f", yd[0])
	}
	if yd[1] <= 3800 {
		t.Fatalf("expected max > 3800 (with padding), got %f", yd[1])
	}
}

func TestYDomain_emptyRows(t *testing.T) {
	s := NewAppState(nil)
	yd := s.YDomain.Get()
	if yd != [2]float64{0, 100} {
		t.Fatalf("expected default [0,100] for empty rows, got %v", yd)
	}
}

func TestYDomain_clampByAxisMax(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.YAxisMax.Set(float64(2000))
	yd := s.YDomain.Get()
	if yd[1] != 2000 {
		t.Fatalf("expected max clamped to 2000, got %f", yd[1])
	}
}

func TestYDomain_noClampWhenAxisMaxAboveMax(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.YAxisMax.Set(float64(50000))
	yd := s.YDomain.Get()
	if yd[1] <= 3800 {
		t.Fatalf("expected max > 3800 (padding above real max), got %f", yd[1])
	}
}

func TestYDomain_zeroAxisMax(t *testing.T) {
	s := NewAppState(makeTestRows())
	s.YAxisMax.Set(float64(0))
	yd := s.YDomain.Get()
	if yd[1] <= 3800 {
		t.Fatalf("expected max > 3800 when AxisMax is 0, got %f", yd[1])
	}
}

func TestYDomain_usersExceedRevenue(t *testing.T) {
	rows := makeTestRows()
	rows[0].Revenue = 50
	rows[0].Users = 5000
	s := NewAppState(rows)
	yd := s.YDomain.Get()
	if yd[1] < 5000 {
		t.Fatalf("expected max >= 5000 (from users), got %f", yd[1])
	}
}

func TestYDomain_allZeroValues(t *testing.T) {
	rows := []dataset.Row{
		{Date: tm("2026-01-01"), Revenue: 0, Users: 0, Region: "NA"},
	}
	s := NewAppState(rows)
	yd := s.YDomain.Get()
	if yd != [2]float64{0, 100} {
		t.Fatalf("expected [0,100] for zero values, got %v", yd)
	}
}

func TestBarBuckets_aggregatesByRegion(t *testing.T) {
	s := NewAppState(makeTestRows())
	buckets := s.BarBuckets.Get()

	if len(buckets) != 3 {
		t.Fatalf("expected 3 buckets (APAC, EU, NA), got %d", len(buckets))
	}

	naRev := 1000.0 + 1500 + 1200 + 1800 + 1100
	naUsers := 100.0 + 150 + 120 + 180 + 110

	for _, b := range buckets {
		if b.Region == "NA" {
			if b.Revenue != naRev {
				t.Fatalf("NA revenue: expected %.0f, got %.0f", naRev, b.Revenue)
			}
			if b.Users != naUsers {
				t.Fatalf("NA users: expected %.0f, got %.0f", naUsers, b.Users)
			}
		}
	}
}

func TestBarBuckets_emptyRows(t *testing.T) {
	s := NewAppState(nil)
	buckets := s.BarBuckets.Get()
	if len(buckets) != 0 {
		t.Fatalf("expected 0 buckets for no rows, got %d", len(buckets))
	}
}

func TestBarBuckets_ordered(t *testing.T) {
	s := NewAppState(makeTestRows())
	buckets := s.BarBuckets.Get()
	if buckets[0].Region != "APAC" {
		t.Fatalf("expected APAC first, got %q", buckets[0].Region)
	}
	if buckets[1].Region != "EU" {
		t.Fatalf("expected EU second, got %q", buckets[1].Region)
	}
	if buckets[2].Region != "NA" {
		t.Fatalf("expected NA third, got %q", buckets[2].Region)
	}
}

func TestVisibleRows_subscriberFiresOnce(t *testing.T) {
	s := NewAppState(makeTestRows())
	_ = s.FilteredRows.Get()
	_ = s.VisibleRows.Get()

	callCount := 0
	subID := s.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		callCount++
	})
	defer s.VisibleRows.OnChange.Unsubscribe(subID)

	s.SelectedSource.Set("NA")
	_ = s.FilteredRows.Get()
	_ = s.VisibleRows.Get()
	if callCount != 1 {
		t.Fatalf("expected 1 fire after SelectedSource change, got %d", callCount)
	}
}

func TestVisibleRows_noFireOnIrrelevantChange(t *testing.T) {
	s := NewAppState(makeTestRows())
	_ = s.FilteredRows.Get()
	_ = s.VisibleRows.Get()

	callCount := 0
	subID := s.VisibleRows.OnChange.Subscribe(func(c signal.Change[[]dataset.Row]) {
		callCount++
	})
	defer s.VisibleRows.OnChange.Unsubscribe(subID)

	s.ChartTitle.Set("irrelevant")
	_ = s.FilteredRows.Get()
	_ = s.VisibleRows.Get()
	if callCount != 0 {
		t.Fatalf("expected 0 fires on irrelevant change, got %d", callCount)
	}
}

func TestYDomain_subscriberFiresOnYAxisMaxChange(t *testing.T) {
	s := NewAppState(makeTestRows())
	_ = s.FilteredRows.Get()
	_ = s.YDomain.Get()

	callCount := 0
	subID := s.YDomain.OnChange.Subscribe(func(c signal.Change[[2]float64]) {
		callCount++
	})
	defer s.YDomain.OnChange.Unsubscribe(subID)

	s.YAxisMax.Set(float64(2000))
	_ = s.FilteredRows.Get()
	_ = s.YDomain.Get()
	if callCount != 1 {
		t.Fatalf("expected 1 fire after YAxisMax change, got %d", callCount)
	}
}

func TestBarBuckets_subscriberFiresOnSelectedSourceChange(t *testing.T) {
	s := NewAppState(makeTestRows())
	_ = s.FilteredRows.Get()
	_ = s.BarBuckets.Get()

	callCount := 0
	subID := s.BarBuckets.OnChange.Subscribe(func(c signal.Change[[]BarBucket]) {
		callCount++
	})
	defer s.BarBuckets.OnChange.Unsubscribe(subID)

	s.SelectedSource.Set("EU")
	_ = s.FilteredRows.Get()
	_ = s.BarBuckets.Get()
	if callCount != 1 {
		t.Fatalf("expected 1 fire after SelectedSource change, got %d", callCount)
	}
}

func TestBarBuckets_noFireOnIrrelevantChange(t *testing.T) {
	s := NewAppState(makeTestRows())
	_ = s.FilteredRows.Get()
	_ = s.BarBuckets.Get()

	callCount := 0
	subID := s.BarBuckets.OnChange.Subscribe(func(c signal.Change[[]BarBucket]) {
		callCount++
	})
	defer s.BarBuckets.OnChange.Unsubscribe(subID)

	s.ChartTitle.Set("irrelevant")
	_ = s.FilteredRows.Get()
	_ = s.BarBuckets.Get()
	if callCount != 0 {
		t.Fatalf("expected 0 fires on irrelevant change, got %d", callCount)
	}
}
