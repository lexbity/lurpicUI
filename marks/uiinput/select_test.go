package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestSelect_click_opens_popup(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}}) {
		t.Fatal("expected trigger press to be handled")
	}
	if !s.open {
		t.Fatal("expected popup to open")
	}
}

func TestSelect_hover_updates_state(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	s.layoutRole.Arrange(gfx.RectFromXYWH(25, 30, 300, 40))
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerMove, Position: gfx.Point{X: 40, Y: 40}}) {
		t.Fatal("expected hover move to be handled")
	}
	if !s.state.hovered {
		t.Fatal("expected hovered state to update")
	}
}

func TestSelect_keyboard_navigation(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}, {Key: "c"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	s.state.focused = true
	if !s.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected key to be handled")
	}
	if !s.open || s.highlight != 1 {
		t.Fatalf("open=%v highlight=%d", s.open, s.highlight)
	}
}

func TestSelect_outside_click_dismisses(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	s.open = true
	s.layoutRole.Arrange(gfx.RectFromXYWH(0, 0, 300, 40))
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 300, Y: 300}}) {
		t.Fatal("outside press should be handled for state cleanup")
	}
	if s.open {
		t.Fatal("expected popup to close on outside press")
	}
}

func TestSelect_selects_option_updates_store(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}}) {
		t.Fatal("expected trigger press to be handled")
	}
	s.open = true
	s.state.focused = true
	s.highlight = 1
	if !s.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter to be handled")
	}
	if got := s.Selected.Get(); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}
}

func TestSelect_uses_arranged_bounds_for_hit_testing(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	s.layoutRole.Arrange(gfx.RectFromXYWH(100, 200, 300, 40))
	if got := s.bounds(); got.Min.X != 100 || got.Min.Y != 200 || got.Width() != 300 {
		t.Fatalf("bounds = %#v, want arranged bounds", got)
	}
	if got := s.popupBounds(); got.Min.X != 100 || got.Min.Y != 240 || got.Width() != 300 {
		t.Fatalf("popup bounds = %#v, want arranged popup below trigger", got)
	}
}

func TestSelect_layerBounds_expandWhenOpen(t *testing.T) {
	s := &Select{
		Options: []SelectOption{{Key: "a"}, {Key: "b"}},
		Selected: store.NewBinding("a"),
	}
	s.ensureInit()
	s.layoutRole.Arrange(gfx.RectFromXYWH(100, 200, 300, 40))
	closed := s.LayerBounds()
	if closed.Height() != 40 {
		t.Fatalf("closed layer height = %v, want 40", closed.Height())
	}
	s.open = true
	open := s.LayerBounds()
	if open.Height() <= closed.Height() {
		t.Fatalf("open layer height = %v, want > %v", open.Height(), closed.Height())
	}
	if !open.Contains(gfx.Point{X: 150, Y: 250}) {
		t.Fatalf("open layer bounds should contain popup row")
	}
}
