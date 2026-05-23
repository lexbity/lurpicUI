package cook

import (
	"os"
	"testing"

	flatbuffers "github.com/google/flatbuffers/go"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/cfnt"
)

func TestFontCompilerCompilesMetricsGlyphsAndKerns(t *testing.T) {
	src := mustReadTestFont(t)
	f := mustParseTestFont(t, src)

	compiler := &FontCompiler{
		Ranges: []UnicodeRange{
			{Start: 'A', End: 'Z'},
			{Start: 'a', End: 'z'},
		},
	}
	lods, err := compiler.Compile(src, PlatformLinux)
	if err != nil {
		t.Fatalf("compile font: %v", err)
	}
	if len(lods) != 2 {
		t.Fatalf("unexpected lod count: %d", len(lods))
	}
	if lods[0].Level != 0 || lods[1].Level != 1 {
		t.Fatalf("unexpected lod levels: %+v", lods)
	}
	if len(lods[0].Data) == 0 {
		t.Fatal("expected lod0 data")
	}
	if len(lods[1].Data) == 0 {
		t.Fatal("expected lod1 data")
	}

	var lod0 cfnt.CFNTDocument
	lod0.Init(lods[0].Data, flatbuffers.GetUOffsetT(lods[0].Data))
	var lod1 cfnt.CFNTDocument
	lod1.Init(lods[1].Data, flatbuffers.GetUOffsetT(lods[1].Data))

	if got := lod0.SfntBytesLength(); got != len(src) {
		t.Fatalf("unexpected lod0 sfnt size: %d", got)
	}
	if got := lod1.SfntBytesLength(); got != 0 {
		t.Fatalf("expected lod1 to omit sfnt bytes, got %d", got)
	}

	wantMetrics, wantUnitsPerEm := expectedFontMetrics(t, f)
	var gotMetrics cfnt.FontMetrics
	if lod0.Metrics(&gotMetrics) == nil {
		t.Fatal("expected lod0 metrics")
	}
	if lod1.Metrics(&gotMetrics) == nil {
		t.Fatal("expected lod1 metrics")
	}
	if got := gotMetrics.UnitsPerEm(); got != wantUnitsPerEm {
		t.Fatalf("unexpected units_per_em: got %d want %d", got, wantUnitsPerEm)
	}
	if got := gotMetrics.Ascent(); got != wantMetrics.Ascent {
		t.Fatalf("unexpected ascent: got %v want %v", got, wantMetrics.Ascent)
	}
	if got := gotMetrics.Descent(); got != wantMetrics.Descent {
		t.Fatalf("unexpected descent: got %v want %v", got, wantMetrics.Descent)
	}
	if got := gotMetrics.LineGap(); got != wantMetrics.LineGap {
		t.Fatalf("unexpected line gap: got %v want %v", got, wantMetrics.LineGap)
	}
	if got := gotMetrics.CapHeight(); got != wantMetrics.CapHeight {
		t.Fatalf("unexpected cap height: got %v want %v", got, wantMetrics.CapHeight)
	}
	if got := gotMetrics.XHeight(); got != wantMetrics.XHeight {
		t.Fatalf("unexpected x height: got %v want %v", got, wantMetrics.XHeight)
	}

	if got := lod0.GlyphsLength(); got != 52 {
		t.Fatalf("unexpected glyph count: %d", got)
	}
	if got := lod1.GlyphsLength(); got != 52 {
		t.Fatalf("unexpected lod1 glyph count: %d", got)
	}
	if got := lod0.KernPairsLength(); got == 0 {
		t.Fatal("expected non-zero kern pairs in lod0")
	}
	if got := lod1.KernPairsLength(); got == 0 {
		t.Fatal("expected non-zero kern pairs in lod1")
	}

	var glyph cfnt.GlyphMetric
	if !lod0.Glyphs(&glyph, 0) {
		t.Fatal("expected glyph 0 in lod0")
	}
	if glyph.Codepoint() != 'A' {
		t.Fatalf("unexpected first glyph codepoint: %d", glyph.Codepoint())
	}
	wantGlyph := expectedGlyphMetric(t, f, 'A')
	if glyph.GlyphId() != wantGlyph.GlyphID {
		t.Fatalf("unexpected glyph id: got %d want %d", glyph.GlyphId(), wantGlyph.GlyphID)
	}
	if glyph.AdvanceWidth() != wantGlyph.AdvanceWidth {
		t.Fatalf("unexpected advance width: got %v want %v", glyph.AdvanceWidth(), wantGlyph.AdvanceWidth)
	}
	if glyph.Lsb() != wantGlyph.LSB {
		t.Fatalf("unexpected lsb: got %v want %v", glyph.Lsb(), wantGlyph.LSB)
	}
	if glyph.BoundsXmin() != wantGlyph.BoundsXMin || glyph.BoundsYmin() != wantGlyph.BoundsYMin || glyph.BoundsXmax() != wantGlyph.BoundsXMax || glyph.BoundsYmax() != wantGlyph.BoundsYMax {
		t.Fatalf("unexpected glyph bounds: got=(%v,%v)-(%v,%v) want=(%v,%v)-(%v,%v)",
			glyph.BoundsXmin(), glyph.BoundsYmin(), glyph.BoundsXmax(), glyph.BoundsYmax(),
			wantGlyph.BoundsXMin, wantGlyph.BoundsYMin, wantGlyph.BoundsXMax, wantGlyph.BoundsYMax,
		)
	}

	wantGlyphA := expectedGlyphMetric(t, f, 'A')
	wantGlyphV := expectedGlyphMetric(t, f, 'V')
	wantKern := expectedKernValue(t, f, 'A', 'V')
	found := false
	var kern cfnt.KernPair
	for i := 0; i < lod0.KernPairsLength(); i++ {
		if !lod0.KernPairs(&kern, i) {
			t.Fatalf("expected kern pair %d", i)
		}
		if kern.Left() == wantGlyphA.GlyphID && kern.Right() == wantGlyphV.GlyphID {
			found = true
			if kern.Kern() != wantKern {
				t.Fatalf("unexpected AV kern: got %v want %v", kern.Kern(), wantKern)
			}
			break
		}
	}
	if !found {
		t.Fatal("expected AV kern pair in lod0")
	}
}

func TestParseUnicodeRange(t *testing.T) {
	r, err := ParseUnicodeRange("U+0020-U+007E")
	if err != nil {
		t.Fatalf("parse range: %v", err)
	}
	if r.Start != 0x20 || r.End != 0x7E {
		t.Fatalf("unexpected range: %+v", r)
	}
}

func mustReadTestFont(t *testing.T) []byte {
	t.Helper()

	src, err := os.ReadFile("/usr/share/fonts/liberation/LiberationSerif-Regular.ttf")
	if err != nil {
		t.Fatalf("read test font: %v", err)
	}
	return src
}

func mustParseTestFont(t *testing.T, src []byte) *sfnt.Font {
	t.Helper()

	f, err := sfnt.Parse(src)
	if err != nil {
		t.Fatalf("parse test font: %v", err)
	}
	return f
}

type expectedMetrics struct {
	UnitsPerEm uint16
	Ascent     float32
	Descent    float32
	LineGap    float32
	CapHeight  float32
	XHeight    float32
}

func expectedFontMetrics(t *testing.T, f *sfnt.Font) (expectedMetrics, uint16) {
	t.Helper()

	buf := &sfnt.Buffer{}
	units := uint16(f.UnitsPerEm())
	ppem := fixed.Int26_6(units)
	metrics, err := f.Metrics(buf, ppem, font.HintingNone)
	if err != nil {
		t.Fatalf("expected metrics: %v", err)
	}
	lineGap := float32(metrics.Height-metrics.Ascent-metrics.Descent) / 64
	if lineGap < 0 {
		lineGap = 0
	}
	return expectedMetrics{
		UnitsPerEm: units,
		Ascent:     float32(metrics.Ascent) / 64,
		Descent:    float32(metrics.Descent) / 64,
		LineGap:    lineGap,
		CapHeight:  float32(metrics.CapHeight) / 64,
		XHeight:    float32(metrics.XHeight) / 64,
	}, units
}

func expectedGlyphMetric(t *testing.T, f *sfnt.Font, cp rune) fontGlyphMetric {
	t.Helper()

	buf := &sfnt.Buffer{}
	units := uint16(f.UnitsPerEm())
	ppem := fixed.Int26_6(units)
	gid, err := f.GlyphIndex(buf, cp)
	if err != nil {
		t.Fatalf("glyph index %q: %v", cp, err)
	}
	advance, err := f.GlyphAdvance(buf, gid, ppem, font.HintingNone)
	if err != nil {
		t.Fatalf("glyph advance %q: %v", cp, err)
	}
	bounds, _, err := f.GlyphBounds(buf, gid, ppem, font.HintingNone)
	if err != nil {
		t.Fatalf("glyph bounds %q: %v", cp, err)
	}
	return fontGlyphMetric{
		Codepoint:    uint32(cp),
		GlyphID:      uint16(gid),
		AdvanceWidth: float32(advance) / 64,
		LSB:          float32(bounds.Min.X) / 64,
		BoundsXMin:   float32(bounds.Min.X) / 64,
		BoundsYMin:   float32(bounds.Min.Y) / 64,
		BoundsXMax:   float32(bounds.Max.X) / 64,
		BoundsYMax:   float32(bounds.Max.Y) / 64,
	}
}

func expectedKernValue(t *testing.T, f *sfnt.Font, left, right rune) float32 {
	t.Helper()

	buf := &sfnt.Buffer{}
	units := uint16(f.UnitsPerEm())
	ppem := fixed.Int26_6(units)
	lid, err := f.GlyphIndex(buf, left)
	if err != nil {
		t.Fatalf("glyph index %q: %v", left, err)
	}
	rid, err := f.GlyphIndex(buf, right)
	if err != nil {
		t.Fatalf("glyph index %q: %v", right, err)
	}
	kern, err := f.Kern(buf, lid, rid, ppem, font.HintingNone)
	if err != nil {
		t.Fatalf("kern %q%q: %v", left, right, err)
	}
	return float32(kern) / 64
}
