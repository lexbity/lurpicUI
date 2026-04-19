package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestTransaction_defers_signals_until_commit(t *testing.T) {
	s := NewValueStore(1)
	tx := Begin()
	called := 0
	s.OnChange.Subscribe(func(signal.Change[int]) { called++ })

	s.SetTx(2, tx)
	if called != 0 {
		t.Fatalf("called before commit = %d", called)
	}

	tx.Commit()
	if called != 1 {
		t.Fatalf("called after commit = %d", called)
	}
}

func TestTransaction_signals_fire_in_mutation_order(t *testing.T) {
	a := NewValueStore(1)
	b := NewValueStore(2)
	tx := Begin()
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

func TestTransaction_rollback_suppresses_signals(t *testing.T) {
	s := NewValueStore(1)
	tx := Begin()
	called := 0
	s.OnChange.Subscribe(func(signal.Change[int]) { called++ })

	s.SetTx(2, tx)
	tx.Rollback()

	if got := s.Get(); got != 2 {
		t.Fatalf("mutation should remain in effect, got %d", got)
	}
	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestTransaction_mutations_immediate(t *testing.T) {
	s := NewValueStore(1)
	tx := Begin()
	s.SetTx(2, tx)
	if got := s.Get(); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestTransaction_commit_after_commit_panics(t *testing.T) {
	tx := Begin()
	tx.Commit()
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	tx.Commit()
}

func TestTransaction_multi_store_atomic(t *testing.T) {
	a := NewValueStore(1)
	b := NewValueStore(2)
	tx := Begin()
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
