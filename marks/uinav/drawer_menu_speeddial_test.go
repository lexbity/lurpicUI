package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

type testAnchorMark struct {
	ID   string
	pt   gfx.Point
	base facet.Facet
}

func (m *testAnchorMark) Base() *facet.Facet {
	m.base.BindImpl(m)
	return &m.base
}

func (m *testAnchorMark) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("test:anchor"), AnchorExporting: true}
}

func (m *testAnchorMark) AuthoredID() string               { return m.ID }
func (m *testAnchorMark) OnAttach(ctx facet.AttachContext) {}
func (m *testAnchorMark) OnDetach()                        {}
func (m *testAnchorMark) OnActivate()                      {}
func (m *testAnchorMark) OnDeactivate()                    {}

func (m *testAnchorMark) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return layout.AnchorSet{"center": m.pt}
}

func TestDrawer_modal_traps_tab_and_escape_closes(t *testing.T) {
	d := &Drawer{
		Mode: DrawerModal,
		Open: store.NewBinding(true),
	}
	d.ensureInit()
	if !d.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyTab}) {
		t.Fatal("expected tab to be handled")
	}
	if !d.Open.Get() {
		t.Fatal("modal drawer should stay open on tab")
	}
	if !d.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to be handled")
	}
	if d.Open.Get() {
		t.Fatal("modal drawer should close on escape")
	}
	if specs := d.OnLayerSpecs(); len(specs) != 2 {
		t.Fatalf("layer specs = %d, want 2 for modal drawer", len(specs))
	}
}

func TestDrawer_dismissible_outside_press_closes(t *testing.T) {
	d := &Drawer{
		Mode: DrawerDismissible,
		Edge: DrawerLeft,
		Open: store.NewBinding(true),
	}
	d.ensureInit()
	if !d.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 220, Y: 20}}) {
		t.Fatal("expected outside press to be handled")
	}
	if d.Open.Get() {
		t.Fatal("dismissible drawer should close on outside press")
	}
}

func TestDrawer_edge_surface_bounds_follow_edge(t *testing.T) {
	d := &Drawer{Edge: DrawerRight, Open: store.NewBinding(true)}
	if got := d.surfaceBounds(); got.Min.X != 80 || got.Width() != 160 {
		t.Fatalf("surface bounds = %#v, want right-edge offset", got)
	}
	if got := d.entryTransform().TransformPoint(gfx.Point{}); got.X != 240 {
		t.Fatalf("entry transform = %#v, want x offset 240", got)
	}
}

func TestMenu_anchorPoint_uses_target_anchor(t *testing.T) {
	target := &testAnchorMark{
		ID: "target",
		pt: gfx.Point{X: 30, Y: 30},
	}
	menu := &Menu{
		Anchor: AnchorSourceRef{MarkID: "target", Anchor: "center"},
		Items:  []MenuItem{{Key: "a"}},
	}
	root := facet.NewFacet()
	root.AddChild(menu.Base())
	root.AddChild(target.Base())
	got := menu.anchorPoint()
	if got != (gfx.Point{X: 30, Y: 30}) {
		t.Fatalf("anchor point = %#v, want center", got)
	}
}

func TestMenu_keyboard_navigation_skips_disabled_and_activates(t *testing.T) {
	var selected string
	m := &Menu{
		Open: store.NewBinding(true),
		Items: []MenuItem{
			{Key: "a", Disabled: true},
			{Key: "b"},
			{Key: "c"},
		},
		OnSelect: func(key string) { selected = key },
	}
	m.ensureInit()
	m.focusRole.OnFocusGained()
	if !m.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyUp}) {
		t.Fatal("expected up key to be handled")
	}
	if m.highlight != 2 {
		t.Fatalf("highlight = %d, want 2", m.highlight)
	}
	if !m.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter to be handled")
	}
	if selected != "c" {
		t.Fatalf("selected = %q, want c", selected)
	}
	if m.Open.Get() {
		t.Fatal("menu should close after activation")
	}
}

func TestMenu_outside_press_closes(t *testing.T) {
	m := &Menu{
		Open: store.NewBinding(true),
		Items: []MenuItem{
			{Key: "a"},
		},
	}
	m.ensureInit()
	if !m.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 500, Y: 500}}) {
		t.Fatal("expected outside press to be handled")
	}
	if m.Open.Get() {
		t.Fatal("menu should close on outside press")
	}
}

func TestMenu_dense_item_height_is_compact(t *testing.T) {
	m := &Menu{Dense: true}
	if got := m.itemHeight(); got != 22 {
		t.Fatalf("item height = %v, want 22", got)
	}
}

func TestSpeedDial_fab_toggle_action_positions_and_activation(t *testing.T) {
	var activated string
	target := &testAnchorMark{
		ID: "anchor",
		pt: gfx.Point{X: 200, Y: 200},
	}
	s := &SpeedDial{
		Open: store.NewBinding(false),
		Anchor: AnchorSourceRef{
			MarkID: "anchor",
			Anchor: "center",
		},
		Actions: []SpeedDialAction{
			{Key: "compose"},
			{Key: "share"},
		},
		OnAction: func(key string) { activated = key },
	}
	root := facet.NewFacet()
	root.AddChild(s.Base())
	root.AddChild(target.Base())
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 210, Y: 210}}) {
		t.Fatal("expected fab press to be handled")
	}
	if !s.Open.Get() {
		t.Fatal("speed dial should open after fab press")
	}
	if got := s.actionRect(0); got.Min.Y != 140 {
		t.Fatalf("first action rect = %#v, want min.y 140", got)
	}
	if got := s.actionRect(1); got.Min.Y != 80 {
		t.Fatalf("second action rect = %#v, want min.y 80", got)
	}
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 210, Y: 90}}) {
		t.Fatal("expected action press to be handled")
	}
	if activated != "share" {
		t.Fatalf("activated = %q, want share", activated)
	}
	if s.Open.Get() {
		t.Fatal("speed dial should close after action")
	}
}
