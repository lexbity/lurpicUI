package runtime

import (
	"sort"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
)

// tickDirtyFacet invalidates its projection on every tick so a frame always
// does real projection work (a realistic dirty workload for the A/B test).
type tickDirtyFacet struct {
	facet.Facet
	layout facet.LayoutRole
	proj   facet.ProjectionRole
	tick   facet.TickRole
	store  *store.ValueStore[string]
}

func newTickDirtyFacet() *tickDirtyFacet {
	f := &tickDirtyFacet{store: store.NewValueStore("idle")}
	f.Facet = facet.NewFacet()
	f.layout = facet.LayoutRole{
		OnMeasure: func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.MaxSize}
		},
		OnArrange: func(_ facet.ArrangeContext, bounds gfx.Rect) { f.layout.ArrangedBounds = bounds },
	}
	f.proj = facet.ProjectionRole{
		OnProject: func(_ facet.ProjectionContext) *gfx.CommandList {
			// A few commands per facet keeps the projection cost the dominant
			// term, matching a real exhibit's ratio (the sink's per-frame
			// copies are then a small fraction of the frame work).
			return &gfx.CommandList{Commands: []gfx.Command{
				gfx.FillRect{Rect: gfx.Rect{Max: gfx.Point{X: 640, Y: 480}}, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(1, 2, 3, 255))},
				gfx.StrokeRect{Rect: gfx.Rect{Max: gfx.Point{X: 640, Y: 480}}, Stroke: gfx.DefaultStroke(1), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(1, 2, 3, 255))},
			}}
		},
	}
	f.tick = facet.TickRole{OnTick: func(dt time.Duration) {
		if f.store != nil {
			f.store.Set("tick")
		}
	}}
	f.AddRole(&f.layout)
	f.AddRole(&f.proj)
	f.AddRole(&f.tick)
	facet.Store(facet.Subscribe(f), &f.store.OnChange, f.store.Version, func(signal.Change[string]) {
		f.InvalidateWithSource(facet.DirtyProjection, "tick.store")
	})
	return f
}

func (f *tickDirtyFacet) Base() *facet.Facet { f.BindImpl(f); return &f.Facet }

// dirtyWorkloadRoot builds a tree of N store-driven facets so each frame's
// projection pass forks (the tree exceeds the fork threshold) and re-projects
// every dirty leaf.
func dirtyWorkloadRoot(n int) facet.FacetImpl {
	root := facet.NewFacet()
	root.BindImpl(&root)
	for i := 0; i < n; i++ {
		root.AddChildRuntime(newTickDirtyFacet().Base())
	}
	return &root
}

// measureProjectMedian runs warm frames + samples sampled frames and returns
// the median ProjectDuration and the p90 (both robust to CI noise).
func measureProjectMedian(t *testing.T, root facet.FacetImpl, sink DiagnosticsHook, warm, samples int) (median, p90 time.Duration) {
	t.Helper()
	rt := mustRuntimeTree(t, root)
	if sink != nil {
		rt.EnableDiagnostics(sink)
	}
	for i := 0; i < warm; i++ {
		rt.RunOneFrame()
	}
	times := make([]time.Duration, 0, samples)
	for i := 0; i < samples; i++ {
		rt.RunOneFrame()
		times = append(times, rt.LastFrameStats().ProjectDuration)
	}
	sorted := append([]time.Duration(nil), times...)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
	median = sorted[len(sorted)/2]
	p90 = sorted[int(float64(len(sorted))*0.90)]
	return median, p90
}

// TestRuntime_DirtySnapshotSinkFrameNeutrality is the A/B proof that the sink
// does not perturb the observed frame (NFR-introspect-neutral): a fixed dirty
// workload projected with and without the sink must have statistically
// equivalent projection-time distributions.
func TestRuntime_DirtySnapshotSinkFrameNeutrality(t *testing.T) {
	const (
		facets  = 48 // exceeds the projection fork threshold; projection-dominant
		warm    = 80
		samples = 400
		// Caps are float64 to avoid int64 truncation when scaled by durations.
		medianCap float64 = 1.10 // with-sink median within 10% of baseline
		p90Cap    float64 = 1.25 // no tail regression beyond 25%
	)
	baselineMedian, baselineP90 := measureProjectMedian(t, dirtyWorkloadRoot(facets), nil, warm, samples)

	sink := &recordingDirtySink{}
	probeMedian, probeP90 := measureProjectMedian(t, dirtyWorkloadRoot(facets), sink, warm, samples)

	if probeMedian > time.Duration(float64(baselineMedian)*medianCap) {
		t.Fatalf("sink perturbed projection: median with-sink=%v vs baseline=%v (cap %v)", probeMedian, baselineMedian, time.Duration(float64(baselineMedian)*medianCap))
	}
	if probeP90 > time.Duration(float64(baselineP90)*p90Cap) {
		t.Fatalf("sink added a tail: p90 with-sink=%v vs baseline=%v (cap %v)", probeP90, baselineP90, time.Duration(float64(baselineP90)*p90Cap))
	}
	if got := len(sink.snapshots); got < samples/2 {
		t.Fatalf("sink recorded only %d snapshots, want ~%d", got, samples)
	}
}
