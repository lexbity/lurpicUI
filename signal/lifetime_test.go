package signal

import "testing"

func TestSubscriptions_release_calls_all(t *testing.T) {
	var bag Subscriptions
	called := 0
	bag.Add(func() { called++ })
	bag.Add(func() { called++ })

	bag.Release()

	if called != 2 {
		t.Fatalf("called = %d", called)
	}
}

func TestSubscriptions_release_idempotent(t *testing.T) {
	var bag Subscriptions
	called := 0
	bag.Add(func() { called++ })

	bag.Release()
	bag.Release()

	if called != 1 {
		t.Fatalf("called = %d", called)
	}
}

func TestSubscriptions_len_accurate(t *testing.T) {
	var bag Subscriptions
	if got := bag.Len(); got != 0 {
		t.Fatalf("len = %d", got)
	}
	bag.Add(func() {})
	bag.Add(func() {})
	if got := bag.Len(); got != 2 {
		t.Fatalf("len = %d", got)
	}
	bag.Release()
	if got := bag.Len(); got != 0 {
		t.Fatalf("len = %d", got)
	}
}

func TestTrack_subscribes_and_registers(t *testing.T) {
	s := NewSignal[int]("numbers")
	var bag Subscriptions
	called := 0

	Track(&bag, &s, func(v int) {
		called = v
	})
	if got := bag.Len(); got != 1 {
		t.Fatalf("len = %d", got)
	}

	s.Emit(7)
	if called != 7 {
		t.Fatalf("called = %d", called)
	}

	bag.Release()
	s.Emit(8)
	if called != 7 {
		t.Fatalf("called after release = %d", called)
	}
}

func TestTrack_multiple_signals(t *testing.T) {
	s1 := NewSignal[int]("one")
	s2 := NewSignal[int]("two")
	var bag Subscriptions
	a := 0
	b := 0

	Track(&bag, &s1, func(v int) { a = v })
	Track(&bag, &s2, func(v int) { b = v })
	if got := bag.Len(); got != 2 {
		t.Fatalf("len = %d", got)
	}

	s1.Emit(1)
	s2.Emit(2)
	if a != 1 || b != 2 {
		t.Fatalf("a=%d b=%d", a, b)
	}

	bag.Release()
	s1.Emit(3)
	s2.Emit(4)
	if a != 1 || b != 2 {
		t.Fatalf("after release a=%d b=%d", a, b)
	}
}

func TestUnit_fired_is_zero_value(t *testing.T) {
	if Fired != (Unit{}) {
		t.Fatalf("fired = %#v", Fired)
	}
}

func TestChange_fields_accessible(t *testing.T) {
	got := Change[int]{Old: 1, New: 2}
	if got.Old != 1 || got.New != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectionEvent_kind_constants_distinct(t *testing.T) {
	kinds := []CollectionEventKind{
		CollectionInserted,
		CollectionRemoved,
		CollectionUpdated,
		CollectionReplaced,
	}
	seen := make(map[CollectionEventKind]struct{}, len(kinds))
	for _, kind := range kinds {
		if _, ok := seen[kind]; ok {
			t.Fatalf("duplicate kind %v", kind)
		}
		seen[kind] = struct{}{}
	}
}
