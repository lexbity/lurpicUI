package store

import (
	"sync/atomic"

	"codeburg.org/lexbit/lurpicui/internal/syncutil"
)

// Transaction batches store mutations and defers signal delivery until Commit.
type Transaction struct {
	mutations []func()
	deferred  []func()
	committed atomic.Bool
}

// Begin starts a transaction on the runtime thread.
func Begin() *Transaction {
	syncutil.AssertRuntimeThread()
	return &Transaction{}
}

func (t *Transaction) stage(mutate func(), notify func()) {
	if t == nil {
		if mutate != nil {
			mutate()
		}
		if notify != nil {
			notify()
		}
		return
	}
	if mutate != nil {
		t.mutations = append(t.mutations, mutate)
	}
	if notify != nil {
		t.deferred = append(t.deferred, notify)
	}
}

func (t *Transaction) deferCall(fn func()) {
	if t == nil || fn == nil {
		return
	}
	t.deferred = append(t.deferred, fn)
}

// Commit fires all deferred signals.
func (t *Transaction) Commit() {
	syncutil.AssertRuntimeThread()
	if t == nil {
		return
	}
	if !t.committed.CompareAndSwap(false, true) {
		panic("store: Commit called on completed transaction")
	}
	mutations := t.mutations
	t.mutations = nil
	for _, mutate := range mutations {
		if mutate != nil {
			mutate()
		}
	}
	deferred := t.deferred
	t.deferred = nil
	for _, fn := range deferred {
		if fn != nil {
			fn()
		}
	}
}

// Rollback discards deferred notifications without firing them.
func (t *Transaction) Rollback() {
	syncutil.AssertRuntimeThread()
	if t == nil || !t.committed.CompareAndSwap(false, true) {
		return
	}
	for i := range t.mutations {
		t.mutations[i] = nil
	}
	for i := range t.deferred {
		t.deferred[i] = nil
	}
	t.mutations = nil
	t.deferred = nil
}
