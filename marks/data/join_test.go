package data

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/store"
)

type binderRuntimeStub struct{}

func (binderRuntimeStub) Schedule(j job.AnyJob)                  {}
func (binderRuntimeStub) CancelJob(id job.JobID)                 {}
func (binderRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}

// --- Test helpers ---

type testItem struct {
	id   store.ItemID
	name string
}

func testIdentify(t testItem) store.ItemID { return t.id }

type testChild struct {
	facet.Facet
	item testItem
}

func newTestChild(item testItem) *testChild {
	return &testChild{Facet: facet.NewFacet(), item: item}
}

func (c *testChild) Base() *facet.Facet            { c.Facet.BindImpl(c); return &c.Facet }
func (c *testChild) OnAttach(ctx facet.AttachContext) {}
func (c *testChild) OnDetach()                         {}
func (c *testChild) OnActivate()                       {}
func (c *testChild) OnDeactivate()                     {}

type testParent struct {
	facet.Facet
}

func newTestParent() *testParent {
	return &testParent{Facet: facet.NewFacet()}
}

func (p *testParent) Base() *facet.Facet            { p.Facet.BindImpl(p); return &p.Facet }
func (p *testParent) OnAttach(ctx facet.AttachContext) {}
func (p *testParent) OnDetach()                         {}
func (p *testParent) OnActivate()                       {}
func (p *testParent) OnDeactivate()                     {}

// --- Tests ---

func TestBinder_insert_attaches_child(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})

	child := b.Child(1)
	if child == nil {
		t.Fatal("expected child to be created for item 1")
	}
	if len(b.Children()) != 1 {
		t.Fatalf("expected 1 child, got %d", len(b.Children()))
	}
}

func TestBinder_insert_with_initial_data(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})

	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	if len(b.Children()) != 2 {
		t.Fatalf("expected 2 initial children, got %d", len(b.Children()))
	}
	if b.Child(1) == nil || b.Child(2) == nil {
		t.Fatal("expected both initial children to exist")
	}
}

func TestBinder_remove_disposes_child(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Remove(1)

	if b.Child(1) != nil {
		t.Fatal("expected child to be removed after store remove")
	}
	if len(b.Children()) != 0 {
		t.Fatalf("expected 0 children after remove, got %d", len(b.Children()))
	}
}

func TestBinder_update_invalidates_child(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	child := b.Child(1)

	if flags := child.Base().DirtyFlags(); flags != 0 {
		t.Fatal("expected clean child before update")
	}

	s.Update(testItem{id: 1, name: "updated"})

	if flags := child.Base().DirtyFlags(); flags&facet.DirtyProjection == 0 {
		t.Fatal("expected DirtyProjection after update")
	}
}

func TestBinder_replace_reconciles(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})
	s.Insert(testItem{id: 3, name: "three"})

	s.Replace([]testItem{
		{id: 2, name: "two-replaced"},
		{id: 4, name: "four"},
	})

	if b.Child(1) != nil {
		t.Fatal("expected item 1 to be removed after replace")
	}
	if b.Child(2) == nil {
		t.Fatal("expected item 2 to persist after replace")
	}
	if b.Child(3) != nil {
		t.Fatal("expected item 3 to be removed after replace")
	}
	if b.Child(4) == nil {
		t.Fatal("expected item 4 to be added after replace")
	}
	if len(b.Children()) != 2 {
		t.Fatalf("expected 2 children after replace, got %d", len(b.Children()))
	}
}

func TestBinder_remove_nonexistent_id_noops(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Remove(999) // should not panic
}

func TestBinder_insert_duplicate_id_updates(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 1, name: "one-again"}) // Insert with same ID triggers Update in CollectionStore

	children := b.Children()
	if len(children) != 1 {
		t.Fatalf("expected 1 child after duplicate insert, got %d", len(children))
	}
}

func TestBinder_on_detach_disposes_all(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})

	b.OnDetach()

	if len(b.Children()) != 0 {
		t.Fatal("expected no children after detach")
	}

	// After detach, store changes should not affect binder
	s.Insert(testItem{id: 3, name: "three"})
	if b.Child(3) != nil {
		t.Fatal("expected no child creation after detach")
	}
}

func TestBinder_children_in_store_order(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 3, name: "three"})
	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})

	s.Insert(testItem{id: 0, name: "zero"})

	children := b.Children()
	if len(children) != 4 {
		t.Fatalf("expected 4 children, got %d", len(children))
	}
}

func TestBinder_reorder_preserves_stable_identity(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})
	s.Insert(testItem{id: 3, name: "three"})

	child1Before := b.Child(1)
	child2Before := b.Child(2)
	child3Before := b.Child(3)

	// Replace with reordered items
	s.Replace([]testItem{
		{id: 3, name: "three"},
		{id: 1, name: "one"},
		{id: 2, name: "two"},
	})

	// Same facet objects — identity preserved
	if b.Child(1) != child1Before {
		t.Fatal("expected item 1 facet to be preserved across reorder")
	}
	if b.Child(2) != child2Before {
		t.Fatal("expected item 2 facet to be preserved across reorder")
	}
	if b.Child(3) != child3Before {
		t.Fatal("expected item 3 facet to be preserved across reorder")
	}

	// Children returned in new store order
	children := b.Children()
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}

func TestBinder_replace_with_empty_disposes_all(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})
	s.Replace([]testItem{})

	if len(b.Children()) != 0 {
		t.Fatalf("expected 0 children after empty replace, got %d", len(b.Children()))
	}
	if b.Child(1) != nil {
		t.Fatal("expected child 1 to be removed")
	}
}

func TestBinder_replace_with_same_set_no_recreate(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	s.Insert(testItem{id: 2, name: "two"})

	child1 := b.Child(1)

	s.Replace([]testItem{
		{id: 1, name: "one"},
		{id: 2, name: "two"},
	})

	if b.Child(1) != child1 {
		t.Fatal("expected same facet object after replace with same set")
	}
}

func TestBinder_insert_remove_insert_same_id(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "first"})
	first := b.Child(1)

	s.Remove(1)

	if b.Child(1) != nil {
		t.Fatal("expected child to be disposed after remove")
	}

	s.Insert(testItem{id: 1, name: "second"})
	second := b.Child(1)

	if second == nil {
		t.Fatal("expected new child after re-insert")
	}
	if second == first {
		t.Fatal("expected new facet object, not the old disposed one")
	}
}

func TestBinder_empty_store_no_children(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	if len(b.Children()) != 0 {
		t.Fatal("expected no children for empty store")
	}
}

func TestBinder_multiple_updates_all_invalidate(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})
	child := b.Child(1)

	for i := 0; i < 5; i++ {
		child.Base().ClearDirty(facet.DirtyAll)
		s.Update(testItem{id: 1, name: "updated"})
		if flags := child.Base().DirtyFlags(); flags&facet.DirtyProjection == 0 {
			t.Fatalf("iteration %d: expected DirtyProjection after update", i)
		}
	}
}

func TestBinder_child_is_attached(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	s.Insert(testItem{id: 1, name: "one"})

	child := b.Child(1)
	if child.Base().State() != facet.StateAttached {
		t.Fatalf("expected child state Attached, got %v", child.Base().State())
	}
}

func TestBinder_children_track_insert_index(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	// Insert items out of order; the binder should follow store's insert order
	s.Insert(testItem{id: 10, name: "ten"})
	s.Insert(testItem{id: 20, name: "twenty"})
	s.Insert(testItem{id: 5, name: "five"})

	children := b.Children()
	if len(children) != 3 {
		t.Fatalf("expected 3 children, got %d", len(children))
	}
}

func TestBinder_no_leaks_on_dispose(t *testing.T) {
	s := store.NewCollectionStore(testIdentify)
	p := newTestParent()
	b := NewCollectionBinder(&p.Facet, s, func(item testItem) facet.FacetImpl {
		return newTestChild(item)
	})

	facet.Attach(p, facet.AttachContext{})
	b.OnAttach(facet.AttachContext{Runtime: binderRuntimeStub{}})

	for i := 0; i < 10; i++ {
		s.Insert(testItem{id: store.ItemID(i), name: "item"})
	}

	if len(b.Children()) != 10 {
		t.Fatalf("expected 10 children, got %d", len(b.Children()))
	}

	b.OnDetach()

	if len(b.Children()) != 0 {
		t.Fatal("expected zero children after dispose")
	}
}
