package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestRadioGroup_single_selection_invariant(t *testing.T) {
	selected := store.NewBinding("a")
	r := &RadioGroup{
		Options: []RadioOption{{Key: "a"}, {Key: "b"}},
		Selected: selected,
	}
	r.ensureInit()
	if got := r.Selected.Get(); got != "a" {
		t.Fatalf("selected = %q, want a", got)
	}
	r.Selected.Set("b")
	r.syncRoles()
	if got := r.Selected.Get(); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}
}

func TestRadioGroup_arrow_keys_change_selection(t *testing.T) {
	selected := store.NewBinding("a")
	r := &RadioGroup{
		Options: []RadioOption{{Key: "a"}, {Key: "b"}, {Key: "c"}},
		Selected: selected,
	}
	r.ensureInit()
	r.state.focused = true
	r.focusedIndex = 0
	if !r.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := r.Selected.Get(); got != "b" {
		t.Fatalf("selected = %q, want b", got)
	}
}

func TestRadioGroup_focus_moves_with_selection(t *testing.T) {
	selected := store.NewBinding("a")
	r := &RadioGroup{
		Options: []RadioOption{{Key: "a"}, {Key: "b"}, {Key: "c"}},
		Selected: selected,
	}
	r.ensureInit()
	r.state.focused = true
	r.Selected.Set("c")
	r.syncRoles()
	if got := r.focusedIndex; got != 2 {
		t.Fatalf("focusedIndex = %d, want 2", got)
	}
}
