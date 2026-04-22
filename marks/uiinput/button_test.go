package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/platform"
)

func TestButton_pointer_click_invokes_action(t *testing.T) {
	var calls int
	b := &Button{OnPress: func() { calls++ }}
	b.ensureInit()
	b.Base().LayoutRole().Arrange(gfx.RectFromXYWH(0, 0, 96, buttonHeight()))
	if !b.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("expected press to be handled")
	}
	if !b.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease}) {
		t.Fatal("expected release to be handled")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestButton_pointer_enter_updates_hover_state(t *testing.T) {
	b := &Button{}
	b.ensureInit()
	if !b.handlePointer(facet.PointerEvent{Kind: platform.PointerEnter}) {
		t.Fatal("expected enter to be handled")
	}
	if !b.state.hovered {
		t.Fatal("expected hover state")
	}
}

func TestButton_keyboard_space_invokes_action(t *testing.T) {
	var calls int
	b := &Button{OnPress: func() { calls++ }}
	b.ensureInit()
	b.state.focused = true
	if !b.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space to be handled")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}

func TestButton_disabled_blocks_action(t *testing.T) {
	var calls int
	b := &Button{Disabled: true, OnPress: func() { calls++ }}
	b.ensureInit()
	if b.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("disabled press should not be handled")
	}
	if b.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("disabled key should not be handled")
	}
	if calls != 0 {
		t.Fatalf("calls = %d, want 0", calls)
	}
}

func TestButton_focus_ring_slot_used_when_focused(t *testing.T) {
	b := &Button{Variant: ButtonOutlined}
	b.ensureInit()
	b.state.focused = true
	list := b.project(facet.ProjectionContext{})
	if list == nil {
		t.Fatal("expected command list")
	}
	foundStroke := false
	for _, cmd := range list.Commands {
		if _, ok := cmd.(gfx.StrokeRect); ok {
			foundStroke = true
			break
		}
	}
	if !foundStroke {
		t.Fatal("expected focused button to emit a stroke rect")
	}
}

func TestButton_custom_icon_slot_preserves_press_behavior(t *testing.T) {
	var calls int
	b := &Button{
		Icon:    &annotation.Icon{Name: "missing"},
		OnPress: func() { calls++ },
	}
	b.ensureInit()
	if !b.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("expected press to be handled")
	}
	if !b.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease}) {
		t.Fatal("expected release to be handled")
	}
	if calls != 1 {
		t.Fatalf("calls = %d, want 1", calls)
	}
}
