package studio

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks/viz"
	"codeburg.org/lexbit/lurpicui/theme"
)

// seedRows loads the committed seed CSV (same schema the app ships).
func seedRows(t *testing.T) []dataset.Row {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "assets", "metrics.csv"))
	if err != nil {
		t.Fatalf("read seed: %v", err)
	}
	rows, err := dataset.Parse(data)
	if err != nil {
		t.Fatalf("parse seed: %v", err)
	}
	return rows
}

// newProbeHarness builds the probe over the seed and runs one frame.
func newProbeHarness(t *testing.T) (*VizProbe, *testkit.Harness) {
	t.Helper()
	probe := NewVizProbe(seedRows(t), testkit.TestFontRegistry(t), theme.DefaultResolvedContext())
	h := testkit.NewStandardHarness(t, 640, 360, probe)
	h.RunFrame()
	return probe, h
}

// TestVizProbe_roundTripBothAxes asserts Map → Invert recovers the domain
// value on both axes (R-scale: the screen↔data mapping is the load-bearing
// fact the chart and brushing build on).
func TestVizProbe_roundTripBothAxes(t *testing.T) {
	probe, _ := newProbeHarness(t)
	xs := probe.XScale().Get()
	ys := probe.YScale().Get()

	rows := seedRows(t)
	for _, r := range rows {
		// X axis: time (unix seconds) → pixel → time.
		tv := vizRowTime(r)
		if back := xs.Invert(xs.Map(tv)); math.Abs(back-tv) > 1e-3 {
			t.Errorf("x round-trip row %s: %v -> %v", r.Time, tv, back)
		}
		// Y axis: value → pixel → value.
		vv := vizRowValue(r)
		if back := ys.Invert(ys.Map(vv)); math.Abs(back-vv) > 1e-6 {
			t.Errorf("y round-trip row %s: %v -> %v", r.Time, vv, back)
		}
	}

	// The scales must map into the plot's pixel ranges (screen y-down).
	plot := probe.PlotRect()
	if lo, hi := xs.Range(); lo != 0 || math.Abs(hi-float64(plot.Width())) > 1 {
		t.Errorf("x range = [%v %v], want [0 %v]", lo, hi, plot.Width())
	}
	if lo, hi := ys.Range(); lo != float64(plot.Height()) || hi != 0 {
		t.Errorf("y range = [%v %v], want [%v 0] (screen y-down)", lo, hi, plot.Height())
	}
}

// TestVizProbe_linePathSegments asserts the line projects one point per seed
// row (N rows → N points → N-1 segments).
func TestVizProbe_linePathSegments(t *testing.T) {
	probe, h := newProbeHarness(t)
	cmds := probe.Line().Base().ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      h.Runtime(),
		Bounds:       probe.PlotRect(),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("line produced no commands")
	}
	var pts []gfx.Point
	for _, c := range cmds.Commands {
		if poly, ok := c.(gfx.DrawPolyline); ok {
			pts = poly.Points
		}
	}
	want := len(seedRows(t))
	if len(pts) != want {
		t.Fatalf("line points = %d, want %d (one per seed row)", len(pts), want)
	}
	if got := len(pts) - 1; got != want-1 {
		t.Fatalf("line segments = %d, want %d", got, want-1)
	}
}

// TestVizProbe_rulePosition asserts the reference rule is drawn at the y-scale
// pixel of its value.
func TestVizProbe_rulePosition(t *testing.T) {
	probe, h := newProbeHarness(t)
	val := probe.RuleValue().Get()
	want := probe.PlotRect().Min.Y + float32(probe.YScale().Get().Map(val))

	cmds := probe.Rule().Base().ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      h.Runtime(),
		Bounds:       probe.PlotRect(),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("rule produced no commands")
	}
	for _, c := range cmds.Commands {
		path, ok := c.(gfx.StrokePath)
		if !ok || len(path.Path.Segments) == 0 {
			continue
		}
		got := path.Path.Segments[0].Pts[0].Y
		if math.Abs(float64(got-want)) > 1 {
			t.Fatalf("rule Y = %v, want %v (scale pixel of value %v)", got, want, val)
		}
		return
	}
	t.Fatal("rule produced no StrokePath")
}

// probeMeasureArrange attaches the probe and drives its layout directly,
// without a runtime — so store signals fire synchronously (the runtime's
// deferred signal queue is not installed), letting tests observe facet-level
// invalidation.
func probeMeasureArrange(t *testing.T, probe *VizProbe, w, h float32) {
	t.Helper()
	ctx := theme.DefaultResolvedContext()
	facet.Attach(probe, facet.AttachContext{Theme: ctx})
	mctx := facet.MeasureContext{Theme: ctx, ContentScale: 1}
	probe.layout.Measure(mctx, facet.Constraints{MaxSize: gfx.Size{W: w, H: h}})
	probe.layout.Arrange(facet.ArrangeContext{Theme: ctx}, gfx.RectFromXYWH(0, 0, w, h))
}

// TestVizProbe_axisTickLabels asserts both axes project tick marks and shaped
// labels from their scale entries (R-scale: the axes are the legibility layer
// the chart and brushing depend on).
func TestVizProbe_axisTickLabels(t *testing.T) {
	probe, h := newProbeHarness(t)
	for name, axis := range map[string]*viz.Axis{"x": probe.XAxis(), "y": probe.YAxis()} {
		cmds := axis.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      h.Runtime(),
			Bounds:       axis.Base().LayoutRole().ArrangedBounds,
			ContentScale: 1,
		})
		if cmds == nil || cmds.Len() == 0 {
			t.Fatalf("%s axis produced no commands", name)
		}
		var ticks, labels int
		for _, c := range cmds.Commands {
			switch c.(type) {
			case gfx.StrokePath:
				ticks++
			case gfx.DrawGlyphRun:
				labels++
			}
		}
		if ticks == 0 {
			t.Errorf("%s axis produced no tick marks", name)
		}
		if labels == 0 {
			t.Errorf("%s axis produced no tick labels", name)
		}
	}
}

// TestVizProbe_zoomIsolation asserts the FR-rt isolation property here first:
// a domain Set re-projects the line with DirtyProjection only — never
// DirtyLayout — and does not dirty the probe shell at all.
func TestVizProbe_zoomIsolation(t *testing.T) {
	probe := NewVizProbe(seedRows(t), testkit.TestFontRegistry(t), theme.DefaultResolvedContext())
	probeMeasureArrange(t, probe, 640, 360)

	probe.ZoomController().Zoom(probe.XScale().Get().Invert(float64(probe.PlotRect().Width()/2)), 2.0)
	probe.XScale().Get() // force the scale recompute → its OnChange invalidates the line

	lineFlags := probe.Line().Base().DirtyFlags()
	if lineFlags&facet.DirtyLayout != 0 {
		t.Fatalf("line DirtyLayout after zoom = %v", lineFlags)
	}
	if lineFlags&facet.DirtyProjection == 0 {
		t.Fatalf("line DirtyProjection not set after zoom: %v", lineFlags)
	}
	if shell := probe.Base().DirtyFlags(); shell&facet.DirtyLayout != 0 {
		t.Fatalf("probe shell DirtyLayout after zoom = %v", shell)
	}
}

// TestVizProbe_panZoomInput drives the probe through the harness's input
// pipeline and asserts drag pans and wheel zooms the x-domain.
func TestVizProbe_panZoomInput(t *testing.T) {
	probe, h := newProbeHarness(t)
	cx := probe.PlotRect().Min.X + probe.PlotRect().Width()*0.5
	cy := probe.PlotRect().Min.Y + probe.PlotRect().Height()*0.5

	before := probe.XDomain().Get()
	testkit.DriveDrag(h, cx, cy, cx+50, cy)
	after := probe.XDomain().Get()
	if after == before {
		t.Fatal("drag did not pan the x-domain")
	}
	if after[0] >= before[0] {
		t.Fatalf("drag right should pan toward earlier data: domain %v -> %v", before, after)
	}

	before = probe.XDomain().Get()
	testkit.DriveScroll(h, cx, cy, 0, -100) // scroll up → zoom in (factor 2)
	after = probe.XDomain().Get()
	spanBefore := before[1] - before[0]
	spanAfter := after[1] - after[0]
	if spanAfter >= spanBefore {
		t.Fatalf("scroll up should zoom in: span %v -> %v", spanBefore, spanAfter)
	}
}

// TestVizProbe_golden pins the rendered probe chart (line + axes + rule) so
// viz regressions are caught by pixel diff.
func TestVizProbe_golden(t *testing.T) {
	_, h := newProbeHarness(t)
	testkit.AssertGolden(t, h.Surface(), "viz_probe")
}
