package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/platform"
)

// assertAnchoredOverlay verifies the E3 geometry contract: popovers sit below
// the trigger's bottom-center anchor on the anchored layer (AnchorSide=Below)
// at the configured gap, with their x-center on the trigger.
func assertAnchoredOverlay(t *testing.T, step string, trigger, pop0 gfx.Rect, gap float32) {
	t.Helper()
	if trigger.IsEmpty() {
		t.Fatalf("[%s] trigger is empty %v", step, trigger)
	}
	if pop0.IsEmpty() {
		t.Fatalf("[%s] popover is empty %v", step, pop0)
	}
	wantTop := trigger.Max.Y + gap
	if pop0.Min.Y != wantTop {
		t.Fatalf("[%s] popover below-gap mismatch: pop0.Min.Y=%v want %v (trigger max.Y=%v)", step, pop0.Min.Y, wantTop, trigger.Max.Y)
	}
	triggerCenter := trigger.Min.X + trigger.Width()*0.5
	popCenter := pop0.Min.X + pop0.Width()*0.5
	if popCenter != triggerCenter {
		t.Fatalf("[%s] popover x-center %v != trigger x-center %v (trigger=%v pop=%v)", step, popCenter, triggerCenter, trigger, pop0)
	}
}

// TestE3_anchorTracking moves the trigger store and checks that the anchored
// popovers re-resolve to the new anchor each frame (P8 anchored-overlay
// re-export) while keeping the E3 geometry contract.
func TestE3_anchorTracking(t *testing.T) {
	themeCtx := StudioThemeContext()
	reg, _ := StudioLayerRegistry()
	ids := studioLayersFrom(reg)
	e3 := NewAnchorsFacet(testkit.TestFontRegistry(t), themeCtx, ids)
	h := newStudioHarness(t, 800, 600, e3)
	h.RunFrame()
	h.RunFrame()

	b0 := e3.Popovers()[0].Base().LayoutRole().ArrangedBounds
	t0 := e3.Trigger().Base().LayoutRole().ArrangedBounds

	e3.pos.Set(gfx.Point{X: 400, Y: 300})
	h.RunFrame()
	h.RunFrame()

	b1 := e3.Popovers()[0].Base().LayoutRole().ArrangedBounds
	t1 := e3.Trigger().Base().LayoutRole().ArrangedBounds

	const gap = float32(8)
	assertAnchoredOverlay(t, "initial", t0, b0, gap)
	assertAnchoredOverlay(t, "after move", t1, b1, gap)
	if t0 == t1 {
		t.Fatalf("trigger bounds did not move: before=%v after=%v", t0, t1)
	}
	if b0 == b1 {
		t.Fatalf("popover bounds did not track the trigger: before=%v after=%v", b0, b1)
	}
}

// TestE3Drag_draggingTriggerMovesPopovers drives the trigger through real
// pointer input and checks that the anchored popovers track it: the framework
// re-exports the trigger's anchors from the free layer each frame and the
// anchor policy re-arranges the popovers (P8 anchored-overlay tracking).
func TestE3Drag_draggingTriggerMovesPopovers(t *testing.T) {
	themeCtx := StudioThemeContext()
	reg, _ := StudioLayerRegistry()
	ids := studioLayersFrom(reg)
	e3 := NewAnchorsFacet(testkit.TestFontRegistry(t), themeCtx, ids)
	h := newStudioHarness(t, 800, 600, e3)
	h.RunFrame()
	h.RunFrame()

	before := e3.Popovers()[0].Base().LayoutRole().ArrangedBounds
	triggerBounds := e3.Trigger().Base().LayoutRole().ArrangedBounds
	drag := gfx.Point{X: 120, Y: 80}
	start := gfx.Point{X: triggerBounds.Min.X + triggerBounds.Width()*0.5, Y: triggerBounds.Min.Y + triggerBounds.Height()*0.5}
	end := gfx.Point{X: start.X + drag.X, Y: start.Y + drag.Y}

	h.InjectEvent(platform.EventPointer{Kind: platform.PointerPress, Position: start, Button: platform.PointerLeft})
	h.RunFrame()
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerMove, Position: end})
	h.RunFrame()
	h.InjectEvent(platform.EventPointer{Kind: platform.PointerRelease, Position: end, Button: platform.PointerLeft})
	h.RunFrame()
	h.RunFrame()

	after := e3.Popovers()[0].Base().LayoutRole().ArrangedBounds
	want := gfx.Point{X: before.Min.X + drag.X, Y: before.Min.Y + drag.Y}
	if after.Min != want {
		t.Fatalf("popover did not follow drag: before.Min=%v after.Min=%v want %v", before.Min, after.Min, want)
	}
}
