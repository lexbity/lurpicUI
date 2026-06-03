package reactive

import "codeburg.org/lexbit/lurpicui/store"

// RangeFromRegion creates a ValueStore[[2]float64] initialized with the
// given plot-region dimensions. The adapter calls Set on this store from
// the resolved plot box after layout. A ReactiveScale using this store
// as its range source will recompute on resize.
func RangeFromRegion(initialLo, initialHi float64) *store.ValueStore[[2]float64] {
	return store.NewValueStore([2]float64{initialLo, initialHi})
}
