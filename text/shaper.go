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
	stackedY := float32(0)
	for i := range layout.Lines {
		h := layout.Lines[i].Bounds.Height()
		layout.Lines[i].Bounds.Min.Y = stackedY
		layout.Lines[i].Bounds.Max.Y = stackedY + h
		stackedY += h
	}
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
		var level int
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
