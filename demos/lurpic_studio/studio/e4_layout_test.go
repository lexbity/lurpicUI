package studio

import (
	"bytes"
	"image"
	"testing"

	"codeburg.org/lexbit/lurpicui/internal/testkit"
)

// newE4Harness builds the E4 exhibit as the root and runs one frame.
func newE4Harness(t *testing.T) (*LayoutPolicies, *testkit.Harness) {
	t.Helper()
	e := NewLayoutPolicies()
	h := testkit.NewStandardHarness(t, 640, 360, e)
	h.RunFrame()
	return e, h
}

// afterSet applies a control-store change then runs the frame that delivers
// the store signal and re-lays the split.
func afterSet(t *testing.T, h *testkit.Harness, fn func()) {
	t.Helper()
	fn()
	h.RunFrame()
}

func TestLayoutPolicies_initialSplit(t *testing.T) {
	e, _ := newE4Harness(t)
	panes := e.Split().Panes()
	if len(panes) != 3 {
		t.Fatalf("initial panes = %d, want 3 (D hidden)", len(panes))
	}
	// A and C fixed at the slider width; B (flex) absorbs the residual.
	if got := paneRect(t, panes[0]).Width(); got != 120 {
		t.Errorf("pane A width = %v, want 120", got)
	}
	if got := paneRect(t, panes[2]).Width(); got != 120 {
		t.Errorf("pane C width = %v, want 120", got)
	}
	wantB := float32(640) - 240 - 2*dividerSize
	if got := paneRect(t, panes[1]).Width(); got != wantB {
		t.Errorf("pane B (flex) width = %v, want residual %v", got, wantB)
	}
}

func TestLayoutPolicies_addRemovePane(t *testing.T) {
	e, h := newE4Harness(t)

	// Add the fourth pane: B absorbs the extra fixed pane's width.
	afterSet(t, h, func() { e.ExtraVisible().Set(true) })
	if panes := e.Split().Panes(); len(panes) != 4 {
		t.Fatalf("panes after add = %d, want 4", len(panes))
	}
	wantB := float32(640) - 360 - 3*dividerSize
	if got := paneRect(t, e.Split().Panes()[1]).Width(); got != wantB {
		t.Errorf("pane B width after add = %v, want %v", got, wantB)
	}

	// Remove it again: B regains the residual.
	afterSet(t, h, func() { e.ExtraVisible().Set(false) })
	if panes := e.Split().Panes(); len(panes) != 3 {
		t.Fatalf("panes after remove = %d, want 3", len(panes))
	}
	if got := paneRect(t, e.Split().Panes()[1]).Width(); got != float32(640)-240-2*dividerSize {
		t.Errorf("pane B width after remove = %v, want %v", got, float32(640)-240-2*dividerSize)
	}
}

func TestLayoutPolicies_flexFixedToggle(t *testing.T) {
	e, h := newE4Harness(t)

	// B switches from flex to fixed: it takes the slider width and the split
	// no longer fills the row (A, B, C all fixed at 120 → total 368).
	afterSet(t, h, func() { e.FlexFixed().Set(false) })
	panes := e.Split().Panes()
	if len(panes) != 3 {
		t.Fatalf("panes = %d, want 3", len(panes))
	}
	if got := paneRect(t, panes[1]).Width(); got != 120 {
		t.Errorf("fixed pane B width = %v, want 120", got)
	}
	if got := paneRect(t, panes[2]).Max.X; got != 120+dividerSize+120+dividerSize+120 {
		t.Errorf("all-fixed row ends at %v, want %v", got, 120+dividerSize+120+dividerSize+120)
	}
}

func TestLayoutPolicies_paneMinSlider(t *testing.T) {
	e, h := newE4Harness(t)

	// Raising the fixed width shrinks the flex pane's residual.
	afterSet(t, h, func() { e.PaneMin().Set(200) })
	panes := e.Split().Panes()
	if got := paneRect(t, panes[0]).Width(); got != 200 {
		t.Errorf("pane A width = %v, want 200", got)
	}
	wantB := float32(640) - 400 - 2*dividerSize
	if got := paneRect(t, panes[1]).Width(); got != wantB {
		t.Errorf("pane B (flex) width = %v, want %v", got, wantB)
	}
}

func surfacesEqual(a, b *image.RGBA) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	return bytes.Equal(a.Pix, b.Pix)
}

// TestLayoutPolicies_flexVsFixedGoldens pins the two variants and proves they
// differ byte-wise (NFR-determinism: variant goldens must discriminate).
func TestLayoutPolicies_flexVsFixedGoldens(t *testing.T) {
	flex, h := newE4Harness(t)
	_ = flex
	testkit.AssertGolden(t, h.Surface(), "e4_flex")

	fixed := NewLayoutPolicies()
	h2 := testkit.NewStandardHarness(t, 640, 360, fixed)
	fixed.FlexFixed().Set(false)
	h2.RunFrame()
	testkit.AssertGolden(t, h2.Surface(), "e4_fixed")

	if surfacesEqual(h.Surface().Capture(), h2.Surface().Capture()) {
		t.Fatal("flex and fixed goldens are identical; the variants do not discriminate")
	}
}
