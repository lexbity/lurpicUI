package runtime

import (
	"errors"
	"image/color"
	"strings"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/syncutil"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

type stubBackend struct {
	submitErr error
}

type recordingLogger struct {
	warnings []string
}

func (l *recordingLogger) Debug(string, ...any) {}
func (l *recordingLogger) Info(string, ...any)  {}
func (l *recordingLogger) Warn(msg string, args ...any) {
	l.warnings = append(l.warnings, msg)
}
func (l *recordingLogger) Error(string, ...any) {}

func (s *stubBackend) Initialize(surface render.Surface) error { return nil }
func (s *stubBackend) Submit(frame *render.Frame) error        { return s.submitErr }
func (s *stubBackend) Resize(width, height int) error          { return nil }
func (s *stubBackend) Destroy()                                {}

type recordingBackend struct {
	last        *render.Frame
	submitCount int
}

type countingDiagHook struct {
	count int
}

func (h *countingDiagHook) OnFrame(stats diagnostics.FrameStats) {
	h.count++
}

func (r *recordingBackend) Initialize(surface render.Surface) error { return nil }
func (r *recordingBackend) Submit(frame *render.Frame) error {
	r.submitCount++
	r.last = frame
	return nil
}
func (r *recordingBackend) Resize(width, height int) error { return nil }
func (r *recordingBackend) Destroy()                       {}

type runtimeTestFacet struct {
	facet.Facet
	attachCount   int
	activateCount int
	detachOrder   *[]string
	name          string
}

func (f *runtimeTestFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}
func (f *runtimeTestFacet) OnAttach(ctx facet.AttachContext) { f.attachCount++ }
func (f *runtimeTestFacet) OnDetach() {
	if f.detachOrder != nil {
		*f.detachOrder = append(*f.detachOrder, f.name)
	}
}
func (f *runtimeTestFacet) OnActivate()   { f.activateCount++ }
func (f *runtimeTestFacet) OnDeactivate() {}

type runtimeRenderFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	name   string
}

type runtimeLayerFacet struct {
	facet.Facet
	layout     facet.LayoutRole
	specs      []layout.LayerSpec
	anchors    layout.AnchorSet
	onExport   func(ctx layout.AnchorExportContext)
	exportHits int
}

type runtimeProjectedFacet struct {
	facet.Facet
	layout    facet.LayoutRole
	worldPos  gfx.Point
	worldSize gfx.Size
}

type spyPolicy struct {
	measure func(children []layout.ChildNode, constraints gfx.Size) gfx.Size
	arrange func(children []layout.ChildNode, layer layout.ResolvedLayer)
}

func (p *spyPolicy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	if p != nil && p.measure != nil {
		return p.measure(children, constraints)
	}
	return gfx.Size{}
}

func (p *spyPolicy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p != nil && p.arrange != nil {
		p.arrange(children, layer)
	}
}

type runtimeFocusFacet struct {
	facet.Facet
	focus facet.FocusRole
	text  facet.TextRole
}

type layoutCountLeaf struct {
	facet.Facet
	layout facet.LayoutRole

	measureCount int
	arrangeCount int
	size         gfx.Size
}

type runtimeJobFacet struct {
	facet.Facet
	projection facet.ProjectionRole
	lastResult job.AnyResult
}

type runtimeSubscriptionFacet struct {
	facet.Facet
	store       *store.ValueStore[int]
	changeCount int
}

type projectionJobFacet struct {
	facet.Facet
	projection facet.ProjectionRole

	rt             *Runtime
	scheduled      bool
	projectCalls   int
	commitCount    int
	jobResultCount int
	commitValue    int
	dirtySeen      bool
	jobStarted     chan struct{}
	jobDone        chan struct{}
	jobRelease     chan struct{}
	lastResult     job.AnyResult
	versionSource  *store.ValueStore[int]
}

type projectionRuntimeFacet struct {
	facet.Facet
	projection facet.ProjectionRole
	scheduled  bool
}

func (f *runtimeLayerFacet) OnLayerSpecs() []layout.LayerSpec {
	return f.specs
}

func (f *runtimeLayerFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *runtimeLayerFacet) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	f.exportHits++
	if f.onExport != nil {
		f.onExport(ctx)
	}
	if len(f.anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(f.anchors))
	for id, pos := range f.anchors {
		out[id] = pos
	}
	return out
}

func (f *runtimeProjectedFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func (f *runtimeProjectedFacet) WorldPosition() gfx.Point {
	return f.worldPos
}

func (f *runtimeProjectedFacet) WorldSize() gfx.Size {
	return f.worldSize
}

func newRuntimeRenderFacet(name string, bounds gfx.Rect, fill color.RGBA) *runtimeRenderFacet {
	f := &runtimeRenderFacet{Facet: facet.NewFacet(), name: name}
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	f.layout.OnArrange = func(b gfx.Rect) {
		f.layout.ArrangedBounds = b
	}
	f.render.OnCollect = func(list *gfx.CommandList, b gfx.Rect) {
		list.Add(gfx.FillRect{Rect: b, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(fill.R, fill.G, fill.B, fill.A))})
	}
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
}

func newRuntimeFocusFacet(tabIndex int) *runtimeFocusFacet {
	f := &runtimeFocusFacet{Facet: facet.NewFacet()}
	f.focus.Focusable = func() bool { return true }
	f.focus.TabIndex = tabIndex
	f.AddRole(&f.focus)
	return f
}

func newLayoutCountLeaf(size gfx.Size) *layoutCountLeaf {
	leaf := &layoutCountLeaf{Facet: facet.NewFacet(), size: size}
	leaf.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		leaf.measureCount++
		return leaf.size
	}
	leaf.layout.OnArrange = func(bounds gfx.Rect) {
		leaf.arrangeCount++
		leaf.layout.ArrangedBounds = bounds
	}
	leaf.AddRole(&leaf.layout)
	return leaf
}

func newRuntimeJobFacet() *runtimeJobFacet {
	f := &runtimeJobFacet{Facet: facet.NewFacet()}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		return nil
	}
	f.projection.OnJobResult = func(result job.AnyResult) {
		f.lastResult = result
	}
	f.AddRole(&f.projection)
	return f
}

func newRuntimeSubscriptionFacet(s *store.ValueStore[int]) *runtimeSubscriptionFacet {
	f := &runtimeSubscriptionFacet{Facet: facet.NewFacet(), store: s}
	return f
}

func (f *runtimeSubscriptionFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}
func (f *runtimeSubscriptionFacet) OnAttach(ctx facet.AttachContext) {
	facet.Store(facet.Subscribe(f), &f.store.OnChange, f.store.Version, func(signal.Change[int]) {
		f.changeCount++
	})
}
func (f *runtimeSubscriptionFacet) OnDetach()     {}
func (f *runtimeSubscriptionFacet) OnActivate()   {}
func (f *runtimeSubscriptionFacet) OnDeactivate() {}

func newProjectionJobFacet() *projectionJobFacet {
	f := &projectionJobFacet{
		Facet:      facet.NewFacet(),
		jobStarted: make(chan struct{}),
		jobDone:    make(chan struct{}),
		jobRelease: make(chan struct{}),
	}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		f.projectCalls++
		if !f.scheduled {
			f.scheduled = true
			snap := job.NewSnapshot(5, store.Version(0))
			if f.versionSource != nil {
				snap = job.NewSnapshot(5, f.versionSource.Version())
				snap = job.BindCurrentVersions(snap, func() []store.Version {
					return []store.Version{f.versionSource.Version()}
				})
			}
			f.rt.Schedule(job.BindJob(uint64(f.ID()), job.Job[int, int]{
				ID:       1,
				Priority: job.PriorityBackground,
				Snapshot: snap,
				Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
					defer close(f.jobDone)
					close(f.jobStarted)
					<-f.jobRelease
					return snap.Data * 2, nil
				},
			}, func(v int) {
				f.commitCount++
				f.commitValue = v
			}))
		}
		return &gfx.CommandList{}
	}
	f.projection.OnJobResult = func(result job.AnyResult) {
		f.jobResultCount++
		f.lastResult = result
		if f.rt != nil && f.ID() != 0 {
			f.dirtySeen = f.rt.dirtyFacets[f.ID()]&facet.DirtyProjection != 0
		}
		f.Base().InvalidateWithSource(facet.DirtyProjection, "OnJobResult")
	}
	f.AddRole(&f.projection)
	return f
}

func newProjectionRuntimeFacet() *projectionRuntimeFacet {
	f := &projectionRuntimeFacet{Facet: facet.NewFacet()}
	f.projection.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		if !f.scheduled {
			f.scheduled = true
			ctx.Runtime.Schedule(job.BindJob(uint64(f.ID()), job.Job[int, int]{
				ID:       2,
				Priority: job.PriorityBackground,
				Snapshot: job.NewSnapshot(1),
				Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
					return snap.Data + 1, nil
				},
			}, func(int) {}))
		}
		return &gfx.CommandList{}
	}
	f.AddRole(&f.projection)
	return f
}

func newRuntimeTestTree() (*runtimeTestFacet, *runtimeTestFacet, *runtimeTestFacet) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	child := &runtimeTestFacet{Facet: facet.NewFacet(), name: "child"}
	leaf := &runtimeTestFacet{Facet: facet.NewFacet(), name: "leaf"}
	root.AddChild(&child.Facet)
	child.AddChild(&leaf.Facet)
	return root, child, leaf
}

func newRuntimeRenderTree() (*runtimeRenderFacet, *runtimeRenderFacet) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 200, 200), color.RGBA{R: 10, G: 10, B: 10, A: 255})
	root.layout.OnArrange = func(bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
		for i, childBase := range root.Base().Children() {
			if childBase == nil {
				continue
			}
			childRole := childBase.LayoutRole()
			if childRole == nil {
				continue
			}
			offset := float32(i * 30)
			childRole.Arrange(gfx.RectFromXYWH(bounds.Min.X+offset, bounds.Min.Y+offset, 40, 40))
		}
	}
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 40, 40), color.RGBA{R: 200, G: 0, B: 0, A: 255})
	return root, child
}

func newRuntimeLayerTree() (*runtimeLayerFacet, *runtimeRenderFacet) {
	root := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		specs: []layout.LayerSpec{
			{
				ID:          1,
				Placement:   layout.PlacementFree,
				Measurement: layout.MeasureNonStructural,
				CoordLimits: layout.CoordLimits{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)},
			},
		},
	}
	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: 100, H: 100}
	}
	root.layout.OnArrange = func(bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
	}
	root.AddRole(&root.layout)
	child := newRuntimeRenderFacet("child", gfx.RectFromXYWH(0, 0, 20, 10), color.RGBA{R: 200, G: 0, B: 0, A: 255})
	return root, child
}

func newRuntimeAnchorTree() (*runtimeLayerFacet, *runtimeLayerFacet, *runtimeTestFacet) {
	root := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		specs: []layout.LayerSpec{
			{
				ID:          1,
				Placement:   layout.PlacementStack,
				Measurement: layout.MeasureStructural,
			},
			{
				ID:          2,
				Placement:   layout.PlacementAnchor,
				Measurement: layout.MeasureNonStructural,
			},
		},
	}
	root.AddRole(&root.layout)
	exporter := &runtimeLayerFacet{
		Facet:   facet.NewFacet(),
		anchors: layout.AnchorSet{"mark": gfx.Point{X: 10, Y: 20}},
	}
	exporter.layout.OnMeasure = func(c facet.Constraints) gfx.Size { return gfx.Size{} }
	exporter.layout.OnArrange = func(bounds gfx.Rect) { exporter.layout.ArrangedBounds = bounds }
	exporter.AddRole(&exporter.layout)
	child := &runtimeTestFacet{Facet: facet.NewFacet(), name: "anchor-child"}
	root.Base()
	exporter.Base()
	child.Base()
	root.AddChild(&exporter.Facet)
	root.AddChild(&child.Facet)
	return root, exporter, child
}

func newRuntimeAnchorPlacementTree() (*runtimeLayerFacet, *runtimeLayerFacet, *runtimeRenderFacet) {
	root := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		specs: []layout.LayerSpec{
			{
				ID:          1,
				Placement:   layout.PlacementStack,
				Measurement: layout.MeasureStructural,
			},
			{
				ID:          2,
				Placement:   layout.PlacementAnchor,
				Measurement: layout.MeasureNonStructural,
			},
		},
	}
	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: 300, H: 300}
	}
	root.layout.OnArrange = func(bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
	}
	root.AddRole(&root.layout)

	exporter := &runtimeLayerFacet{
		Facet:   facet.NewFacet(),
		anchors: layout.AnchorSet{"mark": gfx.Point{X: 100, Y: 200}},
	}
	exporter.layout.OnMeasure = func(c facet.Constraints) gfx.Size { return gfx.Size{} }
	exporter.layout.OnArrange = func(bounds gfx.Rect) { exporter.layout.ArrangedBounds = bounds }
	exporter.AddRole(&exporter.layout)

	child := newRuntimeRenderFacet("anchor-child", gfx.RectFromXYWH(0, 0, 50, 30), color.RGBA{R: 0, G: 128, B: 255, A: 255})
	child.layout.OnArrange = func(bounds gfx.Rect) {
		child.layout.ArrangedBounds = bounds
	}
	return root, exporter, child
}

func newRuntimeProjectedTree() (*runtimeLayerFacet, *runtimeProjectedFacet) {
	root := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		specs: []layout.LayerSpec{
			{
				ID:          1,
				Placement:   layout.PlacementStack,
				Measurement: layout.MeasureStructural,
			},
			{
				ID:          2,
				Placement:   layout.PlacementProjected,
				Measurement: layout.MeasureNonStructural,
				CoordSpace:  layout.CoordViewport,
			},
		},
	}
	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: 400, H: 400}
	}
	root.layout.OnArrange = func(bounds gfx.Rect) {
		root.layout.ArrangedBounds = bounds
	}
	root.AddRole(&root.layout)

	child := &runtimeProjectedFacet{
		Facet:     facet.NewFacet(),
		worldPos:  gfx.Point{X: 100, Y: 200},
		worldSize: gfx.Size{W: 50, H: 30},
	}
	child.layout.OnMeasure = func(c facet.Constraints) gfx.Size { return gfx.Size{W: 50, H: 30} }
	child.layout.OnArrange = func(bounds gfx.Rect) {
		child.layout.ArrangedBounds = bounds
	}
	child.AddRole(&child.layout)

	root.Base()
	child.Base()
	return root, child
}

func setupAnchorExportRuntime(t *testing.T, root *runtimeLayerFacet, exporter *runtimeLayerFacet, child *runtimeTestFacet) *Runtime {
	t.Helper()
	rt := mustRuntimeTree(t, root)
	rt.layerStates[root.ID()] = &resolvedLayerSet{
		specs: append([]layout.LayerSpec(nil), root.specs...),
		layers: []layout.ResolvedLayer{
			{
				LayerID:     1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				Transform:   gfx.Identity(),
				ClipRect:    gfx.Rect{},
				CoordLimits: layout.CoordLimits{},
				HitPolicy:   layout.HitNormal,
				RenderOrder: 0,
				CoordSpace:  layout.CoordParentLayout,
			},
			{
				LayerID:     2,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				Transform:   gfx.Identity(),
				ClipRect:    gfx.Rect{},
				CoordLimits: layout.CoordLimits{},
				HitPolicy:   layout.HitNormal,
				RenderOrder: 0,
				CoordSpace:  layout.CoordParentLayout,
			},
		},
	}
	rt.childAttachments[exporter.ID()] = layout.ChildAttachment{LayerID: 1}
	rt.childAttachments[child.ID()] = layout.ChildAttachment{
		LayerID: 2,
		Placement: layout.PlacementHints{
			AnchorRef: "mark",
		},
	}
	return rt
}

type assemblyLayerResolverStub struct {
	layers      map[facet.FacetID]facet.ProjectionLayer
	attachments map[facet.FacetID]layout.ChildAttachment
}

func (s assemblyLayerResolverStub) ResolveProjectionLayer(id facet.FacetID) (facet.ProjectionLayer, bool) {
	if s.layers == nil {
		return facet.ProjectionLayer{}, false
	}
	layer, ok := s.layers[id]
	return layer, ok
}

func (s assemblyLayerResolverStub) ResolveChildAttachment(id facet.FacetID) (layout.ChildAttachment, bool) {
	if s.attachments == nil {
		return layout.ChildAttachment{}, false
	}
	attachment, ok := s.attachments[id]
	return attachment, ok
}

func TestRuntimeNew_nil_fontregistry_errors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.FontRegistry = nil
	if _, err := New(cfg, nil, nil, &stubBackend{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRuntimeNew_zero_targetfps_errors(t *testing.T) {
	cfg := DefaultConfig()
	cfg.TargetFPS = 0
	if _, err := New(cfg, nil, nil, &stubBackend{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error")
	}
}

func TestRuntimeNew_nil_root_errors(t *testing.T) {
	cfg := DefaultConfig()
	if _, err := New(cfg, nil, nil, &stubBackend{}, nil); err == nil {
		t.Fatal("expected error")
	}
}

func TestFrameTimer_wait_respects_target_period(t *testing.T) {
	timer := NewFrameTimer(20)
	start := timer.Wait()
	_ = start
	before := time.Now()
	_ = timer.Wait()
	elapsed := time.Since(before)
	if elapsed < 40*time.Millisecond {
		t.Fatalf("elapsed = %v", elapsed)
	}
}

func TestFrameTimer_request_frame_returns_immediately(t *testing.T) {
	timer := NewFrameTimer(60)
	timer.RequestFrame()
	before := time.Now()
	_ = timer.Wait()
	if time.Since(before) > 20*time.Millisecond {
		t.Fatal("expected immediate wake")
	}
}

func TestFrameTimer_request_frame_noop_if_pending(t *testing.T) {
	timer := NewFrameTimer(60)
	timer.RequestFrame()
	timer.RequestFrame()
	_ = timer.Wait()
}

func TestRenderPipeline_submit_blocks_on_full(t *testing.T) {
	pipe := newRenderPipeline(&stubBackend{})
	pipe.Submit(&render.Frame{})
	done := make(chan struct{})
	go func() {
		pipe.Submit(&render.Frame{})
		close(done)
	}()
	select {
	case <-done:
		t.Fatal("second submit should block")
	case <-time.After(20 * time.Millisecond):
	}
	<-pipe.handoffCh
	select {
	case <-done:
	case <-time.After(20 * time.Millisecond):
		t.Fatal("expected second submit to unblock after drain")
	}
}

func TestRenderPipeline_fatalch_readable(t *testing.T) {
	pipe := newRenderPipeline(&stubBackend{})
	err := errors.New("boom")
	pipe.fatalCh <- err
	select {
	case got := <-pipe.fatalCh:
		if got == nil || got.Error() != "boom" {
			t.Fatalf("got %v", got)
		}
	default:
		t.Fatal("expected readable fatal channel")
	}
}

func TestRuntime_start_registers_thread(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if !syncutil.OnRuntimeThread() {
		t.Fatal("expected runtime thread")
	}
	rt.Shutdown()
}

func TestRuntime_shutdown_clean(t *testing.T) {
	rt := mustRuntime(t)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.Shutdown()
}

func TestRuntime_shutdown_disposes_tree_bottomup(t *testing.T) {
	order := []string{}
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root", detachOrder: &order}
	rt := mustRuntimeTree(t, root)
	rt.disposeTree(root)
	if len(order) != 1 || order[0] != "root" {
		t.Fatalf("order = %#v", order)
	}
}

func TestRuntime_attachtree_calls_onattach(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	if root.attachCount != 1 {
		t.Fatalf("attach count = %d", root.attachCount)
	}
}

func TestRuntime_subscribe_builder_integrates_with_lifecycle(t *testing.T) {
	s := store.NewValueStore(1)
	root := newRuntimeSubscriptionFacet(s)
	rt := mustRuntimeTree(t, root)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}

	if got := root.Subs().Len(); got != 1 {
		t.Fatalf("subscriptions after attach = %d", got)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 0 {
		t.Fatalf("subscribed versions after attach = %#v", got)
	}

	s.Set(2)
	rt.RunOneFrame()

	if root.changeCount != 1 {
		t.Fatalf("changeCount = %d", root.changeCount)
	}
	if got := root.SubscribedVersions(); len(got) != 1 || got[0] != 1 {
		t.Fatalf("subscribed versions after update = %#v", got)
	}

	rt.Shutdown()
	if s.OnChange.HasSubscribers() {
		t.Fatal("expected subscriptions to be released on shutdown")
	}
}

func TestRuntime_activatetree_calls_onactivate(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rt := mustRuntimeTree(t, root)
	rt.attachTree(root)
	rt.activateTree(root)
	if root.activateCount != 1 {
		t.Fatalf("activate count = %d", root.activateCount)
	}
}

func TestRuntime_marktreedirty_sets_flags(t *testing.T) {
	root, child, leaf := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	rt.markTreeDirty(root, facet.DirtyAll)
	if root.DirtyFlags() != facet.DirtyAll || child.DirtyFlags() != facet.DirtyAll || leaf.DirtyFlags() != facet.DirtyAll {
		t.Fatalf("dirty flags = %#v %#v %#v", root.DirtyFlags(), child.DirtyFlags(), leaf.DirtyFlags())
	}
}

func TestRuntime_assembleFrame_prepends_transform(t *testing.T) {
	rt := mustRuntime(t)
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{
				FacetID:   1,
				Bounds:    gfx.RectFromXYWH(0, 0, 10, 10),
				Transform: gfx.Translation(12, 18),
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 0, 0, 255))},
				}},
			},
		},
	}
	frame := rt.assembleFrame(output, map[facet.FacetID]facet.DirtyFlags{1: facet.DirtyAll})
	if frame == nil || len(frame.RenderBatchs) != 1 {
		t.Fatalf("frame = %#v", frame)
	}
	cmds := frame.RenderBatchs[0].Commands.Commands
	if len(cmds) < 3 {
		t.Fatalf("commands = %#v", cmds)
	}
	if _, ok := cmds[0].(gfx.PushTransform); !ok {
		t.Fatalf("first command = %T", cmds[0])
	}
	if _, ok := cmds[len(cmds)-1].(gfx.PopTransform); !ok {
		t.Fatalf("last command = %T", cmds[len(cmds)-1])
	}
	if frame.RenderBatchs[0].CommandHash == 0 {
		t.Fatal("expected command hash")
	}
}

func TestRuntime_assembleFrame_dirty_regions(t *testing.T) {
	rt := mustRuntime(t)
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10)},
			{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 10, 10, 10)},
		},
	}
	frame := rt.assembleFrame(output, map[facet.FacetID]facet.DirtyFlags{2: facet.DirtyProjection})
	if got := len(frame.DirtyRegions); got != 1 {
		t.Fatalf("dirty regions = %d, want 1", got)
	}
	if frame.DirtyRegions[0] != (gfx.RectFromXYWH(10, 10, 10, 10)) {
		t.Fatalf("dirty regions = %#v", frame.DirtyRegions)
	}
}

func TestRuntime_addfacet_visible_next_frame(t *testing.T) {
	root, child := newRuntimeRenderTree()
	backend := &recordingBackend{}
	rt := mustRuntimeWithBackend(t, root, backend)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	before := rt.LastFrameStats().RenderBatchCount
	rt.AddFacet(root, child, layout.ChildAttachment{})
	rt.RunOneFrame()
	if got := rt.LastFrameStats().RenderBatchCount; got <= before {
		t.Fatalf("RenderBatch count = %d, before = %d", got, before)
	}
	if backend.last == nil || len(backend.last.RenderBatchs) != 2 {
		t.Fatalf("backend frame = %#v", backend.last)
	}
	rt.Shutdown()
}

func TestRuntime_assembleFrame_orders_layers_and_z(t *testing.T) {
	resolver := assemblyLayerResolverStub{
		layers: map[facet.FacetID]facet.ProjectionLayer{
			1: {RenderOrder: 2, ClipRect: gfx.Rect{}},
			2: {RenderOrder: 0, ClipRect: gfx.Rect{}},
			3: {RenderOrder: 1, ClipRect: gfx.Rect{}},
		},
	}
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10), Commands: gfx.CommandList{}},
			{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 0, 10, 10), Commands: gfx.CommandList{}},
			{FacetID: 3, Bounds: gfx.RectFromXYWH(20, 0, 10, 10), Commands: gfx.CommandList{}},
		},
	}
	frame := assembleFrameWithLayers(output, nil, resolver)
	if len(frame.Layers) != 3 {
		t.Fatalf("layers = %d, want 3", len(frame.Layers))
	}
	if frame.Layers[0].RenderOrder != 0 || frame.Layers[1].RenderOrder != 1 || frame.Layers[2].RenderOrder != 2 {
		t.Fatalf("layer order = %#v", frame.Layers)
	}
	if frame.RenderBatchs[0].ID != 2 || frame.RenderBatchs[1].ID != 3 || frame.RenderBatchs[2].ID != 1 {
		t.Fatalf("render batch order = %#v", frame.RenderBatchs)
	}
}

func TestRuntime_assembleFrame_groups_same_layer_by_zpriority(t *testing.T) {
	rt := mustRuntime(t)
	rt.policyRegistry = DefaultRegistry()
	rtResolver := assemblyLayerResolverStub{
		layers: map[facet.FacetID]facet.ProjectionLayer{
			1: {RenderOrder: 1, ClipRect: gfx.RectFromXYWH(0, 0, 100, 100)},
			2: {RenderOrder: 1, ClipRect: gfx.RectFromXYWH(0, 0, 100, 100)},
		},
		attachments: map[facet.FacetID]layout.ChildAttachment{
			1: {ZPriority: 1},
			2: {ZPriority: 0},
		},
	}
	output := &projection.FrameOutput{
		RenderBatchs: []projection.RenderBatchOutput{
			{FacetID: 1, Bounds: gfx.RectFromXYWH(0, 0, 10, 10), Commands: gfx.CommandList{}},
			{FacetID: 2, Bounds: gfx.RectFromXYWH(10, 0, 10, 10), Commands: gfx.CommandList{}},
		},
	}
	frame := assembleFrameWithLayers(output, nil, rtResolver)
	if len(frame.Layers) != 1 {
		t.Fatalf("layers = %d, want 1", len(frame.Layers))
	}
	if got := frame.RenderBatchs[0].ID; got != 2 {
		t.Fatalf("first batch id = %d, want 2", got)
	}
	if got := frame.RenderBatchs[1].ID; got != 1 {
		t.Fatalf("second batch id = %d, want 1", got)
	}
	if got := len(frame.Layers[0].Batches); got != 2 {
		t.Fatalf("layer batch count = %d, want 2", got)
	}
}

func TestRuntime_addchild_arranges_by_layer_attachment(t *testing.T) {
	root, child := newRuntimeLayerTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.AddFacet(root, child, layout.ChildAttachment{
		LayerID: 1,
		Placement: layout.PlacementHints{
			FreeAnchor: layout.FreeBottomRight,
		},
	})
	rt.RunOneFrame()
	got := child.LayoutRole().ArrangedBounds
	want := gfx.RectFromXYWH(80, 90, 20, 10)
	if got != want {
		t.Fatalf("arranged bounds = %#v, want %#v", got, want)
	}
	rt.Shutdown()
}

func TestRuntime_addfacet_wrapper_preserves_old_api(t *testing.T) {
	root, child := newRuntimeLayerTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.AddFacet(root, child, layout.ChildAttachment{})
	rt.RunOneFrame()
	if got := child.LayoutRole().ArrangedBounds; got.IsEmpty() {
		t.Fatal("expected child to be arranged")
	}
	rt.Shutdown()
}

func TestRuntime_layerpolicy_registry_invokes_registered_policy(t *testing.T) {
	root, child := newRuntimeLayerTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	called := false
	rt.policyRegistry.policies[layout.PlacementFree] = &spyPolicy{
		measure: func(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
			return gfx.Size{}
		},
		arrange: func(children []layout.ChildNode, layer layout.ResolvedLayer) {
			called = true
			for i := range children {
				children[i].SetArrangedBounds(gfx.RectFromXYWH(0, 0, 1, 1))
			}
		},
	}
	rt.AddFacet(root, child, layout.ChildAttachment{
		LayerID: 1,
		Placement: layout.PlacementHints{
			FreeAnchor: layout.FreeTopLeft,
		},
	})
	rt.RunOneFrame()
	if !called {
		t.Fatal("expected policy to be invoked")
	}
	rt.Shutdown()
}

func TestRuntime_anchorExport_updates_cache_and_skips_unchanged(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	rt := setupAnchorExportRuntime(t, root, exporter, child)

	rt.resolveAnchorExports()

	cache := rt.anchorCaches[root.ID()]
	if cache == nil {
		t.Fatal("expected cache to be created")
	}
	if got := cache.Version(); got != 1 {
		t.Fatalf("cache version = %d, want 1", got)
	}
	if pos, ok := cache.Get("mark"); !ok || pos != (gfx.Point{X: 10, Y: 20}) {
		t.Fatalf("cached mark = %#v, %v", pos, ok)
	}
	if flags := child.DirtyFlags(); flags&facet.DirtyLayout == 0 {
		t.Fatalf("child flags = %v, want layout dirty", flags)
	}
	if got := exporter.exportHits; got != 1 {
		t.Fatalf("export hits = %d, want 1", got)
	}

	child.Base().ClearDirty(facet.DirtyLayout)
	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	rt.dirtySources = make(map[facet.FacetID]string)

	rt.resolveAnchorExports()

	if got := cache.Version(); got != 1 {
		t.Fatalf("cache version after identical export = %d, want 1", got)
	}
	if flags := child.DirtyFlags(); flags&facet.DirtyLayout != 0 {
		t.Fatalf("child flags = %v, want clean", flags)
	}
	if got := exporter.exportHits; got != 2 {
		t.Fatalf("export hits = %d, want 2", got)
	}
}

func TestRuntime_anchorExport_marks_children_on_move_and_remove(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	rt := setupAnchorExportRuntime(t, root, exporter, child)

	rt.resolveAnchorExports()
	cache := rt.anchorCaches[root.ID()]
	if cache == nil {
		t.Fatal("expected cache")
	}

	child.Base().ClearDirty(facet.DirtyLayout)
	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	rt.dirtySources = make(map[facet.FacetID]string)

	exporter.anchors = layout.AnchorSet{"mark": gfx.Point{X: 20, Y: 25}}
	rt.resolveAnchorExports()
	if got := cache.Version(); got != 2 {
		t.Fatalf("cache version after move = %d, want 2", got)
	}
	if flags := child.DirtyFlags(); flags&facet.DirtyLayout == 0 {
		t.Fatalf("child flags after move = %v, want layout dirty", flags)
	}

	child.Base().ClearDirty(facet.DirtyLayout)
	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	rt.dirtySources = make(map[facet.FacetID]string)

	exporter.anchors = nil
	rt.resolveAnchorExports()
	if got := cache.Version(); got != 3 {
		t.Fatalf("cache version after removal = %d, want 3", got)
	}
	if _, ok := cache.Get("mark"); ok {
		t.Fatal("expected removed anchor to be absent")
	}
	if flags := child.DirtyFlags(); flags&facet.DirtyLayout == 0 {
		t.Fatalf("child flags after removal = %v, want layout dirty", flags)
	}
}

func TestRuntime_anchorExport_resets_cache_when_exporter_detached(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	rt := setupAnchorExportRuntime(t, root, exporter, child)

	rt.resolveAnchorExports()
	cache := rt.anchorCaches[root.ID()]
	if cache == nil {
		t.Fatal("expected cache")
	}

	child.Base().ClearDirty(facet.DirtyLayout)
	rt.dirtyFacets = make(map[facet.FacetID]facet.DirtyFlags)
	rt.dirtySources = make(map[facet.FacetID]string)

	rt.RemoveFacet(exporter)
	rt.resolveAnchorExports()

	if got := cache.Version(); got != 2 {
		t.Fatalf("cache version after detach = %d, want 2", got)
	}
	if _, ok := cache.Get("mark"); ok {
		t.Fatal("expected cache to be reset")
	}
	if flags := child.DirtyFlags(); flags&facet.DirtyLayout == 0 {
		t.Fatalf("child flags after detach = %v, want layout dirty", flags)
	}
}

func TestRuntime_anchorExport_discards_cache_when_anchor_children_removed(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	rt := setupAnchorExportRuntime(t, root, exporter, child)

	rt.resolveAnchorExports()
	if _, ok := rt.anchorCaches[root.ID()]; !ok {
		t.Fatal("expected cache")
	}

	rt.RemoveFacet(child)
	rt.resolveAnchorExports()

	if _, ok := rt.anchorCaches[root.ID()]; ok {
		t.Fatal("expected cache to be discarded when no anchor children remain")
	}
}

func TestRuntime_anchorExport_panics_on_store_set(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	s := store.NewValueStore(0)
	exporter.onExport = func(ctx layout.AnchorExportContext) {
		s.Set(1)
	}
	rt := setupAnchorExportRuntime(t, root, exporter, child)
	expectPanicContains(t, "store.Set", func() {
		rt.resolveAnchorExports()
	})
}

func TestRuntime_anchorExport_panics_on_job_schedule(t *testing.T) {
	root, exporter, child := newRuntimeAnchorTree()
	pool := job.NewPool(1)
	defer pool.Shutdown()
	exporter.onExport = func(ctx layout.AnchorExportContext) {
		_ = job.Schedule(pool, job.Job[int, int]{
			ID:       1,
			Priority: job.PriorityBackground,
			Snapshot: job.NewSnapshot(1),
			Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
				return snap.Data, nil
			},
		}, nil)
	}
	rt := setupAnchorExportRuntime(t, root, exporter, child)
	expectPanicContains(t, "job.Schedule", func() {
		rt.resolveAnchorExports()
	})
}

func TestRuntime_anchorLayer_uses_exported_anchor_cache(t *testing.T) {
	root, exporter, child := newRuntimeAnchorPlacementTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.AddFacet(root, exporter, layout.ChildAttachment{LayerID: 1})
	rt.AddFacet(root, child, layout.ChildAttachment{
		LayerID: 2,
		Placement: layout.PlacementHints{
			AnchorRef:  "mark",
			AnchorSide: layout.AnchorAbove,
			AnchorGap:  8,
		},
	})
	rt.RunOneFrame()
	got := child.LayoutRole().ArrangedBounds
	want := gfx.RectFromXYWH(75, 162, 50, 30)
	if got != want {
		t.Fatalf("anchor bounds = %#v, want %#v", got, want)
	}
	rt.Shutdown()
}

func TestRuntime_projectedLayer_uses_world_positioned_child(t *testing.T) {
	root, child := newRuntimeProjectedTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.AddFacet(root, child, layout.ChildAttachment{LayerID: 2})
	rt.RunOneFrame()
	got := child.LayoutRole().ArrangedBounds
	want := gfx.RectFromXYWH(100, 200, 50, 30)
	if got != want {
		t.Fatalf("projected bounds = %#v, want %#v", got, want)
	}
	rt.Shutdown()
}

func TestRuntime_projectedLayer_warns_when_world_position_missing(t *testing.T) {
	root := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		specs: []layout.LayerSpec{
			{ID: 1, Placement: layout.PlacementStack, Measurement: layout.MeasureStructural},
			{ID: 2, Placement: layout.PlacementProjected, Measurement: layout.MeasureNonStructural, CoordSpace: layout.CoordViewport},
		},
	}
	root.layout.OnMeasure = func(c facet.Constraints) gfx.Size { return gfx.Size{W: 400, H: 400} }
	root.layout.OnArrange = func(bounds gfx.Rect) { root.layout.ArrangedBounds = bounds }
	root.AddRole(&root.layout)

	child := newRuntimeRenderFacet("plain", gfx.RectFromXYWH(0, 0, 20, 20), color.RGBA{R: 200, G: 0, B: 0, A: 255})
	child.layout.OnArrange = func(bounds gfx.Rect) { child.layout.ArrangedBounds = bounds }

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.log = &recordingLogger{}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.AddFacet(root, child, layout.ChildAttachment{LayerID: 2})
	rt.RunOneFrame()
	if got := child.LayoutRole().ArrangedBounds; got != (gfx.Rect{}) {
		t.Fatalf("missing world-position child bounds = %#v, want zero", got)
	}
	if got := len(rt.log.(*recordingLogger).warnings); got == 0 {
		t.Fatal("expected warning for missing WorldPositioned")
	}
	rt.Shutdown()
}

func TestRuntime_HitTest_passThrough_prefers_lower_layer(t *testing.T) {
	rt := mustRuntime(t)
	rt.projectionLayers = map[facet.FacetID]facet.ProjectionLayer{
		1: {RenderOrder: 2, HitPolicy: uint8(layout.HitPassThrough)},
		2: {RenderOrder: 1, HitPolicy: uint8(layout.HitNormal)},
	}
	rt.projectionSystem = projection.NewSystem()
	rt.projectionSystem.SetCurrentHitMap(projection.NewHitMap(
		projection.HitMapEntry{FacetID: 1, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
		projection.HitMapEntry{FacetID: 2, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
	))
	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != 2 {
		t.Fatalf("HitTest = %d, want 2", got)
	}
}

func TestRuntime_HitTest_blockBelow_stops_traversal(t *testing.T) {
	rt := mustRuntime(t)
	rt.projectionLayers = map[facet.FacetID]facet.ProjectionLayer{
		1: {RenderOrder: 2, HitPolicy: uint8(layout.HitBlockBelow)},
		2: {RenderOrder: 1, HitPolicy: uint8(layout.HitNormal)},
	}
	rt.projectionSystem = projection.NewSystem()
	rt.projectionSystem.SetCurrentHitMap(projection.NewHitMap(
		projection.HitMapEntry{FacetID: 1, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(200, 200, 10, 10)}}},
		projection.HitMapEntry{FacetID: 2, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
	))
	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != 0 {
		t.Fatalf("HitTest = %d, want 0", got)
	}
}

func TestRuntime_HitTest_disabled_skips_layer(t *testing.T) {
	rt := mustRuntime(t)
	rt.projectionLayers = map[facet.FacetID]facet.ProjectionLayer{
		1: {RenderOrder: 2, HitPolicy: uint8(layout.HitDisabled)},
		2: {RenderOrder: 1, HitPolicy: uint8(layout.HitNormal)},
	}
	rt.projectionSystem = projection.NewSystem()
	rt.projectionSystem.SetCurrentHitMap(projection.NewHitMap(
		projection.HitMapEntry{FacetID: 1, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
		projection.HitMapEntry{FacetID: 2, Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
	))
	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != 2 {
		t.Fatalf("HitTest = %d, want 2", got)
	}
}

func TestRuntime_HitTest_recordsTrace(t *testing.T) {
	rt := mustRuntime(t)
	rt.EnableHitTrace(true)
	rt.root = &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	childOne := &runtimeTestFacet{Facet: facet.NewFacet(), name: "childOne"}
	childTwo := &runtimeTestFacet{Facet: facet.NewFacet(), name: "childTwo"}
	rootBase := rt.root.Base()
	rootBase.AddChildRuntime(&childOne.Facet)
	rootBase.AddChildRuntime(&childTwo.Facet)
	rt.projectionLayers = map[facet.FacetID]facet.ProjectionLayer{
		childOne.ID(): {RenderOrder: 2, HitPolicy: uint8(layout.HitPassThrough)},
		childTwo.ID(): {RenderOrder: 1, HitPolicy: uint8(layout.HitNormal)},
	}
	rt.projectionSystem = projection.NewSystem()
	rt.projectionSystem.SetCurrentHitMap(projection.NewHitMap(
		projection.HitMapEntry{FacetID: childOne.ID(), Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
		projection.HitMapEntry{FacetID: childTwo.ID(), Transform: gfx.Identity(), Regions: []projection.HitRegion{{Bounds: gfx.RectFromXYWH(0, 0, 100, 100)}}},
	))
	if got := rt.HitTest(gfx.Point{X: 10, Y: 10}); got != childTwo.ID() {
		t.Fatalf("HitTest = %d, want %d", got, childTwo.ID())
	}
	trace := rt.HitTrace()
	if trace.Result != childTwo.ID() || len(trace.TestedLayers) != 2 {
		t.Fatalf("trace = %#v", trace)
	}
	if trace.TestedLayers[0].StoppedHere || !trace.TestedLayers[1].StoppedHere {
		t.Fatalf("trace stop flags = %#v", trace.TestedLayers)
	}
}

func TestRuntime_Inspect_exposesLayerSnapshots(t *testing.T) {
	rt := mustRuntime(t)
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	rootBase := root.Base()
	rt.root = root
	rt.layerStates[rootBase.ID()] = &resolvedLayerSet{
		specs: []layout.LayerSpec{{
			ID:          9,
			Placement:   layout.PlacementStack,
			Measurement: layout.MeasureStructural,
			CoordSpace:  layout.CoordParentLayout,
			RenderOrder: 4,
			HitPolicy:   layout.HitNormal,
		}},
		layers: []layout.ResolvedLayer{{
			LayerID:     9,
			Bounds:      gfx.RectFromXYWH(1, 2, 3, 4),
			ClipRect:    gfx.RectFromXYWH(1, 2, 3, 4),
			RenderOrder: 4,
			HitPolicy:   layout.HitNormal,
		}},
		childCounts: []int{1},
	}
	rt.anchorCaches[rootBase.ID()] = layout.NewAnchorPositionCache()
	rt.anchorCaches[rootBase.ID()].Update("mark-a", gfx.Point{X: 7, Y: 8})
	var inspected bool
	rt.Inspect(func(insp *diagnostics.Inspector) {
		inspected = true
		info, ok := insp.Find(rootBase.ID())
		if !ok {
			t.Fatal("expected root info")
		}
		if len(info.Layers) != 1 || info.Layers[0].LayerID != 9 || info.Layers[0].ChildCount != 1 {
			t.Fatalf("layer info = %#v", info.Layers)
		}
		snap, ok := insp.AnchorSnapshot(rootBase.ID())
		if !ok || snap.Version == 0 || len(snap.Entries) != 1 {
			t.Fatalf("anchor snapshot = %#v", snap)
		}
		desc := insp.Describe()
		if !strings.Contains(desc, "Layers:") || !strings.Contains(desc, "AnchorCache:") {
			t.Fatalf("describe output = %q", desc)
		}
	})
	if !inspected {
		t.Fatal("expected inspect callback")
	}
}

func TestRuntime_removefacet_absent_next_frame(t *testing.T) {
	root, child := newRuntimeRenderTree()
	backend := &recordingBackend{}
	rt := mustRuntimeWithBackend(t, root, backend)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	rt.AddFacet(root, child, layout.ChildAttachment{})
	rt.RunOneFrame()
	if got := rt.LastFrameStats().RenderBatchCount; got != 2 {
		t.Fatalf("RenderBatch count after add = %d", got)
	}
	rt.RemoveFacet(child)
	rt.RunOneFrame()
	if got := rt.LastFrameStats().RenderBatchCount; got != 1 {
		t.Fatalf("RenderBatch count after remove = %d", got)
	}
	if backend.last == nil || len(backend.last.RenderBatchs) != 1 {
		t.Fatalf("backend frame = %#v", backend.last)
	}
	rt.Shutdown()
}

func TestRuntime_removefacet_parent_gets_layout_dirty(t *testing.T) {
	root, child := newRuntimeRenderTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	rt.AddFacet(root, child, layout.ChildAttachment{})
	rt.RunOneFrame()
	rt.RemoveFacet(child)
	if flags := root.DirtyFlags(); flags&facet.DirtyLayout == 0 {
		t.Fatalf("root flags = %v", flags)
	}
	rt.Shutdown()
}

func TestRuntime_request_frame_when_dirty(t *testing.T) {
	root, child := newRuntimeRenderTree()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	select {
	case <-rt.frameTimer.requestCh:
	default:
	}
	rt.AddFacet(root, child, layout.ChildAttachment{})
	select {
	case <-rt.frameTimer.requestCh:
	default:
		t.Fatal("expected immediate frame request after AddFacet")
	}
	rt.Shutdown()
}

func TestRuntime_layout_only_remeasures_dirty_subtree(t *testing.T) {
	root := layout.NewRowLayout()
	left := newLayoutCountLeaf(gfx.Size{W: 50, H: 20})
	right := newLayoutCountLeaf(gfx.Size{W: 60, H: 20})
	root.Add(layout.Fixed(left))
	root.Add(layout.Fixed(right))

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.window = &testWindow{width: 200, height: 100}
	rt.RunOneFrame()

	leftInitial := left.measureCount
	rightInitial := right.measureCount

	rt.dirtyFacets[left.ID()] = facet.DirtyLayout
	rt.markLayoutDirtyFacets()
	rt.layoutSystem.Run(gfx.Size{W: 200, H: 100})

	if left.measureCount != leftInitial+1 {
		t.Fatalf("left measure count = %d, want %d", left.measureCount, leftInitial+1)
	}
	if right.measureCount != rightInitial {
		t.Fatalf("right measure count = %d, want %d", right.measureCount, rightInitial)
	}
}

func TestRuntime_layout_full_tree_on_resize(t *testing.T) {
	root := layout.NewRowLayout()
	left := newLayoutCountLeaf(gfx.Size{W: 50, H: 20})
	right := newLayoutCountLeaf(gfx.Size{W: 60, H: 20})
	root.Add(layout.Fixed(left))
	root.Add(layout.Fixed(right))

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.window = &testWindow{width: 200, height: 100}
	rt.RunOneFrame()

	leftInitial := left.measureCount
	rightInitial := right.measureCount

	rt.handleResize(320, 180)
	rt.markLayoutDirtyFacets()
	rt.layoutSystem.Run(gfx.Size{W: 320, H: 180})

	if left.measureCount != leftInitial+1 {
		t.Fatalf("left measure count = %d, want %d", left.measureCount, leftInitial+1)
	}
	if right.measureCount != rightInitial+1 {
		t.Fatalf("right measure count = %d, want %d", right.measureCount, rightInitial+1)
	}
}

func TestRuntime_layout_skipped_when_no_dirty(t *testing.T) {
	root := layout.NewRowLayout()
	left := newLayoutCountLeaf(gfx.Size{W: 50, H: 20})
	right := newLayoutCountLeaf(gfx.Size{W: 60, H: 20})
	root.Add(layout.Fixed(left))
	root.Add(layout.Fixed(right))

	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.window = &testWindow{width: 200, height: 100}
	rt.RunOneFrame()

	leftInitial := left.measureCount
	rightInitial := right.measureCount
	rt.RunOneFrame()

	if left.measureCount != leftInitial {
		t.Fatalf("left measure count = %d, want %d", left.measureCount, leftInitial)
	}
	if right.measureCount != rightInitial {
		t.Fatalf("right measure count = %d, want %d", right.measureCount, rightInitial)
	}
}

func TestRuntime_contentScale_from_config(t *testing.T) {
	root := facet.NewFacet()
	win := &testWindow{width: 200, height: 100, contentScale: 3}
	cfg := DefaultConfig()
	cfg.ContentScale = 2
	rt, err := New(cfg, nil, win, &stubBackend{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := rt.contentScale; got != 2 {
		t.Fatalf("contentScale = %v, want 2", got)
	}
	rt.Shutdown()
}

func TestRuntime_contentScale_from_window(t *testing.T) {
	root := facet.NewFacet()
	win := &testWindow{width: 200, height: 100, contentScale: 1.5}
	cfg := DefaultConfig()
	cfg.ContentScale = 0
	rt, err := New(cfg, nil, win, &stubBackend{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	if got := rt.contentScale; got != 1.5 {
		t.Fatalf("contentScale = %v, want 1.5", got)
	}
	rt.Shutdown()
}

func TestRuntime_resize_marks_dirty_on_scale_change(t *testing.T) {
	root := facet.NewFacet()
	win := &testWindow{width: 200, height: 100, contentScale: 1}
	rt, err := New(DefaultConfig(), nil, win, &stubBackend{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.dirtyFacets = map[facet.FacetID]facet.DirtyFlags{}
	win.contentScale = 2
	rt.handleResize(320, 180)
	if got := rt.contentScale; got != 2 {
		t.Fatalf("contentScale = %v, want 2", got)
	}
	if flags := rt.dirtyFacets[root.ID()]; flags&facet.DirtyAll == 0 {
		t.Fatalf("dirty flags = %v", flags)
	}
	rt.Shutdown()
}

func TestRuntime_run_returns_render_error(t *testing.T) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	rt := mustRuntimeWithBackend(t, root, &stubBackend{submitErr: errors.New("boom")})
	errCh := make(chan error, 1)
	go func() {
		errCh <- rt.Run()
	}()
	select {
	case err := <-errCh:
		if err == nil || err.Error() == "" {
			t.Fatal("expected render error")
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for runtime error")
	}
}

func TestRuntime_diagnostics_hook_records_frames(t *testing.T) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	hook := &countingDiagHook{}
	cfg := DefaultConfig()
	cfg.DiagnosticsHook = hook
	rt, err := New(cfg, nil, nil, &stubBackend{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	if hook.count == 0 {
		t.Fatal("expected diagnostics hook to fire")
	}
	rt.Shutdown()
}

func TestRuntime_focus_manager_not_nil(t *testing.T) {
	rt := mustRuntime(t)
	if rt.focusManager == nil {
		t.Fatal("expected focus manager")
	}
}

func TestRuntime_RebuildTabOrder_called_each_frame(t *testing.T) {
	root := facet.NewFacet()
	a := newRuntimeFocusFacet(1)
	b := newRuntimeFocusFacet(0)
	root.AddChild(&a.Facet)
	root.AddChild(&b.Facet)
	rt := mustRuntimeTree(t, &root)

	rt.RunOneFrame()
	rt.focusManager.SetFocus(b)
	rt.focusManager.TabNext()
	if got := rt.focusManager.Focused(); got != a.ID() {
		t.Fatalf("focused after first frame = %d", got)
	}

	a.focus.TabIndex = -1
	rt.RunOneFrame()
	rt.focusManager.SetFocus(b)
	rt.focusManager.TabNext()
	if got := rt.focusManager.Focused(); got != b.ID() {
		t.Fatalf("focused after second frame = %d", got)
	}
}

func TestRuntime_WindowBlur_clears_focus(t *testing.T) {
	root := facet.NewFacet()
	child := newRuntimeFocusFacet(0)
	root.AddChild(&child.Facet)
	rt := mustRuntimeTree(t, &root)
	rt.RunOneFrame()
	rt.focusManager.SetFocus(child)
	rt.pendingEvents = []platform.Event{platform.EventWindowFocus{Focused: false}}
	rt.RunOneFrame()
	if got := rt.focusManager.Focused(); got != 0 {
		t.Fatalf("focused = %d", got)
	}
}

func TestRuntime_Tab_advances_focus(t *testing.T) {
	root := facet.NewFacet()
	a := newRuntimeFocusFacet(0)
	b := newRuntimeFocusFacet(1)
	root.AddChild(&a.Facet)
	root.AddChild(&b.Facet)
	rt := mustRuntimeTree(t, &root)
	rt.RunOneFrame()
	rt.focusManager.SetFocus(a)
	rt.pendingEvents = []platform.Event{platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab}}
	rt.RunOneFrame()
	if got := rt.focusManager.Focused(); got != b.ID() {
		t.Fatalf("focused = %d", got)
	}
}

func TestRuntime_ShiftTab_reverses_focus(t *testing.T) {
	root := facet.NewFacet()
	a := newRuntimeFocusFacet(0)
	b := newRuntimeFocusFacet(1)
	root.AddChild(&a.Facet)
	root.AddChild(&b.Facet)
	rt := mustRuntimeTree(t, &root)
	rt.RunOneFrame()
	rt.focusManager.SetFocus(b)
	rt.pendingEvents = []platform.Event{platform.EventKey{Kind: platform.KeyPress, Key: platform.KeyTab, Modifiers: platform.ModShift}}
	rt.RunOneFrame()
	if got := rt.focusManager.Focused(); got != a.ID() {
		t.Fatalf("focused = %d", got)
	}
}

func TestRuntime_updateIMECursorRect_sets_window_rect(t *testing.T) {
	root := facet.NewFacet()
	child := &runtimeFocusFacet{Facet: facet.NewFacet()}
	child.focus.Focusable = func() bool { return true }
	child.focus.TabIndex = 0
	child.text.Layout = &text.TextLayout{
		Lines: []text.ShapedLine{
			{
				Bounds:    text.RectFromXYWH(4, 6, 20, 14),
				Baseline:  12,
				FirstRune: 0,
				RuneCount: 1,
			},
		},
		LineHeight: 14,
	}
	child.text.CaretVisible = true
	child.text.CaretPosition = text.TextPosition{Index: 0, Affinity: text.AffinityDownstream}
	child.AddRole(&child.focus)
	child.AddRole(&child.text)
	root.AddChild(&child.Facet)

	win := &testWindow{width: 200, height: 100, contentScale: 1}
	rt := mustRuntimeWithBackend(t, &root, &stubBackend{})
	rt.window = win
	rt.focusManager.SetFocus(child)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	if win.imeRect != (gfx.RectFromXYWH(4, 6, 2, 14)) {
		t.Fatalf("imeRect = %#v", win.imeRect)
	}
	rt.Shutdown()
}

func TestRuntime_projection_context_has_runtime(t *testing.T) {
	root := newProjectionRuntimeFacet()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	rt.RunOneFrame()
	if !root.scheduled {
		t.Fatal("expected projection to schedule a job")
	}
	rt.RunOneFrame()
	rt.Shutdown()
}

func TestRuntime_OnJobResult_called_after_drain(t *testing.T) {
	root := newProjectionJobFacet()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	root.rt = rt

	rt.RunOneFrame()
	<-root.jobStarted
	close(root.jobRelease)
	<-root.jobDone
	time.Sleep(10 * time.Millisecond)
	rt.RunOneFrame()

	if root.commitCount != 1 {
		t.Fatalf("commitCount = %d", root.commitCount)
	}
	if root.commitValue != 10 {
		t.Fatalf("commitValue = %d", root.commitValue)
	}
	if root.jobResultCount != 1 {
		t.Fatalf("jobResultCount = %d", root.jobResultCount)
	}
	if root.lastResult == nil || root.lastResult.JobID() != 1 || root.lastResult.OwnerID() != uint64(root.ID()) {
		t.Fatalf("lastResult = %#v", root.lastResult)
	}
	if root.projectCalls != 2 {
		t.Fatalf("projectCalls = %d", root.projectCalls)
	}
}

func TestRuntime_OnJobResult_stale_snapshot_not_committed(t *testing.T) {
	root := newProjectionJobFacet()
	root.versionSource = store.NewValueStore[int](1)
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	root.rt = rt

	rt.RunOneFrame()
	<-root.jobStarted
	root.versionSource.Set(2)
	close(root.jobRelease)
	<-root.jobDone
	time.Sleep(10 * time.Millisecond)
	rt.RunOneFrame()

	if root.commitCount != 0 {
		t.Fatalf("commitCount = %d", root.commitCount)
	}
	if root.jobResultCount != 0 {
		t.Fatalf("jobResultCount = %d", root.jobResultCount)
	}
	if root.projectCalls != 1 {
		t.Fatalf("projectCalls = %d", root.projectCalls)
	}
}

func TestRuntime_OnJobResult_cancelled_not_delivered(t *testing.T) {
	root := newProjectionJobFacet()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	root.rt = rt

	rt.RunOneFrame()
	<-root.jobStarted
	rt.CancelJob(1)
	close(root.jobRelease)
	<-root.jobDone
	time.Sleep(10 * time.Millisecond)
	rt.RunOneFrame()

	if root.commitCount != 0 {
		t.Fatalf("commitCount = %d", root.commitCount)
	}
	if root.jobResultCount != 0 {
		t.Fatalf("jobResultCount = %d", root.jobResultCount)
	}
}

func TestRuntime_drainJobResults_returns_correct_counts(t *testing.T) {
	rt := mustRuntime(t)

	if err := job.Schedule(rt.jobPool, job.Job[int, int]{
		ID:       1,
		Priority: job.PriorityBackground,
		Snapshot: job.NewSnapshot(2, 1),
		Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
			return snap.Data + 1, nil
		},
	}, func(int) {}); err != nil {
		t.Fatalf("schedule success: %v", err)
	}

	if err := job.Schedule(rt.jobPool, job.Job[int, int]{
		ID:       2,
		Priority: job.PriorityBackground,
		Snapshot: job.NewSnapshot(3, 1),
		Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
			return 0, errors.New("boom")
		},
	}, func(int) {}); err != nil {
		t.Fatalf("schedule error: %v", err)
	}

	started := make(chan struct{})
	release := make(chan struct{})
	done := make(chan struct{})
	if err := job.Schedule(rt.jobPool, job.Job[int, int]{
		ID:       3,
		Priority: job.PriorityBackground,
		Snapshot: job.NewSnapshot(4, 1),
		Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
			defer close(done)
			close(started)
			<-release
			return snap.Data, nil
		},
	}, func(int) {}); err != nil {
		t.Fatalf("schedule cancel: %v", err)
	}

	<-started
	rt.CancelJob(3)
	close(release)
	<-done
	time.Sleep(10 * time.Millisecond)

	committed, discarded := rt.drainJobResults()
	if committed != 1 || discarded != 2 {
		t.Fatalf("committed=%d discarded=%d", committed, discarded)
	}
}

func TestRuntime_dirty_after_OnJobResult(t *testing.T) {
	root := newProjectionJobFacet()
	rt := mustRuntimeWithBackend(t, root, &stubBackend{})
	root.rt = rt

	rt.RunOneFrame()
	<-root.jobStarted
	close(root.jobRelease)
	<-root.jobDone
	rt.drainJobResults()

	if flags := rt.dirtyFacets[root.ID()]; flags&facet.DirtyProjection == 0 {
		t.Fatalf("dirty flags = %v", flags)
	}
}

func TestRuntime_Schedule_submits_to_pool(t *testing.T) {
	root := newRuntimeJobFacet()
	rt := mustRuntimeTree(t, root)
	committed := 0
	started := make(chan struct{})
	release := make(chan struct{})
	j := job.BindJob(uint64(root.ID()), job.Job[int, int]{
		ID:       7,
		Priority: job.PriorityBackground,
		Snapshot: job.NewSnapshot(3, 1),
		Work: func(snap job.Snapshot[int], cancel *job.CancelToken) (int, error) {
			close(started)
			<-release
			return snap.Data * 2, nil
		},
	}, func(v int) {
		committed = v
	})

	rt.Schedule(j)
	<-started
	close(release)
	time.Sleep(10 * time.Millisecond)
	rt.RunOneFrame()

	if committed != 6 {
		t.Fatalf("commit = %d", committed)
	}
	if root.lastResult == nil {
		t.Fatal("expected on-job-result callback")
	}
	if got := root.lastResult.JobID(); got != 7 {
		t.Fatalf("job id = %d", got)
	}
	if got := root.lastResult.OwnerID(); got != uint64(root.ID()) {
		t.Fatalf("owner id = %d", got)
	}
	if root.lastResult.Cancelled() {
		t.Fatal("expected non-cancelled result")
	}
	if root.lastResult.Err() != nil {
		t.Fatalf("unexpected error = %v", root.lastResult.Err())
	}
}

func TestRuntime_Schedule_nil_job_noop(t *testing.T) {
	var rt *Runtime
	rt.Schedule(nil)
}

func TestRuntime_findFacetByID_root(t *testing.T) {
	root, _, _ := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	if got := rt.findFacetByID(root, root.ID()); got == nil || got.Base() != root.Base() {
		t.Fatalf("got %#v", got)
	}
}

func TestRuntime_findFacetByID_child(t *testing.T) {
	root, _, leaf := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	if got := rt.findFacetByID(root, leaf.ID()); got == nil || got.Base() != leaf.Base() {
		t.Fatalf("got %#v", got)
	}
}

func TestRuntime_findFacetByID_missing(t *testing.T) {
	root, _, _ := newRuntimeTestTree()
	rt := mustRuntimeTree(t, root)
	if got := rt.findFacetByID(root, 9999); got != nil {
		t.Fatalf("got %#v", got)
	}
}

func expectPanicContains(t *testing.T, want string, fn func()) {
	t.Helper()
	defer func() {
		recovered := recover()
		if recovered == nil {
			t.Fatalf("expected panic containing %q", want)
		}
		msg, _ := recovered.(string)
		if !strings.Contains(msg, want) {
			t.Fatalf("panic %q missing %q", msg, want)
		}
	}()
	fn()
}

func mustRuntime(t *testing.T) *Runtime {
	t.Helper()
	root := facet.NewFacet()
	rt, err := New(DefaultConfig(), nil, nil, &stubBackend{}, &root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

func mustRuntimeTree(t *testing.T, root facet.FacetImpl) *Runtime {
	t.Helper()
	rt, err := New(DefaultConfig(), nil, nil, &stubBackend{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

func mustRuntimeWithBackend(t *testing.T, root facet.FacetImpl, backend render.Backend) *Runtime {
	t.Helper()
	rt, err := New(DefaultConfig(), nil, nil, backend, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt
}

type testWindow struct {
	width        int
	height       int
	contentScale float32
	imeRect      gfx.Rect
}

func (w *testWindow) Surface() platform.Surface { return nil }
func (w *testWindow) SetTitle(title string)     {}
func (w *testWindow) Size() (width, height int) { return w.width, w.height }
func (w *testWindow) ContentScale() float32 {
	if w != nil && w.contentScale > 0 {
		return w.contentScale
	}
	return 1
}
func (w *testWindow) SetIMECursorRect(rect gfx.Rect) { w.imeRect = rect }
func (w *testWindow) Show()                          {}
func (w *testWindow) Hide()                          {}
func (w *testWindow) Close()                         {}
func (w *testWindow) Destroy()                       {}

var _ platform.App = (*nilApp)(nil)

type nilApp struct{}

func (n *nilApp) NewWindow(platform.WindowOptions) (platform.Window, error) { return nil, nil }
func (n *nilApp) Events() platform.EventQueue                               { return nil }
func (n *nilApp) Clipboard() platform.Clipboard                             { return nil }
func (n *nilApp) Destroy()                                                  {}

var _ = text.FontRegistry{}
