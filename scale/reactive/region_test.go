package reactive

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/store"
)

func TestRangeFromRegion_basic(t *testing.T) {
	rng := RangeFromRegion(0, 500)
	d := rng.Get()
	if d[0] != 0 || d[1] != 500 {
		t.Fatalf("range = [%f,%f], want [0,500]", d[0], d[1])
	}
}

func TestRangeFromRegion_updates_scale_on_resize(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := RangeFromRegion(0, 500)
	rs := NewLinearReactive(domain, rng)

	s := rs.Get()
	if got := s.Map(50); got != 250 {
		t.Fatalf("initial Map(50) = %f, want 250", got)
	}

	// Simulate resize: update the range
	rng.Set([2]float64{0, 1000})
	rs.Get() // trigger recompute
	s = rs.Get()
	if got := s.Map(50); got != 500 {
		t.Fatalf("after resize Map(50) = %f, want 500", got)
	}
}

func TestRangeFromRegion_version_bumps_on_set(t *testing.T) {
	rng := RangeFromRegion(0, 500)
	rs := NewLinearReactive(store.NewValueStore([2]float64{0, 100}), rng)
	rs.Get() // prime
	v0 := rs.Version()

	rng.Set([2]float64{0, 1000})
	rs.Get()
	v1 := rs.Version()
	if v1 <= v0 {
		t.Fatal("version should bump after range change")
	}
}

func TestRangeFromRegion_chained_via_scale(t *testing.T) {
	// End-to-end: domain from collection, range from region
	coll := store.NewCollectionStore(func(i testItem) store.ItemID { return i.id })
	coll.Insert(testItem{id: 1, val: 10})
	coll.Insert(testItem{id: 2, val: 50})

	domainDerived := DomainFromCollection(coll, func(i testItem) float64 { return i.val })
	rng := RangeFromRegion(0, 500)
	rngDerived := bridgeToDerived(rng)

	rs := NewLinearReactiveFromDerived(domainDerived, rngDerived)

	s := rs.Get()
	if got := s.Map(30); got != 250 {
		t.Fatalf("initial Map(30) = %f, want 250", got)
	}

	// Change range (simulate resize)
	rng.Set([2]float64{0, 1000})
	rngDerived.Get() // trigger OnChange → bridge updates
	rs.Get()
	s = rs.Get()
	if got := s.Map(30); got != 500 {
		t.Fatalf("after resize Map(30) = %f, want 500", got)
	}

	// Change domain (simulate data change)
	coll.Insert(testItem{id: 3, val: 100})
	domainDerived.Get() // trigger OnChange → bridge
	rs.Get()
	s = rs.Get()
	if got := s.Map(30); math.Abs(got-222.2222222222222) > 1e-9 {
		t.Fatalf("after data+resize Map(30) = %f, want ~222.22", got)
	}
}
