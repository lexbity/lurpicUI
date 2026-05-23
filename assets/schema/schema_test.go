package schema

import (
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"

	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/cfnt"
	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/csg"
)

func TestGeneratedSchemasRoundTripAndZeroAllocReads(t *testing.T) {
	csgBuf := buildCSGDocument(t)
	cfntBuf := buildCFNTDocument(t)

	var doc csg.Document
	doc.Init(csgBuf, flatbuffers.GetUOffsetT(csgBuf))
	var shape csg.Shape
	var bounds csg.Rect
	var min csg.Vec2
	var max csg.Vec2
	if got := doc.ShapesLength(); got != 1 {
		t.Fatalf("unexpected csg shape count: %d", got)
	}
	if !doc.Shapes(&shape, 0) {
		t.Fatal("expected csg shape 0")
	}
	if got := shape.Fill(); got != 0x11223344 {
		t.Fatalf("unexpected fill: %#x", got)
	}
	if got := shape.VerbsLength(); got != 3 {
		t.Fatalf("unexpected verb count: %d", got)
	}
	if got := shape.Bounds(&bounds); got == nil {
		t.Fatal("expected shape bounds")
	}
	if got := bounds.Min(&min); got == nil {
		t.Fatal("expected min vector")
	}
	if got := bounds.Max(&max); got == nil {
		t.Fatal("expected max vector")
	}
	if min.X() != 0 || min.Y() != 0 || max.X() != 10 || max.Y() != 10 {
		t.Fatalf("unexpected bounds: min=(%v,%v) max=(%v,%v)", min.X(), min.Y(), max.X(), max.Y())
	}

	if allocs := testing.AllocsPerRun(1000, func() {
		var localDoc csg.Document
		localDoc.Init(csgBuf, flatbuffers.GetUOffsetT(csgBuf))
		var localShape csg.Shape
		var localBounds csg.Rect
		var localMin csg.Vec2
		var localMax csg.Vec2
		if !localDoc.Shapes(&localShape, 0) {
			t.Fatal("missing csg shape during alloc check")
		}
		_ = localShape.Fill()
		_ = localShape.Stroke()
		_ = localShape.StrokeWidth()
		_ = localShape.Verbs(0)
		_ = localShape.VerbsLength()
		_ = localShape.Coords(0)
		_ = localShape.CoordsLength()
		_ = localShape.Bounds(&localBounds)
		_ = localBounds.Min(&localMin)
		_ = localBounds.Max(&localMax)
	}); allocs != 0 {
		t.Fatalf("expected zero allocations while reading csg buffer, got %v", allocs)
	}

	var font cfnt.CFNTDocument
	font.Init(cfntBuf, flatbuffers.GetUOffsetT(cfntBuf))
	var metrics cfnt.FontMetrics
	var glyph cfnt.GlyphMetric
	var kern cfnt.KernPair
	if got := font.GlyphsLength(); got != 2 {
		t.Fatalf("unexpected glyph count: %d", got)
	}
	if got := font.KernPairsLength(); got != 1 {
		t.Fatalf("unexpected kern count: %d", got)
	}
	if got := font.Metrics(&metrics); got == nil {
		t.Fatal("expected metrics")
	}
	if metrics.UnitsPerEm() != 2048 || metrics.Ascent() != 1500 || metrics.Descent() != -500 {
		t.Fatalf("unexpected metrics: %+v", metrics)
	}
	if !font.Glyphs(&glyph, 1) {
		t.Fatal("expected glyph 1")
	}
	if glyph.Codepoint() != 66 || glyph.GlyphId() != 8 {
		t.Fatalf("unexpected glyph: codepoint=%d glyph=%d", glyph.Codepoint(), glyph.GlyphId())
	}
	if !font.KernPairs(&kern, 0) {
		t.Fatal("expected kern pair 0")
	}
	if kern.Left() != 7 || kern.Right() != 8 || kern.Kern() != -1.25 {
		t.Fatalf("unexpected kern pair: %+v", kern)
	}
	if got := font.SfntBytesLength(); got != 4 {
		t.Fatalf("unexpected sfnt byte count: %d", got)
	}
	if font.SfntBytes(2) != 0xCC {
		t.Fatalf("unexpected sfnt payload byte: %x", font.SfntBytes(2))
	}

	if allocs := testing.AllocsPerRun(1000, func() {
		var localFont cfnt.CFNTDocument
		localFont.Init(cfntBuf, flatbuffers.GetUOffsetT(cfntBuf))
		var localMetrics cfnt.FontMetrics
		var localGlyph cfnt.GlyphMetric
		var localKern cfnt.KernPair
		_ = localFont.Metrics(&localMetrics)
		_ = localFont.Glyphs(&localGlyph, 0)
		_ = localFont.KernPairs(&localKern, 0)
		_ = localFont.GlyphsLength()
		_ = localFont.KernPairsLength()
		_ = localFont.SfntBytesLength()
		_ = localFont.SfntBytesBytes()
	}); allocs != 0 {
		t.Fatalf("expected zero allocations while reading cfnt buffer, got %v", allocs)
	}
}

func buildCSGDocument(t *testing.T) []byte {
	t.Helper()

	builder := flatbuffers.NewBuilder(0)

	csg.ShapeStartVerbsVector(builder, 3)
	builder.PrependInt8(int8(csg.VerbClose))
	builder.PrependInt8(int8(csg.VerbLineTo))
	builder.PrependInt8(int8(csg.VerbMoveTo))
	verbs := builder.EndVector(3)

	csg.ShapeStartCoordsVector(builder, 4)
	builder.PrependFloat32(10)
	builder.PrependFloat32(10)
	builder.PrependFloat32(0)
	builder.PrependFloat32(0)
	coords := builder.EndVector(4)

	csg.ShapeStart(builder)
	csg.ShapeAddFill(builder, 0x11223344)
	csg.ShapeAddStroke(builder, 0x55667788)
	csg.ShapeAddStrokeWidth(builder, 2.5)
	csg.ShapeAddVerbs(builder, verbs)
	csg.ShapeAddCoords(builder, coords)
	bounds := csg.CreateRect(builder, 0, 0, 10, 10)
	csg.ShapeAddBounds(builder, bounds)
	shape := csg.ShapeEnd(builder)

	csg.DocumentStartShapesVector(builder, 1)
	builder.PrependUOffsetT(shape)
	shapes := builder.EndVector(1)

	csg.DocumentStart(builder)
	docBounds := csg.CreateRect(builder, 0, 0, 10, 10)
	csg.DocumentAddBounds(builder, docBounds)
	csg.DocumentAddShapes(builder, shapes)
	doc := csg.DocumentEnd(builder)
	csg.FinishDocumentBuffer(builder, doc)
	return builder.FinishedBytes()
}

func buildCFNTDocument(t *testing.T) []byte {
	t.Helper()

	builder := flatbuffers.NewBuilder(0)

	cfnt.FontMetricsStart(builder)
	cfnt.FontMetricsAddUnitsPerEm(builder, 2048)
	cfnt.FontMetricsAddAscent(builder, 1500)
	cfnt.FontMetricsAddDescent(builder, -500)
	cfnt.FontMetricsAddLineGap(builder, 120)
	cfnt.FontMetricsAddCapHeight(builder, 1400)
	cfnt.FontMetricsAddXHeight(builder, 1000)
	metrics := cfnt.FontMetricsEnd(builder)

	cfnt.GlyphMetricStart(builder)
	cfnt.GlyphMetricAddCodepoint(builder, 65)
	cfnt.GlyphMetricAddGlyphId(builder, 7)
	cfnt.GlyphMetricAddAdvanceWidth(builder, 9.5)
	cfnt.GlyphMetricAddLsb(builder, 1.25)
	cfnt.GlyphMetricAddBoundsXmin(builder, -0.5)
	cfnt.GlyphMetricAddBoundsYmin(builder, -1)
	cfnt.GlyphMetricAddBoundsXmax(builder, 9.5)
	cfnt.GlyphMetricAddBoundsYmax(builder, 10.25)
	glyphA := cfnt.GlyphMetricEnd(builder)

	cfnt.GlyphMetricStart(builder)
	cfnt.GlyphMetricAddCodepoint(builder, 66)
	cfnt.GlyphMetricAddGlyphId(builder, 8)
	cfnt.GlyphMetricAddAdvanceWidth(builder, 8.25)
	cfnt.GlyphMetricAddLsb(builder, 0.5)
	cfnt.GlyphMetricAddBoundsXmin(builder, 0)
	cfnt.GlyphMetricAddBoundsYmin(builder, -0.75)
	cfnt.GlyphMetricAddBoundsXmax(builder, 8.25)
	cfnt.GlyphMetricAddBoundsYmax(builder, 9.75)
	glyphB := cfnt.GlyphMetricEnd(builder)

	cfnt.KernPairStart(builder)
	cfnt.KernPairAddLeft(builder, 7)
	cfnt.KernPairAddRight(builder, 8)
	cfnt.KernPairAddKern(builder, -1.25)
	kern := cfnt.KernPairEnd(builder)

	cfnt.CFNTDocumentStartGlyphsVector(builder, 2)
	builder.PrependUOffsetT(glyphB)
	builder.PrependUOffsetT(glyphA)
	glyphs := builder.EndVector(2)

	cfnt.CFNTDocumentStartKernPairsVector(builder, 1)
	builder.PrependUOffsetT(kern)
	kernPairs := builder.EndVector(1)

	sfnt := builder.CreateByteVector([]byte{0xaa, 0xbb, 0xcc, 0xdd})

	cfnt.CFNTDocumentStart(builder)
	cfnt.CFNTDocumentAddMetrics(builder, metrics)
	cfnt.CFNTDocumentAddGlyphs(builder, glyphs)
	cfnt.CFNTDocumentAddKernPairs(builder, kernPairs)
	cfnt.CFNTDocumentAddSfntBytes(builder, sfnt)
	doc := cfnt.CFNTDocumentEnd(builder)
	cfnt.FinishCFNTDocumentBuffer(builder, doc)
	return builder.FinishedBytes()
}
