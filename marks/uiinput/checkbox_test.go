package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestCheckbox_toggle_updates_store(t *testing.T) {
	checked := store.NewBinding(false)
	c := &Checkbox{Checked: checked}
	c.ensureInit()
	if !c.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("expected press to be handled")
	}
	if !c.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease}) {
		t.Fatal("expected release to be handled")
	}
	if got := c.Checked.Get(); !got {
		t.Fatalf("checked = %v, want true", got)
	}
}

func TestCheckbox_keyboard_toggle(t *testing.T) {
	checked := store.NewBinding(false)
	c := &Checkbox{Checked: checked}
	c.ensureInit()
	c.state.focused = true
	if !c.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter to be handled")
	}
	if got := c.Checked.Get(); !got {
		t.Fatalf("checked = %v, want true", got)
	}
}

func TestCheckbox_disabled_no_toggle(t *testing.T) {
	checked := store.NewBinding(false)
	c := &Checkbox{Disabled: true, Checked: checked}
	c.ensureInit()
	if c.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("disabled press should not be handled")
	}
	if c.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("disabled key should not be handled")
	}
	if got := c.Checked.Get(); got {
		t.Fatalf("checked = %v, want false", got)
	}
}
