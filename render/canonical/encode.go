// Package canonical owns the complete, typed encoding of a render packet.
//
// A render packet is the optimization oracle: two runs are equivalent only
// when they produced the same visible commands, layer order, clip stacks,
// opacity, assets and glyph content. This package therefore encodes every
// production gfx command explicitly and fails on anything it does not know.
// It never hashes an "UnknownCommand" sentinel, never uses reflection to
// discover fields, and never rounds distinct finite floats together.
//
// SHA-256 is the exported determinism digest. Fast non-cryptographic hashes
// (render.RenderBatch.CommandHash) may remain local cache indexes, but they are
// not equality or correctness evidence.
package canonical

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/render"
	"codeburg.org/lexbit/lurpicui/text"
)

// Encoder accumulates one canonical digest.
type Encoder struct {
	h   hash.Hash
	err error
}

// NewEncoder returns an empty canonical encoder.
func NewEncoder() *Encoder {
	return &Encoder{h: sha256.New()}
}

// Sum returns the lowercase hexadecimal SHA-256 digest of everything written.
func (e *Encoder) Sum() string {
	return hex.EncodeToString(e.h.Sum(nil))
}

// Digest encodes one complete frame and returns its canonical digest.
func Digest(frame *render.Frame) (string, error) {
	e := NewEncoder()
	if err := e.WriteFrame(frame); err != nil {
		return "", err
	}
	if e.err != nil {
		return "", e.err
	}
	return e.Sum(), nil
}

// DigestCommands encodes one command list. It is the narrow oracle used by
// tests that do not build a whole packet.
func DigestCommands(list gfx.CommandList) (string, error) {
	e := NewEncoder()
	if err := e.writeCommands(list); err != nil {
		return "", err
	}
	if e.err != nil {
		return "", e.err
	}
	return e.Sum(), nil
}

// CommandDigests returns one complete canonical digest per command. It is
// intended for ordered diff consumers that need both exact equality and the
// corresponding command values for conservative damage bounds.
func CommandDigests(list gfx.CommandList) ([]string, error) {
	e := NewEncoder()
	digests := make([]string, len(list.Commands))
	for i, command := range list.Commands {
		if command == nil {
			return nil, fmt.Errorf("canonical: nil command at index %d", i)
		}
		e.h.Reset()
		e.err = nil
		e.writeTag("command")
		if err := e.writeCommand(command); err != nil {
			return nil, fmt.Errorf("canonical: command %d: %w", i, err)
		}
		if e.err != nil {
			return nil, fmt.Errorf("canonical: command %d: %w", i, e.err)
		}
		digests[i] = e.Sum()
	}
	return digests, nil
}

// DigestCommand encodes one complete command for bounded render-diff metadata.
func DigestCommand(command gfx.Command) (string, error) {
	e := NewEncoder()
	e.writeTag("command")
	if err := e.writeCommand(command); err != nil {
		return "", err
	}
	if e.err != nil {
		return "", e.err
	}
	return e.Sum(), nil
}

// WriteFrame encodes a frame's complete packet: layer order and clips, each
// batch's bounds/opacity/order, every command, and the damaged-region record.
// Process identities (RenderBatchID) and local cache indexes (CommandHash) are
// deliberately excluded: they differ between equal runs.
func (e *Encoder) WriteFrame(frame *render.Frame) error {
	if frame == nil {
		e.writeTag("frame.nil")
		return nil
	}
	e.writeTag("frame")
	e.writeTag("layers")
	if err := e.writeCount(frame.LayerCount()); err != nil {
		return err
	}
	for i := 0; i < frame.LayerCount(); i++ {
		layer, ok := frame.LayerAt(i)
		if !ok {
			return errors.New("canonical: released frame layer")
		}
		e.writeTag("layer")
		e.writeInt64(int64(layer.RenderOrder))
		e.writeRect(layer.ClipRect)
		if err := e.writeCount(layer.FirstBatch); err != nil {
			return err
		}
		if err := e.writeCount(layer.BatchCount); err != nil {
			return err
		}
	}
	e.writeTag("batches")
	if err := e.writeCount(frame.BatchCount()); err != nil {
		return err
	}
	for i := 0; i < frame.BatchCount(); i++ {
		if err := e.writeFrameBatch(frame, i); err != nil {
			return err
		}
	}
	e.writeTag("dirty-regions")
	if err := e.writeCount(frame.DirtyRegionCount()); err != nil {
		return err
	}
	for i := 0; i < frame.DirtyRegionCount(); i++ {
		rect, ok := frame.DirtyRegionAt(i)
		if !ok {
			return errors.New("canonical: released frame damage")
		}
		e.writeRect(rect)
	}
	return e.err
}

func (e *Encoder) writeFrameBatch(frame *render.Frame, index int) error {
	batch, ok := frame.BatchAt(index)
	if !ok {
		return errors.New("canonical: invalid frame batch range")
	}
	e.writeTag("batch")
	e.writeRect(batch.Bounds)
	e.writeFloat32(batch.Opacity)
	count, ok := frame.BatchCommandCount(index)
	if !ok {
		return errors.New("canonical: invalid command range")
	}
	if err := e.writeCount(count); err != nil {
		return err
	}
	commandIndex := 0
	var commandErr error
	if !frame.VisitBatchCommands(index, func(command gfx.Command) bool {
		if command == nil {
			commandErr = fmt.Errorf("canonical: nil command at index %d", commandIndex)
			return false
		}
		if err := e.writeCommand(command); err != nil {
			commandErr = fmt.Errorf("canonical: command %d: %w", commandIndex, err)
			return false
		}
		commandIndex++
		return true
	}) {
		return errors.New("canonical: released frame command page")
	}
	if commandErr != nil {
		return commandErr
	}
	if e.err != nil {
		return e.err
	}
	return nil
}

func (e *Encoder) writeCount(value int) error {
	if value < 0 {
		return errors.New("canonical: negative frame count or range")
	}
	e.writeUint64(uint64(value))
	return e.err
}

func (e *Encoder) writeCommands(list gfx.CommandList) error {
	e.writeUint64(uint64(len(list.Commands)))
	for i, command := range list.Commands {
		if command == nil {
			return fmt.Errorf("canonical: nil command at index %d", i)
		}
		if err := e.writeCommand(command); err != nil {
			return fmt.Errorf("canonical: command %d: %w", i, err)
		}
	}
	return e.err
}

func (e *Encoder) writeCommand(command gfx.Command) error {
	switch c := command.(type) {
	case gfx.PushTransform:
		e.writeTag("PushTransform")
		e.writeTransform(c.Matrix)
	case gfx.PopTransform:
		e.writeTag("PopTransform")
	case gfx.PushClipRect:
		e.writeTag("PushClipRect")
		e.writeRect(c.Rect)
	case gfx.PopClip:
		e.writeTag("PopClip")
	case gfx.PushOpacity:
		e.writeTag("PushOpacity")
		e.writeFloat32(c.Alpha)
	case gfx.PopOpacity:
		e.writeTag("PopOpacity")
	case gfx.FillRect:
		e.writeTag("FillRect")
		e.writeRect(c.Rect)
		e.writeBrush(c.Brush)
	case gfx.StrokeRect:
		e.writeTag("StrokeRect")
		e.writeRect(c.Rect)
		e.writeStroke(c.Stroke)
		e.writeBrush(c.Brush)
	case gfx.FillPath:
		e.writeTag("FillPath")
		e.writePath(c.Path)
		e.writeBrush(c.Brush)
	case gfx.StrokePath:
		e.writeTag("StrokePath")
		e.writePath(c.Path)
		e.writeStroke(c.Stroke)
		e.writeBrush(c.Brush)
	case gfx.DrawPolyline:
		e.writeTag("DrawPolyline")
		e.writePoints(c.Points)
		e.writeStroke(c.Stroke)
		e.writeBrush(c.Brush)
		e.writeBool(c.Closed)
	case gfx.DrawPoints:
		e.writeTag("DrawPoints")
		e.writePoints(c.Points)
		e.writeFloat32(c.Radius)
		e.writeBrush(c.Brush)
	case gfx.DrawGlyphRun:
		e.writeTag("DrawGlyphRun")
		e.writeGlyphRun(c.Run)
		e.writePoint(c.Origin)
		e.writeBrush(c.Brush)
	case gfx.DrawSelectionRects:
		e.writeTag("DrawSelectionRects")
		e.writeRects(c.Rects)
		e.writeBrush(c.Brush)
	case gfx.DrawImage:
		e.writeTag("DrawImage")
		e.writeImage(c.Image)
		e.writeRect(c.DestRect)
		e.writeRect(c.SrcRect)
		e.writeUint8(uint8(c.Sampling))
		e.writeFloat32(c.Opacity)
	case gfx.DrawTexture:
		e.writeTag("DrawTexture")
		e.writeUint64(c.TextureID)
		e.writeRect(c.DestRect)
		e.writeRect(c.SrcRect)
		e.writeUint8(uint8(c.Sampling))
		e.writeFloat32(c.Opacity)
	case gfx.DrawBlurredShadow:
		e.writeTag("DrawBlurredShadow")
		e.writePath(c.Path)
		e.writeColor(c.Color)
		e.writeFloat32(c.BlurRadius)
		e.writePoint(c.Offset)
		e.writeBool(c.Inner)
	case gfx.BeginRenderBatch:
		e.writeTag("BeginRenderBatch")
		e.writeRect(c.Bounds)
		e.writeUint64(uint64(c.CacheID))
	case gfx.EndRenderBatch:
		e.writeTag("EndRenderBatch")
	default:
		return fmt.Errorf("unsupported render command %T", command)
	}
	return nil
}

func (e *Encoder) writeGlyphRun(run text.GlyphRun) {
	e.writeTag("glyph-run")
	e.writeUint64(uint64(len(run.Glyphs)))
	for _, glyph := range run.Glyphs {
		e.writeUint32(glyph.GlyphID)
		e.writeFloat32(glyph.X)
		e.writeFloat32(glyph.Y)
		e.writeFloat32(glyph.Advance)
		e.writeInt64(int64(glyph.RuneIndex))
	}
	// FontFace is an opaque handle; CacheKey is the content-derived font
	// identity (font bytes + face index), so equal fonts compare equal across
	// independent registries.
	e.writeUint64(run.Face.CacheKey())
	e.writeFloat32(run.Size)
	e.writeTextStyle(run.Style)
	e.writeTextRect(run.Bounds)
	e.writeFloat32(run.Advance)
	e.writeString(run.Text)
	e.writeInt64(int64(run.Direction))
	e.writeInt64(int64(run.Level))
	e.writeUint64(uint64(len(run.GraphemeAdvances)))
	for _, advance := range run.GraphemeAdvances {
		e.writeFloat32(advance)
	}
	e.writeString(string(run.Language))
	e.writeUint32(uint32(run.Script))
	e.writeLineMetrics(run.Metrics)
	e.writeInt64(int64(run.LogicalIndex))
}

func (e *Encoder) writeTextStyle(style text.TextStyle) {
	e.writeString(style.Family)
	e.writeFloat32(style.Size)
	e.writeUint16(uint16(style.Weight))
	e.writeUint8(uint8(style.Style))
	e.writeFloat32(style.LineHeight)
	e.writeFloat32(style.LetterSpacing)
	e.writeInt64(int64(style.TabWidth))
}

func (e *Encoder) writeLineMetrics(metrics text.LineMetrics) {
	e.writeFloat32(metrics.Ascent)
	e.writeFloat32(metrics.Descent)
	e.writeFloat32(metrics.Leading)
	e.writeFloat32(metrics.LineHeight)
}

func (e *Encoder) writeTextRect(rect text.Rect) {
	e.writeFloat32(rect.Min.X)
	e.writeFloat32(rect.Min.Y)
	e.writeFloat32(rect.Max.X)
	e.writeFloat32(rect.Max.Y)
}

func (e *Encoder) writeImage(img gfx.ImageHandle) {
	if img.IsZero() {
		e.writeTag("image.nil")
		return
	}
	e.writeTag("image")
	e.writeUint64(img.Generation())
	e.writeInt64(int64(img.Width()))
	e.writeInt64(int64(img.Height()))
	e.writeInt64(int64(img.Stride()))
	digest := img.ContentDigest()
	e.writeBytes(digest[:])
}

func (e *Encoder) writePath(path gfx.Path) {
	e.writeTag("path")
	e.writeUint64(uint64(len(path.Segments)))
	for _, segment := range path.Segments {
		e.writeUint8(uint8(segment.Verb))
		// Encode the full fixed point array. SegmentPointCount decides how
		// many points a renderer reads; unused points are still field content
		// and must not be silently dropped from equality.
		for _, point := range segment.Pts {
			e.writePoint(point)
		}
	}
}

func (e *Encoder) writeStroke(stroke gfx.StrokeStyle) {
	e.writeTag("stroke")
	e.writeFloat32(stroke.Width)
	e.writeUint8(uint8(stroke.Cap))
	e.writeUint8(uint8(stroke.Join))
	e.writeFloat32(stroke.MiterLimit)
	e.writeUint64(uint64(len(stroke.Dash)))
	for _, dash := range stroke.Dash {
		e.writeFloat32(dash)
	}
	e.writeFloat32(stroke.DashOffset)
}

func (e *Encoder) writeBrush(brush gfx.Brush) {
	e.writeTag("brush")
	e.writeUint8(uint8(brush.Kind))
	e.writeColor(brush.Color)
	e.writePoint(brush.GradientStart)
	e.writePoint(brush.GradientEnd)
	e.writeUint64(uint64(len(brush.GradientStops)))
	for _, stop := range brush.GradientStops {
		e.writeFloat32(stop.Offset)
		e.writeColor(stop.Color)
	}
}

func (e *Encoder) writeColor(color gfx.Color) {
	e.writeFloat32(color.R)
	e.writeFloat32(color.G)
	e.writeFloat32(color.B)
	e.writeFloat32(color.A)
}

func (e *Encoder) writeTransform(transform gfx.Transform) {
	e.writeFloat32(transform.A)
	e.writeFloat32(transform.B)
	e.writeFloat32(transform.C)
	e.writeFloat32(transform.D)
	e.writeFloat32(transform.TX)
	e.writeFloat32(transform.TY)
}

func (e *Encoder) writePoints(points []gfx.Point) {
	e.writeUint64(uint64(len(points)))
	for _, point := range points {
		e.writePoint(point)
	}
}

func (e *Encoder) writePoint(point gfx.Point) {
	e.writeFloat32(point.X)
	e.writeFloat32(point.Y)
}

func (e *Encoder) writeRect(rect gfx.Rect) {
	e.writePoint(rect.Min)
	e.writePoint(rect.Max)
}

func (e *Encoder) writeRects(rects []gfx.Rect) {
	e.writeUint64(uint64(len(rects)))
	for _, rect := range rects {
		e.writeRect(rect)
	}
}

func (e *Encoder) writeTag(tag string) {
	e.writeString(tag)
}

func (e *Encoder) writeString(value string) {
	e.writeBytes([]byte(value))
}

func (e *Encoder) writeBytes(value []byte) {
	e.writeUint64(uint64(len(value)))
	_, _ = e.h.Write(value) // hash.Hash never returns an error
}

func (e *Encoder) writeBool(value bool) {
	if value {
		e.writeUint8(1)
		return
	}
	e.writeUint8(0)
}

func (e *Encoder) writeUint8(value uint8) {
	var buf [1]byte
	buf[0] = value
	_, _ = e.h.Write(buf[:])
}

func (e *Encoder) writeUint16(value uint16) {
	var buf [2]byte
	binary.BigEndian.PutUint16(buf[:], value)
	_, _ = e.h.Write(buf[:])
}

func (e *Encoder) writeUint32(value uint32) {
	var buf [4]byte
	binary.BigEndian.PutUint32(buf[:], value)
	_, _ = e.h.Write(buf[:])
}

func (e *Encoder) writeUint64(value uint64) {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], value)
	_, _ = e.h.Write(buf[:])
}

func (e *Encoder) writeInt64(value int64) {
	e.writeUint64(uint64(value)) //nolint:gosec // exact two's-complement round trip
}

// writeFloat32 rejects non-finite values and normalizes negative zero. Two
// distinct finite values keep distinct bit patterns; no tolerance is applied.
func (e *Encoder) writeFloat32(value float32) {
	if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
		if e.err == nil {
			e.err = nonFiniteError{value: value}
		}
		return
	}
	if value == 0 {
		value = 0 // normalize -0 to +0
	}
	e.writeUint32(math.Float32bits(value))
}

type nonFiniteError struct{ value float32 }

func (err nonFiniteError) Error() string {
	return fmt.Sprintf("canonical: non-finite float32 %v", err.value)
}

func (nonFiniteError) Unwrap() error { return ErrNonFinite }

// ErrNonFinite identifies a non-finite render value. Encoders are called on
// sealed packets; such values are a contract failure, not comparable input.
var ErrNonFinite = errors.New("canonical: non-finite render value")

// WriteFrame is a package-level convenience wrapper matching Digest.
func WriteFrame(frame *render.Frame) (string, error) {
	return Digest(frame)
}
