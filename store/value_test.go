package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

func TestVersionSource_starts_at_zero(t *testing.T) {
	var vs VersionSource
	if got := vs.Current(); got != 0 {
		t.Fatalf("current = %d", got)
	}
}

func TestVersionSource_increment_monotonic(t *testing.T) {
	var vs VersionSource
	a := vs.Increment()
	b := vs.Increment()
	if a == 0 || b == 0 {
		t.Fatalf("unexpected zero versions: %d %d", a, b)
	}
	if !(b > a) {
		t.Fatalf("expected monotonic increase: %d then %d", a, b)
	}
}

func TestVersionSource_zero_is_null(t *testing.T) {
	s := NewValueStore(1)
	if got := s.Version(); got != 0 {
		t.Fatalf("initial version = %d", got)
	}
	s.Set(2)
	if got := s.Version(); got == 0 {
		t.Fatal("version should not be zero after mutation")
	}
}

func TestValueStore_get_initial_value(t *testing.T) {
	s := NewValueStore(42)
	if got := s.Get(); got != 42 {
		t.Fatalf("got %d", got)
	}
}

func TestValueStore_set_changes_value(t *testing.T) {
	s := NewValueStore(42)
	s.Set(99)
	if got := s.Get(); got != 99 {
		t.Fatalf("got %d", got)
	}
}

func TestValueStore_set_increments_version(t *testing.T) {
	s := NewValueStore(42)
	ver := s.Version()
	s.Set(99)
	if got := s.Version(); got <= ver {
		t.Fatalf("version did not increment: %d -> %d", ver, got)
	}
}

func TestValueStore_onchange_fires_with_old_and_new(t *testing.T) {
	s := NewValueStore(10)
	var got signal.Change[int]
	s.OnChange.Subscribe(func(c signal.Change[int]) { got = c })

	s.Set(20)

	if got.Old != 10 || got.New != 20 {
		t.Fatalf("got %#v", got)
	}
}

func TestValueStore_multiple_sets_correct_change_chain(t *testing.T) {
	// immediate-delivery mode only
	s := NewValueStore(1)
	var got []signal.Change[int]
	s.OnChange.Subscribe(func(c signal.Change[int]) { got = append(got, c) })

	s.Set(2)
	s.Set(3)
	s.Set(4)

	want := []signal.Change[int]{
		{Old: 1, New: 2},
		{Old: 2, New: 3},
		{Old: 3, New: 4},
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestValueStore_get_from_goroutine_safe(t *testing.T) {
	s := NewValueStore(42)
	done := make(chan int, 1)
	go func() {
		done <- s.Get()
	}()
	if got := <-done; got != 42 {
		t.Fatalf("got %d", got)
	}
}

func TestValueStore_setTx_defers_signal_until_commit(t *testing.T) {
	s := NewValueStore(1)
	tx := Begin()
	called := 0
	s.OnChange.Subscribe(func(signal.Change[int]) { called++ })

	s.SetTx(2, tx)
	if called != 0 {
		t.Fatalf("called before commit = %d", called)
	}
	if got := s.Get(); got != 1 {
		t.Fatalf("get before commit = %d", got)
	}

	tx.Commit()
	if called != 1 {
		t.Fatalf("called after commit = %d", called)
	}
	if got := s.Get(); got != 2 {
		t.Fatalf("get after commit = %d", got)
	}
}

func TestValueStore_setTx_staged_mutation(t *testing.T) {
	s := NewValueStore(1)
	tx := Begin()
	s.SetTx(2, tx)
	if got := s.Get(); got != 1 {
		t.Fatalf("get before commit = %d", got)
	}
	if got := s.Version(); got != 0 {
		t.Fatal("version should be zero before Commit")
	}
	tx.Rollback()
	if got := s.Get(); got != 1 {
		t.Fatalf("get after rollback = %d", got)
	}
}

func TestValueStore_interface_implementation(t *testing.T) {
	var _ Invalidatable = (*ValueStore[int])(nil)
}

func TestValueStore_change_chain_under_queued_hook(t *testing.T) {
	var queue []func()
	SetSignalQueueHook(func(fn func()) {
		queue = append(queue, fn)
	})
	defer SetSignalQueueHook(nil)

	s := NewValueStore(1)
	var got []signal.Change[int]
	s.OnChange.Subscribe(func(c signal.Change[int]) { got = append(got, c) })

	// These Sets should NOT deliver signals immediately — they queue.
	s.Set(2)
	s.Set(3)
	s.Set(4)

	if len(got) != 0 {
		t.Fatalf("signals delivered before drain: %d", len(got))
	}

	// Drain the queue in order.
	for _, fn := range queue {
		fn()
	}

	want := []signal.Change[int]{
		{Old: 1, New: 2},
		{Old: 2, New: 3},
		{Old: 3, New: 4},
	}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}
