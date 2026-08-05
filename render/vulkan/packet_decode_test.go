package vulkan

import (
	"encoding/binary"
	"fmt"
	"math"

	"codeburg.org/lexbit/lurpicui/gfx"
)

// testFramePacket is a Go-side mirror of the packet v2 wire format consumed by
// the Rust decoder (crates/lurpic_render/src/frame.rs). It exists so tests can
// round-trip Go-encoded packets without requiring the Rust library, catching
// schema drift on the encoding side. The Go<->Rust cross-check is the
// equivalence corpus, which decodes real packets with the Rust decoder.
type testFramePacket struct {
	version   uint32
	surfaceW  uint32
	surfaceH  uint32
	deviceDPR float32
	batches   []testBatchPacket
	trailing  int
}

type testBatchPacket struct {
	id        uint64
	bounds    gfx.Rect
	opacity   float32
	transform gfx.Transform
	clip      gfx.Rect
	commands  []testCommandPacket
}

type testCommandPacket struct {
	kind   uint8
	rect   gfx.Rect
	path   gfx.Path
	brush  gfx.Brush
	stroke gfx.StrokeStyle
	closed bool
	points []gfx.Point
	rects  []gfx.Rect
	radius float32
	matrix gfx.Transform
	alpha  float32
	// glyph run
	fontID   uint64
	sizeBits uint32
	origin   gfx.Point
	glyphs   []testGlyphPacket
	// image / texture / shadow
	handle     uint64
	textureID  uint64
	src        gfx.Rect
	sampling   uint8
	opacity    float32
	color      gfx.Color
	blurRadius float32
	offset     gfx.Point
	inner      bool
	cacheID    uint64
}

type testGlyphPacket struct {
	glyphID uint32
	x       float32
	y       float32
}

type testPacketReader struct {
	data []byte
	pos  int
}

func (r *testPacketReader) remaining() int { return len(r.data) - r.pos }

func (r *testPacketReader) readU8() (uint8, error) {
	if r.remaining() < 1 {
		return 0, fmt.Errorf("truncated at byte %d", r.pos)
	}
	v := r.data[r.pos]
	r.pos++
	return v, nil
}

func (r *testPacketReader) readU32() (uint32, error) {
	if r.remaining() < 4 {
		return 0, fmt.Errorf("truncated at byte %d", r.pos)
	}
	v := binary.LittleEndian.Uint32(r.data[r.pos:])
	r.pos += 4
	return v, nil
}

func (r *testPacketReader) readU64() (uint64, error) {
	if r.remaining() < 8 {
		return 0, fmt.Errorf("truncated at byte %d", r.pos)
	}
	v := binary.LittleEndian.Uint64(r.data[r.pos:])
	r.pos += 8
	return v, nil
}

func (r *testPacketReader) readF32() (float32, error) {
	if r.remaining() < 4 {
		return 0, fmt.Errorf("truncated at byte %d", r.pos)
	}
	v := math.Float32frombits(binary.LittleEndian.Uint32(r.data[r.pos:]))
	r.pos += 4
	return v, nil
}

func (r *testPacketReader) readPoint() (gfx.Point, error) {
	x, err := r.readF32()
	if err != nil {
		return gfx.Point{}, err
	}
	y, err := r.readF32()
	if err != nil {
		return gfx.Point{}, err
	}
	return gfx.Point{X: x, Y: y}, nil
}

func (r *testPacketReader) readRect() (gfx.Rect, error) {
	min, err := r.readPoint()
	if err != nil {
		return gfx.Rect{}, err
	}
	max, err := r.readPoint()
	if err != nil {
		return gfx.Rect{}, err
	}
	return gfx.Rect{Min: min, Max: max}, nil
}

func (r *testPacketReader) readColor8() (gfx.Color, error) {
	rv, err := r.readU8()
	if err != nil {
		return gfx.Color{}, err
	}
	g, err := r.readU8()
	if err != nil {
		return gfx.Color{}, err
	}
	b, err := r.readU8()
	if err != nil {
		return gfx.Color{}, err
	}
	a, err := r.readU8()
	if err != nil {
		return gfx.Color{}, err
	}
	return gfx.Color{
		R: float32(rv) / 255,
		G: float32(g) / 255,
		B: float32(b) / 255,
		A: float32(a) / 255,
	}, nil
}

func (r *testPacketReader) readTransform() (gfx.Transform, error) {
	var t gfx.Transform
	var err error
	if t.A, err = r.readF32(); err != nil {
		return t, err
	}
	if t.B, err = r.readF32(); err != nil {
		return t, err
	}
	if t.C, err = r.readF32(); err != nil {
		return t, err
	}
	if t.D, err = r.readF32(); err != nil {
		return t, err
	}
	if t.TX, err = r.readF32(); err != nil {
		return t, err
	}
	if t.TY, err = r.readF32(); err != nil {
		return t, err
	}
	return t, nil
}

func (r *testPacketReader) readPoints() ([]gfx.Point, error) {
	count, err := r.readU32()
	if err != nil {
		return nil, err
	}
	points := make([]gfx.Point, 0, count)
	for i := uint32(0); i < count; i++ {
		p, err := r.readPoint()
		if err != nil {
			return nil, err
		}
		points = append(points, p)
	}
	return points, nil
}

func (r *testPacketReader) readRects() ([]gfx.Rect, error) {
	count, err := r.readU32()
	if err != nil {
		return nil, err
	}
	rects := make([]gfx.Rect, 0, count)
	for i := uint32(0); i < count; i++ {
		rr, err := r.readRect()
		if err != nil {
			return nil, err
		}
		rects = append(rects, rr)
	}
	return rects, nil
}

func (r *testPacketReader) readPath() (gfx.Path, error) {
	verbCount, err := r.readU32()
	if err != nil {
		return gfx.Path{}, err
	}
	verbs := make([]gfx.PathVerb, 0, verbCount)
	for i := uint32(0); i < verbCount; i++ {
		v, err := r.readU8()
		if err != nil {
			return gfx.Path{}, err
		}
		switch v {
		case 0:
			verbs = append(verbs, gfx.PathMoveTo)
		case 1:
			verbs = append(verbs, gfx.PathLineTo)
		case 2:
			verbs = append(verbs, gfx.PathQuadTo)
		case 3:
			verbs = append(verbs, gfx.PathCubicTo)
		case 4:
			verbs = append(verbs, gfx.PathClose)
		default:
			return gfx.Path{}, fmt.Errorf("unknown path verb %d", v)
		}
	}
	pointCount, err := r.readU32()
	if err != nil {
		return gfx.Path{}, err
	}
	points := make([]gfx.Point, 0, pointCount)
	for i := uint32(0); i < pointCount; i++ {
		p, err := r.readPoint()
		if err != nil {
			return gfx.Path{}, err
		}
		points = append(points, p)
	}

	var segments []gfx.PathSegment
	pi := 0
	for _, verb := range verbs {
		seg := gfx.PathSegment{Verb: verb}
		switch verb {
		case gfx.PathMoveTo, gfx.PathLineTo:
			if pi >= len(points) {
				return gfx.Path{}, fmt.Errorf("path points exhausted for verb %d", verb)
			}
			seg.Pts[0] = points[pi]
			pi++
		case gfx.PathQuadTo:
			if pi+1 >= len(points) {
				return gfx.Path{}, fmt.Errorf("path points exhausted for verb %d", verb)
			}
			seg.Pts[0] = points[pi]
			seg.Pts[1] = points[pi+1]
			pi += 2
		case gfx.PathCubicTo:
			if pi+2 >= len(points) {
				return gfx.Path{}, fmt.Errorf("path points exhausted for verb %d", verb)
			}
			seg.Pts[0] = points[pi]
			seg.Pts[1] = points[pi+1]
			seg.Pts[2] = points[pi+2]
			pi += 3
		}
		segments = append(segments, seg)
	}
	return gfx.Path{Segments: segments}, nil
}

func (r *testPacketReader) readBrush() (gfx.Brush, error) {
	kind, err := r.readU8()
	if err != nil {
		return gfx.Brush{}, err
	}
	switch kind {
	case 0:
		color, err := r.readColor8()
		if err != nil {
			return gfx.Brush{}, err
		}
		return gfx.Brush{Kind: gfx.BrushSolid, Color: color}, nil
	case 1:
		start, err := r.readPoint()
		if err != nil {
			return gfx.Brush{}, err
		}
		end, err := r.readPoint()
		if err != nil {
			return gfx.Brush{}, err
		}
		stopCount, err := r.readU32()
		if err != nil {
			return gfx.Brush{}, err
		}
		stops := make([]gfx.GradientStop, 0, stopCount)
		for i := uint32(0); i < stopCount; i++ {
			offset, err := r.readF32()
			if err != nil {
				return gfx.Brush{}, err
			}
			color, err := r.readColor8()
			if err != nil {
				return gfx.Brush{}, err
			}
			stops = append(stops, gfx.GradientStop{Offset: offset, Color: color})
		}
		return gfx.Brush{
			Kind:          gfx.BrushLinearGradient,
			GradientStart: start,
			GradientEnd:   end,
			GradientStops: stops,
		}, nil
	default:
		return gfx.Brush{}, fmt.Errorf("unknown brush kind %d", kind)
	}
}

func (r *testPacketReader) readStrokeStyle() (gfx.StrokeStyle, error) {
	var s gfx.StrokeStyle
	var err error
	if s.Width, err = r.readF32(); err != nil {
		return s, err
	}
	var cap, join uint8
	if cap, err = r.readU8(); err != nil {
		return s, err
	}
	if join, err = r.readU8(); err != nil {
		return s, err
	}
	s.Cap = gfx.LineCap(cap)
	s.Join = gfx.LineJoin(join)
	if s.MiterLimit, err = r.readF32(); err != nil {
		return s, err
	}
	dashCount, err := r.readU32()
	if err != nil {
		return s, err
	}
	for i := uint32(0); i < dashCount; i++ {
		d, err := r.readF32()
		if err != nil {
			return s, err
		}
		s.Dash = append(s.Dash, d)
	}
	if s.DashOffset, err = r.readF32(); err != nil {
		return s, err
	}
	return s, nil
}

func (r *testPacketReader) readGlyphs() ([]testGlyphPacket, error) {
	count, err := r.readU32()
	if err != nil {
		return nil, err
	}
	glyphs := make([]testGlyphPacket, 0, count)
	for i := uint32(0); i < count; i++ {
		gid, err := r.readU32()
		if err != nil {
			return nil, err
		}
		x, err := r.readF32()
		if err != nil {
			return nil, err
		}
		y, err := r.readF32()
		if err != nil {
			return nil, err
		}
		glyphs = append(glyphs, testGlyphPacket{glyphID: gid, x: x, y: y})
	}
	return glyphs, nil
}

// decodeTestFramePacket mirrors the Rust `decode_frame` reader over the same
// wire format, returning the parsed structure for round-trip assertions.
func decodeTestFramePacket(data []byte) (*testFramePacket, error) {
	r := &testPacketReader{data: data}
	if r.remaining() < 4 || string(data[:4]) != framePacketMagic {
		return nil, fmt.Errorf("bad magic")
	}
	r.pos = 4
	version, err := r.readU32()
	if err != nil {
		return nil, err
	}
	if version != framePacketVersion {
		return nil, fmt.Errorf("version mismatch: got %d", version)
	}
	surfaceW, err := r.readU32()
	if err != nil {
		return nil, err
	}
	surfaceH, err := r.readU32()
	if err != nil {
		return nil, err
	}
	dpr, err := r.readF32()
	if err != nil {
		return nil, err
	}
	batchCount, err := r.readU32()
	if err != nil {
		return nil, err
	}
	frame := &testFramePacket{
		version:   version,
		surfaceW:  surfaceW,
		surfaceH:  surfaceH,
		deviceDPR: dpr,
	}
	for i := uint32(0); i < batchCount; i++ {
		batch, err := r.readBatch()
		if err != nil {
			return nil, err
		}
		frame.batches = append(frame.batches, batch)
	}
	frame.trailing = r.remaining()
	return frame, nil
}

func (r *testPacketReader) readBatch() (testBatchPacket, error) {
	var b testBatchPacket
	var err error
	if b.id, err = r.readU64(); err != nil {
		return b, err
	}
	if b.bounds, err = r.readRect(); err != nil {
		return b, err
	}
	if b.opacity, err = r.readF32(); err != nil {
		return b, err
	}
	if b.transform, err = r.readTransform(); err != nil {
		return b, err
	}
	if b.clip, err = r.readRect(); err != nil {
		return b, err
	}
	commandCount, err := r.readU32()
	if err != nil {
		return b, err
	}
	for i := uint32(0); i < commandCount; i++ {
		cmd, err := r.readCommand()
		if err != nil {
			return b, err
		}
		b.commands = append(b.commands, cmd)
	}
	return b, nil
}

func (r *testPacketReader) readCommand() (testCommandPacket, error) {
	var c testCommandPacket
	var err error
	if c.kind, err = r.readU8(); err != nil {
		return c, err
	}
	switch c.kind {
	case packetCmdFillRect:
		if c.rect, err = r.readRect(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdStrokeRect:
		if c.rect, err = r.readRect(); err != nil {
			return c, err
		}
		if c.stroke, err = r.readStrokeStyle(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdFillPath:
		if c.path, err = r.readPath(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdStrokePath:
		if c.path, err = r.readPath(); err != nil {
			return c, err
		}
		if c.stroke, err = r.readStrokeStyle(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdDrawPolyline:
		if c.points, err = r.readPoints(); err != nil {
			return c, err
		}
		if c.stroke, err = r.readStrokeStyle(); err != nil {
			return c, err
		}
		if c.brush, err = r.readBrush(); err != nil {
			return c, err
		}
		var closed uint8
		closed, err = r.readU8()
		c.closed = closed != 0
	case packetCmdDrawPoints:
		if c.points, err = r.readPoints(); err != nil {
			return c, err
		}
		if c.radius, err = r.readF32(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdDrawSelectionRects:
		if c.rects, err = r.readRects(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdPushTransform:
		c.matrix, err = r.readTransform()
	case packetCmdPopTransform:
	case packetCmdPushClipRect:
		c.rect, err = r.readRect()
	case packetCmdPopClip:
	case packetCmdPushOpacity:
		c.alpha, err = r.readF32()
	case packetCmdPopOpacity:
	case packetCmdDrawGlyphRun:
		if c.fontID, err = r.readU64(); err != nil {
			return c, err
		}
		if c.sizeBits, err = r.readU32(); err != nil {
			return c, err
		}
		if c.origin, err = r.readPoint(); err != nil {
			return c, err
		}
		if c.glyphs, err = r.readGlyphs(); err != nil {
			return c, err
		}
		c.brush, err = r.readBrush()
	case packetCmdDrawImage:
		if c.handle, err = r.readU64(); err != nil {
			return c, err
		}
		if c.rect, err = r.readRect(); err != nil {
			return c, err
		}
		if c.src, err = r.readRect(); err != nil {
			return c, err
		}
		if c.sampling, err = r.readU8(); err != nil {
			return c, err
		}
		c.opacity, err = r.readF32()
	case packetCmdDrawTexture:
		if c.textureID, err = r.readU64(); err != nil {
			return c, err
		}
		if c.rect, err = r.readRect(); err != nil {
			return c, err
		}
		if c.src, err = r.readRect(); err != nil {
			return c, err
		}
		if c.sampling, err = r.readU8(); err != nil {
			return c, err
		}
		c.opacity, err = r.readF32()
	case packetCmdDrawBlurredShadow:
		if c.path, err = r.readPath(); err != nil {
			return c, err
		}
		if c.color, err = r.readColor8(); err != nil {
			return c, err
		}
		if c.blurRadius, err = r.readF32(); err != nil {
			return c, err
		}
		if c.offset, err = r.readPoint(); err != nil {
			return c, err
		}
		var inner uint8
		inner, err = r.readU8()
		c.inner = inner != 0
	case packetCmdBeginRenderBatch:
		if c.rect, err = r.readRect(); err != nil {
			return c, err
		}
		c.cacheID, err = r.readU64()
	case packetCmdEndRenderBatch:
	default:
		err = fmt.Errorf("unknown opcode %d", c.kind)
	}
	return c, err
}
