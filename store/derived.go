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

// Derived memoizes a computation whose complete source set is declared at
// construction. Its owner scope schedules dirty work and owns every upstream
// release token.
type Derived[T any] struct {
	scope    *Scope
	version  VersionSource
	compute  func() T
	rank     uint64
	sequence uint64

	mu             sync.RWMutex
	value          T
	dirty          bool
	initialized    bool
	sourceVersions []Version
	sources        []versionedInvalidatable
	releases       []Subscription
	invalidations  invalidationSet

	OnChange      signal.Signal[signal.Change[T]]
	OnInvalidated signal.Signal[struct{}]
}

// NewDerived creates a lazy derived value owned by scope. Sources must belong
// to this scope or one of its ancestors; sibling and cross-runtime edges fail.
func NewDerived[T any](scope *Scope, compute func() T, sources ...Invalidatable) *Derived[T] {
	requireScope(scope)
	if compute == nil {
		panic("store: derived compute function is required")
	}
	d := &Derived[T]{
		scope:         scope,
		compute:       compute,
		dirty:         true,
		OnChange:      signal.NewSignal[signal.Change[T]]("Derived.OnChange"),
		OnInvalidated: signal.NewSignal[struct{}]("Derived.OnInvalidated"),
	}
	for _, source := range sources {
		if source == nil {
			panic("store: nil derived source")
		}
		vs, ok := source.(versionedInvalidatable)
		if !ok {
			panic(fmt.Sprintf("store: Derived source %T does not implement Version()", source))
		}
		sourceScope := vs.storeScope()
		if sourceScope == nil || !sourceScope.ancestorOf(scope) {
			panic("store: derived dependency must be in the same scope or an ancestor")
		}
		d.sources = append(d.sources, vs)
		if parent, ok := source.(interface{ scopeRank() uint64 }); ok && parent.scopeRank()+1 > d.rank {
			d.rank = parent.scopeRank() + 1
		}
	}
	d.sequence = scope.registerDerived(d)
	for _, source := range d.sources {
		d.releases = append(d.releases, source.addInvalidationTarget(d.markDirty))
	}
	scope.addResource(d)
	return d
}

// Get returns the current derived value, recomputing it when dirty.
func (d *Derived[T]) Get() T {
	d.scope.assertDerivedReadAllowed(d)
	d.scope.recordDerivedGet(false)
	d.mu.RLock()
	if d.initialized && !d.dirty && !d.sourcesChangedLocked() {
		value := d.value
		d.mu.RUnlock()
		return value
	}
	compute := d.compute
	d.mu.RUnlock()
	if compute == nil {
		var zero T
		return zero
	}

	d.scope.pushComputing(d)
	var next T
	func() {
		defer d.scope.popComputing(d)
		next = compute()
	}()
	d.mu.Lock()
	old := d.value
	d.value = next
	d.initialized = true
	d.dirty = false
	d.sourceVersions = d.snapshotSourceVersionsLocked()
	d.version.Increment()
	invalidations := d.invalidations.snapshot()
	d.mu.Unlock()
	d.scope.recordDerivedRecompute()
	for _, fn := range invalidations {
		fn()
	}
	d.scope.enqueueSignal(func() {
		d.OnChange.Emit(signal.Change[T]{Old: old, New: next})
	})
	return next
}

// Peek returns the last computed value and whether the derived is dirty.
func (d *Derived[T]) Peek() (T, bool) {
	d.scope.assertUsable()
	d.mu.RLock()
	defer d.mu.RUnlock()
	return d.value, d.dirty || (d.initialized && d.sourcesChangedLocked())
}

// Version returns the version of the last computed value.
func (d *Derived[T]) Version() Version {
	d.scope.assertUsable()
	return d.version.Current()
}

func (d *Derived[T]) markDirty() {
	d.mu.Lock()
	wasClean := d.initialized && !d.dirty
	d.dirty = true
	d.mu.Unlock()
	d.scope.enqueueDerived(d)
	if wasClean {
		invalidations := d.invalidations.snapshot()
		d.scope.enqueueSignal(func() {
			for _, fn := range invalidations {
				fn()
			}
			d.OnInvalidated.Emit(struct{}{})
		})
	}
}

func (d *Derived[T]) sourcesChangedLocked() bool {
	if len(d.sources) != len(d.sourceVersions) {
		return true
	}
	for i, source := range d.sources {
		if source.Version() != d.sourceVersions[i] {
			return true
		}
	}
	return false
}

func (d *Derived[T]) snapshotSourceVersionsLocked() []Version {
	if len(d.sources) == 0 {
		return []Version{}
	}
	versions := make([]Version, len(d.sources))
	for i, source := range d.sources {
		versions[i] = source.Version()
	}
	return versions
}

func (d *Derived[T]) addInvalidationTarget(fn func()) Subscription {
	return d.invalidations.add(fn)
}

func (d *Derived[T]) storeScope() *Scope    { return d.scope }
func (d *Derived[T]) OwnerScope() *Scope    { return d.scope }
func (d *Derived[T]) scopeRank() uint64     { return d.rank }
func (d *Derived[T]) scopeSequence() uint64 { return d.sequence }

func (d *Derived[T]) declares(source any) bool {
	for _, declared := range d.sources {
		if declared == source {
			return true
		}
	}
	return false
}

func (d *Derived[T]) flushIfDirty() bool {
	d.mu.RLock()
	dirty := !d.initialized || d.dirty || d.sourcesChangedLocked()
	d.mu.RUnlock()
	if !dirty {
		return false
	}
	d.Get()
	return true
}

func (d *Derived[T]) closeScopeResource() {
	d.mu.Lock()
	for i := range d.releases {
		d.releases[i].Release()
	}
	d.releases = nil
	for i := range d.sources {
		d.sources[i] = nil
	}
	d.sources = nil
	d.sourceVersions = nil
	d.compute = nil
	var zero T
	d.value = zero
	d.initialized = false
	d.dirty = true
	d.invalidations.clear()
	d.OnChange.UnsubscribeAll()
	d.OnInvalidated.UnsubscribeAll()
	d.mu.Unlock()
}
