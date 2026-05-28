package signal

// Signal is a typed, named event emitter.
// It is not safe for concurrent use.
type Signal[T any] struct {
	name        string
	subscribers []subscription[T]
	nextID      SubscriptionID
}

type subscription[T any] struct {
	id      SubscriptionID
	handler func(T)
}

// SubscriptionID identifies a registered handler.
type SubscriptionID uint64

// NewSignal constructs a Signal with the provided name.
func NewSignal[T any](name string) Signal[T] {
	return Signal[T]{name: name}
}

// Name returns the diagnostics name associated with the signal.
func (s *Signal[T]) Name() string {
	return s.name
}

// Subscribe registers a handler and returns its subscription ID.
func (s *Signal[T]) Subscribe(handler func(T)) SubscriptionID {
	s.nextID++
	id := s.nextID
	s.subscribers = append(s.subscribers, subscription[T]{id: id, handler: handler})
	return id
}

// Unsubscribe removes a handler by ID.
func (s *Signal[T]) Unsubscribe(id SubscriptionID) {
	if id == 0 {
		return
	}
	for i := range s.subscribers {
		if s.subscribers[i].id == id {
			s.subscribers = append(s.subscribers[:i], s.subscribers[i+1:]...)
			return
		}
	}
}

// UnsubscribeAll removes all handlers.
func (s *Signal[T]) UnsubscribeAll() {
	s.subscribers = s.subscribers[:0]
}

// HasSubscribers reports whether any handlers are currently registered.
func (s *Signal[T]) HasSubscribers() bool {
	return len(s.subscribers) > 0
}

// Emit delivers value to all subscribers in registration order.
func (s *Signal[T]) Emit(value T) {
	if len(s.subscribers) == 0 {
		return
	}
	snapshot := append([]subscription[T](nil), s.subscribers...)
	for _, sub := range snapshot {
		if sub.handler != nil {
			sub.handler(value)
		}
	}
}
