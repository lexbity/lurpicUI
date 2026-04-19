package facet

import "testing"

type focusTestFacet struct {
	Facet

	focusRole FocusRole

	gained int
	lost   int
}

func newFocusTestFacet(tabIndex int, focusable bool) *focusTestFacet {
	f := &focusTestFacet{Facet: NewFacet()}
	f.focusRole.TabIndex = tabIndex
	f.focusRole.Focusable = func() bool { return focusable }
	f.focusRole.OnFocusGained = func() { f.gained++ }
	f.focusRole.OnFocusLost = func() { f.lost++ }
	f.AddRole(&f.focusRole)
	return f
}

func (f *focusTestFacet) Base() *Facet               { return &f.Facet }
func (f *focusTestFacet) OnAttach(ctx AttachContext) {}
func (f *focusTestFacet) OnDetach()                  {}
func (f *focusTestFacet) OnActivate()                {}
func (f *focusTestFacet) OnDeactivate()              {}

func TestFocusManager_set_focus_calls_gained(t *testing.T) {
	m := NewFocusManager()
	f := newFocusTestFacet(0, true)
	m.SetFocus(f)
	if f.gained != 1 {
		t.Fatalf("gained = %d", f.gained)
	}
	if m.Focused() != f.ID() {
		t.Fatalf("focused = %d", m.Focused())
	}
}

func TestFocusManager_set_focus_calls_lost_on_previous(t *testing.T) {
	m := NewFocusManager()
	a := newFocusTestFacet(0, true)
	b := newFocusTestFacet(1, true)
	m.SetFocus(a)
	m.SetFocus(b)
	if a.lost != 1 {
		t.Fatalf("a lost = %d", a.lost)
	}
	if b.gained != 1 {
		t.Fatalf("b gained = %d", b.gained)
	}
}

func TestFocusManager_clear_focus(t *testing.T) {
	m := NewFocusManager()
	f := newFocusTestFacet(0, true)
	m.SetFocus(f)
	m.ClearFocus()
	if f.lost != 1 {
		t.Fatalf("lost = %d", f.lost)
	}
	if m.Focused() != 0 {
		t.Fatalf("focused = %d", m.Focused())
	}
}

func TestFocusManager_tab_order_built_from_tab_index(t *testing.T) {
	root := &Facet{state: StateCreated, id: nextID()}
	a := newFocusTestFacet(20, true)
	b := newFocusTestFacet(-1, true)
	c := newFocusTestFacet(10, true)
	d := newFocusTestFacet(10, true)
	root.children = []*Facet{&a.Facet, &b.Facet, &c.Facet, &d.Facet}
	m := NewFocusManager()
	m.RebuildTabOrder(root)
	if len(m.tabOrder) != 3 {
		t.Fatalf("tabOrder = %#v", m.tabOrder)
	}
	if m.tabOrder[0] != c.ID() || m.tabOrder[1] != d.ID() || m.tabOrder[2] != a.ID() {
		t.Fatalf("unexpected tab order: %#v", m.tabOrder)
	}
}

func TestFocusManager_tab_next_wraps(t *testing.T) {
	m := NewFocusManager()
	root := &Facet{state: StateCreated, id: nextID()}
	a := newFocusTestFacet(0, true)
	b := newFocusTestFacet(1, true)
	root.children = []*Facet{&a.Facet, &b.Facet}
	m.RebuildTabOrder(root)
	m.SetFocus(b)
	m.TabNext()
	if m.Focused() != a.ID() {
		t.Fatalf("expected wrap to a, got %d", m.Focused())
	}
}

func TestFocusManager_non_focusable_skipped(t *testing.T) {
	m := NewFocusManager()
	root := &Facet{state: StateCreated, id: nextID()}
	a := newFocusTestFacet(0, false)
	b := newFocusTestFacet(1, true)
	root.children = []*Facet{&a.Facet, &b.Facet}
	m.RebuildTabOrder(root)
	if len(m.tabOrder) != 1 || m.tabOrder[0] != b.ID() {
		t.Fatalf("unexpected tab order: %#v", m.tabOrder)
	}
	m.TabNext()
	if m.Focused() != b.ID() {
		t.Fatalf("expected focus on b, got %d", m.Focused())
	}
}
