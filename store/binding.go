package store

// Binding is a light compatibility wrapper around ValueStore for authored marks.
type Binding[T any] struct {
	store *ValueStore[T]
}

// NewBinding constructs a binding with an initial value.
func NewBinding[T any](initial T) Binding[T] {
	return Binding[T]{store: NewValueStore(initial)}
}

// BindValueStore wraps an existing value store.
func BindValueStore[T any](s *ValueStore[T]) Binding[T] {
	return Binding[T]{store: s}
}

// Get returns the current value.
func (b Binding[T]) Get() T {
	if b.store == nil {
		var zero T
		return zero
	}
	return b.store.Get()
}

// Set updates the bound value.
func (b Binding[T]) Set(value T) {
	if b.store == nil {
		return
	}
	b.store.Set(value)
}

// Version returns the current version of the bound value.
func (b Binding[T]) Version() Version {
	if b.store == nil {
		return 0
	}
	return b.store.Version()
}

// Store returns the underlying store if one exists.
func (b Binding[T]) Store() *ValueStore[T] {
	return b.store
}

