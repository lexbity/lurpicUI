package reactive

import (
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/store"
)

// DomainFromCollection builds a *store.Derived[[2]float64] that computes
// the data extent from a CollectionStore via scale.ExtentBy. The returned
// derived store invalidates whenever the collection changes (insert, remove,
// update, replace), making it suitable as a domain source for ReactiveScale.
func DomainFromCollection[T any](
	coll *store.CollectionStore[T],
	accessor func(T) float64,
) *store.Derived[[2]float64] {
	return store.NewDerived(
		func() [2]float64 {
			items := coll.All()
			lo, hi := scale.ExtentBy(items, accessor)
			return [2]float64{lo, hi}
		},
		coll,
	)
}
