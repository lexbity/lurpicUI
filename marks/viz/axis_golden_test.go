package viz

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/scale/reactive"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestAxisGoldenLinear(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)

	fonts := mustVizFontRegistry(t)
	a := NewAxis(rs, marks.Const(AxisBottom), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(10, 10, 300, 30)
	a.Layout.Measure(facet.MeasureContext{
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 60}})
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	a.TickCount = marks.Const(6)

	cmds := a.Projection.Project(facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	surface := testkit.NewMemorySurface(400, 60)
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	if err := r.Submit(&render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds.Commands},
			},
		},
	}); err != nil {
		t.Fatalf("submit frame: %v", err)
	}

	testkit.AssertGolden(t, surface, "axis_linear_bottom")
}

func TestAxisGoldenLinearLeft(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	fonts := mustVizFontRegistry(t)
	a := NewAxis(rs, marks.Const(AxisLeft), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(10, 10, 40, 200)
	a.Layout.Measure(facet.MeasureContext{
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 60, H: 240}})
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	a.TickCount = marks.Const(5)

	cmds := a.Projection.Project(facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands")
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 60, 240)
	testkit.AssertGolden(t, surface, "axis_linear_left")
}

func TestAxisGoldenLinearTop(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewLinearReactive(domain, rng)

	fonts := mustVizFontRegistry(t)
	a := NewAxis(rs, marks.Const(AxisTop), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(10, 10, 300, 30)
	a.Layout.Measure(facet.MeasureContext{
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 60}})
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	a.TickCount = marks.Const(6)

	cmds := a.Projection.Project(facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands for top axis")
	}

	surface := testkit.NewMemorySurface(400, 60)
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	if err := r.Submit(&render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds.Commands},
			},
		},
	}); err != nil {
		t.Fatalf("submit frame: %v", err)
	}

	testkit.AssertGolden(t, surface, "axis_linear_top")
}

func TestAxisGoldenLinearRight(t *testing.T) {
	domain := store.NewValueStore([2]float64{0, 100})
	rng := store.NewValueStore([2]float64{0, 200})
	rs := reactive.NewLinearReactive(domain, rng)

	fonts := mustVizFontRegistry(t)
	a := NewAxis(rs, marks.Const(AxisRight), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(10, 10, 40, 200)
	a.Layout.Measure(facet.MeasureContext{
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 60, H: 240}})
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	a.TickCount = marks.Const(5)

	cmds := a.Projection.Project(facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected projection commands for right axis")
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 60, 240)
	testkit.AssertGolden(t, surface, "axis_linear_right")
}

func epochMs(year int, month time.Month, day int) float64 {
	return float64(time.Date(year, month, day, 0, 0, 0, 0, time.UTC).UnixMilli())
}

func timeAxisFixture(t *testing.T, startMs, endMs float64, tickCount int, goldenName string) {
	t.Helper()
	domain := store.NewValueStore([2]float64{startMs, endMs})
	rng := store.NewValueStore([2]float64{0, 300})
	rs := reactive.NewTimeReactive(domain, rng)

	fonts := mustVizFontRegistry(t)
	a := NewAxis(rs, marks.Const(AxisBottom), fonts)
	facet.Attach(a, facet.AttachContext{Runtime: axisGoldenRuntime{}})

	bounds := gfx.RectFromXYWH(10, 10, 300, 30)
	a.Layout.Measure(facet.MeasureContext{}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 60}})
	a.Layout.Arrange(facet.ArrangeContext{}, bounds)
	a.TickCount = marks.Const(tickCount)

	cmds := a.Projection.Project(facet.ProjectionContext{
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatalf("expected projection commands for %s", goldenName)
	}

	surface := renderAxisGolden(t, cmds.Commands, bounds, 400, 60)
	testkit.AssertGolden(t, surface, goldenName)
}

func TestAxisGoldenTimeDays(t *testing.T) {
	// Jan 10–17, 2026 (7-day intra-month span, daily ticks)
	timeAxisFixture(t, epochMs(2026, time.January, 10), epochMs(2026, time.January, 17), 8, "axis_time_days")
}

func TestAxisGoldenTimeMonths(t *testing.T) {
	// Jan 1 – Mar 15, 2026 (crosses month boundary, ~73-day span)
	timeAxisFixture(t, epochMs(2026, time.January, 1), epochMs(2026, time.March, 15), 6, "axis_time_months")
}

func TestAxisGoldenTimeQuarters(t *testing.T) {
	// Jan 1 – Dec 31, 2026 (full year, quarterly ticks)
	timeAxisFixture(t, epochMs(2026, time.January, 1), epochMs(2026, time.December, 31), 5, "axis_time_quarters")
}

func TestAxisGoldenTimeYears(t *testing.T) {
	// Jan 1, 2020 – Jan 1, 2026 (6-year span, yearly ticks)
	timeAxisFixture(t, epochMs(2020, time.January, 1), epochMs(2026, time.January, 1), 6, "axis_time_years")
}

func renderAxisGolden(t *testing.T, cmds []gfx.Command, bounds gfx.Rect, w, h int) *testkit.MemorySurface {
	t.Helper()
	surface := testkit.NewMemorySurface(w, h)
	r := software.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	if err := r.Submit(&render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds},
			},
		},
	}); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	return surface
}

type axisGoldenRuntime struct{}

func (axisGoldenRuntime) Schedule(j job.AnyJob)                            {}
func (axisGoldenRuntime) CancelJob(id job.JobID)                           {}
func (axisGoldenRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (axisGoldenRuntime) FontRegistry() *text.FontRegistry                 { return mustVizFontRegistry(nil) }

var vizFontRegistry *text.FontRegistry

func mustVizFontRegistry(t *testing.T) *text.FontRegistry {
	if vizFontRegistry != nil {
		return vizFontRegistry
	}
	if t == nil {
		return nil
	}
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	rel := "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf"
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	path := filepath.Join(string(bytes.TrimSpace(out)), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font %q: %v", path, err)
	}
	if err := reg.LoadFontBytes(data, filepath.Base(rel)); err != nil {
		t.Fatalf("load font: %v", err)
	}
	vizFontRegistry = reg
	return reg
}
