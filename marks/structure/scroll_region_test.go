package structure

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

func TestScrollRegionGoldenVerticalOverflow(t *testing.T) {
	AssertScrollRegionGolden(t, "vertical_overflow", ScrollDirectionVertical, func(sr *ScrollRegion) {
		sr.SetChildren(scrollRegionVerticalChildren())
	}, func(sr *ScrollRegion) {
		if sr.cachedVerticalTrack.IsEmpty() || sr.cachedVerticalThumb.IsEmpty() {
			t.Fatalf("expected vertical scrollbar geometry, got track=%#v thumb=%#v", sr.cachedVerticalTrack, sr.cachedVerticalThumb)
		}
		if !sr.cachedHorizontalTrack.IsEmpty() || !sr.cachedHorizontalThumb.IsEmpty() {
			t.Fatalf("did not expect horizontal scrollbar geometry, got track=%#v thumb=%#v", sr.cachedHorizontalTrack, sr.cachedHorizontalThumb)
		}
	})
}

func TestScrollRegionGoldenHorizontalOverflow(t *testing.T) {
	AssertScrollRegionGolden(t, "horizontal_overflow", ScrollDirectionHorizontal, func(sr *ScrollRegion) {
		sr.SetChildren(scrollRegionHorizontalChildren())
	}, func(sr *ScrollRegion) {
		if sr.cachedHorizontalTrack.IsEmpty() || sr.cachedHorizontalThumb.IsEmpty() {
			t.Fatalf("expected horizontal scrollbar geometry, got track=%#v thumb=%#v", sr.cachedHorizontalTrack, sr.cachedHorizontalThumb)
		}
		if !sr.cachedVerticalTrack.IsEmpty() || !sr.cachedVerticalThumb.IsEmpty() {
			t.Fatalf("did not expect vertical scrollbar geometry, got track=%#v thumb=%#v", sr.cachedVerticalTrack, sr.cachedVerticalThumb)
		}
	})
}

func AssertScrollRegionGolden(t *testing.T, name string, direction ScrollDirection, mutate func(*ScrollRegion), assert func(*ScrollRegion)) {
	t.Helper()
	sr := NewScrollRegion("Scrollable region")
	sr.Direction = marks.Const(direction)
	sr.Gap = marks.Const[float32](10)
	if mutate != nil {
		mutate(sr)
	}
	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sr, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	_ = sr.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	sr.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: sr.Layout.Parent,
		ChildGroup:  sr.Layout.Child,
	}, canvas)
	if assert != nil {
		assert(sr)
	}
	cmds := sr.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       canvas,
		ContentScale: 1,
	})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	// The scroll content is a real tree child (RX-1 F-scroll-content): project
	// the region chrome first, then each tree child at its arranged (scrolled)
	// bounds, in the runtime's pre-order paint order.
	merged := projectTreeCommands(sr, rt)
	surface := testkit.NewMemorySurface(272, 192)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      canvas,
			Opacity:     1,
			Commands:    merged,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "scroll_region_"+name)
}

func scrollRegionVerticalChildren() []ScrollRegionChild {
	out := make([]ScrollRegionChild, 0, 12)
	for i := 0; i < 12; i++ {
		out = append(out, ScrollRegionChild{
			Facet:     primitive.NewText(marks.Const("Row " + string(rune('A'+i%26)))),
			MarkID:    facet.MarkID(i + 10),
			Placement: facet.Placement{Mode: facet.PlacementFree},
		})
	}
	return out
}

func scrollRegionHorizontalChildren() []ScrollRegionChild {
	return []ScrollRegionChild{
		{
			Facet:     primitive.NewText(marks.Const(strings.Repeat("Wide content segment ", 8))),
			MarkID:    facet.MarkID(10),
			Placement: facet.Placement{Mode: facet.PlacementFree},
		},
	}
}

func TestScrollRegionViewportRoleRegisteredWithIdentityTransform(t *testing.T) {
	sr := NewScrollRegion("Scrollable region")
	role := sr.ViewportRole()
	if role == nil {
		t.Fatal("expected ViewportRole to be registered")
	}
	if role.Transform != gfx.Identity() {
		t.Fatalf("ViewportRole transform = %#v, want identity; a zero transform degenerates every composed layer matrix and rasterizes to nothing", role.Transform)
	}
}

// --- RX-2 P2: interactive content hosting -----------------------------------
//
// Post-27146ec the scroll region attaches its content as real tree children
// (the Card pattern); these tests prove the interactive-content contract the
// P2 migration depends on: a click on a child inside the region reaches the
// child's mark, keyboard scrolling clamps, and focus can move into children.

// newScrollHarness mounts a scroll region hosting the given children as the
// harness root and warms it for interaction.
func newScrollHarness(t *testing.T, children []ScrollRegionChild) (*ScrollRegion, *testkit.Harness) {
	t.Helper()
	sr := NewScrollRegion("Scrollable region")
	sr.Gap = marks.Const[float32](8)
	sr.SetChildren(children)
	h := testkit.NewStandardHarness(t, 320, 160, sr)
	testkit.Warmup(h)
	return sr, h
}

func scrollChildButton(t *testing.T, label string) (*action.Button, ScrollRegionChild) {
	t.Helper()
	btn := action.NewButton(marks.Const(label), marks.Const(uiinput.ButtonFilled))
	return btn, ScrollRegionChild{
		Facet:  btn,
		MarkID: facet.MarkID(10),
	}
}

// TestScrollRegion_hostsInteractiveChildClick proves a click on a child mark
// inside the scroll region fires the child's Activated (RX-2 P2 / AC-2: the
// region hosts its content as real tree children, so interactive marks inside
// receive input).
func TestScrollRegion_hostsInteractiveChildClick(t *testing.T) {
	btn, child := scrollChildButton(t, "Tap me")
	_, h := newScrollHarness(t, []ScrollRegionChild{child})

	fired := false
	id := btn.Activated.Subscribe(func(signal.Unit) { fired = true })
	defer btn.Activated.Unsubscribe(id)

	b := btn.Base().LayoutRole().ArrangedBounds
	if b.IsEmpty() {
		t.Fatal("child button was not arranged inside the scroll region")
	}
	testkit.DriveClick(h, b.Min.X+b.Width()*0.5, b.Min.Y+b.Height()*0.5)
	h.RunFrame()
	if !fired {
		t.Fatal("click inside the scroll region did not reach the child button (interactive content not hosted)")
	}
}

// TestScrollRegion_keyboardPageKeysClamp proves PageUp/PageDown/Home/End
// scroll the content and clamp to the content extent (RX-2 P2: the demoList
// PageUp/PageDown semantics migrated into the mark).
func TestScrollRegion_keyboardPageKeysClamp(t *testing.T) {
	children := scrollRegionVerticalChildren() // 12 text rows: content exceeds the viewport
	sr, h := newScrollHarness(t, children)

	sr.Focus.Focusable = func() bool { return true }
	h.Runtime().SetFocus(sr)
	h.RunFrame()

	// PageDown scrolls by ~80% of the viewport span and clamps at the end.
	testkit.DriveKeyPress(h, platform.KeyPageDown, 0)
	h.RunFrame()
	testkit.DriveKeyPress(h, platform.KeyPageDown, 0)
	h.RunFrame()
	max := sr.maxScrollY()
	if sr.scrollOffset.Y <= 0 {
		t.Fatalf("PageDown did not advance the offset, got %v", sr.scrollOffset.Y)
	}
	// Drive enough PageDowns to bottom out: the offset must never exceed max.
	for i := 0; i < 12; i++ {
		testkit.DriveKeyPress(h, platform.KeyPageDown, 0)
		h.RunFrame()
	}
	if sr.scrollOffset.Y > max {
		t.Fatalf("PageDown overscrolled: offset %v > max %v", sr.scrollOffset.Y, max)
	}
	// End jumps to the content end; Home returns to the origin.
	testkit.DriveKeyPress(h, platform.KeyEnd, 0)
	h.RunFrame()
	if sr.scrollOffset.Y != max {
		t.Fatalf("End offset = %v, want the max %v", sr.scrollOffset.Y, max)
	}
	testkit.DriveKeyPress(h, platform.KeyHome, 0)
	h.RunFrame()
	if sr.scrollOffset != (gfx.Point{}) {
		t.Fatalf("Home offset = %v, want the origin", sr.scrollOffset)
	}
}

// TestScrollRegion_focusTraversalReachesChildren proves a scroll region hosting
// focusable children does not swallow Tab traversal: focus moves into the
// child marks (RX-2 P2: the E6 family bodies' marks stay keyboard-reachable).
func TestScrollRegion_focusTraversalReachesChildren(t *testing.T) {
	btn, child := scrollChildButton(t, "First")
	btn2, child2 := scrollChildButton(t, "Second")
	_, h := newScrollHarness(t, []ScrollRegionChild{child, child2})

	h.Runtime().SetFocus(btn)
	h.RunFrame()
	testkit.DriveKeyPress(h, platform.KeyTab, 0)
	h.RunFrame()
	// Tab from the first child moves focus forward to the second child; the
	// region must not swallow the traversal.
	if got := h.Runtime().FocusedID(); got != btn2.Base().ID() {
		t.Fatalf("Tab traversal did not move focus from the first to the second child: focused=%v want child 2", got)
	}
}
