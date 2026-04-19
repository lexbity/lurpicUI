package input

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

type testInputFacet struct {
	facet.Facet
	in   facet.InputRole
	foc  facet.FocusRole
	text []string
	keys []platform.Key
}

func newTestInputFacet(tabIndex int, focusable bool) *testInputFacet {
	f := &testInputFacet{Facet: facet.NewFacet()}
	f.foc.TabIndex = tabIndex
	f.foc.Focusable = func() bool { return focusable }
	f.foc.OnFocusGained = func() {}
	f.foc.OnFocusLost = func() {}
	f.AddRole(&f.in)
	f.AddRole(&f.foc)
	return f
}

func (f *testInputFacet) Base() *facet.Facet               { return &f.Facet }
func (f *testInputFacet) OnAttach(ctx facet.AttachContext) {}
func (f *testInputFacet) OnDetach()                        {}
func (f *testInputFacet) OnActivate()                      {}
func (f *testInputFacet) OnDeactivate()                    {}

func newHitMapFor(facetID facet.FacetID, transform gfx.Transform, regions ...projection.HitRegion) *projection.HitMap {
	return projection.NewHitMap(projection.HitMapEntry{
		FacetID:   facetID,
		Transform: transform,
		Regions:   regions,
	})
}

func TestPointerPress_returns_press_event(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	hitMap := newHitMapFor(11, gfx.Identity(), projection.HitRegion{
		Bounds: gfx.RectFromXYWH(0, 0, 50, 50),
		MarkID: 22,
		Cursor: facet.CursorPointer,
	})
	got := sys.handlePress(ptr, platform.EventPointer{
		Kind:     platform.PointerPress,
		Position: gfx.Point{X: 10, Y: 10},
		Button:   platform.PointerLeft,
	}, hitMap)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != 11 {
		t.Fatalf("target = %d", got[0].Target)
	}
	ev, ok := got[0].Event.(PointerPressEvent)
	if !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
	if ev.MarkID != 22 || ev.ClickCount != 1 {
		t.Fatalf("event = %#v", ev)
	}
	if ptr.PressTarget == nil || ptr.PressTarget.FacetID != 11 {
		t.Fatalf("capture = %#v", ptr.PressTarget)
	}
}

func TestPointerPress_no_hit_returns_nothing(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	got := sys.handlePress(ptr, platform.EventPointer{
		Kind:     platform.PointerPress,
		Position: gfx.Point{X: 10, Y: 10},
		Button:   platform.PointerLeft,
	}, projection.NewHitMap())
	if len(got) != 0 {
		t.Fatalf("len = %d", len(got))
	}
	if ptr.PressTarget != nil {
		t.Fatalf("capture = %#v", ptr.PressTarget)
	}
}

func TestPointerMove_captured_goes_to_press_target(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.PressPosition = gfx.Point{X: 0, Y: 0}
	hitMap := newHitMapFor(99, gfx.Identity(), projection.HitRegion{
		Bounds: gfx.RectFromXYWH(0, 0, 100, 100),
		MarkID: 77,
	})
	got := sys.handleMove(ptr, platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 2, Y: 2},
	}, hitMap)
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != 11 {
		t.Fatalf("target = %d", got[0].Target)
	}
	if _, ok := got[0].Event.(PointerMoveEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
}

func TestPointerMove_drag_threshold_not_crossed(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.PressPosition = gfx.Point{X: 0, Y: 0}
	got := sys.handleMove(ptr, platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 2, Y: 0},
	}, newHitMapFor(11, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].Event.(PointerMoveEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
	if ptr.DragActive {
		t.Fatal("drag should not be active")
	}
}

func TestPointerMove_drag_threshold_crossed(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.PressPosition = gfx.Point{X: 0, Y: 0}
	got := sys.handleMove(ptr, platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 5, Y: 0},
	}, newHitMapFor(11, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}))
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].Event.(DragStartEvent); !ok {
		t.Fatalf("event0 = %T", got[0].Event)
	}
	if _, ok := got[1].Event.(DragMoveEvent); !ok {
		t.Fatalf("event1 = %T", got[1].Event)
	}
	if !ptr.DragActive {
		t.Fatal("drag should be active")
	}
}

func TestPointerMove_drag_fires_dragmove_while_active(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.DragActive = true
	ptr.PressPosition = gfx.Point{X: 0, Y: 0}
	got := sys.handleMove(ptr, platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 10, Y: 10},
	}, newHitMapFor(11, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].Event.(DragMoveEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
}

func TestPointerRelease_after_drag_emits_dragend(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.PressedButton = platform.PointerLeft
	ptr.DragActive = true
	got := sys.handleRelease(ptr, platform.EventPointer{
		Kind:     platform.PointerRelease,
		Position: gfx.Point{X: 10, Y: 10},
		Button:   platform.PointerLeft,
	}, newHitMapFor(11, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].Event.(DragEndEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
	if ptr.PressTarget != nil || ptr.DragActive {
		t.Fatal("capture should be cleared")
	}
}

func TestPointerRelease_no_drag_emits_click(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 11, MarkID: 22}
	ptr.PressedButton = platform.PointerLeft
	ptr.clickCount = 2
	got := sys.handleRelease(ptr, platform.EventPointer{
		Kind:     platform.PointerRelease,
		Position: gfx.Point{X: 10, Y: 10},
		Button:   platform.PointerLeft,
	}, newHitMapFor(11, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50)}))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	ev, ok := got[0].Event.(ClickEvent)
	if !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
	if ev.ClickCount != 2 {
		t.Fatalf("click count = %d", ev.ClickCount)
	}
}

func TestPointerMove_enter_leave_on_boundary_cross(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	oldID := facet.FacetID(1)
	newID := facet.FacetID(2)
	sys.hover.OnMove(oldID, 10, time.Unix(0, 0))
	hitMap := projection.NewHitMap(projection.HitMapEntry{
		FacetID:   newID,
		Transform: gfx.Identity(),
		Regions:   []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 20}},
	})
	got := sys.handleMove(ptr, platform.EventPointer{
		Kind:     platform.PointerMove,
		Position: gfx.Point{X: 1, Y: 1},
	}, hitMap)
	if len(got) < 2 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != oldID {
		t.Fatalf("leave target = %d", got[0].Target)
	}
	if _, ok := got[0].Event.(PointerLeaveEvent); !ok {
		t.Fatalf("event0 = %T", got[0].Event)
	}
	if got[1].Target != newID {
		t.Fatalf("enter target = %d", got[1].Target)
	}
	if _, ok := got[1].Event.(PointerEnterEvent); !ok {
		t.Fatalf("event1 = %T", got[1].Event)
	}
}

func TestTransformToLocal_identity_unchanged(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	pt := gfx.Point{X: 10, Y: 20}
	if got := sys.transformToLocal(pt, 1, projection.NewHitMap(projection.HitMapEntry{FacetID: 1, Transform: gfx.Identity()})); got != pt {
		t.Fatalf("got %#v", got)
	}
}

func TestTransformToLocal_translation_applied(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	got := sys.transformToLocal(gfx.Point{X: 110, Y: 70}, 1, projection.NewHitMap(projection.HitMapEntry{
		FacetID:   1,
		Transform: gfx.Transform{A: 1, D: 1, TX: 100, TY: 50},
	}))
	if got != (gfx.Point{X: 10, Y: 20}) {
		t.Fatalf("got %#v", got)
	}
}

func TestTransformToLocal_zoom_applied(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	got := sys.transformToLocal(gfx.Point{X: 20, Y: 40}, 1, projection.NewHitMap(projection.HitMapEntry{
		FacetID:   1,
		Transform: gfx.Transform{A: 2, D: 2},
	}))
	if got != (gfx.Point{X: 10, Y: 20}) {
		t.Fatalf("got %#v", got)
	}
}

func TestTransformToLocal_degenerate_returns_screen(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	pt := gfx.Point{X: 20, Y: 40}
	got := sys.transformToLocal(pt, 1, projection.NewHitMap(projection.HitMapEntry{
		FacetID:   1,
		Transform: gfx.Transform{},
	}))
	if got != pt {
		t.Fatalf("got %#v", got)
	}
}

func TestDeltaToLocal_accounts_for_zoom(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	got := sys.deltaToLocal(gfx.Point{X: 10, Y: 0}, 1, projection.NewHitMap(projection.HitMapEntry{
		FacetID:   1,
		Transform: gfx.Transform{A: 2, D: 2},
	}))
	if got != (gfx.Point{X: 5, Y: 0}) {
		t.Fatalf("got %#v", got)
	}
}

func TestDeliver_bubbles_to_parent(t *testing.T) {
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	var calls []string
	rootRole := &facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "root")
		return true
	}}
	childRole := &facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "child")
		return false
	}}
	root.AddRole(rootRole)
	child.AddRole(childRole)
	got := Deliver(RoutedEvent{Target: child.ID(), Event: PointerPressEvent{}}, &root)
	if !got {
		t.Fatal("expected consumed event")
	}
	if len(calls) != 2 || calls[0] != "child" || calls[1] != "root" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestDeliver_stops_on_consumed(t *testing.T) {
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	var calls []string
	root.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "root")
		return true
	}})
	child.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "child")
		return true
	}})
	if !Deliver(RoutedEvent{Target: child.ID(), Event: PointerPressEvent{}}, &root) {
		t.Fatal("expected consumed event")
	}
	if len(calls) != 1 || calls[0] != "child" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestDeliver_no_bubble_for_enter_leave(t *testing.T) {
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	var calls []string
	root.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "root")
		return true
	}})
	child.AddRole(&facet.InputRole{OnPointer: func(e facet.PointerEvent) bool {
		calls = append(calls, "child")
		return true
	}})
	if !Deliver(RoutedEvent{Target: child.ID(), Event: PointerEnterEvent{}}, &root) {
		t.Fatal("expected consumed event")
	}
	if len(calls) != 1 || calls[0] != "child" {
		t.Fatalf("calls = %#v", calls)
	}
}

func TestDeliver_nil_handler_safe(t *testing.T) {
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	child.AddRole(&facet.InputRole{})
	if Deliver(RoutedEvent{Target: child.ID(), Event: PointerPressEvent{}}, &root) {
		t.Fatal("expected unconsumed event")
	}
}

func TestDeliver_missing_target_safe(t *testing.T) {
	root := facet.NewFacet()
	if Deliver(RoutedEvent{Target: 999, Event: PointerPressEvent{}}, &root) {
		t.Fatal("expected false")
	}
}

func TestProcessKey_no_focus_returns_nothing(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	if got := sys.processKey(platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyA}); len(got) != 0 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestProcessKey_routes_to_focused_facet(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	sys.focus.SetFocused(child.ID())
	sys.focusTree = &root
	got := sys.processKey(platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyA})
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != child.ID() {
		t.Fatalf("target = %d", got[0].Target)
	}
	if _, ok := got[0].Event.(KeyInputEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
}

func TestProcessText_routes_to_focused_facet(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	sys.focus.SetFocused(child.ID())
	got := sys.processText(platform.EventText{Text: "hello"})
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != child.ID() {
		t.Fatalf("target = %d", got[0].Target)
	}
	if _, ok := got[0].Event.(TextInputEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
}

func TestFocusFollowsClick_sets_focus_on_press(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	focusable := facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0}
	child.AddRole(&focusable)
	sys.focusTree = &root
	sys.focus.Clear()
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50), MarkID: 99})
	got := sys.processPointer(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}, hitMap)
	if sys.focus.Focused() != child.ID() {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
	if len(got) < 2 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestFocusFollowsClick_non_focusable_skipped(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	child.AddRole(&facet.FocusRole{Focusable: func() bool { return false }, TabIndex: 0})
	sys.focusTree = &root
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50), MarkID: 99})
	_ = sys.processPointer(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}, hitMap)
	if sys.focus.Focused() != 0 {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
}

func TestFocusFollowsClick_walks_ancestors(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	parent := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&parent)
	parent.AddChild(&child)
	parent.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0})
	child.AddRole(&facet.FocusRole{Focusable: func() bool { return false }, TabIndex: 1})
	sys.focusTree = &root
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 50, 50), MarkID: 99})
	_ = sys.processPointer(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}, hitMap)
	if sys.focus.Focused() != parent.ID() {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
}

func TestTabNavigation_moves_focus(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	a := facet.NewFacet()
	b := facet.NewFacet()
	root.AddChild(&a)
	root.AddChild(&b)
	a.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0})
	b.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 1})
	sys.focusTree = &root
	sys.focus.SetFocused(a.ID())
	got := sys.handleTabNavigation(platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab}, &root)
	if sys.focus.Focused() != b.ID() {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestTabNavigation_shift_tab_moves_backward(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	a := facet.NewFacet()
	b := facet.NewFacet()
	root.AddChild(&a)
	root.AddChild(&b)
	a.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0})
	b.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 1})
	sys.focusTree = &root
	sys.focus.SetFocused(b.ID())
	_ = sys.handleTabNavigation(platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab, Modifiers: platform.ModShift}, &root)
	if sys.focus.Focused() != a.ID() {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
}

func TestTabNavigation_consumed_keeps_focus(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	a := facet.NewFacet()
	root.AddChild(&a)
	a.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0})
	a.AddRole(&facet.InputRole{OnKey: func(e facet.KeyEvent) bool {
		return e.Key == platform.KeyTab
	}})
	sys.focusTree = &root
	sys.focus.SetFocused(a.ID())
	got := sys.processKey(platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab})
	if sys.focus.Focused() != a.ID() {
		t.Fatalf("focus = %d", sys.focus.Focused())
	}
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestScrollRouting_routes_to_hit_facet(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	sys.focusTree = &root
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 77})
	got := sys.processScroll(platform.EventScroll{Position: gfx.Point{X: 1, Y: 1}, DeltaY: 3}, hitMap)
	if len(got) != 1 || got[0].Target != child.ID() {
		t.Fatalf("got %#v", got)
	}
	if _, ok := got[0].Event.(ScrollEvent); !ok {
		t.Fatalf("event = %T", got[0].Event)
	}
}

func TestScrollRouting_captured_goes_to_capture(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	ptr := sys.getOrCreatePointer(0)
	ptr.PressTarget = &CaptureTarget{FacetID: 99, MarkID: 7}
	ptr.DragActive = true
	hitMap := newHitMapFor(1, gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 77})
	got := sys.processScroll(platform.EventScroll{Position: gfx.Point{X: 1, Y: 1}, DeltaY: 3}, hitMap)
	if len(got) != 1 || got[0].Target != 99 {
		t.Fatalf("got %#v", got)
	}
}

func TestScrollRouting_scales_delta(t *testing.T) {
	sys := NewSystem(GestureConfig{ScrollMultiplier: 2, DragThreshold: 4, DoubleClickInterval: 400 * time.Millisecond, DoubleClickRadius: 8, HoverDelay: 500 * time.Millisecond})
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 77})
	got := sys.processScroll(platform.EventScroll{Position: gfx.Point{X: 1, Y: 1}, DeltaY: 3}, hitMap)
	ev := got[0].Event.(ScrollEvent)
	if ev.DeltaY != 6 {
		t.Fatalf("deltaY = %v", ev.DeltaY)
	}
}

func TestProcess_dispatches_all_event_types(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	root := facet.NewFacet()
	child := facet.NewFacet()
	root.AddChild(&child)
	child.AddRole(&facet.FocusRole{Focusable: func() bool { return true }, TabIndex: 0})
	child.AddRole(&facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool { return true },
		OnKey:     func(e facet.KeyEvent) bool { return false },
		OnText:    func(e facet.TextEvent) bool { return false },
		OnScroll:  func(e facet.ScrollEvent) bool { return false },
	})
	sys.focusTree = &root
	hitMap := newHitMapFor(child.ID(), gfx.Identity(), projection.HitRegion{Bounds: gfx.RectFromXYWH(0, 0, 10, 10), MarkID: 1})
	got := sys.Process([]platform.Event{
		platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft},
		platform.EventScroll{Position: gfx.Point{X: 1, Y: 1}, DeltaY: 3},
		platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyA},
		platform.EventText{Text: "x"},
	}, hitMap, &root)
	if len(got) == 0 {
		t.Fatal("expected routed events")
	}
}

func TestTickHover_returns_hover_after_delay(t *testing.T) {
	sys := NewSystem(DefaultGestureConfig())
	sys.hover.OnMove(11, 22, time.Unix(0, 0))
	got := sys.TickHover(time.Unix(0, 0).Add(sys.config.HoverDelay))
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if got[0].Target != 11 {
		t.Fatalf("target = %d", got[0].Target)
	}
}
