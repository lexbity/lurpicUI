package uinotification

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestDialog_modal_blocks_below(t *testing.T) {
	d := &Dialog{
		Open:              store.NewBinding(true),
		DismissOnBackdrop: false,
		DismissOnEscape:   true,
	}
	if !d.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}}) {
		t.Fatal("expected backdrop press to be consumed")
	}
	if !d.Open.Get() {
		t.Fatal("dialog should remain open when backdrop dismiss is disabled")
	}
}

func TestDialog_focus_trap(t *testing.T) {
	d := &Dialog{
		Open:    store.NewBinding(true),
		Actions: []marks.Mark{&basic.Rect{}},
	}
	d.ensureInit()
	if !d.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyTab}) {
		t.Fatal("expected tab to be handled")
	}
	if d.focusIndex != 0 {
		t.Fatalf("focus index = %d, want 0 with one action", d.focusIndex)
	}
}

func TestDialog_escape_dismiss_when_enabled(t *testing.T) {
	d := &Dialog{
		Open:            store.NewBinding(true),
		DismissOnEscape: true,
	}
	if !d.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to be handled")
	}
	if d.Open.Get() {
		t.Fatal("expected dialog to close on escape")
	}
}

func TestDialog_custom_action_area_preserves_modal_behavior(t *testing.T) {
	d := &Dialog{
		Open:    store.NewBinding(true),
		Body:    []marks.Mark{&basic.Rect{}},
		Actions: []marks.Mark{&basic.Rect{}},
	}
	d.ensureInit()
	if specs := d.OnLayerSpecs(); len(specs) != 2 {
		t.Fatalf("layer specs = %d, want 2", len(specs))
	}
}

func TestDialog_body_can_host_arbitrary_marks(t *testing.T) {
	d := &Dialog{
		Open: store.NewBinding(true),
		Body: []marks.Mark{&basic.Rect{}},
	}
	d.ensureInit()
	if got := len(d.base.Children()); got == 0 {
		t.Fatal("expected body child to be attached")
	}
}
