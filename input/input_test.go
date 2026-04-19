package input

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
)

func TestGestureConfig_defaults(t *testing.T) {
	cfg := DefaultGestureConfig()
	if cfg.DragThreshold <= 0 {
		t.Fatalf("drag threshold = %v", cfg.DragThreshold)
	}
	if cfg.DoubleClickInterval <= 0 {
		t.Fatalf("double click interval = %v", cfg.DoubleClickInterval)
	}
	if cfg.DoubleClickRadius <= 0 {
		t.Fatalf("double click radius = %v", cfg.DoubleClickRadius)
	}
	if cfg.HoverDelay <= 0 {
		t.Fatalf("hover delay = %v", cfg.HoverDelay)
	}
	if cfg.ScrollMultiplier <= 0 {
		t.Fatalf("scroll multiplier = %v", cfg.ScrollMultiplier)
	}
}

func TestPointerState_initial_values(t *testing.T) {
	var state PointerState
	if state.Position != (gfx.Point{}) {
		t.Fatalf("position = %#v", state.Position)
	}
	if state.PressTarget != nil {
		t.Fatal("expected nil press target")
	}
	if state.DragActive {
		t.Fatal("expected drag inactive")
	}
	if state.PressedButton != platform.PointerNone {
		t.Fatalf("button = %v", state.PressedButton)
	}
}

func TestSystem_getOrCreate_same_id_returns_same(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	a := sys.getOrCreatePointer(0)
	b := sys.getOrCreatePointer(0)
	if a != b {
		t.Fatal("expected same pointer state")
	}
}

func TestSystem_getOrCreate_different_ids_different(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	a := sys.getOrCreatePointer(0)
	b := sys.getOrCreatePointer(1)
	if a == b {
		t.Fatal("expected distinct pointer states")
	}
}

func TestSystem_clearPointerState_nils_all_captures(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	a := sys.getOrCreatePointer(0)
	b := sys.getOrCreatePointer(1)
	a.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	b.PressTarget = &CaptureTarget{FacetID: 33, MarkID: 44}
	a.DragActive = true
	b.DragActive = true
	a.PressedButton = platform.PointerLeft
	b.PressedButton = platform.PointerRight
	sys.ClearPointerState()
	if a.PressTarget != nil || b.PressTarget != nil {
		t.Fatal("expected cleared press targets")
	}
	if a.DragActive || b.DragActive {
		t.Fatal("expected cleared drag state")
	}
	if a.PressedButton != platform.PointerNone || b.PressedButton != platform.PointerNone {
		t.Fatal("expected cleared buttons")
	}
}

func TestResolveClickCount_single_click(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	if got := sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, time.Unix(0, 0)); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestResolveClickCount_double_click(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	base := time.Unix(0, 0)
	if got := sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base); got != 1 {
		t.Fatalf("got %d", got)
	}
	if got := sys.resolveClickCount(gfx.Point{X: 2, Y: 3}, base.Add(100*time.Millisecond)); got != 2 {
		t.Fatalf("got %d", got)
	}
}

func TestResolveClickCount_triple_click(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	base := time.Unix(0, 0)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(100*time.Millisecond))
	if got := sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(200*time.Millisecond)); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestResolveClickCount_resets_on_timeout(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	base := time.Unix(0, 0)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base)
	if got := sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(500*time.Millisecond)); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestResolveClickCount_resets_on_distance(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	base := time.Unix(0, 0)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base)
	if got := sys.resolveClickCount(gfx.Point{X: 100, Y: 100}, base.Add(100*time.Millisecond)); got != 1 {
		t.Fatalf("got %d", got)
	}
}

func TestHoverState_fires_after_delay(t *testing.T) {
	var h HoverState
	cfg := DefaultGestureConfig()
	base := time.Unix(0, 0)
	h.OnMove(11, 22, base)
	if got := h.Tick(base.Add(cfg.HoverDelay-time.Nanosecond), cfg); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
	got := h.Tick(base.Add(cfg.HoverDelay), cfg)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != 11 {
		t.Fatalf("target = %d", got[0].Target)
	}
	if ev, ok := got[0].Event.(HoverSettledEvent); !ok || ev.MarkID != 22 {
		t.Fatalf("event = %#v", got[0].Event)
	}
}

func TestHoverState_does_not_fire_before_delay(t *testing.T) {
	var h HoverState
	cfg := DefaultGestureConfig()
	base := time.Unix(0, 0)
	h.OnMove(11, 22, base)
	if got := h.Tick(base.Add(cfg.HoverDelay/2), cfg); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestHoverState_resets_on_move(t *testing.T) {
	var h HoverState
	cfg := DefaultGestureConfig()
	base := time.Unix(0, 0)
	h.OnMove(11, 22, base)
	if got := h.Tick(base.Add(cfg.HoverDelay), cfg); len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	h.OnMove(33, 44, base.Add(2*cfg.HoverDelay))
	if got := h.Tick(base.Add(3*cfg.HoverDelay), cfg); len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got := h.Tick(base.Add(3*cfg.HoverDelay+time.Second), cfg); len(got) != 0 {
		t.Fatalf("expected one hover per idle period, got %#v", got)
	}
}

func TestHoverState_fires_once_per_idle(t *testing.T) {
	var h HoverState
	cfg := DefaultGestureConfig()
	base := time.Unix(0, 0)
	h.OnMove(11, 22, base)
	if got := h.Tick(base.Add(cfg.HoverDelay), cfg); len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got := h.Tick(base.Add(cfg.HoverDelay+time.Second), cfg); len(got) != 0 {
		t.Fatalf("len = %d", len(got))
	}
	if got := h.Tick(base.Add(cfg.HoverDelay+2*time.Second), cfg); len(got) != 0 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestFocusState_roundtrip(t *testing.T) {
	var f FocusState
	if got := f.Focused(); got != 0 {
		t.Fatalf("got %d", got)
	}
	f.SetFocused(99)
	if got := f.Focused(); got != 99 {
		t.Fatalf("got %d", got)
	}
	f.Clear()
	if got := f.Focused(); got != 0 {
		t.Fatalf("got %d", got)
	}
}

func TestResolveClickCount_caps_at_three(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	base := time.Unix(0, 0)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base)
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(100*time.Millisecond))
	sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(200*time.Millisecond))
	if got := sys.resolveClickCount(gfx.Point{X: 1, Y: 2}, base.Add(300*time.Millisecond)); got != 3 {
		t.Fatalf("got %d", got)
	}
}

func TestHoverState_clear(t *testing.T) {
	var h HoverState
	h.OnMove(11, 22, time.Unix(0, 0))
	h.Clear()
	if got := h.Tick(time.Unix(0, 0).Add(time.Second), DefaultGestureConfig()); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestSystem_clearPointerState_keeps_positions(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.Position = gfx.Point{X: 10, Y: 20}
	ptr.PressPosition = gfx.Point{X: 30, Y: 40}
	sys.ClearPointerState()
	if ptr.Position != (gfx.Point{X: 10, Y: 20}) || ptr.PressPosition != (gfx.Point{X: 30, Y: 40}) {
		t.Fatalf("positions changed: %#v %#v", ptr.Position, ptr.PressPosition)
	}
}

var _ = facet.FacetID(0)
