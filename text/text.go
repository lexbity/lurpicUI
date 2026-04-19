package text

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/language"
	textsegmenter "github.com/go-text/typesetting/segmenter"
	gotextshaping "github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
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
	Runs       []GlyphRun
	Bounds     Rect
	Baseline   float32
	FirstRune  int
	RuneCount  int
	clusterMap []float32
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
	if idx, ok := line.hitTestClusterIndex(p.X - line.Bounds.Min.X); ok {
		return TextPosition{Index: line.FirstRune + idx, Affinity: AffinityUpstream}
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
	words := wordRanges(runes)
	if len(words) == 0 {
		return TextRange{Start: i, End: i}
	}
	for _, word := range words {
		if i >= word.Start && i < word.End {
			return word
		}
	}
	if i <= words[0].Start {
		return words[0]
	}
	for idx := 1; idx < len(words); idx++ {
		if i < words[idx].Start {
			return words[idx-1]
		}
	}
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
	if len(line.clusterMap) > 0 {
		idx := pos.Index - line.FirstRune
		if idx < 0 {
			idx = 0
		}
		if idx >= len(line.clusterMap) {
			return line.Bounds.Max.X
		}
		return line.Bounds.Min.X + line.clusterMap[idx]
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
	registry     *FontRegistry
	shaper       gotextshaping.HarfbuzzShaper
	segmenter    gotextshaping.Segmenter
	contentScale float32
}

// NewShaper constructs a shaper using the provided registry.
func NewShaper(registry *FontRegistry) *Shaper {
	return &Shaper{registry: registry, contentScale: 1}
}

// SetContentScale adjusts the pixels-per-em scale used while shaping.
func (s *Shaper) SetContentScale(scale float32) {
	if s == nil || scale <= 0 {
		return
	}
	s.contentScale = scale
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
	if s == nil || len(p.Spans) == 0 {
		return layout
	}

	availableFaces := s.registryFaces()
	if len(availableFaces) == 0 {
		return layout
	}

	var (
		lines                 []ShapedLine
		b                     lineBuilder
		source                strings.Builder
		totalRune             int
		alignment             = p.Alignment
		maxWidth              = p.MaxWidth
		defaultLineH          = DefaultStyle().Size * 1.2
		currentLineH          = defaultLineH
		pendingBlankLine      bool
		trimLeadingWhitespace bool
	)

	flushLine := func(force bool) {
		if !force && !b.hasContent() {
			return
		}
		if !b.hasContent() && len(lines) == 0 {
			currentLineH = maxFloat32(currentLineH, defaultLineH)
		}
		line := b.finish(totalRune, currentLineH, alignment, maxWidth)
		lines = append(lines, line)
		totalRune += line.RuneCount
		b.reset()
		currentLineH = defaultLineH
	}

	addSoftSegment := func(seg softSegment, style TextStyle, availableFaces []*font.Face) {
		if len(seg.Text) == 0 {
			return
		}
		if trimLeadingWhitespace {
			seg = trimSoftSegment(seg)
			if len(seg.Text) == 0 {
				return
			}
		}
		segments := s.segmentRunes(seg.Text, availableFaces, style)
		if len(segments) == 0 {
			return
		}
		if pendingBlankLine {
			pendingBlankLine = false
		}
		segmentWidth := float32(0)
		for _, sub := range segments {
			if sub.Face.IsZero() || len(sub.Text) == 0 {
				continue
			}
			out := s.shapeSegment(sub, style)
			if out == nil || len(out.Glyphs) == 0 {
				continue
			}
			segmentWidth += float32(out.Advance) / 64
		}
		if maxWidth > 0 && b.hasContent() && b.width+segmentWidth > maxWidth {
			flushLine(true)
			trimLeadingWhitespace = true
			seg = trimSoftSegment(seg)
			if len(seg.Text) == 0 {
				return
			}
			segments = s.segmentRunes(seg.Text, availableFaces, style)
			if len(segments) == 0 {
				return
			}
		}
		for _, sub := range segments {
			if sub.Face.IsZero() || len(sub.Text) == 0 {
				continue
			}
			out := s.shapeSegment(sub, style)
			if out == nil || len(out.Glyphs) == 0 {
				continue
			}
			segmentLineH := float32(out.LineBounds.LineThickness()) / 64
			if segmentLineH <= 0 {
				segmentLineH = maxFloat32(style.Size*1.2, 1)
			}
			if segmentLineH > currentLineH {
				currentLineH = segmentLineH
			}
			for i := 0; i < len(out.Glyphs); {
				j := i + 1
				for j < len(out.Glyphs) && out.Glyphs[j].ClusterIndex == out.Glyphs[i].ClusterIndex {
					j++
				}
				cluster := out.Glyphs[i:j]
				b.addCluster(sub, style, cluster)
				i = j
			}
		}
		trimLeadingWhitespace = false
	}

	for _, span := range p.Spans {
		source.WriteString(span.Text)
		style := span.Style
		if style.Size <= 0 {
			style = DefaultStyle()
		}
		runes := []rune(span.Text)
		if len(runes) == 0 {
			continue
		}
		for i := 0; i < len(runes); {
			if runes[i] == '\r' || runes[i] == '\n' {
				flushLine(true)
				trimLeadingWhitespace = true
				pendingBlankLine = true
				if runes[i] == '\r' && i+1 < len(runes) && runes[i+1] == '\n' {
					i += 2
				} else {
					i++
				}
				continue
			}
			j := i
			for j < len(runes) && runes[j] != '\r' && runes[j] != '\n' {
				j++
			}
			segments := s.breakSoftSegments(runes[i:j])
			for _, seg := range segments {
				addSoftSegment(seg, style, availableFaces)
			}
			i = j
		}
	}

	if pendingBlankLine {
		flushLine(true)
		pendingBlankLine = false
	}

	if b.hasContent() {
		flushLine(true)
	}

	if len(lines) == 0 {
		return layout
	}

	layout.Lines = lines
	layout.LineHeight = lineHeightFromLines(lines)
	layout.Baseline = lines[0].Baseline
	layout.source = source.String()
	layout.Bounds = RectFromXYWH(0, 0, lineMaxWidth(lines), layout.LineHeight*float32(len(lines)))
	if len(lines) == 1 && lines[0].RuneCount == 0 {
		layout.LineHeight = defaultLineH
		layout.Bounds = RectFromXYWH(0, 0, 0, layout.LineHeight)
	}
	return layout
}

func (s *Shaper) shapeSegment(seg resolvedSegment, style TextStyle) *gotextshaping.Output {
	if s == nil {
		return nil
	}
	in := seg.Run
	size := style.Size
	if size <= 0 {
		size = DefaultStyle().Size
	}
	scale := s.contentScale
	if scale <= 0 {
		scale = 1
	}
	in.Size = fixed.I(int(math.Round(float64(size * scale))))
	out := s.shaper.Shape(in)
	return &out
}

func (s *Shaper) registryFaces() []*font.Face {
	if s == nil || s.registry == nil {
		return nil
	}
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()
	faces := make([]*font.Face, 0, len(s.registry.faces))
	for _, rec := range s.registry.faces {
		if rec == nil || rec.face == nil {
			continue
		}
		faces = append(faces, rec.face)
	}
	return faces
}

type resolvedSegment struct {
	Text string
	Face FontFace
	Run  gotextshaping.Input
}

func (s *Shaper) segmentRunes(runes []rune, availableFaces []*font.Face, style TextStyle) []resolvedSegment {
	if len(runes) == 0 || len(availableFaces) == 0 {
		return nil
	}
	input := gotextshaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: di.DirectionLTR,
		Script:    language.Common,
	}
	segs := s.segmenter.Split(input, registryFontMap{faces: availableFaces})
	if len(segs) == 0 {
		return nil
	}
	out := make([]resolvedSegment, 0, len(segs))
	for _, seg := range segs {
		if seg.Face == nil {
			continue
		}
		selected := s.registry.Resolve(style)
		if selected.IsZero() {
			selected = s.fontFaceFor(seg.Face)
		}
		if selected.IsZero() {
			continue
		}
		run := seg
		run.Face = selected.face.face
		out = append(out, resolvedSegment{
			Text: string(runes[seg.RunStart:seg.RunEnd]),
			Face: selected,
			Run:  run,
		})
	}
	return out
}

func (s *Shaper) fontFaceFor(face *font.Face) FontFace {
	if s == nil || s.registry == nil || face == nil {
		return FontFace{}
	}
	s.registry.mu.RLock()
	defer s.registry.mu.RUnlock()
	for _, rec := range s.registry.faces {
		if rec != nil && rec.face == face {
			return FontFace{face: rec}
		}
	}
	return FontFace{}
}

type softSegment struct {
	Text []rune
}

type lineBreaker struct {
	seg textsegmenter.Segmenter
}

func (lb *lineBreaker) breakOpportunities(text []rune) []int {
	if lb == nil || len(text) == 0 {
		return nil
	}
	lb.seg.Init(text)
	iter := lb.seg.LineIterator()
	out := make([]int, 0, len(text))
	for iter.Next() {
		line := iter.Line()
		out = append(out, line.Offset+len(line.Text))
	}
	if len(out) == 0 || out[len(out)-1] != len(text) {
		out = append(out, len(text))
	}
	return out
}

func (s *Shaper) breakSoftSegments(text []rune) []softSegment {
	if len(text) == 0 {
		return nil
	}
	lb := lineBreaker{}
	breaks := lb.breakOpportunities(text)
	if len(breaks) == 0 {
		return []softSegment{{Text: append([]rune(nil), text...)}}
	}
	out := make([]softSegment, 0, len(breaks))
	start := 0
	for _, end := range breaks {
		if end < start {
			continue
		}
		if end > len(text) {
			end = len(text)
		}
		out = append(out, softSegment{Text: append([]rune(nil), text[start:end]...)})
		start = end
	}
	if start < len(text) {
		out = append(out, softSegment{Text: append([]rune(nil), text[start:]...)})
	}
	return out
}

func trimSoftSegment(seg softSegment) softSegment {
	if len(seg.Text) == 0 {
		return seg
	}
	i := 0
	for i < len(seg.Text) && unicode.IsSpace(seg.Text[i]) {
		i++
	}
	if i == 0 {
		return seg
	}
	return softSegment{Text: append([]rune(nil), seg.Text[i:]...)}
}

type registryFontMap struct {
	faces []*font.Face
}

func (m registryFontMap) ResolveFace(r rune) *font.Face {
	if len(m.faces) == 0 {
		return nil
	}
	for _, face := range m.faces {
		if face == nil {
			continue
		}
		if _, has := face.NominalGlyph(r); has {
			return face
		}
	}
	return m.faces[0]
}

type lineBuilder struct {
	runs       []GlyphRun
	cur        *runBuilder
	width      float32
	runeCount  int
	clusterMap []float32
}

type runBuilder struct {
	face     FontFace
	style    TextStyle
	text     string
	glyphs   []PositionedGlyph
	startX   float32
	advance  float32
	runeText int
}

func (b *lineBuilder) hasContent() bool {
	return b != nil && (b.cur != nil || len(b.runs) > 0 || b.width > 0 || b.runeCount > 0)
}

func (b *lineBuilder) reset() {
	if b == nil {
		return
	}
	b.runs = b.runs[:0]
	b.cur = nil
	b.width = 0
	b.runeCount = 0
	b.clusterMap = b.clusterMap[:0]
}

func (b *lineBuilder) ensureRun(seg resolvedSegment, style TextStyle) {
	if b == nil {
		return
	}
	if b.cur != nil && b.cur.face == seg.Face && sameStyle(b.cur.style, style) {
		return
	}
	b.flushRun()
	b.cur = &runBuilder{
		face:   seg.Face,
		style:  style,
		text:   seg.Text,
		startX: b.width,
	}
}

func (b *lineBuilder) addCluster(seg resolvedSegment, style TextStyle, glyphs []gotextshaping.Glyph) {
	if b == nil {
		return
	}
	b.ensureRun(seg, style)
	if b.cur == nil {
		return
	}
	if len(glyphs) == 0 {
		return
	}
	clusterStart := b.runeCount
	clusterRuneCount := glyphs[0].RuneCount
	clusterAdvance := float32(0)
	for _, g := range glyphs {
		clusterAdvance += float32(g.XAdvance) / 64
	}
	startX := b.cur.advance
	for _, g := range glyphs {
		adv := float32(g.XAdvance) / 64
		glyph := PositionedGlyph{
			GlyphID:   uint32(g.GlyphID),
			Advance:   adv,
			RuneIndex: clusterStart,
			X:         b.cur.advance,
			Y:         float32(g.YOffset) / 64,
		}
		b.cur.glyphs = append(b.cur.glyphs, glyph)
		b.cur.advance += adv
	}
	if clusterRuneCount > 0 {
		if len(b.clusterMap) == 0 {
			b.clusterMap = append(b.clusterMap, 0)
		}
		for i := 0; i < clusterRuneCount; i++ {
			b.clusterMap = append(b.clusterMap, startX+clusterAdvance*float32(i+1)/float32(clusterRuneCount))
		}
		b.runeCount += clusterRuneCount
	}
	b.width = b.cur.startX + b.cur.advance
}

func (b *lineBuilder) flushRun() {
	if b == nil || b.cur == nil {
		return
	}
	run := GlyphRun{
		Glyphs:  append([]PositionedGlyph(nil), b.cur.glyphs...),
		Face:    b.cur.face,
		Size:    b.cur.style.Size,
		Style:   b.cur.style,
		Bounds:  RectFromXYWH(b.cur.startX, 0, b.cur.advance, maxFloat32(b.cur.style.Size*1.2, 1)),
		Advance: b.cur.advance,
		Text:    b.cur.text,
	}
	b.runs = append(b.runs, run)
	b.cur = nil
}

func (b *lineBuilder) finish(firstRune int, lineHeight float32, alignment TextAlignment, maxWidth float32) ShapedLine {
	if b == nil {
		return ShapedLine{}
	}
	b.flushRun()
	runs := append([]GlyphRun(nil), b.runs...)
	line := ShapedLine{
		Runs:       runs,
		Bounds:     RectFromXYWH(0, 0, b.width, maxFloat32(lineHeight, 1)),
		Baseline:   maxFloat32(lineHeight*0.8, 1),
		FirstRune:  firstRune,
		RuneCount:  b.runeCount,
		clusterMap: append([]float32(nil), b.clusterMap...),
	}
	if len(line.clusterMap) == 0 {
		line.clusterMap = []float32{0}
	}
	shiftLine(&line, alignment, maxWidth)
	return line
}

func (l *ShapedLine) hitTestClusterIndex(x float32) (int, bool) {
	if l == nil || len(l.clusterMap) < 2 {
		return 0, false
	}
	if x <= 0 {
		return 0, true
	}
	last := len(l.clusterMap) - 1
	if x >= l.clusterMap[last] {
		return last, true
	}
	idx := sort.Search(len(l.clusterMap), func(i int) bool {
		return l.clusterMap[i] >= x
	})
	if idx <= 0 {
		return 0, true
	}
	if idx >= len(l.clusterMap) {
		return last, true
	}
	left := l.clusterMap[idx-1]
	right := l.clusterMap[idx]
	if x-left <= right-x {
		return idx - 1, true
	}
	return idx, true
}

func wordRanges(runes []rune) []TextRange {
	if len(runes) == 0 {
		return nil
	}
	var seg textsegmenter.Segmenter
	seg.Init(runes)
	iter := seg.WordIterator()
	var out []TextRange
	for iter.Next() {
		word := iter.Word()
		start := word.Offset
		end := word.Offset + len(word.Text)
		if start < end {
			out = append(out, TextRange{Start: start, End: end})
		}
	}
	return out
}

func sameStyle(a, b TextStyle) bool {
	return a.Family == b.Family &&
		a.Size == b.Size &&
		a.Weight == b.Weight &&
		a.Style == b.Style &&
		a.LineHeight == b.LineHeight &&
		a.LetterSpacing == b.LetterSpacing &&
		a.TabWidth == b.TabWidth
}

func lineHeightFromLines(lines []ShapedLine) float32 {
	var maxH float32
	for _, line := range lines {
		if h := line.Bounds.Height(); h > maxH {
			maxH = h
		}
	}
	if maxH <= 0 {
		return DefaultStyle().Size * 1.2
	}
	return maxH
}

func lineMaxWidth(lines []ShapedLine) float32 {
	var maxW float32
	for _, line := range lines {
		if w := line.Bounds.Width(); w > maxW {
			maxW = w
		}
	}
	return maxW
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
	mu    sync.RWMutex
	faces []*fontFaceRecord
}

// NewFontRegistry creates an empty registry.
func NewFontRegistry() (*FontRegistry, error) {
	return &FontRegistry{}, nil
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
	faces, err := font.ParseTTC(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("text: parse font %q: %w", name, err)
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	for i, face := range faces {
		if face == nil || face.Font == nil {
			continue
		}
		rec := &fontFaceRecord{
			face:     face,
			desc:     face.Font.Describe(),
			source:   FontSource{Name: name, Data: append([]byte(nil), data...)},
			cacheKey: computeFontCacheKey(data, i),
		}
		r.faces = append(r.faces, rec)
	}
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
	return FontFace{}
}

func (r *FontRegistry) resolveLocked(style TextStyle) *fontFaceRecord {
	if r == nil {
		return nil
	}
	targetFamily := font.NormalizeFamily(style.Family)
	if targetFamily == "" {
		return nil
	}
	var (
		best      *fontFaceRecord
		bestScore int
	)
	for _, face := range r.faces {
		if face == nil || face.face == nil {
			continue
		}
		if font.NormalizeFamily(face.desc.Family) != targetFamily {
			continue
		}
		score := faceMatchScore(face.desc.Aspect, style)
		if best == nil || score < bestScore {
			best = face
			bestScore = score
		}
	}
	return best
}

// IsZero reports whether the face wraps an unresolved record.
func (f FontFace) IsZero() bool {
	return f.face == nil
}

// GoFace returns the underlying go-text font face.
func (f FontFace) GoFace() *font.Face {
	if f.face == nil {
		return nil
	}
	return f.face.face
}

// CacheKey returns a stable identifier suitable for glyph atlas keys.
func (f FontFace) CacheKey() uint64 {
	if f.face == nil {
		return 0
	}
	return f.face.cacheKey
}

func computeFontCacheKey(data []byte, index int) uint64 {
	sum := sha256.Sum256(append(append([]byte(nil), data...), byte(index>>24), byte(index>>16), byte(index>>8), byte(index)))
	return binaryToUint64(sum[:8])
}

func binaryToUint64(b []byte) uint64 {
	if len(b) < 8 {
		return 0
	}
	return uint64(b[0])<<56 | uint64(b[1])<<48 | uint64(b[2])<<40 | uint64(b[3])<<32 |
		uint64(b[4])<<24 | uint64(b[5])<<16 | uint64(b[6])<<8 | uint64(b[7])
}

func faceMatchScore(aspect font.Aspect, style TextStyle) int {
	score := 0
	wantStyle := font.StyleNormal
	if style.Style != StyleNormal {
		wantStyle = font.StyleItalic
	}
	if aspect.Style != wantStyle {
		score += 1000
	}
	wantWeight := font.Weight(style.Weight)
	if wantWeight == 0 {
		wantWeight = font.WeightNormal
	}
	diff := aspect.Weight - wantWeight
	if diff < 0 {
		diff = -diff
	}
	score += int(diff)
	return score
}
