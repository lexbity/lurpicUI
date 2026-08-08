package studio

import (
	"image"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/theme"
)

// newE1Harness builds the E1 facet over the seed and runs one frame.
func newE1Harness(t *testing.T) (*Realtime, *testkit.Harness) {
	t.Helper()
	appState := state.NewAppState(seedRows(t))
	e := NewRealtimeFacet(appState, testkit.TestFontRegistry(t), theme.DefaultResolvedContext())
	h := testkit.NewStandardHarness(t, 640, 480, e)
	h.RunFrame()
	return e, h
}

func linePoints(t *testing.T, e *Realtime) []gfx.Point {
	t.Helper()
	cmds := e.Canvas().Line().Base().ProjectionRole().Project(facet.ProjectionContext{
		Bounds:       e.Canvas().PlotRect(),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		return nil
	}
	for _, c := range cmds.Commands {
		if poly, ok := c.(gfx.DrawPolyline); ok {
			return poly.Points
		}
	}
	return nil
}

// TestRealtime_tickRechartsWithoutRelayout asserts the FR-rt isolation
// property end-to-end: a feed tick appends a row, the chart re-projects it,
// and the frame ran no layout pass (the tick dirties projection only — the
// canvas and shell are not re-laid-out).
func TestRealtime_tickRechartsWithoutRelayout(t *testing.T) {
	e, h := newE1Harness(t)
	before := e.appState.Rows.Len()
	pointsBefore := len(linePoints(t, e))

	e.Feed().OnTick(100 * time.Millisecond)
	h.RunUntil(func() bool { return e.appState.Rows.Len() == before+1 }, 60)

	if got := len(linePoints(t, e)); got != pointsBefore+1 {
		t.Fatalf("line points = %d, want %d (chart re-projected the new row)", got, pointsBefore+1)
	}
	// The tick frame ran no layout pass (DirtyProjection only, no DirtyLayout).
	if d := h.LastFrameStats().LayoutDuration; d > 5*time.Millisecond {
		t.Fatalf("a feed tick triggered a layout pass (%v); FR-rt violated", d)
	}
}

// TestRealtime_liveTailPauseAndJumpToLive drives the live-tail loop through
// the real gestures (FR-window): a pan/zoom gesture sets Paused so the feed's
// AnchorLiveWindow freezes the window while rows keep appending; jump-to-live
// resets the domain and resumes the slide.
func TestRealtime_liveTailPauseAndJumpToLive(t *testing.T) {
	e, h := newE1Harness(t)
	plot := e.Canvas().PlotRect()
	if plot.IsEmpty() {
		t.Fatal("chart plot is empty; cannot drive a pan gesture")
	}
	midY := plot.Min.Y + plot.Height()*0.5

	// A wheel zoom (a zoom gesture) must pause the live tail — without it the
	// next tick's AnchorLiveWindow would overwrite the zoom.
	zoomX := plot.Min.X + plot.Width()*0.5
	testkit.DriveScroll(h, zoomX, midY, 0, -40)
	if !e.appState.Paused.Get() {
		t.Fatal("wheel zoom did not set Paused (FR-window)")
	}
	windowBefore := e.appState.LiveWindow.Get()
	rowsBefore := e.appState.Rows.Len()

	e.Feed().OnTick(100 * time.Millisecond)
	h.RunUntil(func() bool { return e.appState.Rows.Len() == rowsBefore+1 }, 60)
	if w := e.appState.LiveWindow.Get(); w != windowBefore {
		t.Fatalf("paused live window moved after a wheel zoom: %v -> %v", windowBefore, w)
	}

	// Jump to live: reset the domain to [now-W, now] and clear the pause.
	hi := float64(e.appState.Rows.All()[e.appState.Rows.Len()-1].Time.Unix())
	e.Canvas().ResetDomain([2]float64{hi - e.appState.WindowSeconds.Get(), hi})
	if e.appState.Paused.Get() {
		t.Fatal("jump-to-live did not clear Paused")
	}
	if w := e.appState.LiveWindow.Get(); w[1] != hi {
		t.Fatalf("jump-to-live domain = %v, want hi %v", w, hi)
	}

	// A pan gesture also pauses (FR-window). A drag must move beyond the
	// drag threshold to count as a pan.
	testkit.DriveDrag(h, plot.Min.X+plot.Width()*0.2, midY, plot.Min.X+plot.Width()*0.4, midY)
	if !e.appState.Paused.Get() {
		t.Fatal("pan drag did not set Paused (FR-window)")
	}
	e.Canvas().ResetDomain([2]float64{hi - e.appState.WindowSeconds.Get(), hi})
}

// TestRealtime_selectionClickDoesNotPause asserts the FR-window over-trigger
// guard: a plain left-click that selects a point (chart → grid brush) must NOT
// pause the live tail — only a pan/zoom gesture may.
func TestRealtime_selectionClickDoesNotPause(t *testing.T) {
	e, h := newE1Harness(t)
	e.appState.Paused.Set(false)
	plot := e.Canvas().PlotRect()
	if plot.IsEmpty() {
		t.Fatal("chart plot is empty; cannot drive a selection click")
	}
	// Click on the plot center (no drag movement): a selection candidate.
	testkit.DriveClick(h, plot.Min.X+plot.Width()*0.5, plot.Min.Y+plot.Height()*0.5)
	if e.appState.Paused.Get() {
		t.Fatal("a plain selection click paused the live tail (over-trigger)")
	}
}

// TestRealtime_chartTypeGoldens pins the four series variants and proves they
// are mutually byte-distinct (NFR-determinism).
func TestRealtime_chartTypeGoldens(t *testing.T) {
	types := []string{"line", "area", "point", "bar"}
	images := map[string]*image.RGBA{}
	for _, ct := range types {
		probe := NewVizProbe(seedRows(t), testkit.TestFontRegistry(t), theme.DefaultResolvedContext())
		probe.ChartTypeStore().Set(ct)
		h := testkit.NewStandardHarness(t, 640, 360, probe)
		h.RunFrame()
		testkit.AssertGolden(t, h.Surface(), "e1_"+ct)
		images[ct] = h.Surface().Capture()
	}
	for i := 0; i < len(types); i++ {
		for j := i + 1; j < len(types); j++ {
			if surfacesEqual(images[types[i]], images[types[j]]) {
				t.Fatalf("chart types %s and %s produced identical renders; variants do not discriminate", types[i], types[j])
			}
		}
	}
}

// TestRealtime_radialReshapeChangesChartType drives the E1 radial_menu
// chart-reshape dial (the §3.3 placement: radial_menu · chart reshape · radial
// layout): clicking a radial icon-button child writes ChartType and the canvas
// re-projects the matching series.
func TestRealtime_radialReshapeChangesChartType(t *testing.T) {
	e, h := newE1Harness(t)
	menu := e.Reshape()
	b := e6Arranged(t, menu)
	cx := b.Min.X + b.Width()*0.5
	cy := b.Min.Y + b.Height()*0.5

	// The four children sit at the cardinal angles around the center. Click the
	// top child (angle 270° → the "bar" glyph) and the right child (angle 0° →
	// the "line" glyph).
	testkit.DriveClick(h, cx, cy-reshapeDialRadius)
	if got := e.ChartType().Get(); got != ChartBar.String() {
		t.Fatalf("top radial child set chart type %q, want %q", got, ChartBar.String())
	}
	h.RunFrame()
	// The canvas's active series follows the chart type.
	if got := e.Canvas().ChartTypeStore().Get(); got != ChartBar.String() {
		t.Fatalf("canvas chart type = %q, want %q", got, ChartBar.String())
	}
	if e.Canvas().Bar().Base().LayoutRole().ArrangedBounds.IsEmpty() {
		t.Fatal("bar series is not arranged after the reshape")
	}

	testkit.DriveClick(h, cx+reshapeDialRadius, cy)
	if got := e.ChartType().Get(); got != ChartLine.String() {
		t.Fatalf("right radial child set chart type %q, want %q", got, ChartLine.String())
	}
}
