package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
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

// TestE3_triggerInertWhenInactive is a regression test for the dormant-exhibit
// bug: when the stage hides E3 (arranged to empty bounds), the runtime's free
// layer still re-positions the trigger from its layer attachment — with empty
// parent bounds + the seed pos that landed it inside the active exhibit's chart
// area, stealing clicks. The trigger must have empty bounds, project nothing,
// and not resolve hits while its host exhibit is hidden (F-inactive-layer-child).
func TestE3_triggerInertWhenInactive(t *testing.T) {
	sink := NewDirtySink(10)
	root, h := newShellWithSink(t, 1280, 800, sink)
	stage := root.Stage()
	stage.ActiveExhibit().Set(ExhibitAnchors)
	h.RunFrame()
	h.RunFrame()
	stage.ActiveExhibit().Set(ExhibitRealtime)
	h.RunFrame()
	h.RunFrame()

	e3 := stage.RootFor(ExhibitAnchors).(*Anchors)
	tb := e3.Trigger().Base().LayoutRole().ArrangedBounds
	if !tb.IsEmpty() {
		t.Fatalf("inactive E3 trigger has non-empty bounds: %v", tb)
	}
	if cmds := e3.Trigger().Base().ProjectionRole().Project(facet.ProjectionContext{
		Bounds:       tb,
		ContentScale: 1,
	}); cmds != nil && cmds.Len() > 0 {
		t.Fatal("inactive E3 trigger still projects its fill")
	}
	// A press at the trigger's old seed position must not reach it (the hit map
	// resolves by the gated bounds; the click must not change the exhibit).
	plot := root.Stage().RootFor(ExhibitRealtime).(*Realtime).Canvas().PlotRect()
	if plot.IsEmpty() {
		t.Fatal("E1 chart plot not arranged")
	}
	testkit.DriveClick(h, 268, 188)
	if got := root.Shell().ActiveExhibit.Get(); got != ExhibitRealtime {
		t.Fatalf("click at the dormant trigger position switched the exhibit to %v", got)
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
