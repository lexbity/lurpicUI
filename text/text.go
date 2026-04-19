package text

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"
	"unicode/utf8"

	_ "github.com/go-text/typesetting/font"
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
	Text  string
	Style TextStyle
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
	id     uint64
	source FontSource
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
	Glyphs  []PositionedGlyph
	Face    FontFace
	Size    float32
	Style   TextStyle
	Bounds  Rect
	Advance float32
	Text    string
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
type ShapedLine struct {
	Runs      []GlyphRun
	Bounds    Rect
	Baseline  float32
	FirstRune int
	RuneCount int
}

// TextLayout is the shaped and wrapped result of a paragraph.
type TextLayout struct {
	Lines      []ShapedLine
	Bounds     Rect
	LineHeight float32
	Baseline   float32
	source     string
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
	Affinity Affinity
}

// TextRange represents a half-open range of rune indices.
type TextRange struct {
	Start int
	End   int
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
	return TextRange{Start: r.End, End: r.Start}
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

// HitTest maps a point in layout-local space to the nearest text position.
func (l *TextLayout) HitTest(p Point) TextPosition {
	if l == nil || len(l.Lines) == 0 {
		return TextPosition{Index: 0, Affinity: AffinityDownstream}
	}
	lineIndex := l.lineIndexForPoint(p)
	line := &l.Lines[lineIndex]
	if len(line.Runs) == 0 {
		return TextPosition{Index: line.FirstRune, Affinity: AffinityDownstream}
	}
	if p.X <= line.Bounds.Min.X {
		return TextPosition{Index: line.FirstRune, Affinity: AffinityDownstream}
	}
	if p.X >= line.Bounds.Max.X {
		return TextPosition{Index: line.FirstRune + line.RuneCount, Affinity: AffinityUpstream}
	}
	for _, run := range line.Runs {
		if len(run.Glyphs) == 0 {
			continue
		}
		g := run.Glyphs[0]
		mid := run.Bounds.Min.X + g.Advance/2
		if p.X < mid {
			return TextPosition{Index: g.RuneIndex, Affinity: AffinityDownstream}
		}
		if p.X < run.Bounds.Max.X {
			return TextPosition{Index: g.RuneIndex + 1, Affinity: AffinityUpstream}
		}
	}
	return TextPosition{Index: line.FirstRune + line.RuneCount, Affinity: AffinityUpstream}
}

// CaretRect returns the cursor rectangle for a text position.
func (l *TextLayout) CaretRect(pos TextPosition) Rect {
	if l == nil || len(l.Lines) == 0 {
		return Rect{}
	}
	lineIndex := l.lineIndexForPosition(pos)
	line := &l.Lines[lineIndex]
	x := l.positionX(pos, lineIndex)
	return RectFromXYWH(x, line.Bounds.Min.Y, 2, l.LineHeight)
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
	var rects []Rect
	for i := range l.Lines {
		line := &l.Lines[i]
		start := maxInt(r.Start, line.FirstRune)
		end := minInt(r.End, line.FirstRune+line.RuneCount)
		if start >= end {
			continue
		}
		x0 := l.positionX(TextPosition{Index: start, Affinity: AffinityDownstream}, i)
		x1 := l.positionX(TextPosition{Index: end, Affinity: AffinityUpstream}, i)
		rects = append(rects, RectFromXYWH(x0, line.Bounds.Min.Y, x1-x0, l.LineHeight))
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
		return TextPosition{}
	}
	line = clampInt(line, 0, len(l.Lines)-1)
	return TextPosition{Index: l.Lines[line].FirstRune, Affinity: AffinityDownstream}
}

// PositionAtLineEnd returns the last cursor position on line i.
func (l *TextLayout) PositionAtLineEnd(line int) TextPosition {
	if l == nil || len(l.Lines) == 0 {
		return TextPosition{}
	}
	line = clampInt(line, 0, len(l.Lines)-1)
	ln := l.Lines[line]
	return TextPosition{Index: ln.FirstRune + ln.RuneCount, Affinity: AffinityUpstream}
}

// NextPosition advances by one rune, clamping at the end of the document.
func (l *TextLayout) NextPosition(pos TextPosition) TextPosition {
	count := l.RuneCount()
	if pos.Index >= count {
		return TextPosition{Index: count, Affinity: AffinityUpstream}
	}
	return TextPosition{Index: pos.Index + 1, Affinity: AffinityDownstream}
}

// PrevPosition moves back by one rune, clamping at the start.
func (l *TextLayout) PrevPosition(pos TextPosition) TextPosition {
	if pos.Index <= 0 {
		return TextPosition{Index: 0, Affinity: AffinityDownstream}
	}
	return TextPosition{Index: pos.Index - 1, Affinity: AffinityUpstream}
}

// WordBoundaryAt returns the word containing pos.
func (l *TextLayout) WordBoundaryAt(pos TextPosition) TextRange {
	if l == nil || l.source == "" {
		return TextRange{Start: pos.Index, End: pos.Index}
	}
	runes := []rune(l.source)
	if len(runes) == 0 {
		return TextRange{Start: pos.Index, End: pos.Index}
	}
	i := clampInt(pos.Index, 0, len(runes))
	if i == len(runes) {
		if i == 0 {
			return TextRange{Start: 0, End: 0}
		}
		i = len(runes) - 1
	}
	isBoundary := func(r rune) bool {
		return r == 0 || unicode.IsSpace(r) || unicode.IsPunct(r)
	}
	start := i
	if !isBoundary(runes[i]) {
		for start > 0 && !isBoundary(runes[start-1]) {
			start--
		}
		end := i + 1
		for end < len(runes) && !isBoundary(runes[end]) {
			end++
		}
		return TextRange{Start: start, End: end}
	}
	end := i + 1
	for end < len(runes) && isBoundary(runes[end]) {
		end++
	}
	return TextRange{Start: i, End: end}
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
	if pos.Index <= l.Lines[0].FirstRune {
		if pos.Index == l.Lines[0].FirstRune && pos.Affinity == AffinityUpstream && len(l.Lines) > 1 {
			return 0
		}
		return 0
	}
	for i := range l.Lines {
		line := l.Lines[i]
		start := line.FirstRune
		end := line.FirstRune + line.RuneCount
		if pos.Index < end {
			return i
		}
		if pos.Index == end {
			if pos.Affinity == AffinityDownstream && i+1 < len(l.Lines) {
				return i + 1
			}
			return i
		}
		if pos.Index >= start && pos.Index < end {
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
	if pos.Index <= line.FirstRune {
		if pos.Index == line.FirstRune && pos.Affinity == AffinityUpstream && lineIndex > 0 {
			prev := l.Lines[lineIndex-1]
			return prev.Bounds.Max.X
		}
		return line.Bounds.Min.X
	}
	endIndex := line.FirstRune + line.RuneCount
	if pos.Index >= endIndex {
		return line.Bounds.Max.X
	}
	for _, run := range line.Runs {
		if len(run.Glyphs) == 0 {
			continue
		}
		g := run.Glyphs[0]
		if pos.Index <= g.RuneIndex {
			return run.Bounds.Min.X
		}
		if pos.Index <= g.RuneIndex+1 {
			return run.Bounds.Min.X + run.Advance
		}
	}
	return line.Bounds.Max.X
}

// Shaper performs paragraph shaping and line wrapping.
type Shaper struct {
	registry *FontRegistry
}

// NewShaper constructs a shaper using the provided registry.
func NewShaper(registry *FontRegistry) *Shaper {
	return &Shaper{registry: registry}
}

// ShapeSimple shapes a single-span paragraph on one line.
func (s *Shaper) ShapeSimple(text string, style TextStyle) *TextLayout {
	return s.Shape(Paragraph{
		Spans: []TextSpan{{Text: text, Style: style}},
	})
}

// Shape converts a paragraph into a shaped layout.
func (s *Shaper) Shape(p Paragraph) *TextLayout {
	layout := &TextLayout{}
	if len(p.Spans) == 0 {
		return layout
	}

	maxWidth := p.MaxWidth
	var (
		lines      []ShapedLine
		current    shapedLineBuilder
		globalRune int
		source     strings.Builder
	)

	flushLine := func(force bool) {
		if !force && current.isEmpty() {
			return
		}
		line := current.finish()
		lines = append(lines, line)
		current.reset()
	}

	for _, span := range p.Spans {
		source.WriteString(span.Text)
		style := span.Style
		if style.Size <= 0 {
			style = DefaultStyle()
		}
		face := s.resolveFace(style)
		text := span.Text
		for len(text) > 0 {
			r, size := utf8.DecodeRuneInString(text)
			text = text[size:]
			if r == '\n' {
				flushLine(true)
				globalRune++
				continue
			}
			glyph := makeGlyph(r, style, globalRune)
			if maxWidth > 0 && !current.isEmpty() && current.width+glyph.Advance > maxWidth {
				flushLine(true)
			}
			current.appendGlyph(glyph, face, style, string(r))
			globalRune++
		}
	}
	flushLine(false)

	if len(lines) == 0 {
		return layout
	}

	lineHeight := maxLineHeight(lines)
	if lineHeight <= 0 {
		lineHeight = DefaultStyle().Size * 1.2
	}
	layout.LineHeight = lineHeight

	var y float32
	maxX := float32(0)
	totalRune := 0
	for i := range lines {
		line := &lines[i]
		line.RuneCount = countRunes(line)
		line.FirstRune = totalRune
		totalRune += line.RuneCount
		line.Baseline = y + lineHeight*0.8
		shiftLine(line, p.Alignment, maxWidth)
		line.Bounds.Min.Y = y
		line.Bounds.Max.Y = y + lineHeight
		if line.Bounds.Max.X > maxX {
			maxX = line.Bounds.Max.X
		}
		y += lineHeight
	}
	layout.Lines = lines
	if len(lines) > 0 {
		layout.Baseline = lines[0].Baseline
	}
	layout.source = source.String()
	layout.Bounds = RectFromXYWH(0, 0, maxX, y)
	return layout
}

func (s *Shaper) resolveFace(style TextStyle) FontFace {
	if s == nil || s.registry == nil {
		reg, _ := NewFontRegistry()
		return reg.Resolve(style)
	}
	return s.registry.Resolve(style)
}

type shapedLineBuilder struct {
	runs  []GlyphRun
	width float32
	size  float32
}

func (b *shapedLineBuilder) isEmpty() bool {
	return b == nil || (len(b.runs) == 0 && b.width == 0)
}

func (b *shapedLineBuilder) appendGlyph(g PositionedGlyph, face FontFace, style TextStyle, text string) {
	g.X = b.width
	run := GlyphRun{
		Glyphs:  []PositionedGlyph{g},
		Face:    face,
		Size:    style.Size,
		Style:   style,
		Bounds:  RectFromXYWH(g.X, 0, g.Advance, maxFloat32(style.Size*1.2, 1)),
		Advance: g.Advance,
		Text:    text,
	}
	b.runs = append(b.runs, run)
	b.width += g.Advance
	if style.Size > b.size {
		b.size = style.Size
	}
}

func (b *shapedLineBuilder) finish() ShapedLine {
	line := ShapedLine{
		Runs:   append([]GlyphRun(nil), b.runs...),
		Bounds: RectFromXYWH(0, 0, b.width, maxFloat32(b.size*1.2, 1)),
	}
	return line
}

func (b *shapedLineBuilder) reset() {
	if b == nil {
		return
	}
	b.runs = b.runs[:0]
	b.width = 0
	b.size = 0
}

func makeGlyph(r rune, style TextStyle, runeIndex int) PositionedGlyph {
	advance := glyphAdvance(r, style)
	return PositionedGlyph{
		GlyphID:   uint32(r),
		Advance:   advance,
		RuneIndex: runeIndex,
	}
}

func glyphAdvance(r rune, style TextStyle) float32 {
	size := style.Size
	if size <= 0 {
		size = DefaultStyle().Size
	}
	if size < 1 {
		size = 1
	}
	switch r {
	case '\t':
		tabWidth := style.TabWidth
		if tabWidth <= 0 {
			tabWidth = 4
		}
		return size * 0.33 * float32(tabWidth)
	case ' ':
		return size * 0.33
	case '\r':
		return 0
	default:
		return size * 0.6
	}
}

func countRunes(line *ShapedLine) int {
	total := 0
	if line == nil {
		return 0
	}
	for _, run := range line.Runs {
		total += len(run.Glyphs)
	}
	return total
}

func maxLineHeight(lines []ShapedLine) float32 {
	var maxH float32
	for _, line := range lines {
		h := line.Bounds.Height()
		if h > maxH {
			maxH = h
		}
	}
	return maxH
}

func shiftLine(line *ShapedLine, alignment TextAlignment, maxWidth float32) {
	if line == nil {
		return
	}
	shift := float32(0)
	if maxWidth > 0 {
		switch alignment {
		case AlignCenter:
			shift = (maxWidth - line.Bounds.Width()) / 2
		case AlignRight:
			shift = maxWidth - line.Bounds.Width()
		}
	}
	if shift == 0 {
		return
	}
	for i := range line.Runs {
		for j := range line.Runs[i].Glyphs {
			line.Runs[i].Glyphs[j].X += shift
		}
		line.Runs[i].Bounds.Min.X += shift
		line.Runs[i].Bounds.Max.X += shift
	}
	line.Bounds.Min.X += shift
	line.Bounds.Max.X += shift
}

func maxFloat32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func clampInt(v, min, max int) int {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// FontRegistry manages loaded font sources and face resolution.
type FontRegistry struct {
	mu       sync.RWMutex
	fallback *fontFaceRecord
	faces    []*fontFaceRecord
}

var fontFaceSeq atomic.Uint64

// NewFontRegistry creates a registry with a fallback face.
func NewFontRegistry() (*FontRegistry, error) {
	r := &FontRegistry{}
	r.fallback = &fontFaceRecord{id: fontFaceSeq.Add(1), source: FontSource{Name: "fallback", Data: []byte("fallback")}}
	r.faces = []*fontFaceRecord{r.fallback}
	return r, nil
}

// LoadFontFile loads a font from disk and stores it in the registry.
func (r *FontRegistry) LoadFontFile(path string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if path == "" {
		return errors.New("text: empty font path")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("text: load font %q: %w", filepath.Clean(path), err)
	}
	return r.LoadFontBytes(data, filepath.Base(path))
}

// LoadFontBytes loads a font from memory and stores it in the registry.
func (r *FontRegistry) LoadFontBytes(data []byte, name string) error {
	if r == nil {
		return errors.New("text: nil registry")
	}
	if len(data) == 0 {
		return errors.New("text: empty font data")
	}
	if strings.TrimSpace(name) == "" {
		return errors.New("text: font data requires name")
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	rec := &fontFaceRecord{id: fontFaceSeq.Add(1), source: FontSource{Name: name, Data: append([]byte(nil), data...)}}
	r.faces = append(r.faces, rec)
	return nil
}

// Sources returns a copy of the loaded font sources.
func (r *FontRegistry) Sources() []FontSource {
	if r == nil {
		return nil
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]FontSource, 0, len(r.faces))
	for _, face := range r.faces {
		if face == nil {
			continue
		}
		out = append(out, face.source)
	}
	return out
}

// Resolve finds the best available face for the given style.
func (r *FontRegistry) Resolve(style TextStyle) FontFace {
	if r == nil {
		return FontFace{}
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	if face := r.resolveLocked(style); face != nil {
		return FontFace{face: face}
	}
	return FontFace{face: r.fallback}
}

func (r *FontRegistry) resolveLocked(style TextStyle) *fontFaceRecord {
	if r == nil {
		return nil
	}
	var familyMatch *fontFaceRecord
	var styleMatch *fontFaceRecord
	for _, face := range r.faces {
		if face == nil {
			continue
		}
		if style.Family != "" && strings.EqualFold(face.source.Name, style.Family) {
			familyMatch = face
			if style.Weight >= WeightBold && strings.Contains(strings.ToLower(face.source.Name), "bold") {
				return face
			}
			if style.Style == StyleItalic && strings.Contains(strings.ToLower(face.source.Name), "italic") {
				return face
			}
			styleMatch = face
		}
	}
	if familyMatch != nil {
		return familyMatch
	}
	if styleMatch != nil {
		return styleMatch
	}
	return r.fallback
}

// IsZero reports whether the face wraps an unresolved record.
func (f FontFace) IsZero() bool {
	return f.face == nil
}

// CacheKey returns a stable identifier suitable for glyph atlas keys.
func (f FontFace) CacheKey() uint64 {
	if f.face == nil {
		return 0
	}
	return f.face.id
}
