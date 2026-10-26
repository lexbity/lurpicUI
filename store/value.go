package store

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
)

// Invalidatable is implemented by all store types.
type Invalidatable interface {
	addInvalidationTarget(func()) Subscription
	storeScope() *Scope
}

// ValueStore holds a single observable value of type T.
type ValueStore[T any] struct {
	scope   *Scope
	version VersionSource

	mu            sync.RWMutex
	value         T
	invalidations invalidationSet

	OnChange signal.Signal[signal.Change[T]]
}

func NewValueStore[T any](scope *Scope, initial T) *ValueStore[T] {
	requireScope(scope)
	s := &ValueStore[T]{
		scope:    scope,
		value:    initial,
		OnChange: signal.NewSignal[signal.Change[T]]("ValueStore.OnChange"),
	}
	scope.addResource(s)
	return s
}

// Get returns the current value.
func (s *ValueStore[T]) Get() T {
	s.scope.assertDerivedReadAllowed(s)
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.value
}

// Set updates the value immediately and emits OnChange unconditionally —
// even when the value is unchanged — so subscribers must tolerate redundant
// deliveries (marks deduplicate with echo guards, see marks/value.go).
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
	s.scope.assertUsable()
	return s.version.Current()
}

func (s *ValueStore[T]) set(value T, tx *Transaction) {
	s.scope.assertUsable()
	assertNotProjecting()

	if tx != nil {
		tx.requireScope(s.scope)
		var old T
		var invalidations []func()
		tx.stage(
			func() {
				s.mu.Lock()
				old = s.value
				s.value = value
				s.version.Increment()
				invalidations = s.invalidations.snapshot()
				s.mu.Unlock()
			},
			func() {
				for _, fn := range invalidations {
					if fn != nil {
						fn()
					}
				}
				s.OnChange.Emit(signal.Change[T]{Old: old, New: value})
			},
		)
		return
	}

	s.mu.Lock()
	old := s.value
	s.value = value
	s.version.Increment()
	invalidations := s.invalidations.snapshot()
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.OnChange.Emit(signal.Change[T]{Old: old, New: value})
	}
	s.scope.enqueueSignal(notify)
}

func (s *ValueStore[T]) addInvalidationTarget(fn func()) Subscription {
	return s.invalidations.add(fn)
}

func (s *ValueStore[T]) storeScope() *Scope { return s.scope }
func (s *ValueStore[T]) OwnerScope() *Scope { return s.scope }

func (s *ValueStore[T]) closeScopeResource() {
	s.mu.Lock()
	var zero T
	s.value = zero
	s.invalidations.clear()
	s.OnChange.UnsubscribeAll()
	s.mu.Unlock()
}
