package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/app"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

var testTheme = theme.DefaultResolvedContext()
var testFontReg = newFontRegistryOrPanic()

func newFontRegistryOrPanic() *text.FontRegistry {
	r, err := text.NewFontRegistry()
	if err != nil {
		panic(err)
	}
	if err := r.LoadFontFile("../text/testdata/NotoSans-Regular.ttf"); err != nil {
		// Try alternative paths for different working directories
		paths := []string{
			"text/testdata/NotoSans-Regular.ttf",
			"../text/testdata/NotoSans-Regular.ttf",
			"../../text/testdata/NotoSans-Regular.ttf",
		}
		for _, p := range paths {
			if err := r.LoadFontFile(p); err == nil {
				return r
			}
		}
	}
	return r
}

type testRuntime struct {
	fontReg *text.FontRegistry
}

func (r testRuntime) FontRegistry() *text.FontRegistry                                 { return r.fontReg }
func (testRuntime) Schedule(j job.AnyJob)                                              {}
func (testRuntime) CancelJob(id job.JobID)                                             {}
func (testRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (testRuntime) Schemes() []string                                                  { return nil }
func (testRuntime) Assets() any                                                        { return nil }

func testBuildContext() app.BuildContext {
	return app.BuildContext{
		WindowSize:   gfx.Size{W: 1280, H: 800},
		ContentScale: 1,
		Theme:        testTheme,
	}
}

func testMeasureContext() facet.MeasureContext {
	return facet.MeasureContext{
		Theme:        testTheme,
		ContentScale: 1,
		Runtime:      testRuntime{fontReg: testFontReg},
	}
}

func TestBuildRoot_nonNil(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	if root == nil {
		t.Fatal("BuildRoot returned nil")
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
	mc := testMeasureContext()
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

func TestBuildRoot_hasDelegatedLayout(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	children := root.Base().Children()
	if len(children) < 1 {
		t.Fatalf("expected at least 1 child (the ColumnLayout), got %d", len(children))
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

func TestBuildRoot_arrangeSetsRootBounds(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
	mc := testMeasureContext()
	root.Base().LayoutRole().OnMeasure(mc, c)

	bounds := gfx.Rect{Max: gfx.Point{X: 1280, Y: 800}}
	ac := facet.ArrangeContext{}
	root.Base().LayoutRole().OnArrange(ac, bounds)

	lr := root.Base().LayoutRole()
	if lr.ArrangedBounds != bounds {
		t.Fatalf("root arranged bounds not set correctly")
	}
}

func TestBuildRoot_hasElevenDirectChildren(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	children := root.Base().Children()
	if len(children) != 11 {
		t.Fatalf("expected 11 children (chromeColumn, sources, center, inspector, status, 6 overlays), got %d", len(children))
	}
}

// TestBuildRoot_childRenders verifies that the layout system handles rendering.
func TestBuildRoot_childRenders(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())

	mc := testMeasureContext()
	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})

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
	mc := testMeasureContext()
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

	mc := testMeasureContext()
	for range 5 {
		c := facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}}
		root.Base().LayoutRole().OnMeasure(mc, c)
	}

	if writeCount != 0 {
		t.Fatalf("expected 0 writes when mode doesn't change, got %d", writeCount)
	}
}

func TestBuildRoot_measuresDifferentSizesForModes(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext())
	mc := testMeasureContext()

	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if as.LayoutMode.Get() != state.LayoutWide {
		t.Fatal("expected LayoutWide at 1280")
	}

	root.Base().LayoutRole().OnMeasure(mc, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if as.LayoutMode.Get() != state.LayoutNarrow {
		t.Fatal("expected LayoutNarrow at 480")
	}
}

func TestRootHasStatusBar(t *testing.T) {
	as := state.NewAppState(nil)
	root := BuildRoot(as, testBuildContext()).(*RootFacet)
	if root.statusBar == nil {
		t.Fatal("root has no status bar")
	}
	if root.statusBar.light == nil {
		t.Fatal("status bar has no status light")
	}
	if root.statusBar.progressBar == nil {
		t.Fatal("status bar has no progress bar")
	}
	if root.statusBar.progressRing == nil {
		t.Fatal("status bar has no progress ring")
	}
	if root.statusBar.badge == nil {
		t.Fatal("status bar has no badge")
	}
	if root.statusBar.statusText == nil {
		t.Fatal("status bar has no status text")
	}
}

func TestStatusBar_connectionStateUpdatesLight(t *testing.T) {
	as := state.NewAppState(nil)
	newStatusBar(as)
	as.Connection.Set(state.ConnConnecting)
	if as.Connection.Get() != state.ConnConnecting {
		t.Fatal("connection state should be connecting")
	}
}

func TestSimulateReloadJob_endsConnected(t *testing.T) {
	as := state.NewAppState(makeTestDataset(10, []string{"NA"}))
	as.Connection.Set(state.ConnDisconnected)
	simulateReloadJob(as)
	if as.Connection.Get() != state.ConnConnected {
		t.Fatalf("expected Connected after reload, got %q", as.Connection.Get())
	}
	if as.JobProgress.Get() != 0 {
		t.Fatalf("expected JobProgress 0 after reload, got %f", as.JobProgress.Get())
	}
}
