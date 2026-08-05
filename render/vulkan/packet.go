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
	framePacketVersion = uint32(2)
)

// Packet v2 opcodes. Mirrored byte-for-byte by the Rust decoder in
// crates/lurpic_render/src/frame.rs.
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
	packetCmdDrawBlurredShadow
	packetCmdBeginRenderBatch
	packetCmdEndRenderBatch
)

// Brush kinds (wire values).
const (
	packetBrushSolid uint8 = iota
	packetBrushLinearGradient
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

// batchWithClip pairs a batch with the layer clip rect that governs it, if any.
type batchWithClip struct {
	batch render.RenderBatch
	clip  gfx.Rect
}

func collectBatches(frame *render.Frame) []batchWithClip {
	var out []batchWithClip
	if len(frame.Layers) > 0 {
		for _, layer := range frame.Layers {
			for _, b := range layer.Batches {
				out = append(out, batchWithClip{batch: b, clip: layer.ClipRect})
			}
		}
		return out
	}
	for _, b := range frame.RenderBatchs {
		out = append(out, batchWithClip{batch: b})
	}
	return out
}

func encodeFramePacketWithAssets(frame *render.Frame, assets imageAssetUploader) ([]byte, error) {
	if frame == nil {
		return nil, nil
	}

	// Encode all batches first so the batch-count field reflects exactly the
	// number of encoded batches (batches with no decodable commands are
	// dropped, e.g. an empty PushTransform/PopTransform wrapper).
	var encoded []encodedBatch
	for _, item := range collectBatches(frame) {
		e, ok, err := encodeBatch(item.batch, item.clip, assets)
		if err != nil {
			return nil, err
		}
		if ok {
			encoded = append(encoded, e)
		}
	}

	var w packetWriter
	w.writeString(framePacketMagic)
	w.writeU32(framePacketVersion)
	// Surface dimensions are surfaced by the readback/present path explicitly;
	// the GPU pipeline (Slice 3) threads the real swapchain size through here.
	w.writeU32(0)                    // surface_w
	w.writeU32(0)                    // surface_h
	w.writeF32(1)                    // device_pixel_ratio
	w.writeU32(uint32(len(encoded))) //nolint:gosec // integer overflow conversion
	for _, entry := range encoded {
		w.writeU64(uint64(entry.batch.ID))
		w.writeRect(entry.batch.Bounds)
		w.writeF32(entry.batch.Opacity)
		w.writeTransform(entry.transform)
		w.writeRect(entry.clip)
		w.writeU32(uint32(entry.commands)) //nolint:gosec // integer overflow conversion
		w.writeBytes(entry.payload)
	}
	return w.buf.Bytes(), nil
}

type encodedBatch struct {
	batch     render.RenderBatch
	transform gfx.Transform
	clip      gfx.Rect
	payload   []byte
	commands  int
}

// extractBatchTransform lifts a leading PushTransform / trailing PopTransform
// wrapper (injected by runtime/core.go around every non-identity batch
// transform) into the batch header so it travels as packet metadata. The
// wrapper is only extracted when it provably brackets the whole batch: the
// transform depth must return to zero exactly at the final command.
func extractBatchTransform(cmds []gfx.Command) (gfx.Transform, []gfx.Command, bool) {
	if len(cmds) < 2 {
		return gfx.Identity(), cmds, false
	}
	pt, ok := cmds[0].(gfx.PushTransform)
	if !ok {
		return gfx.Identity(), cmds, false
	}
	if _, ok := cmds[len(cmds)-1].(gfx.PopTransform); !ok {
		return gfx.Identity(), cmds, false
	}
	depth := 0
	for i, c := range cmds {
		switch c.(type) {
		case gfx.PushTransform:
			depth++
		case gfx.PopTransform:
			depth--
			if depth < 0 || (depth == 0 && i != len(cmds)-1) {
				return gfx.Identity(), cmds, false
			}
		}
	}
	if depth != 0 {
		return gfx.Identity(), cmds, false
	}
	return pt.Matrix, cmds[1 : len(cmds)-1], true
}

func encodeBatch(batch render.RenderBatch, clip gfx.Rect, assets imageAssetUploader) (encodedBatch, bool, error) {
	if batch.Commands.Len() == 0 {
		return encodedBatch{}, false, nil
	}

	transform, cmds, _ := extractBatchTransform(batch.Commands.Commands)
	if len(cmds) == 0 {
		return encodedBatch{}, false, nil
	}

	var w packetWriter
	commands := 0
	for _, cmd := range cmds {
		switch c := cmd.(type) {
		case gfx.FillRect:
			commands++
			w.writeU8(packetCmdFillRect)
			w.writeRect(c.Rect)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.StrokeRect:
			commands++
			w.writeU8(packetCmdStrokeRect)
			w.writeRect(c.Rect)
			w.writeStrokeStyle(c.Stroke)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.FillPath:
			commands++
			w.writeU8(packetCmdFillPath)
			w.writePath(c.Path)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.StrokePath:
			commands++
			w.writeU8(packetCmdStrokePath)
			w.writePath(c.Path)
			w.writeStrokeStyle(c.Stroke)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.DrawPolyline:
			commands++
			w.writeU8(packetCmdDrawPolyline)
			w.writeU32(uint32(len(c.Points))) //nolint:gosec // integer overflow conversion
			for _, p := range c.Points {
				w.writePoint(p)
			}
			w.writeStrokeStyle(c.Stroke)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
			if c.Closed {
				w.writeU8(1)
			} else {
				w.writeU8(0)
			}
		case gfx.DrawPoints:
			commands++
			w.writeU8(packetCmdDrawPoints)
			w.writeU32(uint32(len(c.Points))) //nolint:gosec // integer overflow conversion
			for _, p := range c.Points {
				w.writePoint(p)
			}
			w.writeF32(c.Radius)
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.DrawSelectionRects:
			commands++
			w.writeU8(packetCmdDrawSelectionRects)
			w.writeU32(uint32(len(c.Rects))) //nolint:gosec // integer overflow conversion
			for _, rr := range c.Rects {
				w.writeRect(rr)
			}
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
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
			if err := uploadGlyphRun(c.Run); err != nil {
				return encodedBatch{}, false, err
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
			w.writeU32(uint32(len(c.Run.Glyphs))) //nolint:gosec // integer overflow conversion
			for _, glyph := range c.Run.Glyphs {
				w.writeU32(glyph.GlyphID)
				w.writeF32(glyph.X)
				w.writeF32(glyph.Y)
			}
			if err := w.writeBrush(c.Brush); err != nil {
				return encodedBatch{}, false, err
			}
		case gfx.DrawImage:
			if c.Image == nil {
				continue
			}
			if assets == nil {
				return encodedBatch{}, false, fmt.Errorf("vulkan: image asset cache unavailable")
			}
			handle, err := assets.ensureImage(c.Image)
			if err != nil {
				return encodedBatch{}, false, err
			}
			commands++
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
		case gfx.DrawBlurredShadow:
			commands++
			w.writeU8(packetCmdDrawBlurredShadow)
			w.writePath(c.Path)
			w.writeColor8(c.Color)
			w.writeF32(c.BlurRadius)
			w.writePoint(c.Offset)
			if c.Inner {
				w.writeU8(1)
			} else {
				w.writeU8(0)
			}
		case gfx.BeginRenderBatch:
			commands++
			w.writeU8(packetCmdBeginRenderBatch)
			w.writeRect(c.Bounds)
			w.writeU64(uint64(c.CacheID))
		case gfx.EndRenderBatch:
			commands++
			w.writeU8(packetCmdEndRenderBatch)
		default:
			return encodedBatch{}, false, fmt.Errorf("vulkan: unsupported command type %T", cmd)
		}
	}

	if commands == 0 {
		return encodedBatch{}, false, nil
	}
	return encodedBatch{
		batch:     batch,
		transform: transform,
		clip:      clip,
		payload:   w.buf.Bytes(),
		commands:  commands,
	}, true, nil
}

func hashImage(img *image.RGBA) uint64 {
	if img == nil {
		return 0
	}
	b := hashutil.NewCacheKeyBuilder()
	b.WriteUint32(uint32(img.Rect.Min.X)) //nolint:gosec // integer overflow conversion
	b.WriteUint32(uint32(img.Rect.Min.Y)) //nolint:gosec // integer overflow conversion
	b.WriteUint32(uint32(img.Rect.Max.X)) //nolint:gosec // integer overflow conversion
	b.WriteUint32(uint32(img.Rect.Max.Y)) //nolint:gosec // integer overflow conversion
	b.WriteUint32(uint32(img.Stride))     //nolint:gosec // integer overflow conversion
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

// writeColor8 encodes a premultiplied float color as four 8-bit channels,
// matching the wire Brush color layout (4 x u8).
func (w *packetWriter) writeColor8(c gfx.Color) {
	w.writeU8(colorByte(c.R))
	w.writeU8(colorByte(c.G))
	w.writeU8(colorByte(c.B))
	w.writeU8(colorByte(c.A))
}

func (w *packetWriter) writeTransform(t gfx.Transform) {
	w.writeF32(t.A)
	w.writeF32(t.B)
	w.writeF32(t.C)
	w.writeF32(t.D)
	w.writeF32(t.TX)
	w.writeF32(t.TY)
}

// writeBrush encodes both brush kinds without loss. The gfx BrushKind surface
// is sealed to solid/linear-gradient; any other kind is a caller contract
// violation and is surfaced as an error rather than silently degraded.
func (w *packetWriter) writeBrush(brush gfx.Brush) error {
	switch brush.Kind {
	case gfx.BrushSolid:
		w.writeU8(packetBrushSolid)
		w.writeColor8(brush.Color)
		return nil
	case gfx.BrushLinearGradient:
		w.writeU8(packetBrushLinearGradient)
		w.writePoint(brush.GradientStart)
		w.writePoint(brush.GradientEnd)
		w.writeU32(uint32(len(brush.GradientStops))) //nolint:gosec // integer overflow conversion
		for _, stop := range brush.GradientStops {
			w.writeF32(stop.Offset)
			w.writeColor8(stop.Color)
		}
		return nil
	default:
		return fmt.Errorf("vulkan: unsupported brush kind %d", brush.Kind)
	}
}

func (w *packetWriter) writeStrokeStyle(s gfx.StrokeStyle) {
	w.writeF32(s.Width)
	w.writeU8(uint8(s.Cap))
	w.writeU8(uint8(s.Join))
	w.writeF32(s.MiterLimit)
	w.writeU32(uint32(len(s.Dash))) //nolint:gosec // integer overflow conversion
	for _, d := range s.Dash {
		w.writeF32(d)
	}
	w.writeF32(s.DashOffset)
}

// writePath encodes a path as two parallel arrays: the verb sequence followed
// by the concatenated point sequence (MoveTo/LineTo carry 1 point, QuadTo 2,
// CubicTo 3, Close 0).
func (w *packetWriter) writePath(path gfx.Path) {
	pointCount := 0
	for _, seg := range path.Segments {
		pointCount += gfx.SegmentPointCount(seg.Verb)
	}
	w.writeU32(uint32(len(path.Segments))) //nolint:gosec // integer overflow conversion
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo:
			w.writeU8(0)
		case gfx.PathLineTo:
			w.writeU8(1)
		case gfx.PathQuadTo:
			w.writeU8(2)
		case gfx.PathCubicTo:
			w.writeU8(3)
		case gfx.PathClose:
			w.writeU8(4)
		}
	}
	w.writeU32(uint32(pointCount))
	for _, seg := range path.Segments {
		switch seg.Verb {
		case gfx.PathMoveTo, gfx.PathLineTo:
			w.writePoint(seg.Pts[0])
		case gfx.PathQuadTo:
			w.writePoint(seg.Pts[0])
			w.writePoint(seg.Pts[1])
		case gfx.PathCubicTo:
			w.writePoint(seg.Pts[0])
			w.writePoint(seg.Pts[1])
			w.writePoint(seg.Pts[2])
		}
	}
}
func colorByte(v float32) uint8 {
	if v < 0 {
		v = 0
	}
	if v > 1 {
		v = 1
	}
	return uint8(math.Round(float64(v) * 255))
}
