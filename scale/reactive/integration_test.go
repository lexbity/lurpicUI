package reactive

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/scale"
	"codeburg.org/lexbit/lurpicui/store"
)

type dataItem struct {
	id  store.ItemID
	val float64
}

func TestIntegration_full_live_data_chain(t *testing.T) {
	// 1. Data in a CollectionStore
	coll := store.NewCollectionStore(func(i dataItem) store.ItemID { return i.id })
	coll.Insert(dataItem{id: 1, val: 10})
	coll.Insert(dataItem{id: 2, val: 50})
	coll.Insert(dataItem{id: 3, val: 90})

	// 2. Reactive domain from collection extent
	domainDerived := DomainFromCollection(coll, func(i dataItem) float64 { return i.val })

	// 3. Range from plot region (e.g., a 500px-wide plot area)
	rng := RangeFromRegion(0, 500)
	rngDerived := bridgeToDerived(rng)

	// 4. Reactive scale chain
	rs := NewLinearReactiveFromDerived(domainDerived, rngDerived)
	domainDerived.Get() // prime the chain
	rngDerived.Get()
	s := rs.Get()

	// 5. Scale produces meaningful ticks
	ticker, ok := s.(scale.Ticker)
	if !ok {
		t.Fatal("expected Ticker interface")
	}
	ticks := ticker.Ticks(5)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks from reactive scale")
	}
	for i, tk := range ticks {
		if tk.Label == "" {
			t.Errorf("tick[%d] has empty label", i)
		}
	}

	// 6. Simulate: user clicks at pixel 250.
	// Use identity transforms for the projection layer and viewport.
	layer := facet.ProjectionLayer{
		Transform: gfx.Identity(),
	}
	viewport := &facet.ViewportRole{
		Transform: gfx.Identity(),
	}

	screenPt := gfx.Point{X: 250, Y: 100}
	localPt, ok := facet.ScreenToLocal(layer, viewport, screenPt)
	if !ok {
		t.Fatal("ScreenToLocal failed")
	}

	// 7. Invert the local x coordinate to recover the data value.
	dataValue := s.Invert(float64(localPt.X))

	// Domain from data: [10, 90], range: [0, 500].
	// normalize(250, 0, 500) = 0.5
	// lerp(10, 90, 0.5) = 50
	if math.Abs(dataValue-50) > 1e-9 {
		t.Fatalf("inverted data = %f, want 50", dataValue)
	}

	// 8. Verify that the original datum is recoverable from a click on its
	//    visual position.
	// Map each data value → pixel position → Invert → original value.
	for _, expected := range []float64{10, 50, 90} {
		px := s.Map(expected)
		localPt, ok := facet.ScreenToLocal(layer, viewport, gfx.Point{X: float32(px), Y: 0})
		if !ok {
			t.Fatalf("ScreenToLocal failed for datum %g", expected)
		}
		got := s.Invert(float64(localPt.X))
		if math.Abs(got-expected) > 1e-9 {
			t.Errorf("Invert(ScreenToLocal(Map(%g))) = %g, want %g", expected, got, expected)
		}
	}
}

func TestIntegration_reactive_scale_ticks_update_on_data_change(t *testing.T) {
	coll := store.NewCollectionStore(func(i dataItem) store.ItemID { return i.id })
	coll.Insert(dataItem{id: 1, val: 10})
	coll.Insert(dataItem{id: 2, val: 90})

	domainDerived := DomainFromCollection(coll, func(i dataItem) float64 { return i.val })
	rng := RangeFromRegion(0, 500)
	rngDerived := bridgeToDerived(rng)

	rs := NewLinearReactiveFromDerived(domainDerived, rngDerived)
	domainDerived.Get()
	rngDerived.Get()

	s := rs.Get()
	ticker := s.(scale.Ticker)
	initialTicks := ticker.Ticks(5)

	// Extend data range
	coll.Insert(dataItem{id: 3, val: 900})
	domainDerived.Get() // trigger recompute
	s = rs.Get()
	ticker = s.(scale.Ticker)
	afterTicks := ticker.Ticks(5)

	if len(afterTicks) == 0 {
		t.Fatalf("expected non-empty ticks after data change")
	}
	if afterTicks[len(afterTicks)-1].Value <= initialTicks[len(initialTicks)-1].Value {
		t.Fatalf("domain did not extend after data change: last tick %g <= %g",
			afterTicks[len(afterTicks)-1].Value, initialTicks[len(initialTicks)-1].Value)
	}
	_, hi := rs.Get().Domain()
	if hi < 900 {
		t.Fatalf("domain hi = %g, want >= 900 after inserting val=900", hi)
	}
}

func TestIntegration_reactive_scale_resize_updates_scale(t *testing.T) {
	coll := store.NewCollectionStore(func(i dataItem) store.ItemID { return i.id })
	coll.Insert(dataItem{id: 1, val: 0})
	coll.Insert(dataItem{id: 2, val: 100})

	domainDerived := DomainFromCollection(coll, func(i dataItem) float64 { return i.val })
	rng := RangeFromRegion(0, 500)
	rngDerived := bridgeToDerived(rng)

	rs := NewLinearReactiveFromDerived(domainDerived, rngDerived)
	domainDerived.Get()
	rngDerived.Get()

	s := rs.Get()
	if got := s.Map(50); got != 250 {
		t.Fatalf("initial Map(50) = %f, want 250", got)
	}

	// Resize: wider plot area
	rng.Set([2]float64{0, 1000})
	rngDerived.Get() // trigger bridge
	s = rs.Get()
	if got := s.Map(50); got != 500 {
		t.Fatalf("after resize Map(50) = %f, want 500", got)
	}
}

func TestIntegration_zoom_then_hit_test(t *testing.T) {
	coll := store.NewCollectionStore(func(i dataItem) store.ItemID { return i.id })
	coll.Insert(dataItem{id: 1, val: 10})
	coll.Insert(dataItem{id: 2, val: 90})

	domainStore := store.NewValueStore([2]float64{10, 90})
	rng := store.NewValueStore([2]float64{0, 500})

	rs := NewLinearReactive(domainStore, rng)
	zc := NewZoomController(domainStore)

	s := rs.Get()

	// Before zoom: pixel 250 → data value 50, pixel 0 → data value 10
	lo, hi := s.Domain()
	if math.Abs(lo-10) > 1e-9 || math.Abs(hi-90) > 1e-9 {
		t.Fatalf("pre-zoom domain = [%g,%g], want [10,90]", lo, hi)
	}
	if got := s.Invert(0); math.Abs(got-10) > 1e-9 {
		t.Fatalf("pixel 0 inverts to %g before zoom, want 10", got)
	}

	// Zoom in 2x around focal value 50.
	zc.Zoom(50, 2)
	s = rs.Get()

	// After zoom-in 2x around 50 on domain [10,90], domain is [30, 70].
	lo, hi = s.Domain()
	if math.Abs(lo-30) > 1e-9 || math.Abs(hi-70) > 1e-9 {
		t.Fatalf("post-zoom domain = [%g,%g], want [30,70]", lo, hi)
	}

	// Map(50) should still be 250 (focal preserved).
	if math.Abs(s.Map(50)-250) > 1e-9 {
		t.Fatalf("focal not preserved after zoom: Map(50) = %f", s.Map(50))
	}

	// Non-focal pixel: pixel 0 now maps to domain value 30, not 10.
	if got := s.Invert(0); math.Abs(got-30) > 1e-9 {
		t.Fatalf("pixel 0 inverts to %g after zoom, want 30 (was 10 before)", got)
	}
}

func TestIntegration_band_from_range_snapshot(t *testing.T) {
	rng := RangeFromRegion(0, 300)
	members := []string{"Jan", "Feb", "Mar"}

	s := scale.NewBand(members, scale.WithRange(rng.Get()[0], rng.Get()[1]))

	start, width, ok := s.Band("Feb")
	if !ok {
		t.Fatal("Band(Feb) not found")
	}
	if start != 100 || width != 100 {
		t.Fatalf("Band(Feb) = (%f,%f), want (100,100)", start, width)
	}
}

func TestIntegration_time_scale_with_reactive_domain(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 31536000000}) // ~1 year in ms
	rng := store.NewValueStore([2]float64{0, 500})

	rs := NewTimeReactive(domain, rng)
	s := rs.Get()

	ticker, ok := s.(scale.Ticker)
	if !ok {
		t.Fatal("expected Ticker interface")
	}
	initialTicks := ticker.Ticks(10)
	if len(initialTicks) == 0 {
		t.Fatal("expected non-empty ticks from reactive time scale")
	}

	domain.Set([2]float64{0, 86400000}) // narrow to 1 day in ms
	s = rs.Get()
	ticker = s.(scale.Ticker)
	afterTicks := ticker.Ticks(10)

	if len(afterTicks) == 0 {
		t.Fatal("expected non-empty ticks after domain change")
	}
	if len(afterTicks) >= len(initialTicks) {
		t.Fatal("expected fewer ticks for 1-day domain vs 1-year domain")
	}
	lo, hi := s.Domain()
	if math.Abs(lo-0) > 1e-9 || math.Abs(hi-86400000) > 1e-9 {
		t.Fatalf("domain after change = [%g,%g], want [0,86400000]", lo, hi)
	}
}
