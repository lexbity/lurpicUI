package store

import "sync"

type invalidationSlot struct {
	id uint64
	fn func()
}

type invalidationSet struct {
	mu    sync.Mutex
	slots []invalidationSlot
	free  []int
	next  uint64
}

func (s *invalidationSet) add(fn func()) Subscription {
	if fn == nil {
		return Subscription{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.next++
	if s.next == 0 {
		panic("store: invalidation subscription generation exhausted")
	}
	id := s.next
	var index int
	if n := len(s.free); n != 0 {
		index = s.free[n-1]
		s.free = s.free[:n-1]
		s.slots[index] = invalidationSlot{id: id, fn: fn}
	} else {
		index = len(s.slots)
		s.slots = append(s.slots, invalidationSlot{id: id, fn: fn})
	}
	return Subscription{release: func() {
		s.mu.Lock()
		defer s.mu.Unlock()
		if index < 0 || index >= len(s.slots) || s.slots[index].id != id {
			return
		}
		s.slots[index] = invalidationSlot{}
		s.free = append(s.free, index)
	}}
}

func (s *invalidationSet) snapshot() []func() {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.slots) == 0 {
		return nil
	}
	callbacks := make([]func(), 0, len(s.slots)-len(s.free))
	for index, slot := range s.slots {
		if slot.fn == nil {
			continue
		}
		slotIndex := index
		id := slot.id
		callbacks = append(callbacks, func() {
			s.mu.Lock()
			if slotIndex >= len(s.slots) || s.slots[slotIndex].id != id {
				s.mu.Unlock()
				return
			}
			fn := s.slots[slotIndex].fn
			s.mu.Unlock()
			if fn != nil {
				fn()
			}
		})
	}
	return callbacks
}

func (s *invalidationSet) clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.slots {
		s.slots[i] = invalidationSlot{}
	}
	s.slots = nil
	s.free = nil
}

// Subscription is an idempotent source-subscription release token.
type Subscription struct {
	once    sync.Once
	release func()
}

// Release unregisters the source callback once.
func (s *Subscription) Release() {
	if s == nil {
		return
	}
	s.once.Do(func() {
		if s.release != nil {
			s.release()
			s.release = nil
		}
	})
}
