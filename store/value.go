package store

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
)

// Invalidatable is implemented by all store types.
type Invalidatable interface {
	addInvalidationTarget(func())
}

// ValueStore holds a single observable value of type T.
type ValueStore[T comparable] struct {
	version VersionSource

	mu            sync.RWMutex
	value         T
	invalidations []func()

	OnChange signal.Signal[signal.Change[T]]
}

func NewValueStore[T comparable](initial T) *ValueStore[T] {
	return &ValueStore[T]{
		value:    initial,
		OnChange: signal.NewSignal[signal.Change[T]]("ValueStore.OnChange"),
	}
}

// Get returns the current value.
func (s *ValueStore[T]) Get() T {
	if s == nil {
		var zero T
		return zero
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set updates the value immediately and emits OnChange if the value changed.
func (s *ValueStore[T]) Set(value T) {
	syncutil.AssertRuntimeThread()
	syncutil.AssertNotAnchorExporting("store.Set")
	s.set(value, nil)
}

// SetTx is like Set but defers notifications until tx.Commit.
func (s *ValueStore[T]) SetTx(value T, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	syncutil.AssertNotAnchorExporting("store.SetTx")
	s.set(value, tx)
}

// Version returns the current store version.
func (s *ValueStore[T]) Version() Version {
	if s == nil {
		return 0
	}
	return s.version.Current()
}

func (s *ValueStore[T]) set(value T, tx *Transaction) {
	if s == nil {
		return
	}

	s.mu.Lock()
	old := s.value
	if old == value {
		s.mu.Unlock()
		return
	}
	s.value = value
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.OnChange.Emit(signal.Change[T]{Old: old, New: value})
	}

	if tx != nil {
		tx.deferCall(notify)
		return
	}
	enqueueSignal(notify)
}

func (s *ValueStore[T]) addInvalidationTarget(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.invalidations = append(s.invalidations, fn)
	s.mu.Unlock()
}
