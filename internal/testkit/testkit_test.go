package testkit

import (
	"image/color"
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
)

type testRenderFacet struct {
	facet.Facet
	input  facet.InputRole
	hit    facet.HitRole
	layout facet.LayoutRole
	render facet.RenderRole
}

func newTestRenderFacet() *testRenderFacet {
	f := &testRenderFacet{Facet: facet.NewFacet()}
	f.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{}
	}
	f.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: c.MaxSize}
	}
	f.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
	f.layout.Child.SupportedPlacement = facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree | facet.SupportsLinear
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 255, 255, 255)),
		})
	}
	f.AddRole(&f.input)
	f.AddRole(&f.hit)
	f.AddRole(&f.layout)
	f.AddRole(&f.render)
	return f
}

func testHarnessConfig(t testing.TB) HarnessConfig {
	t.Helper()
	reg, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	cfg := DefaultHarnessConfig()
	cfg.LayerRegistry = reg
	return cfg
}

func customInsertionRegistry(t testing.TB) *layout.LayerRegistry {
	t.Helper()
	b := layout.NewLayerRegistryBuilder()
	if err := b.RegisterStandardLayers(); err != nil {
		t.Fatalf("register standard layers: %v", err)
	}
	if _, err := b.RegisterLayer(layout.LayerRegistration{
		Name:  "app.card",
		Order: 2500,
	}); err != nil {
		t.Fatalf("register custom layer: %v", err)
	}
	reg, err := b.Freeze()
	if err != nil {
		t.Fatalf("freeze registry: %v", err)
	}
	return reg
}

func newColoredRenderFacet(fill color.RGBA) *testRenderFacet {
	f := newTestRenderFacet()
	f.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		list.Add(gfx.FillRect{
			Rect:  bounds,
			Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(fill.R, fill.G, fill.B, fill.A)),
		})
	}
	return f
}

func TestHarnessConfig_usesFrozenStandardRegistry(t *testing.T) {
	cfg := testHarnessConfig(t)
	if cfg.LayerRegistry == nil {
		t.Fatal("expected layer registry")
	}
	if got := len(cfg.LayerRegistry.OrderedLayers()); got == 0 {
		t.Fatal("expected frozen standard layers")
	}
	if _, ok := cfg.LayerRegistry.LookupName(layout.StandardLayerBase); !ok {
		t.Fatal("missing standard base layer")
	}
}

func (f *testRenderFacet) Base() *facet.Facet {
	f.BindImpl(f)
	return &f.Facet
}
func (f *testRenderFacet) OnAttach(ctx facet.AttachContext) {}
func (f *testRenderFacet) OnDetach()                        {}
func (f *testRenderFacet) OnActivate()                      {}
func (f *testRenderFacet) OnDeactivate()                    {}

type recordingTB struct {
	helperCalls int
	errors      []string
}

func (r *recordingTB) Helper() { r.helperCalls++ }

func (r *recordingTB) Errorf(format string, args ...any) {
	r.errors = append(r.errors, format)
}

func (r *recordingTB) Fatalf(format string, args ...any) {
	r.errors = append(r.errors, format)
}

func TestMemorySurface_buffer_only_between_lock_unlock(t *testing.T) {
	s := NewMemorySurface(2, 2)
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	_ = s.Buffer()
}

func TestMemorySurface_capture_is_copy(t *testing.T) {
	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	buf := s.Buffer()
	buf[0] = 255
	buf[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	img := s.Capture()
	img.Pix[0] = 0
	img2 := s.Capture()
	if img2.Pix[0] != 255 {
		t.Fatalf("capture mutated surface: %d", img2.Pix[0])
	}
}

func TestMemorySurface_pixel_at_correct_coordinates(t *testing.T) {
	s := NewMemorySurface(2, 2)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	buf := s.Buffer()
	buf[0] = 255
	buf[3] = 255
	buf[4] = 0
	buf[5] = 255
	buf[7] = 255
	buf[8] = 0
	buf[9] = 0
	buf[10] = 255
	buf[11] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	if got := s.PixelAt(0, 0); got != (color.RGBA{R: 255, A: 255}) {
		t.Fatalf("got %#v", got)
	}
	if got := s.PixelAt(1, 0); got != (color.RGBA{G: 255, A: 255}) {
		t.Fatalf("got %#v", got)
	}
	if got := s.PixelAt(0, 1); got != (color.RGBA{B: 255, A: 255}) {
		t.Fatalf("got %#v", got)
	}
}

func TestNullApp_inject_event_polled_next_frame(t *testing.T) {
	app := NewNullApp(640, 480)
	ev := PointerMove(10, 20)
	app.InjectEvent(ev)
	got := app.Events().Poll()
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].(platform.EventPointer); !ok {
		t.Fatalf("unexpected event type %T", got[0])
	}
}

func TestNullApp_event_queue_accepts_cross_thread_push(t *testing.T) {
	app := NewNullApp(640, 480)
	done := make(chan struct{})
	go func() {
		app.InjectEvent(platform.EventText{Text: "async"})
		close(done)
	}()
	got := app.Events().Wait(250 * time.Millisecond)
	<-done
	if len(got) != 1 {
		t.Fatalf("len = %d", len(got))
	}
	if ev, ok := got[0].(platform.EventText); !ok || ev.Text != "async" {
		t.Fatalf("unexpected event %#v", got[0])
	}
}

func TestNullEventQueue_preserves_fifo_order(t *testing.T) {
	q := newNullEventQueue(8)
	q.Push(platform.EventText{Text: "one"})
	q.Push(platform.EventText{Text: "two"})
	q.Push(platform.EventText{Text: "three"})
	got := q.Poll()
	if len(got) != 3 {
		t.Fatalf("len = %d", len(got))
	}
	for i, want := range []string{"one", "two", "three"} {
		ev, ok := got[i].(platform.EventText)
		if !ok || ev.Text != want {
			t.Fatalf("event %d = %#v, want %q", i, got[i], want)
		}
	}
}

func TestNullEventQueue_blocks_when_full(t *testing.T) {
	q := newNullEventQueue(1)
	q.Push(platform.EventText{Text: "first"})

	started := make(chan struct{})
	done := make(chan struct{})
	go func() {
		close(started)
		q.Push(platform.EventText{Text: "second"})
		close(done)
	}()

	<-started
	select {
	case <-done:
		t.Fatal("push returned before capacity was freed")
	case <-time.After(50 * time.Millisecond):
	}

	got := q.Poll()
	if len(got) != 1 {
		t.Fatalf("poll len = %d", len(got))
	}
	if ev, ok := got[0].(platform.EventText); !ok || ev.Text != "first" {
		t.Fatalf("unexpected first event %#v", got[0])
	}

	select {
	case <-done:
	case <-time.After(250 * time.Millisecond):
		t.Fatal("push did not unblock after poll")
	}

	got = q.Poll()
	if len(got) != 1 {
		t.Fatalf("second poll len = %d", len(got))
	}
	if ev, ok := got[0].(platform.EventText); !ok || ev.Text != "second" {
		t.Fatalf("unexpected second event %#v", got[0])
	}
}

func TestNullClipboard_write_read_roundtrip(t *testing.T) {
	var clip NullClipboard
	if err := clip.WriteText("x"); err != nil {
		t.Fatalf("write: %v", err)
	}
	got, err := clip.ReadText()
	if err != nil || got != "x" {
		t.Fatalf("got %q %v", got, err)
	}
}

func TestHarness_run_frame_increments_count(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	h.RunFrame()
	h.RunFrame()
	if h.FrameCount != 2 {
		t.Fatalf("count = %d", h.FrameCount)
	}
}

func TestHarness_creates_without_panic(t *testing.T) {
	data := TestFontBytes()
	h := NewHarness(t, HarnessConfig{
		Width:  320,
		Height: 240,
		Fonts:  []text.FontSource{{Name: "noto-sans", Data: data}},
		LayerRegistry: func() *layout.LayerRegistry {
			reg, err := layout.StandardLayerRegistry()
			if err != nil {
				t.Fatalf("standard layer registry: %v", err)
			}
			return reg
		}(),
	}, newTestRenderFacet())
	if h == nil || h.Runtime() == nil || h.Surface() == nil {
		t.Fatal("expected initialized harness")
	}
	if got := len(h.fonts.Sources()); got == 0 {
		t.Fatalf("fonts = %d", got)
	}
}

func TestHarness_cleanup_registered(t *testing.T) {
	var h *Harness
	t.Run("sub", func(t *testing.T) {
		h = NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	})
	if h == nil || h.app == nil || !h.app.destroyed {
		t.Fatal("expected cleanup to destroy app")
	}
}

func TestHarness_run_until_returns_true_on_condition(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	calls := 0
	if !h.RunUntil(func() bool {
		calls++
		return calls == 3
	}, 10) {
		t.Fatal("expected true")
	}
}

func TestHarness_run_until_returns_false_on_timeout(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	if h.RunUntil(func() bool { return false }, 2) {
		t.Fatal("expected false")
	}
}

func TestHarness_inject_event_visible_next_frame(t *testing.T) {
	root := newTestRenderFacet()
	got := 0
	root.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return facet.HitResult{Hit: true, MarkID: 1}
	}
	root.input.OnPointer = func(e facet.PointerEvent) bool {
		got++
		return true
	}
	h := NewHarness(t, testHarnessConfig(t), root)
	h.RunFrame()
	before := got
	h.InjectEvent(PointerMove(10, 20))
	h.RunFrame()
	if got <= before {
		t.Fatalf("got = %d before = %d", got, before)
	}
}

func TestHarness_initial_frame_renders(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	h.RunFrame()
	AssertNotBlank(t, h.Surface())
}

func TestHarness_frame_stats_populated(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	h.RunFrame()
	stats := h.LastFrameStats()
	if stats.RenderBatchCount == 0 {
		t.Fatalf("expected RenderBatch count, got %#v", stats)
	}
	if stats.ProjectedFacets == 0 {
		t.Fatalf("expected projected facets, got %#v", stats)
	}
}

func TestHarness_multiple_independent_instances(t *testing.T) {
	a := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	b := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	a.RunFrame()
	b.RunFrame()
	if a.FrameCount != 1 || b.FrameCount != 1 {
		t.Fatalf("counts = %d,%d", a.FrameCount, b.FrameCount)
	}
	if a.Surface() == b.Surface() {
		t.Fatal("expected distinct surfaces")
	}
}

func TestHarness_projection_cache_stable(t *testing.T) {
	h := NewHarness(t, testHarnessConfig(t), newTestRenderFacet())
	h.RunFrame()
	first := h.LastFrameStats()
	h.RunFrame()
	second := h.LastFrameStats()
	if first.ProjectedFacets == 0 {
		t.Fatalf("expected initial projections, got %#v", first)
	}
	if second.ProjectedFacets != 0 {
		t.Fatalf("expected cache reuse on second frame, got %#v", second)
	}
}

func TestGoldenOrdering_standardStack(t *testing.T) {
	root := newColoredRenderFacet(color.RGBA{R: 248, G: 248, B: 248, A: 255})
	base := newColoredRenderFacet(color.RGBA{R: 75, G: 140, B: 255, A: 220})
	floating := newColoredRenderFacet(color.RGBA{R: 255, G: 120, B: 64, A: 180})
	modal := newColoredRenderFacet(color.RGBA{R: 80, G: 200, B: 120, A: 180})

	h := NewHarness(t, testHarnessConfig(t), root)
	rt := h.Runtime()
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDBase)})
	rt.AddFacet(root, floating, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDFloating)})
	rt.AddFacet(root, modal, facet.Attachment{LayerID: facet.LayerID(layout.StandardLayerIDModal)})
	h.RunFrame()

	AssertGolden(t, h.Surface(), "ordering_standard_stack")
}

func TestGoldenOrdering_customInsertion(t *testing.T) {
	root := newColoredRenderFacet(color.RGBA{R: 246, G: 246, B: 246, A: 255})
	base := newColoredRenderFacet(color.RGBA{R: 64, G: 128, B: 224, A: 220})
	card := newColoredRenderFacet(color.RGBA{R: 122, G: 196, B: 96, A: 190})
	floating := newColoredRenderFacet(color.RGBA{R: 230, G: 96, B: 72, A: 180})

	cfg := testHarnessConfig(t)
	cfg.LayerRegistry = customInsertionRegistry(t)
	h := NewHarness(t, cfg, root)
	rt := h.Runtime()
	reg := cfg.LayerRegistry
	baseLayer, ok := reg.LookupName(layout.StandardLayerBase)
	if !ok {
		t.Fatal("missing base layer")
	}
	cardLayer, ok := reg.LookupName("app.card")
	if !ok {
		t.Fatal("missing custom card layer")
	}
	floatingLayer, ok := reg.LookupName(layout.StandardLayerFloating)
	if !ok {
		t.Fatal("missing floating layer")
	}
	rt.AddFacet(root, base, facet.Attachment{LayerID: facet.LayerID(baseLayer.ID)})
	rt.AddFacet(root, card, facet.Attachment{LayerID: facet.LayerID(cardLayer.ID)})
	rt.AddFacet(root, floating, facet.Attachment{LayerID: facet.LayerID(floatingLayer.ID)})
	h.RunFrame()

	AssertGolden(t, h.Surface(), "ordering_custom_insertion")
}

func TestAssertPixelColor_passes_on_match(t *testing.T) {
	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertPixelColor(t, s, 0, 0, color.RGBA{R: 255, A: 255}, 0)
}

func TestAssertPixelColor_fails_on_mismatch(t *testing.T) {
	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	r := &recordingTB{}
	AssertPixelColor(r, s, 0, 0, color.RGBA{R: 255, A: 255}, 0)
	if len(r.errors) == 0 {
		t.Fatal("expected error")
	}
}

func TestAssertRegionColor_passes_solid_fill(t *testing.T) {
	s := NewMemorySurface(2, 2)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	buf := s.Buffer()
	for i := range buf {
		buf[i] = 255
	}
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertRegionColor(t, s, gfx.RectFromXYWH(0, 0, 2, 2), color.RGBA{R: 255, G: 255, B: 255, A: 255}, 0)
}

func TestAssertNotBlank_fails_on_blank_surface(t *testing.T) {
	s := NewMemorySurface(1, 1)
	r := &recordingTB{}
	AssertNotBlank(r, s)
	if len(r.errors) == 0 {
		t.Fatal("expected error")
	}
}

func TestSyntheticEvents_leftclick_is_press_release(t *testing.T) {
	got := LeftClick(1, 2)
	if len(got) != 2 {
		t.Fatalf("len = %d", len(got))
	}
}

func TestSyntheticEvents_drag_has_intermediate_moves(t *testing.T) {
	got := Drag(0, 0, 10, 10)
	if len(got) != 7 {
		t.Fatalf("len = %d", len(got))
	}
	if _, ok := got[0].(platform.EventPointer); !ok {
		t.Fatalf("unexpected first event %T", got[0])
	}
}
