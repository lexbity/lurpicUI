package vulkan

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/hashutil"
	"codeburg.org/lexbit/lurpicui/render"
)

const (
	framePacketMagic   = "LPVF"
	framePacketVersion = uint32(1)
)

const (
	packetCmdFillRect uint8 = iota
	packetCmdStrokeRect
	packetCmdFillPath
	packetCmdStrokePath
	packetCmdDrawPolyline
	packetCmdDrawPoints
	packetCmdDrawSelectionRects
	packetCmdPushTransform
	packetCmdPopTransform
	packetCmdPushClipRect
	packetCmdPopClip
	packetCmdPushOpacity
	packetCmdPopOpacity
	packetCmdDrawGlyphRun
	packetCmdDrawImage
	packetCmdDrawTexture
)

type packetWriter struct {
	buf bytes.Buffer
}

func encodeFramePacket(frame *render.Frame) ([]byte, error) {
	return encodeFramePacketWithAssets(frame, nil)
}

type imageAssetUploader interface {
	ensureImage(img *image.RGBA) (uint64, error)
}

func encodeFramePacketWithAssets(frame *render.Frame, assets imageAssetUploader) ([]byte, error) {
	if frame == nil {
		return nil, nil
	}

	batches := frame.RenderBatchs
	if len(batches) == 0 && len(frame.Layers) > 0 {
		batches = flattenLayerBatches(frame.Layers)
	}

	encoded := make([]encodedBatch, 0, len(batches))
	for _, batch := range batches {
		payload, commands, ok, err := encodeBatch(batch, assets)
		if err != nil {
			return nil, err
		}
		if !ok {
			continue
		}
		encoded = append(encoded, encodedBatch{
			batch:    batch,
			payload:  payload,
			commands: commands,
		})
	}

	var w packetWriter
	w.writeString(framePacketMagic)
	w.writeU32(framePacketVersion)
	w.writeU32(uint32(len(encoded)))
	for _, entry := range encoded {
		w.writeU64(uint64(entry.batch.ID))
		w.writeRect(entry.batch.Bounds)
		w.writeF32(entry.batch.Opacity)
		w.writeU32(uint32(entry.commands))
		w.writeBytes(entry.payload)
	}
	return w.buf.Bytes(), nil
}

func flattenLayerBatches(layers []render.LayeredBatch) []render.RenderBatch {
	if len(layers) == 0 {
		return nil
	}
	var total int
	for _, layer := range layers {
		total += len(layer.Batches)
	}
	out := make([]render.RenderBatch, 0, total)
	for _, layer := range layers {
		out = append(out, layer.Batches...)
	}
	return out
}

type encodedBatch struct {
	batch    render.RenderBatch
	payload  []byte
	commands int
}

func encodeBatch(batch render.RenderBatch, assets imageAssetUploader) ([]byte, int, bool, error) {
	if batch.Commands.Len() == 0 {
		return nil, 0, false, nil
	}

	var w packetWriter
	commands := 0
	for _, cmd := range batch.Commands.Commands {
		switch c := cmd.(type) {
		case gfx.FillRect:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdFillRect)
			w.writeRect(c.Rect)
			w.writeColor(c.Brush.Color)
		case gfx.StrokeRect:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdStrokeRect)
			w.writeRect(c.Rect)
			w.writeF32(c.Stroke.Width)
			w.writeColor(c.Brush.Color)
		case gfx.FillPath:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdFillPath)
			w.writePath(c.Path)
			w.writeColor(c.Brush.Color)
		case gfx.StrokePath:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdStrokePath)
			w.writePath(c.Path)
			w.writeF32(c.Stroke.Width)
			w.writeColor(c.Brush.Color)
		case gfx.DrawPolyline:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdDrawPolyline)
			if c.Closed {
				w.writeU8(1)
			} else {
				w.writeU8(0)
			}
			w.writeF32(c.Stroke.Width)
			w.writeU32(uint32(len(c.Points)))
			for _, p := range c.Points {
				w.writePoint(p)
			}
			w.writeColor(c.Brush.Color)
		case gfx.DrawPoints:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdDrawPoints)
			w.writeF32(c.Radius)
			w.writeU32(uint32(len(c.Points)))
			for _, p := range c.Points {
				w.writePoint(p)
			}
			w.writeColor(c.Brush.Color)
		case gfx.DrawSelectionRects:
			if !isSolidBrush(c.Brush) {
				continue
			}
			commands++
			w.writeU8(packetCmdDrawSelectionRects)
			w.writeU32(uint32(len(c.Rects)))
			for _, rr := range c.Rects {
				w.writeRect(rr)
			}
			w.writeColor(c.Brush.Color)
		case gfx.PushTransform:
			commands++
			w.writeU8(packetCmdPushTransform)
			w.writeTransform(c.Matrix)
		case gfx.PopTransform:
			commands++
			w.writeU8(packetCmdPopTransform)
		case gfx.PushClipRect:
			commands++
			w.writeU8(packetCmdPushClipRect)
			w.writeRect(c.Rect)
		case gfx.PopClip:
			commands++
			w.writeU8(packetCmdPopClip)
		case gfx.PushOpacity:
			commands++
			w.writeU8(packetCmdPushOpacity)
			w.writeF32(c.Alpha)
		case gfx.PopOpacity:
			commands++
			w.writeU8(packetCmdPopOpacity)
		case gfx.DrawGlyphRun:
			if !isSolidBrush(c.Brush) {
				continue
			}
			if err := uploadGlyphRun(c.Run); err != nil {
				return nil, 0, false, err
			}
			commands++
			w.writeU8(packetCmdDrawGlyphRun)
			w.writeU64(c.Run.Face.CacheKey())
			size := c.Run.Size
			if size <= 0 {
				size = c.Run.Style.Size
			}
			if size <= 0 {
				size = 14
			}
			w.writeU32(math.Float32bits(size))
			w.writePoint(c.Origin)
			w.writeColor(c.Brush.Color)
			w.writeU32(uint32(len(c.Run.Glyphs)))
			for _, glyph := range c.Run.Glyphs {
				w.writeU32(glyph.GlyphID)
				w.writeF32(glyph.X)
				w.writeF32(glyph.Y)
			}
		case gfx.DrawImage:
			if c.Image == nil {
				continue
			}
			if assets == nil {
				return nil, 0, false, fmt.Errorf("vulkan: image asset cache unavailable")
			}
			commands++
			handle, err := assets.ensureImage(c.Image)
			if err != nil {
				return nil, 0, false, err
			}
			w.writeU8(packetCmdDrawImage)
			w.writeU64(handle)
			w.writeRect(c.DestRect)
			w.writeRect(c.SrcRect)
			w.writeU8(uint8(c.Sampling))
			w.writeF32(c.Opacity)
		case gfx.DrawTexture:
			commands++
			w.writeU8(packetCmdDrawTexture)
			w.writeU64(c.TextureID)
			w.writeRect(c.DestRect)
			w.writeRect(c.SrcRect)
			w.writeU8(uint8(c.Sampling))
			w.writeF32(c.Opacity)
		case gfx.BeginRenderBatch, gfx.EndRenderBatch:
		default:
			return nil, 0, false, fmt.Errorf("vulkan: unsupported command type %T", cmd)
		}
	}

	if commands == 0 {
		return nil, 0, false, nil
	}
	return w.buf.Bytes(), commands, true, nil
}

func isSolidBrush(brush gfx.Brush) bool {
	return brush.Kind == gfx.BrushSolid
}

func hashImage(img *image.RGBA) uint64 {
	if img == nil {
		return 0
	}
	b := hashutil.NewCacheKeyBuilder()
	b.WriteUint32(uint32(img.Rect.Min.X))
	b.WriteUint32(uint32(img.Rect.Min.Y))
	b.WriteUint32(uint32(img.Rect.Max.X))
	b.WriteUint32(uint32(img.Rect.Max.Y))
	b.WriteUint32(uint32(img.Stride))
	b.WriteBytes(img.Pix)
	return b.Sum()
}

func (w *packetWriter) writeBytes(b []byte) {
	_, _ = w.buf.Write(b)
}

func (w *packetWriter) writeString(s string) {
	_, _ = w.buf.WriteString(s)
}

func (w *packetWriter) writeU8(v uint8) {
	_ = w.buf.WriteByte(v)
}

func (w *packetWriter) writeU32(v uint32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], v)
	_, _ = w.buf.Write(tmp[:])
}

func (w *packetWriter) writeU64(v uint64) {
	var tmp [8]byte
	binary.LittleEndian.PutUint64(tmp[:], v)
	_, _ = w.buf.Write(tmp[:])
}

func (w *packetWriter) writeF32(v float32) {
	var tmp [4]byte
	binary.LittleEndian.PutUint32(tmp[:], math.Float32bits(v))
	_, _ = w.buf.Write(tmp[:])
}

func (w *packetWriter) writePoint(p gfx.Point) {
	w.writeF32(p.X)
	w.writeF32(p.Y)
}

func (w *packetWriter) writeRect(r gfx.Rect) {
	w.writePoint(r.Min)
	w.writePoint(r.Max)
}

func (w *packetWriter) writeColor(c gfx.Color) {
	w.writeF32(c.R)
	w.writeF32(c.G)
	w.writeF32(c.B)
	w.writeF32(c.A)
}

func (w *packetWriter) writeTransform(t gfx.Transform) {
	w.writeF32(t.A)
	w.writeF32(t.B)
	w.writeF32(t.C)
	w.writeF32(t.D)
	w.writeF32(t.TX)
	w.writeF32(t.TY)
}

func (w *packetWriter) writePath(path gfx.Path) {
	w.writeU32(uint32(len(path.Segments)))
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			w.writeU8(0)
			w.writePoint(seg.Pts[0])
		case gfx.PathLineTo:
			w.writeU8(1)
			w.writePoint(seg.Pts[0])
		case gfx.PathQuadTo:
			w.writeU8(2)
			w.writePoint(seg.Pts[0])
			w.writePoint(seg.Pts[1])
		case gfx.PathCubicTo:
			w.writeU8(3)
			w.writePoint(seg.Pts[0])
			w.writePoint(seg.Pts[1])
			w.writePoint(seg.Pts[2])
		case gfx.PathClose:
			w.writeU8(4)
		}
	}
}
