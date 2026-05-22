package software

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"github.com/go-text/typesetting/font"
	ot "github.com/go-text/typesetting/font/opentype"
)

type testSurface struct {
	buf    []byte
	stride int
	w      int
	h      int
}

func newTestSurface(w, h int) *testSurface {
	return &testSurface{
		buf:    make([]byte, w*h*4),
		stride: w * 4,
		w:      w,
		h:      h,
	}
}

func (s *testSurface) Buffer() []byte { return s.buf }
func (s *testSurface) Stride() int    { return s.stride }
func (s *testSurface) Size() (width, height int) {
	return s.w, s.h
}
func (s *testSurface) Resize(width, height int) {
	s.w = width
	s.h = height
	s.stride = width * 4
	s.buf = make([]byte, width*height*4)
}
func (s *testSurface) Lock() error { return nil }
func (s *testSurface) Unlock([]gfx.Rect) error {
	return nil
}

type shrinkingSurface struct {
	*testSurface
	shrinkToW int
	shrinkToH int
	shrunk    bool
}

type genericSurface struct {
	w int
	h int
}

func (s *genericSurface) Size() (width, height int) { return s.w, s.h }
func (s *genericSurface) Resize(width, height int) {
	s.w = width
	s.h = height
}

func (s *shrinkingSurface) Lock() error {
	if !s.shrunk {
		s.Resize(s.shrinkToW, s.shrinkToH)
		s.shrunk = true
	}
	return nil
}

func newRenderer(t *testing.T, w, h int) (*SoftwareRenderer, *testSurface) {
	t.Helper()
	surf := newTestSurface(w, h)
	r := NewSoftwareRenderer()
	if err := r.Initialize(surf); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	return r, surf
}

func solidRenderBatch(id render.RenderBatchID, bounds gfx.Rect, hash uint64, c gfx.Color) render.RenderBatch {
	return render.RenderBatch{
		ID:          id,
		Bounds:      bounds,
		Opacity:     1,
		CommandHash: hash,
		Commands: gfx.CommandList{Commands: []gfx.Command{
			gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, bounds.Width(), bounds.Height()), Brush: gfx.SolidBrush(c)},
		}},
	}
}

func pxAt(s *testSurface, x, y int) color.RGBA {
	off := y*s.stride + x*4
	return color.RGBA{R: s.buf[off], G: s.buf[off+1], B: s.buf[off+2], A: s.buf[off+3]}
}

func TestSoftwareRenderer_fillrect_solid(t *testing.T) {
	r, s := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 10, 10), 1, gfx.Color{R: 1, A: 1}),
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 5, 5); got != (color.RGBA{R: 255, G: 0, B: 0, A: 255}) {
		t.Fatalf("inside pixel mismatch: %#v", got)
	}
	if got := pxAt(s, 15, 15); got != (color.RGBA{}) {
		t.Fatalf("outside pixel mismatch: %#v", got)
	}
}

func TestSoftwareRenderer_evict_caches_clears_recoverable_state(t *testing.T) {
	r, _ := newRenderer(t, 20, 20)
	r.RenderBatchCache[1] = &RenderBatchCacheEntry{buffer: image.NewRGBA(image.Rect(0, 0, 1, 1))}
	r.rasterizeCount = 3
	r.EvictCaches()
	if len(r.RenderBatchCache) != 0 {
		t.Fatalf("expected render batch cache cleared, got %d", len(r.RenderBatchCache))
	}
	if r.diffCache == nil {
		t.Fatal("expected diff cache reinitialized")
	}
	if r.glyphAtlas == nil || len(r.glyphAtlas.entries) != 0 {
		t.Fatal("expected glyph atlas cleared")
	}
	if r.rasterizeCount != 0 {
		t.Fatalf("expected rasterize count reset, got %d", r.rasterizeCount)
	}
}

func TestSoftwareRenderer_initialize_rejects_generic_surface(t *testing.T) {
	r := NewSoftwareRenderer()
	if err := r.Initialize(&genericSurface{w: 20, h: 20}); err == nil {
		t.Fatal("expected initialize to reject a non-software surface")
	}
}

func TestSoftwareRenderer_fillrect_alpha_blend(t *testing.T) {
	r, s := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 20, 20), 1, gfx.Color{R: 1, G: 1, B: 1, A: 1}),
			{
				ID:          2,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 0.5})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := pxAt(s, 5, 5)
	want := color.RGBA{R: 255, G: 128, B: 128, A: 255}
	if !approxRGBA(got, want, 2) {
		t.Fatalf("blend mismatch: got %#v want %#v", got, want)
	}
}

func TestSoftwareRenderer_fillpath_antialiased(t *testing.T) {
	r, s := newRenderer(t, 32, 32)
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 4.25, Y: 4.25}).
		LineTo(gfx.Point{X: 24.75, Y: 5.1}).
		LineTo(gfx.Point{X: 10.5, Y: 24.75}).
		Close().
		Build()
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 32, 32),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillPath{Path: path, Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasPartialAlpha(s, 4, 4, 25, 25) {
		t.Fatalf("expected antialiased edge coverage")
	}
}

func TestSoftwareRenderer_strokepath_rasterizes(t *testing.T) {
	r, s := newRenderer(t, 32, 32)
	path := gfx.NewPath().
		MoveTo(gfx.Point{X: 4, Y: 4}).
		LineTo(gfx.Point{X: 28, Y: 20}).
		Build()
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 32, 32),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.StrokePath{Path: path, Stroke: gfx.DefaultStroke(4), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasNonBlankPixels(s, 4, 4, 28, 20) {
		t.Fatalf("expected stroked path pixels")
	}
}

func TestSoftwareRenderer_rendersPrimitiveIcon(t *testing.T) {
	tokens := theme.DefaultTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(18, 52, 86, 255)
	rt := iconRenderRuntime{rootStyle: theme.NewRootStyleContext(nil, tokens, nil)}
	icon := primitive.NewIcon(primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="currentColor"><path d="M1 1H9V9H1Z"/></svg>`))
	icon.SetAccessibleName("Square")
	icon.SetColorSlot(theme.ColorPrimary)
	facet.Attach(icon, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	size := icon.Base().LayoutRole().Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 64, H: 64}}).Size
	bounds := gfx.RectFromXYWH(0, 0, size.W, size.H)
	icon.Base().LayoutRole().Arrange(facet.ArrangeContext{}, bounds)
	cmds := icon.Base().ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) == 0 {
		t.Fatal("expected icon commands")
	}
	r, s := newRenderer(t, int(size.W), int(size.H))
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    *cmds,
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := pxAt(s, int(size.W/2), int(size.H/2))
	if got.A == 0 {
		t.Fatalf("expected icon output to render opaque pixels, got %#v", got)
	}
}

func TestSoftwareRenderer_transform_stack_translates(t *testing.T) {
	r, s := newRenderer(t, 150, 150)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 150, 150),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushTransform{Matrix: gfx.Translation(100, 100)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 105, 105); got.R != 255 || got.A != 255 {
		t.Fatalf("translated pixel missing: %#v", got)
	}
	if got := pxAt(s, 5, 5); got != (color.RGBA{}) {
		t.Fatalf("unexpected pixel before translation: %#v", got)
	}
}

type iconRenderRuntime struct {
	rootStyle any
}

func (s iconRenderRuntime) Schedule(j job.AnyJob)  {}
func (s iconRenderRuntime) CancelJob(id job.JobID) {}
func (s iconRenderRuntime) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s iconRenderRuntime) RootStyleContext() any { return s.rootStyle }
func (s iconRenderRuntime) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}

func TestSoftwareRenderer_batch_bounds_are_localized(t *testing.T) {
	r, s := newRenderer(t, 80, 80)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(20, 30, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(20, 30, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 25, 35); got.G != 255 || got.A != 255 {
		t.Fatalf("localized batch pixel missing: %#v", got)
	}
	if got := pxAt(s, 5, 5); got != (color.RGBA{}) {
		t.Fatalf("unexpected pixel before localized region: %#v", got)
	}
}

func TestSoftwareRenderer_clip_rect_clips(t *testing.T) {
	r, s := newRenderer(t, 100, 100)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 50, 50)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 100, 100), Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 25, 25); got.B != 255 {
		t.Fatalf("clipped inside pixel missing: %#v", got)
	}
	if got := pxAt(s, 75, 75); got != (color.RGBA{}) {
		t.Fatalf("clipped outside pixel should be transparent: %#v", got)
	}
}

func TestSoftwareRenderer_clip_rect_translates_with_batch_bounds(t *testing.T) {
	r, s := newRenderer(t, 100, 100)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(20, 30, 40, 40),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(20, 30, 20, 20)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(20, 30, 40, 40), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 25, 35); got.R != 255 || got.A != 255 {
		t.Fatalf("translated clip inside pixel missing: %#v", got)
	}
	if got := pxAt(s, 45, 55); got != (color.RGBA{}) {
		t.Fatalf("translated clip outside pixel should be transparent: %#v", got)
	}
}

func TestSoftwareRenderer_opacity_stack(t *testing.T) {
	r, s := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushOpacity{Alpha: 0.5},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 20), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := pxAt(s, 10, 10)
	want := color.RGBA{R: 0, G: 128, B: 0, A: 128}
	if !approxRGBA(got, want, 2) {
		t.Fatalf("opacity mismatch: got %#v want %#v", got, want)
	}
}

func TestSoftwareRenderer_RenderBatch_caching_skip(t *testing.T) {
	r, _ := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 10, 10), 1, gfx.Color{R: 1, A: 1}),
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit1: %v", err)
	}
	first := r.RasterizeCount()
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit2: %v", err)
	}
	if got := r.RasterizeCount(); got != first {
		t.Fatalf("expected second submit to skip rasterization, got %d -> %d", first, got)
	}
}

func TestSoftwareRenderer_RenderBatch_compositing_order(t *testing.T) {
	r, s := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 20, 20), 1, gfx.Color{R: 1, A: 1}),
			{
				ID:          2,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 5, 5); got.B != 255 || got.R != 0 {
		t.Fatalf("top-left compositing mismatch: %#v", got)
	}
	if got := pxAt(s, 15, 15); got.R != 255 || got.B != 0 {
		t.Fatalf("background compositing mismatch: %#v", got)
	}
}

func TestSoftwareRenderer_multiRenderBatch_opacity(t *testing.T) {
	r, s := newRenderer(t, 20, 20)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 20, 20), 1, gfx.Color{R: 1, A: 1}),
			{
				ID:          2,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     0.5,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 20), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	got := pxAt(s, 10, 10)
	want := color.RGBA{R: 128, G: 128, B: 0, A: 255}
	if !approxRGBA(got, want, 2) {
		t.Fatalf("opacity composite mismatch: got %#v want %#v", got, want)
	}
}

func TestSoftwareRenderer_drawimage_nearest(t *testing.T) {
	r, s := newRenderer(t, 8, 8)
	src := image.NewRGBA(image.Rect(0, 0, 2, 2))
	src.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	src.SetRGBA(1, 0, color.RGBA{G: 255, A: 255})
	src.SetRGBA(0, 1, color.RGBA{B: 255, A: 255})
	src.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 8, 8),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawImage{
						Image:    src,
						DestRect: gfx.RectFromXYWH(0, 0, 4, 4),
						SrcRect:  gfx.RectFromXYWH(0, 0, 2, 2),
						Sampling: gfx.SamplingNearest,
						Opacity:  1,
					},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 0, 0); got.R != 255 || got.G != 0 || got.B != 0 {
		t.Fatalf("top-left mismatch: %#v", got)
	}
	if got := pxAt(s, 3, 0); got.G != 255 || got.R != 0 {
		t.Fatalf("top-right mismatch: %#v", got)
	}
	if got := pxAt(s, 0, 3); got.B != 255 || got.R != 0 {
		t.Fatalf("bottom-left mismatch: %#v", got)
	}
}

func TestSoftwareRenderer_resize_reallocates(t *testing.T) {
	r, s := newRenderer(t, 100, 100)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 100, 100), 1, gfx.Color{R: 1, A: 1}),
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit1: %v", err)
	}
	if err := r.Resize(200, 200); err != nil {
		t.Fatalf("resize: %v", err)
	}
	frame = &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 200, 200), 2, gfx.Color{G: 1, A: 1}),
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit2: %v", err)
	}
	if s.w != 200 || s.h != 200 || len(s.buf) != 200*200*4 {
		t.Fatalf("surface not resized: %+v", s)
	}
	if got := pxAt(s, 199, 199); got.G != 255 {
		t.Fatalf("resized pixel missing: %#v", got)
	}
}

func TestSoftwareRenderer_blit_clamps_to_live_surface_size(t *testing.T) {
	base := newTestSurface(20, 20)
	surf := &shrinkingSurface{
		testSurface: base,
		shrinkToW:   10,
		shrinkToH:   10,
	}
	r := NewSoftwareRenderer()
	if err := r.Initialize(surf); err != nil {
		t.Fatalf("initialize: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			solidRenderBatch(1, gfx.RectFromXYWH(0, 0, 20, 20), 1, gfx.Color{R: 1, A: 1}),
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if surf.w != 10 || surf.h != 10 || len(surf.buf) != 10*10*4 {
		t.Fatalf("surface not shrunk as expected: %+v", surf.testSurface)
	}
	if got := pxAt(surf.testSurface, 5, 5); got.R != 255 || got.A != 255 {
		t.Fatalf("expected preserved blit after resize, got %#v", got)
	}
}

func TestSoftwareRenderer_unbalanced_push_pop(t *testing.T) {
	r, s := newRenderer(t, 150, 150)
	frame1 := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 150, 150),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushTransform{Matrix: gfx.Translation(100, 100)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame1); err != nil {
		t.Fatalf("submit1: %v", err)
	}
	frame2 := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 150, 150),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame2); err != nil {
		t.Fatalf("submit2: %v", err)
	}
	if got := pxAt(s, 5, 5); got.B != 255 || got.R != 0 {
		t.Fatalf("expected second frame to reset transform state, got %#v", got)
	}
}

func TestSoftwareRenderer_drawglyphrun_produces_pixels(t *testing.T) {
	r, s := newRenderer(t, 80, 80)
	run := testGlyphRun(t, "Hello", 18)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 80, 80),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{
						Run:    run,
						Origin: gfx.Point{X: 10, Y: 10},
						Brush:  gfx.SolidBrush(gfx.Color{R: 1, A: 1}),
					},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasNonBlankPixels(s, 10, 10, 40, 40) {
		t.Fatalf("expected glyph pixels in rendered region")
	}
}

func TestSoftwareRenderer_drawglyphrun_color_matches_brush(t *testing.T) {
	r, s := newRenderer(t, 80, 80)
	run := testGlyphRun(t, "Hi", 18)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 80, 80),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{
						Run:    run,
						Origin: gfx.Point{X: 8, Y: 24},
						Brush:  gfx.SolidBrush(gfx.Color{R: 1, A: 1}),
					},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasDominantRedPixels(s, 8, 8, 32, 32) {
		t.Fatalf("expected red-dominant glyph pixels")
	}
}

func TestSoftwareRenderer_drawglyphrun_atlas_caches(t *testing.T) {
	r, _ := newRenderer(t, 80, 80)
	run := testGlyphRun(t, "A", 18)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 80, 80),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 10, Y: 10}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
			{
				ID:          2,
				Bounds:      gfx.RectFromXYWH(0, 0, 80, 80),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 20, Y: 10}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := r.GlyphRasterizeCount(); got != 1 {
		t.Fatalf("expected one glyph rasterization, got %d", got)
	}
}

func TestSoftwareRenderer_drawglyphrun_clipped_by_cliprect(t *testing.T) {
	r, s := newRenderer(t, 100, 100)
	run := testGlyphRun(t, "Clip", 28)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushClipRect{Rect: gfx.RectFromXYWH(0, 0, 20, 100)},
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 0, Y: 0}, Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasNonBlankPixels(s, 0, 0, 20, 40) {
		t.Fatalf("expected clipped glyph pixels on left side")
	}
	if hasNonBlankPixels(s, 25, 0, 60, 40) {
		t.Fatalf("expected no glyph pixels outside clip rect")
	}
}

func TestSoftwareRenderer_drawglyphrun_translated(t *testing.T) {
	r, s := newRenderer(t, 100, 100)
	run := testGlyphRun(t, "Move", 18)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 100),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushTransform{Matrix: gfx.Translation(40, 15)},
					gfx.DrawGlyphRun{Run: run, Origin: gfx.Point{X: 0, Y: 0}, Brush: gfx.SolidBrush(gfx.Color{B: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if !hasNonBlankPixels(s, 40, 15, 70, 45) {
		t.Fatalf("expected translated glyph pixels")
	}
	if hasNonBlankPixels(s, 0, 0, 20, 20) {
		t.Fatalf("unexpected pixels before translation")
	}
}

func TestRasterizeBitmapGlyph_uses_native_size_and_extents(t *testing.T) {
	entry := rasterizeBitmapGlyph(
		font.GlyphBitmap{
			Data:   []byte{0b10000100},
			Format: font.BlackAndWhite,
			Width:  2,
			Height: 3,
		},
		font.GlyphExtents{XBearing: 10, YBearing: 20},
		true,
		1,
	)
	if entry == nil || entry.bitmap == nil {
		t.Fatal("expected rasterized bitmap entry")
	}
	if got := entry.bitmap.Rect.Dx(); got != 2 {
		t.Fatalf("bitmap width mismatch: got %d want 2", got)
	}
	if got := entry.bitmap.Rect.Dy(); got != 3 {
		t.Fatalf("bitmap height mismatch: got %d want 3", got)
	}
	if entry.offsetX != 10 {
		t.Fatalf("offsetX mismatch: got %v want 10", entry.offsetX)
	}
	if entry.offsetY != -20 {
		t.Fatalf("offsetY mismatch: got %v want -20", entry.offsetY)
	}
	if got := entry.bitmap.AlphaAt(0, 0).A; got == 0 {
		t.Fatal("expected decoded alpha coverage at top-left")
	}
	if got := entry.bitmap.AlphaAt(1, 2).A; got == 0 {
		t.Fatal("expected decoded alpha coverage at bottom-right")
	}
}

func TestRasterizeOutlineGlyph_offsets_include_padding(t *testing.T) {
	entry := rasterizeOutlineGlyph(
		font.GlyphOutline{
			Segments: []ot.Segment{
				{
					Op: ot.SegmentOpMoveTo,
					Args: [3]ot.SegmentPoint{
						{X: 0, Y: 0},
					},
				},
				{
					Op: ot.SegmentOpLineTo,
					Args: [3]ot.SegmentPoint{
						{X: 10, Y: 0},
					},
				},
				{
					Op: ot.SegmentOpLineTo,
					Args: [3]ot.SegmentPoint{
						{X: 10, Y: 10},
					},
				},
				{
					Op: ot.SegmentOpLineTo,
					Args: [3]ot.SegmentPoint{
						{X: 0, Y: 10},
					},
				},
			},
		},
		font.GlyphExtents{XBearing: 0, YBearing: 10},
		true,
		1,
	)
	if entry == nil || entry.bitmap == nil {
		t.Fatal("expected rasterized outline entry")
	}
	if entry.offsetX != -1 {
		t.Fatalf("offsetX mismatch: got %v want -1", entry.offsetX)
	}
	if entry.offsetY != -11 {
		t.Fatalf("offsetY mismatch: got %v want -11", entry.offsetY)
	}
}

func TestRasterizeBitmapGlyph_preserves_color_images(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 1, 1))
	src.SetNRGBA(0, 0, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
	var encoded bytes.Buffer
	if err := png.Encode(&encoded, src); err != nil {
		t.Fatalf("encode png: %v", err)
	}

	entry := rasterizeBitmapGlyph(
		font.GlyphBitmap{
			Data:   encoded.Bytes(),
			Format: font.PNG,
			Width:  1,
			Height: 1,
		},
		font.GlyphExtents{XBearing: 4, YBearing: 7},
		true,
		1,
	)
	if entry == nil || !entry.color || entry.image == nil {
		t.Fatal("expected color bitmap entry")
	}
	if entry.offsetX != 4 {
		t.Fatalf("offsetX mismatch: got %v want 4", entry.offsetX)
	}
	if entry.offsetY != -7 {
		t.Fatalf("offsetY mismatch: got %v want -7", entry.offsetY)
	}
	r, g, b, a := entry.image.At(0, 0).RGBA()
	if r == 0 || g == 0 || b == 0 || a == 0 {
		t.Fatalf("expected preserved source color, got %d %d %d %d", r, g, b, a)
	}
}

func TestRasterizeBitmapGlyph_uses_raw_black_and_white_size(t *testing.T) {
	entry := rasterizeBitmapGlyph(
		font.GlyphBitmap{
			Data:   []byte{0b10010000},
			Format: font.BlackAndWhite,
			Width:  2,
			Height: 2,
		},
		font.GlyphExtents{},
		false,
		1,
	)
	if entry == nil || entry.bitmap == nil {
		t.Fatal("expected rasterized bitmap entry")
	}
	if got := entry.bitmap.Rect.Dx(); got != 2 {
		t.Fatalf("bitmap width mismatch: got %d want 2", got)
	}
	if got := entry.bitmap.Rect.Dy(); got != 2 {
		t.Fatalf("bitmap height mismatch: got %d want 2", got)
	}
	if got := entry.bitmap.AlphaAt(0, 0).A; got == 0 {
		t.Fatal("expected first bit to be set")
	}
	if got := entry.bitmap.AlphaAt(1, 1).A; got == 0 {
		t.Fatal("expected second bit to be set")
	}
}

func TestSoftwareRenderer_drawselectionrects_fills_region(t *testing.T) {
	r, s := newRenderer(t, 40, 40)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 40, 40),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawSelectionRects{
						Rects: []gfx.Rect{gfx.RectFromXYWH(5, 5, 10, 10)},
						Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1}),
					},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if got := pxAt(s, 8, 8); got.G != 255 {
		t.Fatalf("expected selection rect fill, got %#v", got)
	}
}

func TestSoftwareRenderer_drawglyphrun_empty_no_panic(t *testing.T) {
	r, s := newRenderer(t, 40, 40)
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 40, 40),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawGlyphRun{Run: text.GlyphRun{}, Origin: gfx.Point{X: 0, Y: 0}, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit: %v", err)
	}
	if hasNonBlankPixels(s, 0, 0, 40, 40) {
		t.Fatalf("empty glyph run should not draw pixels")
	}
}

func TestGlyphAtlas_independent_per_face_and_size(t *testing.T) {
	r, _ := newRenderer(t, 40, 40)
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	regular := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	boldItalic := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Bold.ttf")
	if err := reg.LoadFontBytes(regular, "roboto-regular"); err != nil {
		t.Fatalf("load face one: %v", err)
	}
	if err := reg.LoadFontBytes(boldItalic, "roboto-bolditalic"); err != nil {
		t.Fatalf("load face two: %v", err)
	}
	faceOne := reg.Resolve(text.TextStyle{Family: "Noto Sans", Weight: text.WeightRegular, Size: 12})
	faceTwo := reg.Resolve(text.TextStyle{Family: "Noto Sans", Weight: text.WeightBold, Size: 12})
	runOne := text.GlyphRun{
		Glyphs: []text.PositionedGlyph{{GlyphID: 65, Advance: 8}},
		Face:   faceOne,
		Size:   12,
		Style:  text.TextStyle{Size: 12},
	}
	runTwo := text.GlyphRun{
		Glyphs: []text.PositionedGlyph{{GlyphID: 65, Advance: 16}},
		Face:   faceTwo,
		Size:   24,
		Style:  text.TextStyle{Size: 24},
	}
	entryOne := r.glyphAtlas.getOrRasterize(runOne, runOne.Glyphs[0])
	entryTwo := r.glyphAtlas.getOrRasterize(runTwo, runTwo.Glyphs[0])
	if entryOne == nil || entryTwo == nil {
		t.Fatalf("expected atlas entries")
	}
	if entryOne == entryTwo {
		t.Fatalf("expected distinct atlas entries for face/size variations")
	}
	if got := r.GlyphRasterizeCount(); got != 2 {
		t.Fatalf("expected two glyph rasterizations, got %d", got)
	}
}

func testGlyphRun(t *testing.T, label string, size float32) text.GlyphRun {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("registry: %v", err)
	}
	data := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "roboto-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	layout := text.NewShaper(reg).ShapeSimple(label, text.TextStyle{Family: "Noto Sans", Size: size})
	if layout == nil || len(layout.Lines) == 0 || len(layout.Lines[0].Runs) == 0 {
		t.Fatalf("expected shaped run for %q", label)
	}
	return layout.Lines[0].Runs[0]
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

func hasNonBlankPixels(s *testSurface, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= s.w || y >= s.h {
				continue
			}
			if pxAt(s, x, y).A != 0 {
				return true
			}
		}
	}
	return false
}

func hasDominantRedPixels(s *testSurface, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= s.w || y >= s.h {
				continue
			}
			px := pxAt(s, x, y)
			if px.A != 0 && px.R > px.G && px.R > px.B {
				return true
			}
		}
	}
	return false
}

func hasPartialAlpha(s *testSurface, x0, y0, x1, y1 int) bool {
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			if x < 0 || y < 0 || x >= s.w || y >= s.h {
				continue
			}
			a := pxAt(s, x, y).A
			if a > 0 && a < 255 {
				return true
			}
		}
	}
	return false
}

func approxRGBA(a, b color.RGBA, tol uint8) bool {
	d := func(x, y uint8) uint8 {
		if x > y {
			return x - y
		}
		return y - x
	}
	return d(a.R, b.R) <= tol && d(a.G, b.G) <= tol && d(a.B, b.B) <= tol && d(a.A, b.A) <= tol
}
