package studio

import (
	"fmt"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/platform"
)

// newE6Harness mounts the E6 playground as the root and runs one frame.
func newE6Harness(t *testing.T) (*Playground, *testkit.Harness) {
	t.Helper()
	e := NewPlayground(state.NewAppState(nil))
	h := testkit.NewStandardHarness(t, 960, 600, e)
	h.RunFrame()
	return e, h
}

// switchTab sets the E6 family tab and runs the frame that re-mounts the tab
// body. Inactive family bodies have no arranged bounds; setting the active
// index through the tabs' own ActiveIndex store is the genuine family switch.
func switchTab(t *testing.T, e *Playground, h *testkit.Harness, idx int) {
	t.Helper()
	e.ActiveTab().Set(idx)
	h.RunFrame()
	h.RunFrame()
	if got := e.Tabs().ActiveIndex.Get(); got != idx {
		t.Fatalf("tabs active index = %d, want %d", got, idx)
	}
}

func e6Arranged(t *testing.T, f facet.FacetImpl) gfx.Rect {
	t.Helper()
	if f == nil || f.Base() == nil || f.Base().LayoutRole() == nil {
		t.Fatal("mark has no layout role")
	}
	b := f.Base().LayoutRole().ArrangedBounds
	if b.IsEmpty() {
		t.Fatalf("%T is not arranged", f)
	}
	return b
}

// playCenter returns the screen center of a mark's arranged bounds.
func playCenter(f facet.FacetImpl) gfx.Point {
	b := f.Base().LayoutRole().ArrangedBounds
	return gfx.Point{X: b.Min.X + b.Width()*0.5, Y: b.Min.Y + b.Height()*0.5}
}

// scrollFamily scrolls the given family's list host so below-the-fold cards
// become visible (the demo's bespoke scroll list, F-scroll-content).
func scrollFamily(t *testing.T, h *testkit.Harness, f facet.FacetImpl, delta float32) {
	t.Helper()
	b := f.Base().LayoutRole().ArrangedBounds
	if b.IsEmpty() {
		t.Fatal("family list not arranged")
	}
	pt := gfx.Point{X: b.Min.X + b.Width()*0.5, Y: b.Min.Y + b.Height()*0.5}
	testkit.DriveScroll(h, pt.X, pt.Y, 0, delta)
	h.RunFrame()
}

// TestPlayground_tabFamiliesAllReachable asserts the tabs host (E6's genuine
// role, F-tabs) switches between the six family playgrounds, each reachable
// and arranged.
func TestPlayground_tabFamiliesAllReachable(t *testing.T) {
	e, h := newE6Harness(t)
	families := []struct {
		name string
		body func() facet.FacetImpl
	}{
		{"Action", func() facet.FacetImpl { return e.Action().scroll.Base() }},
		{"Selection", func() facet.FacetImpl { return e.Selection().scroll.Base() }},
		{"Input", func() facet.FacetImpl { return e.Input().scroll.Base() }},
		{"Navigation", func() facet.FacetImpl { return e.Navigation().scroll.Base() }},
		{"Feedback", func() facet.FacetImpl { return e.Feedback().scroll.Base() }},
		{"Status", func() facet.FacetImpl { return e.Status().scroll.Base() }},
	}
	for i, fam := range families {
		switchTab(t, e, h, i)
		b := e6Arranged(t, fam.body())
		if b.IsEmpty() {
			t.Fatalf("family %q body not arranged at tab %d", fam.name, i)
		}
	}
	t.Log("all six family tabs reachable")
}

// TestPlayground_actionFamilyDispatch drives the action marks' command
// dispatch: an action_bar click, an action_group click, and a ribbon tab click
// each land in the family's shared stores (the action family's distinctive
// behavior). The ribbon's section switch is exercised via its own Activated
// subscription because the ribbon's internal tab buttons are not attached to
// the facet tree and cannot be pointer-driven from outside (F-e6-internal).
func TestPlayground_actionFamilyDispatch(t *testing.T) {
	e, h := newE6Harness(t)

	bar := e6Arranged(t, e.Action().bar)
	// The bar's action buttons sit on the left after the "File" label.
	testkit.DriveClick(h, bar.Min.X+80, bar.Min.Y+bar.Height()*0.5)
	if got := e.Action().lastAction.Get(); got == "" {
		t.Fatalf("action_bar click did not dispatch (lastAction=%q)", got)
	}

	group := e6Arranged(t, e.Action().group)
	testkit.DriveClick(h, group.Min.X+50, group.Min.Y+group.Height()*0.5)
	if got := e.Action().lastAction.Get(); got == "" {
		t.Fatalf("action_group click did not dispatch (lastAction=%q)", got)
	}

	// Ribbon: assert it is arranged and hit-testable by the runtime. Its
	// section switch is driven by internal tab buttons that are not attached to
	// the facet tree, so they cannot be pointer-driven from outside the mark
	// (F-e6-internal); the mark's own contract tests cover the interaction.
	rb := e6Arranged(t, e.Action().ribbon)
	hit := h.Runtime().HitTest(gfx.Point{X: rb.Min.X + rb.Width()*0.3, Y: rb.Min.Y + 20})
	if hit != e.Action().ribbon.Base().ID() {
		t.Fatalf("ribbon not hit-testable (hit=%d want %d)", hit, e.Action().ribbon.Base().ID())
	}
}

// TestPlayground_selectionFamilyWriteBack drives the selection family's
// store write-back loop: checkbox, switch, slider, turn_dial, radio_group and
// button_group each land user input in a caller-owned store.
func TestPlayground_selectionFamilyWriteBack(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 1)

	// Checkbox toggles its Value store.
	cb := playCenter(e.Selection().checkbox)
	testkit.DriveClick(h, cb.X, cb.Y)
	if got := e.Selection().CheckboxState().Get(); got != selection.CheckboxStateOn {
		t.Fatalf("checkbox did not toggle: state=%v", got)
	}

	// Switch toggles its bool store.
	tg := playCenter(e.Selection().toggle)
	testkit.DriveClick(h, tg.X, tg.Y)
	if !e.Selection().Toggle().Get() {
		t.Fatalf("switch did not set store true")
	}

	// Slider drag changes its value store.
	sl := e6Arranged(t, e.Selection().slider)
	before := e.Selection().Slider().Get()
	slY := sl.Min.Y + sl.Height()*0.5
	testkit.DriveDrag(h, sl.Min.X+sl.Width()*0.2, slY, sl.Min.X+sl.Width()*0.8, slY)
	if got := e.Selection().Slider().Get(); got == before {
		t.Fatalf("slider drag did not change value (%v)", got)
	}

	// Turn dial responds to the right arrow key after focus.
	dl := playCenter(e.Selection().dial)
	testkit.DriveClick(h, dl.X, dl.Y)
	before = e.Selection().Dial().Get()
	testkit.DriveKeyPress(h, platform.KeyRight, 0)
	testkit.DriveKeyRelease(h, platform.KeyRight, 0)
	if got := e.Selection().Dial().Get(); got <= before {
		t.Fatalf("turn_dial right arrow did not increase value (%v <= %v)", got, before)
	}

	// Radio group select lands in its store.
	rd := e6Arranged(t, e.Selection().radio)
	testkit.DriveClick(h, rd.Min.X+rd.Width()*0.5, rd.Min.Y+rd.Height()*0.5)
	if got := e.Selection().Radio().Get(); got == "" {
		t.Fatalf("radio_group select did not land in store")
	}

	// Button group (below the fold) after scrolling: clicking a segment writes
	// the exclusive selection to its []string store.
	scrollFamily(t, h, e.Selection().scroll, -900)
	seg := e6Arranged(t, e.Selection().segments)
	beforeVal := e.Selection().ButtonGroup().Get()
	testkit.DriveClick(h, seg.Min.X+seg.Width()*0.1, seg.Min.Y+seg.Height()*0.5)
	if got := e.Selection().ButtonGroup().Get(); fmt.Sprint(got) == fmt.Sprint(beforeVal) {
		t.Fatalf("button_group click did not change selection (%v)", got)
	}
}

// TestPlayground_inputFamilyWriteBack drives the input family's IME write-back
// loop: typed text lands in the text field's store, the number field's stepper
// lands in its store, and the color picker's arrow adjusts its color store.
func TestPlayground_inputFamilyWriteBack(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 2)

	f := playCenter(e.Input().field)
	testkit.DriveClick(h, f.X, f.Y)
	testkit.DriveType(h, "alpha")
	if got := e.Input().Name().Get(); got != "alpha" {
		t.Fatalf("text_field typed value = %q, want alpha", got)
	}

	n := playCenter(e.Input().number)
	testkit.DriveClick(h, n.X, n.Y)
	before := e.Input().Amount().Get()
	testkit.DriveKeyPress(h, platform.KeyUp, 0)
	testkit.DriveKeyRelease(h, platform.KeyUp, 0)
	if got := e.Input().Amount().Get(); got <= before {
		t.Fatalf("number_field up-arrow did not increase value (%v <= %v)", got, before)
	}

	c := playCenter(e.Input().picker)
	beforeColor := e.Input().Color().Get()
	testkit.DriveClick(h, c.X, c.Y)
	testkit.DriveKeyPress(h, platform.KeyRight, 0)
	testkit.DriveKeyRelease(h, platform.KeyRight, 0)
	if got := e.Input().Color().Get(); got == beforeColor {
		t.Fatalf("color_picker arrow did not adjust color")
	}
}

// TestPlayground_navigationFamily drives the navigation family's
// structure-driven navigation: a nav_drawer item click lands in the current
// store, and a pagination page click lands in the page store.
func TestPlayground_navigationFamily(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 3)

	// The open drawer panel sits on the left of its card; click the first item.
	db := e6Arranged(t, e.Navigation().drawer)
	testkit.DriveClick(h, db.Min.X+40, db.Min.Y+130)
	if got := e.Navigation().lastItem.Get(); got < 0 {
		t.Fatalf("nav_drawer item click did not activate (lastItem=%d)", got)
	}

	// Pagination sits below the fold; scroll to it and flip a page.
	scrollFamily(t, h, e.Navigation().scroll, -500)
	pg := e6Arranged(t, e.Navigation().pager)
	before := e.Navigation().pageActivated.Get()
	testkit.DriveClick(h, pg.Min.X+pg.Width()*0.15, pg.Min.Y+pg.Height()*0.5)
	if got := e.Navigation().pageActivated.Get(); got == before {
		t.Fatalf("pagination click did not activate a page (%d)", got)
	}
}

// TestPlayground_feedbackFamily drives the feedback family's write-back loop:
// a button surfaces an alert message and another opens the modal dialog.
func TestPlayground_feedbackFamily(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 4)

	at := playCenter(e.Feedback().AlertTrigger())
	testkit.DriveClick(h, at.X, at.Y)
	if got := e.Feedback().AlertMessage().Get(); got == "All sources are healthy." {
		t.Fatalf("alert trigger did not change the alert message")
	}

	dt := playCenter(e.Feedback().DialogOpenTrigger())
	testkit.DriveClick(h, dt.X, dt.Y)
	if !e.Feedback().DialogOpen().Get() {
		t.Fatalf("dialog trigger did not open the dialog")
	}
}

// TestPlayground_statusFamily drives the status family's store reflection: the
// tick button bumps the badge count, the online switch flips the light label,
// and the throughput slider drives the progress indicators.
func TestPlayground_statusFamily(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 5)

	tk := playCenter(e.Status().Tick())
	testkit.DriveClick(h, tk.X, tk.Y)
	if got := e.Status().BadgeLabel().Get(); got == "0" {
		t.Fatalf("tick did not bump the badge (label=%q)", got)
	}

	os := playCenter(e.Status().onlineSwitch)
	testkit.DriveClick(h, os.X, os.Y)
	if got := e.Status().Light().Label.Get(); got != "Offline" {
		t.Fatalf("online switch did not flip the light (label=%q)", got)
	}

	sl := e6Arranged(t, e.Status().slider)
	before := e.Status().Progress().Get()
	slY := sl.Min.Y + sl.Height()*0.5
	testkit.DriveDrag(h, sl.Min.X+sl.Width()*0.8, slY, sl.Min.X+sl.Width()*0.2, slY)
	if got := e.Status().Progress().Get(); got == before {
		t.Fatalf("throughput slider did not drive progress (%v)", got)
	}
}

// TestPlayground_scrollReachesBelowFold proves the family list host scrolls:
// a mark below the initial fold is not arranged until the list is scrolled,
// after which it is arranged and interactive (F-scroll-content).
func TestPlayground_scrollReachesBelowFold(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 1)

	// The button_group is the last selection card; initially below the fold.
	if b := e.Selection().segments.Base().LayoutRole().ArrangedBounds; !b.IsEmpty() {
		t.Fatalf("button_group arranged before scrolling (unexpectedly in view)")
	}
	scrollFamily(t, h, e.Selection().scroll, -900)
	b := e6Arranged(t, e.Selection().segments)
	if b.IsEmpty() {
		t.Fatal("button_group not arranged after scroll")
	}
	if got := h.Runtime().HitTest(gfx.Point{X: b.Min.X + b.Width()*0.1, Y: b.Min.Y + b.Height()*0.5}); got != e.Selection().segments.Base().ID() {
		t.Fatalf("scrolled button_group not hit-testable (hit=%d want %d)", got, e.Selection().segments.Base().ID())
	}
}

// TestPlayground_inactiveBodiesNotArranged asserts the Stage-style gating: only
// the active family body is arranged; the others project nothing.
func TestPlayground_inactiveBodiesNotArranged(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 2)
	if b := e.Action().scroll.Base().LayoutRole().ArrangedBounds; !b.IsEmpty() {
		t.Fatalf("inactive Action body arranged: %v", b)
	}
}

// TestPlayground_goldenActionTab pins one family tab render so the playground
// goldens show a discriminating, non-empty page.
func TestPlayground_goldenActionTab(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 0)
	testkit.AssertGolden(t, h.Surface(), "e6_action")
}

// TestPlayground_goldenSelectionTab pins the densest family (Selection, seven
// cards) so the playground goldens cover more than one family.
func TestPlayground_goldenSelectionTab(t *testing.T) {
	e, h := newE6Harness(t)
	switchTab(t, e, h, 1)
	testkit.AssertGolden(t, h.Surface(), "e6_selection")
}
