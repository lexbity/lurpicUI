package store

import (
	"fmt"
	"sync"

	"codeburg.org/lexbit/lurpicui/signal"
)

type versionedInvalidatable interface {
	Invalidatable
	Version() Version
}

// Derived is a read-only computed store.
//
// It memoizes one computed value and marks itself dirty when any source store
// changes version. The actual recomputation still happens lazily on Get so the
// runtime pays the cost only when a consumer asks for the value.
//
// Derived exposes two signals:
//
//   - OnChange emits after a recomputation observes a new value (inside Get,
//     after the memoized value has been updated).
//   - OnInvalidated emits eagerly on the clean→dirty transition, before any
//     recomputation runs. It exists so consumers that need to react to a
//     potential value change (bindings, RX-1 FR-2) are not forced to wait for a
//     lazy Get to be called by somebody else — which is exactly what a
//     projection cache hit never does.
type Derived[T any] struct {
	version VersionSource
	compute func() T

	mu             sync.RWMutex
	value          T
	dirty          bool
	initialized    bool
	sourceVersions []Version
	sources        []versionedInvalidatable
	invalidations  []func()

	OnChange      signal.Signal[signal.Change[T]]
	OnInvalidated signal.Signal[struct{}]
}

func NewDerived[T any](compute func() T, sources ...Invalidatable) *Derived[T] {
	d := &Derived[T]{
		compute:       compute,
		dirty:         true,
		OnChange:      signal.NewSignal[signal.Change[T]]("Derived.OnChange"),
		OnInvalidated: signal.NewSignal[struct{}]("Derived.OnInvalidated"),
	}
	if len(sources) > 0 {
		d.sources = make([]versionedInvalidatable, 0, len(sources))
		for _, src := range sources {
			vs, ok := src.(versionedInvalidatable)
			if !ok {
				panic(fmt.Sprintf("store: Derived source %T does not implement Version() — all Derived sources must be versioned", src))
			}
			d.sources = append(d.sources, vs)
			vs.addInvalidationTarget(d.markDirty)
		}
	}
	return d
}

// Get returns the current derived value, recomputing if dirty or stale.
// The version snapshot is updated only after recomputation succeeds so chained
// derived stores can tell whether they are still reading the same source state.
func (d *Derived[T]) Get() T {
	d.mu.RLock()
	if d.initialized && !d.dirty && !d.sourcesChangedLocked() {
		value := d.value
		d.mu.RUnlock()
		return value
	}
	d.mu.RUnlock()

	if d.compute == nil {
		var zero T
		return zero
	}

	next := d.compute()
	d.mu.Lock()
	old := d.value
	d.value = next
	d.initialized = true
	d.dirty = false
	d.sourceVersions = d.snapshotSourceVersionsLocked()
	d.version.Increment()
	invalidations := append([]func(){}, d.invalidations...)
	d.mu.Unlock()
	for _, fn := range invalidations {
		if fn != nil {
			fn()
		}
	}
	enqueueSignal(func() {
		d.OnChange.Emit(signal.Change[T]{Old: old, New: next})
	})
	return next
}

// Version returns the version of the last computed value.
func (d *Derived[T]) Version() Version {
	return d.version.Current()
}

// markDirty flags the Derived dirty and emits OnInvalidated exactly once per
// clean→dirty transition.
//
// The emission is deferred through enqueueSignal so it always runs in the
// signal-delivery phase of the owning runtime (or immediately in unit tests
// without a queue hook) — never from the writer's stack. N upstream writes
// inside one dirty period coalesce into a single notification because the
// transition is detected under the same mutex that guards the dirty flag.
//
// A never-initialized Derived (no Get has run) is never "clean", so a write
// before the first Get emits nothing: there are no consumers yet, and the
// first projection is always uncached and therefore fresh (RX-1 FR-2 corollary).
func (d *Derived[T]) markDirty() {
	d.mu.Lock()
	wasClean := d.initialized && !d.dirty
	d.dirty = true
	d.mu.Unlock()
	if wasClean {
		enqueueSignal(func() {
			d.OnInvalidated.Emit(struct{}{})
		})
	}
}

func (d *Derived[T]) sourcesChangedLocked() bool {
	if len(d.sources) != len(d.sourceVersions) {
		return true
	}
	for i, src := range d.sources {
		if src.Version() != d.sourceVersions[i] {
			return true
		}
	}
	return false
}

func (d *Derived[T]) snapshotSourceVersionsLocked() []Version {
	if len(d.sources) == 0 {
		return []Version{}
	}
	out := make([]Version, len(d.sources))
	for i, src := range d.sources {
		out[i] = src.Version()
	}
	return out
}

func (d *Derived[T]) addInvalidationTarget(fn func()) {
	if fn == nil {
		return
	}
	d.mu.Lock()
	d.invalidations = append(d.invalidations, fn)
	d.mu.Unlock()
}
