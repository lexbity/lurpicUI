package state

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

func testRows() []dataset.Row {
	return []dataset.Row{
		{Date: date(2026, 1, 1), Revenue: 10000, Users: 1000, Region: "NA"},
		{Date: date(2026, 1, 2), Revenue: 20000, Users: 2000, Region: "EU"},
		{Date: date(2026, 1, 3), Revenue: 30000, Users: 3000, Region: "APAC"},
		{Date: date(2026, 1, 4), Revenue: 5000, Users: 500, Region: "LATAM"},
		{Date: date(2026, 1, 5), Revenue: 15000, Users: 1500, Region: "NA"},
		{Date: date(2026, 1, 6), Revenue: 25000, Users: 2500, Region: "EU"},
		{Date: date(2026, 1, 7), Revenue: 35000, Users: 3500, Region: "APAC"},
		{Date: date(2026, 1, 8), Revenue: 8000, Users: 800, Region: "LATAM"},
		{Date: date(2026, 1, 9), Revenue: 12000, Users: 1200, Region: "NA"},
		{Date: date(2026, 1, 10), Revenue: 22000, Users: 2200, Region: "EU"},
		{Date: date(2026, 1, 11), Revenue: 32000, Users: 3200, Region: "APAC"},
		{Date: date(2026, 1, 12), Revenue: 6000, Users: 600, Region: "LATAM"},
	}
}

func date(year, month, day int) time.Time {
	return time.Date(year, time.Month(month), day, 0, 0, 0, 0, time.UTC)
}

func TestNewAppState(t *testing.T) {
	rows := testRows()
	s := NewAppState(rows)
	if s == nil {
		t.Fatal("NewAppState returned nil")
	}
	if s.Rows.Len() != 12 {
		t.Errorf("expected 12 rows, got %d", s.Rows.Len())
	}
}

func TestAppStateInitialValues(t *testing.T) {
	s := NewAppState(testRows())
	if got := s.SelectedSource.Get(); got != "" {
		t.Errorf("expected empty SelectedSource, got %q", got)
	}
	if got := s.ActiveTab.Get(); got != TabData {
		t.Errorf("expected TabData, got %v", got)
	}
	if got := s.ChartType.Get(); got != ChartLine {
		t.Errorf("expected ChartLine, got %v", got)
	}
	if got := s.ChartTitle.Get(); got != "Revenue Over Time" {
		t.Errorf("expected 'Revenue Over Time', got %q", got)
	}
	if got := s.YAxisMax.Get(); got != 0 {
		t.Errorf("expected 0 YAxisMax, got %f", got)
	}
	if got := s.Opacity.Get(); got != 0.3 {
		t.Errorf("expected 0.3 Opacity, got %f", got)
	}
	if got := s.Rotation.Get(); got != 0 {
		t.Errorf("expected 0 Rotation, got %f", got)
	}
	if got := s.Smoothing.Get(); got != 0 {
		t.Errorf("expected 0 Smoothing, got %f", got)
	}
	if got := s.ShowGrid.Get(); got != true {
		t.Errorf("expected true ShowGrid, got %v", got)
	}
	if got := s.Live.Get(); got != false {
		t.Errorf("expected false Live, got %v", got)
	}
	if got := s.Aggregation.Get(); got != AggNone {
		t.Errorf("expected AggNone, got %v", got)
	}
	if got := s.Page.Get(); got != 1 {
		t.Errorf("expected 1 Page, got %d", got)
	}
	if got := s.LayoutMode.Get(); got != LayoutWide {
		t.Errorf("expected LayoutWide, got %v", got)
	}
	if got := s.JobProgress.Get(); got != 0 {
		t.Errorf("expected 0 JobProgress, got %f", got)
	}
	if got := s.Connection.Get(); got != ConnDisconnected {
		t.Errorf("expected ConnDisconnected, got %v", got)
	}
	if got := s.Threshold.Get(); got != 15000 {
		t.Errorf("expected 15000 Threshold, got %f", got)
	}
	if got := s.OverlayState.Get(); got != OverlayNone {
		t.Errorf("expected OverlayNone, got %v", got)
	}
}

func TestSeriesColorDefault(t *testing.T) {
	s := NewAppState(testRows())
	c := s.SeriesColor.Get()
	if c.R != 0.20 || c.G != 0.40 || c.B != 0.80 || c.A != 1 {
		t.Errorf("unexpected default SeriesColor: %v", c)
	}
}

func TestVisibleRowsAll(t *testing.T) {
	s := NewAppState(testRows())
	rows := s.VisibleRows.Get()
	if len(rows) != 10 {
		t.Fatalf("expected 10 rows (page 1, pageSize 10), got %d", len(rows))
	}
}

func TestVisibleRowsPage2(t *testing.T) {
	s := NewAppState(testRows())
	s.Page.Set(2)
	rows := s.VisibleRows.Get()
	if len(rows) != 2 {
		t.Fatalf("expected 2 rows on page 2, got %d", len(rows))
	}
	if rows[0].Revenue != 32000 || rows[0].Region != "APAC" {
		t.Errorf("unexpected first row on page 2: %+v", rows[0])
	}
	if rows[1].Revenue != 6000 || rows[1].Region != "LATAM" {
		t.Errorf("unexpected second row on page 2: %+v", rows[1])
	}
}

func TestVisibleRowsPageOutOfRange(t *testing.T) {
	s := NewAppState(testRows())
	s.Page.Set(10)
	rows := s.VisibleRows.Get()
	if rows != nil {
		t.Errorf("expected nil for page 10, got %d rows", len(rows))
	}
}

func TestVisibleRowsFilteredBySource(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("NA")
	rows := s.VisibleRows.Get()
	if len(rows) != 3 {
		t.Fatalf("expected 3 NA rows, got %d", len(rows))
	}
	for _, r := range rows {
		if r.Region != "NA" {
			t.Errorf("expected NA region, got %q", r.Region)
		}
	}
}

func TestVisibleRowsFilteredBySourceWithPage(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("EU")
	rows := s.VisibleRows.Get()
	if len(rows) != 3 {
		t.Fatalf("expected 3 EU rows (all on one page), got %d", len(rows))
	}
}

func TestVisibleRowsUnknownSource(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("UNKNOWN")
	rows := s.VisibleRows.Get()
	if rows != nil {
		t.Errorf("expected nil for unknown source, got %d rows", len(rows))
	}
}

func TestVisibleRowsAggSum(t *testing.T) {
	s := NewAppState(testRows())
	s.Aggregation.Set(AggSum)
	rows := s.VisibleRows.Get()
	if len(rows) != 4 {
		t.Fatalf("expected 4 aggregated rows (one per region), got %d", len(rows))
	}
	expected := map[string]struct{ rev, users float64 }{
		"NA":    {10000 + 15000 + 12000, 1000 + 1500 + 1200},
		"EU":    {20000 + 25000 + 22000, 2000 + 2500 + 2200},
		"APAC":  {30000 + 35000 + 32000, 3000 + 3500 + 3200},
		"LATAM": {5000 + 8000 + 6000, 500 + 800 + 600},
	}
	for _, r := range rows {
		exp, ok := expected[r.Region]
		if !ok {
			t.Errorf("unexpected region %q in aggregated rows", r.Region)
			continue
		}
		if r.Revenue != exp.rev {
			t.Errorf("region %q: expected revenue %f, got %f", r.Region, exp.rev, r.Revenue)
		}
		if r.Users != exp.users {
			t.Errorf("region %q: expected users %f, got %f", r.Region, exp.users, r.Users)
		}
	}
}

func TestVisibleRowsAggAvg(t *testing.T) {
	s := NewAppState(testRows())
	s.Aggregation.Set(AggAvg)
	rows := s.VisibleRows.Get()
	if len(rows) != 4 {
		t.Fatalf("expected 4 aggregated rows, got %d", len(rows))
	}
	expected := map[string]struct{ rev, users float64 }{
		"NA":    {float64(10000+15000+12000) / 3, float64(1000+1500+1200) / 3},
		"EU":    {float64(20000+25000+22000) / 3, float64(2000+2500+2200) / 3},
		"APAC":  {float64(30000+35000+32000) / 3, float64(3000+3500+3200) / 3},
		"LATAM": {float64(5000+8000+6000) / 3, float64(500+800+600) / 3},
	}
	for _, r := range rows {
		exp, ok := expected[r.Region]
		if !ok {
			continue
		}
		if r.Revenue != exp.rev {
			t.Errorf("region %q: expected avg revenue %f, got %f", r.Region, exp.rev, r.Revenue)
		}
		if r.Users != exp.users {
			t.Errorf("region %q: expected avg users %f, got %f", r.Region, exp.users, r.Users)
		}
	}
}

func TestVisibleRowsRegionOrderStable(t *testing.T) {
	s := NewAppState(testRows())
	s.Aggregation.Set(AggSum)
	rows := s.VisibleRows.Get()
	expectedOrder := []string{"NA", "EU", "APAC", "LATAM"}
	for i, r := range rows {
		if r.Region != expectedOrder[i] {
			t.Errorf("row %d: expected region %q, got %q", i, expectedOrder[i], r.Region)
		}
	}
}

func TestVisibleRowsUnchangedWhenIrrelevantStoreChanges(t *testing.T) {
	s := NewAppState(testRows())
	initial, count := trackChanges(s.VisibleRows)
	s.ChartTitle.Set("New Title")
	s.ShowGrid.Set(false)
	_ = s.VisibleRows.Get()
	if count() > 0 {
		t.Errorf("VisibleRows subscriber fired %d times when only unrelated stores changed", count())
	}
	_ = initial
}

func TestVisibleRowsRecomputedOnSelectedSourceChange(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.VisibleRows)
	s.SelectedSource.Set("NA")
	rows := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected exactly 1 subscriber fire, got %d", count())
	}
	if len(rows) != 3 {
		t.Errorf("expected 3 NA rows, got %d", len(rows))
	}
}

func TestVisibleRowsRecomputedOnPageChange(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.VisibleRows)
	s.Page.Set(2)
	rows := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected exactly 1 subscriber fire, got %d", count())
	}
	if len(rows) != 2 {
		t.Errorf("expected 2 rows on page 2, got %d", len(rows))
	}
}

func TestVisibleRowsRecomputedOnAggregationChange(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.VisibleRows)
	s.Aggregation.Set(AggSum)
	rows := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected exactly 1 subscriber fire, got %d", count())
	}
	if len(rows) != 4 {
		t.Errorf("expected 4 aggregated rows, got %d", len(rows))
	}
}

func TestVisibleRowsCoalescedChanges(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.VisibleRows)
	s.SelectedSource.Set("EU")
	s.Page.Set(2)
	rows := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected exactly 1 subscriber fire for 2 coalesced changes, got %d", count())
	}
	if len(rows) != 0 {
		t.Errorf("expected 0 rows (EU on page 2 has 0 items), got %d", len(rows))
	}
}

func TestVisibleRowsNoRecomputationOnSameValue(t *testing.T) {
	s := NewAppState(testRows())
	initialRows := s.VisibleRows.Get()
	_, count := trackChanges(s.VisibleRows)
	s.SelectedSource.Set("NA")
	s.SelectedSource.Set("")
	s.Page.Set(1)
	rows := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected 1 subscriber fire (net result is same), got %d", count())
	}
	if len(rows) != len(initialRows) {
		t.Errorf("expected same row count as initial, got %d", len(rows))
	}
}

func TestYDomainFromVisibleRows(t *testing.T) {
	s := NewAppState(testRows())
	domain := s.YDomain.Get()
	if domain[0] != 0 {
		t.Errorf("expected domain min 0, got %f", domain[0])
	}
	expectedMax := 35000 * 1.1
	if domain[1] != expectedMax {
		t.Errorf("expected domain max %f (35000*1.1), got %f", expectedMax, domain[1])
	}
}

func TestYDomainEmptyVisibleRows(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("UNKNOWN")
	domain := s.YDomain.Get()
	if domain[0] != 0 || domain[1] != 100 {
		t.Errorf("expected default domain [0, 100] for empty visible rows, got [%f, %f]", domain[0], domain[1])
	}
}

func TestYDomainClampedByYAxisMax(t *testing.T) {
	s := NewAppState(testRows())
	s.YAxisMax.Set(25000)
	domain := s.YDomain.Get()
	if domain[0] != 0 {
		t.Errorf("expected domain min 0, got %f", domain[0])
	}
	if domain[1] != 25000 {
		t.Errorf("expected domain max clamped to 25000, got %f", domain[1])
	}
}

func TestYDomainNoClampWhenYAxisMaxIsZero(t *testing.T) {
	s := NewAppState(testRows())
	domain := s.YDomain.Get()
	expectedMax := 35000 * 1.1
	if domain[1] != expectedMax {
		t.Errorf("expected domain max %f (YAxisMax=0 means no clamp), got %f", expectedMax, domain[1])
	}
}

func TestYDomainNoClampWhenYAxisMaxAboveNaturalMax(t *testing.T) {
	s := NewAppState(testRows())
	s.YAxisMax.Set(100000)
	domain := s.YDomain.Get()
	expectedMax := 35000 * 1.1
	if domain[1] != expectedMax {
		t.Errorf("expected domain max %f (YAxisMax=100000 > natural max), got %f", expectedMax, domain[1])
	}
}

func TestYDomainChangesWithVisibleRows(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.YDomain)
	s.SelectedSource.Set("LATAM")
	domain := s.YDomain.Get()
	if count() != 1 {
		t.Errorf("expected 1 subscriber fire, got %d", count())
	}
	if domain[1] != 8000*1.1 {
		t.Errorf("expected domain max %f (LATAM max 8000*1.1), got %f", 8000*1.1, domain[1])
	}
}

func TestYDomainChangesWithYAxisMax(t *testing.T) {
	s := NewAppState(testRows())
	s.YAxisMax.Set(10000)
	domain := s.YDomain.Get()
	expectedMax := minFloat(35000*1.1, 10000)
	if domain[1] != expectedMax {
		t.Errorf("expected domain max %f, got %f", expectedMax, domain[1])
	}
}

func TestYDomainIrrelevantChangeNoRecompute(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.YDomain)
	s.ChartTitle.Set("Irrelevant")
	s.ShowGrid.Set(false)
	_ = s.YDomain.Get()
	if count() != 0 {
		t.Errorf("YDomain recomputed when only irrelevant stores changed")
	}
}

func TestBarBucketsEmptyVisibleRows(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("UNKNOWN")
	buckets := s.BarBuckets.Get()
	if buckets != nil {
		t.Errorf("expected nil for empty visible rows, got %d buckets", len(buckets))
	}
}

func TestBarBucketsAllRegions(t *testing.T) {
	s := NewAppState(testRows())
	buckets := s.BarBuckets.Get()
	if len(buckets) != 4 {
		t.Fatalf("expected 4 bar buckets (one per region), got %d", len(buckets))
	}
	expected := map[string]float64{
		"APAC":  30000 + 35000,
		"EU":    20000 + 25000 + 22000,
		"LATAM": 5000 + 8000,
		"NA":    10000 + 15000 + 12000,
	}
	for _, b := range buckets {
		exp, ok := expected[b.Region]
		if !ok {
			t.Errorf("unexpected bucket region %q", b.Region)
			continue
		}
		if b.Value != exp {
			t.Errorf("region %q: expected value %f, got %f", b.Region, exp, b.Value)
		}
	}
}

func TestBarBucketsSortedOrder(t *testing.T) {
	s := NewAppState(testRows())
	buckets := s.BarBuckets.Get()
	for i := 1; i < len(buckets); i++ {
		if buckets[i].Region < buckets[i-1].Region {
			t.Errorf("BarBuckets not sorted: %q before %q", buckets[i-1].Region, buckets[i].Region)
		}
	}
}

func TestBarBucketsSingleRegion(t *testing.T) {
	s := NewAppState(testRows())
	s.SelectedSource.Set("NA")
	buckets := s.BarBuckets.Get()
	if len(buckets) != 1 {
		t.Fatalf("expected 1 bucket for NA, got %d", len(buckets))
	}
	if buckets[0].Region != "NA" {
		t.Errorf("expected region NA, got %q", buckets[0].Region)
	}
	expectedVal := 10000.0 + 15000 + 12000
	if buckets[0].Value != expectedVal {
		t.Errorf("expected value %f, got %f", expectedVal, buckets[0].Value)
	}
}

func TestBarBucketsChangesWithVisibleRows(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.BarBuckets)
	s.SelectedSource.Set("EU")
	_ = s.BarBuckets.Get()
	if count() != 1 {
		t.Errorf("expected 1 subscriber fire, got %d", count())
	}
}

func TestBarBucketsIrrelevantChangeNoRecompute(t *testing.T) {
	s := NewAppState(testRows())
	_, count := trackChanges(s.BarBuckets)
	s.ShowGrid.Set(false)
	s.ChartTitle.Set("Irrelevant")
	_ = s.BarBuckets.Get()
	if count() != 0 {
		t.Errorf("BarBuckets recomputed when only irrelevant stores changed")
	}
}

func TestDerivedChainsProperly(t *testing.T) {
	s := NewAppState(testRows())
	beforeDomain := s.YDomain.Get()

	s.Aggregation.Set(AggSum)

	afterVisible := s.VisibleRows.Get()
	if len(afterVisible) != 4 {
		t.Fatalf("expected 4 aggregated rows, got %d", len(afterVisible))
	}

	afterDomain := s.YDomain.Get()
	if afterDomain == beforeDomain {
		t.Error("YDomain should change after aggregation")
	}

	afterBuckets := s.BarBuckets.Get()
	if len(afterBuckets) != 4 {
		t.Fatalf("expected 4 bar buckets, got %d", len(afterBuckets))
	}
}

func TestRowsReplaceTriggersVisibleRecompute(t *testing.T) {
	s := NewAppState(testRows())
	original := s.VisibleRows.Get()
	_, count := trackChanges(s.VisibleRows)

	newRows := []dataset.Row{
		{Date: date(2026, 2, 1), Revenue: 50000, Users: 5000, Region: "NA"},
		{Date: date(2026, 2, 2), Revenue: 60000, Users: 6000, Region: "EU"},
	}
	s.Rows.Replace(newRows)
	updated := s.VisibleRows.Get()
	if count() != 1 {
		t.Errorf("expected 1 subscriber fire after Replace, got %d", count())
	}
	if len(updated) == len(original) {
		t.Error("VisibleRows should return different count after Replace")
	}
	if len(updated) != 2 {
		t.Errorf("expected 2 rows after Replace, got %d", len(updated))
	}
}

func TestStoreMutationFromValueStore(t *testing.T) {
	s := NewAppState(testRows())
	s.ChartTitle.Set("New Chart Title")
	if got := s.ChartTitle.Get(); got != "New Chart Title" {
		t.Errorf("ChartTitle.Set failed: got %q", got)
	}
}

func TestStoreMutationFromBoolStore(t *testing.T) {
	s := NewAppState(testRows())
	s.ShowGrid.Set(false)
	if s.ShowGrid.Get() {
		t.Error("ShowGrid should be false after Set(false)")
	}
	s.Live.Set(true)
	if !s.Live.Get() {
		t.Error("Live should be true after Set(true)")
	}
}

func TestStoreMutationFromColorStore(t *testing.T) {
	s := NewAppState(testRows())
	newColor := gfx.Color{R: 1, G: 0, B: 0, A: 1}
	s.SeriesColor.Set(newColor)
	got := s.SeriesColor.Get()
	if got.R != 1 || got.G != 0 || got.B != 0 || got.A != 1 {
		t.Errorf("SeriesColor.Set failed: got %v", got)
	}
}

func TestStoreMutationFromEnumStores(t *testing.T) {
	s := NewAppState(testRows())
	s.ActiveTab.Set(TabChart)
	if got := s.ActiveTab.Get(); got != TabChart {
		t.Errorf("ActiveTab should be TabChart, got %v", got)
	}
	s.ChartType.Set(ChartBar)
	if got := s.ChartType.Get(); got != ChartBar {
		t.Errorf("ChartType should be ChartBar, got %v", got)
	}
	s.Connection.Set(ConnConnected)
	if got := s.Connection.Get(); got != ConnConnected {
		t.Errorf("Connection should be ConnConnected, got %v", got)
	}
	s.OverlayState.Set(OverlayDialog)
	if got := s.OverlayState.Get(); got != OverlayDialog {
		t.Errorf("OverlayState should be OverlayDialog, got %v", got)
	}
	s.LayoutMode.Set(LayoutNarrow)
	if got := s.LayoutMode.Get(); got != LayoutNarrow {
		t.Errorf("LayoutMode should be LayoutNarrow, got %v", got)
	}
}

func trackChanges[T any](d *store.Derived[T]) (T, func() int) {
	var count int
	subID := d.OnChange.Subscribe(func(c signal.Change[T]) {
		count++
	})
	initial := d.Get()
	baseline := count
	return initial, func() int {
		_ = subID
		return count - baseline
	}
}

func minFloat(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}
