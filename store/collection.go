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
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.items[i]
}

// Len returns the number of items in the collection.
func (s *CollectionStore[T]) Len() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.items)
}

// All returns a snapshot copy of the collection.
func (s *CollectionStore[T]) All() []T {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.items) == 0 {
		return []T{}
	}
	out := make([]T, len(s.items))
	copy(out, s.items)
	return out
}

// Identify returns the stable ItemID for the given item.
func (s *CollectionStore[T]) Identify(item T) ItemID {
	return s.identify(item)
}

// Version returns the current store version.
func (s *CollectionStore[T]) Version() Version {
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
	assertNotProjecting()
	id := s.identify(item)
	if tx != nil {
		var old T
		var idx int
		var isUpdate bool
		var invalidations []func()
		tx.stage(
			func() {
				s.mu.Lock()
				if existingIdx, ok := s.index[id]; ok {
					isUpdate = true
					idx = existingIdx
					old = s.items[existingIdx]
					s.items[existingIdx] = item
				} else {
					isUpdate = false
					idx = len(s.items)
					s.items = append(s.items, item)
					s.index[id] = idx
				}
				s.version.Increment()
				invalidations = append([]func(){}, s.invalidations...)
				s.mu.Unlock()
			},
			func() {
				for _, fn := range invalidations {
					if fn != nil {
						fn()
					}
				}
				if isUpdate {
					s.onUpdate.Emit(CollectionUpdateEvent[T]{ID: id, Index: idx, Old: old, New: item})
				} else {
					s.onInsert.Emit(CollectionInsertEvent[T]{Item: item, Index: idx})
				}
			},
		)
		return
	}
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
	notify()
}

func (s *CollectionStore[T]) remove(id ItemID, tx *Transaction) {
	assertNotProjecting()
	if tx != nil {
		var item T
		var idx int
		var invalidations []func()
		var skipped bool
		tx.stage(
			func() {
				s.mu.Lock()
				existingIdx, ok := s.index[id]
				if !ok || existingIdx < 0 || existingIdx >= len(s.items) {
					s.mu.Unlock()
					skipped = true
					return
				}
				idx = existingIdx
				item = s.items[existingIdx]
				copy(s.items[existingIdx:], s.items[existingIdx+1:])
				var zero T
				s.items[len(s.items)-1] = zero
				s.items = s.items[:len(s.items)-1]
				delete(s.index, id)
				for i := existingIdx; i < len(s.items); i++ {
					s.index[s.identify(s.items[i])] = i
				}
				s.version.Increment()
				invalidations = append([]func(){}, s.invalidations...)
				s.mu.Unlock()
			},
			func() {
				if skipped {
					return
				}
				for _, fn := range invalidations {
					if fn != nil {
						fn()
					}
				}
				s.onRemove.Emit(CollectionRemoveEvent[T]{ID: id, Index: idx, Item: item})
			},
		)
		return
	}
	s.mu.Lock()
	idx, ok := s.index[id]
	if !ok || idx < 0 || idx >= len(s.items) {
		s.mu.Unlock()
		return
	}
	item := s.items[idx]
	copy(s.items[idx:], s.items[idx+1:])
	var zero T
	s.items[len(s.items)-1] = zero
	s.items = s.items[:len(s.items)-1]
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
	notify()
}

func (s *CollectionStore[T]) update(item T, tx *Transaction) {
	assertNotProjecting()
	id := s.identify(item)
	if tx != nil {
		var old T
		var idx int
		var invalidations []func()
		var skipped bool
		tx.stage(
			func() {
				s.mu.Lock()
				existingIdx, ok := s.index[id]
				if !ok || existingIdx < 0 || existingIdx >= len(s.items) {
					s.mu.Unlock()
					skipped = true
					return
				}
				idx = existingIdx
				old = s.items[existingIdx]
				s.items[existingIdx] = item
				s.version.Increment()
				invalidations = append([]func(){}, s.invalidations...)
				s.mu.Unlock()
			},
			func() {
				if skipped {
					return
				}
				for _, fn := range invalidations {
					if fn != nil {
						fn()
					}
				}
				s.onUpdate.Emit(CollectionUpdateEvent[T]{ID: id, Index: idx, Old: old, New: item})
			},
		)
		return
	}
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
	notify()
}

func (s *CollectionStore[T]) replace(items []T, tx *Transaction) {
	assertNotProjecting()
	if tx != nil {
		var invalidations []func()
		tx.stage(
			func() {
				s.mu.Lock()
				s.items = append([]T(nil), items...)
				s.index = make(map[ItemID]int, len(items))
				for i, item := range s.items {
					s.index[s.identify(item)] = i
				}
				s.version.Increment()
				invalidations = append([]func(){}, s.invalidations...)
				s.mu.Unlock()
			},
			func() {
				for _, fn := range invalidations {
					if fn != nil {
						fn()
					}
				}
				s.onReplace.Emit(signal.Fired)
			},
		)
		return
	}
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
	notify()
}

func (s *CollectionStore[T]) addInvalidationTarget(fn func()) {
	if fn == nil {
		return
	}
	s.mu.Lock()
	s.invalidations = append(s.invalidations, fn)
	s.mu.Unlock()
}

// OnInsertSubscribe subscribes handler to Insert events and returns an unsubscribe function.
func (s *CollectionStore[T]) OnInsertSubscribe(handler func(CollectionInsertEvent[T])) func() {
	id := s.onInsert.Subscribe(handler)
	return func() { s.onInsert.Unsubscribe(id) }
}

// OnRemoveSubscribe subscribes handler to Remove events and returns an unsubscribe function.
func (s *CollectionStore[T]) OnRemoveSubscribe(handler func(CollectionRemoveEvent[T])) func() {
	id := s.onRemove.Subscribe(handler)
	return func() { s.onRemove.Unsubscribe(id) }
}

// OnUpdateSubscribe subscribes handler to Update events and returns an unsubscribe function.
func (s *CollectionStore[T]) OnUpdateSubscribe(handler func(CollectionUpdateEvent[T])) func() {
	id := s.onUpdate.Subscribe(handler)
	return func() { s.onUpdate.Unsubscribe(id) }
}

// OnReplaceSubscribe subscribes handler to Replace events and returns an unsubscribe function.
func (s *CollectionStore[T]) OnReplaceSubscribe(handler func(signal.Unit)) func() {
	id := s.onReplace.Subscribe(handler)
	return func() { s.onReplace.Unsubscribe(id) }
}
