package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
)

// selectionTestFacet is a minimal FacetImpl used to bind a SelectionBinding
// (the binding rides the facet's subscription bag).
type selectionTestFacet struct {
	facet.Facet
	syncs int
	last  int
}

func (f *selectionTestFacet) Base() *facet.Facet           { f.BindImpl(f); return &f.Facet }
func (f *selectionTestFacet) OnAttach(facet.AttachContext) {}
func (f *selectionTestFacet) OnDetach()                    {}
func (f *selectionTestFacet) OnActivate()                  {}
func (f *selectionTestFacet) OnDeactivate()                {}

// TestSelectionBinding_preAttachWriteThenBind pins FR-8 (a) — the A-7 killer:
// a store write before attach is adopted at bind time, so the mark's internal
// selection is correct on the first frame without any replay.
func TestSelectionBinding_preAttachWriteThenBind(t *testing.T) {
	sel := store.NewValueStore(0)
	sel.Set(3) // pre-attach write
	f := &selectionTestFacet{}
	b := BindSelection(f, sel, func(v int) { f.syncs++; f.last = v })
	if got := b.Current(); got != 3 {
		t.Fatalf("Current() = %d, want 3 (pre-attach write adopted at attach)", got)
	}
	if f.syncs != 0 {
		t.Fatalf("attach adoption re-fired onSync %d times (already applied)", f.syncs)
	}
}

// TestSelectionBinding_externalWriteReSyncs pins FR-8 (b): an external store
// write mid-session re-syncs the mark's internal selection and runs onSync so
// the mark re-renders within the same store-change delivery.
func TestSelectionBinding_externalWriteReSyncs(t *testing.T) {
	sel := store.NewValueStore(1)
	f := &selectionTestFacet{}
	b := BindSelection(f, sel, func(v int) { f.syncs++; f.last = v })
	sel.Set(5)
	if got := b.Current(); got != 5 {
		t.Fatalf("Current() = %d, want 5 after external write", got)
	}
	if f.syncs != 1 || f.last != 5 {
		t.Fatalf("onSync = (%d times, last %d), want (1, 5)", f.syncs, f.last)
	}
}

// TestSelectionBinding_publishWritesStore pins FR-8 (c): a user selection
// published through the binding lands in the store, and the echo of that own
// write does not re-fire onSync (the mark already holds the value).
func TestSelectionBinding_publishWritesStore(t *testing.T) {
	sel := store.NewValueStore(0)
	f := &selectionTestFacet{}
	b := BindSelection(f, sel, func(v int) { f.syncs++; f.last = v })
	b.Publish(7)
	if got := sel.Get(); got != 7 {
		t.Fatalf("store = %d, want 7 after publish", got)
	}
	if f.syncs != 0 {
		t.Fatalf("echo of our own publish re-fired onSync %d times", f.syncs)
	}
}

// TestSelectionBinding_echoGuardPreventsLoop pins FR-8 (d): even a mark whose
// onSync writes back to the store cannot loop — the binding's equality guard
// suppresses re-sync of values the mark already holds, so an external write
// that the mark echoes settles after one onSync.
func TestSelectionBinding_echoGuardPreventsLoop(t *testing.T) {
	sel := store.NewValueStore(0)
	f := &selectionTestFacet{}
	b := BindSelection(f, sel, func(v int) {
		f.syncs++
		f.last = v
		sel.Set(v) // pathological write-back: would loop without the guard
	})

	b.Publish(1)
	if f.syncs != 0 {
		t.Fatalf("own-publish echo looped onSync %d times", f.syncs)
	}

	sel.Set(2) // external write, echoed back by the handler
	if f.syncs != 1 || f.last != 2 {
		t.Fatalf("external write onSync = (%d, last %d), want (1, 2) — must settle after one pass", f.syncs, f.last)
	}
	if got := sel.Get(); got != 2 {
		t.Fatalf("store = %d, want 2", got)
	}
}

// TestNavRailBinding_preAttachWriteSurvivesAttach pins FR-8 (a) on a real nav
// mark: a rail whose ActiveIndex store was written before attach renders the
// pre-attach selection on its first frame.
func TestNavRailBinding_preAttachWriteSurvivesAttach(t *testing.T) {
	sel := store.NewValueStore(-1)
	sel.Set(1) // pre-attach write
	rail := NewNavRail("Rail", []NavRailItem{
		{Label: "A", IconRef: "a"},
		{Label: "B", IconRef: "b"},
	}, sel)
	facet.Attach(rail, facet.AttachContext{})
	if got := rail.clampedActiveIndex(); got != 1 {
		t.Fatalf("clampedActiveIndex = %d, want 1 (pre-attach write adopted at attach)", got)
	}
}

// TestNavRailBinding_externalWriteReSyncs pins FR-8 (b) on a real nav mark: an
// external ActiveIndex write after attach re-syncs the rail's internal
// selection through the binding.
func TestNavRailBinding_externalWriteReSyncs(t *testing.T) {
	sel := store.NewValueStore(0)
	rail := NewNavRail("Rail", []NavRailItem{
		{Label: "A", IconRef: "a"},
		{Label: "B", IconRef: "b"},
	}, sel)
	facet.Attach(rail, facet.AttachContext{})
	sel.Set(1)
	if got := rail.clampedActiveIndex(); got != 1 {
		t.Fatalf("clampedActiveIndex = %d, want 1 after external write", got)
	}
}
