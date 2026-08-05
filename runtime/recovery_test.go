package runtime

import (
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/log"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
)

// panicTickFacet panics on every tick invocation and counts calls.
type panicTickFacet struct {
	facet.Facet
	tick  facet.TickRole
	calls atomic.Int32
}

func (f *panicTickFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicTickFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicTickFacet) OnDetach()                        {}
func (f *panicTickFacet) OnActivate()                      {}
func (f *panicTickFacet) OnDeactivate()                    {}

func (f *panicTickFacet) init() {
	f.tick.OnTick = func(dt time.Duration) {
		f.calls.Add(1)
		panic("boom")
	}
	f.AddRole(&f.tick)
}

// countTickFacet counts tick invocations; used to prove frames still run.
type countTickFacet struct {
	facet.Facet
	tick  facet.TickRole
	ticks atomic.Int32
}

func (f *countTickFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *countTickFacet) OnAttach(ctx facet.AttachContext) {}
func (f *countTickFacet) OnDetach()                        {}
func (f *countTickFacet) OnActivate()                      {}
func (f *countTickFacet) OnDeactivate()                    {}

func (f *countTickFacet) init() {
	f.tick.OnTick = func(dt time.Duration) {
		f.ticks.Add(1)
	}
	f.AddRole(&f.tick)
}

// rearmTicks returns a phase-1 hook that re-arms the given tick roles every
// frame. tickFacets calls Reset after each tick, so without re-arming the
// roles go inactive after the first frame regardless of quarantine.
func rearmTicks(facets ...facet.FacetImpl) func(time.Duration) {
	return func(time.Duration) {
		for _, f := range facets {
			if tr := f.Base().TickRole(); tr != nil {
				tr.RequestTick()
			}
		}
	}
}

func TestTickCallback_Panic_QuarantinesFacet(t *testing.T) {
	panicFacet := &panicTickFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	countFacet := &countTickFacet{Facet: facet.NewFacet()}
	countFacet.init()

	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)
	root.AddChild(&countFacet.Facet)

	rt := mustRuntimeTree(t, &root)
	rt.RegisterPhase1TickHook(rearmTicks(panicFacet, countFacet))

	rt.RunOneFrame()
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after frame 1, want 1", got)
	}
	if got := panicFacet.calls.Load(); got != 1 {
		t.Fatalf("panicking facet tick invoked %d times on frame 1, want 1", got)
	}
	if got := countFacet.ticks.Load(); got != 1 {
		t.Fatalf("sibling facet tick invoked %d times on frame 1, want 1", got)
	}

	rt.RunOneFrame()
	if got := panicFacet.calls.Load(); got != 1 {
		t.Fatalf("panicking facet tick invoked %d times by frame 2; quarantined facet must be skipped", got)
	}
	if got := countFacet.ticks.Load(); got != 2 {
		t.Fatalf("sibling facet tick invoked %d times by frame 2, want 2 (frame still ran)", got)
	}
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after frame 2, want 1", got)
	}
}

// panicJobResultFacet registers a ProjectionRole whose OnJobResult panics.
type panicJobResultFacet struct {
	facet.Facet
	proj facet.ProjectionRole
}

func (f *panicJobResultFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicJobResultFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicJobResultFacet) OnDetach()                        {}
func (f *panicJobResultFacet) OnActivate()                      {}
func (f *panicJobResultFacet) OnDeactivate()                    {}

func (f *panicJobResultFacet) init() {
	f.proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList { return nil }
	f.proj.OnJobResult = func(result job.AnyResult) { panic("boom") }
	f.AddRole(&f.proj)
}

func TestJobResultCallback_Panic_QuarantinesFacet(t *testing.T) {
	mark := &panicJobResultFacet{Facet: facet.NewFacet()}
	mark.init()
	root := facet.NewFacet()
	root.AddChild(&mark.Facet)

	rt := mustRuntimeTree(t, &root)
	ownerID := mark.Base().ID()

	j := job.BindJob(uint64(ownerID), job.Job[int, int]{
		ID:       job.JobID(1),
		Priority: job.PriorityInteractive,
		Snapshot: job.NewSnapshot(0),
		Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
			return 42, nil
		},
	}, func(int) {})
	rt.Schedule(j)

	// Run frames until the completed job result is drained and its callback
	// panics (recovered by the runtime).
	deadline := time.Now().Add(2 * time.Second)
	for rt.PoisonedCount() == 0 && time.Now().Before(deadline) {
		rt.RunOneFrame()
	}
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after job result drain, want 1", got)
	}

	// The runtime must continue running frames after the poisoned job result.
	rt.RunOneFrame()
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after a further frame, want 1", got)
	}
	report := rt.poisonReports[ownerID]
	if report == nil {
		t.Fatal("poison report missing for job-result owner facet")
	}
	if report.Role != "job" {
		t.Fatalf("poison report role = %q, want \"job\"", report.Role)
	}
	if report.Stack == "" {
		t.Fatal("poison report stack is empty")
	}
}

// panicMeasureFacet panics in OnMeasure and counts calls.
type panicMeasureFacet struct {
	facet.Facet
	layout facet.LayoutRole
	calls  atomic.Int32
}

func (f *panicMeasureFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicMeasureFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicMeasureFacet) OnDetach()                        {}
func (f *panicMeasureFacet) OnActivate()                      {}
func (f *panicMeasureFacet) OnDeactivate()                    {}

func (f *panicMeasureFacet) init() {
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		f.calls.Add(1)
		panic("boom")
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, b gfx.Rect) {}
	f.AddRole(&f.layout)
}

// goodMeasureFacet measures to a fixed size.
type goodMeasureFacet struct {
	facet.Facet
	layout facet.LayoutRole
}

func (f *goodMeasureFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *goodMeasureFacet) OnAttach(ctx facet.AttachContext) {}
func (f *goodMeasureFacet) OnDetach()                        {}
func (f *goodMeasureFacet) OnActivate()                      {}
func (f *goodMeasureFacet) OnDeactivate()                    {}

func (f *goodMeasureFacet) init() {
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 40, H: 40}}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, b gfx.Rect) {}
	f.AddRole(&f.layout)
}

func TestMeasureCallback_Panic_QuarantinesFacet(t *testing.T) {
	panicFacet := &panicMeasureFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	goodFacet := &goodMeasureFacet{Facet: facet.NewFacet()}
	goodFacet.init()

	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)
	root.AddChild(&goodFacet.Facet)

	rt := mustRuntimeTree(t, &root)
	rt.start()

	if sz := rt.measureLayoutChild(panicFacet, layout.Loose(gfx.Size{W: 100, H: 100})); sz != (gfx.Size{}) {
		t.Fatalf("panicking child measured to %v, want zero size", sz)
	}
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after measure panic, want 1", got)
	}

	want := gfx.Size{W: 40, H: 40}
	if got := rt.measureLayoutChild(goodFacet, layout.Loose(gfx.Size{W: 100, H: 100})); got != want {
		t.Fatalf("sibling measured to %v, want %v", got, want)
	}

	rt.measureLayoutChild(panicFacet, layout.Loose(gfx.Size{W: 100, H: 100}))
	if got := panicFacet.calls.Load(); got != 1 {
		t.Fatalf("panicking facet measured %d times, want 1 (poisoned facet skipped)", got)
	}
}

// panicArrangeFacet panics in OnArrange and counts calls.
type panicArrangeFacet struct {
	facet.Facet
	layout facet.LayoutRole
	calls  atomic.Int32
}

func (f *panicArrangeFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicArrangeFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicArrangeFacet) OnDetach()                        {}
func (f *panicArrangeFacet) OnActivate()                      {}
func (f *panicArrangeFacet) OnDeactivate()                    {}

func (f *panicArrangeFacet) init() {
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 100, H: 100}}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, b gfx.Rect) {
		f.calls.Add(1)
		panic("boom")
	}
	f.AddRole(&f.layout)
}

func TestPoison_SubtreeQuarantined(t *testing.T) {
	parent := &panicArrangeFacet{Facet: facet.NewFacet()}
	parent.init()
	child1 := facet.NewFacet()
	child2 := facet.NewFacet()
	parent.AddChild(&child1)
	parent.AddChild(&child2)

	root := facet.NewFacet()
	root.AddChild(&parent.Facet)

	rt := mustRuntimeTree(t, &root)
	rt.start()

	rt.arrangeLayoutChild(parent, gfx.RectFromXYWH(0, 0, 100, 100))

	if got := parent.calls.Load(); got != 1 {
		t.Fatalf("parent arrange invoked %d times, want 1", got)
	}
	if got := rt.PoisonedCount(); got != 3 {
		t.Fatalf("PoisonedCount() = %d, want 3 (parent + 2 children)", got)
	}
	for _, f := range []facet.FacetImpl{parent, &child1, &child2} {
		if !rt.isPoisoned(f.Base().ID()) {
			t.Fatalf("expected facet %d to be quarantined", f.Base().ID())
		}
	}

	// A second arrange of the quarantined subtree is skipped entirely.
	rt.arrangeLayoutChild(parent, gfx.RectFromXYWH(0, 0, 100, 100))
	if got := parent.calls.Load(); got != 1 {
		t.Fatalf("parent arrange invoked %d times after quarantine, want 1", got)
	}
}

// recordingLogger counts Warn calls so one-shot logging can be asserted.
type recordingLogger struct {
	warnCount atomic.Int32
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(string, ...any)  { l.warnCount.Add(1) }
func (l *recordingLogger) Error(string, ...any) {}

var _ log.Logger = (*recordingLogger)(nil)

func TestPoison_OneShot_Logging(t *testing.T) {
	logger := &recordingLogger{}
	panicFacet := &panicTickFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)

	cfg := DefaultConfig()
	cfg.Logger = logger
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.RegisterPhase1TickHook(rearmTicks(panicFacet))

	for i := 0; i < 10; i++ {
		rt.RunOneFrame()
	}
	if got := logger.warnCount.Load(); got != 1 {
		t.Fatalf("quarantine warning logged %d times across 10 frames, want exactly 1", got)
	}
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d, want 1", got)
	}
}

// panicProjectionFacet panics in OnProject and counts calls.
type panicProjectionFacet struct {
	facet.Facet
	proj     facet.ProjectionRole
	projects atomic.Int32
}

func (f *panicProjectionFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *panicProjectionFacet) OnAttach(ctx facet.AttachContext) {}
func (f *panicProjectionFacet) OnDetach()                        {}
func (f *panicProjectionFacet) OnActivate()                      {}
func (f *panicProjectionFacet) OnDeactivate()                    {}

func (f *panicProjectionFacet) init() {
	f.proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projects.Add(1)
		panic("boom")
	}
	f.AddRole(&f.proj)
}

// goodProjectionFacet projects a command and counts calls.
type goodProjectionFacet struct {
	facet.Facet
	proj     facet.ProjectionRole
	projects atomic.Int32
}

func (f *goodProjectionFacet) Base() *facet.Facet               { f.BindImpl(f); return &f.Facet }
func (f *goodProjectionFacet) OnAttach(ctx facet.AttachContext) {}
func (f *goodProjectionFacet) OnDetach()                        {}
func (f *goodProjectionFacet) OnActivate()                      {}
func (f *goodProjectionFacet) OnDeactivate()                    {}

func (f *goodProjectionFacet) init() {
	f.proj.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projects.Add(1)
		list := &gfx.CommandList{}
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(0, 0, 10, 10),
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 255, 255)),
		})
		return list
	}
	f.AddRole(&f.proj)
}

func TestRuntime_QuarantinesPanickingProjectionFacet(t *testing.T) {
	panicFacet := &panicProjectionFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	goodFacet := &goodProjectionFacet{Facet: facet.NewFacet()}
	goodFacet.init()
	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)
	root.AddChild(&goodFacet.Facet)
	rt := mustRuntimeTree(t, &root)

	rt.RunOneFrame()
	if got := rt.PoisonedCount(); got != 1 {
		t.Fatalf("PoisonedCount() = %d after frame 1, want 1", got)
	}
	if got := panicFacet.projects.Load(); got != 1 {
		t.Fatalf("panicking facet projected %d times on frame 1, want 1", got)
	}
	if got := goodFacet.projects.Load(); got != 1 {
		t.Fatalf("good sibling projected %d times on frame 1, want 1", got)
	}

	// Re-project the whole tree; the quarantined facet must be pruned while
	// the good sibling continues to render.
	rt.MarkTreeDirty(rt.root, facet.DirtyProjection)
	rt.RunOneFrame()
	if got := panicFacet.projects.Load(); got != 1 {
		t.Fatalf("panicking facet projected %d times by frame 2, want 1 (quarantined facet pruned)", got)
	}
	if got := goodFacet.projects.Load(); got != 2 {
		t.Fatalf("good sibling projected %d times by frame 2, want 2", got)
	}
}

func TestRecoveryDisabled_ReRaises_WithAttribution(t *testing.T) {
	panicFacet := &panicTickFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)

	cfg := DefaultConfig()
	cfg.RecoveryDisabled = true
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	defer rt.Shutdown()
	rt.RegisterPhase1TickHook(rearmTicks(panicFacet))

	ownerID := panicFacet.Base().ID()
	panicked := false
	func() {
		defer func() {
			if r := recover(); r != nil {
				panicked = true
				if r != "boom" {
					t.Fatalf("re-raised panic = %v, want boom", r)
				}
			}
		}()
		rt.RunOneFrame()
	}()
	if !panicked {
		t.Fatal("expected the panic to be re-raised with RecoveryDisabled")
	}

	// The report must be captured before the re-raise so attribution survives.
	report := rt.poisonReports[ownerID]
	if report == nil {
		t.Fatal("poison report not captured before the re-raise")
	}
	if report.Stack == "" {
		t.Fatal("poison report stack is empty in RecoveryDisabled mode")
	}
	if report.Role != "tick" {
		t.Fatalf("poison report role = %q, want \"tick\"", report.Role)
	}
}

// recordingPoisonSink implements DiagnosticsHook and opts into per-poison
// events by also implementing OnFacetPoisoned (structural poisoningSink
// satisfaction).
type recordingPoisonSink struct {
	mu      sync.Mutex
	last    diagnostics.FrameStats
	reports []diagnostics.PoisonReport
}

func (r *recordingPoisonSink) OnFrame(stats diagnostics.FrameStats) {
	r.mu.Lock()
	r.last = stats
	r.mu.Unlock()
}

func (r *recordingPoisonSink) OnFacetPoisoned(report diagnostics.PoisonReport) {
	r.mu.Lock()
	r.reports = append(r.reports, report)
	r.mu.Unlock()
}

func (r *recordingPoisonSink) poisonReports() []diagnostics.PoisonReport {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]diagnostics.PoisonReport(nil), r.reports...)
}

func TestPoison_ReflectedInFrameStats(t *testing.T) {
	hook := &recordingFrameStats{}
	panicFacet := &panicTickFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)

	cfg := DefaultConfig()
	cfg.DiagnosticsHook = hook
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.RegisterPhase1TickHook(rearmTicks(panicFacet))

	rt.RunOneFrame()
	if got := hook.last().PoisonedFacets; got != 1 {
		t.Fatalf("PoisonedFacets = %d after frame 1, want 1", got)
	}
	rt.RunOneFrame()
	if got := hook.last().PoisonedFacets; got != 1 {
		t.Fatalf("PoisonedFacets = %d after frame 2, want 1 (no double-count)", got)
	}
}

func TestOnFacetPoisoned_HookFires(t *testing.T) {
	hook := &recordingPoisonSink{}
	panicFacet := &panicTickFacet{Facet: facet.NewFacet()}
	panicFacet.init()
	root := facet.NewFacet()
	root.AddChild(&panicFacet.Facet)

	cfg := DefaultConfig()
	cfg.DiagnosticsHook = hook
	cfg.LayerRegistry = testLayerRegistry(t)
	rt, err := New(cfg, nil, nil, &backendFixture{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	rt.RegisterPhase1TickHook(rearmTicks(panicFacet))

	ownerID := panicFacet.Base().ID()
	rt.RunOneFrame()

	reports := hook.poisonReports()
	if len(reports) != 1 {
		t.Fatalf("OnFacetPoisoned called %d times, want 1", len(reports))
	}
	report := reports[0]
	if report.FacetID != ownerID {
		t.Fatalf("report FacetID = %d, want %d", report.FacetID, ownerID)
	}
	if report.Role != "tick" {
		t.Fatalf("report Role = %q, want \"tick\"", report.Role)
	}
	if report.Panic != "boom" {
		t.Fatalf("report Panic = %q, want \"boom\"", report.Panic)
	}
	if report.Stack == "" {
		t.Fatal("report Stack is empty")
	}
	if report.FirstSeen.IsZero() {
		t.Fatal("report FirstSeen is zero")
	}
}
