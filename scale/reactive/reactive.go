package reactive

import (
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// ReactiveScale wraps a *store.Derived[InvertibleScale] so that the scale
// is recomputed whenever its domain or range sources change. The underlying
// derived store satisfies the engine's version/invalidation contract, so
// facets that TrackVersion on it re-project on change.
type ReactiveScale struct {
	derived *store.Derived[scale.InvertibleScale]
}

// Get returns the current scale, recomputing if stale.
func (r *ReactiveScale) Get() scale.InvertibleScale {
	return r.derived.Get()
}

// Version returns the current version of the derived store.
func (r *ReactiveScale) Version() store.Version {
	return r.derived.Version()
}

// NewLinearReactive constructs a ReactiveScale over a LinearScale whose
// domain and range are driven by the given ValueStores. Fixed options
// (clamp, etc.) are captured at construction and reapplied on every
// recompute; domain and range from the stores always take precedence.
func NewLinearReactive(
	domain *store.ValueStore[[2]float64],
	rng *store.ValueStore[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	derived := store.NewDerived(
		func() scale.InvertibleScale {
			d := domain.Get()
			r := rng.Get()
			all := make([]scale.Option, 0, 2+len(opts))
			all = append(all, opts...)
			all = append(all,
				scale.WithDomain(d[0], d[1]),
				scale.WithRange(r[0], r[1]),
			)
			return scale.NewLinear(all...)
		},
		domain, rng,
	)
	return &ReactiveScale{derived: derived}
}

// NewLogReactive constructs a ReactiveScale over a LogScale. Panics if the
// initial scale construction fails (programming contract violation).
func NewLogReactive(
	domain *store.ValueStore[[2]float64],
	rng *store.ValueStore[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	derived := store.NewDerived(
		func() scale.InvertibleScale {
			d := domain.Get()
			r := rng.Get()
			all := make([]scale.Option, 0, 2+len(opts))
			all = append(all, opts...)
			all = append(all,
				scale.WithDomain(d[0], d[1]),
				scale.WithRange(r[0], r[1]),
			)
			s, err := scale.NewLog(all...)
			if err != nil {
				panic(err)
			}
			return s
		},
		domain, rng,
	)
	return &ReactiveScale{derived: derived}
}

// NewTimeReactive constructs a ReactiveScale over a TimeScale.
func NewTimeReactive(
	domain *store.ValueStore[[2]float64],
	rng *store.ValueStore[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	derived := store.NewDerived(
		func() scale.InvertibleScale {
			d := domain.Get()
			r := rng.Get()
			all := make([]scale.Option, 0, 2+len(opts))
			all = append(all, opts...)
			all = append(all,
				scale.WithDomain(d[0], d[1]),
				scale.WithRange(r[0], r[1]),
			)
			return scale.NewTime(all...)
		},
		domain, rng,
	)
	return &ReactiveScale{derived: derived}
}

// NewLinearReactiveFromDerived is like NewLinearReactive but accepts
// *store.Derived sources, enabling chaining from DomainFromCollection.
// OnChange on the source Deriveds is used to bridge to ValueStores for
// version tracking.
func NewLinearReactiveFromDerived(
	domain *store.Derived[[2]float64],
	rng *store.Derived[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	domainStore := bridgeDerived(domain)
	rngStore := bridgeDerived(rng)
	return NewLinearReactive(domainStore, rngStore, opts...)
}

// NewLogReactiveFromDerived is like NewLogReactive but accepts
// *store.Derived sources.
func NewLogReactiveFromDerived(
	domain *store.Derived[[2]float64],
	rng *store.Derived[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	domainStore := bridgeDerived(domain)
	rngStore := bridgeDerived(rng)
	return NewLogReactive(domainStore, rngStore, opts...)
}

// NewTimeReactiveFromDerived is like NewTimeReactive but accepts
// *store.Derived sources.
func NewTimeReactiveFromDerived(
	domain *store.Derived[[2]float64],
	rng *store.Derived[[2]float64],
	opts ...scale.Option,
) *ReactiveScale {
	domainStore := bridgeDerived(domain)
	rngStore := bridgeDerived(rng)
	return NewTimeReactive(domainStore, rngStore, opts...)
}

// bridgeDerived creates a ValueStore that mirrors the Derived's value.
// The Derived's OnChange signal is used to update the ValueStore whenever
// the Derived recomputes (which happens when Get() is called while dirty).
func bridgeDerived(d *store.Derived[[2]float64]) *store.ValueStore[[2]float64] {
	vs := store.NewValueStore(d.Get())
	d.OnChange.Subscribe(func(c signal.Change[[2]float64]) {
		vs.Set(c.New)
	})
	return vs
}
