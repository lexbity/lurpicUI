package navigation

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// SelectionBinding implements the RX-1 Q7 / FR-8 two-way selection contract
// shared by the nav marks:
//
//	(a) adopt the bound store's value at attach,
//	(b) re-sync the mark on every store change (the store is version-tracked
//	    in the mark's projection cache key through facet.Store, so a change
//	    re-projects via the Q3 content path),
//	(c) publish a user selection back to the store,
//	(d) tolerate external writes during the mark's own write-back (echo guard:
//	    the store's change notification for the value the mark already holds is
//	    a no-op).
//
// The mark reads Current() for its internal selection and calls Publish() for a
// user-driven selection; onSync runs on every adopted external value so the
// mark can re-render.
type SelectionBinding[T comparable] struct {
	store  *store.ValueStore[T]
	value  T
	init   bool
	onSync func(value T)
}

// BindSelection binds the mark's selection store and adopts its current value
// at attach (a). The subscription rides the facet's own subscription bag, so it
// is released on dispose. A nil store yields a no-op binding (an unbound mark
// keeps its own internal default).
func BindSelection[T comparable](mark facet.FacetImpl, sel *store.ValueStore[T], onSync func(value T)) *SelectionBinding[T] {
	b := &SelectionBinding[T]{store: sel, onSync: onSync}
	if sel != nil {
		b.value = sel.Get()
		b.init = true
		facet.Store(facet.Subscribe(mark), &sel.OnChange, sel.Version, func(signal.Change[T]) {
			b.receive(sel.Get())
		})
	}
	return b
}

// receive adopts an externally-written store value (b) unless it echoes the
// mark's own publish (d).
func (b *SelectionBinding[T]) receive(next T) {
	if b == nil || b.store == nil {
		return
	}
	if b.init && next == b.value {
		return
	}
	b.value = next
	b.init = true
	if b.onSync != nil {
		b.onSync(next)
	}
}

// Current returns the mark's internal selection (the last adopted store value).
func (b *SelectionBinding[T]) Current() T {
	if b == nil {
		var zero T
		return zero
	}
	return b.value
}

// Publish writes a user selection to the store (c). The echo guard (d) keeps
// the store's change notification for this same value from re-entering onSync.
func (b *SelectionBinding[T]) Publish(value T) {
	if b == nil || b.store == nil {
		return
	}
	b.value = value
	b.init = true
	b.store.Set(value)
}
