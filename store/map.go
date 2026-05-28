package store

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
)

// MapStore holds an observable key/value map.
type MapStore[K comparable, V any] struct {
	version VersionSource

	mu            sync.RWMutex
	entries       map[K]V
	invalidations []func()

	onSet    signal.Signal[MapSetEvent[K, V]]
	onDelete signal.Signal[MapDeleteEvent[K, V]]
	onClear  signal.Signal[signal.Unit]
}

type MapSetEvent[K comparable, V any] struct {
	Key      K
	Value    V
	Previous V
	WasNew   bool
}

type MapDeleteEvent[K comparable, V any] struct {
	Key   K
	Value V
}

func NewMapStore[K comparable, V any]() *MapStore[K, V] {
	return &MapStore[K, V]{
		entries:  make(map[K]V),
		onSet:    signal.NewSignal[MapSetEvent[K, V]]("MapStore.onSet"),
		onDelete: signal.NewSignal[MapDeleteEvent[K, V]]("MapStore.onDelete"),
		onClear:  signal.NewSignal[signal.Unit]("MapStore.onClear"),
	}
}

// Get returns the value for a key.
func (s *MapStore[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	value, ok := s.entries[key]
	return value, ok
}

// Has reports whether a key exists.
func (s *MapStore[K, V]) Has(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, ok := s.entries[key]
	return ok
}

// Len returns the number of entries.
func (s *MapStore[K, V]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.entries)
}

// Snapshot returns a copy of the underlying map.
func (s *MapStore[K, V]) Snapshot() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.entries) == 0 {
		return nil
	}
	out := make(map[K]V, len(s.entries))
	for k, v := range s.entries {
		out[k] = v
	}
	return out
}

// Version returns the current store version.
func (s *MapStore[K, V]) Version() Version {
	return s.version.Current()
}

// Set assigns a value to a key.
func (s *MapStore[K, V]) Set(key K, value V) {
	syncutil.AssertRuntimeThread()
	s.set(key, value, nil)
}

// Delete removes a key if it exists.
func (s *MapStore[K, V]) Delete(key K) {
	syncutil.AssertRuntimeThread()
	s.delete(key, nil)
}

// Clear removes all entries.
func (s *MapStore[K, V]) Clear() {
	syncutil.AssertRuntimeThread()
	s.clear(nil)
}

func (s *MapStore[K, V]) SetTx(key K, value V, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.set(key, value, tx)
}

func (s *MapStore[K, V]) DeleteTx(key K, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.delete(key, tx)
}

func (s *MapStore[K, V]) ClearTx(tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.clear(tx)
}

func (s *MapStore[K, V]) set(key K, value V, tx *Transaction) {
	assertNotProjecting()
	s.mu.Lock()
	previous, ok := s.entries[key]
	s.entries[key] = value
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onSet.Emit(MapSetEvent[K, V]{Key: key, Value: value, Previous: previous, WasNew: !ok})
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *MapStore[K, V]) delete(key K, tx *Transaction) {
	assertNotProjecting()
	s.mu.Lock()
	value, ok := s.entries[key]
	if !ok {
		s.mu.Unlock()
		return
	}
	delete(s.entries, key)
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onDelete.Emit(MapDeleteEvent[K, V]{Key: key, Value: value})
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *MapStore[K, V]) clear(tx *Transaction) {
	assertNotProjecting()
	s.mu.Lock()
	if len(s.entries) == 0 {
		s.mu.Unlock()
		return
	}
	s.entries = make(map[K]V)
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onClear.Emit(signal.Fired)
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *MapStore[K, V]) addInvalidationTarget(fn func()) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.invalidations = append(s.invalidations, fn)
	s.mu.Unlock()
}
