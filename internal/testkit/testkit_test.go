package testkit

import (
	"bytes"
	"image/color"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
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
	f.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return c.MaxSize
	}
	f.layout.OnArrange = func(bounds gfx.Rect) {
		f.layout.ArrangedBounds = bounds
	}
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

func (f *testRenderFacet) Base() *facet.Facet               { return &f.Facet }
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
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	h.RunFrame()
	h.RunFrame()
	if h.FrameCount != 2 {
		t.Fatalf("count = %d", h.FrameCount)
	}
}

func TestHarness_creates_without_panic(t *testing.T) {
	data := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	h := NewHarness(t, HarnessConfig{
		Width:  320,
		Height: 240,
		Fonts:  []text.FontSource{{Name: "noto-sans", Data: data}},
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
		h = NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	})
	if h == nil || h.app == nil || !h.app.destroyed {
		t.Fatal("expected cleanup to destroy app")
	}
}

func TestHarness_run_until_returns_true_on_condition(t *testing.T) {
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	calls := 0
	if !h.RunUntil(func() bool {
		calls++
		return calls == 3
	}, 10) {
		t.Fatal("expected true")
	}
}

func TestHarness_run_until_returns_false_on_timeout(t *testing.T) {
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
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
	h := NewHarness(t, DefaultHarnessConfig(), root)
	h.RunFrame()
	before := got
	h.InjectEvent(PointerMove(10, 20))
	h.RunFrame()
	if got <= before {
		t.Fatalf("got = %d before = %d", got, before)
	}
}

func TestHarness_initial_frame_renders(t *testing.T) {
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	h.RunFrame()
	AssertNotBlank(t, h.Surface())
}

func TestHarness_frame_stats_populated(t *testing.T) {
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	h.RunFrame()
	stats := h.LastFrameStats()
	if stats.LayerCount == 0 {
		t.Fatalf("expected layer count, got %#v", stats)
	}
	if stats.ProjectedFacets == 0 {
		t.Fatalf("expected projected facets, got %#v", stats)
	}
}

func TestHarness_multiple_independent_instances(t *testing.T) {
	a := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
	b := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
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
	h := NewHarness(t, DefaultHarnessConfig(), newTestRenderFacet())
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

func TestAssertGolden_creates_on_first_run(t *testing.T) {
	old := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = old })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertGolden(t, s, "sample")
	if _, err := os.Stat(filepath.Join(goldenBaseDir, "sample.png")); err != nil {
		t.Fatalf("stat: %v", err)
	}
}

func TestAssertGolden_passes_on_match(t *testing.T) {
	old := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = old })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertGolden(t, s, "sample")
	AssertGolden(t, s, "sample")
}

func TestAssertGolden_fails_on_mismatch(t *testing.T) {
	old := goldenBaseDir
	goldenBaseDir = t.TempDir()
	t.Cleanup(func() { goldenBaseDir = old })

	s := NewMemorySurface(1, 1)
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 255
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	AssertGolden(t, s, "sample")
	if err := s.Lock(); err != nil {
		t.Fatalf("lock: %v", err)
	}
	s.Buffer()[0] = 0
	s.Buffer()[3] = 255
	if err := s.Unlock(nil); err != nil {
		t.Fatalf("unlock: %v", err)
	}
	r := &recordingTB{}
	AssertGolden(r, s, "sample")
	if len(r.errors) == 0 {
		t.Fatal("expected error")
	}
}

func mustReadTestFont(t *testing.T, rel string) []byte {
	t.Helper()
	path := mustTestFontPath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %q: %v", path, err)
	}
	return data
}

func mustTestFontPath(t *testing.T, rel string) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	path := filepath.Join(string(bytes.TrimSpace(out)), rel)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("test font path %q: %v", path, err)
	}
	return path
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
