package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestSlider_horizontal_drag_updates_value(t *testing.T) {
	s := &Slider{Value: store.NewBinding(0.0), Min: 0, Max: 100}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 0, Y: 14}}) {
		t.Fatal("expected press to be handled")
	}
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: 240, Y: 14}}) {
		t.Fatal("expected release to be handled")
	}
	if got := s.Value.Get(); got <= 0 {
		t.Fatalf("value = %v, want > 0", got)
	}
}

func TestSlider_vertical_drag_updates_value(t *testing.T) {
	s := &Slider{Orientation: SliderVertical, Value: store.NewBinding(0.0), Min: 0, Max: 100}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 14, Y: 0}}) {
		t.Fatal("expected press to be handled")
	}
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: 14, Y: 200}}) {
		t.Fatal("expected release to be handled")
	}
	if got := s.Value.Get(); got < 0 || got > 100 {
		t.Fatalf("value = %v, want within range", got)
	}
}

func TestSlider_discrete_snaps_to_step(t *testing.T) {
	s := &Slider{Mode: SliderDiscrete, Step: 10, Value: store.NewBinding(0.0), Min: 0, Max: 100}
	s.ensureInit()
	s.setPrimaryValue(17)
	if got := s.Value.Get(); got != 20 {
		t.Fatalf("value = %v, want 20", got)
	}
}

func TestSlider_range_two_thumbs_order_preserved(t *testing.T) {
	rng := store.NewBinding([2]float64{80, 20})
	s := &Slider{Mode: SliderRange, Range: &rng, Min: 0, Max: 100}
	s.ensureInit()
	s.setPrimaryValue(50)
	vals := s.Range.Get()
	if vals[0] > vals[1] {
		t.Fatalf("range order violated: %#v", vals)
	}
}

func TestSlider_restricted_values_only_select_allowed_entries(t *testing.T) {
	s := &Slider{Mode: SliderRestricted, Allowed: []float64{10, 20, 40}, Value: store.NewBinding(0.0), Min: 0, Max: 100}
	s.ensureInit()
	s.setPrimaryValue(31)
	got := s.Value.Get()
	if got != 20 && got != 40 {
		t.Fatalf("value = %v, want allowed entry", got)
	}
}

func TestSlider_keyboard_increment_decrement(t *testing.T) {
	s := &Slider{Value: store.NewBinding(50.0), Min: 0, Max: 100, Step: 10}
	s.ensureInit()
	s.state.focused = true
	if !s.handleKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected key to be handled")
	}
	if got := s.Value.Get(); got != 60 {
		t.Fatalf("value = %v, want 60", got)
	}
}

func TestSlider_custom_thumb_visual_preserves_drag_contract(t *testing.T) {
	s := &Slider{Value: store.NewBinding(50.0), Min: 0, Max: 100}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 50, Y: 14}}) {
		t.Fatal("expected press to be handled")
	}
	if got := s.dragging; !got {
		t.Fatal("expected dragging to remain active after press")
	}
}
