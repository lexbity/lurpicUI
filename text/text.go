package text

import (
	"sort"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	textsegmenter "github.com/go-text/typesetting/segmenter"
)

// Weight encodes font weight values in CSS-like increments.
type Weight uint16

const (
	WeightThin     Weight = 100
	WeightLight    Weight = 300
	WeightRegular  Weight = 400
	WeightMedium   Weight = 500
	WeightSemiBold Weight = 600
	WeightBold     Weight = 700
	WeightBlack    Weight = 900
)

// Style describes the slant of a font.
type Style uint8

const (
	StyleNormal Style = iota
	StyleItalic
	StyleOblique
)

// TextUnit identifies the indexing unit used by a position or range.
type TextUnit uint8

const (
	TextUnitRune TextUnit = iota
	TextUnitGrapheme
)

// LineMetrics captures the shaped metrics for a line box.
type LineMetrics struct {
	Ascent     float32
	Descent    float32
	Leading    float32
	LineHeight float32
}

// TextStyle describes the styling to apply to a span of text.
type TextStyle struct {
	Family        string
	Size          float32
	Weight        Weight
	Style         Style
	LineHeight    float32
	LetterSpacing float32
	TabWidth      int
}

// DefaultStyle returns the baseline text style.
func DefaultStyle() TextStyle {
	return TextStyle{
		Size:   14,
		Weight: WeightRegular,
		Style:  StyleNormal,
	}
}

// TextSpan represents a run of text with a uniform style.
type TextSpan struct {
	Text      string
	Style     TextStyle
	Direction di.Direction
	Language  language.Language
	Script    language.Script
}

// TextAlignment controls paragraph alignment.
type TextAlignment uint8

const (
	AlignLeft TextAlignment = iota
	AlignCenter
	AlignRight
)

// Paragraph is the input to layout and shaping.
type Paragraph struct {
	Spans     []TextSpan
	MaxWidth  float32
	Alignment TextAlignment
	Direction di.Direction
	Language  language.Language
	Script    language.Script
}

// Point is a lightweight 2D coordinate used by text layout.
type Point struct {
	X float32
	Y float32
}

// Rect is a lightweight 2D axis-aligned rectangle used by text layout.
type Rect struct {
	Min Point
	Max Point
}

// RectFromXYWH constructs a rectangle from position and size.
func RectFromXYWH(x, y, w, h float32) Rect {
	return Rect{
		Min: Point{X: x, Y: y},
		Max: Point{X: x + w, Y: y + h},
	}
}

// Width returns the rectangle width.
func (r Rect) Width() float32 {
	return r.Max.X - r.Min.X
}

// Height returns the rectangle height.
func (r Rect) Height() float32 {
	return r.Max.Y - r.Min.Y
}

// IsEmpty reports whether the rectangle is empty.
func (r Rect) IsEmpty() bool {
	return r.Max.X <= r.Min.X || r.Max.Y <= r.Min.Y
}

// Contains reports whether the point lies within the rectangle.
func (r Rect) Contains(p Point) bool {
	return p.X >= r.Min.X && p.X <= r.Max.X && p.Y >= r.Min.Y && p.Y <= r.Max.Y
}

// Inset expands or shrinks the rectangle.
func (r Rect) Inset(dx, dy float32) Rect {
	return Rect{
		Min: Point{X: r.Min.X - dx, Y: r.Min.Y - dy},
		Max: Point{X: r.Max.X + dx, Y: r.Max.Y + dy},
	}
}

// FontSource describes a font input source.
type FontSource struct {
	Path string
	Data []byte
	Name string
}

type fontFaceRecord struct {
	face     *font.Face
	desc     font.Description
	source   FontSource
	cacheKey uint64
}

// FontFace is an opaque wrapper around a resolved font face record.
type FontFace struct {
	face *fontFaceRecord
}

// GlyphRun is the exported glyph-run handle referenced by gfx.DrawGlyphRun.
//
// Phase 1 keeps the shape intentionally minimal. Later phases expand it into a
// full positioned glyph sequence.
type GlyphRun struct {
	Glyphs           []PositionedGlyph
	Face             FontFace
	Size             float32
	Style            TextStyle
	Bounds           Rect
	Advance          float32
	Text             string
	Direction        di.Direction
	Level            int
	GraphemeAdvances []float32
	Language         language.Language
	Script           language.Script
	Metrics          LineMetrics
	LogicalIndex     int
}

// PositionedGlyph is a shaped glyph positioned within a run.
type PositionedGlyph struct {
	GlyphID   uint32
	X         float32
	Y         float32
	Advance   float32
	RuneIndex int
}

// ShapedLine is one wrapped line within a layout.
//
// Bounds is the line box in layout-local coordinates. It includes the authored
// line height and any alignment shift; it is not a glyph-ink bounds estimate.
// Baseline is the line-local vertical offset from Bounds.Min.Y to the text
// baseline derived from the shaped line metrics.
type ShapedLine struct {
	Runs       []GlyphRun
	Bounds     Rect
	Baseline   float32
	FirstRune  int
	RuneCount  int
	Direction  di.Direction
	Language   language.Language
	Script     language.Script
	Metrics    LineMetrics
	clusterMap []float32
}

// TextLayout is the shaped and wrapped result of a paragraph.
//
// Bounds is the content box for the whole shaped paragraph in layout-local
// coordinates. Individual ShapedLine values retain their own line boxes and
// baselines inside that content box.
type TextLayout struct {
	Lines      []ShapedLine
	Bounds     Rect
	LineHeight float32
	Baseline   float32
	Paragraph  Paragraph
	Metrics    LineMetrics
	Source     string
	source     string
	graphemes  []int
}

// Affinity resolves cursor placement at line boundaries.
type Affinity uint8

const (
	AffinityUpstream Affinity = iota
	AffinityDownstream
)

// TextPosition identifies a cursor location within a shaped layout.
type TextPosition struct {
	Index    int
	Unit     TextUnit
	Affinity Affinity
}

// TextRange represents a half-open range of rune indices.
type TextRange struct {
	Start int
	End   int
	Unit  TextUnit
}

// IsEmpty reports whether the range contains no runes.
func (r TextRange) IsEmpty() bool {
	return r.Start >= r.End
}

// Contains reports whether i lies within the range.
func (r TextRange) Contains(i int) bool {
	r = r.Normalized()
	return i >= r.Start && i < r.End
}

// Normalized ensures Start <= End.
func (r TextRange) Normalized() TextRange {
	if r.Start <= r.End {
		return r
	}
	return TextRange{Start: r.End, End: r.Start, Unit: r.Unit}
}

// RunePosition constructs a rune-indexed text position.
func RunePosition(index int, affinity Affinity) TextPosition {
	return TextPosition{Index: index, Unit: TextUnitRune, Affinity: affinity}
}

// GraphemePosition constructs a grapheme-indexed text position.
func GraphemePosition(index int, affinity Affinity) TextPosition {
	return TextPosition{Index: index, Unit: TextUnitGrapheme, Affinity: affinity}
}

// RuneRange constructs a rune-indexed text range.
func RuneRange(start, end int) TextRange {
	return TextRange{Start: start, End: end, Unit: TextUnitRune}
}

// GraphemeRange constructs a grapheme-indexed text range.
func GraphemeRange(start, end int) TextRange {
	return TextRange{Start: start, End: end, Unit: TextUnitGrapheme}
}

// LineCount returns the number of shaped lines.
func (l *TextLayout) LineCount() int {
	if l == nil {
		return 0
	}
	return len(l.Lines)
}

// RuneCount returns the total number of runes represented by the layout.
func (l *TextLayout) RuneCount() int {
	if l == nil {
		return 0
	}
	total := 0
	for _, line := range l.Lines {
		total += line.RuneCount
	}
	return total
}

// GraphemeCount returns the total number of grapheme clusters represented by the layout.
func (l *TextLayout) GraphemeCount() int {
	if l == nil {
		return 0
	}
	if len(l.graphemes) > 0 {
		return len(l.graphemes) - 1
	}
	return l.RuneCount()
}

// RuneIndex returns the rune index for pos, converting grapheme positions when needed.
func (l *TextLayout) RuneIndex(pos TextPosition) int {
	if l == nil {
		return 0
	}
	if pos.Unit != TextUnitGrapheme {
		return clampInt(pos.Index, 0, l.RuneCount())
	}
	return l.runeIndexForGraphemeIndex(pos.Index)
}

// RuneBounds returns rune-based bounds for r, converting grapheme positions when needed.
func (l *TextLayout) RuneBounds(r TextRange) (int, int) {
	if l == nil {
		return 0, 0
	}
	if r.Unit != TextUnitGrapheme {
		r = r.Normalized()
		return clampInt(r.Start, 0, l.RuneCount()), clampInt(r.End, 0, l.RuneCount())
	}
	r = r.Normalized()
	return l.runeIndexForGraphemeIndex(r.Start), l.runeIndexForGraphemeIndex(r.End)
}

// HitTest maps a point in layout-local space to the nearest text position.
func (l *TextLayout) HitTest(p Point) TextPosition {
	if l == nil || len(l.Lines) == 0 {
		return GraphemePosition(0, AffinityDownstream)
	}
	lineIndex := l.lineIndexForPoint(p)
	line := &l.Lines[lineIndex]
	if len(line.Runs) == 0 {
		return l.positionFromRuneIndex(line.FirstRune, AffinityDownstream)
	}
	if p.X <= line.Bounds.Min.X {
		return l.positionFromRuneIndex(line.FirstRune, AffinityDownstream)
	}
	if p.X >= line.Bounds.Max.X {
		return l.positionFromRuneIndex(line.FirstRune+line.RuneCount, AffinityUpstream)
	}
	if idx, ok := line.hitTestClusterIndex(p.X); ok {
		lineFirstGrapheme := l.graphemeIndexForRuneIndex(line.FirstRune, AffinityDownstream)
		return GraphemePosition(lineFirstGrapheme+idx, AffinityDownstream)
	}
	return l.positionFromRuneIndex(line.FirstRune+line.RuneCount, AffinityUpstream)
}

// CaretRect returns the cursor rectangle for a text position.
func (l *TextLayout) CaretRect(pos TextPosition) Rect {
	if l == nil || len(l.Lines) == 0 {
		return Rect{}
	}
	lineIndex := l.lineIndexForPosition(pos)
	line := &l.Lines[lineIndex]
	x := l.positionX(pos, lineIndex)
	return RectFromXYWH(x, line.Bounds.Min.Y, 2, line.Bounds.Height())
}

// SelectionRects returns rectangles covering the supplied range.
func (l *TextLayout) SelectionRects(r TextRange) []Rect {
	if l == nil || len(l.Lines) == 0 {
		return nil
	}
	r = r.Normalized()
	if r.IsEmpty() {
		return nil
	}
	startRune, endRune := l.RuneBounds(r)
	var rects []Rect
	for i := range l.Lines {
		line := &l.Lines[i]
		start := maxInt(startRune, line.FirstRune)
		end := minInt(endRune, line.FirstRune+line.RuneCount)
		if start >= end {
			continue
		}
		x0 := l.positionX(l.positionFromRuneIndex(start, AffinityDownstream), i)
		x1 := l.positionX(l.positionFromRuneIndex(end, AffinityUpstream), i)
		rects = append(rects, RectFromXYWH(x0, line.Bounds.Min.Y, x1-x0, line.Bounds.Height()))
	}
	return rects
}

// LineAt returns the index of the line containing pos.
func (l *TextLayout) LineAt(pos TextPosition) int {
	return l.lineIndexForPosition(pos)
}

// PositionAtLineStart returns the first cursor position on line i.
func (l *TextLayout) PositionAtLineStart(line int) TextPosition {
	if l == nil || len(l.Lines) == 0 {
		return GraphemePosition(0, AffinityDownstream)
	}
	line = clampInt(line, 0, len(l.Lines)-1)
	return l.positionFromRuneIndex(l.Lines[line].FirstRune, AffinityDownstream)
}

// PositionAtLineEnd returns the last cursor position on line i.
func (l *TextLayout) PositionAtLineEnd(line int) TextPosition {
	if l == nil || len(l.Lines) == 0 {
		return GraphemePosition(0, AffinityDownstream)
	}
	line = clampInt(line, 0, len(l.Lines)-1)
	ln := l.Lines[line]
	return l.positionFromRuneIndex(ln.FirstRune+ln.RuneCount, AffinityUpstream)
}

// NextPosition advances by one unit, clamping at the end of the document.
func (l *TextLayout) NextPosition(pos TextPosition) TextPosition {
	if pos.Unit == TextUnitGrapheme {
		count := l.GraphemeCount()
		if pos.Index >= count {
			return TextPosition{Index: count, Unit: pos.Unit, Affinity: AffinityUpstream}
		}
		return TextPosition{Index: pos.Index + 1, Unit: pos.Unit, Affinity: AffinityDownstream}
	}
	count := l.RuneCount()
	if pos.Index >= count {
		return TextPosition{Index: count, Unit: pos.Unit, Affinity: AffinityUpstream}
	}
	return TextPosition{Index: pos.Index + 1, Unit: pos.Unit, Affinity: AffinityDownstream}
}

// PrevPosition moves back by one unit, clamping at the start.
func (l *TextLayout) PrevPosition(pos TextPosition) TextPosition {
	if pos.Unit == TextUnitGrapheme {
		if pos.Index <= 0 {
			return TextPosition{Index: 0, Unit: pos.Unit, Affinity: AffinityDownstream}
		}
		return TextPosition{Index: pos.Index - 1, Unit: pos.Unit, Affinity: AffinityUpstream}
	}
	if pos.Index <= 0 {
		return TextPosition{Index: 0, Unit: pos.Unit, Affinity: AffinityDownstream}
	}
	return TextPosition{Index: pos.Index - 1, Unit: pos.Unit, Affinity: AffinityUpstream}
}

// WordBoundaryAt returns the word containing pos.
func (l *TextLayout) WordBoundaryAt(pos TextPosition) TextRange {
	if l == nil || l.Source == "" {
		return TextRange{Start: pos.Index, End: pos.Index, Unit: pos.Unit}
	}
	runes := []rune(l.Source)
	if len(runes) == 0 {
		return TextRange{Start: pos.Index, End: pos.Index, Unit: pos.Unit}
	}
	i := l.RuneIndex(pos)
	words := wordRanges(runes)
	if len(words) == 0 {
		return TextRange{Start: i, End: i, Unit: pos.Unit}
	}
	for _, word := range words {
		if i >= word.Start && i < word.End {
			if pos.Unit == TextUnitGrapheme {
				start, end := l.graphemeRangeForRuneBounds(word.Start, word.End)
				return TextRange{Start: start, End: end, Unit: pos.Unit}
			}
			word.Unit = pos.Unit
			return word
		}
	}
	if i <= words[0].Start {
		if pos.Unit == TextUnitGrapheme {
			start, end := l.graphemeRangeForRuneBounds(words[0].Start, words[0].End)
			return TextRange{Start: start, End: end, Unit: pos.Unit}
		}
		words[0].Unit = pos.Unit
		return words[0]
	}
	for idx := 1; idx < len(words); idx++ {
		if i < words[idx].Start {
			if pos.Unit == TextUnitGrapheme {
				start, end := l.graphemeRangeForRuneBounds(words[idx-1].Start, words[idx-1].End)
				return TextRange{Start: start, End: end, Unit: pos.Unit}
			}
			words[idx-1].Unit = pos.Unit
			return words[idx-1]
		}
	}
	if pos.Unit == TextUnitGrapheme {
		start, end := l.graphemeRangeForRuneBounds(words[len(words)-1].Start, words[len(words)-1].End)
		return TextRange{Start: start, End: end, Unit: pos.Unit}
	}
	words[len(words)-1].Unit = pos.Unit
	return words[len(words)-1]
}

func (l *TextLayout) lineIndexForPoint(p Point) int {
	if len(l.Lines) == 0 {
		return 0
	}
	if p.Y <= l.Lines[0].Bounds.Min.Y {
		return 0
	}
	last := len(l.Lines) - 1
	if p.Y >= l.Lines[last].Bounds.Max.Y {
		return last
	}
	for i := range l.Lines {
		line := l.Lines[i]
		mid := (line.Bounds.Min.Y + line.Bounds.Max.Y) / 2
		if p.Y < mid {
			return i
		}
		if p.Y >= line.Bounds.Min.Y && p.Y < line.Bounds.Max.Y {
			return i
		}
	}
	return last
}

func (l *TextLayout) lineIndexForPosition(pos TextPosition) int {
	if len(l.Lines) == 0 {
		return 0
	}
	posIndex := l.RuneIndex(pos)
	if posIndex <= l.Lines[0].FirstRune {
		if posIndex == l.Lines[0].FirstRune && pos.Affinity == AffinityUpstream && len(l.Lines) > 1 {
			return 0
		}
		return 0
	}
	for i := range l.Lines {
		line := l.Lines[i]
		start := line.FirstRune
		end := line.FirstRune + line.RuneCount
		if posIndex < end {
			return i
		}
		if posIndex == end {
			if pos.Affinity == AffinityDownstream && i+1 < len(l.Lines) {
				return i + 1
			}
			return i
		}
		if posIndex >= start && posIndex < end {
			return i
		}
	}
	return len(l.Lines) - 1
}

func (l *TextLayout) positionX(pos TextPosition, lineIndex int) float32 {
	if l == nil || len(l.Lines) == 0 {
		return 0
	}
	lineIndex = clampInt(lineIndex, 0, len(l.Lines)-1)
	line := l.Lines[lineIndex]

	gpos := pos
	if pos.Unit != TextUnitGrapheme {
		gpos = l.positionFromRuneIndex(pos.Index, pos.Affinity)
	}

	lineFirstGrapheme := l.graphemeIndexForRuneIndex(line.FirstRune, AffinityDownstream)
	idx := gpos.Index - lineFirstGrapheme

	if idx <= 0 {
		return line.Bounds.Min.X
	}

	lineGraphemesCount := l.graphemeIndexForRuneIndex(line.FirstRune+line.RuneCount, AffinityUpstream) - lineFirstGrapheme
	if idx >= lineGraphemesCount {
		return line.Bounds.Max.X
	}

	if len(line.clusterMap) > 0 {
		if idx >= len(line.clusterMap) {
			return line.Bounds.Max.X
		}
		return line.clusterMap[idx]
	}

	return line.Bounds.Max.X
}

func (l *TextLayout) positionFromRuneIndex(index int, affinity Affinity) TextPosition {
	if l == nil {
		return GraphemePosition(0, affinity)
	}
	if len(l.graphemes) > 0 {
		return GraphemePosition(l.graphemeIndexForRuneIndex(index, affinity), affinity)
	}
	return RunePosition(clampInt(index, 0, l.RuneCount()), affinity)
}

func (l *TextLayout) graphemeIndexForRuneIndex(index int, affinity Affinity) int {
	if l == nil {
		return 0
	}
	index = clampInt(index, 0, l.RuneCount())
	if len(l.graphemes) == 0 {
		return index
	}
	last := len(l.graphemes) - 1
	if index <= l.graphemes[0] {
		return 0
	}
	if index >= l.graphemes[last] {
		return last
	}
	i := sort.Search(len(l.graphemes), func(i int) bool {
		return l.graphemes[i] >= index
	})
	if i < len(l.graphemes) && l.graphemes[i] == index {
		return i
	}
	if affinity == AffinityUpstream {
		return i - 1
	}
	return i
}

func (l *TextLayout) runeIndexForGraphemeIndex(index int) int {
	if l == nil {
		return 0
	}
	if len(l.graphemes) == 0 {
		return clampInt(index, 0, l.RuneCount())
	}
	return l.graphemes[clampInt(index, 0, len(l.graphemes)-1)]
}

func (l *TextLayout) graphemeRangeForRuneBounds(start, end int) (int, int) {
	if l == nil {
		return 0, 0
	}
	return l.graphemeIndexForRuneIndex(start, AffinityDownstream), l.graphemeIndexForRuneIndex(end, AffinityUpstream)
}

func graphemeBoundaries(runes []rune) []int {
	if len(runes) == 0 {
		return []int{0}
	}
	var seg textsegmenter.Segmenter
	seg.Init(runes)
	iter := seg.GraphemeIterator()
	out := make([]int, 0, len(runes)+1)
	out = append(out, 0)
	for iter.Next() {
		g := iter.Grapheme()
		end := g.Offset + len(g.Text)
		if end > out[len(out)-1] {
			out = append(out, end)
		}
	}
	if out[len(out)-1] != len(runes) {
		out = append(out, len(runes))
	}
	return out
}

// GraphemeCountString returns the number of grapheme clusters in s.
func GraphemeCountString(s string) int {
	return len(graphemeBoundaries([]rune(s))) - 1
}

// GraphemeRuneBoundsString converts a grapheme range into rune offsets for s.
func GraphemeRuneBoundsString(s string, r TextRange) (int, int) {
	if r.Unit != TextUnitGrapheme {
		r = r.Normalized()
		return r.Start, r.End
	}
	r = r.Normalized()
	bounds := graphemeBoundaries([]rune(s))
	start := clampInt(r.Start, 0, len(bounds)-1)
	end := clampInt(r.End, 0, len(bounds)-1)
	return bounds[start], bounds[end]
}
