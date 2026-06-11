package cook

import (
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"unicode/utf8"

	flatbuffers "github.com/google/flatbuffers/go"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"

	"codeburg.org/lexbit/lurpicui/assets/schema/lurpic/cfnt"
)

// UnicodeRange describes an inclusive rune range selected for font cooking.
type UnicodeRange struct {
	Start rune
	End   rune
}

// ParseUnicodeRange parses a range in the form "U+0020-U+007E" or "U+0041".
func ParseUnicodeRange(spec string) (UnicodeRange, error) {
	s := strings.TrimSpace(spec)
	if s == "" {
		return UnicodeRange{}, fmt.Errorf("empty unicode range")
	}

	parts := strings.Split(s, "-")
	switch len(parts) {
	case 1:
		cp, err := parseUnicodeCodePoint(parts[0])
		if err != nil {
			return UnicodeRange{}, err
		}
		return UnicodeRange{Start: cp, End: cp}, nil
	case 2:
		start, err := parseUnicodeCodePoint(parts[0])
		if err != nil {
			return UnicodeRange{}, err
		}
		end, err := parseUnicodeCodePoint(parts[1])
		if err != nil {
			return UnicodeRange{}, err
		}
		if end < start {
			return UnicodeRange{}, fmt.Errorf("invalid unicode range %q: end before start", spec)
		}
		return UnicodeRange{Start: start, End: end}, nil
	default:
		return UnicodeRange{}, fmt.Errorf("invalid unicode range %q", spec)
	}
}

func parseUnicodeCodePoint(spec string) (rune, error) {
	s := strings.TrimSpace(strings.ToUpper(spec))
	s = strings.TrimPrefix(s, "U+")
	s = strings.TrimPrefix(s, "0X")
	if s == "" {
		return 0, fmt.Errorf("empty unicode code point")
	}
	v, err := strconv.ParseUint(s, 16, 32)
	if err != nil {
		return 0, fmt.Errorf("invalid unicode code point %q: %w", spec, err)
	}
	if v > utf8.MaxRune {
		return 0, fmt.Errorf("unicode code point out of range %q", spec)
	}
	return rune(v), nil
}

// FontCompiler compiles TTF/OTF sources into CFNT documents.
type FontCompiler struct {
	Ranges []UnicodeRange
}

// Extensions reports the handled source file extensions.
func (c *FontCompiler) Extensions() []string {
	return []string{".ttf", ".otf"}
}

// Compile parses src as an SFNT font and emits full and metric-only CFNT documents.
func (c *FontCompiler) Compile(src []byte, target Platform) ([]CompiledLOD, error) {
	_ = target

	if len(c.Ranges) == 0 {
		return nil, fmt.Errorf("font compiler has no unicode ranges configured")
	}

	f, err := sfnt.Parse(src)
	if err != nil {
		return nil, fmt.Errorf("sfnt.Parse: %w", err)
	}

	buf := &sfnt.Buffer{}
	unitsPerEm := uint16(f.UnitsPerEm()) //nolint:gosec // integer overflow conversion
	ppem := fixed.Int26_6(unitsPerEm)

	metrics, err := f.Metrics(buf, ppem, font.HintingNone)
	if err != nil {
		return nil, fmt.Errorf("font metrics: %w", err)
	}

	glyphs, err := collectGlyphMetrics(f, buf, ppem, c.Ranges)
	if err != nil {
		return nil, err
	}
	kerns, err := collectKernPairs(f, buf, ppem, glyphs)
	if err != nil {
		return nil, err
	}

	lod0, err := buildCFNTDocument(src, unitsPerEm, metrics, glyphs, kerns)
	if err != nil {
		return nil, err
	}
	lod1, err := buildCFNTDocument(nil, unitsPerEm, metrics, glyphs, kerns)
	if err != nil {
		return nil, err
	}

	return []CompiledLOD{
		{Level: 0, Data: lod0},
		{Level: 1, Data: lod1},
	}, nil
}

type fontGlyphMetric struct {
	Codepoint    uint32
	GlyphID      uint16
	AdvanceWidth float32
	LSB          float32
	BoundsXMin   float32
	BoundsYMin   float32
	BoundsXMax   float32
	BoundsYMax   float32
}

type fontKernPair struct {
	Left  uint16
	Right uint16
	Kern  float32
}

func collectGlyphMetrics(f *sfnt.Font, buf *sfnt.Buffer, ppem fixed.Int26_6, ranges []UnicodeRange) ([]fontGlyphMetric, error) {
	codepoints, err := expandUnicodeRanges(ranges)
	if err != nil {
		return nil, err
	}
	glyphs := make([]fontGlyphMetric, 0, len(codepoints))

	for _, cp := range codepoints {
		gid, err := f.GlyphIndex(buf, cp)
		if err != nil {
			if errors.Is(err, sfnt.ErrNotFound) {
				continue
			}
			return nil, fmt.Errorf("glyph index for U+%04X: %w", cp, err)
		}
		if gid == 0 {
			continue
		}

		advance, err := f.GlyphAdvance(buf, gid, ppem, font.HintingNone)
		if err != nil {
			return nil, fmt.Errorf("glyph advance for U+%04X: %w", cp, err)
		}
		bounds, _, err := f.GlyphBounds(buf, gid, ppem, font.HintingNone)
		if err != nil {
			return nil, fmt.Errorf("glyph bounds for U+%04X: %w", cp, err)
		}

		glyphs = append(glyphs, fontGlyphMetric{
			Codepoint:    uint32(cp), //nolint:gosec // integer overflow conversion
			GlyphID:      uint16(gid),
			AdvanceWidth: fixedToFloat32(advance),
			LSB:          fixedToFloat32(bounds.Min.X),
			BoundsXMin:   fixedToFloat32(bounds.Min.X),
			BoundsYMin:   fixedToFloat32(bounds.Min.Y),
			BoundsXMax:   fixedToFloat32(bounds.Max.X),
			BoundsYMax:   fixedToFloat32(bounds.Max.Y),
		})
	}

	sort.Slice(glyphs, func(i, j int) bool {
		return glyphs[i].Codepoint < glyphs[j].Codepoint
	})
	return glyphs, nil
}

func collectKernPairs(f *sfnt.Font, buf *sfnt.Buffer, ppem fixed.Int26_6, glyphs []fontGlyphMetric) ([]fontKernPair, error) {
	pairs := make([]fontKernPair, 0)
	for i := range glyphs {
		for j := range glyphs {
			kern, err := f.Kern(buf, sfnt.GlyphIndex(glyphs[i].GlyphID), sfnt.GlyphIndex(glyphs[j].GlyphID), ppem, font.HintingNone)
			if err != nil {
				if errors.Is(err, sfnt.ErrNotFound) {
					continue
				}
				return nil, fmt.Errorf("kern %d,%d: %w", glyphs[i].GlyphID, glyphs[j].GlyphID, err)
			}
			if kern == 0 {
				continue
			}
			pairs = append(pairs, fontKernPair{
				Left:  glyphs[i].GlyphID,
				Right: glyphs[j].GlyphID,
				Kern:  fixedToFloat32(kern),
			})
		}
	}

	sort.Slice(pairs, func(i, j int) bool {
		if pairs[i].Left != pairs[j].Left {
			return pairs[i].Left < pairs[j].Left
		}
		return pairs[i].Right < pairs[j].Right
	})
	return pairs, nil
}

func buildCFNTDocument(sfntBytes []byte, unitsPerEm uint16, metrics font.Metrics, glyphs []fontGlyphMetric, kernPairs []fontKernPair) ([]byte, error) {
	builder := flatbuffers.NewBuilder(0)

	metricsOffset := buildCFNTMetrics(builder, unitsPerEm, metrics)
	glyphsOffset := buildCFNTGlyphs(builder, glyphs)
	kernsOffset := buildCFNTKerns(builder, kernPairs)

	var sfntOffset flatbuffers.UOffsetT
	if len(sfntBytes) > 0 {
		sfntOffset = builder.CreateByteVector(sfntBytes)
	}

	cfnt.CFNTDocumentStart(builder)
	cfnt.CFNTDocumentAddMetrics(builder, metricsOffset)
	cfnt.CFNTDocumentAddGlyphs(builder, glyphsOffset)
	cfnt.CFNTDocumentAddKernPairs(builder, kernsOffset)
	if len(sfntBytes) > 0 {
		cfnt.CFNTDocumentAddSfntBytes(builder, sfntOffset)
	}
	root := cfnt.CFNTDocumentEnd(builder)
	cfnt.FinishCFNTDocumentBuffer(builder, root)
	return append([]byte(nil), builder.FinishedBytes()...), nil
}

func buildCFNTMetrics(builder *flatbuffers.Builder, unitsPerEm uint16, metrics font.Metrics) flatbuffers.UOffsetT {
	cfnt.FontMetricsStart(builder)
	cfnt.FontMetricsAddUnitsPerEm(builder, unitsPerEm)
	cfnt.FontMetricsAddAscent(builder, fixedToFloat32(metrics.Ascent))
	cfnt.FontMetricsAddDescent(builder, fixedToFloat32(metrics.Descent))
	lineGap := fixedToFloat32(metrics.Height - metrics.Ascent - metrics.Descent)
	if lineGap < 0 {
		lineGap = 0
	}
	cfnt.FontMetricsAddLineGap(builder, lineGap)
	cfnt.FontMetricsAddCapHeight(builder, fixedToFloat32(metrics.CapHeight))
	cfnt.FontMetricsAddXHeight(builder, fixedToFloat32(metrics.XHeight))
	return cfnt.FontMetricsEnd(builder)
}

func buildCFNTGlyphs(builder *flatbuffers.Builder, glyphs []fontGlyphMetric) flatbuffers.UOffsetT {
	offsets := make([]flatbuffers.UOffsetT, len(glyphs))
	for i := range glyphs {
		offsets[i] = buildCFNTGlyphMetric(builder, glyphs[i])
	}
	cfnt.CFNTDocumentStartGlyphsVector(builder, len(glyphs))
	for i := len(offsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(offsets[i])
	}
	return builder.EndVector(len(glyphs))
}

func buildCFNTKerns(builder *flatbuffers.Builder, kernPairs []fontKernPair) flatbuffers.UOffsetT {
	offsets := make([]flatbuffers.UOffsetT, len(kernPairs))
	for i := range kernPairs {
		offsets[i] = buildCFNTKernPair(builder, kernPairs[i])
	}
	cfnt.CFNTDocumentStartKernPairsVector(builder, len(kernPairs))
	for i := len(offsets) - 1; i >= 0; i-- {
		builder.PrependUOffsetT(offsets[i])
	}
	return builder.EndVector(len(kernPairs))
}

func buildCFNTGlyphMetric(builder *flatbuffers.Builder, metric fontGlyphMetric) flatbuffers.UOffsetT {
	cfnt.GlyphMetricStart(builder)
	cfnt.GlyphMetricAddCodepoint(builder, metric.Codepoint)
	cfnt.GlyphMetricAddGlyphId(builder, metric.GlyphID)
	cfnt.GlyphMetricAddAdvanceWidth(builder, metric.AdvanceWidth)
	cfnt.GlyphMetricAddLsb(builder, metric.LSB)
	cfnt.GlyphMetricAddBoundsXmin(builder, metric.BoundsXMin)
	cfnt.GlyphMetricAddBoundsYmin(builder, metric.BoundsYMin)
	cfnt.GlyphMetricAddBoundsXmax(builder, metric.BoundsXMax)
	cfnt.GlyphMetricAddBoundsYmax(builder, metric.BoundsYMax)
	return cfnt.GlyphMetricEnd(builder)
}

func buildCFNTKernPair(builder *flatbuffers.Builder, pair fontKernPair) flatbuffers.UOffsetT {
	cfnt.KernPairStart(builder)
	cfnt.KernPairAddLeft(builder, pair.Left)
	cfnt.KernPairAddRight(builder, pair.Right)
	cfnt.KernPairAddKern(builder, pair.Kern)
	return cfnt.KernPairEnd(builder)
}

func expandUnicodeRanges(ranges []UnicodeRange) ([]rune, error) {
	if len(ranges) == 0 {
		return nil, fmt.Errorf("font compiler has no unicode ranges configured")
	}
	out := make([]rune, 0)
	for _, r := range ranges {
		if r.End < r.Start {
			return nil, fmt.Errorf("invalid unicode range: %U-%U", r.Start, r.End)
		}
		for cp := r.Start; cp <= r.End; cp++ {
			if utf8.ValidRune(cp) {
				out = append(out, cp)
			}
		}
	}
	return out, nil
}

func fixedToFloat32(v fixed.Int26_6) float32 {
	return float32(v) / 64
}
