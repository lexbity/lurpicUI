package store

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/signal"
)

// ItemID is a stable identifier for a collection item.
type ItemID uint64

// CollectionStore holds an ordered collection with per-item change signals.
type CollectionStore[T any] struct {
	version  VersionSource
	identify func(T) ItemID

	mu            sync.RWMutex
	items         []T
	index         map[ItemID]int
	invalidations []func()

	onInsert  signal.Signal[CollectionInsertEvent[T]]
	onRemove  signal.Signal[CollectionRemoveEvent[T]]
	onUpdate  signal.Signal[CollectionUpdateEvent[T]]
	onReplace signal.Signal[signal.Unit]
}

type CollectionInsertEvent[T any] struct {
	Item  T
	Index int
}

type CollectionRemoveEvent[T any] struct {
	ID    ItemID
	Index int
	Item  T
}

type CollectionUpdateEvent[T any] struct {
	ID    ItemID
	Index int
	Old   T
	New   T
}

func NewCollectionStore[T any](identify func(T) ItemID) *CollectionStore[T] {
	return &CollectionStore[T]{
		identify:  identify,
		index:     make(map[ItemID]int),
		onInsert:  signal.NewSignal[CollectionInsertEvent[T]]("CollectionStore.onInsert"),
		onRemove:  signal.NewSignal[CollectionRemoveEvent[T]]("CollectionStore.onRemove"),
		onUpdate:  signal.NewSignal[CollectionUpdateEvent[T]]("CollectionStore.onUpdate"),
		onReplace: signal.NewSignal[signal.Unit]("CollectionStore.onReplace"),
	}
}

// Get returns the item with the given ID.
func (s *CollectionStore[T]) Get(id ItemID) (T, bool) {
	if s == nil {
		var zero T
		return zero, false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	idx, ok := s.index[id]
	if !ok || idx < 0 || idx >= len(s.items) {
		var zero T
		return zero, false
	}
	return s.items[idx], true
}

// At returns the item at index i.
func (s *CollectionStore[T]) At(i int) T {
	if s == nil {
		var zero T
		return zero
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[i]
}

// Len returns the number of items in the collection.
func (s *CollectionStore[T]) Len() int {
	if s == nil {
		return 0
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// All returns a snapshot copy of the collection.
func (s *CollectionStore[T]) All() []T {
	if s == nil {
		return nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		return nil
	}
	out := make([]T, len(s.items))
	copy(out, s.items)
	return out
}

// Version returns the current store version.
func (s *CollectionStore[T]) Version() Version {
	if s == nil {
		return 0
	}
	return s.version.Current()
}

// Insert adds an item or updates an existing one with the same ID.
func (s *CollectionStore[T]) Insert(item T) {
	syncutil.AssertRuntimeThread()
	s.insert(item, nil)
}

// Remove deletes an item by ID.
func (s *CollectionStore[T]) Remove(id ItemID) {
	syncutil.AssertRuntimeThread()
	s.remove(id, nil)
}

// Update updates an item by ID if it exists.
func (s *CollectionStore[T]) Update(item T) {
	syncutil.AssertRuntimeThread()
	s.update(item, nil)
}

// Replace replaces the entire collection.
func (s *CollectionStore[T]) Replace(items []T) {
	syncutil.AssertRuntimeThread()
	s.replace(items, nil)
}

func (s *CollectionStore[T]) InsertTx(item T, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.insert(item, tx)
}

func (s *CollectionStore[T]) RemoveTx(id ItemID, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.remove(id, tx)
}

func (s *CollectionStore[T]) UpdateTx(item T, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.update(item, tx)
}

func (s *CollectionStore[T]) ReplaceTx(items []T, tx *Transaction) {
	syncutil.AssertRuntimeThread()
	s.replace(items, tx)
}

func (s *CollectionStore[T]) insert(item T, tx *Transaction) {
	if s == nil {
		return
	}
	assertNotProjecting()
	id := s.identify(item)
	s.mu.Lock()
	if idx, ok := s.index[id]; ok {
		old := s.items[idx]
		s.items[idx] = item
		s.version.Increment()
		invalidations := append([]func(){}, s.invalidations...)
		s.mu.Unlock()
		notify := func() {
			for _, fn := range invalidations {
				if fn != nil {
					fn()
				}
			}
			s.onUpdate.Emit(CollectionUpdateEvent[T]{ID: id, Index: idx, Old: old, New: item})
		}
		if tx != nil {
			tx.deferCall(notify)
			return
		}
		notify()
		return
	}

	idx := len(s.items)
	s.items = append(s.items, item)
	s.index[id] = idx
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onInsert.Emit(CollectionInsertEvent[T]{Item: item, Index: idx})
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *CollectionStore[T]) remove(id ItemID, tx *Transaction) {
	if s == nil {
		return
	}
	assertNotProjecting()
	s.mu.Lock()
	idx, ok := s.index[id]
	if !ok || idx < 0 || idx >= len(s.items) {
		s.mu.Unlock()
		return
	}
	item := s.items[idx]
	s.items = append(s.items[:idx], s.items[idx+1:]...)
	delete(s.index, id)
	for i := idx; i < len(s.items); i++ {
		s.index[s.identify(s.items[i])] = i
	}
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onRemove.Emit(CollectionRemoveEvent[T]{ID: id, Index: idx, Item: item})
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *CollectionStore[T]) update(item T, tx *Transaction) {
	if s == nil {
		return
	}
	assertNotProjecting()
	id := s.identify(item)
	s.mu.Lock()
	idx, ok := s.index[id]
	if !ok || idx < 0 || idx >= len(s.items) {
		s.mu.Unlock()
		return
	}
	old := s.items[idx]
	s.items[idx] = item
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onUpdate.Emit(CollectionUpdateEvent[T]{ID: id, Index: idx, Old: old, New: item})
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *CollectionStore[T]) replace(items []T, tx *Transaction) {
	if s == nil {
		return
	}
	assertNotProjecting()
	s.mu.Lock()
	s.items = append([]T(nil), items...)
	s.index = make(map[ItemID]int, len(items))
	for i, item := range s.items {
		s.index[s.identify(item)] = i
	}
	s.version.Increment()
	invalidations := append([]func(){}, s.invalidations...)
	s.mu.Unlock()

	notify := func() {
		for _, fn := range invalidations {
			if fn != nil {
				fn()
			}
		}
		s.onReplace.Emit(signal.Fired)
	}
	if tx != nil {
		tx.deferCall(notify)
		return
	}
	notify()
}

func (s *CollectionStore[T]) addInvalidationTarget(fn func()) {
	if s == nil || fn == nil {
		return
	}
	s.mu.Lock()
	s.invalidations = append(s.invalidations, fn)
	s.mu.Unlock()
}

// OnReplaceSubscribe subscribes handler to Replace events and returns an unsubscribe function.
func (s *CollectionStore[T]) OnReplaceSubscribe(handler func(signal.Unit)) func() {
	if s == nil {
		return func() {}
	}
	id := s.onReplace.Subscribe(handler)
	return func() { s.onReplace.Unsubscribe(id) }
}
