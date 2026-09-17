package structure

import (
	"sync/atomic"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

// TestCardHostedMarkHitTestable pins RX-1 F-card-content: a mark inside a
// structure.Card's ChildrenContent is attached to the facet tree (OnAttach
// AddChild), so the runtime projects and hit-tests it. A button in a card is
// clickable through real pointer input.
func TestCardHostedMarkHitTestable(t *testing.T) {
	var activations int32
	btn := action.NewButton(marks.Const("Do it"), marks.Const(uiinput.ButtonFilled))
	btn.Activated.Subscribe(func(signal.Unit) {
		atomic.AddInt32(&activations, 1)
	})

	card := NewCard("Host card")
	card.GridColumns = marks.Const(1)
	card.GridRows = marks.Const(1)
	card.ChildrenContent = []CardChild{
		{Key: "btn", Facet: btn, Grid: facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1}},
		{Key: "label", Facet: primitive.NewText(marks.Const("content")), Grid: facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1}},
	}

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 400, 200), card)
	testkit.Warmup(h)

	// The card attached its content as tree children.
	treeChildren := 0
	for _, childBase := range card.Base().Children() {
		if childBase != nil && childBase.LayoutRole() != nil {
			treeChildren++
		}
	}
	if treeChildren < 2 {
		t.Fatalf("card tree children = %d, want >= 2 (content must be attached)", treeChildren)
	}

	// The button is arranged and hit-testable at its center.
	btnBounds := btn.Layout.ArrangedBounds
	if btnBounds.IsEmpty() {
		t.Fatal("card-hosted button has no arranged bounds")
	}
	cx := btnBounds.Min.X + btnBounds.Width()/2
	cy := btnBounds.Min.Y + btnBounds.Height()/2
	if got := h.Runtime().HitTest(gfx.Point{X: cx, Y: cy}); got != btn.Base().ID() {
		t.Fatalf("hit=%d want button %d (card content not hit-testable)", got, btn.Base().ID())
	}

	// A real click toggles the button.
	testkit.DriveClick(h, cx, cy)
	if got := atomic.LoadInt32(&activations); got != 1 {
		t.Fatalf("expected 1 activation after DriveClick, got %d", got)
	}
	_ = theme.DefaultResolvedContext()
	_ = templates.Notes()
}
