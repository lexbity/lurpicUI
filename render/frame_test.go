package render_test

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// TestFrame_packet_layer_shape pins the Q6 frame-output-assembly shape: a
// Frame carries a layer-ordered FramePacket whose LayeredBatch groups batches
// by (RenderOrder, ClipRect), and the flat RenderBatchs are retained in the
// same back-to-front order.
func TestFrame_packet_layer_shape(t *testing.T) {
	frame := render.Frame{
		FramePacket: render.FramePacket{
			Layers: []render.LayeredBatch{
				{RenderOrder: 1000, ClipRect: gfx.RectFromXYWH(0, 0, 10, 10), Batches: []render.RenderBatch{{ID: 1}}},
				{RenderOrder: 7000, ClipRect: gfx.Rect{}, Batches: []render.RenderBatch{{ID: 2}, {ID: 3}}},
			},
		},
		RenderBatchs: []render.RenderBatch{{ID: 1}, {ID: 2}, {ID: 3}},
	}
	if len(frame.Layers) != 2 {
		t.Fatalf("Layers = %d, want 2", len(frame.Layers))
	}
	if frame.Layers[0].RenderOrder != 1000 || frame.Layers[0].ClipRect.Width() != 10 {
		t.Fatalf("base layer = %+v", frame.Layers[0])
	}
	if len(frame.Layers[1].Batches) != 2 {
		t.Fatalf("modal layer batch count = %d, want 2", len(frame.Layers[1].Batches))
	}
	if len(frame.RenderBatchs) != 3 {
		t.Fatalf("RenderBatchs = %d, want 3", len(frame.RenderBatchs))
	}
}

// TestFrame_batch_id_bounds_shape pins the RenderBatch shape used by assembly:
// each batch carries its facet-derived ID, bounds, opacity, command list, and a
// stable command hash.
func TestFrame_batch_id_bounds_shape(t *testing.T) {
	b := render.RenderBatch{
		ID:      42,
		Bounds:  gfx.RectFromXYWH(5, 6, 20, 30),
		Opacity: 0.5,
	}
	b.Commands.Add(gfx.FillRect{Rect: b.Bounds, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 0, 0, 255))})
	if b.ID != 42 {
		t.Fatalf("ID = %d, want 42", b.ID)
	}
	if b.Bounds.Width() != 20 || b.Bounds.Height() != 30 {
		t.Fatalf("Bounds = %+v", b.Bounds)
	}
	if len(b.Commands.Commands) != 1 {
		t.Fatalf("Commands = %d, want 1", len(b.Commands.Commands))
	}
}

// TestFrame_packet_empty_nil_safe pins the zero-Frame shape: a fresh Frame has
// nil layers, nil batches, and nil dirty regions — safe to submit.
func TestFrame_packet_empty_nil_safe(t *testing.T) {
	var frame render.Frame
	if frame.Layers != nil || frame.RenderBatchs != nil || frame.DirtyRegions != nil {
		t.Fatal("expected zero Frame to be nil-safe across all slices")
	}
}
