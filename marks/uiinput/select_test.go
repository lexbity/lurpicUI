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
	if s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 300, Y: 300}}) {
		t.Fatal("outside press should not be consumed")
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
