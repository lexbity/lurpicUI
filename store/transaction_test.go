package store

import (
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestTransaction_deferCall_order_and_nil_safe(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	tx := Begin(scope)
	var got []string

	tx.deferCall(nil)
	tx.deferCall(func() { got = append(got, "first") })
	tx.deferCall(func() { got = append(got, "second") })
	tx.Commit()
	scope.FlushSignals()

	want := []string{"first", "second"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestTransaction_defers_signals_until_commit(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	s := NewValueStore(scope, 1)
	tx := Begin(scope)
	called := 0
	s.OnChange.Subscribe(func(signal.Change[int]) { called++ })

	s.SetTx(2, tx)
	if called != 0 {
		t.Fatalf("called before commit = %d", called)
	}

	tx.Commit()
	if called != 0 {
		t.Fatalf("called before signal boundary = %d", called)
	}
	scope.FlushSignals()
	if called != 1 {
		t.Fatalf("called after commit = %d", called)
	}
}

func TestTransaction_signals_fire_in_mutation_order(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	a := NewValueStore(scope, 1)
	b := NewValueStore(scope, 2)
	tx := Begin(scope)
	var got []string
	a.OnChange.Subscribe(func(signal.Change[int]) {
		got = append(got, "a")
	})
	b.OnChange.Subscribe(func(signal.Change[int]) {
		got = append(got, "b")
	})

	a.SetTx(10, tx)
	b.SetTx(20, tx)
	tx.Commit()
	scope.FlushSignals()

	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestTransaction_rollback_discards_mutations(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	s := NewValueStore(scope, 1)
	tx := Begin(scope)
	called := 0
	s.OnChange.Subscribe(func(signal.Change[int]) { called++ })

	s.SetTx(2, tx)
	tx.Rollback()

	if got := s.Get(); got != 1 {
		t.Fatalf("mutation should be discarded, got %d", got)
	}
	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestTransaction_mutations_staged(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	s := NewValueStore(scope, 1)
	tx := Begin(scope)
	s.SetTx(2, tx)
	if got := s.Get(); got != 1 {
		t.Fatalf("got %d, expected staged value of 1", got)
	}
	tx.Commit()
	scope.FlushSignals()
	if got := s.Get(); got != 2 {
		t.Fatalf("got %d, expected committed value of 2", got)
	}
}

func TestTransaction_commit_after_commit_panics(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	tx := Begin(scope)
	tx.Commit()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.Commit()
}

func TestTransaction_multi_store_atomic(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	a := NewValueStore(scope, 1)
	b := NewValueStore(scope, 2)
	tx := Begin(scope)
	var got []string
	a.OnChange.Subscribe(func(signal.Change[int]) {
		got = append(got, "a")
		if a.Get() != 10 || b.Get() != 20 {
			t.Fatalf("observed stale values a=%d b=%d", a.Get(), b.Get())
		}
	})
	b.OnChange.Subscribe(func(signal.Change[int]) {
		got = append(got, "b")
		if a.Get() != 10 || b.Get() != 20 {
			t.Fatalf("observed stale values a=%d b=%d", a.Get(), b.Get())
		}
	})

	a.SetTx(10, tx)
	b.SetTx(20, tx)
	tx.Commit()
	scope.FlushSignals()

	want := []string{"a", "b"}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestScopeQueueSignal_defers_until_signal_boundary(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	called := 0
	scope.QueueSignal(func() { called++ })
	if called != 0 {
		t.Fatalf("called before flushing queue = %d", called)
	}
	if queued := scope.FlushSignals(); queued != 1 {
		t.Fatalf("queued = %d", queued)
	}
	if called != 1 {
		t.Fatalf("called after flush = %d", called)
	}

}

func TestTransaction_commit_is_atomic_under_concurrent_access(t *testing.T) {
	scope := NewScope()
	defer scope.Close()
	tx := Begin(scope)
	var successes atomic.Int64
	var panics atomic.Int64
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
				}
			}()
			tx.Commit()
			successes.Add(1)
		}()
	}
	wg.Wait()
	if successes.Load() != 1 {
		t.Fatalf("expected exactly 1 successful commit, got %d", successes.Load())
	}
	if panics.Load() != 9 {
		t.Fatalf("expected 9 panics from concurrent commits, got %d", panics.Load())
	}
}

func TestVersionSource_nil_panics(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic from nil VersionSource")
		}
	}()
	var vs *VersionSource
	vs.Current()
}
