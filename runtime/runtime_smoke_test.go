package runtime

import (
	"errors"
	"image/color"
	"sync"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/diagnostics"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestRuntimeNew_validation(t *testing.T) {
	cfg := DefaultConfig()
	layerRegistry, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	cfg.LayerRegistry = layerRegistry
	cfg.FontRegistry = nil
	if _, err := New(cfg, nil, nil, &backendFixture{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error for nil font registry")
	}
	cfg = DefaultConfig()
	cfg.LayerRegistry = layerRegistry
	cfg.LayerRegistry = nil
	if _, err := New(cfg, nil, nil, &backendFixture{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error for nil layer registry")
	}
	cfg = DefaultConfig()
	cfg.LayerRegistry = layerRegistry
	cfg.TargetFPS = 0
	if _, err := New(cfg, nil, nil, &backendFixture{}, &facet.Facet{}); err == nil {
		t.Fatal("expected error for zero target fps")
	}
	cfg = DefaultConfig()
	cfg.LayerRegistry = layerRegistry
	if _, err := New(cfg, nil, nil, &backendFixture{}, nil); err == nil {
		t.Fatal("expected error for nil root")
	}
}

func TestFrameTimer_basics(t *testing.T) {
	timer := NewFrameTimer(60)
	timer.RequestFrame()
	before := time.Now()
	_ = timer.Wait()
	if time.Since(before) > 20*time.Millisecond {
		t.Fatal("expected immediate wake")
	}
}

func TestRenderPipeline_submit_blocks_on_full(t *testing.T) {
	pipe := newRenderPipeline(&backendFixture{})
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
	pipe := newRenderPipeline(&backendFixture{})
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

type queueApp struct {
	queue *queue
}

func (a *queueApp) Events() platform.EventQueue {
	if a.queue == nil {
		a.queue = &queue{}
	}
	return a.queue
}

func (a *queueApp) Destroy() {}

type queue struct {
	mu     sync.Mutex
	events []platform.Event
}

func (q *queue) Push(e platform.Event) {
	q.mu.Lock()
	q.events = append(q.events, e)
	q.mu.Unlock()
}

func (q *queue) Poll() []platform.Event {
	q.mu.Lock()
	defer q.mu.Unlock()
	out := append([]platform.Event(nil), q.events...)
	q.events = q.events[:0]
	return out
}

func (q *queue) Wait(timeout time.Duration) []platform.Event {
	return q.Poll()
}

type runtimeInteractiveFacet struct {
	facet.Facet
	input  facet.InputRole
	hit    facet.HitRole
	layout facet.LayoutRole
	render facet.RenderRole
}

func (f *runtimeInteractiveFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func newRuntimeInteractiveFacet(fill color.RGBA) *runtimeInteractiveFacet {
	f := &runtimeInteractiveFacet{Facet: facet.NewFacet()}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if f.layout.ArrangedBounds.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: 1}
		}
		return facet.HitResult{}
	}
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(fill.R, fill.G, fill.B, fill.A)),
		})
	}
	f.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	f.AddRole(&f.input)
	f.AddRole(&f.hit)
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
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
	rt.AddFacet(root, child, facet.Attachment{})
	rt.RunOneFrame()
	if got := rt.LastFrameStats().RenderBatchCount; got <= before {
		t.Fatalf("RenderBatch count = %d, before = %d", got, before)
	}
	if backend.last == nil || len(backend.last.RenderBatchs) != 2 {
		t.Fatalf("backend frame = %#v", backend.last)
	}
	rt.Shutdown()
}

func TestRuntime_render_batches_follow_registry_order(t *testing.T) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 200, 200), color.RGBA{A: 255})
	first := newRuntimeRenderFacet("first", gfx.RectFromXYWH(0, 0, 20, 20), color.RGBA{R: 255, A: 255})
	second := newRuntimeRenderFacet("second", gfx.RectFromXYWH(0, 0, 20, 20), color.RGBA{G: 255, A: 255})

	backend := &recordingBackend{}
	rt := mustRuntimeWithBackend(t, root, backend)
	reg := testRegistryWithOrders(t, 3500, 1500)
	rt.config.LayerRegistry = reg
	rt.layerRegistry = reg
	firstLayer, ok := reg.LookupName("a")
	if !ok {
		t.Fatal("missing layer a")
	}
	secondLayer, ok := reg.LookupName("b")
	if !ok {
		t.Fatal("missing layer b")
	}

	rt.AddFacet(root, first, facet.Attachment{LayerID: facet.LayerID(firstLayer.ID)})
	rt.AddFacet(root, second, facet.Attachment{LayerID: facet.LayerID(secondLayer.ID)})
	rt.RunOneFrame()

	frame := backend.last
	if frame == nil {
		t.Fatal("expected rendered frame")
	}
	firstIndex := -1
	secondIndex := -1
	for i, batch := range frame.RenderBatchs {
		switch batch.ID {
		case render.RenderBatchID(first.ID()):
			firstIndex = i
		case render.RenderBatchID(second.ID()):
			secondIndex = i
		}
	}
	if firstIndex == -1 || secondIndex == -1 {
		t.Fatalf("missing child batches in frame: %#v", frame.RenderBatchs)
	}
	if secondIndex > firstIndex {
		t.Fatalf("registry order violated: second layer index %d should be before first layer index %d", secondIndex, firstIndex)
	}
}

func phase17Registry(t *testing.T) *layout.LayerRegistry {
	t.Helper()
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	custom := []layout.LayerRegistration{
		{
			Name:      "app.tooltip",
			Order:     2500,
			HitPolicy: layout.HitPassThrough,
		},
		{
			Name:      "app.modal",
			Order:     7500,
			HitPolicy: layout.HitBlockBelow,
		},
		{
			Name:  "app.overlay",
			Order: 8500,
			Dismissal: layout.DismissalScope{
				Enabled:      true,
				BehindOrders: layout.OrderRange{Min: 0, Max: 8000},
				Triggers:     layout.DismissalTriggerSetPointer,
			},
		},
	}
	for _, spec := range custom {
		if _, err := b.RegisterLayer(spec); err != nil {
			t.Fatalf("register %q: %v", spec.Name, err)
		}
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return reg
}

func newPhase17Runtime(t *testing.T, root facet.FacetImpl) (*Runtime, *queueApp) {
	t.Helper()
	app := &queueApp{}
	cfg := DefaultConfig()
	cfg.LayerRegistry = phase17Registry(t)
	window := &testWindow{width: 400, height: 400}
	rt, err := New(cfg, app, window, &backendFixture{}, root)
	if err != nil {
		t.Fatalf("new runtime: %v", err)
	}
	return rt, app
}

func TestRuntime_integration_modal_blocks_base_input(t *testing.T) {
	root := newRuntimeInteractiveFacet(color.RGBA{R: 245, G: 245, B: 245, A: 255})
	base := newRuntimeInteractiveFacet(color.RGBA{R: 60, G: 120, B: 220, A: 255})
	modal := newRuntimeInteractiveFacet(color.RGBA{R: 220, G: 80, B: 80, A: 255})
	var baseCalls, modalCalls int
	base.input.OnPointer = func(facet.PointerEvent) bool {
		baseCalls++
		return true
	}
	modal.input.OnPointer = func(facet.PointerEvent) bool {
		modalCalls++
		return true
	}
	rt, app := newPhase17Runtime(t, root)
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	modalLayer, ok := rt.layerRegistry.LookupName("app.modal")
	if !ok {
		t.Fatal("missing modal layer")
	}
	rt.AddFacet(root, modal, facet.Attachment{LayerID: facet.LayerID(modalLayer.ID)})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	app.Events().Push(platform.EventPointer{
		Kind:     platform.PointerPress,
		Position: gfx.Point{X: 20, Y: 20},
		Button:   platform.PointerLeft,
	})
	rt.RunOneFrame()
	if modalCalls != 1 {
		t.Fatalf("modalCalls = %d, want 1", modalCalls)
	}
	if baseCalls != 0 {
		t.Fatalf("baseCalls = %d, want 0", baseCalls)
	}
	rt.Shutdown()
}

func TestRuntime_integration_floating_pass_through(t *testing.T) {
	root := newRuntimeInteractiveFacet(color.RGBA{R: 245, G: 245, B: 245, A: 255})
	base := newRuntimeInteractiveFacet(color.RGBA{R: 80, G: 130, B: 220, A: 255})
	tooltip := newRuntimeInteractiveFacet(color.RGBA{R: 255, G: 175, B: 80, A: 255})
	var baseCalls, tooltipCalls int
	base.input.OnPointer = func(facet.PointerEvent) bool {
		baseCalls++
		return true
	}
	tooltip.input.OnPointer = func(facet.PointerEvent) bool {
		tooltipCalls++
		return true
	}
	rt, app := newPhase17Runtime(t, root)
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	tooltipLayer, ok := rt.layerRegistry.LookupName("app.tooltip")
	if !ok {
		t.Fatal("missing tooltip layer")
	}
	rt.AddFacet(root, tooltip, facet.Attachment{LayerID: facet.LayerID(tooltipLayer.ID)})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	app.Events().Push(platform.EventPointer{
		Kind:     platform.PointerPress,
		Position: gfx.Point{X: 20, Y: 20},
		Button:   platform.PointerLeft,
	})
	rt.RunOneFrame()
	if tooltipCalls != 0 {
		t.Fatalf("tooltipCalls = %d, want 0", tooltipCalls)
	}
	if baseCalls != 1 {
		t.Fatalf("baseCalls = %d, want 1", baseCalls)
	}
	rt.Shutdown()
}

func TestRuntime_integration_overlay_dismissal(t *testing.T) {
	root := newRuntimeInteractiveFacet(color.RGBA{R: 245, G: 245, B: 245, A: 255})
	base := newRuntimeInteractiveFacet(color.RGBA{R: 75, G: 140, B: 220, A: 255})
	overlay := newRuntimeInteractiveFacet(color.RGBA{R: 220, G: 120, B: 120, A: 255})
	var baseCalls, dismissCalls int
	base.input.OnPointer = func(facet.PointerEvent) bool {
		baseCalls++
		return true
	}
	overlay.input.OnDismiss = func(e facet.DismissEvent) bool {
		dismissCalls++
		return true
	}
	overlay.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 160, H: 160}}
	}
	overlay.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		overlay.layout.ArrangedBounds = bounds
	}
	rt, app := newPhase17Runtime(t, root)
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	overlayLayer, ok := rt.layerRegistry.LookupName("app.overlay")
	if !ok {
		t.Fatal("missing overlay layer")
	}
	rt.AddFacet(root, overlay, facet.Attachment{LayerID: facet.LayerID(overlayLayer.ID)})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	app.Events().Push(platform.EventPointer{
		Kind:     platform.PointerPress,
		Position: gfx.Point{X: 300, Y: 300},
		Button:   platform.PointerLeft,
	})
	rt.RunOneFrame()
	if dismissCalls != 1 {
		t.Fatalf("dismissCalls = %d, want 1", dismissCalls)
	}
	if baseCalls != 0 {
		t.Fatalf("baseCalls = %d, want 0", baseCalls)
	}
	rt.Shutdown()
}

func TestRuntime_crossSeam_layout_projection_input_diagnostics(t *testing.T) {
	root := newRuntimeInteractiveFacet(color.RGBA{R: 246, G: 246, B: 246, A: 255})
	child := newRuntimeInteractiveFacet(color.RGBA{R: 95, G: 145, B: 225, A: 255})
	child.input.OnPointer = func(facet.PointerEvent) bool { return true }
	rt, app := newPhase17Runtime(t, root)
	rt.EnableHitTrace(true)
	rt.AddFacet(root, child, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	if err := rt.start(); err != nil {
		t.Fatalf("start: %v", err)
	}
	rt.RunOneFrame()
	app.Events().Push(platform.EventPointer{Kind: platform.PointerPress, Position: gfx.Point{X: 20, Y: 20}, Button: platform.PointerLeft})
	rt.RunOneFrame()

	insp := diagnostics.NewInspector(root)
	insp.SetLayerSource(rt)
	insp.SetAnchorSource(rt)
	insp.SetHitTraceSource(rt)
	if got := rt.HitTest(gfx.Point{X: 20, Y: 20}); got == 0 {
		t.Fatal("expected a hit result")
	}
	layers := insp.LayerSnapshots(root.ID())
	if len(layers) == 0 {
		t.Fatal("expected layer snapshots")
	}
	if layers[0].LayerID == 0 || layers[0].RecipeVersion == 0 {
		t.Fatalf("layer snapshot = %#v", layers[0])
	}
	hit := insp.HitTrace()
	if len(hit.TestedLayers) == 0 || hit.Result == 0 {
		t.Fatalf("hit trace = %#v", hit)
	}
	if stats := rt.LastFrameStats(); stats.RenderBatchCount == 0 || stats.ProjectedFacets == 0 {
		t.Fatalf("frame stats = %#v", stats)
	}
	rt.Shutdown()
}

func TestRuntime_run_returns_render_error(t *testing.T) {
	root := newRuntimeRenderFacet("root", gfx.RectFromXYWH(0, 0, 100, 100), color.RGBA{A: 255})
	rt := mustRuntimeWithBackend(t, root, &backendFixture{submitErr: errors.New("boom")})
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
