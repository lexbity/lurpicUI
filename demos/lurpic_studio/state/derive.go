package state

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/store"
)

// RegionBucket aggregates one categorical region over the visible window.
type RegionBucket struct {
	Region string
	Value  float64
	Count  int
}

// buildDeriveds wires the three derived views. Every derived lists its
// sources explicitly (the store.NewDerived contract). The deriveds source the
// RAW stores rather than each other on purpose: a chained derived does not
// recompute when its upstream sibling is marked dirty but not yet re-Get()'d
// (probe-confirmed staleness in store.Derived), so independent source sets
// plus the shared pure helpers below keep every derived correct in a single
// Get() (F-derived-independence).
func buildDeriveds(a *AppState) {
	a.VisibleRows = store.NewDerived(func() []dataset.Row {
		return visibleRowsIn(a.Rows.All(), a.LiveWindow.Get())
	}, a.Rows, a.LiveWindow)

	a.YDomain = store.NewDerived(func() [2]float64 {
		return yDomainOf(visibleRowsIn(a.Rows.All(), a.LiveWindow.Get()), a.YAxisMax.Get())
	}, a.Rows, a.LiveWindow, a.YAxisMax)

	a.BarBuckets = store.NewDerived(func() []RegionBucket {
		return bucketByRegion(visibleRowsIn(a.Rows.All(), a.LiveWindow.Get()))
	}, a.Rows, a.LiveWindow)
}

// visibleRowsIn filters rows to the closed window [lo, hi] (fractional unix
// seconds), preserving insertion order. Fractional (UnixNano) precision keeps
// a sub-second synthetic feed clock aligned with the window.
func visibleRowsIn(all []dataset.Row, window [2]float64) []dataset.Row {
	lo, hi := window[0], window[1]
	out := make([]dataset.Row, 0, len(all))
	for _, r := range all {
		t := float64(r.Time.UnixNano()) / 1e9
		if t >= lo && t <= hi {
			out = append(out, r)
		}
	}
	return out
}

// yDomainOf returns the [lo, hi] extent of the rows' values with the upper
// bound clamped to yAxisMax when positive. Degenerate inputs (empty,
// all-equal, fully clamped) widen or fall back so a scale never collapses.
func yDomainOf(rows []dataset.Row, yAxisMax float64) [2]float64 {
	if len(rows) == 0 {
		return [2]float64{0, 1}
	}
	lo, hi := rows[0].Value, rows[0].Value
	for _, r := range rows[1:] {
		if r.Value < lo {
			lo = r.Value
		}
		if r.Value > hi {
			hi = r.Value
		}
	}
	if lo == hi {
		lo--
		hi++
	}
	if yAxisMax > 0 && hi > yAxisMax {
		hi = yAxisMax
	}
	if lo >= hi {
		lo = hi - 1
	}
	return [2]float64{lo, hi}
}

// bucketByRegion groups rows by region with summed value and row count,
// ordered by region name for deterministic goldens.
func bucketByRegion(rows []dataset.Row) []RegionBucket {
	buckets := make([]RegionBucket, 0, 4)
	byRegion := make(map[string]int, 4)
	for _, r := range rows {
		if idx, ok := byRegion[r.Region]; ok {
			buckets[idx].Value += r.Value
			buckets[idx].Count++
		} else {
			byRegion[r.Region] = len(buckets)
			buckets = append(buckets, RegionBucket{Region: r.Region, Value: r.Value, Count: 1})
		}
	}
	sort.Slice(buckets, func(i, j int) bool { return buckets[i].Region < buckets[j].Region })
	return buckets
}
