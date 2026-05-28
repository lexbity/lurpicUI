package store

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/signal"
)

type versionedInvalidatable interface {
	Invalidatable
	Version() Version
}

// Derived is a read-only computed store.
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

	OnChange signal.Signal[signal.Change[T]]
}

func NewDerived[T any](compute func() T, sources ...Invalidatable) *Derived[T] {
	d := &Derived[T]{
		compute:  compute,
		dirty:    true,
		OnChange: signal.NewSignal[signal.Change[T]]("Derived.OnChange"),
	}
	if len(sources) > 0 {
		d.sources = make([]versionedInvalidatable, 0, len(sources))
		for _, src := range sources {
			vs, ok := src.(versionedInvalidatable)
			if !ok {
				continue
			}
			d.sources = append(d.sources, vs)
			vs.addInvalidationTarget(d.markDirty)
		}
	}
	return d
}

// Get returns the current derived value, recomputing if dirty or stale.
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

func (d *Derived[T]) markDirty() {
	d.mu.Lock()
	d.dirty = true
	d.mu.Unlock()
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
		return nil
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
