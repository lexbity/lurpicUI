package runtime

import (
	"errors"
	"image/color"
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

func (f *runtimeTestFacet) Base() *facet.Facet               { return &f.Facet }
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

func (f *runtimeSubscriptionFacet) Base() *facet.Facet { return &f.Facet }
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
		Layers: []projection.LayerOutput{
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
	if frame == nil || len(frame.Layers) != 1 {
		t.Fatalf("frame = %#v", frame)
	}
	cmds := frame.Layers[0].Commands.Commands
	if len(cmds) < 3 {
		t.Fatalf("commands = %#v", cmds)
	}
	if _, ok := cmds[0].(gfx.PushTransform); !ok {
		t.Fatalf("first command = %T", cmds[0])
	}
	if _, ok := cmds[len(cmds)-1].(gfx.PopTransform); !ok {
		t.Fatalf("last command = %T", cmds[len(cmds)-1])
	}
	if frame.Layers[0].CommandHash == 0 {
		t.Fatal("expected command hash")
	}
}

func TestRuntime_assembleFrame_dirty_regions(t *testing.T) {
	rt := mustRuntime(t)
	output := &projection.FrameOutput{
		Layers: []projection.LayerOutput{
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
	before := rt.LastFrameStats().LayerCount
	rt.AddFacet(root, child)
	rt.RunOneFrame()
	if got := rt.LastFrameStats().LayerCount; got <= before {
		t.Fatalf("layer count = %d, before = %d", got, before)
	}
	if backend.last == nil || len(backend.last.Layers) != 2 {
		t.Fatalf("backend frame = %#v", backend.last)
	}
	rt.Shutdown()
}

func TestRuntime_removefacet_absent_next_frame(t *testing.T) {
	root, child := newRuntimeRenderTree()
	backend := &recordingBackend{}
	rt := mustRuntimeWithBackend(t, root, backend)
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	rt.AddFacet(root, child)
	rt.RunOneFrame()
	if got := rt.LastFrameStats().LayerCount; got != 2 {
		t.Fatalf("layer count after add = %d", got)
	}
	rt.RemoveFacet(child)
	rt.RunOneFrame()
	if got := rt.LastFrameStats().LayerCount; got != 1 {
		t.Fatalf("layer count after remove = %d", got)
	}
	if backend.last == nil || len(backend.last.Layers) != 1 {
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
	rt.AddFacet(root, child)
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
	rt.AddFacet(root, child)
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
