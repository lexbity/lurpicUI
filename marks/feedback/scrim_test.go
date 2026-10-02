package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

func measureScrim(t *testing.T, s *Scrim, w, h float32) {
	t.Helper()
	s.measure(facet.MeasureContext{Theme: theme.DefaultResolvedContext()}, facet.Constraints{MaxSize: gfx.Size{W: w, H: h}})
}

func TestScrim_fills_bounds_and_dims(t *testing.T) {
	s := NewScrim()
	measureScrim(t, s, 400, 300)
	bounds := gfx.RectFromXYWH(0, 0, 400, 300)
	s.Layout.Arrange(facet.ArrangeContext{}, bounds)

	cmds := s.buildCommands(bounds)
	if len(cmds) == 0 {
		t.Fatal("expected dim fill commands for an arranged scrim")
	}
	fill, ok := cmds[0].(gfx.FillRect)
	if !ok {
		t.Fatalf("expected FillRect, got %T", cmds[0])
	}
	if fill.Rect != bounds {
		t.Fatalf("scrim fill %v does not cover bounds %v", fill.Rect, bounds)
	}
	if fill.Brush.Color.A == 0 {
		t.Fatal("themed scrim color must be non-transparent")
	}
}

func TestScrim_empty_bounds_render_and_hit_nothing(t *testing.T) {
	s := NewScrim()
	measureScrim(t, s, 400, 300)
	if cmds := s.buildCommands(gfx.Rect{}); len(cmds) != 0 {
		t.Fatal("empty arrangement must contribute no commands")
	}
	if hit := s.Hit.OnHitTest(gfx.Point{X: 10, Y: 10}); hit.Hit {
		t.Fatal("empty arrangement must not hit")
	}
}

func TestScrim_explicit_color_override(t *testing.T) {
	s := NewScrim()
	s.Color = marks.Const(gfx.Color{R: 0, G: 0, B: 0, A: 0.6})
	measureScrim(t, s, 100, 100)
	bounds := gfx.RectFromXYWH(0, 0, 100, 100)
	cmds := s.buildCommands(bounds)
	if len(cmds) == 0 {
		t.Fatal("expected commands")
	}
	fill := cmds[0].(gfx.FillRect)
	if c := fill.Brush.Color; c != (gfx.Color{R: 0, G: 0, B: 0, A: 0.6}) {
		t.Fatalf("override color not honored: %#v", c)
	}
}

func TestScrim_press_dismisses_and_blocks(t *testing.T) {
	s := NewScrim()
	measureScrim(t, s, 200, 200)
	s.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, 200, 200))

	fired := 0
	id := s.Dismissed.Subscribe(func(signal.Unit) { fired++ })
	defer s.Dismissed.Unsubscribe(id)

	// A left press inside the scrim dismisses and consumes.
	if !s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Button: platform.PointerLeft, Position: gfx.Point{X: 100, Y: 100}}) {
		t.Fatal("press inside the scrim must be consumed")
	}
	if fired != 1 {
		t.Fatalf("press did not dismiss (fired=%d)", fired)
	}
	// Release/hover are consumed but do not re-dismiss.
	s.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Button: platform.PointerLeft, Position: gfx.Point{X: 100, Y: 100}})
	if fired != 1 {
		t.Fatalf("release re-dismissed (fired=%d)", fired)
	}
	// A press outside the arranged bounds does nothing.
	if s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Button: platform.PointerLeft, Position: gfx.Point{X: 500, Y: 500}}) {
		t.Fatal("press outside bounds must not be consumed")
	}
	if fired != 1 {
		t.Fatalf("outside press dismissed (fired=%d)", fired)
	}
}

func TestScrim_dismissal_scope(t *testing.T) {
	s := NewScrim()
	measureScrim(t, s, 100, 100)

	// No scope: the layer dismissal path is inert.
	if s.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerPointer}) {
		t.Fatal("dismissal without a scope must be inert")
	}

	s.SetDismissal(facet.DismissalScope{
		Enabled:  true,
		Triggers: facet.DismissalTriggerSetPointer,
	})
	fired := 0
	id := s.Dismissed.Subscribe(func(signal.Unit) { fired++ })
	defer s.Dismissed.Unsubscribe(id)
	if !s.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerPointer}) {
		t.Fatal("permitted dismissal trigger must dismiss")
	}
	if fired != 1 {
		t.Fatalf("dismissed signal not emitted (fired=%d)", fired)
	}
	// A non-permitted trigger is ignored.
	if s.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerKey}) {
		t.Fatal("non-permitted trigger must be ignored")
	}
	if fired != 1 {
		t.Fatalf("non-permitted trigger dismissed (fired=%d)", fired)
	}
}

func TestScrim_descriptor(t *testing.T) {
	d := NewScrim().Descriptor()
	if d.Family != "feedback" || d.TypeName != "scrim" {
		t.Fatalf("descriptor = %v", d)
	}
}
