package signal

import (
	"testing"
)

func TestSignal_subscribe_and_emit(t *testing.T) {
	s := NewSignal[int]("numbers")
	var got []int
	s.Subscribe(func(v int) { got = append(got, v) })

	s.Emit(42)

	if len(got) != 1 || got[0] != 42 {
		t.Fatalf("got %#v", got)
	}
}

func TestSignal_multiple_subscribers_fire_in_order(t *testing.T) {
	s := NewSignal[int]("numbers")
	var got []int
	s.Subscribe(func(v int) { got = append(got, v+1) })
	s.Subscribe(func(v int) { got = append(got, v+2) })
	s.Subscribe(func(v int) { got = append(got, v+3) })

	s.Emit(10)

	want := []int{11, 12, 13}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestSignal_unsubscribe_removes_handler(t *testing.T) {
	s := NewSignal[int]("numbers")
	var called int
	id := s.Subscribe(func(int) { called++ })
	s.Unsubscribe(id)

	s.Emit(1)

	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestSignal_unsubscribe_unknown_id_is_noop(t *testing.T) {
	s := NewSignal[int]("numbers")
	s.Unsubscribe(999)
	s.Emit(1)
}

func TestSignal_unsubscribeAll_clears_all(t *testing.T) {
	s := NewSignal[int]("numbers")
	called := 0
	s.Subscribe(func(int) { called++ })
	s.Subscribe(func(int) { called++ })
	s.UnsubscribeAll()

	if s.HasSubscribers() {
		t.Fatal("expected no subscribers")
	}
	s.Emit(1)
	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestSignal_hasSubscribers_accurate(t *testing.T) {
	s := NewSignal[int]("numbers")
	if s.HasSubscribers() {
		t.Fatal("expected empty signal")
	}
	id := s.Subscribe(func(int) {})
	if !s.HasSubscribers() {
		t.Fatal("expected subscribers")
	}
	s.Unsubscribe(id)
	if s.HasSubscribers() {
		t.Fatal("expected no subscribers after unsubscribe")
	}
}

func TestSignal_reentrant_emit_safe(t *testing.T) {
	s := NewSignal[int]("numbers")
	var got []int
	s.Subscribe(func(v int) {
		got = append(got, v)
		if v == 1 {
			s.Emit(2)
		}
	})

	s.Emit(1)

	want := []int{1, 2}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestSignal_unsubscribe_during_emit_safe(t *testing.T) {
	s := NewSignal[int]("numbers")
	var got []int
	var selfID SubscriptionID
	selfID = s.Subscribe(func(v int) {
		got = append(got, v)
		s.Unsubscribe(selfID)
	})
	s.Subscribe(func(v int) { got = append(got, v+10) })

	s.Emit(5)
	s.Emit(6)

	want := []int{5, 15, 16}
	if len(got) != len(want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %#v want %#v", got, want)
		}
	}
}

func TestSignal_subscription_ids_unique(t *testing.T) {
	s := NewSignal[int]("numbers")
	ids := []SubscriptionID{
		s.Subscribe(func(int) {}),
		s.Subscribe(func(int) {}),
		s.Subscribe(func(int) {}),
	}
	seen := make(map[SubscriptionID]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			t.Fatalf("duplicate id %v", id)
		}
		seen[id] = struct{}{}
	}
}

func TestSignal_emit_no_subscribers_is_noop(t *testing.T) {
	s := NewSignal[int]("numbers")
	s.Emit(1)
}

func TestSignal_name_stored(t *testing.T) {
	s := NewSignal[int]("my-signal")
	if got := s.Name(); got != "my-signal" {
		t.Fatalf("got %q", got)
	}
}
