package vulkan

import (
	"encoding/binary"
	"image"
	"image/color"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
)

func TestEncodeFramePacket_solidRectBatch(t *testing.T) {
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          7,
				Bounds:      gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity:     0.75,
				CommandHash: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.FillRect{Rect: gfx.RectFromXYWH(0, 0, 10, 10), Brush: gfx.SolidBrush(gfx.Color{R: 1, G: 0.5, B: 0.25, A: 1})},
				}},
			},
		},
	}

	packet, err := encodeFramePacket(frame)
	if err != nil {
		t.Fatalf("encodeFramePacket: %v", err)
	}
	if len(packet) == 0 {
		t.Fatal("expected a non-empty packet")
	}
	if got := string(packet[:4]); got != framePacketMagic {
		t.Fatalf("unexpected magic %q", got)
	}
	if got := binary.LittleEndian.Uint32(packet[4:8]); got != framePacketVersion {
		t.Fatalf("unexpected version %d", got)
	}
	if got := binary.LittleEndian.Uint32(packet[8:12]); got != 1 {
		t.Fatalf("unexpected batch count %d", got)
	}
}

func TestEncodeFramePacket_drawImageBatch(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 1, 1))
	img.SetRGBA(0, 0, color.RGBA{R: 255, A: 255})
	uploader := &stubImageUploader{handle: 99}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:      7,
				Bounds:  gfx.RectFromXYWH(0, 0, 10, 10),
				Opacity: 1,
				Commands: gfx.CommandList{Commands: []gfx.Command{
					gfx.DrawImage{
						Image:    img,
						DestRect: gfx.RectFromXYWH(1, 2, 3, 4),
						SrcRect:  gfx.RectFromXYWH(0, 0, 1, 1),
						Sampling: gfx.SamplingBilinear,
						Opacity:  0.5,
					},
				}},
			},
		},
	}

	packet, err := encodeFramePacketWithAssets(frame, uploader)
	if err != nil {
		t.Fatalf("encodeFramePacketWithAssets: %v", err)
	}
	if uploader.calls != 1 {
		t.Fatalf("expected one image upload, got %d", uploader.calls)
	}
	if got := packet[44]; got != packetCmdDrawImage {
		t.Fatalf("unexpected opcode %d", got)
	}
	if got := binary.LittleEndian.Uint64(packet[45:53]); got != uploader.handle {
		t.Fatalf("unexpected image handle %d", got)
	}
}

type stubImageUploader struct {
	handle uint64
	calls  int
}

func (s *stubImageUploader) ensureImage(img *image.RGBA) (uint64, error) {
	s.calls++
	return s.handle, nil
}
