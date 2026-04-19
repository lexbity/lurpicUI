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
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
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

var _ platform.App = (*nilApp)(nil)

type nilApp struct{}

func (n *nilApp) NewWindow(platform.WindowOptions) (platform.Window, error) { return nil, nil }
func (n *nilApp) Events() platform.EventQueue                               { return nil }
func (n *nilApp) Clipboard() platform.Clipboard                             { return nil }
func (n *nilApp) Destroy()                                                  {}

var _ = text.FontRegistry{}
