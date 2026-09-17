package structure

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// TestScrollRegionHostedMarkHitTestable pins RX-1 F-scroll-content: a mark
// inside a ScrollRegion's children is attached to the facet tree, so the
// runtime projects and hit-tests it at its arranged (scrolled) bounds. A
// button in a scroll region is clickable through real pointer input.
func TestScrollRegionHostedMarkHitTestable(t *testing.T) {
	var activations int32
	btn := action.NewButton(marks.Const("Go"), marks.Const(uiinput.ButtonFilled))
	btn.Activated.Subscribe(func(signal.Unit) {
		atomic.AddInt32(&activations, 1)
	})

	sr := NewScrollRegion("Scroller")
	sr.Direction = marks.Const(ScrollDirectionVertical)
	sr.SetChildren([]ScrollRegionChild{
		{Facet: primitive.NewText(marks.Const("first row")), MarkID: 1},
		{Facet: btn, MarkID: 2},
		{Facet: primitive.NewText(marks.Const("last row")), MarkID: 3},
	})

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 240, 160), sr)
	testkit.Warmup(h)

	// The scroll content is attached as tree children.
	treeChildren := 0
	for _, childBase := range sr.Base().Children() {
		if childBase != nil && childBase.LayoutRole() != nil {
			treeChildren++
		}
	}
	if treeChildren < 3 {
		t.Fatalf("scroll tree children = %d, want >= 3 (content must be attached)", treeChildren)
	}

	// The button is arranged (scrolled) and hit-testable at its center.
	btnBounds := btn.Layout.ArrangedBounds
	if btnBounds.IsEmpty() {
		t.Fatal("scroll-hosted button has no arranged bounds")
	}
	cx := btnBounds.Min.X + btnBounds.Width()/2
	cy := btnBounds.Min.Y + btnBounds.Height()/2
	if got := h.Runtime().HitTest(gfx.Point{X: cx, Y: cy}); got != btn.Base().ID() {
		t.Fatalf("hit=%d want button %d (scroll content not hit-testable)", got, btn.Base().ID())
	}

	// A real click activates the button.
	testkit.DriveClick(h, cx, cy)
	if got := atomic.LoadInt32(&activations); got != 1 {
		t.Fatalf("expected 1 activation after DriveClick, got %d", got)
	}
}
