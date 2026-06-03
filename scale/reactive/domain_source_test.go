package reactive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/store"
)

type testItem struct {
	id  store.ItemID
	val float64
}

func TestDomainFromCollection_basic(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})
	coll.Insert(testItem{id: 3, val: 5})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	d := domainDerived.Get()
	if d[0] != 5 || d[1] != 50 {
		t.Fatalf("extent = [%f,%f], want [5,50]", d[0], d[1])
	}
}

func TestDomainFromCollection_empty(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	d := domainDerived.Get()
	if d[0] != 0 || d[1] != 0 {
		t.Fatalf("empty extent = [%f,%f], want [0,0]", d[0], d[1])
	}
}

func TestDomainFromCollection_single_item(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 42})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	d := domainDerived.Get()
	if d[0] != 42 || d[1] != 42 {
		t.Fatalf("single extent = [%f,%f], want [42,42]", d[0], d[1])
	}
}

func TestDomainFromCollection_insert_updates_extent(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	domainDerived.Get()

	coll.Insert(testItem{id: 3, val: 100})
	d := domainDerived.Get()
	if d[1] != 100 {
		t.Fatalf("after insert: hi = %f, want 100", d[1])
	}
}

func TestDomainFromCollection_remove_updates_extent(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 5})
	coll.Insert(testItem{id: 2, val: 50})
	coll.Insert(testItem{id: 3, val: 10})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	domainDerived.Get()

	coll.Remove(1)
	d := domainDerived.Get()
	if d[0] != 10 {
		t.Fatalf("after remove: lo = %f, want 10", d[0])
	}
}

func TestDomainFromCollection_update_changes_extent(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	domainDerived.Get()

	coll.Update(testItem{id: 2, val: 100})
	d := domainDerived.Get()
	if d[1] != 100 {
		t.Fatalf("after update: hi = %f, want 100", d[1])
	}
}

func TestDomainFromCollection_replace_changes_extent(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	domainDerived.Get()

	coll.Replace([]testItem{
		{id: 100, val: 200},
		{id: 200, val: 300},
	})
	d := domainDerived.Get()
	if d[0] != 200 || d[1] != 300 {
		t.Fatalf("after replace: extent = [%f,%f], want [200,300]", d[0], d[1])
	}
}

func TestDomainFromCollection_noop_update_stable(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	domainDerived.Get()

	coll.Update(testItem{id: 1, val: 10})
	d := domainDerived.Get()
	if d[0] != 10 || d[1] != 50 {
		t.Fatalf("after no-op: extent = [%f,%f], want [10,50]", d[0], d[1])
	}
}

func TestDomainFromCollection_chained_to_reactive(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 5})
	coll.Insert(testItem{id: 2, val: 15})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	rng := store.NewValueStore([2]float64{0, 500})

	rngDerived := bridgeToDerived(rng)
	rs := NewLinearReactiveFromDerived(domainDerived, rngDerived)

	s := rs.Get()
	if got := s.Map(10); got != 250 {
		t.Fatalf("initial Map(10) = %f, want 250", got)
	}

	// Simulate the projection-phase flow: read the derived domain first
	// (this triggers recompute + OnChange → ValueStore bridge update),
	// then read the reactive scale.
	coll.Insert(testItem{id: 3, val: 25})
	domainDerived.Get() // triggers OnChange → updates bridged ValueStore
	s = rs.Get()        // reads updated domain from ValueStore
	if got := s.Map(10); got != 125 {
		t.Fatalf("after insert Map(10) = %f, want 125", got)
	}
}

func TestDomainFromCollection_chained_log_reactive(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 100})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	rng := store.NewValueStore([2]float64{0, 500})
	rngDerived := bridgeToDerived(rng)

	rs := NewLogReactiveFromDerived(domainDerived, rngDerived, scale.WithBase(10))
	s := rs.Get()
	if s.Kind() != scale.KindLog {
		t.Fatalf("kind = %s, want KindLog", s.Kind())
	}
}

func TestDomainFromCollection_chained_time_reactive(t *testing.T) {
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 0})
	coll.Insert(testItem{id: 2, val: 1000})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	rng := store.NewValueStore([2]float64{0, 500})
	rngDerived := bridgeToDerived(rng)

	rs := NewTimeReactiveFromDerived(domainDerived, rngDerived)
	s := rs.Get()
	if s.Kind() != scale.KindTime {
		t.Fatalf("kind = %s, want KindTime", s.Kind())
	}
}

// bridgeToDerived wraps a ValueStore as a Derived for use with FromDerived.
func bridgeToDerived(vs *store.ValueStore[[2]float64]) *store.Derived[[2]float64] {
	return store.NewDerived(
		func() [2]float64 { return vs.Get() },
		vs,
	)
}
