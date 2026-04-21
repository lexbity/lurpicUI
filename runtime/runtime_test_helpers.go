package runtime

import (
	"image/color"
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
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
