package text

import (
	"math"

	"sort"

	"strings"

	"unicode"

	"github.com/go-text/typesetting/di"

	"github.com/go-text/typesetting/font"

	"github.com/go-text/typesetting/language"

	textsegmenter "github.com/go-text/typesetting/segmenter"

	gotextshaping "github.com/go-text/typesetting/shaping"

	"golang.org/x/image/math/fixed"

	xtextbidi "golang.org/x/text/unicode/bidi"
)

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

// ShapeTruncated shapes the content, truncating it to fit within maxWidth by appending an ellipsis ("…")
// at grapheme boundaries if it exceeds maxWidth.
func (s *Shaper) ShapeTruncated(content string, style TextStyle, maxWidth float32) *TextLayout {
	layout := s.ShapeSimple(content, style)
	if layout == nil || maxWidth <= 0 || layout.Bounds.Width() <= maxWidth {
		return layout
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return layout
	}
	bounds := graphemeBoundaries(runes)
	numGraphemes := len(bounds) - 1
	if numGraphemes <= 0 {
		return layout
	}

	// Try with ellipsis first
	ellipsis := s.ShapeSimple("…", style)
	if ellipsis != nil && ellipsis.Bounds.Width() <= maxWidth {
		best := 0
		lo, hi := 0, numGraphemes
		for lo <= hi {
			mid := (lo + hi) / 2
			prefix := string(runes[:bounds[mid]])
			candidate := s.ShapeSimple(prefix+"…", style)
			if candidate != nil && candidate.Bounds.Width() <= maxWidth {
				best = mid
				lo = mid + 1
			} else {
				hi = mid - 1
			}
		}
		if best > 0 {
			return s.ShapeSimple(string(runes[:bounds[best]])+"…", style)
		}
		return ellipsis
	}

	// Fallback to no ellipsis if ellipsis itself doesn't fit
	best := 0
	lo, hi := 0, numGraphemes
	for lo <= hi {
		mid := (lo + hi) / 2
		prefix := string(runes[:bounds[mid]])
		candidate := s.ShapeSimple(prefix, style)
		if candidate != nil && candidate.Bounds.Width() <= maxWidth {
			best = mid
			lo = mid + 1
		} else {
			hi = mid - 1
		}
	}
	if best > 0 {
		return s.ShapeSimple(string(runes[:bounds[best]]), style)
	}
	return s.ShapeSimple("", style)
}

func (s *Shaper) spaceWidth(face FontFace, style TextStyle) float32 {
	if face.IsZero() {
		return style.Size * 0.25
	}
	goFace := face.GoFace()
	if goFace == nil {
		return style.Size * 0.25
	}
	seg := resolvedSegment{
		Text: " ",
		Face: face,
		Run: gotextshaping.Input{
			Face:      goFace,
			Text:      []rune{' '},
			RunStart:  0,
			RunEnd:    1,
			Direction: di.DirectionLTR,
		},
	}
	out := s.shapeSegment(seg, style)
	if out == nil || len(out.Glyphs) == 0 {
		return style.Size * 0.25
	}
	return float32(out.Glyphs[0].XAdvance) / 64
}

// Shape converts a paragraph into a shaped layout.
func (s *Shaper) Shape(p Paragraph) *TextLayout {
	layout := &TextLayout{Paragraph: p}
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
	b.baseDirection = resolveBidiDirection(p.Direction, di.DirectionLTR, paragraphTextRunes(p.Spans))
	b.direction = b.baseDirection
	b.shaper = s

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
		b.shaper = s
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
		segments := s.segmentRunes(seg.Text, availableFaces, style, seg.direction, seg.Level, p, seg.span)
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
			segments = s.segmentRunes(seg.Text, availableFaces, style, seg.direction, seg.Level, p, seg.span)
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
			var lh float32
			if style.LineHeight > 0 {
				if style.LineHeight < 5 {
					lh = style.LineHeight * style.Size
				} else {
					lh = style.LineHeight
				}
			} else {
				lh = float32(out.LineBounds.LineThickness()) / 64
				if lh <= 0 {
					lh = style.Size * 1.2
				}
			}
			segmentLineH := maxFloat32(lh, 1)
			segmentAscent := float32(out.LineBounds.Ascent) / 64
			segmentDescent := float32(out.LineBounds.Descent) / 64
			if segmentLineH > currentLineH {
				currentLineH = segmentLineH
			}
			for i := 0; i < len(out.Glyphs); {
				j := i + 1
				for j < len(out.Glyphs) && out.Glyphs[j].ClusterIndex == out.Glyphs[i].ClusterIndex {
					j++
				}
				cluster := out.Glyphs[i:j]
				b.addCluster(sub, style, cluster, segmentAscent, segmentDescent)
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
			for _, run := range s.logicalRuns(runes[i:j], p, span) {
				segments := s.breakSoftSegments(run.Text)
				for _, seg := range segments {
					seg.span = span
					seg.direction = run.Direction
					seg.Level = run.Level
					addSoftSegment(seg, style, availableFaces)
				}
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
	layout.Metrics = aggregateMetrics(lines)
	layout.Source = source.String()
	layout.source = layout.Source
	layout.graphemes = graphemeBoundaries([]rune(layout.Source))
	layout.Bounds = RectFromXYWH(0, 0, lineMaxWidth(lines), totalLineHeight(lines))
	if len(lines) == 1 && lines[0].RuneCount == 0 {
		layout.LineHeight = defaultLineH
		layout.Metrics.LineHeight = defaultLineH
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
	Text      string
	Face      FontFace
	Run       gotextshaping.Input
	Direction di.Direction
	Level     int
	Language  language.Language
	Script    language.Script
}

type visualRun struct {
	Text      []rune
	Direction di.Direction
}

func (s *Shaper) visualRuns(text []rune, paragraph Paragraph, span TextSpan) []visualRun {
	if len(text) == 0 {
		return nil
	}
	baseDirection := resolveBidiDirection(paragraph.Direction, span.Direction, text)
	var para xtextbidi.Paragraph
	if _, err := para.SetString(string(text), xtextbidi.DefaultDirection(toBidiDirection(baseDirection))); err != nil {
		return []visualRun{{Text: append([]rune(nil), text...), Direction: baseDirection}}
	}
	ordering, err := para.Order()
	if err != nil || ordering.NumRuns() == 0 {
		return []visualRun{{Text: append([]rune(nil), text...), Direction: baseDirection}}
	}
	out := make([]visualRun, 0, ordering.NumRuns())
	for i := 0; i < ordering.NumRuns(); i++ {
		run := ordering.Run(i)
		dir := fromBidiDirection(run.Direction())
		out = append(out, visualRun{
			Text:      []rune(run.String()),
			Direction: dir,
		})
	}
	return out
}

type logicalRun struct {
	Text      []rune
	Direction di.Direction
	Level     int
}

func (s *Shaper) logicalRuns(text []rune, paragraph Paragraph, span TextSpan) []logicalRun {
	if len(text) == 0 {
		return nil
	}
	baseDirection := resolveBidiDirection(paragraph.Direction, span.Direction, text)
	var para xtextbidi.Paragraph
	if _, err := para.SetString(string(text), xtextbidi.DefaultDirection(toBidiDirection(baseDirection))); err != nil {
		level := 0
		if baseDirection == di.DirectionRTL {
			level = 1
		}
		return []logicalRun{{Text: append([]rune(nil), text...), Direction: baseDirection, Level: level}}
	}
	ordering, err := para.Order()
	if err != nil || ordering.NumRuns() == 0 {
		level := 0
		if baseDirection == di.DirectionRTL {
			level = 1
		}
		return []logicalRun{{Text: append([]rune(nil), text...), Direction: baseDirection, Level: level}}
	}

	type runWithPos struct {
		run   logicalRun
		start int
	}
	runsWithPos := make([]runWithPos, 0, ordering.NumRuns())
	for i := 0; i < ordering.NumRuns(); i++ {
		r := ordering.Run(i)
		start, _ := r.Pos()
		dir := fromBidiDirection(r.Direction())
		level := 0
		if baseDirection == di.DirectionLTR {
			if dir == di.DirectionRTL {
				level = 1
			} else {
				level = 0
			}
		} else { // RTL paragraph
			if dir == di.DirectionRTL {
				level = 1
			} else {
				level = 2
			}
		}
		runsWithPos = append(runsWithPos, runWithPos{
			run: logicalRun{
				Text:      []rune(r.String()),
				Direction: dir,
				Level:     level,
			},
			start: start,
		})
	}

	sort.Slice(runsWithPos, func(i, j int) bool {
		return runsWithPos[i].start < runsWithPos[j].start
	})

	out := make([]logicalRun, len(runsWithPos))
	for i, rp := range runsWithPos {
		out[i] = rp.run
	}
	return out
}

func (s *Shaper) segmentRunes(runes []rune, availableFaces []*font.Face, style TextStyle, direction di.Direction, level int, paragraph Paragraph, span TextSpan) []resolvedSegment {
	if len(runes) == 0 || len(availableFaces) == 0 {
		return nil
	}
	script := resolveScriptHint(paragraph.Script, span.Script)
	input := gotextshaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: direction,
		Script:    script,
		Language:  resolveLanguage(paragraph.Language, span.Language),
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
		} else if selected.GoFace() != nil && !faceCoversText(selected.GoFace(), runes[seg.RunStart:seg.RunEnd]) {
			// Keep fallback segments on the face selected by the segmenter only
			// when the styled face cannot cover the segment's runes.
			if fallback := s.fontFaceFor(seg.Face); !fallback.IsZero() {
				selected = fallback
			}
		}
		if selected.IsZero() {
			continue
		}
		run := seg
		run.Face = selected.face.face
		out = append(out, resolvedSegment{
			Text:      string(runes[seg.RunStart:seg.RunEnd]),
			Face:      selected,
			Run:       run,
			Direction: run.Direction,
			Level:     level,
			Language:  run.Language,
			Script:    run.Script,
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
	Text      []rune
	span      TextSpan
	direction di.Direction
	Level     int
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
	return softSegment{Text: append([]rune(nil), seg.Text[i:]...), span: seg.span}
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
	runs          []GlyphRun
	cur           *runBuilder
	width         float32
	runeCount     int
	clusterMap    []float32
	ascent        float32
	descent       float32
	metrics       LineMetrics
	direction     di.Direction
	baseDirection di.Direction
	language      language.Language
	script        language.Script
	metadataSet   bool
	shaper        *Shaper
}

type runBuilder struct {
	face             FontFace
	style            TextStyle
	text             string
	glyphs           []PositionedGlyph
	startX           float32
	advance          float32
	runeText         int
	direction        di.Direction
	level            int
	graphemeAdvances []float32
	language         language.Language
	script           language.Script
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
	b.ascent = 0
	b.descent = 0
	b.metrics = LineMetrics{}
	b.direction = b.baseDirection
	b.language = ""
	b.script = language.Common
	b.metadataSet = false
	b.shaper = nil
}

func (b *lineBuilder) ensureRun(seg resolvedSegment, style TextStyle) {
	if b == nil {
		return
	}
	if b.cur != nil && b.cur.face == seg.Face && sameStyle(b.cur.style, style) &&
		b.cur.direction == seg.Direction && b.cur.level == seg.Level && b.cur.language == seg.Language && b.cur.script == seg.Script {
		return
	}
	b.flushRun()
	b.cur = &runBuilder{
		face:      seg.Face,
		style:     style,
		text:      seg.Text,
		startX:    b.width,
		direction: seg.Direction,
		level:     seg.Level,
		language:  seg.Language,
		script:    seg.Script,
	}
}

func (b *lineBuilder) addCluster(seg resolvedSegment, style TextStyle, glyphs []gotextshaping.Glyph, ascent, descent float32) {
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
	if !b.metadataSet {
		b.language = seg.Language
		b.script = seg.Script
		b.metadataSet = true
	}
	clusterStart := b.runeCount
	clusterRuneCount := glyphs[0].RuneCount

	// Tab and LetterSpacing support:
	isTab := false
	if len(glyphs) > 0 {
		runStart := seg.Run.RunStart
		idx := runStart + glyphs[0].ClusterIndex
		if idx >= 0 && idx < len(seg.Run.Text) && seg.Run.Text[idx] == '\t' {
			isTab = true
		}
	}

	tabWidth := style.TabWidth
	if tabWidth <= 0 {
		tabWidth = 4
	}
	var tabAdvance float32
	if isTab && b.shaper != nil {
		spaceWidth := b.shaper.spaceWidth(seg.Face, style)
		tabStopWidth := float32(tabWidth) * spaceWidth
		currentX := b.width
		nextTabStopX := float32(math.Floor(float64(currentX/tabStopWidth))+1) * tabStopWidth
		tabAdvance = nextTabStopX - currentX
	}

	clusterAdvance := float32(0)
	if isTab {
		clusterAdvance = tabAdvance
	} else {
		for _, g := range glyphs {
			clusterAdvance += float32(g.XAdvance) / 64
		}
		clusterAdvance += style.LetterSpacing
	}

	b.cur.graphemeAdvances = append(b.cur.graphemeAdvances, clusterAdvance)

	for idx, g := range glyphs {
		adv := float32(g.XAdvance) / 64
		if isTab {
			adv = tabAdvance / float32(len(glyphs))
		} else if idx == len(glyphs)-1 {
			adv += style.LetterSpacing
		}
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
	if ascent > b.ascent || b.runeCount == 0 {
		b.ascent = maxFloat32(b.ascent, ascent)
	}
	if b.runeCount == 0 || descent < b.descent {
		b.descent = descent
	}
	if clusterRuneCount > 0 {
		b.runeCount += clusterRuneCount
	}
	b.width = b.cur.startX + b.cur.advance
}

func (b *lineBuilder) flushRun() {
	if b == nil || b.cur == nil {
		return
	}
	lh := b.cur.style.LineHeight
	if lh > 0 {
		if lh < 5 {
			lh = lh * b.cur.style.Size
		}
	} else {
		lh = b.cur.style.Size * 1.2
	}
	runHeight := maxFloat32(maxFloat32(b.ascent-b.descent, lh), 1)
	metrics := LineMetrics{
		Ascent:     b.ascent,
		Descent:    b.descent,
		Leading:    0,
		LineHeight: runHeight,
	}
	run := GlyphRun{
		Glyphs:           append([]PositionedGlyph(nil), b.cur.glyphs...),
		Face:             b.cur.face,
		Size:             b.cur.style.Size,
		Style:            b.cur.style,
		Bounds:           RectFromXYWH(b.cur.startX, 0, b.cur.advance, runHeight),
		Advance:          b.cur.advance,
		Text:             b.cur.text,
		Direction:        b.cur.direction,
		Level:            b.cur.level,
		GraphemeAdvances: append([]float32(nil), b.cur.graphemeAdvances...),
		Language:         b.cur.language,
		Script:           b.cur.script,
		Metrics:          metrics,
	}
	b.runs = append(b.runs, run)
	b.cur = nil
}

func (b *lineBuilder) finish(firstRune int, lineHeight float32, alignment TextAlignment, maxWidth float32) ShapedLine {
	if b == nil {
		return ShapedLine{}
	}
	b.flushRun()

	for i := range b.runs {
		b.runs[i].LogicalIndex = i
	}

	// Visually reorder runs using UBA Rule L2:
	if len(b.runs) > 1 {
		maxLvl := -1
		minOddLvl := 999
		for _, r := range b.runs {
			if r.Level > maxLvl {
				maxLvl = r.Level
			}
			if (r.Level & 1) == 1 {
				if r.Level < minOddLvl {
					minOddLvl = r.Level
				}
			}
		}

		if minOddLvl <= maxLvl {
			for lvl := maxLvl; lvl >= minOddLvl; lvl-- {
				i := 0
				for i < len(b.runs) {
					if b.runs[i].Level >= lvl {
						j := i
						for j < len(b.runs) && b.runs[j].Level >= lvl {
							j++
						}
						for k := 0; k < (j-i)/2; k++ {
							b.runs[i+k], b.runs[j-1-k] = b.runs[j-1-k], b.runs[i+k]
						}
						i = j
					} else {
						i++
					}
				}
			}
		}

		// Recompute visual run.Bounds.Min.X after visual reordering!
		visualX := float32(0)
		for i := range b.runs {
			w := b.runs[i].Bounds.Width()
			b.runs[i].Bounds.Min.X = visualX
			b.runs[i].Bounds.Max.X = visualX + w
			visualX += w
		}
	}

	runs := append([]GlyphRun(nil), b.runs...)
	baseline := b.ascent
	if baseline <= 0 {
		baseline = maxFloat32(lineHeight*0.8, 1)
	}
	naturalLineH := baseline - b.descent
	lineBoxH := maxFloat32(lineHeight, naturalLineH)
	if lineBoxH <= 0 {
		lineBoxH = maxFloat32(lineHeight, 1)
	}
	leading := lineBoxH - naturalLineH
	if leading < 0 {
		leading = 0
	}
	baselineOffset := baseline + leading*0.5
	b.metrics = LineMetrics{
		Ascent:     b.ascent,
		Descent:    b.descent,
		Leading:    leading,
		LineHeight: lineBoxH,
	}

	// Make a copy of b.runs and sort it logically to build clusterMap
	logicalRuns := append([]GlyphRun(nil), b.runs...)
	sort.Slice(logicalRuns, func(i, j int) bool {
		return logicalRuns[i].LogicalIndex < logicalRuns[j].LogicalIndex
	})

	// Build line.clusterMap in logical order of grapheme clusters
	var clusterMap []float32
	for _, run := range logicalRuns {
		startX := run.Bounds.Min.X
		runAdv := run.Advance

		var x float32 = 0
		if run.Direction == di.DirectionLTR {
			for _, adv := range run.GraphemeAdvances {
				clusterMap = append(clusterMap, startX+x)
				x += adv
			}
		} else {
			for _, adv := range run.GraphemeAdvances {
				clusterMap = append(clusterMap, startX+runAdv-x-adv)
				x += adv
			}
		}
	}
	if len(logicalRuns) > 0 {
		lastLRun := logicalRuns[len(logicalRuns)-1]
		if lastLRun.Direction == di.DirectionLTR {
			clusterMap = append(clusterMap, lastLRun.Bounds.Max.X)
		} else {
			clusterMap = append(clusterMap, lastLRun.Bounds.Min.X)
		}
	} else {
		clusterMap = append(clusterMap, 0)
	}

	line := ShapedLine{
		Runs:       runs,
		Bounds:     RectFromXYWH(0, 0, b.width, lineBoxH),
		Baseline:   baselineOffset,
		FirstRune:  firstRune,
		RuneCount:  b.runeCount,
		Direction:  b.direction,
		Language:   b.language,
		Script:     b.script,
		Metrics:    b.metrics,
		clusterMap: clusterMap,
	}
	for i := range line.Runs {
		line.Runs[i].Metrics = line.Metrics
	}
	shiftLine(&line, alignment, maxWidth)
	return line
}

func (l *ShapedLine) hitTestClusterIndex(x float32) (int, bool) {
	if l == nil || len(l.clusterMap) == 0 {
		return 0, false
	}
	if len(l.clusterMap) == 1 {
		return 0, true
	}
	bestIdx := 0
	bestDist := float32(math.MaxFloat32)
	for i := 0; i < len(l.clusterMap)-1; i++ {
		x0 := l.clusterMap[i]
		x1 := l.clusterMap[i+1]
		minX := x0
		maxX := x1
		if minX > maxX {
			minX, maxX = maxX, minX
		}
		if x >= minX && x <= maxX {
			mid := (minX + maxX) / 2
			if x < mid {
				return i, true
			}
			return i + 1, true
		}
		dist := float32(0)
		if x < minX {
			dist = minX - x
		} else {
			dist = x - maxX
		}
		if dist < bestDist {
			bestDist = dist
			bestIdx = i
			if x > maxX {
				bestIdx = i + 1
			}
		}
	}
	return bestIdx, true
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
			out = append(out, TextRange{Start: start, End: end, Unit: TextUnitRune})
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

func resolveBidiDirection(paragraph, span di.Direction, runes []rune) di.Direction {
	if paragraph == di.DirectionRTL {
		return di.DirectionRTL
	}
	if span == di.DirectionRTL {
		return di.DirectionRTL
	}
	for _, r := range runes {
		props, _ := xtextbidi.LookupRune(r)
		switch props.Class() {
		case xtextbidi.R, xtextbidi.AL:
			return di.DirectionRTL
		case xtextbidi.L:
			return di.DirectionLTR
		}
	}
	return di.DirectionLTR
}

func resolveScriptHint(paragraph, span language.Script) language.Script {
	if paragraph != 0 {
		return paragraph
	}
	if span != 0 {
		return span
	}
	return language.Common
}

func toBidiDirection(direction di.Direction) xtextbidi.Direction {
	if direction == di.DirectionRTL {
		return xtextbidi.RightToLeft
	}
	return xtextbidi.LeftToRight
}

func fromBidiDirection(direction xtextbidi.Direction) di.Direction {
	if direction == xtextbidi.RightToLeft {
		return di.DirectionRTL
	}
	return di.DirectionLTR
}

func paragraphTextRunes(spans []TextSpan) []rune {
	if len(spans) == 0 {
		return nil
	}
	var b strings.Builder
	for _, span := range spans {
		b.WriteString(span.Text)
	}
	return []rune(b.String())
}

func resolveLanguage(paragraph, span language.Language) language.Language {
	if strings.TrimSpace(string(span)) != "" {
		return span
	}
	if strings.TrimSpace(string(paragraph)) != "" {
		return paragraph
	}
	return language.DefaultLanguage()
}

func lineHeightFromLines(lines []ShapedLine) float32 {
	var maxH float32
	for _, line := range lines {
		if h := line.Metrics.LineHeight; h > maxH {
			maxH = h
			continue
		}
		if h := line.Bounds.Height(); h > maxH {
			maxH = h
		}
	}
	if maxH <= 0 {
		return DefaultStyle().Size * 1.2
	}
	return maxH
}

func totalLineHeight(lines []ShapedLine) float32 {
	var total float32
	for _, line := range lines {
		if h := line.Bounds.Height(); h > 0 {
			total += h
			continue
		}
		if h := line.Metrics.LineHeight; h > 0 {
			total += h
		}
	}
	return total
}

func aggregateMetrics(lines []ShapedLine) LineMetrics {
	if len(lines) == 0 {
		return LineMetrics{}
	}
	first := lines[0].Metrics
	var maxLineHeight float32
	for _, line := range lines {
		if line.Metrics.LineHeight > maxLineHeight {
			maxLineHeight = line.Metrics.LineHeight
		}
	}
	first.LineHeight = maxLineHeight
	return first
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
	for i := range line.clusterMap {
		line.clusterMap[i] += shift
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

func faceCoversText(face *font.Face, runes []rune) bool {
	if face == nil {
		return false
	}
	for _, r := range runes {
		if _, ok := face.NominalGlyph(r); !ok {
			return false
		}
	}
	return true
}
