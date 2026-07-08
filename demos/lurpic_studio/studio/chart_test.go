package studio

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
)

func TestNewChartCanvas_createsAllMarks(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA", "EU"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)
	if cc == nil {
		t.Fatal("newChartCanvas returned nil")
	}
	if cc.xAxis == nil {
		t.Fatal("no x axis")
	}
	if cc.yAxis == nil {
		t.Fatal("no y axis")
	}
	if cc.ruleLine == nil {
		t.Fatal("no rule line")
	}
	if cc.lineMark == nil {
		t.Fatal("no line mark")
	}
	if cc.areaMark == nil {
		t.Fatal("no area mark")
	}
	if cc.scatter == nil {
		t.Fatal("no scatter mark")
	}
	if cc.barMark == nil {
		t.Fatal("no bar mark")
	}
}

func TestChartCanvas_hasSevenChildren(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)
	children := cc.Base().Children()
	if len(children) != 7 {
		t.Fatalf("expected 7 children (yAxis, xAxis, rule, line, area, scatter, bar), got %d", len(children))
	}
}

func TestChartCanvas_childTypes(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)
	expected := []string{"axis", "axis", "rule", "line", "area", "datascatter", "bar"}
	for i, child := range cc.Base().Children() {
		impl := child.Impl()
		if impl == nil {
			continue
		}
		if m, ok := impl.(marks.Mark); ok {
			d := m.Descriptor()
			if d.TypeName != expected[i] {
				t.Fatalf("child[%d]: expected %q, got %q", i, expected[i], d.TypeName)
			}
		}
	}
}

func TestChartCanvas_arrangeUpdatesScales(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)

	bounds := gfx.Rect{Max: gfx.Point{X: 400, Y: 300}}

	cc.layout.OnMeasure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
	cc.layout.OnArrange(facet.ArrangeContext{}, bounds)

	xr := cc.xRange.Get()
	if xr[0] < xr[1] {
		t.Logf("xRange: [%f, %f] (valid)", xr[0], xr[1])
	} else {
		t.Fatalf("xRange: expected [lo < hi], got [%f, %f]", xr[0], xr[1])
	}

	yr := cc.yRange.Get()
	if yr[0] > yr[1] {
		t.Logf("yRange: [%f, %f] (valid, flipped)", yr[0], yr[1])
	} else {
		t.Fatalf("yRange: expected [hi > lo] for flipped scale, got [%f, %f]", yr[0], yr[1])
	}
}

func TestChartCanvas_renderProducesCommands(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)

	bounds := gfx.Rect{Max: gfx.Point{X: 400, Y: 300}}
	cc.layout.OnMeasure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: bounds.Width(), H: bounds.Height()}})
	cc.layout.OnArrange(facet.ArrangeContext{}, bounds)

	list := &gfx.CommandList{}
	cc.render.OnCollect(list, bounds)
	if list.Len() == 0 {
		t.Fatal("expected at least one command from chart canvas render")
	}
}

func TestChartCanvas_xDomainUpdatesWithData(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	_ = newChartCanvas(as)

	xd := as.VisibleRows.Get()
	if len(xd) == 0 {
		t.Fatal("expected visible rows after data set")
	}
}

func TestChartCanvas_ruleBoundToThreshold(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)

	val := cc.ruleLine.Value.Get()
	_ = val

	as.Threshold.Set(float64(5000))
	updated := cc.ruleLine.Value.Get()
	if updated != 5000 {
		t.Fatalf("expected rule value 5000 after Threshold set, got %f", updated)
	}
}

func TestChartCanvas_lineMarkColorIsAccent(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA"})
	as := state.NewAppState(rows)
	cc := newChartCanvas(as)
	expected := gfx.Color{R: 0.31, G: 0.78, B: 0.62, A: 1}
	if cc.lineMark.Color != expected {
		t.Fatalf("expected line color %v, got %v", expected, cc.lineMark.Color)
	}
}

func TestChartCanvas_barMarkRegionsFromBuckets(t *testing.T) {
	rows := makeTestDataset(5, []string{"NA", "EU", "APAC"})
	as := state.NewAppState(rows)
	_ = newChartCanvas(as)
	buckets := as.BarBuckets.Get()
	if len(buckets) != 3 {
		t.Fatalf("expected 3 bar buckets for 3 regions, got %d", len(buckets))
	}
}
