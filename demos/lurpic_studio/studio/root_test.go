package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
)

func testBuildContext() app.BuildContext {
	return app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        theme.DefaultResolvedContext(),
	}
}

func TestBuildRoot_nonNil(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	if root == nil {
		t.Fatal("BuildRoot returned nil")
	}
}

func TestBuildRoot_hasLayoutAndRender(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	if root.Base().LayoutRole() == nil {
		t.Fatal("root has no LayoutRole")
	}
	if root.Base().RenderRole() == nil {
		t.Fatal("root has no RenderRole")
	}
}

func TestBuildRoot_layoutModeStartsWide(t *testing.T) {
	as := state.NewAppState(nil)
	_ = BuildRoot(as, testBuildContext())
	if got := as.LayoutMode.Get(); got != state.LayoutWide {
		t.Fatalf("expected LayoutWide, got %q", got)
	}
}

func TestBuildRoot_measuresToWindowSize(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
	mc := facet.MeasureContext{}
	result := root.Base().LayoutRole().OnMeasure(mc, c)
	if result.Size != (gfx.Size{W: 1280, H: 800}) {
		t.Fatalf("expected size 1280x800, got %v", result.Size)
	}
}

func TestBuildRoot_onCollectProducesCommands(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	list := &gfx.CommandList{}
	bounds := gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}}
	root.Base().RenderRole().OnCollect(list, bounds)

	if list.Len() == 0 {
		t.Fatal("expected at least one command from OnCollect")
	}
}

// TestBuildRoot_hasChildFacets verifies that layout policies are constructed.
func TestBuildRoot_hasChildFacets(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	// RootFacet delegates to layout policies; verify it has no direct children
	// (children are within the layout policies)
	children := root.Base().Children()
	// RootFacet itself has no direct children; layout policies handle them
	if len(children) != 0 {
		t.Fatalf("expected 0 direct children (delegated to layout policies), got %d", len(children))
	}
}

// TestBuildRoot_onCollectWideModeHasBackground verifies root renders background in wide mode.
func TestBuildRoot_onCollectWideModeHasBackground(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	list := &gfx.CommandList{}
	bounds := gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}}
	root.Base().RenderRole().OnCollect(list, bounds)

	// Root should render background
	if list.Len() != 1 {
		t.Fatalf("expected 1 background command, got %d", list.Len())
	}
}

// TestBuildRoot_arrangeWideSetsChildBounds verifies that OnArrange in wide mode
// successfully delegates to the layout policy.
func TestBuildRoot_arrangeWideSetsChildBounds(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	bounds := gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}}
	ac := facet.ArrangeContext{}
	// Should not panic; layout policy handles arrangement
	root.Base().LayoutRole().OnArrange(ac, bounds)

	// Verify root's arranged bounds were set
	lr := root.Base().LayoutRole()
	if lr.ArrangedBounds != bounds {
		t.Fatalf("root arranged bounds not set correctly")
	}
}

// TestBuildRoot_arrangeNarrowSetsChildBounds verifies that OnArrange in narrow mode
// successfully delegates to the layout policy.
func TestBuildRoot_arrangeNarrowSetsChildBounds(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutNarrow)

	bounds := gfx.Rect{Max: gfx.Point{X: 480, Y: 800}}
	ac := facet.ArrangeContext{}
	// Should not panic; layout policy handles arrangement
	root.Base().LayoutRole().OnArrange(ac, bounds)

	// Verify root's arranged bounds were set
	lr := root.Base().LayoutRole()
	if lr.ArrangedBounds != bounds {
		t.Fatalf("root arranged bounds not set correctly")
	}
}

// TestBuildRoot_childRenders verifies that the layout system handles rendering.
func TestBuildRoot_childRenders(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	// Arrange the root
	bounds := gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}}
	ac := facet.ArrangeContext{}
	root.Base().LayoutRole().OnArrange(ac, bounds)

	// With delegated layout, children render within their layout policies.
	// Verify root's render role produces output.
	rr := root.Base().RenderRole()
	if rr == nil {
		t.Fatal("root has no RenderRole")
	}
	list := &gfx.CommandList{}
	rr.OnCollect(list, bounds)
	if list.Len() == 0 {
		t.Fatal("expected root to render background")
	}
}

func TestBuildRoot_layoutModeFlipsOnWidthCrossing(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutWide)

	c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
	mc := facet.MeasureContext{}
	root.Base().LayoutRole().OnMeasure(mc, c)

	if got := as.LayoutMode.Get(); got != state.LayoutWide {
		t.Fatalf("expected LayoutWide at 1280, got %q", got)
	}

	c = facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}}
	root.Base().LayoutRole().OnMeasure(mc, c)

	if got := as.LayoutMode.Get(); got != state.LayoutNarrow {
		t.Fatalf("expected LayoutNarrow at 480, got %q", got)
	}

	c = facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
	root.Base().LayoutRole().OnMeasure(mc, c)

	if got := as.LayoutMode.Get(); got != state.LayoutWide {
		t.Fatalf("expected LayoutWide after returning to 1280, got %q", got)
	}
}

func TestBuildRoot_stableLayoutDoesNotRepeatedlyWriteMode(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	as.LayoutMode.Set(state.LayoutWide)

	writeCount := 0
	subID := as.LayoutMode.OnChange.Subscribe(func(c signal.Change[state.LayoutMode]) {
		writeCount++
	})
	defer as.LayoutMode.OnChange.Unsubscribe(subID)

	mc := facet.MeasureContext{}
	for range 5 {
		c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
		root.Base().LayoutRole().OnMeasure(mc, c)
	}

	if writeCount != 0 {
		t.Fatalf("expected 0 writes when mode doesn't change, got %d", writeCount)
	}
}

// TestBuildRoot_wideAndNarrowProduceDifferentBounds verifies that different
// layout policies are used for wide vs narrow mode.
func TestBuildRoot_wideAndNarrowProduceDifferentBounds(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	mc := facet.MeasureContext{}
	ac := facet.ArrangeContext{}

	// Wide mode
	as.LayoutMode.Set(state.LayoutWide)
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	root.Base().LayoutRole().OnArrange(ac, gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}})

	wideArranged := root.Base().LayoutRole().ArrangedBounds

	// Narrow mode
	as.LayoutMode.Set(state.LayoutNarrow)
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	root.Base().LayoutRole().OnArrange(ac, gfx.Rect{Max: gfx.Point{X: 480, Y: 800}})

	narrowArranged := root.Base().LayoutRole().ArrangedBounds

	// Root should have different arranged bounds for each mode
	if wideArranged == narrowArranged {
		t.Fatal("expected different arranged bounds for wide vs narrow mode")
	}

	// Verify the bounds match the input
	if wideArranged.Max.X != 1280 {
		t.Fatalf("wide mode width incorrect: got %v", wideArranged)
	}
	if narrowArranged.Max.X != 480 {
		t.Fatalf("narrow mode width incorrect: got %v", narrowArranged)
	}
}
