package marks

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// contentInvalidator is the invalidation route Core provides; every mark that
// embeds Core satisfies it, so BindScalar's re-sync propagates through the
// same RX-1 FR-3 path as declared bindings.
type contentInvalidator interface {
	InvalidateContent(flags facet.DirtyFlags, source string)
}

// ScalarBinding implements the RX-2 Q2 two-way value contract for one
// comparable scalar held by a mark:
//
//	(a) attach-time sync — the mark adopts the store's current value,
//	(b) re-sync — every external store change re-applies the value and
//	    invalidates the mark,
//	(c) write-back — a user edit publishes to the store,
//	(d) echo guard — the store's change notification for a value the mark
//	    already holds is a no-op, so one user edit produces exactly one store
//	    change and one invalidation, never a re-apply loop, and an external
//	    write interleaving with a write-back still re-syncs.
//
// The doctrine is: the mark owns truth unless the app injects a store.
// Constructors take the store as an optional argument — nil ⇒ the mark
// creates a private store and exposes it through its Store() accessor — and
// injected stores are bound, never copied.
//
// The binding composes into the mark as an unexported field. The mark calls
// Attach from its OnAttach (after Core.OnAttach), calls Write from its
// user-edit path, and reads the store (directly or through its own mirror
// state kept fresh by apply) when rendering. Because the echo guard swallows
// the notification for the mark's own write, a user-edit path that relies on
// the notification for a local re-sync performs that re-sync itself after
// Write.
//
// A consequence of the echo guard, accepted by contract: a store change to
// the value the mark already holds does not re-apply — the value is already
// applied. Concurrency is none — the guard compares values on the runtime
// thread that wrote them; the store enforces single-writer threading.
//
// A panicking apply is quarantined through facet.RunRecovered: the facet
// degrades to empty output per RX-1, it does not corrupt.
//
// Non-comparable value types (slices, maps) are out of scope for the scalar
// contract — collections are marks/data's contract (RX-2 Q2).
type ScalarBinding[T comparable] struct {
	store   *store.ValueStore[T]
	flags   facet.DirtyFlags
	apply   func(T)
	current T
	init    bool
}

// BindScalar constructs the value contract for mark over external. external
// may be nil — the binding then no-ops (an unbound mark keeps its internal
// default); marks that always hold a store resolve nil to a private store
// before calling. apply re-syncs the mark's state from a store value (clause
// b); it may be nil for marks that read the store directly at render time —
// the invalidation still fires. flags are the dirty flags raised on every
// adopted external change.
func BindScalar[T comparable](mark facet.FacetImpl, external *store.ValueStore[T], flags facet.DirtyFlags, apply func(T)) *ScalarBinding[T] {
	return &ScalarBinding[T]{store: external, flags: flags, apply: apply}
}

// Attach performs the attach-time sync (a) and subscribes re-sync (b) with
// the echo guard (d). The subscription rides the mark's facet subscription
// bag with version tracking — released on dispose, refreshed in the
// projection cache key on every delivery. Marks call this from OnAttach
// after Core.OnAttach.
func (b *ScalarBinding[T]) Attach(mark facet.FacetImpl) {
	if b == nil || b.store == nil || mark == nil || mark.Base() == nil {
		return
	}
	if b.apply != nil {
		value := b.store.Get()
		facet.RunRecovered("value", mark.Base().ID(), func() {
			b.apply(value) // (a)
		})
	}
	b.current = b.store.Get()
	b.init = true
	facet.Store(facet.Subscribe(mark), &b.store.OnChange, b.store.Version, func(ch signal.Change[T]) {
		b.receive(mark, ch.New)
	})
}

// receive adopts an externally-written store value (b) unless the mark
// already holds it (d).
func (b *ScalarBinding[T]) receive(mark facet.FacetImpl, next T) {
	if b == nil || b.store == nil {
		return
	}
	if b.init && next == b.current {
		return // (d) echo guard
	}
	b.current = next
	b.init = true
	if b.apply != nil {
		facet.RunRecovered("value", mark.Base().ID(), func() {
			b.apply(next) // (b)
		})
	}
	if ci, ok := mark.(contentInvalidator); ok {
		ci.InvalidateContent(b.flags, "value")
	} else {
		mark.Base().InvalidateWithSource(b.flags|facet.DirtyProjection, "value")
	}
}

// Write publishes a user edit to the store (c). The echo state is recorded
// before the store write so the mark's own change notification is swallowed
// by the guard (d); the mark invalidates itself on its edit path — the
// notification must not be relied on to do it.
func (b *ScalarBinding[T]) Write(v T) {
	if b == nil || b.store == nil {
		return
	}
	b.current = v
	b.init = true
	b.store.Set(v)
}

// Store exposes the live store — internal when the mark constructed with a
// nil store, the injected store otherwise.
func (b *ScalarBinding[T]) Store() *store.ValueStore[T] {
	if b == nil {
		return nil
	}
	return b.store
}
