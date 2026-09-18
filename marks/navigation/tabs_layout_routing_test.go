package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/store"
)

// tickBody is a minimal panel body whose arranged bounds the test reads.
type tickBody struct {
	facet.Facet
	layout facet.LayoutRole
}

func (b *tickBody) Base() *facet.Facet { return &b.Facet }

func newTickBody() *tickBody {
	b := &tickBody{Facet: facet.NewFacet()}
	b.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 200, H: 100}}
	}
	b.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.layout.ArrangedBounds = bounds
	}
	b.AddRole(&b.layout)
	b.BindImpl(b)
	return b
}

// TestTabsStoreBoundLayoutRoutesRuntime pins RX-1 F-dirtylayout-routing: a
// store-bound DirtyLayout (the tabs' ActiveIndex subscription) routes the
// runtime layout pass, so the panel body re-arranges within the frame. Without
// the routing, a generic tabs mark's store change would leave the panel body
// at stale bounds.
func TestTabsStoreBoundLayoutRoutesRuntime(t *testing.T) {
	active := store.NewValueStore(0)
	bodyA := newTickBody()
	bodyB := newTickBody()
	tabs := NewTabs("Tabs", []TabItem{
		{Key: "a", Label: "Alpha", Body: bodyA},
		{Key: "b", Label: "Beta", Body: bodyB},
	}, active)

	h := testkit.NewHarness(t, testkit.StandardHarnessConfig(t, 400, 300), tabs)
	testkit.Warmup(h)

	// Tab A's body is arranged at the panel area.
	beforeA := bodyA.layout.ArrangedBounds
	if beforeA.IsEmpty() {
		t.Fatal("active tab body A not arranged after warmup")
	}

	// Switch to tab B: the tabs' ActiveIndex store handler declares DirtyLayout,
	// which routes the runtime layout pass. The newly active body B is arranged
	// at the panel area.
	active.Set(1)
	h.RunFrame()
	h.RunFrame()

	afterB := bodyB.layout.ArrangedBounds
	if afterB.IsEmpty() {
		t.Fatal("active tab body B not arranged after the store-bound switch (F-dirtylayout-routing)")
	}
}
