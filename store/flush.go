package store

import (
	"sync"
	"sync/atomic"
)

// derivedFlushable is the narrow view the frame flush needs from a Derived:
// recompute it when its sources changed, reporting whether a recompute ran.
type derivedFlushable interface {
	flushIfDirty() bool
}

// The derived registry. Deriveds register at construction; the runtime's frame
// loop flushes the registry before the projection phase so projection reads
// stable, freshly-composed values instead of tripping lazy recomputes mid-walk
// (RX-1 P5: flush → compose → version).
var (
	derivedMu sync.Mutex
	deriveds  []derivedFlushable
)

func registerDerived(d derivedFlushable) {
	derivedMu.Lock()
	deriveds = append(deriveds, d)
	derivedMu.Unlock()
}

// Frame-derived counters, reset once per frame by the runtime.
var (
	frameDerivedEvals      atomic.Uint64
	frameDerivedRecomputes atomic.Uint64
)

// ResetFrameDerivedCounters zeroes the frame's derived counters. The runtime
// calls it at the start of each frame so the flush and any lazy Get during the
// frame accumulate into one frame-scoped total.
func ResetFrameDerivedCounters() {
	frameDerivedEvals.Store(0)
	frameDerivedRecomputes.Store(0)
}

// SnapshotFrameDerivedCounters returns the frame's derived activity so far:
// evaluated (Get calls) and recomputed (compute calls).
func SnapshotFrameDerivedCounters() (evals, recomputes uint64) {
	return frameDerivedEvals.Load(), frameDerivedRecomputes.Load()
}

// FlushResult reports the derived flush's effect for the frame.
type FlushResult struct {
	Evaluated  int
	Recomputed int
}

// FlushDerived recomputes every registered derived whose sources changed since
// its last computation. It runs before the projection phase: the flush
// composes values and bumps versions up front, so projection never observes a
// mid-frame source flip (RX-1 P5).
func FlushDerived() FlushResult {
	derivedMu.Lock()
	list := append([]derivedFlushable(nil), deriveds...)
	derivedMu.Unlock()

	var res FlushResult
	for _, d := range list {
		if d.flushIfDirty() {
			res.Evaluated++
			res.Recomputed++
		}
	}
	return res
}

// flushIfDirty recomputes this derived when it is dirty or its source versions
// changed, reporting whether a recompute ran. A clean derived is skipped with
// no Get call, so a quiet steady state costs one flag check per registered
// derived and contributes nothing to the frame's eval/recompute counters.
func (d *Derived[T]) flushIfDirty() bool {
	d.mu.RLock()
	dirty := !d.initialized || d.dirty
	d.mu.RUnlock()
	if !dirty {
		return false
	}
	d.Get()
	return true
}
