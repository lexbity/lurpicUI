package store

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/signal"
)

type testItem struct {
	ID   ItemID
	Name string
}

func identifyItem(v testItem) ItemID { return v.ID }

func TestCollectionStore_insert_fires_signal(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	var got CollectionInsertEvent[testItem]
	s.onInsert.Subscribe(func(e CollectionInsertEvent[testItem]) { got = e })

	s.Insert(testItem{ID: 1, Name: "a"})

	if got.Index != 0 || got.Item.ID != 1 || got.Item.Name != "a" {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectionStore_insert_existing_id_fires_update(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	inserted := 0
	updated := 0
	s.onInsert.Subscribe(func(CollectionInsertEvent[testItem]) { inserted++ })
	s.onUpdate.Subscribe(func(CollectionUpdateEvent[testItem]) { updated++ })

	s.Insert(testItem{ID: 1, Name: "a"})
	s.Insert(testItem{ID: 1, Name: "b"})

	if inserted != 1 || updated != 1 {
		t.Fatalf("inserted=%d updated=%d", inserted, updated)
	}
	if got, _ := s.Get(1); got.Name != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectionStore_remove_fires_signal(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	var got CollectionRemoveEvent[testItem]
	s.onRemove.Subscribe(func(e CollectionRemoveEvent[testItem]) { got = e })

	s.Insert(testItem{ID: 1, Name: "a"})
	s.Remove(1)

	if got.ID != 1 || got.Index != 0 || got.Item.Name != "a" {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectionStore_remove_unknown_id_noop(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	called := 0
	s.onRemove.Subscribe(func(CollectionRemoveEvent[testItem]) { called++ })

	s.Remove(99)

	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestCollectionStore_update_fires_signal(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	var got CollectionUpdateEvent[testItem]
	s.onUpdate.Subscribe(func(e CollectionUpdateEvent[testItem]) { got = e })

	s.Insert(testItem{ID: 1, Name: "a"})
	s.Update(testItem{ID: 1, Name: "b"})

	if got.ID != 1 || got.Index != 0 || got.Old.Name != "a" || got.New.Name != "b" {
		t.Fatalf("got %#v", got)
	}
}

func TestCollectionStore_update_unknown_id_noop(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	called := 0
	s.onUpdate.Subscribe(func(CollectionUpdateEvent[testItem]) { called++ })

	s.Update(testItem{ID: 1, Name: "a"})

	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestCollectionStore_replace_fires_onreplace(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	called := 0
	s.onReplace.Subscribe(func(signal.Unit) { called++ })

	s.Replace([]testItem{{ID: 1, Name: "a"}})

	if called != 1 {
		t.Fatalf("called = %d", called)
	}
}

func TestCollectionStore_replace_clears_previous(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	s.Insert(testItem{ID: 1, Name: "a"})
	s.Replace([]testItem{{ID: 2, Name: "b"}})

	if _, ok := s.Get(1); ok {
		t.Fatal("expected old item removed")
	}
	if got, ok := s.Get(2); !ok || got.Name != "b" {
		t.Fatalf("got %#v %v", got, ok)
	}
}

func TestCollectionStore_all_returns_copy(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	s.Insert(testItem{ID: 1, Name: "a"})
	got := s.All()
	got[0].Name = "changed"
	if item, _ := s.Get(1); item.Name != "a" {
		t.Fatalf("store mutated via copy: %#v", item)
	}
}

func TestCollectionStore_get_o1_lookup(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	s.Insert(testItem{ID: 1, Name: "a"})
	s.Insert(testItem{ID: 2, Name: "b"})
	s.Remove(1)
	if got, ok := s.Get(2); !ok || got.Name != "b" {
		t.Fatalf("got %#v %v", got, ok)
	}
}

func TestCollectionStore_version_increments_on_insert(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	ver := s.Version()
	s.Insert(testItem{ID: 1, Name: "a"})
	if got := s.Version(); got <= ver {
		t.Fatalf("version = %d -> %d", ver, got)
	}
}

func TestCollectionStore_version_not_incremented_on_noop(t *testing.T) {
	s := NewCollectionStore(identifyItem)
	ver := s.Version()
	s.Remove(1)
	if got := s.Version(); got != ver {
		t.Fatalf("version changed: %d -> %d", ver, got)
	}
}

func TestMapStore_set_new_key(t *testing.T) {
	s := NewMapStore[string, int]()
	var got MapSetEvent[string, int]
	s.onSet.Subscribe(func(e MapSetEvent[string, int]) { got = e })

	s.Set("a", 1)

	if !got.WasNew || got.Key != "a" || got.Value != 1 || got.Previous != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestMapStore_set_existing_key(t *testing.T) {
	s := NewMapStore[string, int]()
	var got MapSetEvent[string, int]
	s.onSet.Subscribe(func(e MapSetEvent[string, int]) { got = e })

	s.Set("a", 1)
	s.Set("a", 2)

	if got.WasNew || got.Key != "a" || got.Value != 2 || got.Previous != 1 {
		t.Fatalf("got %#v", got)
	}
}

func TestMapStore_delete_fires_signal(t *testing.T) {
	s := NewMapStore[string, int]()
	var got MapDeleteEvent[string, int]
	s.onDelete.Subscribe(func(e MapDeleteEvent[string, int]) { got = e })

	s.Set("a", 1)
	s.Delete("a")

	if got.Key != "a" || got.Value != 1 {
		t.Fatalf("got %#v", got)
	}
}

func TestMapStore_delete_unknown_key_noop(t *testing.T) {
	s := NewMapStore[string, int]()
	called := 0
	s.onDelete.Subscribe(func(MapDeleteEvent[string, int]) { called++ })

	s.Delete("missing")

	if called != 0 {
		t.Fatalf("called = %d", called)
	}
}

func TestMapStore_clear_fires_onclear(t *testing.T) {
	s := NewMapStore[string, int]()
	called := 0
	s.onClear.Subscribe(func(signal.Unit) { called++ })

	s.Set("a", 1)
	s.Clear()

	if called != 1 {
		t.Fatalf("called = %d", called)
	}
	if s.Len() != 0 {
		t.Fatalf("len = %d", s.Len())
	}
}

func TestMapStore_snapshot_returns_copy(t *testing.T) {
	s := NewMapStore[string, int]()
	s.Set("a", 1)
	snap := s.Snapshot()
	snap["a"] = 2
	if got, _ := s.Get("a"); got != 1 {
		t.Fatalf("store mutated via snapshot: %d", got)
	}
}

func TestCollectionMap_invalidatable_compile(t *testing.T) {
	var _ Invalidatable = (*CollectionStore[testItem])(nil)
	var _ Invalidatable = (*MapStore[string, int])(nil)
}
