package store

import "codeburg.org/lexbit/lurpicui/internal/syncutil"

// Transaction batches store mutations and defers signal delivery until Commit.
type Transaction struct {
	deferred  []func()
	committed bool
}

// Begin starts a transaction on the runtime thread.
func Begin() *Transaction {
	syncutil.AssertRuntimeThread()
	return &Transaction{}
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
	if t.committed {
		panic("store: Commit called on completed transaction")
	}
	t.committed = true
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
	if t == nil || t.committed {
		return
	}
	t.committed = true
	t.deferred = nil
}
