package facet

import (
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// Sub is a subscription builder bound to one facet's OnAttach.
// It owns both the subscription cleanup bag and the version tracking
// slots used by the projection cache key.
//
// Obtain via Subscribe. All operations on a nil *Sub are no-ops.
type Sub struct {
	subs *signal.Subscriptions
	base *Facet
}

// Subscribe returns a Sub builder bound to f's subscription bag and
// version slots. Safe to call with a nil FacetImpl — returns a no-op builder.
func Subscribe(f FacetImpl) *Sub {
	if f == nil {
		return &Sub{}
	}
	base := f.Base()
	if base == nil {
		return &Sub{}
	}
	return &Sub{subs: base.Subs(), base: base}
}

// Collect adds an opaque unsubscribe function to the subscription bag.
// Use for stores whose subscription API returns a func() rather than
// a *signal.Signal (e.g. CollectionStore.OnReplaceSubscribe).
// Returns s for call-site grouping.
func (s *Sub) Collect(unsubFn func()) *Sub {
	if s == nil || s.subs == nil || unsubFn == nil {
		return s
	}
	s.subs.Add(unsubFn)
	return s
}

// To subscribes handler to sig and registers the cleanup in the builder's bag.
// Use for non-store signals or signals where cache-key participation is not needed.
// Returns s for call-site grouping.
func To[T any](s *Sub, sig *signal.Signal[T], handler func(T)) *Sub {
	if s == nil || sig == nil {
		return s
	}
	signal.Track(s.subs, sig, handler)
	return s
}

// Store subscribes handler to sig and registers versionFn in the facet's
// version tracking slots. The tracked slot is refreshed on every delivery.
// Returns s for call-site grouping.
func Store[T any](s *Sub, sig *signal.Signal[T], versionFn func() store.Version, handler func(T)) *Sub {
	if s == nil || sig == nil {
		return s
	}
	slot := -1
	if s.base != nil && versionFn != nil {
		slot = s.base.TrackVersion(versionFn)
	}
	signal.Track(s.subs, sig, func(v T) {
		if s.base != nil && slot >= 0 && versionFn != nil {
			s.base.UpdateTrackedVersion(slot, versionFn)
		}
		if handler != nil {
			handler(v)
		}
	})
	return s
}
