package renderutil

import (
	"image"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestLayerCache_unchanged_detection(t *testing.T) {
	cache := NewLayerCache()
	frame := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	cache.Update(frame, nil)
	diff := cache.Diff(frame)
	if got := diff.Layers[1].Kind; got != LayerUnchanged {
		t.Fatalf("kind = %v, want unchanged", got)
	}
	if len(diff.CompositeDirtyRects) != 0 {
		t.Fatalf("expected no composite dirt, got %v", diff.CompositeDirtyRects)
	}
}

func TestLayerCache_added_detection(t *testing.T) {
	cache := NewLayerCache()
	frame := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	diff := cache.Diff(frame)
	if got := diff.Layers[1].Kind; got != LayerAdded {
		t.Fatalf("kind = %v, want added", got)
	}
}

func TestLayerCache_removed_detection(t *testing.T) {
	cache := NewLayerCache()
	frame1 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
			{
				ID:          2,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 20, 20), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	cache.Update(frame1, nil)
	frame2 := &render.Frame{
		Layers: frame1.Layers[:1],
	}
	diff := cache.Diff(frame2)
	if got := diff.Layers[2].Kind; got != LayerRemoved {
		t.Fatalf("kind = %v, want removed", got)
	}
}

func TestLayerCache_fullchange_on_bounds_change(t *testing.T) {
	cache := NewLayerCache()
	frame1 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	cache.Update(frame1, nil)
	frame2 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 20, 20),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	diff := cache.Diff(frame2)
	if got := diff.Layers[1].Kind; got != LayerFullChange {
		t.Fatalf("kind = %v, want full change", got)
	}
}

func TestLayerCache_partialchange_detection(t *testing.T) {
	cache := NewLayerCache()
	oldCmds := make([]gfx.Command, 10)
	newCmds := make([]gfx.Command, 10)
	for i := 0; i < 10; i++ {
		rect := gfx.RectFromXYWH(float32(i*10), 0, 10, 10)
		oldCmds[i] = gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})}
		newCmds[i] = oldCmds[i]
	}
	frame1 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: oldCmds},
			},
		},
	}
	cache.Update(frame1, nil)
	newCmds[3] = gfx.FillRect{Rect: gfx.RectFromXYWH(300, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})}
	frame2 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 100, 10),
				Opacity:     1,
				CommandHash: 2,
				Commands:    gfx.CommandList{Commands: newCmds},
			},
		},
	}
	diff := cache.Diff(frame2)
	layerDiff := diff.Layers[1]
	if layerDiff.Kind != LayerPartialChange {
		t.Fatalf("kind = %v, want partial change", layerDiff.Kind)
	}
	if len(layerDiff.DirtyRects) == 0 {
		t.Fatal("expected dirty rects")
	}
}

func TestLayerCache_complexTransform_forces_full(t *testing.T) {
	cache := NewLayerCache()
	frame1 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})},
				}},
			},
		},
	}
	cache.Update(frame1, nil)
	frame2 := &render.Frame{
		Layers: []render.Layer{
			{
				ID:          1,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     1,
				CommandHash: 2,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.PushTransform{Matrix: gfx.Translation(1, 0)},
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{G: 1, A: 1})},
				}},
			},
		},
	}
	diff := cache.Diff(frame2)
	if got := diff.Layers[1].Kind; got != LayerFullChange {
		t.Fatalf("kind = %v, want full change", got)
	}
}

func TestMergeRects_adjacent_rects_merged(t *testing.T) {
	rects := []gfx.Rect{
		gfx.RectFromXYWH(0, 0, 10, 10),
		gfx.RectFromXYWH(10, 0, 10, 10),
	}
	got := MergeRects(rects, 0.25)
	if len(got) != 1 || !rectEqual(got[0], gfx.RectFromXYWH(0, 0, 20, 10)) {
		t.Fatalf("got %#v", got)
	}
}

func TestMergeRects_distant_rects_not_merged(t *testing.T) {
	rects := []gfx.Rect{
		gfx.RectFromXYWH(0, 0, 10, 10),
		gfx.RectFromXYWH(200, 0, 10, 10),
	}
	got := MergeRects(rects, 0.25)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestMergeRects_single_rect_unchanged(t *testing.T) {
	rects := []gfx.Rect{gfx.RectFromXYWH(1, 2, 3, 4)}
	got := MergeRects(rects, 0.25)
	if len(got) != 1 || !rectEqual(got[0], rects[0]) {
		t.Fatalf("got %#v", got)
	}
	if !rectEqual(rects[0], gfx.RectFromXYWH(1, 2, 3, 4)) {
		t.Fatalf("input mutated: %#v", rects)
	}
}

func TestRemoveContained_fully_contained(t *testing.T) {
	rects := []gfx.Rect{
		gfx.RectFromXYWH(0, 0, 100, 100),
		gfx.RectFromXYWH(10, 10, 10, 10),
	}
	got := RemoveContained(rects)
	if len(got) != 1 || !rectEqual(got[0], rects[0]) {
		t.Fatalf("got %#v", got)
	}
}

func TestRemoveContained_partial_overlap_both_kept(t *testing.T) {
	rects := []gfx.Rect{
		gfx.RectFromXYWH(0, 0, 100, 100),
		gfx.RectFromXYWH(50, 50, 100, 100),
	}
	got := RemoveContained(rects)
	if len(got) != 2 {
		t.Fatalf("got %#v", got)
	}
}

func TestCommandBounds_fillrect(t *testing.T) {
	got := commandBounds(gfx.FillRect{Rect: gfx.RectFromXYWH(10, 10, 50, 50)})
	want := gfx.RectFromXYWH(10, 10, 50, 50)
	if !rectEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCommandBounds_strokerect_expands(t *testing.T) {
	got := commandBounds(gfx.StrokeRect{
		Rect:   gfx.RectFromXYWH(10, 10, 50, 50),
		Stroke: gfx.DefaultStroke(4),
	})
	want := gfx.RectFromXYWH(8, 8, 54, 54)
	if !rectEqual(got, want) {
		t.Fatalf("got %#v want %#v", got, want)
	}
}

func TestCommandBounds_state_commands_empty(t *testing.T) {
	if got := commandBounds(gfx.PushTransform{}); !got.IsEmpty() {
		t.Fatalf("got %#v", got)
	}
}

func TestPropagateDirty_through_transparent_layer(t *testing.T) {
	layers := []render.Layer{
		{
			ID:      1,
			Bounds:  gfx.RectFromXYWH(0, 0, 100, 100),
			Opacity: 1,
		},
		{
			ID:      2,
			Bounds:  gfx.RectFromXYWH(25, 25, 50, 50),
			Opacity: 0.5,
		},
	}
	perLayer := map[render.LayerID][]gfx.Rect{
		1: []gfx.Rect{
			gfx.RectFromXYWH(30, 30, 20, 20),
		},
	}
	got := PropagateDirty(layers, perLayer)
	if len(got) == 0 {
		t.Fatal("expected propagated dirt")
	}
	want := gfx.RectFromXYWH(30, 30, 20, 20)
	found := false
	for _, r := range got {
		if rectEqual(r, want) {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected overlap %v in %#v", want, got)
	}
}

func TestLayerCache_update_stores_buffers(t *testing.T) {
	cache := NewLayerCache()
	frame := &render.Frame{
		Layers: []render.Layer{
			{
				ID:      1,
				Bounds:  gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity: 1,
			},
		},
	}
	bufs := map[render.LayerID]*image.RGBA{
		1: image.NewRGBA(image.Rect(0, 0, 10, 10)),
	}
	cache.Update(frame, bufs)
	if _, ok := cache.layers[1]; !ok {
		t.Fatal("expected layer snapshot")
	}
}
