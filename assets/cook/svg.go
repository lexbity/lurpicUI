package cook

import (
	"encoding/binary"
	"math"

	flatbuffers "github.com/google/flatbuffers/go"

	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/csg"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
)

// SVGCompiler compiles normalized SVG documents into CSG LOD0 plus dominant-color LOD2.
type SVGCompiler struct{}

// Extensions reports the handled source file extensions.
func (c *SVGCompiler) Extensions() []string {
	return []string{".svg"}
}

// Compile parses src as SVG and emits CSG LOD0 plus dominant-color LOD2.
func (c *SVGCompiler) Compile(src []byte, target Platform) ([]CompiledLOD, error) {
	doc, err := gfxsvg.ParseSVG(src)
	if err != nil {
		return nil, err
	}
	lod0, err := compileSVGLOD0(doc)
	if err != nil {
		return nil, err
	}
	lod1 := compileSVGLOD1(doc)
	lod2 := compileSVGLOD2(doc)
	return []CompiledLOD{
		{Level: 0, Data: lod0},
		{Level: 1, Data: lod1},
		{Level: 2, Data: lod2},
	}, nil
}

func compileSVGLOD0(doc gfxsvg.SVGDocument) ([]byte, error) {
	builder := flatbuffers.NewBuilder(0)
	shapeOffsets := make([]flatbuffers.UOffsetT, 0, len(doc.Elements))
	for _, el := range doc.Elements {
		shapeOffsets = append(shapeOffsets, buildCSGShape(builder, el))
	}
	shapesVec := builder.CreateVectorOfTables(shapeOffsets)
	bounds := doc.Bounds
	if bounds.IsEmpty() {
		bounds = doc.ViewBox
	}
	if bounds.IsEmpty() {
		bounds = unionElementBounds(doc.Elements)
	}
	csg.DocumentStart(builder)
	boundsVec := csg.CreateRect(builder, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	csg.DocumentAddBounds(builder, boundsVec)
	csg.DocumentAddShapes(builder, shapesVec)
	root := csg.DocumentEnd(builder)
	csg.FinishDocumentBuffer(builder, root)
	return append([]byte(nil), builder.FinishedBytes()...), nil
}

func buildCSGShape(builder *flatbuffers.Builder, el gfxsvg.SVGElement) flatbuffers.UOffsetT {
	verbsVec := buildVerbVector(builder, el.Path.Segments)
	coordsVec := buildCoordVector(builder, el.Path.Segments)
	bounds := el.Bounds
	if bounds.IsEmpty() {
		bounds = gfxsvg.Bounds(el.Path)
	}
	csg.ShapeStart(builder)
	csg.ShapeAddFill(builder, packPaint(el.Fill, el.Opacity, gfx.Color{A: 1}))
	if el.Stroke != nil {
		csg.ShapeAddStroke(builder, packPaint(el.Stroke.Paint, el.Opacity, gfx.Color{A: 1}))
		csg.ShapeAddStrokeWidth(builder, el.Stroke.Width)
	} else {
		csg.ShapeAddStroke(builder, 0)
		csg.ShapeAddStrokeWidth(builder, 0)
	}
	csg.ShapeAddVerbs(builder, verbsVec)
	csg.ShapeAddCoords(builder, coordsVec)
	boundsVec := csg.CreateRect(builder, bounds.Min.X, bounds.Min.Y, bounds.Max.X, bounds.Max.Y)
	csg.ShapeAddBounds(builder, boundsVec)
	return csg.ShapeEnd(builder)
}

func buildVerbVector(builder *flatbuffers.Builder, segments []gfx.PathSegment) flatbuffers.UOffsetT {
	csg.ShapeStartVerbsVector(builder, len(segments))
	for i := len(segments) - 1; i >= 0; i-- {
		builder.PrependByte(byte(mapPathVerb(segments[i].Verb)))
	}
	return builder.EndVector(len(segments))
}

func buildCoordVector(builder *flatbuffers.Builder, segments []gfx.PathSegment) flatbuffers.UOffsetT {
	coords := make([]float32, 0, countPathCoords(segments))
	for _, seg := range segments {
		switch seg.Verb {
		case gfx.PathMoveTo, gfx.PathLineTo:
			coords = append(coords, seg.Pts[0].X, seg.Pts[0].Y)
		case gfx.PathQuadTo:
			coords = append(coords, seg.Pts[0].X, seg.Pts[0].Y, seg.Pts[1].X, seg.Pts[1].Y)
		case gfx.PathCubicTo:
			coords = append(coords, seg.Pts[0].X, seg.Pts[0].Y, seg.Pts[1].X, seg.Pts[1].Y, seg.Pts[2].X, seg.Pts[2].Y)
		case gfx.PathClose:
		}
	}
	csg.ShapeStartCoordsVector(builder, len(coords))
	for i := len(coords) - 1; i >= 0; i-- {
		builder.PrependFloat32(coords[i])
	}
	return builder.EndVector(len(coords))
}

func countPathCoords(segments []gfx.PathSegment) int {
	total := 0
	for _, seg := range segments {
		switch seg.Verb {
		case gfx.PathMoveTo, gfx.PathLineTo:
			total += 2
		case gfx.PathQuadTo:
			total += 4
		case gfx.PathCubicTo:
			total += 6
		}
	}
	return total
}

func mapPathVerb(v gfx.PathVerb) csg.Verb {
	switch v {
	case gfx.PathMoveTo:
		return csg.VerbMoveTo
	case gfx.PathLineTo:
		return csg.VerbLineTo
	case gfx.PathQuadTo:
		return csg.VerbQuadTo
	case gfx.PathCubicTo:
		return csg.VerbCubicTo
	case gfx.PathClose:
		return csg.VerbClose
	default:
		return csg.VerbClose
	}
}

func compileSVGLOD2(doc gfxsvg.SVGDocument) []byte {
	color := dominantColor(doc)
	out := make([]byte, 4)
	binary.LittleEndian.PutUint32(out, color)
	return out
}

func dominantColor(doc gfxsvg.SVGDocument) uint32 {
	var sumR, sumG, sumB, sumA, sumWeight float64
	for _, el := range doc.Elements {
		color, weight, ok := visibleElementColor(el)
		if !ok || weight <= 0 {
			continue
		}
		sumWeight += weight
		sumR += float64(color.R) * weight
		sumG += float64(color.G) * weight
		sumB += float64(color.B) * weight
		sumA += float64(color.A) * weight
	}
	if sumWeight == 0 {
		return 0
	}
	avg := gfx.Color{
		R: float32(sumR / sumWeight),
		G: float32(sumG / sumWeight),
		B: float32(sumB / sumWeight),
		A: float32(sumA / sumWeight),
	}
	r, g, b, a := avg.ToRGBA8()
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
}

func visibleElementColor(el gfxsvg.SVGElement) (gfx.Color, float64, bool) {
	if el.Opacity <= 0 {
		return gfx.Color{}, 0, false
	}
	if color, ok := visiblePaintColor(el.Fill, el.Opacity, gfx.Color{A: 1}); ok {
		return color, shapeWeight(el), true
	}
	if el.Stroke != nil && el.Stroke.Width > 0 {
		if color, ok := visiblePaintColor(el.Stroke.Paint, el.Opacity, gfx.Color{A: 1}); ok {
			return color, strokeWeight(el), true
		}
	}
	return gfx.Color{}, 0, false
}

func visiblePaintColor(p gfxsvg.SVGPaint, elementOpacity float32, defaultCurrent gfx.Color) (gfx.Color, bool) {
	if p.Opacity <= 0 {
		return gfx.Color{}, false
	}
	var color gfx.Color
	switch p.Kind {
	case gfxsvg.SVGPaintColor:
		color = p.Color
	case gfxsvg.SVGPaintCurrentColor:
		color = defaultCurrent
	case gfxsvg.SVGPaintNone, gfxsvg.SVGPaintUnset, gfxsvg.SVGPaintLinearGradient:
		return gfx.Color{}, false
	default:
		return gfx.Color{}, false
	}
	alpha := p.Opacity * elementOpacity
	if alpha <= 0 {
		return gfx.Color{}, false
	}
	return gfx.Color{
		R: color.R * alpha,
		G: color.G * alpha,
		B: color.B * alpha,
		A: color.A * alpha,
	}, true
}

func shapeWeight(el gfxsvg.SVGElement) float64 {
	bounds := el.Bounds
	if bounds.IsEmpty() {
		bounds = gfxsvg.Bounds(el.Path)
	}
	if bounds.IsEmpty() {
		return 0
	}
	return float64(bounds.Width() * bounds.Height())
}

func strokeWeight(el gfxsvg.SVGElement) float64 {
	bounds := el.Bounds
	if bounds.IsEmpty() {
		bounds = gfxsvg.Bounds(el.Path)
	}
	if bounds.IsEmpty() || el.Stroke == nil || el.Stroke.Width <= 0 {
		return 0
	}
	perimeter := 2 * (math.Abs(float64(bounds.Width())) + math.Abs(float64(bounds.Height())))
	return perimeter * float64(el.Stroke.Width)
}

func packPaint(p gfxsvg.SVGPaint, elementOpacity float32, defaultCurrent gfx.Color) uint32 {
	color, ok := visiblePaintColor(p, elementOpacity, defaultCurrent)
	if !ok {
		return 0
	}
	r, g, b, a := color.ToRGBA8()
	return uint32(r) | uint32(g)<<8 | uint32(b)<<16 | uint32(a)<<24
}

func unionElementBounds(elements []gfxsvg.SVGElement) gfx.Rect {
	var bounds gfx.Rect
	for _, el := range elements {
		if el.Bounds.IsEmpty() {
			continue
		}
		if bounds.IsEmpty() {
			bounds = el.Bounds
			continue
		}
		bounds = bounds.Union(el.Bounds)
	}
	return bounds
}
