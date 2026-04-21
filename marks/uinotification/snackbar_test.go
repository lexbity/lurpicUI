package uinotification

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestSnackbar_auto_dismiss_after_duration(t *testing.T) {
	s := &Snackbar{
		Open:     store.NewBinding(true),
		Duration: 100 * time.Millisecond,
	}
	if !s.Tick(50 * time.Millisecond) {
		t.Fatal("expected first tick to be handled")
	}
	if !s.Open.Get() {
		t.Fatal("expected snackbar to stay open before duration")
	}
	if !s.Tick(60 * time.Millisecond) {
		t.Fatal("expected second tick to close snackbar")
	}
	if s.Open.Get() {
		t.Fatal("expected snackbar to close after duration")
	}
}

func TestSnackbar_action_invokes_callback(t *testing.T) {
	var clicked bool
	s := &Snackbar{
		Open: store.NewBinding(true),
		Action: &ButtonAction{
			Label:   "Undo",
			OnClick: func() { clicked = true },
		},
	}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 350, Y: 10}}) {
		t.Fatal("expected action press to be handled")
	}
	if !clicked {
		t.Fatal("expected action callback to run")
	}
	if s.Open.Get() {
		t.Fatal("expected snackbar to close after action")
	}
}

func TestSnackbar_layering_floats_above_content(t *testing.T) {
	s := &Snackbar{Open: store.NewBinding(true)}
	if specs := s.OnLayerSpecs(); len(specs) != 1 || specs[0].RenderOrder != 500 {
		t.Fatalf("layer specs = %#v, want floating render order", specs)
	}
}
