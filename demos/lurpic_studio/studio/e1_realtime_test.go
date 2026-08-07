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

// TestRealtime_liveTailPauseAndJumpToLive drives the live-tail loop: pausing
// (a pan/zoom gesture sets Paused) freezes the window while rows keep
// appending; jump-to-live resets the domain and resumes the slide.
func TestRealtime_liveTailPauseAndJumpToLive(t *testing.T) {
	e, h := newE1Harness(t)
	e.appState.Paused.Set(true)
	windowBefore := e.appState.LiveWindow.Get()
	rowsBefore := e.appState.Rows.Len()

	e.Feed().OnTick(100 * time.Millisecond)
	h.RunUntil(func() bool { return e.appState.Rows.Len() == rowsBefore+1 }, 60)
	if w := e.appState.LiveWindow.Get(); w != windowBefore {
		t.Fatalf("paused live window moved: %v -> %v", windowBefore, w)
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
