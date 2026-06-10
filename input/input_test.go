package input

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
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
	var state pointerState
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
	a.PressTarget = &captureTarget{FacetID: 11, MarkID: 22}
	b.PressTarget = &captureTarget{FacetID: 33, MarkID: 44}
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
	var h hoverState
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
	var h hoverState
	cfg := DefaultGestureConfig()
	base := time.Unix(0, 0)
	h.OnMove(11, 22, base)
	if got := h.Tick(base.Add(cfg.HoverDelay/2), cfg); len(got) != 0 {
		t.Fatalf("got %#v", got)
	}
}

func TestHoverState_resets_on_move(t *testing.T) {
	var h hoverState
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
	var h hoverState
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
	var f focusState
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
	var h hoverState
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

func TestPointer_rightClickRoutesCorrectly(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	// Right-click press
	events := sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: 100, Y: 200},
			Button:   platform.PointerRight,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for right-click press")
	}
	press, ok := events[0].Event.(PointerPressEvent)
	if !ok {
		t.Fatalf("expected PointerPressEvent, got %T", events[0].Event)
	}
	if press.Button != platform.PointerRight {
		t.Fatalf("expected right button, got %v", press.Button)
	}

	// Right-click release
	events = sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerRelease,
			Position: gfx.Point{X: 100, Y: 200},
			Button:   platform.PointerRight,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for right-click release")
	}
	release, ok := events[0].Event.(ClickEvent)
	if !ok {
		t.Fatalf("expected ClickEvent, got %T", events[0].Event)
	}
	if release.Button != platform.PointerRight {
		t.Fatalf("expected right button, got %v", release.Button)
	}
}

func TestPointer_middleClickRoutesCorrectly(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	events := sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: 150, Y: 250},
			Button:   platform.PointerMiddle,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for middle-click press")
	}
	press, ok := events[0].Event.(PointerPressEvent)
	if !ok {
		t.Fatalf("expected PointerPressEvent, got %T", events[0].Event)
	}
	if press.Button != platform.PointerMiddle {
		t.Fatalf("expected middle button, got %v", press.Button)
	}

	events = sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerRelease,
			Position: gfx.Point{X: 150, Y: 250},
			Button:   platform.PointerMiddle,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for middle-click release")
	}
	release, ok := events[0].Event.(ClickEvent)
	if !ok {
		t.Fatalf("expected ClickEvent, got %T", events[0].Event)
	}
	if release.Button != platform.PointerMiddle {
		t.Fatalf("expected middle button, got %v", release.Button)
	}
}

func TestScroll_eventRoutedToHitFacet(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	// Scroll over the hit target
	events := sys.Process([]platform.Event{
		platform.EventScroll{
			Position: gfx.Point{X: 100, Y: 100},
			DeltaX:   0,
			DeltaY:   -50,
			Precise:  true,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for scroll")
	}
	scroll, ok := events[0].Event.(ScrollEvent)
	if !ok {
		t.Fatalf("expected ScrollEvent, got %T", events[0].Event)
	}
	if scroll.DeltaY != -50 {
		t.Fatalf("expected DeltaY -50, got %f", scroll.DeltaY)
	}
	if scroll.Precise != true {
		t.Fatal("expected precise scroll")
	}
	if events[0].Target != 42 {
		t.Fatalf("expected target 42, got %d", events[0].Target)
	}
}

func TestScroll_horizontalDeltaPassedThrough(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	events := sys.Process([]platform.Event{
		platform.EventScroll{
			Position: gfx.Point{X: 100, Y: 100},
			DeltaX:   30,
			DeltaY:   0,
			Precise:  true,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for horizontal scroll")
	}
	scroll, ok := events[0].Event.(ScrollEvent)
	if !ok {
		t.Fatalf("expected ScrollEvent, got %T", events[0].Event)
	}
	if scroll.DeltaX != 30 {
		t.Fatalf("expected DeltaX 30, got %f", scroll.DeltaX)
	}
}

func TestPointerCancel_clearsPressState(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: 100, Y: 100},
			Button:   platform.PointerLeft,
		},
	}, hitMap, tree)
	ptr := sys.getOrCreatePointer(0)
	if ptr.PressTarget == nil {
		t.Fatal("expected press target after down")
	}

	sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerCancel,
			Position: gfx.Point{X: 100, Y: 100},
		},
	}, hitMap, tree)
	if ptr.PressTarget != nil {
		t.Fatal("expected nil press target after cancel")
	}
	if ptr.PressedButton != platform.PointerNone {
		t.Fatal("expected PointerNone after cancel")
	}
}

func TestTouchCancel_clearsTouchState(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	sys.Process([]platform.Event{
		platform.TouchEvent{SequenceID: 1, Phase: platform.TouchDown, X: 100, Y: 100, Pressure: 0.8},
	}, hitMap, tree)
	touch := sys.getOrCreateTouch(1)
	if !touch.Active {
		t.Fatal("expected active touch after down")
	}

	sys.Process([]platform.Event{
		platform.TouchEvent{SequenceID: 1, Phase: platform.TouchCancel, X: 100, Y: 100},
	}, hitMap, tree)
	if touch.Active {
		t.Fatal("expected inactive touch after cancel")
	}
	if touch.Target != 0 {
		t.Fatal("expected zero target after cancel")
	}
}

func TestStylusPressure_routedThroughPointer(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	hitMap := singleHitMap(42)
	tree := newRootWithFacet(42)

	events := sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerPress,
			Position: gfx.Point{X: 200, Y: 300},
			Button:   platform.PointerLeft,
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for stylus press")
	}
	press, ok := events[0].Event.(PointerPressEvent)
	if !ok {
		t.Fatalf("expected PointerPressEvent, got %T", events[0].Event)
	}
	if press.Position.X != 200 || press.Position.Y != 300 {
		t.Fatalf("expected position (200,300), got (%f,%f)", press.Position.X, press.Position.Y)
	}
	if press.Button != platform.PointerLeft {
		t.Fatalf("expected left button, got %v", press.Button)
	}

	events = sys.Process([]platform.Event{
		platform.EventPointer{
			Kind:     platform.PointerMove,
			Position: gfx.Point{X: 210, Y: 310},
		},
	}, hitMap, tree)
	if len(events) == 0 {
		t.Fatal("expected routed event for stylus move")
	}
	// The move may trigger a DragStartEvent first (crossing drag threshold).
	switch ev := events[0].Event.(type) {
	case DragMoveEvent:
		if ev.Position.X != 210 || ev.Position.Y != 310 {
			t.Fatalf("expected position (210,310), got (%f,%f)", ev.Position.X, ev.Position.Y)
		}
	case DragStartEvent:
		if len(events) >= 2 {
			move, ok := events[1].Event.(DragMoveEvent)
			if !ok {
				t.Fatalf("expected second event to be DragMoveEvent, got %T", events[1].Event)
			}
			if move.Position.X != 210 || move.Position.Y != 310 {
				t.Fatalf("expected position (210,310), got (%f,%f)", move.Position.X, move.Position.Y)
			}
		}
	default:
		t.Fatalf("expected DragStartEvent or DragMoveEvent, got %T", events[0].Event)
	}
}

// singleHitMap returns a HitMap with a single entry for the given facetID
// at full-surface clip with identity transform.
func singleHitMap(facetID facet.FacetID) *projection.HitMap {
	return projection.NewHitMap(projection.HitMapEntry{
		FacetID:   facetID,
		Transform: gfx.Identity(),
		ClipRect:  gfx.Rect{Max: gfx.Point{X: 1000, Y: 1000}},
		Regions: []projection.HitRegion{{
			Bounds: gfx.Rect{Max: gfx.Point{X: 1000, Y: 1000}},
		}},
		LayerOrder: 1,
		HitPolicy:  facet.HitNormal,
	})
}

// newRootWithFacet creates a minimal facet tree for tests. The tree is a
// single facet.FacetImpl that the input system can pass as the focus tree.
// The hit map determines routing, not this tree, so the tree only needs to
// satisfy the FacetImpl interface without panicking. requestFocus will not
// find the target ID (since the real facet has an auto-assigned ID), so
// focus transitions are skipped — acceptable for these routing tests.
func newRootWithFacet(_ facet.FacetID) facet.FacetImpl {
	f := facet.NewFacet()
	return &simpleFacet{base: &f}
}

type simpleFacet struct {
	base *facet.Facet
}

func (s *simpleFacet) Base() *facet.Facet           { return s.base }
func (s *simpleFacet) OnAttach(facet.AttachContext) {}
func (s *simpleFacet) OnDetach()                    {}
func (s *simpleFacet) OnActivate()                  {}
func (s *simpleFacet) OnDeactivate()                {}

var _ = facet.FacetID(0)
