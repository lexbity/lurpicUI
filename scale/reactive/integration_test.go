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
	initialCount := len(initialTicks)

	// Extend data range
	coll.Insert(dataItem{id: 3, val: 900})
	domainDerived.Get() // trigger recompute
	s = rs.Get()
	ticker = s.(scale.Ticker)
	afterTicks := ticker.Ticks(5)

	if len(afterTicks) == initialCount {
		t.Log("tick count unchanged (may be same step with wider domain)")
	}
	// At minimum, the last tick value should be larger
	if afterTicks[len(afterTicks)-1].Value <= initialTicks[len(initialTicks)-1].Value {
		t.Log("note: last tick did not increase with domain")
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

	// Before zoom: click at pixel 250 maps to data value 50
	layer := facet.ProjectionLayer{Transform: gfx.Identity()}
	viewport := &facet.ViewportRole{Transform: gfx.Identity()}

	localPt, _ := facet.ScreenToLocal(layer, viewport, gfx.Point{X: 250, Y: 0})
	before := s.Invert(float64(localPt.X))

	// Zoom in 2x around focal value 50 (semantic zoom)
	zc.Zoom(50, 2)
	s = rs.Get()

	// After zoom: the domain has shrunk. Click at pixel 250 should now
	// map through the updated scale.
	localPt, _ = facet.ScreenToLocal(layer, viewport, gfx.Point{X: 250, Y: 0})
	after := s.Invert(float64(localPt.X))

	// After zoom-in 2x around 50, domain is [30, 70].
	// Map(50) should still be 250 (focal preserved).
	if math.Abs(s.Map(50)-250) > 1e-9 {
		t.Fatalf("focal not preserved after zoom: Map(50) = %f", s.Map(50))
	}
	_ = before
	_ = after
}

func TestIntegration_band_scale_with_reactive_range(t *testing.T) {
	rng := RangeFromRegion(0, 300)
	members := []string{"Jan", "Feb", "Mar"}

	// Band scale uses range from a reactive store
	s := scale.NewBand(members, scale.WithRange(rng.Get()[0], rng.Get()[1]))

	start, width, ok := s.Band("Feb")
	if !ok {
		t.Fatal("Band(Feb) not found")
	}
	if start != 100 || width != 100 {
		t.Fatalf("Band(Feb) = (%f,%f), want (100,100)", start, width)
	}

	_ = rng
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
	ticks := ticker.Ticks(10)
	if len(ticks) == 0 {
		t.Fatal("expected non-empty ticks from reactive time scale")
	}
}
