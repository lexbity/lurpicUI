package studio

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func testRespState(t *testing.T) *state.AppState {
	t.Helper()
	raw, err := os.ReadFile(filepath.FromSlash("../assets/metrics.csv"))
	if err != nil {
		t.Fatalf("reading metrics.csv: %v", err)
	}
	rows, err := dataset.Parse(raw)
	if err != nil {
		t.Fatalf("parsing metrics.csv: %v", err)
	}
	return state.NewAppState(rows)
}

func TestNarrowSourcesCollapsed(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 480, 800))
	b := root.ArrangedBounds()

	if b.Sources.Width() != 0 || b.Sources.Height() != 0 {
		t.Errorf("expected sources pane collapsed in narrow mode, got width=%f height=%f", b.Sources.Width(), b.Sources.Height())
	}
}

func TestNarrowInspectorCollapsed(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 480, 800))
	b := root.ArrangedBounds()

	if b.Inspector.Width() != 0 || b.Inspector.Height() != 0 {
		t.Errorf("expected inspector pane collapsed in narrow mode, got width=%f height=%f", b.Inspector.Width(), b.Inspector.Height())
	}
}

func TestNarrowCenterFullWidth(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 480, 800))
	b := root.ArrangedBounds()

	if b.Center.Width() < 470 {
		t.Errorf("expected center pane near full width in narrow mode, got %f", b.Center.Width())
	}
}

func TestWideSourcesPresent(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	b := root.ArrangedBounds()

	if b.Sources.Width() < 190 {
		t.Errorf("expected sources pane ~200px in wide mode, got %f", b.Sources.Width())
	}
}

func TestWideInspectorPresent(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	root.onArrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	b := root.ArrangedBounds()

	if b.Inspector.Width() < 270 {
		t.Errorf("expected inspector pane ~280px in wide mode, got %f", b.Inspector.Width())
	}
}

func TestStateSurvivesModeSwitch(t *testing.T) {
	s := testRespState(t)
	s.ChartTitle.Set("Preserved Title")
	s.YAxisMax.Set(50000)
	s.ChartType.Set(state.ChartBar)

	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})
	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})

	if s.ChartTitle.Get() != "Preserved Title" {
		t.Errorf("ChartTitle should survive mode switch, got %q", s.ChartTitle.Get())
	}
	if s.YAxisMax.Get() != 50000 {
		t.Errorf("YAxisMax should survive mode switch, got %f", s.YAxisMax.Get())
	}
	if s.ChartType.Get() != state.ChartBar {
		t.Errorf("ChartType should survive mode switch, got %v", s.ChartType.Get())
	}
}

func TestLayoutModeFlipsCorrectly(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 1280, H: 800})

	if s.LayoutMode.Get() != state.LayoutWide {
		t.Errorf("expected LayoutWide for 1280, got %v", s.LayoutMode.Get())
	}

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 800}})
	if s.LayoutMode.Get() != state.LayoutNarrow {
		t.Errorf("expected LayoutNarrow for 480, got %v", s.LayoutMode.Get())
	}

	root.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 800}})
	if s.LayoutMode.Get() != state.LayoutWide {
		t.Errorf("expected LayoutWide again, got %v", s.LayoutMode.Get())
	}
}

func TestNarrowNavDrawerAccessible(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	if root.overlayHost == nil || root.overlayHost.NavDrawer() == nil {
		t.Fatal("nav drawer should exist in narrow mode")
	}
	drawer := root.overlayHost.NavDrawer()
	_ = drawer
}

func TestNarrowBottomSheetToggle(t *testing.T) {
	s := testRespState(t)
	root := testNewRoot(t, s, gfx.Size{W: 480, H: 800})

	if root.overlayHost == nil || root.overlayHost.BottomSheet() == nil {
		t.Fatal("bottom sheet should exist in narrow mode")
	}
	if root.overlayHost.BottomSheetOpen() {
		t.Error("bottom sheet should be closed initially")
	}
	root.overlayHost.ToggleBottomSheet()
	if !root.overlayHost.BottomSheetOpen() {
		t.Error("bottom sheet should be open after toggle")
	}
	root.overlayHost.ToggleBottomSheet()
	if root.overlayHost.BottomSheetOpen() {
		t.Error("bottom sheet should be closed after second toggle")
	}
}
