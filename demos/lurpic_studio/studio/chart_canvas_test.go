package studio

import (
	"math"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/scale"
)

func chartRows() []dataset.Row {
	return []dataset.Row{
		{Date: time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), Revenue: 10000, Users: 1000, Region: "NA"},
		{Date: time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), Revenue: 20000, Users: 2000, Region: "EU"},
		{Date: time.Date(2026, 1, 3, 0, 0, 0, 0, time.UTC), Revenue: 30000, Users: 3000, Region: "APAC"},
		{Date: time.Date(2026, 1, 4, 0, 0, 0, 0, time.UTC), Revenue: 15000, Users: 1500, Region: "LATAM"},
		{Date: time.Date(2026, 1, 5, 0, 0, 0, 0, time.UTC), Revenue: 25000, Users: 2500, Region: "NA"},
	}
}

func TestChartCanvasConstructs(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)
	if cc == nil {
		t.Fatal("NewChartCanvas returned nil")
	}
	if cc.XAxis() == nil || cc.YAxis() == nil {
		t.Fatal("ChartCanvas missing axes")
	}
	if cc.Rule() == nil {
		t.Fatal("ChartCanvas missing rule")
	}
	if cc.Line() == nil || cc.Area() == nil || cc.Point() == nil || cc.Bar() == nil {
		t.Fatal("ChartCanvas missing data marks")
	}
}

func TestChartCanvasScaleMapping(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	xs := cc.XScale().Get()
	ys := cc.YScale().Get()

	first := chartRows()[0]
	xPixel := xs.Map(float64(first.Date.Unix()))
	yPixel := ys.Map(first.Revenue)

	if math.IsInf(xPixel, 0) || math.IsNaN(xPixel) {
		t.Fatal("x pixel mapping produced non-finite value")
	}
	if math.IsInf(yPixel, 0) || math.IsNaN(yPixel) {
		t.Fatal("y pixel mapping produced non-finite value")
	}

	recoveredDate := xs.Invert(xPixel)
	recoveredRev := ys.Invert(yPixel)

	dateDiff := math.Abs(recoveredDate - float64(first.Date.Unix()))
	revDiff := math.Abs(recoveredRev - first.Revenue)
	if dateDiff > 1 {
		t.Errorf("x axis round-trip error: %.2f seconds", dateDiff)
	}
	if revDiff > 0.01 {
		t.Errorf("y axis round-trip error: %.2f", revDiff)
	}
}

func TestChartCanvasScalePoint(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	x, y := cc.ScalePoint(chartRows()[0])
	if math.IsInf(x, 0) || math.IsNaN(x) {
		t.Fatal("ScalePoint x is non-finite")
	}
	if math.IsInf(y, 0) || math.IsNaN(y) {
		t.Fatal("ScalePoint y is non-finite")
	}
	if x < 0 || x > 540 {
		t.Errorf("ScalePoint x=%f out of plot bounds [0,540]", x)
	}
	if y < 0 || y > 360 {
		t.Errorf("ScalePoint y=%f out of plot bounds [0,360]", y)
	}
}

func TestChartCanvasLineSegments(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	proj := cc.Line().Base().ProjectionRole()
	if proj == nil {
		t.Fatal("Line mark has no ProjectionRole")
	}
	cmds := proj.Project(facet.ProjectionContext{Bounds: cc.Line().Base().LayoutRole().ArrangedBounds})
	if cmds == nil || cmds.Len() == 0 {
		t.Errorf("Line projection produced no commands: nil=%v len=%d", cmds == nil, func() int { if cmds == nil { return 0 }; return cmds.Len() }())
	}
}

func TestBarBucketsCount(t *testing.T) {
	s := state.NewAppState(chartRows())
	buckets := s.BarBuckets.Get()
	if len(buckets) != 4 {
		t.Fatalf("expected 4 bar buckets (one per region on page 1), got %d", len(buckets))
	}
}

func TestChartCanvasRuleAtThreshold(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	ys := cc.YScale().Get()
	threshold := s.Threshold.Get()
	thresholdPixel := ys.Map(threshold)
	if math.IsInf(thresholdPixel, 0) || math.IsNaN(thresholdPixel) {
		t.Fatal("threshold pixel mapping is non-finite")
	}
	recovered := ys.Invert(thresholdPixel)
	if math.Abs(recovered-threshold) > 1 {
		t.Errorf("threshold round-trip error: expected %.2f, got %.2f", threshold, recovered)
	}
}

func TestChartCanvasXScaleMapsFirstPoint(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	xs := cc.XScale().Get()

	firstTime := float64(chartRows()[0].Date.Unix())
	firstPixel := xs.Map(firstTime)
	if math.IsInf(firstPixel, 0) || math.IsNaN(firstPixel) {
		t.Fatal("initial x mapping produced non-finite value")
	}
	_ = xs.Map(firstTime + 86400)
	if math.IsInf(firstPixel, 0) || math.IsNaN(firstPixel) {
		t.Fatal("second x mapping produced non-finite value")
	}
}

func TestChartCanvasPlotBounds(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	plot := cc.PlotBounds()
	if plot.Width() != 540 {
		t.Errorf("expected plot width 540 (600-50-10), got %f", plot.Width())
	}
	if plot.Height() != 360 {
		t.Errorf("expected plot height 360 (400-10-30), got %f", plot.Height())
	}
}

func TestChartCanvasAllChartTypesProduceCommands(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	types := []struct {
		name  string
		ct    state.ChartType
		mark  func() facet.FacetImpl
	}{
		{"line", state.ChartLine, func() facet.FacetImpl { return cc.Line() }},
		{"area", state.ChartArea, func() facet.FacetImpl { return cc.Area() }},
		{"point", state.ChartPoint, func() facet.FacetImpl { return cc.Point() }},
		{"bar", state.ChartBar, func() facet.FacetImpl { return cc.Bar() }},
	}
	for _, tc := range types {
		t.Run(tc.name, func(t *testing.T) {
			s.ChartType.Set(tc.ct)
			s.ChartType.Get()
			cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))
			m := tc.mark()
			if m == nil || m.Base() == nil {
				t.Fatal("mark is nil")
			}
			bounds := m.Base().LayoutRole().ArrangedBounds
			if bounds.IsEmpty() {
				t.Fatalf("%s: arranged bounds are empty after ChartType=%v", tc.name, tc.ct)
			}
			proj := m.Base().ProjectionRole()
			if proj == nil {
				t.Fatal("mark has no ProjectionRole")
			}
			cmds := proj.Project(facet.ProjectionContext{Bounds: bounds})
			if cmds == nil || cmds.Len() == 0 {
				t.Errorf("%s: expected at least 1 command with bounds %v, got nil=%v len=%d", tc.name, bounds, cmds == nil, func() int { if cmds == nil { return 0 }; return cmds.Len() }())
			}
		})
	}
}

func TestChartCanvasScaleInvertibility(t *testing.T) {
	s := state.NewAppState(chartRows())
	fonts := testkit.TestFontRegistry(t)
	cc := NewChartCanvas(s, fonts)

	cc.layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 600, H: 400}})
	cc.layout.Arrange(facet.ArrangeContext{Placement: facet.Placement{Mode: facet.PlacementGrid}}, gfx.RectFromXYWH(0, 0, 600, 400))

	ys := cc.YScale().Get().(scale.InvertibleScale)
	testPixels := []float64{0, 100, 200, 300, 360}
	for _, px := range testPixels {
		val := ys.Invert(px)
		rePx := ys.Map(val)
		diff := math.Abs(rePx - px)
		if diff > 1 {
			t.Errorf("pixel %.0f → value %.2f → pixel %.2f (error %.2f)", px, val, rePx, diff)
		}
	}
}
