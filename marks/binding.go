package marks

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// BindingKind classifies a Binding source.
type BindingKind uint8

const (
	BindConst BindingKind = iota
	BindStore
)

// Binding[T] is a named reference to truth — not truth itself.
//
// A const binding carries an immutable literal that never invalidates.
// A store binding references an app-owned ValueStore or Derived, delegates
// reads to the source on every call, and declares which dirty flags to raise
// when the source changes.
//
// Binding is a value type. Copies share the underlying source reference.
type Binding[T any] struct {
	kind  BindingKind
	val   T
	dirty facet.DirtyFlags
	ref   *bindingRef[T]
}

type bindingRef[T any] struct {
	read      func() T
	subscribe func(func()) func()
}

// Const constructs an immutable binding from a literal.
func Const[T any](v T) Binding[T] {
	return Binding[T]{kind: BindConst, val: v}
}

// FromStore constructs a binding backed by a ValueStore.
func FromStore[T any](s *store.ValueStore[T], dirty facet.DirtyFlags) Binding[T] {
	if s == nil {
		return Binding[T]{kind: BindConst}
	}
	return Binding[T]{
		kind:  BindStore,
		dirty: dirty,
		ref: &bindingRef[T]{
			read: s.Get,
			subscribe: func(fn func()) func() {
				id := s.OnChange.Subscribe(func(c signal.Change[T]) { fn() })
				return func() { s.OnChange.Unsubscribe(id) }
			},
		},
	}
}

// FromDerived constructs a binding backed by a Derived store.
func FromDerived[T any](d *store.Derived[T], dirty facet.DirtyFlags) Binding[T] {
	if d == nil {
		return Binding[T]{kind: BindConst}
	}
	return Binding[T]{
		kind:  BindStore,
		dirty: dirty,
		ref: &bindingRef[T]{
			read: d.Get,
			subscribe: func(fn func()) func() {
				id := d.OnChange.Subscribe(func(c signal.Change[T]) { fn() })
				return func() { d.OnChange.Unsubscribe(id) }
			},
		},
	}
}

// Get returns the current value.
//
// For const bindings the literal is returned directly. For store bindings the
// value is read from the bound store — the binding never owns domain data
// (Principle 1) and never caches a copy without a version.
func (b Binding[T]) Get() T {
	if b.kind == BindConst {
		return b.val
	}
	if b.ref != nil {
		return b.ref.read()
	}
	var zero T
	return zero
}

// IsDynamic reports whether the binding is backed by a store.
func (b Binding[T]) IsDynamic() bool {
	return b.kind == BindStore
}

// DirtyFlags returns the flags that should be raised when the source changes.
func (b Binding[T]) DirtyFlags() facet.DirtyFlags {
	return b.dirty
}

// SubscribeOnChange registers fn to be called when the underlying source
// changes. Returns a cleanup function. No-op for const bindings.
func (b Binding[T]) SubscribeOnChange(fn func()) func() {
	if b.kind == BindConst || b.ref == nil || b.ref.subscribe == nil {
		return nil
	}
	return b.ref.subscribe(fn)
}
