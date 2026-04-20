package hashutil

import (
	"image"
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// CacheKeyBuilder builds a uint64 hash from a sequence of values.
type CacheKeyBuilder struct {
	h uint64
}

// NewCacheKeyBuilder returns an initialized builder.
func NewCacheKeyBuilder() CacheKeyBuilder {
	return CacheKeyBuilder{h: fnvOffset64}
}

const fnvOffset64 = 1469598103934665603
const fnvPrime64 = 1099511628211

func (b *CacheKeyBuilder) WriteUint8(v uint8) {
	b.mix(uint64(v))
}

func (b *CacheKeyBuilder) WriteUint32(v uint32) {
	b.mix(uint64(v))
}

func (b *CacheKeyBuilder) WriteUint64(v uint64) {
	b.mix(v)
}

func (b *CacheKeyBuilder) WriteFloat32(v float32) {
	b.mix(uint64(math.Float32bits(v)))
}

func (b *CacheKeyBuilder) WriteString(v string) {
	b.WriteBytes([]byte(v))
}

func (b *CacheKeyBuilder) WriteBytes(v []byte) {
	for _, c := range v {
		b.mix(uint64(c))
	}
}

func (b *CacheKeyBuilder) Sum() uint64 {
	return b.h
}

func (b *CacheKeyBuilder) mix(v uint64) {
	h := b.h
	for i := 0; i < 8; i++ {
		h ^= uint64(byte(v >> (8 * i)))
		h *= fnvPrime64
	}
	b.h = h
}

// HashCommandList computes a stable hash of a gfx.CommandList.
func HashCommandList(cl gfx.CommandList) uint64 {
	b := NewCacheKeyBuilder()
	b.WriteUint64(uint64(len(cl.Commands)))
	for _, cmd := range cl.Commands {
		hashCommand(&b, cmd)
	}
	return b.Sum()
}

func hashCommand(b *CacheKeyBuilder, cmd gfx.Command) {
	switch c := cmd.(type) {
	case gfx.PushTransform:
		b.WriteString("PushTransform")
		hashTransform(b, c.Matrix)
	case gfx.PopTransform:
		b.WriteString("PopTransform")
	case gfx.PushClipRect:
		b.WriteString("PushClipRect")
		hashRect(b, c.Rect)
	case gfx.PopClip:
		b.WriteString("PopClip")
	case gfx.PushOpacity:
		b.WriteString("PushOpacity")
		b.WriteFloat32(c.Alpha)
	case gfx.PopOpacity:
		b.WriteString("PopOpacity")
	case gfx.FillRect:
		b.WriteString("FillRect")
		hashRect(b, c.Rect)
		hashBrush(b, c.Brush)
	case gfx.StrokeRect:
		b.WriteString("StrokeRect")
		hashRect(b, c.Rect)
		hashStroke(b, c.Stroke)
		hashBrush(b, c.Brush)
	case gfx.FillPath:
		b.WriteString("FillPath")
		hashPath(b, c.Path)
		hashBrush(b, c.Brush)
	case gfx.StrokePath:
		b.WriteString("StrokePath")
		hashPath(b, c.Path)
		hashStroke(b, c.Stroke)
		hashBrush(b, c.Brush)
	case gfx.DrawPolyline:
		b.WriteString("DrawPolyline")
		hashPoints(b, c.Points)
		hashStroke(b, c.Stroke)
		hashBrush(b, c.Brush)
		if c.Closed {
			b.WriteUint8(1)
		} else {
			b.WriteUint8(0)
		}
	case gfx.DrawPoints:
		b.WriteString("DrawPoints")
		hashPoints(b, c.Points)
		b.WriteFloat32(c.Radius)
		hashBrush(b, c.Brush)
	case gfx.DrawGlyphRun:
		b.WriteString("DrawGlyphRun")
		hashPoint(b, c.Origin)
		hashBrush(b, c.Brush)
	case gfx.DrawSelectionRects:
		b.WriteString("DrawSelectionRects")
		hashRects(b, c.Rects)
		hashBrush(b, c.Brush)
	case gfx.DrawImage:
		b.WriteString("DrawImage")
		hashImage(b, c.Image)
		hashRect(b, c.DestRect)
		hashRect(b, c.SrcRect)
		b.WriteUint8(uint8(c.Sampling))
		b.WriteFloat32(c.Opacity)
	case gfx.BeginRenderBatch:
		b.WriteString("BeginRenderBatch")
		hashRect(b, c.Bounds)
		b.WriteUint64(uint64(c.CacheID))
	case gfx.EndRenderBatch:
		b.WriteString("EndRenderBatch")
	default:
		b.WriteString("UnknownCommand")
	}
}

func hashImage(b *CacheKeyBuilder, img *image.RGBA) {
	if img == nil {
		b.WriteUint8(0)
		return
	}
	b.WriteUint8(1)
	hashRect(b, gfx.Rect{Min: gfx.Point{X: float32(img.Rect.Min.X), Y: float32(img.Rect.Min.Y)}, Max: gfx.Point{X: float32(img.Rect.Max.X), Y: float32(img.Rect.Max.Y)}})
	b.WriteUint64(uint64(img.Stride))
	b.WriteBytes(img.Pix)
}

func hashPath(b *CacheKeyBuilder, path gfx.Path) {
	b.WriteUint64(uint64(len(path.Segments)))
	for _, seg := range path.Segments {
		b.WriteUint8(uint8(seg.Verb))
		for _, p := range seg.Pts {
			hashPoint(b, p)
		}
	}
}

func hashRects(b *CacheKeyBuilder, rects []gfx.Rect) {
	b.WriteUint64(uint64(len(rects)))
	for _, r := range rects {
		hashRect(b, r)
	}
}

func hashPoints(b *CacheKeyBuilder, pts []gfx.Point) {
	b.WriteUint64(uint64(len(pts)))
	for _, p := range pts {
		hashPoint(b, p)
	}
}

func hashPoint(b *CacheKeyBuilder, p gfx.Point) {
	b.WriteFloat32(p.X)
	b.WriteFloat32(p.Y)
}

func hashRect(b *CacheKeyBuilder, r gfx.Rect) {
	hashPoint(b, r.Min)
	hashPoint(b, r.Max)
}

func hashTransform(b *CacheKeyBuilder, t gfx.Transform) {
	b.WriteFloat32(t.A)
	b.WriteFloat32(t.B)
	b.WriteFloat32(t.C)
	b.WriteFloat32(t.D)
	b.WriteFloat32(t.TX)
	b.WriteFloat32(t.TY)
}

func hashBrush(b *CacheKeyBuilder, brush gfx.Brush) {
	b.WriteUint8(uint8(brush.Kind))
	hashColor(b, brush.Color)
	hashPoint(b, brush.GradientStart)
	hashPoint(b, brush.GradientEnd)
	b.WriteUint64(uint64(len(brush.GradientStops)))
	for _, stop := range brush.GradientStops {
		b.WriteFloat32(stop.Offset)
		hashColor(b, stop.Color)
	}
}

func hashStroke(b *CacheKeyBuilder, stroke gfx.StrokeStyle) {
	b.WriteFloat32(stroke.Width)
	b.WriteUint8(uint8(stroke.Cap))
	b.WriteUint8(uint8(stroke.Join))
	b.WriteFloat32(stroke.MiterLimit)
	b.WriteFloat32(stroke.DashOffset)
	b.WriteUint64(uint64(len(stroke.Dash)))
	for _, v := range stroke.Dash {
		b.WriteFloat32(v)
	}
}

func hashColor(b *CacheKeyBuilder, c gfx.Color) {
	b.WriteFloat32(c.R)
	b.WriteFloat32(c.G)
	b.WriteFloat32(c.B)
	b.WriteFloat32(c.A)
}
