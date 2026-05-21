package text

import (
	"math"

	"sort"

	"strings"

	"github.com/go-text/typesetting/di"

	"github.com/go-text/typesetting/font"

	"github.com/go-text/typesetting/language"

	textsegmenter "github.com/go-text/typesetting/segmenter"

	gotextshaping "github.com/go-text/typesetting/shaping"

	xtextbidi "golang.org/x/text/unicode/bidi"
)

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
