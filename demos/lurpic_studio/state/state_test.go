package state

import (
	"reflect"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/store"
)

func rowAt(sec int64, value float64, region string) dataset.Row {
	return dataset.Row{Time: time.Unix(sec, 0), Value: value, Region: region}
}

func TestInsertRow_monotonicIDs(t *testing.T) {
	a := NewAppState(nil)
	id1 := a.InsertRow(rowAt(100, 10, "north"))
	id2 := a.InsertRow(rowAt(200, 20, "south"))
	id3 := a.InsertRow(rowAt(300, 30, "east"))

	if id1 != 1 || id2 != 2 || id3 != 3 {
		t.Fatalf("inserted ids = %d,%d,%d, want 1,2,3", id1, id2, id3)
	}
	rows := a.Rows.All()
	if len(rows) != 3 {
		t.Fatalf("rows = %d, want 3", len(rows))
	}
	if rows[0].ID != 1 || rows[1].ID != 2 || rows[2].ID != 3 {
		t.Fatalf("stamped ids = %d,%d,%d, want 1,2,3", rows[0].ID, rows[1].ID, rows[2].ID)
	}
	// Identify must be deterministic per row (CollectionStore re-derives ids
	// after removals).
	if a.Rows.Identify(rows[0]) != 1 {
		t.Fatalf("Identify(row0) = %d, want 1", a.Rows.Identify(rows[0]))
	}
}

func TestNewAppState_seedsAndAnchorsWindow(t *testing.T) {
	seed := []dataset.Row{
		rowAt(100, 10, "north"),
		rowAt(120, 20, "south"),
		rowAt(150, 30, "east"),
	}
	a := NewAppState(seed)
	if a.Rows.Len() != 3 {
		t.Fatalf("seeded rows = %d, want 3", a.Rows.Len())
	}
	if got := a.Rows.All()[2].ID; got != 3 {
		t.Fatalf("last seed id = %d, want 3", got)
	}
	// Live window anchors on the last seed time: [150-60, 150].
	if w := a.LiveWindow.Get(); w != [2]float64{90, 150} {
		t.Fatalf("initial LiveWindow = %v, want [90 150]", w)
	}
}

func TestNewAppState_trimsSeedToMaxRows(t *testing.T) {
	seed := make([]dataset.Row, 0, DefaultMaxRows+1)
	for i := 0; i < DefaultMaxRows+1; i++ {
		seed = append(seed, rowAt(int64(100+i), float64(i), "north"))
	}
	a := NewAppState(seed)
	if a.Rows.Len() != DefaultMaxRows {
		t.Fatalf("rows after construction = %d, want %d", a.Rows.Len(), DefaultMaxRows)
	}
	// The oldest seed row (id 1) must have been evicted.
	if _, ok := a.Rows.Get(1); ok {
		t.Fatal("row id 1 survived construction-time trim")
	}
	if first := a.Rows.All()[0]; first.ID != 2 {
		t.Fatalf("oldest surviving id = %d, want 2", first.ID)
	}
}

func TestAnchorLiveWindow(t *testing.T) {
	a := NewAppState(nil)
	a.WindowSeconds.Set(30)
	a.AnchorLiveWindow(1000)
	if w := a.LiveWindow.Get(); w != [2]float64{970, 1000} {
		t.Fatalf("LiveWindow = %v, want [970 1000]", w)
	}
}

func TestVisibleRows_matrix(t *testing.T) {
	t.Run("closed window preserves order", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 1, "a"))
		a.InsertRow(rowAt(150, 2, "b"))
		a.InsertRow(rowAt(200, 3, "c"))
		a.InsertRow(rowAt(250, 4, "d"))

		a.LiveWindow.Set([2]float64{150, 200})
		got := a.VisibleRows.Get()
		want := []uint64{2, 3} // times 150 and 200 are the closed endpoints
		if !idsEqual(got, want) {
			t.Fatalf("VisibleRows ids = %v, want %v", rowIDs(got), want)
		}
	})

	t.Run("out-of-window rows excluded", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(10, 1, "a"))
		a.InsertRow(rowAt(20, 2, "b"))
		a.LiveWindow.Set([2]float64{30, 40})
		if got := a.VisibleRows.Get(); len(got) != 0 {
			t.Fatalf("VisibleRows = %v, want empty", rowIDs(got))
		}
	})

	t.Run("insert lands in window", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 1, "a"))
		a.LiveWindow.Set([2]float64{0, 200})
		_ = a.VisibleRows.Get() // prime
		a.InsertRow(rowAt(150, 2, "b"))
		got := a.VisibleRows.Get()
		if !idsEqual(got, []uint64{1, 2}) {
			t.Fatalf("VisibleRows ids = %v, want [1 2]", rowIDs(got))
		}
	})
}

func TestYDomain_matrix(t *testing.T) {
	t.Run("empty falls back", func(t *testing.T) {
		a := NewAppState(nil)
		a.LiveWindow.Set([2]float64{0, 1000})
		if got := a.YDomain.Get(); got != [2]float64{0, 1} {
			t.Fatalf("YDomain(empty) = %v, want [0 1]", got)
		}
	})

	t.Run("extent of visible values", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 100, "north"))
		a.InsertRow(rowAt(200, 300, "south"))
		a.InsertRow(rowAt(300, 200, "east"))
		a.LiveWindow.Set([2]float64{0, 1000})
		if got := a.YDomain.Get(); got != [2]float64{100, 300} {
			t.Fatalf("YDomain = %v, want [100 300]", got)
		}
	})

	t.Run("min found on a later row", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 300, "north"))
		a.InsertRow(rowAt(200, 100, "south"))
		a.InsertRow(rowAt(300, 250, "east"))
		a.LiveWindow.Set([2]float64{0, 1000})
		if got := a.YDomain.Get(); got != [2]float64{100, 300} {
			t.Fatalf("YDomain = %v, want [100 300]", got)
		}
	})

	t.Run("hi clamped by YAxisMax", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 100, "north"))
		a.InsertRow(rowAt(200, 300, "south"))
		a.LiveWindow.Set([2]float64{0, 1000})
		a.YAxisMax.Set(250)
		if got := a.YDomain.Get(); got != [2]float64{100, 250} {
			t.Fatalf("YDomain(clamped) = %v, want [100 250]", got)
		}
	})

	t.Run("zero YAxisMax disables clamp", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 100, "north"))
		a.InsertRow(rowAt(200, 900, "south"))
		a.LiveWindow.Set([2]float64{0, 1000})
		a.YAxisMax.Set(0)
		if got := a.YDomain.Get(); got != [2]float64{100, 900} {
			t.Fatalf("YDomain(unclamped) = %v, want [100 900]", got)
		}
	})

	t.Run("all-equal values widen", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 300, "north"))
		a.InsertRow(rowAt(200, 300, "south"))
		a.LiveWindow.Set([2]float64{0, 1000})
		if got := a.YDomain.Get(); got != [2]float64{299, 301} {
			t.Fatalf("YDomain(all-equal) = %v, want [299 301]", got)
		}
	})

	t.Run("fully clamped collapses to a band", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 1200, "north"))
		a.LiveWindow.Set([2]float64{0, 1000})
		a.YAxisMax.Set(1000)
		if got := a.YDomain.Get(); got != [2]float64{999, 1000} {
			t.Fatalf("YDomain(fully-clamped) = %v, want [999 1000]", got)
		}
	})

	t.Run("window filters the extent", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 100, "north"))
		a.InsertRow(rowAt(200, 900, "south"))
		a.LiveWindow.Set([2]float64{0, 150}) // only the 100s row is visible
		if got := a.YDomain.Get(); got != [2]float64{99, 101} {
			t.Fatalf("YDomain(windowed) = %v, want [99 101] (widen of single value 100)", got)
		}
	})
}

func TestBarBuckets_matrix(t *testing.T) {
	t.Run("empty", func(t *testing.T) {
		a := NewAppState(nil)
		a.LiveWindow.Set([2]float64{0, 1000})
		if got := a.BarBuckets.Get(); len(got) != 0 {
			t.Fatalf("BarBuckets(empty) = %v, want empty", got)
		}
	})

	t.Run("groups by region with sum and count, sorted by name", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 10, "north"))
		a.InsertRow(rowAt(200, 20, "north"))
		a.InsertRow(rowAt(300, 5, "south"))
		a.InsertRow(rowAt(400, 7, "south"))
		a.InsertRow(rowAt(500, 3, "west"))
		a.LiveWindow.Set([2]float64{0, 1000})
		got := a.BarBuckets.Get()
		want := []RegionBucket{
			{Region: "north", Value: 30, Count: 2},
			{Region: "south", Value: 12, Count: 2},
			{Region: "west", Value: 3, Count: 1},
		}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("BarBuckets = %+v, want %+v", got, want)
		}
	})

	t.Run("window filters aggregation", func(t *testing.T) {
		a := NewAppState(nil)
		a.InsertRow(rowAt(100, 10, "north"))
		a.InsertRow(rowAt(200, 20, "south"))
		a.LiveWindow.Set([2]float64{0, 150}) // only north visible
		got := a.BarBuckets.Get()
		want := []RegionBucket{{Region: "north", Value: 10, Count: 1}}
		if !reflect.DeepEqual(got, want) {
			t.Fatalf("BarBuckets(windowed) = %+v, want %+v", got, want)
		}
	})
}

// TestNoSpuriousRecompute asserts the version discipline: a Derived MUST NOT
// recompute when an irrelevant store changes, and MUST recompute when one of
// its sources changes.
func TestNoSpuriousRecompute(t *testing.T) {
	a := NewAppState(nil)
	a.InsertRow(rowAt(100, 10, "north"))
	a.LiveWindow.Set([2]float64{0, 1000})

	// Prime: every derived computes its initial value.
	_ = a.VisibleRows.Get()
	_ = a.YDomain.Get()
	_ = a.BarBuckets.Get()
	vrV, ydV, bbV := a.VisibleRows.Version(), a.YDomain.Version(), a.BarBuckets.Version()

	// Irrelevant stores: Paused and WindowSeconds are not sources of any
	// derived.
	a.Paused.Set(true)
	a.WindowSeconds.Set(120)
	_ = a.VisibleRows.Get()
	_ = a.YDomain.Get()
	_ = a.BarBuckets.Get()
	if a.VisibleRows.Version() != vrV || a.YDomain.Version() != ydV || a.BarBuckets.Version() != bbV {
		t.Fatal("derived recomputed after irrelevant store change (Paused/WindowSeconds)")
	}

	// YAxisMax is a source of YDomain only.
	a.YAxisMax.Set(500)
	_ = a.YDomain.Get()
	if a.YDomain.Version() == ydV {
		t.Fatal("YDomain did not recompute after YAxisMax change")
	}
	_ = a.VisibleRows.Get()
	_ = a.BarBuckets.Get()
	if a.VisibleRows.Version() != vrV || a.BarBuckets.Version() != bbV {
		t.Fatal("VisibleRows/BarBuckets recomputed after YAxisMax change (not their source)")
	}
	ydV = a.YDomain.Version()

	// Inserting a row is a source change for every derived (they all read
	// Rows via the shared visible-window filter).
	a.InsertRow(rowAt(200, 25, "south"))
	_ = a.VisibleRows.Get()
	_ = a.YDomain.Get()
	_ = a.BarBuckets.Get()
	if a.VisibleRows.Version() == vrV || a.YDomain.Version() == ydV || a.BarBuckets.Version() == bbV {
		t.Fatal("derived did not recompute after a row insert")
	}
}

func TestUpdateRow_preservesIdentity(t *testing.T) {
	a := NewAppState(nil)
	id := a.InsertRow(rowAt(100, 10, "north"))
	a.LiveWindow.Set([2]float64{0, 1000})

	stored, ok := a.Rows.Get(id)
	if !ok {
		t.Fatalf("row %d not found", id)
	}
	stored.Value = 999
	a.Rows.Update(stored)

	got, ok := a.Rows.Get(id)
	if !ok {
		t.Fatalf("row %d not found after update", id)
	}
	if got.Value != 999 || got.ID != uint64(id) {
		t.Fatalf("updated row = %+v, want Value=999 ID=%d", got, id)
	}
	// The visible rows and y-domain reflect the edit.
	rows := a.VisibleRows.Get()
	if len(rows) != 1 || rows[0].Value != 999 {
		t.Fatalf("VisibleRows after edit = %+v, want single row Value=999", rows)
	}
	if d := a.YDomain.Get(); d != [2]float64{998, 1000} {
		t.Fatalf("YDomain after edit = %v, want [998 1000]", d)
	}
}

func TestTrimToMax(t *testing.T) {
	t.Run("evicts oldest, preserves order", func(t *testing.T) {
		a := NewAppState(nil)
		a.MaxRows = 3
		for i := 1; i <= 5; i++ {
			a.InsertRow(rowAt(int64(100+i), float64(i), "north"))
		}
		a.TrimToMax()
		if a.Rows.Len() != 3 {
			t.Fatalf("rows = %d, want 3", a.Rows.Len())
		}
		got := a.Rows.All()
		if !idsEqual(got, []uint64{3, 4, 5}) {
			t.Fatalf("surviving ids = %v, want [3 4 5]", rowIDs(got))
		}
		// Idempotent at/below the cap.
		a.TrimToMax()
		if a.Rows.Len() != 3 {
			t.Fatalf("rows after second TrimToMax = %d, want 3 (idempotent)", a.Rows.Len())
		}
	})

	t.Run("continues from cursor after growth", func(t *testing.T) {
		a := NewAppState(nil)
		a.MaxRows = 3
		for i := 1; i <= 5; i++ {
			a.InsertRow(rowAt(int64(100+i), float64(i), "north"))
		}
		a.TrimToMax() // keeps 3,4,5
		a.InsertRow(rowAt(110, 6, "north"))
		a.InsertRow(rowAt(120, 7, "north"))
		a.TrimToMax() // keeps 5,6,7
		if !idsEqual(a.Rows.All(), []uint64{5, 6, 7}) {
			t.Fatalf("surviving ids = %v, want [5 6 7]", rowIDs(a.Rows.All()))
		}
	})

	t.Run("zero cap evicts everything", func(t *testing.T) {
		a := NewAppState(nil)
		a.MaxRows = 0
		a.InsertRow(rowAt(100, 1, "north"))
		a.InsertRow(rowAt(200, 2, "south"))
		a.TrimToMax()
		if a.Rows.Len() != 0 {
			t.Fatalf("rows = %d, want 0", a.Rows.Len())
		}
	})
}

func rowIDs(rows []dataset.Row) []uint64 {
	out := make([]uint64, len(rows))
	for i, r := range rows {
		out[i] = r.ID
	}
	return out
}

func idsEqual(rows []dataset.Row, want []uint64) bool {
	return reflect.DeepEqual(rowIDs(rows), want)
}

// Compile-time pin: *store.Derived[T] is a versioned Invalidatable, so it
// COULD be a NewDerived source (chaining). The topology deliberately does not
// chain — see F-derived-independence in buildDeriveds.
var _ store.Invalidatable = (*store.Derived[int])(nil)
