package text

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

const testNotoSansRegular = "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf"
const testLigatureFont = "github.com/go-text/typesetting-utils@v0.0.0-20240317173224-1986cbe96c66/opentype/common/DejaVuSans.ttf"

func TestDefaultStyle_non_zero_size(t *testing.T) {
	if got := DefaultStyle(); got.Size <= 0 {
		t.Fatalf("Size = %v", got.Size)
	}
}

func TestWeight_constants_distinct(t *testing.T) {
	seen := map[Weight]struct{}{}
	for _, w := range []Weight{WeightThin, WeightLight, WeightRegular, WeightMedium, WeightSemiBold, WeightBold, WeightBlack} {
		if _, ok := seen[w]; ok {
			t.Fatalf("duplicate weight: %v", w)
		}
		seen[w] = struct{}{}
	}
}

func TestFontRegistry_new_no_error(t *testing.T) {
	reg, err := NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	if reg == nil {
		t.Fatal("expected registry")
	}
	if got := reg.Resolve(TextStyle{}); !got.IsZero() {
		t.Fatalf("expected zero face, got %#v", got)
	}
}

func TestFontRegistry_load_font_file_missing(t *testing.T) {
	reg, _ := NewFontRegistry()
	if err := reg.LoadFontFile(filepath.Join(t.TempDir(), "missing.ttf")); err == nil {
		t.Fatal("expected error")
	}
}

func TestFontRegistry_load_font_bytes(t *testing.T) {
	reg, _ := NewFontRegistry()
	data := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "roboto"); err != nil {
		t.Fatalf("LoadFontBytes: %v", err)
	}
	if got := reg.Resolve(TextStyle{Family: "Noto Sans"}); got.IsZero() {
		t.Fatal("expected loaded face")
	}
}

func TestFontRegistry_resolve_never_nil(t *testing.T) {
	reg, _ := NewFontRegistry()
	if got := reg.Resolve(TextStyle{Family: "NoSuchFontXYZPQR"}); !got.IsZero() {
		t.Fatalf("expected zero face, got %#v", got)
	}
}

func TestFontRegistry_resolve_respects_weight(t *testing.T) {
	reg, _ := NewFontRegistry()
	regularData := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	boldData := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Bold.ttf")
	if err := reg.LoadFontBytes(regularData, "roboto-regular"); err != nil {
		t.Fatalf("LoadFontBytes regular: %v", err)
	}
	if err := reg.LoadFontBytes(boldData, "roboto-bolditalic"); err != nil {
		t.Fatalf("LoadFontBytes bold: %v", err)
	}
	regular := reg.Resolve(TextStyle{Family: "Noto Sans", Weight: WeightRegular})
	bold := reg.Resolve(TextStyle{Family: "Noto Sans", Weight: WeightBold})
	if regular.IsZero() || bold.IsZero() {
		t.Fatal("expected non-zero faces")
	}
	if regular.face == bold.face {
		t.Fatal("expected distinct face records for different requests")
	}
}

func TestParagraph_zero_maxwidth_is_unconstrained(t *testing.T) {
	p := Paragraph{
		Spans: []TextSpan{{Text: "hello", Style: DefaultStyle()}},
	}
	if p.MaxWidth != 0 {
		t.Fatalf("MaxWidth = %v", p.MaxWidth)
	}
}

func TestTextSpan_empty_string(t *testing.T) {
	if got := (TextSpan{Text: ""}); got.Text != "" {
		t.Fatalf("Text = %q", got.Text)
	}
}

func TestLoadFontFile_roundtrip(t *testing.T) {
	reg, _ := NewFontRegistry()
	path := mustTestFontPath(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontFile(path); err != nil {
		t.Fatalf("LoadFontFile: %v", err)
	}
	if got := reg.Resolve(TextStyle{Family: "Noto Sans"}); got.IsZero() {
		t.Fatal("expected loaded face")
	}
}

func TestShaper_shape_simple_non_nil(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.ShapeSimple("abc", style)
	if layout == nil {
		t.Fatal("expected layout")
	}
	if layout.LineCount() != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	if layout.RuneCount() != 3 {
		t.Fatalf("RuneCount = %d", layout.RuneCount())
	}
	if layout.Bounds.Width() <= 0 {
		t.Fatalf("Bounds = %#v", layout.Bounds)
	}
}

func TestShaper_content_scale_affects_size(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	style := DefaultStyle()
	style.Family = family

	base := NewShaper(reg)
	baseLayout := base.ShapeSimple("hello", style)
	if baseLayout == nil || baseLayout.Bounds.Width() <= 0 {
		t.Fatal("expected base layout")
	}

	scaled := NewShaper(reg)
	scaled.SetContentScale(2)
	scaledLayout := scaled.ShapeSimple("hello", style)
	if scaledLayout == nil || scaledLayout.Bounds.Width() <= baseLayout.Bounds.Width() {
		t.Fatalf("scaled width = %v, base width = %v", scaledLayout.Bounds.Width(), baseLayout.Bounds.Width())
	}
}

func TestShaper_wrap_at_word_boundary(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	word := shaper.ShapeSimple("Hello", style)
	layout := shaper.Shape(Paragraph{
		Spans:    []TextSpan{{Text: "Hello World", Style: style}},
		MaxWidth: word.Bounds.Width() + 1,
	})
	if layout.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", layout.LineCount())
	}
	runes := []rune(layout.source)
	prefix := string(runes[:layout.Lines[0].RuneCount])
	if prefix != "Hello" && prefix != "Hello " {
		t.Fatalf("wrapped mid-word, prefix=%q", prefix)
	}
}

func TestShaper_no_wrap_when_maxwidth_zero(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans:    []TextSpan{{Text: "abcdef", Style: style}},
		MaxWidth: 0,
	})
	if layout.LineCount() != 1 {
		t.Fatalf("expected single line, got %d", layout.LineCount())
	}
}

func TestShaper_mandatory_newline_always_breaks(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{{Text: "\n", Style: style}},
	})
	if layout.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", layout.LineCount())
	}
}

func TestShaper_wrap_long_word(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans:    []TextSpan{{Text: "supercalifragilisticexpialidocious", Style: style}},
		MaxWidth: 10,
	})
	if layout.LineCount() != 1 {
		t.Fatalf("expected single line for long word, got %d", layout.LineCount())
	}
}

func TestShaper_alignment_center_and_right(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	center := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "abc", Style: style}},
		MaxWidth:  100,
		Alignment: AlignCenter,
	})
	right := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "abc", Style: style}},
		MaxWidth:  100,
		Alignment: AlignRight,
	})
	if center == nil || right == nil {
		t.Fatal("expected layouts")
	}
	if center.Lines[0].Bounds.Min.X <= 0 {
		t.Fatalf("center line bounds = %#v", center.Lines[0].Bounds)
	}
	if right.Lines[0].Bounds.Min.X <= center.Lines[0].Bounds.Min.X {
		t.Fatalf("right line bounds = %#v, center = %#v", right.Lines[0].Bounds, center.Lines[0].Bounds)
	}
}

func TestShaper_wrap_alignment_center(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "abc", Style: style}},
		MaxWidth:  100,
		Alignment: AlignCenter,
	})
	if layout.LineCount() != 1 {
		t.Fatalf("expected single line, got %d", layout.LineCount())
	}
	if layout.Lines[0].Bounds.Min.X <= 0 {
		t.Fatalf("center line bounds = %#v", layout.Lines[0].Bounds)
	}
}

func TestShaper_wrap_alignment_right(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "abc", Style: style}},
		MaxWidth:  100,
		Alignment: AlignRight,
	})
	if layout.LineCount() != 1 {
		t.Fatalf("expected single line, got %d", layout.LineCount())
	}
	if layout.Lines[0].Bounds.Min.X <= 0 {
		t.Fatalf("right line bounds = %#v", layout.Lines[0].Bounds)
	}
}

func TestShaper_multi_line_bounds(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	single := shaper.ShapeSimple("Hello", style)
	multi := shaper.Shape(Paragraph{
		Spans:    []TextSpan{{Text: "Hello World", Style: style}},
		MaxWidth: single.Bounds.Width() + 1,
	})
	if multi.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %d", multi.LineCount())
	}
	if multi.Bounds.Height() <= single.Bounds.Height() {
		t.Fatalf("expected taller bounds, single=%#v multi=%#v", single.Bounds, multi.Bounds)
	}
}

func TestTextLayout_empty_spans_valid(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{Spans: []TextSpan{{Text: "", Style: style}}})
	if layout == nil {
		t.Fatal("expected layout")
	}
	if layout.LineCount() != 0 {
		t.Fatalf("expected no shaped lines, got %d", layout.LineCount())
	}
}

func TestHitTest_before_all_text(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.HitTest(Point{X: -10, Y: 5})
	if pos.Index != 0 {
		t.Fatalf("pos = %#v", pos)
	}
}

func TestHitTest_after_all_text(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.HitTest(Point{X: 500, Y: 500})
	if pos.Index != layout.RuneCount() {
		t.Fatalf("pos = %#v count=%d", pos, layout.RuneCount())
	}
}

func TestHitTest_middle_of_glyph(t *testing.T) {
	layout := wrappedHelloWorld(t)
	first := layout.Lines[0].Runs[0]
	pos := layout.HitTest(Point{X: first.Bounds.Min.X + first.Advance*0.25, Y: 5})
	if pos.Index < first.Glyphs[0].RuneIndex || pos.Index > first.Glyphs[0].RuneIndex+1 {
		t.Fatalf("pos = %#v", pos)
	}
}

func TestHitTest_right_half_of_glyph(t *testing.T) {
	layout := wrappedHelloWorld(t)
	first := layout.Lines[0].Runs[0]
	pos := layout.HitTest(Point{X: first.Bounds.Min.X + first.Advance*0.75, Y: 5})
	if pos.Index <= first.Glyphs[0].RuneIndex {
		t.Fatalf("pos = %#v", pos)
	}
}

func TestHitTest_between_lines(t *testing.T) {
	layout := wrappedHelloWorld(t)
	y := (layout.Lines[0].Bounds.Max.Y + layout.Lines[1].Bounds.Min.Y) / 2
	pos := layout.HitTest(Point{X: 1, Y: y})
	if layout.LineAt(pos) < 0 || layout.LineAt(pos) >= layout.LineCount() {
		t.Fatalf("pos = %#v line=%d", pos, layout.LineAt(pos))
	}
}

func TestCaretRect_at_start(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rect := layout.CaretRect(layout.PositionAtLineStart(0))
	if rect.Min.X != layout.Lines[0].Bounds.Min.X {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestCaretRect_at_end(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rect := layout.CaretRect(layout.PositionAtLineEnd(layout.LineCount() - 1))
	last := layout.Lines[layout.LineCount()-1]
	if rect.Min.X != last.Bounds.Max.X {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestCaretRect_height_equals_lineheight(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rect := layout.CaretRect(layout.PositionAtLineStart(0))
	if rect.Height() != layout.LineHeight {
		t.Fatalf("rect = %#v lineheight=%v", rect, layout.LineHeight)
	}
}

func TestCaretRect_width_is_2px(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rect := layout.CaretRect(layout.PositionAtLineStart(0))
	if rect.Width() != 2 {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestCaretRect_affinity_upstream_at_wrap(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.PositionAtLineEnd(0)
	rect := layout.CaretRect(pos)
	if rect.Min.X != layout.Lines[0].Bounds.Max.X {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestCaretRect_affinity_downstream_at_wrap(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.PositionAtLineStart(1)
	rect := layout.CaretRect(pos)
	if rect.Min.X != layout.Lines[1].Bounds.Min.X {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestSelectionRects_empty_range(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if rects := layout.SelectionRects(TextRange{Start: 3, End: 3}); len(rects) != 0 {
		t.Fatalf("rects = %#v", rects)
	}
}

func TestSelectionRects_single_line(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rects := layout.SelectionRects(TextRange{Start: 0, End: 2})
	if len(rects) != 1 {
		t.Fatalf("rects = %#v", rects)
	}
}

func TestSelectionRects_cross_line(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rects := layout.SelectionRects(TextRange{Start: 1, End: 6})
	if len(rects) == 0 {
		t.Fatalf("rects = %#v", rects)
	}
}

func TestSelectionRects_full_document(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rects := layout.SelectionRects(TextRange{Start: 0, End: layout.RuneCount()})
	if len(rects) != layout.LineCount() {
		t.Fatalf("rects = %#v lines=%d", rects, layout.LineCount())
	}
}

func TestNextPosition_advances(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.NextPosition(TextPosition{Index: 0}); got.Index != 1 {
		t.Fatalf("pos = %#v", got)
	}
}

func TestNextPosition_clamps_at_end(t *testing.T) {
	layout := wrappedHelloWorld(t)
	got := layout.NextPosition(TextPosition{Index: layout.RuneCount()})
	if got.Index != layout.RuneCount() {
		t.Fatalf("pos = %#v", got)
	}
}

func TestPrevPosition_moves_back(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.PrevPosition(TextPosition{Index: 1}); got.Index != 0 {
		t.Fatalf("pos = %#v", got)
	}
}

func TestPrevPosition_clamps_at_start(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.PrevPosition(TextPosition{Index: 0}); got.Index != 0 {
		t.Fatalf("pos = %#v", got)
	}
}

func TestLineAt_returns_correct_line(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.LineAt(layout.PositionAtLineStart(1)); got != 1 {
		t.Fatalf("line = %d", got)
	}
}

func TestPositionAtLineStart_is_start(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.PositionAtLineStart(1); got.Index != layout.Lines[1].FirstRune {
		t.Fatalf("pos = %#v", got)
	}
}

func TestPositionAtLineEnd_is_end(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.PositionAtLineEnd(0); got.Index != layout.Lines[0].FirstRune+layout.Lines[0].RuneCount {
		t.Fatalf("pos = %#v", got)
	}
}

func TestWordBoundaryAt_middle_of_word(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rng := layout.WordBoundaryAt(TextPosition{Index: 1})
	if rng.Start != 0 || rng.End < 5 {
		t.Fatalf("range = %#v", rng)
	}
}

func TestWordBoundaryAt_whitespace(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rng := layout.WordBoundaryAt(TextPosition{Index: 5})
	if rng.Start != 0 || rng.End != 5 {
		t.Fatalf("range = %#v", rng)
	}
}

func wrappedHelloWorld(t *testing.T) *TextLayout {
	t.Helper()
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	style := DefaultStyle()
	style.Family = family
	shaper := NewShaper(reg)
	return shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "Hello World", Style: style}},
		MaxWidth:  40,
		Alignment: AlignLeft,
	})
}

func TestShaper_glyph_ids_nonzero(t *testing.T) {
	layout := shapedASCII(t, "abc")
	if len(layout.Lines) == 0 || len(layout.Lines[0].Runs) == 0 || len(layout.Lines[0].Runs[0].Glyphs) == 0 {
		t.Fatal("expected shaped glyphs")
	}
	for _, g := range layout.Lines[0].Runs[0].Glyphs {
		if g.GlyphID == 0 {
			t.Fatalf("glyph = %#v", g)
		}
	}
}

func TestShaper_advance_positive(t *testing.T) {
	layout := shapedASCII(t, "abc")
	for _, line := range layout.Lines {
		for _, run := range line.Runs {
			for _, g := range run.Glyphs {
				if g.Advance <= 0 {
					t.Fatalf("glyph = %#v", g)
				}
			}
		}
	}
}

func TestShaper_rune_index_correct(t *testing.T) {
	layout := shapedASCII(t, "abc")
	glyphs := layout.Lines[0].Runs[0].Glyphs
	if len(glyphs) != 3 {
		t.Fatalf("glyphs = %#v", glyphs)
	}
	for i, g := range glyphs {
		if g.RuneIndex != i {
			t.Fatalf("glyph %d = %#v", i, g)
		}
	}
}

func TestShaper_ligature_single_glyph(t *testing.T) {
	reg, family := mustTestRegistry(t, testLigatureFont)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.ShapeSimple("fi", style)
	if layout.LineCount() == 0 || len(layout.Lines[0].Runs) == 0 {
		t.Fatal("expected shaped output")
	}
	glyphs := layout.Lines[0].Runs[0].Glyphs
	if len(glyphs) != 1 {
		t.Fatalf("expected ligature glyph, got %#v", glyphs)
	}
}

func TestTextLayout_HitTest_before_first_glyph(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.HitTest(Point{X: -10, Y: 5})
	if pos.Index != 0 {
		t.Fatalf("pos = %#v", pos)
	}
}

func TestTextLayout_HitTest_after_last_glyph(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.HitTest(Point{X: 500, Y: 500})
	if pos.Index != layout.RuneCount() {
		t.Fatalf("pos = %#v count=%d", pos, layout.RuneCount())
	}
}

func TestTextLayout_HitTest_ligature(t *testing.T) {
	reg, family := mustTestRegistry(t, testLigatureFont)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.ShapeSimple("fi", style)
	if layout.LineCount() != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	glyph := layout.Lines[0].Runs[0]
	pos := layout.HitTest(Point{X: glyph.Bounds.Min.X + glyph.Advance*0.5, Y: 5})
	if pos.Index < 0 || pos.Index > 1 {
		t.Fatalf("pos = %#v", pos)
	}
}

func TestTextLayout_CaretRect_width(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rect := layout.CaretRect(layout.PositionAtLineStart(0))
	if rect.Width() != 2 {
		t.Fatalf("rect = %#v", rect)
	}
	if rect.Height() <= 0 {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestTextLayout_CaretRect_position(t *testing.T) {
	layout := wrappedHelloWorld(t)
	pos := layout.PositionAtLineStart(0)
	rect := layout.CaretRect(pos)
	if rect.Min.X != layout.Lines[0].Bounds.Min.X {
		t.Fatalf("rect = %#v", rect)
	}
}

func TestTextLayout_SelectionRects_multiline(t *testing.T) {
	layout := wrappedHelloWorld(t)
	rects := layout.SelectionRects(TextRange{Start: 0, End: layout.RuneCount()})
	if len(rects) != layout.LineCount() {
		t.Fatalf("rects = %#v lines=%d", rects, layout.LineCount())
	}
}

func TestTextLayout_NextPrev_clamp(t *testing.T) {
	layout := wrappedHelloWorld(t)
	if got := layout.NextPosition(TextPosition{Index: layout.RuneCount()}); got.Index != layout.RuneCount() {
		t.Fatalf("next = %#v", got)
	}
	if got := layout.PrevPosition(TextPosition{Index: 0}); got.Index != 0 {
		t.Fatalf("prev = %#v", got)
	}
}

func TestShaper_empty_string(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.ShapeSimple("", style)
	if layout == nil {
		t.Fatal("expected layout")
	}
	if layout.LineCount() != 0 {
		t.Fatalf("expected no lines, got %d", layout.LineCount())
	}
}

func TestShaper_newline_splits_line(t *testing.T) {
	layout := shapedASCII(t, "a\nb")
	if layout.LineCount() != 2 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
}

func TestShaper_multistyle_span(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	if err := reg.LoadFontBytes(mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Bold.ttf"), "NotoSans-Bold.ttf"); err != nil {
		t.Fatalf("LoadFontBytes bold: %v", err)
	}
	shaper := NewShaper(reg)
	regular := DefaultStyle()
	regular.Family = family
	bold := regular
	bold.Weight = WeightBold
	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{
			{Text: "ab", Style: regular},
			{Text: "cd", Style: bold},
		},
	})
	if layout.LineCount() != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	if len(layout.Lines[0].Runs) < 2 {
		t.Fatalf("Runs = %#v", layout.Lines[0].Runs)
	}
	if layout.Lines[0].Runs[0].Face.CacheKey() == layout.Lines[0].Runs[1].Face.CacheKey() {
		t.Fatalf("expected different faces, got %#v", layout.Lines[0].Runs)
	}
}

func mustTestFontPath(t *testing.T, rel string) string {
	t.Helper()
	modCache := mustGoModCache(t)
	path := filepath.Join(modCache, rel)
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("test font path %q: %v", path, err)
	}
	return path
}

func mustReadTestFont(t *testing.T, rel string) []byte {
	t.Helper()
	path := mustTestFontPath(t, rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read test font %q: %v", path, err)
	}
	return data
}

func mustGoModCache(t *testing.T) string {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	return string(bytes.TrimSpace(out))
}

func mustTestRegistry(t *testing.T, rel string) (*FontRegistry, string) {
	t.Helper()
	reg, err := NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	if err := reg.LoadFontBytes(mustReadTestFont(t, rel), filepath.Base(rel)); err != nil {
		t.Fatalf("LoadFontBytes: %v", err)
	}
	if len(reg.faces) == 0 || reg.faces[0] == nil {
		t.Fatalf("expected loaded faces")
	}
	return reg, reg.faces[0].desc.Family
}

func shapedASCII(t *testing.T, text string) *TextLayout {
	t.Helper()
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.ShapeSimple(text, style)
	if layout == nil {
		t.Fatal("expected layout")
	}
	return layout
}
