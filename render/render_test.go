package render_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/render/vulkan"
)

type testSurface struct {
	w int
	h int
}

func (s *testSurface) Size() (width, height int) { return s.w, s.h }
func (s *testSurface) Resize(width, height int)  { s.w, s.h = width, height }

func TestRenderBackendInterface_vulkan_satisfies(t *testing.T) {
	var _ render.Backend = (*vulkan.Backend)(nil)
}

func TestFrame_dirty_regions_nil_safe(t *testing.T) {
	frame := render.Frame{}
	if frame.DirtyRegions != nil {
		t.Fatal("expected zero Frame dirty regions to be nil")
	}
	if got := len(frame.DirtyRegions); got != 0 {
		t.Fatalf("expected len(nil dirty regions) == 0, got %d", got)
	}
}

func TestRenderBatchID_zero_valid(t *testing.T) {
	if render.RenderBatchID(0) != 0 {
		t.Fatal("expected RenderBatchID(0) to be a valid zero value")
	}
}

func TestRenderBatch_commandhash_field_present(t *testing.T) {
	RenderBatch := render.RenderBatch{CommandHash: 0}
	if RenderBatch.CommandHash != 0 {
		t.Fatalf("expected zero command hash, got %d", RenderBatch.CommandHash)
	}
}

func TestRenderSurfaceInterface_satisfiable(t *testing.T) {
	var s render.Surface = &testSurface{w: 10, h: 20}
	if w, h := s.Size(); w != 10 || h != 20 {
		t.Fatalf("unexpected size: %d x %d", w, h)
	}
	s.Resize(30, 40)
	if w, h := s.Size(); w != 30 || h != 40 {
		t.Fatalf("unexpected resized size: %d x %d", w, h)
	}
}

func TestRenderRenderBatch_uses_gfx_types(t *testing.T) {
	RenderBatch := render.RenderBatch{
		ID:     7,
		Bounds: gfx.RectFromXYWH(1, 2, 3, 4),
	}
	if RenderBatch.Bounds.Width() != 3 || RenderBatch.Bounds.Height() != 4 {
		t.Fatalf("unexpected bounds: %+v", RenderBatch.Bounds)
	}
}
