package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
)

// Children order in RootFacet (root.go newRootFacet):
//
//	[0] chromeColumn (ribbon + chromeRow)
//	[1] sourcesPanel
//	[2] centerPanel
//	[3] inspectorPanel
//	[4] statusBar
//	[5-10] overlays (dialog, exportToast, tooltip, commandPalette, popupPalette, navDrawer)
const (
	idxSources   = 1
	idxCenter    = 2
	idxInspector = 3
)

func TestModeFor_wide(t *testing.T) {
	if got := ModeFor(gfx.Size{W: 960, H: 800}); got != state.LayoutWide {
		t.Fatalf("at breakpoint: expected wide, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: 1280, H: 800}); got != state.LayoutWide {
		t.Fatalf("wide: expected wide, got %q", got)
	}
}

func TestModeFor_narrow(t *testing.T) {
	if got := ModeFor(gfx.Size{W: 959, H: 800}); got != state.LayoutNarrow {
		t.Fatalf("just below: expected narrow, got %q", got)
	}
	if got := ModeFor(gfx.Size{W: 480, H: 800}); got != state.LayoutNarrow {
		t.Fatalf("phone: expected narrow, got %q", got)
	}
}

func TestNarrowMode_sourcesHidden(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutNarrow)

	mc := testMeasureContext()
	c := facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}}
	root.Base().LayoutRole().OnMeasure(mc, c)

	ac := facet.ArrangeContext{}
	bounds := gfx.Rect{Max: gfx.Point{X: 480, Y: 800}}
	root.Base().LayoutRole().OnArrange(ac, bounds)

	children := root.Base().Children()
	spBounds := children[idxSources].LayoutRole().ArrangedBounds
	if spBounds.Width() != 0 || spBounds.Height() != 0 {
		t.Fatalf("sources should be hidden (zero rect) in narrow mode, got %v", spBounds)
	}
}

func TestNarrowMode_inspectorHidden(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutNarrow)

	mc := testMeasureContext()
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})

	ac := facet.ArrangeContext{}
	root.Base().LayoutRole().OnArrange(ac, gfx.Rect{Max: gfx.Point{X: 480, Y: 800}})

	children := root.Base().Children()
	ipBounds := children[idxInspector].LayoutRole().ArrangedBounds
	if ipBounds.Width() != 0 || ipBounds.Height() != 0 {
		t.Fatalf("inspector should be hidden (zero rect) in narrow mode, got %v", ipBounds)
	}
}

func TestNarrowMode_centerTakesFullWidth(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutNarrow)

	mc := testMeasureContext()
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})

	ac := facet.ArrangeContext{}
	bounds := gfx.Rect{Max: gfx.Point{X: 480, Y: 800}}
	root.Base().LayoutRole().OnArrange(ac, bounds)

	children := root.Base().Children()
	cpBounds := children[idxCenter].LayoutRole().ArrangedBounds
	if cpBounds.Width() != 480 {
		t.Fatalf("center should take full width in narrow mode, got %f", cpBounds.Width())
	}
}

func TestHamburger_togglesNavDrawer(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)

	if root.overlays.navDrawer.Open.Get() != false {
		t.Fatal("nav drawer should start closed")
	}

	root.hamburger.Activated.Emit(signal.Unit{})
	if root.overlays.navDrawer.Open.Get() != true {
		t.Fatal("nav drawer should open after hamburger activate")
	}
}

func TestWideMode_sourcesVisible(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutWide)

	mc := testMeasureContext()
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})

	ac := facet.ArrangeContext{}
	root.Base().LayoutRole().OnArrange(ac, gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}})

	children := root.Base().Children()
	spBounds := children[idxSources].LayoutRole().ArrangedBounds
	if spBounds.Width() < 200 {
		t.Fatalf("sources should be visible in wide mode, got width %f", spBounds.Width())
	}
}

func TestPanelIdentityPreservedAcrossModes(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)

	spID := root.sourcesPanel.col.Base().ID()
	cpID := root.centerPanel.col.Base().ID()
	ipID := root.inspectorPanel.col.Base().ID()

	as.LayoutMode.Set(state.LayoutNarrow)
	as.LayoutMode.Set(state.LayoutWide)

	if root.sourcesPanel.col.Base().ID() != spID {
		t.Fatal("sources panel identity changed after mode switch")
	}
	if root.centerPanel.col.Base().ID() != cpID {
		t.Fatal("center panel identity changed after mode switch")
	}
	if root.inspectorPanel.col.Base().ID() != ipID {
		t.Fatal("inspector panel identity changed after mode switch")
	}
}

var _ = facet.ArrangeContext{}
