package uinav

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
)

func TestScrollbar_thumb_size_matches_view_ratio(t *testing.T) {
	s := &Scrollbar{
		Orientation: ScrollbarVertical,
		Viewport: ViewportBinding{
			Offset:      store.NewBinding(0.0),
			Extent:      store.NewBinding(100.0),
			ContentSize: store.NewBinding(400.0),
		},
	}
	if got := s.thumbRatio(); got != 0.25 {
		t.Fatalf("thumb ratio = %v, want 0.25", got)
	}
}

func TestScrollbar_drag_updates_viewport_offset(t *testing.T) {
	s := &Scrollbar{
		Orientation: ScrollbarVertical,
		Viewport: ViewportBinding{
			Offset:      store.NewBinding(0.0),
			Extent:      store.NewBinding(100.0),
			ContentSize: store.NewBinding(400.0),
		},
	}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 6, Y: 10}}) {
		t.Fatal("expected press to be handled")
	}
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerMove, Position: gfx.Point{X: 6, Y: 180}}) {
		t.Fatal("expected move to be handled")
	}
	if got := s.Viewport.Offset.Get(); got <= 0 {
		t.Fatalf("offset = %v, want > 0", got)
	}
}

func TestScrollbar_track_click_pages_by_extent(t *testing.T) {
	s := &Scrollbar{
		Orientation: ScrollbarVertical,
		Viewport: ViewportBinding{
			Offset:      store.NewBinding(100.0),
			Extent:      store.NewBinding(100.0),
			ContentSize: store.NewBinding(400.0),
		},
	}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 6, Y: 0}}) {
		t.Fatal("expected track click to be handled")
	}
	if got := s.Viewport.Offset.Get(); got >= 100 {
		t.Fatalf("offset = %v, want page up", got)
	}
}
