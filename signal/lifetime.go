package signal

// Subscriptions is a collection of active subscriptions owned by one object.
// Call Release on disposal to unsubscribe everything at once.
type Subscriptions struct {
	entries []func()
}

// Add registers a cleanup function for one subscription.
func (s *Subscriptions) Add(release func()) {
	if release == nil {
		return
	}
	s.entries = append(s.entries, release)
}

// Release unsubscribes all registered subscriptions and clears the bag.
// It is idempotent.
func (s *Subscriptions) Release() {
	if len(s.entries) == 0 {
		return
	}
	entries := s.entries
	s.entries = s.entries[:0]
	for _, release := range entries {
		if release != nil {
			release()
		}
	}
}

// Len returns the number of active subscriptions tracked.
func (s *Subscriptions) Len() int {
	return len(s.entries)
}

// Track subscribes to a signal and registers the cleanup with a Subscriptions bag.
func Track[T any](bag *Subscriptions, sig *Signal[T], handler func(T)) {
	if sig == nil {
		return
	}
	id := sig.Subscribe(handler)
	if bag != nil {
		bag.Add(func() {
			sig.Unsubscribe(id)
		})
	}
}

// Unit is the payload type for signals that carry no data.
type Unit struct{}

// Fired is the canonical zero payload value for Unit signals.
var Fired = Unit{}

// Change carries old and new values for signals that describe state transitions.
type Change[T any] struct {
	Old T
	New T
}

// CollectionEventKind classifies a change to an ordered collection.
type CollectionEventKind uint8

const (
	CollectionInserted CollectionEventKind = iota
	CollectionRemoved
	CollectionUpdated
	CollectionReplaced
)

// CollectionEvent describes a single change to a CollectionStore.
type CollectionEvent[T any] struct {
	Kind    CollectionEventKind
	Index   int
	Item    T
	OldItem T
}
