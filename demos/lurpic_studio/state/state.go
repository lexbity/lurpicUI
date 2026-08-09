// Package state owns the pure store topology for the flagship exhibit (E1):
// the row collection, the live-tail window, and the derived views — VisibleRows
// (the windowed series source) and YDomain (the y-scale clamp) feed the chart;
// BarBuckets (the windowed per-region aggregates) feed the feed-legend
// structure.List, while the read-only structure.Table reads the raw Rows
// directly. There is no facet code here — only stores and pure functions, so
// the whole package is trivially unit-testable and framework-free.
//
// All mutations must run on the runtime thread (the CollectionStore /
// ValueStore contract); in the app, the streaming feed (P5) and the editable
// grid (P6) commit through InsertRow / TrimToMax / Rows.Update.
package state

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/store"
)

// Tuning defaults; the E1 inspector controls override these at runtime.
const (
	// DefaultMaxRows bounds the Rows collection. The framework's
	// CollectionStore has no cap/evict API (F-collection-evict), so AppState
	// hand-rolls bounded retention via TrimToMax.
	DefaultMaxRows = 5000
	// DefaultWindowSeconds is the initial live-tail width W in [now-W, now].
	DefaultWindowSeconds = 60.0
	// DefaultYAxisMax is the initial y-domain upper clamp.
	DefaultYAxisMax = 1000.0
)

// AppState is the store topology for the flagship exhibit.
type AppState struct {
	// Rows is the append-only row collection, keyed by a monotonic counter
	// stamped at insert (stable across edits — see dataset.Row.ID).
	Rows *store.CollectionStore[dataset.Row]
	// LiveWindow is the sliding x-domain [now-W, now] in unix seconds, set
	// each feed tick unless Paused.
	LiveWindow *store.ValueStore[[2]float64]
	// Paused suppresses the per-tick LiveWindow.Set while pan/zoom is active.
	Paused *store.ValueStore[bool]
	// YAxisMax clamps the y-domain's upper bound (<= 0 disables the clamp).
	YAxisMax *store.ValueStore[float64]
	// WindowSeconds is the live-tail width W in seconds.
	WindowSeconds *store.ValueStore[float64]

	// VisibleRows is the rows inside the current LiveWindow, insertion order.
	VisibleRows *store.Derived[[]dataset.Row]
	// YDomain is the [lo, hi] y-extent of the visible values, hi clamped to
	// YAxisMax.
	YDomain *store.Derived[[2]float64]
	// BarBuckets aggregates the visible rows per region.
	BarBuckets *store.Derived[[]RegionBucket]

	// MaxRows is the retention cap TrimToMax enforces (>= 0).
	MaxRows int
	// minID is the lowest live row id; TrimToMax evicts from here.
	minID uint64
	// nextID is the next monotonic id InsertRow assigns.
	nextID uint64
}

// identifyRow derives the collection key from the row's stamped counter.
func identifyRow(r dataset.Row) store.ItemID { return store.ItemID(r.ID) }

// NewAppState builds the store topology, seeds it with the given rows
// (stamping each with a monotonic id), anchors the live window on the last
// seed timestamp, and trims to MaxRows. MaxRows defaults to DefaultMaxRows.
func NewAppState(seed []dataset.Row) *AppState {
	a := &AppState{
		Rows:          store.NewCollectionStore(identifyRow),
		LiveWindow:    store.NewValueStore([2]float64{0, DefaultWindowSeconds}),
		Paused:        store.NewValueStore(false),
		YAxisMax:      store.NewValueStore(DefaultYAxisMax),
		WindowSeconds: store.NewValueStore(DefaultWindowSeconds),
		MaxRows:       DefaultMaxRows,
	}
	buildDeriveds(a)
	for _, r := range seed {
		a.InsertRow(r)
	}
	if a.Rows.Len() > 0 {
		last := a.Rows.All()[a.Rows.Len()-1]
		a.AnchorLiveWindow(float64(last.Time.Unix()))
	}
	a.TrimToMax()
	return a
}

// AnchorLiveWindow centers the live tail on t: LiveWindow = [t-W, t]. The
// streaming feed calls this each tick (unless Paused) so the window slides
// with the synthetic clock.
func (a *AppState) AnchorLiveWindow(t float64) {
	w := a.WindowSeconds.Get()
	a.LiveWindow.Set([2]float64{t - w, t})
}

// InsertRow stamps a fresh monotonic id and appends the row to Rows. It is
// the only path that assigns ids, keeping Rows append-only with contiguous
// ids starting at minID. Returns the assigned id.
func (a *AppState) InsertRow(row dataset.Row) store.ItemID {
	a.nextID++
	row.ID = a.nextID
	a.Rows.Insert(row)
	if a.minID == 0 {
		a.minID = row.ID
	}
	return store.ItemID(row.ID)
}
