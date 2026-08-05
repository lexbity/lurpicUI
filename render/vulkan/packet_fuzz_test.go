package vulkan

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

// frameFromFuzzSeed deterministically builds a frame whose command stream is
// driven by the fuzz bytes: each byte selects an opcode and geometry derived
// from surrounding bytes. Commands that require the Rust store (glyph runs,
// images) are excluded so the fuzz loop stays pure Go.
func frameFromFuzzSeed(data []byte) *render.Frame {
	var cmds []gfx.Command
	drawable := false
	isDrawable := func(op uint8) bool {
		return op == packetCmdFillRect || op == packetCmdStrokeRect || op == packetCmdFillPath ||
			op == packetCmdStrokePath || op == packetCmdDrawPolyline || op == packetCmdDrawPoints ||
			op == packetCmdDrawSelectionRects || op == packetCmdDrawTexture || op == packetCmdDrawBlurredShadow
	}
	for i, b := range data {
		opcode := b % 19
		f := float32(len(data)) / 2
		col := gfx.Color{R: 0.5, G: 0.5, B: 0.5, A: 1}
		if isDrawable(opcode) {
			drawable = true
		}
		switch opcode {
		case packetCmdFillRect:
			cmds = append(cmds, gfx.FillRect{Rect: rectFromByte(b), Brush: gfx.SolidBrush(col)})
		case packetCmdStrokeRect:
			cmds = append(cmds, gfx.StrokeRect{Rect: rectFromByte(b), Stroke: gfx.DefaultStroke(f), Brush: gfx.SolidBrush(col)})
		case packetCmdFillPath:
			cmds = append(cmds, gfx.FillPath{Path: gfx.RectPath(rectFromByte(b)), Brush: gfx.SolidBrush(col)})
		case packetCmdStrokePath:
			cmds = append(cmds, gfx.StrokePath{Path: gfx.RectPath(rectFromByte(b)), Stroke: gfx.DefaultStroke(f), Brush: gfx.SolidBrush(col)})
		case packetCmdDrawPolyline:
			cmds = append(cmds, gfx.DrawPolyline{
				Points: []gfx.Point{{X: 0, Y: 0}, {X: f, Y: f}},
				Stroke: gfx.DefaultStroke(f),
				Brush:  gfx.SolidBrush(col),
				Closed: b%2 == 0,
			})
		case packetCmdDrawPoints:
			cmds = append(cmds, gfx.DrawPoints{Points: []gfx.Point{{X: f, Y: f}}, Radius: f, Brush: gfx.SolidBrush(col)})
		case packetCmdDrawSelectionRects:
			cmds = append(cmds, gfx.DrawSelectionRects{Rects: []gfx.Rect{rectFromByte(b)}, Brush: gfx.SolidBrush(col)})
		case packetCmdPushTransform:
			cmds = append(cmds, gfx.PushTransform{Matrix: gfx.Translation(f, f)})
		case packetCmdPopTransform:
			cmds = append(cmds, gfx.PopTransform{})
		case packetCmdPushClipRect:
			cmds = append(cmds, gfx.PushClipRect{Rect: rectFromByte(b)})
		case packetCmdPopClip:
			cmds = append(cmds, gfx.PopClip{})
		case packetCmdPushOpacity:
			cmds = append(cmds, gfx.PushOpacity{Alpha: f})
		case packetCmdPopOpacity:
			cmds = append(cmds, gfx.PopOpacity{})
		case packetCmdDrawTexture:
			cmds = append(cmds, gfx.DrawTexture{
				TextureID: uint64(i),
				DestRect:  rectFromByte(b),
				SrcRect:   rectFromByte(b >> 1),
			})
		case packetCmdDrawBlurredShadow:
			cmds = append(cmds, gfx.DrawBlurredShadow{
				Path:       gfx.RectPath(rectFromByte(b)),
				Color:      col,
				BlurRadius: f,
				Offset:     gfx.Point{X: f, Y: f},
				Inner:      b%2 == 0,
			})
		case packetCmdBeginRenderBatch:
			cmds = append(cmds, gfx.BeginRenderBatch{Bounds: rectFromByte(b), CacheID: gfx.RenderBatchCacheID(i)})
		case packetCmdEndRenderBatch:
			cmds = append(cmds, gfx.EndRenderBatch{})
		}
	}
	if len(cmds) == 0 || !drawable {
		cmds = append(cmds, gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 4, 4), Brush: gfx.SolidBrush(gfx.Color{R: 1, A: 1})})
	}
	return &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:       1,
				Bounds:   gfx.RectFromXYWH(0, 0, 128, 128),
				Opacity:  1,
				Commands: gfx.CommandList{Commands: cmds},
			},
		},
	}
}

func rectFromByte(b byte) gfx.Rect {
	w := float32(b%16) + 1
	h := float32((b>>4)%16) + 1
	return gfx.RectFromXYWH(0, 0, w, h)
}

// FuzzPacketV2_RoundTrip encodes a frame driven by arbitrary fuzz bytes and
// round-trips it through the Go mirror decoder. A mismatch (undecodable
// stream, trailing bytes, opcode drift) is a schema bug. The fuzz bytes are
// also fed raw to the decoder to prove it never panics on arbitrary input.
func FuzzPacketV2_RoundTrip(f *testing.F) {
	f.Add([]byte{})
	f.Add([]byte{0})
	f.Add([]byte{0, 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12})
	f.Add([]byte{15, 16, 17, 18})
	f.Fuzz(func(t *testing.T, data []byte) {
		// The mirror decoder must never panic on arbitrary input.
		func() {
			defer func() {
				if r := recover(); r != nil {
					t.Fatalf("mirror decoder panicked on raw input: %v", r)
				}
			}()
			_, _ = decodeTestFramePacket(data)
		}()

		frame := frameFromFuzzSeed(data)
		packet, err := encodeFramePacket(frame)
		if err != nil {
			t.Fatalf("encodeFramePacket: %v", err)
		}
		decoded, err := decodeTestFramePacket(packet)
		if err != nil {
			t.Fatalf("round-trip decode failed: %v", err)
		}
		if decoded.trailing != 0 {
			t.Fatalf("trailing bytes = %d, want 0", decoded.trailing)
		}
		if len(decoded.batches) == 0 {
			t.Fatal("expected at least one encoded batch")
		}
		for i, b := range decoded.batches {
			if len(b.commands) == 0 {
				t.Fatalf("batch %d has zero commands", i)
			}
		}
	})
}
