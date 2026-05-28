package store

import (
	"sync"
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

// --- Concurrent reads ---

func TestValueStore_concurrent_reads_are_safe(t *testing.T) {
	s := NewValueStore(42)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := s.Get(); got != 42 {
				t.Errorf("got %d", got)
			}
		}()
	}
	wg.Wait()
}

func TestMapStore_concurrent_reads_are_safe(t *testing.T) {
	m := NewMapStore[string, int]()
	m.Set("key", 7)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, ok := m.Get("key"); !ok || got != 7 {
				t.Errorf("got %d %v", got, ok)
			}
		}()
	}
	wg.Wait()
}

func TestCollectionStore_concurrent_reads_are_safe(t *testing.T) {
	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	c.Insert(1)
	var wg sync.WaitGroup
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got, ok := c.Get(ItemID(1)); !ok || got != 1 {
				t.Errorf("got %d %v", got, ok)
			}
		}()
	}
	wg.Wait()
}

// --- ValueStore always emits (no DeepEqual) ---

func TestValueStore_no_deepequal_always_emits_on_set(t *testing.T) {
	s := NewValueStore(1)
	var count atomic.Int32
	s.OnChange.Subscribe(func(c signal.Change[int]) {
		count.Add(1)
	})

	// Same value after reflect.DeepEqual removal always emits.
	s.Set(1)
	if count.Load() != 1 {
		t.Fatalf("expected 1 emit for Set with same value, got %d", count.Load())
	}
}

// --- CollectionStore insert-then-update semantics ---

func TestCollectionStore_insert_then_update_semantics(t *testing.T) {
	c := NewCollectionStore(func(v int) ItemID { return ItemID(v) })
	var inserts, updates int
	c.onInsert.Subscribe(func(CollectionInsertEvent[int]) { inserts++ })
	c.onUpdate.Subscribe(func(CollectionUpdateEvent[int]) { updates++ })

	c.Insert(1)
	if inserts != 1 || updates != 0 {
		t.Fatalf("inserts=%d updates=%d", inserts, updates)
	}

	// Insert again with same ID — should update.
	c.Insert(1)
	if inserts != 1 || updates != 1 {
		t.Fatalf("inserts=%d updates=%d", inserts, updates)
	}
}

// --- Transaction 10000 mutations ---

func TestTransaction_10000_mutations_fire_signals_exactly_once(t *testing.T) {
	const n = 10000
	stores := make([]*ValueStore[int], 10)
	counts := make([]atomic.Int32, 10)
	for i := range stores {
		stores[i] = NewValueStore(0)
		j := i
		stores[i].OnChange.Subscribe(func(c signal.Change[int]) {
			counts[j].Add(1)
		})
	}

	tx := &Transaction{}
	for i := 0; i < n; i++ {
		stores[i%10].SetTx(i, tx)
	}
	tx.Commit()

	for i := range counts {
		if got := counts[i].Load(); got != int32(n/10) {
			t.Fatalf("store %d expected %d signals, got %d", i, n/10, got)
		}
	}
}

// --- Derived always emits ---

func TestDerivedStore_no_deepequal_always_emits(t *testing.T) {
	src := NewValueStore(1)
	var count atomic.Int32
	d := NewDerived(func() int {
		return src.Get()
	}, src)
	d.OnChange.Subscribe(func(c signal.Change[int]) {
		count.Add(1)
	})

	src.Set(1) // same value — Derived.Get will recompute and emit
	d.Get()
	if count.Load() != 1 {
		t.Fatalf("expected 1 emit, got %d", count.Load())
	}
}

// --- Zero-value usability ---

func TestValueStore_zero_value_is_usable(t *testing.T) {
	var s ValueStore[int]
	if got := s.Get(); got != 0 {
		t.Fatalf("expected 0, got %d", got)
	}
}

func TestMapStore_get_on_empty_map(t *testing.T) {
	m := NewMapStore[string, int]()
	v, ok := m.Get("missing")
	if ok {
		t.Fatal("expected false")
	}
	if v != 0 {
		t.Fatalf("expected 0, got %d", v)
	}
}
