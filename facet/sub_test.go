package facet

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestSubscribe_nilFacetImpl_returnsNoopBuilder(t *testing.T) {
	s := Subscribe(nil)
	if s == nil {
		t.Fatal("expected non-nil builder")
	}
	if got := s.Collect(func() {}); got != s {
		t.Fatal("expected Collect to return the builder")
	}
}

func TestSubscribe_bindsToFacetSubscriptions(t *testing.T) {
	f := newTestFacet()
	s := Subscribe(f)
	if s == nil {
		t.Fatal("expected non-nil builder")
	}
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected empty subscription bag, got %d", got)
	}

	called := 0
	s.Collect(func() { called++ })
	if got := f.Subs().Len(); got != 1 {
		t.Fatalf("expected one collected unsubscribe, got %d", got)
	}

	Dispose(f)
	if called != 1 {
		t.Fatalf("expected collected unsubscribe to run once, got %d", called)
	}
}

func TestCollect_nilBuilder_isNoop(t *testing.T) {
	var s *Sub
	if got := s.Collect(func() {}); got != nil {
		t.Fatal("expected nil builder to remain nil")
	}
}

func TestCollect_nilFunc_isNoop(t *testing.T) {
	f := newTestFacet()
	s := Subscribe(f)
	s.Collect(nil)
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected no collected entries, got %d", got)
	}
}

func TestCollect_zeroValueBuilder_isNoop(t *testing.T) {
	s := &Sub{}
	if got := s.Collect(func() {}); got != s {
		t.Fatal("expected Collect to return the builder")
	}
}

func TestTo_subscribes_and_releases_signal(t *testing.T) {
	f := newTestFacet()
	s := Subscribe(f)
	sig := signal.NewSignal[int]("test")

	called := 0
	To(s, &sig, func(v int) {
		called += v
	})

	if got := f.Subs().Len(); got != 1 {
		t.Fatalf("expected one tracked subscription, got %d", got)
	}

	sig.Emit(3)
	if called != 3 {
		t.Fatalf("expected handler to receive signal, got %d", called)
	}

	Dispose(f)
	sig.Emit(5)
	if called != 3 {
		t.Fatalf("expected handler to be released, got %d", called)
	}
}

func TestTo_nilInputs_areNoop(t *testing.T) {
	sig := signal.NewSignal[int]("test")
	if got := To(nil, &sig, func(int) {}); got != nil {
		t.Fatal("expected nil builder to remain nil")
	}

	f := newTestFacet()
	s := Subscribe(f)
	if got := To(s, nil, func(int) {}); got != s {
		t.Fatal("expected nil signal to return builder unchanged")
	}
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected no subscriptions, got %d", got)
	}
}

func TestStore_registers_and_updates_version(t *testing.T) {
	f := newTestFacet()
	s := Subscribe(f)
	sig := signal.NewSignal[int]("test")

	var version store.Version = 7
	called := 0
	Store(s, &sig, func() store.Version { return version }, func(v int) {
		called += v
	})

	got := f.SubscribedVersions()
	if len(got) != 1 || got[0] != 7 {
		t.Fatalf("expected initial version snapshot 7, got %#v", got)
	}
	if got := f.Subs().Len(); got != 1 {
		t.Fatalf("expected one tracked subscription, got %d", got)
	}

	version = 9
	sig.Emit(3)
	if called != 3 {
		t.Fatalf("expected handler to receive signal, got %d", called)
	}

	got = f.SubscribedVersions()
	if len(got) != 1 || got[0] != 9 {
		t.Fatalf("expected refreshed version 9, got %#v", got)
	}
}

func TestStore_nilInputs_areNoop(t *testing.T) {
	sig := signal.NewSignal[int]("test")
	if got := Store(nil, &sig, func() store.Version { return 1 }, func(int) {}); got != nil {
		t.Fatal("expected nil builder to remain nil")
	}

	f := newTestFacet()
	s := Subscribe(f)
	if got := Store(s, nil, func() store.Version { return 1 }, func(int) {}); got != s {
		t.Fatal("expected nil signal to return builder unchanged")
	}
	if got := f.Subs().Len(); got != 0 {
		t.Fatalf("expected no subscriptions, got %d", got)
	}
}
