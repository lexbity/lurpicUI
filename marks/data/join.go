package data

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// CollectionBinder drives child facet lifecycle from a CollectionStore.
//
// Inserted items produce new child facets via the factory. Removed items
// dispose their facet. Updated items invalidate their facet. Replaced
// collections reconcile the full set, preserving stable identity per ItemID.
type CollectionBinder[T any] struct {
	store    *store.CollectionStore[T]
	factory  func(T) facet.FacetImpl
	parent   *facet.Facet
	children map[store.ItemID]facet.FacetImpl
	order    []store.ItemID
	cleanups []func()
	attachCtx facet.AttachContext
}

// NewCollectionBinder constructs a binder that attaches/detaches child
// facets on the given parent in response to CollectionStore mutations.
func NewCollectionBinder[T any](
	parent *facet.Facet,
	s *store.CollectionStore[T],
	factory func(T) facet.FacetImpl,
) *CollectionBinder[T] {
	return &CollectionBinder[T]{
		store:    s,
		factory:  factory,
		parent:   parent,
		children: make(map[store.ItemID]facet.FacetImpl),
	}
}

// OnAttach subscribes to store signals and populates initial children.
// Call from the parent mark's OnAttach, passing the attach context
// so newly created children receive the same runtime services.
func (b *CollectionBinder[T]) OnAttach(ctx facet.AttachContext) {
	b.cleanups = append(b.cleanups,
		b.store.OnReplaceSubscribe(func(signal.Unit) {
			b.reconcile()
		}),
	)
	b.cleanups = append(b.cleanups,
		b.store.OnInsertSubscribe(func(e store.CollectionInsertEvent[T]) {
			b.insertChild(e.Item, e.Index)
		}),
	)
	b.cleanups = append(b.cleanups,
		b.store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[T]) {
			b.removeChild(e.ID)
		}),
	)
	b.cleanups = append(b.cleanups,
		b.store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[T]) {
			b.updateChild(e.ID)
		}),
	)

	b.attachCtx = ctx
	b.reconcile()
}

// OnDetach unsubscribes from store signals and disposes all children.
// Call from the parent mark's OnDetach.
func (b *CollectionBinder[T]) OnDetach() {
	for _, cleanup := range b.cleanups {
		if cleanup != nil {
			cleanup()
		}
	}
	b.cleanups = nil

	for _, child := range b.children {
		facet.Dispose(child)
	}
	b.children = make(map[store.ItemID]facet.FacetImpl)
	b.order = nil
}

// Child returns the facet for the given ItemID, or nil.
func (b *CollectionBinder[T]) Child(id store.ItemID) facet.FacetImpl {
	return b.children[id]
}

// Children returns all child facets in store order.
func (b *CollectionBinder[T]) Children() []facet.FacetImpl {
	out := make([]facet.FacetImpl, 0, len(b.order))
	for _, id := range b.order {
		if child, ok := b.children[id]; ok {
			out = append(out, child)
		}
	}
	return out
}

func (b *CollectionBinder[T]) reconcile() {
	current := b.store.All()
	currentIDs := make(map[store.ItemID]struct{}, len(current))
	for _, item := range current {
		currentIDs[b.store.Identify(item)] = struct{}{}
	}

	for id, child := range b.children {
		if _, ok := currentIDs[id]; !ok {
			facet.Dispose(child)
			delete(b.children, id)
		}
	}

	newOrder := make([]store.ItemID, 0, len(current))
	for _, item := range current {
		id := b.store.Identify(item)
		if _, exists := b.children[id]; !exists {
			child := b.factory(item)
			b.parent.AddChildRuntime(child.Base())
			facet.Attach(child, b.attachCtx)
			b.children[id] = child
		}
		newOrder = append(newOrder, id)
	}
	b.order = newOrder
}

func (b *CollectionBinder[T]) insertChild(item T, index int) {
	id := b.store.Identify(item)
	if _, exists := b.children[id]; exists {
		return
	}
	child := b.factory(item)
	b.parent.AddChildRuntime(child.Base())
	facet.Attach(child, b.attachCtx)
	b.children[id] = child

	if index < 0 || index >= len(b.order) {
		b.order = append(b.order, id)
	} else {
		b.order = append(b.order[:index], append([]store.ItemID{id}, b.order[index:]...)...)
	}
}

func (b *CollectionBinder[T]) removeChild(id store.ItemID) {
	child, ok := b.children[id]
	if !ok {
		return
	}
	facet.Dispose(child)
	delete(b.children, id)

	for i := range b.order {
		if b.order[i] == id {
			b.order = append(b.order[:i], b.order[i+1:]...)
			break
		}
	}
}

func (b *CollectionBinder[T]) updateChild(id store.ItemID) {
	if child, ok := b.children[id]; ok {
		child.Base().Invalidate(facet.DirtyProjection)
	}
}
