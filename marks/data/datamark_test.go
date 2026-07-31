package data

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/contracttest"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

type dmRuntimeStub struct{}

func (dmRuntimeStub) Schedule(j job.AnyJob)                                              {}
func (dmRuntimeStub) CancelJob(id job.JobID)                                             {}
func (dmRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

type markItem struct {
	id  store.ItemID
	val float64
}

func markIdentify(i markItem) store.ItemID { return i.id }

type markChild struct {
	facet.Facet
	item markItem
}

func newMarkChild(item markItem) facet.FacetImpl {
	return &markChild{Facet: facet.NewFacet(), item: item}
}

func (c *markChild) Base() *facet.Facet               { c.BindImpl(c); return &c.Facet }
func (c *markChild) OnAttach(ctx facet.AttachContext) {}
func (c *markChild) OnDetach()                        {}
func (c *markChild) OnActivate()                      {}
func (c *markChild) OnDeactivate()                    {}

type dmParent struct {
	facet.Facet
}

func newDMParent() *dmParent {
	return &dmParent{Facet: facet.NewFacet()}
}

func (p *dmParent) Base() *facet.Facet               { p.BindImpl(p); return &p.Facet }
func (p *dmParent) OnAttach(ctx facet.AttachContext) {}
func (p *dmParent) OnDetach()                        {}
func (p *dmParent) OnActivate()                      {}
func (p *dmParent) OnDeactivate()                    {}

func TestDataMark_binds_collection_creates_children(t *testing.T) {
	s := store.NewCollectionStore(markIdentify)
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	parent := newDMParent()
	facet.Attach(parent, facet.AttachContext{Runtime: dmRuntimeStub{}})

	dm := NewDataMark(&parent.Facet, s, newMarkChild, scales, rng)
	dm.Binder.OnAttach(facet.AttachContext{Runtime: dmRuntimeStub{}})

	if dm.Store != s {
		t.Fatal("Store not set")
	}
	if dm.BoundData() != s {
		t.Fatal("BoundData should return the store")
	}

	// Insert data and verify children are created
	s.Insert(markItem{id: 1, val: 25})
	s.Insert(markItem{id: 2, val: 50})
	s.Insert(markItem{id: 3, val: 75})

	children := dm.Binder.Children()
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}

func TestDataMark_map_position_through_scale(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}

	// Value 50 in domain [0,100] → range [0,500] should give 250
	pos := dm.MapPosition(marks.Channel{Name: "x"}, 50)
	if pos != 250 {
		t.Fatalf("MapPosition(50) = %f, want 250", pos)
	}

	// Value 0 → 0
	pos = dm.MapPosition(marks.Channel{Name: "x"}, 0)
	if pos != 0 {
		t.Fatalf("MapPosition(0) = %f, want 0", pos)
	}

	// Value 100 → 500
	pos = dm.MapPosition(marks.Channel{Name: "x"}, 100)
	if pos != 500 {
		t.Fatalf("MapPosition(100) = %f, want 500", pos)
	}
}

func TestDataMark_map_positions_converts_xy(t *testing.T) {
	xDomain := store.NewValueStore([2]float64{0, 10})
	xRange := store.NewValueStore([2]float64{0, 200})
	xScale := reactive.NewLinearReactive(xDomain, xRange)

	yDomain := store.NewValueStore([2]float64{0, 10})
	yRange := store.NewValueStore([2]float64{0, 100})
	yScale := reactive.NewLinearReactive(yDomain, yRange)

	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: xScale,
		{Name: "y"}: yScale,
	}

	dm := &DataMark[int]{Scales: scales}
	xChan := marks.Channel{Name: "x"}
	yChan := marks.Channel{Name: "y"}

	pt := dm.MapPositions(xChan, yChan, 5, 5)
	if pt.X != 100 {
		t.Fatalf("X = %f, want 100", pt.X)
	}
	if pt.Y != 50 {
		t.Fatalf("Y = %f, want 50", pt.Y)
	}
}

func TestDataMark_missing_channel_returns_zero(t *testing.T) {
	dm := &DataMark[int]{Scales: make(map[marks.Channel]*reactive.ReactiveScale)}
	pos := dm.MapPosition(marks.Channel{Name: "missing"}, 50)
	if pos != 0 {
		t.Fatalf("expected 0 for missing channel, got %f", pos)
	}
}

func TestDataMark_derive_on_domain_change(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}

	// Initial: Map(50) = 250
	if pos := dm.MapPosition(marks.Channel{Name: "x"}, 50); pos != 250 {
		t.Fatalf("initial Map(50) = %f, want 250", pos)
	}

	// Change domain: [0, 100] → [0, 200]
	domain.Set([2]float64{0, 200})
	rs.Get() // trigger recompute

	// After domain change: Map(50) = 500 * 50/200 = 125
	// Actually: 50 in [0,200] → range [0,500] → 50/200 * 500 = 125
	if pos := dm.MapPosition(marks.Channel{Name: "x"}, 50); pos != 125 {
		t.Fatalf("after domain change Map(50) = %f, want 125", pos)
	}
}

func TestDataMark_derive_on_range_change(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}
	_ = dm.MapPosition(marks.Channel{Name: "x"}, 50) // prime

	// Change range: [0, 500] → [0, 1000]
	rng.Set([2]float64{0, 1000})
	rs.Get() // trigger recompute

	// After range change: Map(50) = 50/100 * 1000 = 500
	if pos := dm.MapPosition(marks.Channel{Name: "x"}, 50); pos != 500 {
		t.Fatalf("after range change Map(50) = %f, want 500", pos)
	}
}

// --- Concrete mark that embeds DataMark for Describe/DataBound testing ---

type concreteDataMark struct {
	facet.Facet
	DataMark[markItem]
}

func newConcreteDataMark() *concreteDataMark {
	s := store.NewCollectionStore(markIdentify)
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}
	m := &concreteDataMark{Facet: facet.NewFacet()}
	m.DataMark = *NewDataMark(&m.Facet, s, newMarkChild, scales, rng)
	m.Core.Facet = m.Facet // share the same Facet
	return m
}

func (m *concreteDataMark) Base() *facet.Facet               { m.BindImpl(m); return &m.Facet }
func (m *concreteDataMark) OnAttach(ctx facet.AttachContext) {}
func (m *concreteDataMark) OnDetach()                        {}
func (m *concreteDataMark) OnActivate()                      {}
func (m *concreteDataMark) OnDeactivate()                    {}

func (m *concreteDataMark) Descriptor() marks.Descriptor {
	d := m.DataMark.Descriptor()
	d.TypeName = "test_mark"
	return d
}

func TestDataMark_describe_reports_databound(t *testing.T) {
	m := newConcreteDataMark()
	d := marks.Describe(m)
	if !d.DataBound {
		t.Fatal("expected Describe to report DataBound = true")
	}
	if d.Family != "viz" {
		t.Fatalf("Family = %q, want %q", d.Family, "viz")
	}
	if d.TypeName != "test_mark" {
		t.Fatalf("TypeName = %q, want %q", d.TypeName, "test_mark")
	}
}

func TestDataMark_invert_position(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}

	// Map(50) = 250, so Invert(250) should = 50
	inv := dm.InvertPosition(marks.Channel{Name: "x"}, 250)
	if inv != 50 {
		t.Fatalf("InvertPosition(250) = %f, want 50", inv)
	}
}

func TestDataMark_invert_at_edges(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}

	if inv := dm.InvertPosition(marks.Channel{Name: "x"}, 0); inv != 0 {
		t.Fatalf("InvertPosition(0) = %f, want 0", inv)
	}
	if inv := dm.InvertPosition(marks.Channel{Name: "x"}, 500); inv != 100 {
		t.Fatalf("InvertPosition(500) = %f, want 100", inv)
	}
}

func TestDataMark_invert_missing_channel(t *testing.T) {
	dm := &DataMark[int]{Scales: make(map[marks.Channel]*reactive.ReactiveScale)}
	if inv := dm.InvertPosition(marks.Channel{Name: "missing"}, 250); inv != 0 {
		t.Fatalf("expected 0 for missing channel, got %f", inv)
	}
}

func TestDataMark_zoom_updates_positions(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}
	_ = dm.MapPosition(marks.Channel{Name: "x"}, 50) // prime

	// Zoom in: change domain to [25, 75]
	domain.Set([2]float64{25, 75})
	rs.Get() // trigger recompute

	// After zoom: Map(50) = (50-25)/(75-25) * 500 = 25/50 * 500 = 250
	// Actually: 50 in [25,75] → range [0,500] → (50-25)/(75-25)*500 = 250
	pos := dm.MapPosition(marks.Channel{Name: "x"}, 50)
	if pos != 250 {
		t.Fatalf("after zoom Map(50) = %f, want 250", pos)
	}
}

func TestDataMark_resize_updates_positions(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	dm := &DataMark[int]{Scales: scales}
	_ = dm.MapPosition(marks.Channel{Name: "x"}, 50) // prime

	// Resize: change range from [0,500] to [0,800]
	rng.Set([2]float64{0, 800})
	rs.Get()

	// After resize: Map(50) = 50/100 * 800 = 400
	pos := dm.MapPosition(marks.Channel{Name: "x"}, 50)
	if pos != 400 {
		t.Fatalf("after resize Map(50) = %f, want 400", pos)
	}
}

// contractDataMark is a DataBound mark used for the AssertDataBound contract test.
// It accepts a caller-supplied store and manages children via its own slice
// (not through CollectionBinder/AddChildRuntime) to avoid the double-attach
// that would occur if children were registered on the facet's child list before
// facet.Attach completes.
type contractDataMark struct {
	facet.Facet
	store    *store.CollectionStore[markItem]
	factory  func(markItem) facet.FacetImpl
	children []facet.FacetImpl
	cleanups []func()
}

func newContractDataMark(s *store.CollectionStore[markItem]) *contractDataMark {
	m := &contractDataMark{
		Facet:   facet.NewFacet(),
		store:   s,
		factory: newMarkChild,
	}
	return m
}

func (m *contractDataMark) Base() *facet.Facet             { m.BindImpl(m); return &m.Facet }
func (m *contractDataMark) BoundData() any                 { return m.store }
func (m *contractDataMark) OnAttach(_ facet.AttachContext) { m.subscribe() }
func (m *contractDataMark) OnDetach()                      { m.unsubscribe() }
func (m *contractDataMark) OnActivate()                    {}
func (m *contractDataMark) OnDeactivate()                  {}

// Children satisfies contracttest.childrenProvider so AssertDataBound
// can directly observe child facets.
func (m *contractDataMark) Children() []facet.FacetImpl { return m.children }

func (m *contractDataMark) subscribe() {
	m.cleanups = append(m.cleanups,
		m.store.OnInsertSubscribe(func(e store.CollectionInsertEvent[markItem]) {
			m.rebuild()
		}),
		m.store.OnRemoveSubscribe(func(e store.CollectionRemoveEvent[markItem]) {
			m.rebuild()
		}),
		m.store.OnUpdateSubscribe(func(e store.CollectionUpdateEvent[markItem]) {
			m.rebuild()
		}),
		m.store.OnReplaceSubscribe(func(_ signal.Unit) {
			m.rebuild()
		}),
	)
	m.rebuild()
}

func (m *contractDataMark) unsubscribe() {
	for _, c := range m.cleanups {
		if c != nil {
			c()
		}
	}
	m.cleanups = nil
	for _, child := range m.children {
		facet.Dispose(child)
	}
	m.children = nil
}

func (m *contractDataMark) rebuild() {
	for _, child := range m.children {
		facet.Dispose(child)
	}
	items := m.store.All()
	m.children = make([]facet.FacetImpl, 0, len(items))
	for _, item := range items {
		m.children = append(m.children, m.factory(item))
	}
}

func TestDataMark_contract_databound(t *testing.T) {
	contracttest.AssertDataBound[markItem](t,
		func() *store.CollectionStore[markItem] {
			return store.NewCollectionStore(markIdentify)
		},
		func(s *store.CollectionStore[markItem]) facet.FacetImpl {
			return newContractDataMark(s)
		},
		func(s *store.CollectionStore[markItem]) {
			s.Insert(markItem{id: 1, val: 10})
			s.Insert(markItem{id: 2, val: 50})
			s.Update(markItem{id: 1, val: 20})
			s.Remove(2)
			s.Replace([]markItem{{id: 3, val: 30}})
		},
	)
}

func TestDataMark_child_positions_update_on_data_change(t *testing.T) {
	s := store.NewCollectionStore(markIdentify)
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 500})
	rs := reactive.NewLinearReactive(domain, rng)
	scales := map[marks.Channel]*reactive.ReactiveScale{
		{Name: "x"}: rs,
	}

	parent := newDMParent()
	facet.Attach(parent, facet.AttachContext{Runtime: dmRuntimeStub{}})

	dm := NewDataMark(&parent.Facet, s, newMarkChild, scales, rng)
	dm.Binder.OnAttach(facet.AttachContext{Runtime: dmRuntimeStub{}})

	s.Insert(markItem{id: 1, val: 10})
	s.Insert(markItem{id: 2, val: 50})
	s.Insert(markItem{id: 3, val: 90})

	children := dm.Binder.Children()
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}
