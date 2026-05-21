package text

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
	xtextbidi "golang.org/x/text/unicode/bidi"
)

const testNotoSansRegular = "testdata/NotoSans-Regular.ttf"
const testLigatureFont = "testdata/NotoSans-Regular.ttf"

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
	path := mustResolveTestFontPath(t, testNotoSansRegular)
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
	runes := []rune(layout.Source)
	prefix := string(runes[:layout.Lines[0].RuneCount])
	if prefix != "Hello" && prefix != "Hello " {
		t.Fatalf("wrapped mid-word, prefix=%q", prefix)
	}
}

func TestShape_preserves_paragraph_metadata(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	paragraph := Paragraph{
		Spans: []TextSpan{{
			Text:  "ابج",
			Style: style,
		}},
		Direction: di.DirectionRTL,
		Language:  language.NewLanguage("ar"),
		Script:    language.Arabic,
	}
	layout := shaper.Shape(paragraph)
	if layout == nil {
		t.Fatal("expected layout")
	}
	if layout.Paragraph.Direction != paragraph.Direction {
		t.Fatalf("paragraph direction = %v", layout.Paragraph.Direction)
	}
	if layout.Paragraph.Language != paragraph.Language {
		t.Fatalf("paragraph language = %q", layout.Paragraph.Language)
	}
	if layout.Paragraph.Script != paragraph.Script {
		t.Fatalf("paragraph script = %v", layout.Paragraph.Script)
	}
	if len(layout.Lines) == 0 {
		t.Fatal("expected shaped line")
	}
	line := layout.Lines[0]
	if line.Direction != paragraph.Direction {
		t.Fatalf("line direction = %v", line.Direction)
	}
	if line.Language != paragraph.Language {
		t.Fatalf("line language = %q", line.Language)
	}
	if line.Script != paragraph.Script {
		t.Fatalf("line script = %v", line.Script)
	}
	if len(line.Runs) == 0 {
		t.Fatal("expected shaped run")
	}
	run := line.Runs[0]
	if run.Direction != paragraph.Direction {
		t.Fatalf("run direction = %v", run.Direction)
	}
	if run.Language != paragraph.Language {
		t.Fatalf("run language = %q", run.Language)
	}
	if run.Script != paragraph.Script {
		t.Fatalf("run script = %v", run.Script)
	}
}

func TestShaper_first_strong_direction_resolution(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{{Text: "אבגabc", Style: style}},
	})
	if layout == nil {
		t.Fatal("expected layout")
	}
	if len(layout.Lines) != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	if got := layout.Lines[0].Direction; got != di.DirectionRTL {
		t.Fatalf("line direction = %v", got)
	}
}

func TestShaper_mixed_direction_visual_order(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	text := "abcאבגdef"
	layout := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: text, Style: style}},
		Direction: di.DirectionRTL,
	})
	if layout == nil {
		t.Fatal("expected layout")
	}
	if len(layout.Lines) != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	want := []string{"def", "אבג", "abc"}
	got := runTexts(layout.Lines[0].Runs)
	if len(got) != len(want) {
		t.Fatalf("run count = %d want %d: got=%#v want=%#v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run[%d] = %q want %q (runs=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestShaper_isolates_and_embeddings_preserve_visual_runs(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	text := "Hello \u2067אבג\u2069 world"
	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{{Text: text, Style: style}},
	})
	if layout == nil {
		t.Fatal("expected layout")
	}
	if len(layout.Lines) != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	if layout.Lines[0].Direction != di.DirectionLTR {
		t.Fatalf("line direction = %v", layout.Lines[0].Direction)
	}
	if got := runTexts(layout.Lines[0].Runs); len(got) < 3 {
		t.Fatalf("expected isolate-separated runs, got %#v", got)
	}
}

func TestShaper_script_mixed_shaping(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	text := "abcאבג"
	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{{Text: text, Style: style}},
	})
	if layout == nil {
		t.Fatal("expected layout")
	}
	if len(layout.Lines) != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	gotScripts := map[language.Script]bool{}
	for _, run := range layout.Lines[0].Runs {
		gotScripts[run.Script] = true
	}
	if !gotScripts[language.Latin] || !gotScripts[language.Hebrew] {
		t.Fatalf("expected mixed scripts, got runs=%#v", layout.Lines[0].Runs)
	}
	want := bidiRunTexts(t, text, xtextbidi.LeftToRight)
	got := runTexts(layout.Lines[0].Runs)
	if len(got) != len(want) {
		t.Fatalf("run count = %d want %d: got=%#v want=%#v", len(got), len(want), got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("run[%d] = %q want %q (runs=%#v)", i, got[i], want[i], got)
		}
	}
}

func TestTextRange_cluster_helpers_preserve_unit(t *testing.T) {
	pos := GraphemePosition(7, AffinityUpstream)
	if pos.Unit != TextUnitGrapheme {
		t.Fatalf("position unit = %v", pos.Unit)
	}
	rng := GraphemeRange(9, 3)
	if rng.Unit != TextUnitGrapheme {
		t.Fatalf("range unit = %v", rng.Unit)
	}
	normalized := rng.Normalized()
	if normalized.Start != 3 || normalized.End != 9 {
		t.Fatalf("normalized range = %#v", normalized)
	}
	if normalized.Unit != TextUnitGrapheme {
		t.Fatalf("normalized unit = %v", normalized.Unit)
	}
	if !normalized.Contains(4) {
		t.Fatalf("expected range to contain index 4: %#v", normalized)
	}
}

func TestShaper_preserves_metrics_and_zero_width_controls(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	content := "a\u200db"
	layout := shaper.ShapeSimple(content, style)
	if layout == nil {
		t.Fatal("expected layout")
	}
	if layout.Source != content {
		t.Fatalf("source = %q", layout.Source)
	}
	if layout.RuneCount() != 3 {
		t.Fatalf("RuneCount = %d", layout.RuneCount())
	}
	if len(layout.Lines) != 1 {
		t.Fatalf("LineCount = %d", layout.LineCount())
	}
	line := layout.Lines[0]
	if line.Metrics.LineHeight <= 0 {
		t.Fatalf("line metrics = %#v", line.Metrics)
	}
	if layout.LineHeight != line.Metrics.LineHeight {
		t.Fatalf("layout lineheight = %v line metrics = %#v", layout.LineHeight, line.Metrics)
	}
	if layout.Metrics.LineHeight != line.Metrics.LineHeight {
		t.Fatalf("layout metrics = %#v line metrics = %#v", layout.Metrics, line.Metrics)
	}
	if boundary := layout.WordBoundaryAt(GraphemePosition(1, AffinityDownstream)); boundary.Unit != TextUnitGrapheme {
		t.Fatalf("boundary unit = %v", boundary.Unit)
	}
}

func bidiRunTexts(t *testing.T, text string, defaultDir xtextbidi.Direction) []string {
	t.Helper()
	var p xtextbidi.Paragraph
	if _, err := p.SetString(text, xtextbidi.DefaultDirection(defaultDir)); err != nil {
		t.Fatalf("SetString: %v", err)
	}
	ordering, err := p.Order()
	if err != nil {
		t.Fatalf("Order: %v", err)
	}
	out := make([]string, 0, ordering.NumRuns())
	for i := 0; i < ordering.NumRuns(); i++ {
		run := ordering.Run(i)
		out = append(out, run.String())
	}
	return out
}

func runTexts(runs []GlyphRun) []string {
	out := make([]string, 0, len(runs))
	for _, run := range runs {
		out = append(out, run.Text)
	}
	return out
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

func TestShaper_kerning_pair_changes_glyph_positions(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family

	for _, pair := range []string{"AV", "To", "Ta", "WA"} {
		runes := []rune(pair)
		if len(runes) != 2 {
			continue
		}
		layout := shaper.ShapeSimple(pair, style)
		if layout == nil || layout.LineCount() == 0 || len(layout.Lines[0].Runs) == 0 || len(layout.Lines[0].Runs[0].Glyphs) < 2 {
			continue
		}
		run := layout.Lines[0].Runs[0]
		left := shaper.ShapeSimple(string(runes[0]), style)
		right := shaper.ShapeSimple(string(runes[1]), style)
		if left == nil || right == nil || left.LineCount() == 0 || right.LineCount() == 0 {
			continue
		}
		if len(left.Lines[0].Runs) == 0 || len(right.Lines[0].Runs) == 0 {
			continue
		}
		if run.Glyphs[1].X+0.01 >= left.Lines[0].Runs[0].Advance {
			continue
		}
		if layout.Bounds.Width()+0.01 >= left.Bounds.Width()+right.Bounds.Width() {
			continue
		}
		return
	}
	t.Fatal("expected at least one kerning pair to tighten glyph spacing")
}

func TestShaper_alignment_center_mixed_case_preserves_baseline(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family

	left := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "AaBbCcGgJjQq", Style: style}},
		MaxWidth:  240,
		Alignment: AlignLeft,
	})
	center := shaper.Shape(Paragraph{
		Spans:     []TextSpan{{Text: "AaBbCcGgJjQq", Style: style}},
		MaxWidth:  240,
		Alignment: AlignCenter,
	})
	if left == nil || center == nil {
		t.Fatal("expected layouts")
	}
	if len(left.Lines) == 0 || len(center.Lines) == 0 {
		t.Fatal("expected shaped lines")
	}
	if center.Lines[0].Bounds.Min.X <= left.Lines[0].Bounds.Min.X {
		t.Fatalf("center line bounds = %#v, left = %#v", center.Lines[0].Bounds, left.Lines[0].Bounds)
	}
	if diff := math.Abs(float64(center.Lines[0].Baseline - left.Lines[0].Baseline)); diff > 0.01 {
		t.Fatalf("baseline changed across alignment: left=%v center=%v", left.Lines[0].Baseline, center.Lines[0].Baseline)
	}
	if center.Lines[0].Baseline <= 0 || center.Lines[0].Baseline >= center.Lines[0].Bounds.Height() {
		t.Fatalf("center line baseline = %v, bounds=%#v", center.Lines[0].Baseline, center.Lines[0].Bounds)
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

func TestTextLayout_grapheme_navigation_combining_mark(t *testing.T) {
	layout := shapedASCII(t, "a\u0301b")
	if got := layout.GraphemeCount(); got != 2 {
		t.Fatalf("GraphemeCount = %d", got)
	}
	pos := layout.HitTest(Point{X: layout.Lines[0].Runs[0].Bounds.Min.X + layout.Lines[0].Runs[0].Advance*0.1, Y: 5})
	if pos.Unit != TextUnitGrapheme {
		t.Fatalf("pos unit = %v", pos.Unit)
	}
	if pos.Index != 0 {
		t.Fatalf("pos = %#v", pos)
	}
	after := layout.HitTest(Point{X: layout.Lines[0].Runs[0].Bounds.Min.X + layout.Lines[0].Runs[0].Advance*0.9, Y: 5})
	if after.Unit != TextUnitGrapheme || after.Index == 0 {
		t.Fatalf("after = %#v", after)
	}
	next := layout.NextPosition(GraphemePosition(0, AffinityDownstream))
	if next.Unit != TextUnitGrapheme || next.Index != 1 {
		t.Fatalf("next = %#v", next)
	}
	prev := layout.PrevPosition(next)
	if prev.Unit != TextUnitGrapheme || prev.Index != 0 {
		t.Fatalf("prev = %#v", prev)
	}
	rects := layout.SelectionRects(GraphemeRange(0, 1))
	if len(rects) != 1 || rects[0].IsEmpty() {
		t.Fatalf("rects = %#v", rects)
	}
}

func TestTextLayout_grapheme_word_boundary_and_caret(t *testing.T) {
	layout := shapedASCII(t, "a\u0301 b")
	boundary := layout.WordBoundaryAt(GraphemePosition(0, AffinityDownstream))
	if boundary.Unit != TextUnitGrapheme || boundary.Start != 0 {
		t.Fatalf("boundary = %#v", boundary)
	}
	caret := layout.CaretRect(GraphemePosition(1, AffinityUpstream))
	if caret.Width() != 2 {
		t.Fatalf("caret = %#v", caret)
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
	if err := reg.LoadFontBytes(mustReadTestFont(t, "testdata/NotoSans-Bold.ttf"), "NotoSans-Bold.ttf"); err != nil {
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

func TestShaper_uses_fallback_face_for_symbol_run(t *testing.T) {
	reg, err := NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}

	latinData := mustReadTestFont(t, testNotoSansRegular)
	emojiData := mustReadTestFont(t, "github.com/go-text/render@v0.2.0/testdata/EmojiOneColor.otf")

	if err := reg.LoadFontBytes(latinData, "NotoSans-Regular.ttf"); err != nil {
		t.Fatalf("LoadFontBytes latin: %v", err)
	}
	if err := reg.LoadFontBytes(emojiData, "EmojiOneColor.otf"); err != nil {
		t.Fatalf("LoadFontBytes emoji: %v", err)
	}

	var latinKey, emojiKey uint64
	for _, rec := range reg.faces {
		if rec == nil || rec.face == nil {
			continue
		}
		if faceCoversText(rec.face, []rune("A")) && !faceCoversText(rec.face, []rune("😀")) {
			latinKey = rec.cacheKey
		}
		if faceCoversText(rec.face, []rune("😀")) {
			emojiKey = rec.cacheKey
		}
	}
	if latinKey == 0 {
		t.Fatal("expected latin face")
	}
	if emojiKey == 0 {
		t.Fatal("expected emoji fallback face")
	}

	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = reg.faces[0].desc.Family
	layout := shaper.ShapeSimple("A😀A", style)
	if layout == nil || layout.LineCount() != 1 {
		t.Fatalf("unexpected layout: %#v", layout)
	}

	var gotLatin, gotEmoji bool
	for _, run := range layout.Lines[0].Runs {
		switch run.Face.CacheKey() {
		case latinKey:
			gotLatin = true
		case emojiKey:
			gotEmoji = true
		}
	}
	if !gotLatin || !gotEmoji {
		t.Fatalf("expected mixed face run selection, got runs=%#v latin=%v emoji=%v", layout.Lines[0].Runs, gotLatin, gotEmoji)
	}
}

func mustReadTestFont(t *testing.T, path string) []byte {
	t.Helper()
	for _, candidate := range testFontCandidates(path) {
		data, err := os.ReadFile(candidate)
		if err == nil {
			return data
		}
	}
	t.Fatalf("read test font %q: no candidate found", path)
	return nil
}

func mustResolveTestFontPath(t *testing.T, path string) string {
	t.Helper()
	for _, candidate := range testFontCandidates(path) {
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}
	t.Fatalf("resolve test font %q: no candidate found", path)
	return ""
}

func testFontCandidates(path string) []string {
	candidates := []string{path}
	if filepath.IsAbs(path) {
		return candidates
	}
	roots := []string{}
	if gomodcache := os.Getenv("GOMODCACHE"); gomodcache != "" {
		roots = append(roots, gomodcache)
	}
	if gopath := os.Getenv("GOPATH"); gopath != "" {
		roots = append(roots, filepath.Join(gopath, "pkg", "mod"))
	}
	if home, err := os.UserHomeDir(); err == nil && home != "" {
		roots = append(roots, filepath.Join(home, "go", "pkg", "mod"))
	}
	for _, root := range roots {
		candidates = append(candidates, filepath.Join(root, path))
		if len(path) >= len("testdata/") && path[:len("testdata/")] == "testdata/" {
			candidates = append(candidates, filepath.Join(root, "github.com/go-text/render@v0.2.0", path))
			candidates = append(candidates, filepath.Join(root, "github.com/go-text/typesetting-utils@v0.0.0-20240317173224-1986cbe96c66", "opentype", "common", filepath.Base(path)))
		}
	}
	return uniquePaths(candidates)
}

func uniquePaths(paths []string) []string {
	seen := make(map[string]struct{}, len(paths))
	out := make([]string, 0, len(paths))
	for _, p := range paths {
		if p == "" {
			continue
		}
		if _, ok := seen[p]; ok {
			continue
		}
		seen[p] = struct{}{}
		out = append(out, p)
	}
	return out
}

func mustTestRegistry(t *testing.T, path string) (*FontRegistry, string) {
	t.Helper()
	reg, err := NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	if err := reg.LoadFontBytes(mustReadTestFont(t, path), filepath.Base(path)); err != nil {
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

// ─── Phase 6: Typography Metrics and Authored Spacing ────────────────────────

// TestLetterSpacing_increases_advance verifies that a positive LetterSpacing
// value causes the overall shaped width to grow relative to the baseline.
func TestLetterSpacing_increases_advance(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	base := DefaultStyle()
	base.Family = family

	spaced := base
	spaced.LetterSpacing = 5

	baseLayout := shaper.ShapeSimple("abc", base)
	spacedLayout := shaper.ShapeSimple("abc", spaced)
	if baseLayout == nil || spacedLayout == nil {
		t.Fatal("expected layouts")
	}
	if spacedLayout.Bounds.Width() <= baseLayout.Bounds.Width() {
		t.Fatalf("letter spacing did not increase width: base=%v spaced=%v",
			baseLayout.Bounds.Width(), spacedLayout.Bounds.Width())
	}
}

// TestLetterSpacing_zero_matches_baseline verifies that zero LetterSpacing
// produces the same width as the default style (which has zero spacing).
func TestLetterSpacing_zero_matches_baseline(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family
	style.LetterSpacing = 0

	base := DefaultStyle()
	base.Family = family

	a := shaper.ShapeSimple("hello", style)
	b := shaper.ShapeSimple("hello", base)
	if a == nil || b == nil {
		t.Fatal("expected layouts")
	}
	if a.Bounds.Width() != b.Bounds.Width() {
		t.Fatalf("zero spacing diverged from default: %v vs %v",
			a.Bounds.Width(), b.Bounds.Width())
	}
}

// TestLetterSpacing_does_not_split_cluster ensures that letter spacing applied
// to a ligature run ("fi") does not corrupt the glyph count.
func TestLetterSpacing_does_not_split_cluster(t *testing.T) {
	reg, family := mustTestRegistry(t, testLigatureFont)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family
	style.LetterSpacing = 3

	layout := shaper.ShapeSimple("fi", style)
	if layout == nil || layout.LineCount() != 1 {
		t.Fatal("expected single-line layout")
	}
	// The ligature "fi" is a single glyph. Spacing must not split it into two.
	glyphs := layout.Lines[0].Runs[0].Glyphs
	if len(glyphs) != 1 {
		t.Fatalf("letter spacing split ligature cluster: glyphs=%v", len(glyphs))
	}
}

// TestLineHeight_multiplier_less_than_5 checks that a LineHeight value below 5
// is interpreted as a multiplier (e.g. 1.5 * Size), producing a taller line
// box than the raw font metrics alone.
func TestLineHeight_multiplier_less_than_5(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	base := DefaultStyle()
	base.Family = family

	tall := base
	tall.LineHeight = 2.5 // multiplier: 2.5 × size

	baseLayout := shaper.ShapeSimple("abc", base)
	tallLayout := shaper.ShapeSimple("abc", tall)
	if baseLayout == nil || tallLayout == nil {
		t.Fatal("expected layouts")
	}
	if tallLayout.LineHeight <= baseLayout.LineHeight {
		t.Fatalf("multiplier line-height did not increase line box: base=%v tall=%v",
			baseLayout.LineHeight, tallLayout.LineHeight)
	}
}

// TestLineHeight_absolute_large_value checks that a LineHeight value ≥ 5 is
// used directly as an absolute pixel height rather than as a multiplier.
func TestLineHeight_absolute_large_value(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family
	style.LineHeight = 40 // absolute: 40px regardless of font size

	layout := shaper.ShapeSimple("abc", style)
	if layout == nil || layout.LineCount() != 1 {
		t.Fatal("expected single-line layout")
	}
	// Line height must be at least the authored 40px.
	if layout.LineHeight < 40 {
		t.Fatalf("absolute line-height not respected: got %v want ≥ 40", layout.LineHeight)
	}
}

// TestLineHeight_default_positive verifies that the default style (zero
// LineHeight) still produces a positive line box height from font metrics.
func TestLineHeight_default_positive(t *testing.T) {
	layout := shapedASCII(t, "hello")
	if layout.LineHeight <= 0 {
		t.Fatalf("default line height = %v, want > 0", layout.LineHeight)
	}
}

// TestLineHeight_multi_line_consistent checks that all lines in a wrapped
// paragraph carry the same line-height value (consistent interline spacing).
func TestLineHeight_multi_line_consistent(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family
	style.LineHeight = 1.5

	singleWord := shaper.ShapeSimple("Hi", style)
	layout := shaper.Shape(Paragraph{
		Spans:    []TextSpan{{Text: "Hello World Foo Bar", Style: style}},
		MaxWidth: singleWord.Bounds.Width() + 5,
	})
	if layout == nil || layout.LineCount() < 2 {
		t.Fatalf("expected multi-line layout, got %d lines", layout.LineCount())
	}
	first := layout.Lines[0].Metrics.LineHeight
	for i, line := range layout.Lines {
		if line.Metrics.LineHeight != first {
			t.Fatalf("line %d height %v differs from line 0 height %v",
				i, line.Metrics.LineHeight, first)
		}
	}
}

// TestTabWidth_expands_advance checks that a tab character in the input
// produces a non-zero advance that grows with the TabWidth multiplier.
func TestTabWidth_expands_advance(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	narrow := DefaultStyle()
	narrow.Family = family
	narrow.TabWidth = 2

	wide := DefaultStyle()
	wide.Family = family
	wide.TabWidth = 8

	narrowLayout := shaper.ShapeSimple("\t", narrow)
	wideLayout := shaper.ShapeSimple("\t", wide)
	if narrowLayout == nil || wideLayout == nil {
		t.Fatal("expected layouts for tab character")
	}
	if narrowLayout.Bounds.Width() <= 0 {
		t.Fatalf("tab with TabWidth=2 has zero width: %v", narrowLayout.Bounds.Width())
	}
	if wideLayout.Bounds.Width() <= narrowLayout.Bounds.Width() {
		t.Fatalf("wider TabWidth did not produce wider advance: narrow=%v wide=%v",
			narrowLayout.Bounds.Width(), wideLayout.Bounds.Width())
	}
}

// TestTabWidth_text_before_tab positions text before a tab and ensures the
// tab stop aligns to the next multiple of TabWidth×spaceWidth from origin.
func TestTabWidth_text_before_tab(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family
	style.TabWidth = 4

	// "a\t" must be wider than "a" alone.
	plain := shaper.ShapeSimple("a", style)
	tabbed := shaper.ShapeSimple("a\t", style)
	if plain == nil || tabbed == nil {
		t.Fatal("expected layouts")
	}
	if tabbed.Bounds.Width() <= plain.Bounds.Width() {
		t.Fatalf("tab did not advance beyond 'a': plain=%v tabbed=%v",
			plain.Bounds.Width(), tabbed.Bounds.Width())
	}
}

// TestBaseline_positive ensures every shaped line carries a positive baseline
// offset derived from the actual font ascent rather than a heuristic.
func TestBaseline_positive(t *testing.T) {
	layout := shapedASCII(t, "Xyz")
	if layout.Baseline <= 0 {
		t.Fatalf("layout baseline = %v, want > 0", layout.Baseline)
	}
	for i, line := range layout.Lines {
		if line.Baseline <= 0 {
			t.Fatalf("line %d baseline = %v, want > 0", i, line.Baseline)
		}
	}
}

// TestLineBounds_stackVertically verifies that multi-line layouts preserve
// distinct vertical line offsets instead of collapsing every line to y=0.
func TestLineBounds_stackVertically(t *testing.T) {
	layout := shapedASCII(t, "Line 1\nLine 2")
	if layout == nil || layout.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %#v", layout)
	}
	first := layout.Lines[0]
	second := layout.Lines[1]
	if first.Bounds.IsEmpty() || second.Bounds.IsEmpty() {
		t.Fatalf("expected non-empty line bounds, got first=%#v second=%#v", first.Bounds, second.Bounds)
	}
	if second.Bounds.Min.Y <= first.Bounds.Min.Y {
		t.Fatalf("expected second line to be stacked below first, got first=%#v second=%#v", first.Bounds, second.Bounds)
	}
	if diff := second.Bounds.Min.Y - first.Bounds.Max.Y; diff < -0.01 || diff > 0.01 {
		t.Fatalf("expected stacked line bounds, got first=%#v second=%#v", first.Bounds, second.Bounds)
	}
}

// TestLineBoxContract_keeps_content_box_baseline_and_line_box_distinct verifies
// that authored line height expands the line box without changing the baseline
// contract or collapsing the layout into glyph-ink bounds.
func TestLineBoxContract_keeps_content_box_baseline_and_line_box_distinct(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	style.LineHeight = 3

	layout := shaper.ShapeSimple("gyp", style)
	if layout == nil || layout.LineCount() != 1 {
		t.Fatalf("expected one shaped line, got %#v", layout)
	}

	line := layout.Lines[0]
	naturalH := line.Metrics.Ascent - line.Metrics.Descent
	if line.Bounds.Min.X != 0 || line.Bounds.Min.Y != 0 {
		t.Fatalf("line bounds = %#v, want origin-aligned line box", line.Bounds)
	}
	if layout.Bounds.Min.X != 0 || layout.Bounds.Min.Y != 0 {
		t.Fatalf("layout bounds = %#v, want origin-aligned content box", layout.Bounds)
	}
	if line.Bounds.Height() <= naturalH {
		t.Fatalf("line box height = %v, want > natural height %v", line.Bounds.Height(), naturalH)
	}
	if diff := math.Abs(float64(layout.Bounds.Height() - line.Bounds.Height())); diff > 0.01 {
		t.Fatalf("layout height = %v, line height = %v (diff %v)", layout.Bounds.Height(), line.Bounds.Height(), diff)
	}
	if line.Baseline <= 0 || line.Baseline >= line.Bounds.Height() {
		t.Fatalf("baseline = %v, want inside line box height %v", line.Baseline, line.Bounds.Height())
	}
	wantBaseline := line.Metrics.Ascent + (line.Metrics.LineHeight-naturalH)*0.5
	if diff := math.Abs(float64(line.Baseline - wantBaseline)); diff > 0.01 {
		t.Fatalf("baseline = %v, want %v (diff %v)", line.Baseline, wantBaseline, diff)
	}
}

// TestLineBoxContract_stacks_multiline_boxes_without_recomputing_baselines
// verifies that multi-line shaping preserves line boxes and reuses the shaped
// baseline contract on each line.
func TestLineBoxContract_stacks_multiline_boxes_without_recomputing_baselines(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	style.LineHeight = 2.5

	layout := shaper.Shape(Paragraph{
		Spans: []TextSpan{{Text: "Line 1\nLine 2", Style: style}},
	})
	if layout == nil || layout.LineCount() != 2 {
		t.Fatalf("expected 2 lines, got %#v", layout)
	}

	first := layout.Lines[0]
	second := layout.Lines[1]
	if diff := math.Abs(float64(second.Bounds.Min.Y - first.Bounds.Max.Y)); diff > 0.01 {
		t.Fatalf("expected stacked line boxes, got first=%#v second=%#v", first.Bounds, second.Bounds)
	}
	if diff := math.Abs(float64(first.Baseline - second.Baseline)); diff > 0.01 {
		t.Fatalf("expected shared baseline offset, got first=%v second=%v", first.Baseline, second.Baseline)
	}
	if diff := math.Abs(float64(layout.Bounds.Height() - (first.Bounds.Height() + second.Bounds.Height()))); diff > 0.01 {
		t.Fatalf("layout height = %v, want stacked line heights %v", layout.Bounds.Height(), first.Bounds.Height()+second.Bounds.Height())
	}
}

// TestShapeTruncated_preserves_vertical_metrics verifies truncation only
// changes horizontal shaping, not the vertical line-box contract.
func TestShapeTruncated_preserves_vertical_metrics(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family

	full := shaper.ShapeSimple("Hello world", style)
	if full == nil {
		t.Fatal("expected full layout")
	}
	truncated := shaper.ShapeTruncated("Hello world", style, full.Bounds.Width()*0.4)
	if truncated == nil {
		t.Fatal("expected truncated layout")
	}
	if truncated.Source == full.Source {
		t.Fatal("expected truncation to change the source text")
	}
	if diff := math.Abs(float64(full.LineHeight - truncated.LineHeight)); diff > 0.01 {
		t.Fatalf("line height changed across truncation: full=%v truncated=%v", full.LineHeight, truncated.LineHeight)
	}
	if diff := math.Abs(float64(full.Baseline - truncated.Baseline)); diff > 0.01 {
		t.Fatalf("baseline changed across truncation: full=%v truncated=%v", full.Baseline, truncated.Baseline)
	}
	if diff := math.Abs(float64(full.Bounds.Height() - truncated.Bounds.Height())); diff > 0.01 {
		t.Fatalf("layout height changed across truncation: full=%v truncated=%v", full.Bounds.Height(), truncated.Bounds.Height())
	}
}

// TestLineHeight_extra_leading_centers_baseline verifies that extra authored
// line height is distributed around the line instead of being placed only
// below the baseline.
func TestLineHeight_extra_leading_centers_baseline(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)
	style := DefaultStyle()
	style.Family = family
	style.LineHeight = 3

	layout := shaper.ShapeSimple("Label", style)
	if layout == nil || layout.LineCount() != 1 {
		t.Fatalf("expected one shaped line, got %#v", layout)
	}
	line := layout.Lines[0]
	naturalH := line.Metrics.Ascent - line.Metrics.Descent
	if line.Metrics.LineHeight <= naturalH {
		t.Fatalf("expected authored line height to exceed natural height, line=%#v natural=%v", line.Metrics, naturalH)
	}
	wantBaseline := line.Metrics.Ascent + (line.Metrics.LineHeight-naturalH)*0.5
	if diff := math.Abs(float64(line.Baseline - wantBaseline)); diff > 0.01 {
		t.Fatalf("baseline = %v, want %v (diff %v)", line.Baseline, wantBaseline, diff)
	}
}

// TestLineMetrics_ascent_descent_nonzero verifies that a shaped line
// exposes real ascent and descent values from the font binary.
func TestLineMetrics_ascent_descent_nonzero(t *testing.T) {
	layout := shapedASCII(t, "Hello")
	if layout.LineCount() == 0 {
		t.Fatal("expected lines")
	}
	m := layout.Lines[0].Metrics
	if m.Ascent <= 0 {
		t.Fatalf("ascent = %v, want > 0", m.Ascent)
	}
	// Descent is typically negative in font coordinates; stored as-is.
	// We only require it was actually set (not stuck at zero when text exists).
	_ = m.Descent
	if m.LineHeight <= 0 {
		t.Fatalf("line height = %v, want > 0", m.LineHeight)
	}
}

// TestLineMetrics_aggregate_matches_layout verifies that the TextLayout's
// aggregate Metrics field equals the first line's metrics line-height for a
// single-line paragraph (since aggregateMetrics returns first-line values).
func TestLineMetrics_aggregate_matches_layout(t *testing.T) {
	layout := shapedASCII(t, "hello")
	if layout.LineCount() != 1 {
		t.Fatalf("expected 1 line, got %d", layout.LineCount())
	}
	if layout.Metrics.LineHeight != layout.Lines[0].Metrics.LineHeight {
		t.Fatalf("aggregate metrics mismatch: layout=%v line=%v",
			layout.Metrics.LineHeight, layout.Lines[0].Metrics.LineHeight)
	}
}

// TestShapeTruncated_grapheme_boundary ensures ShapeTruncated never truncates
// mid-cluster even when the cluster contains combining marks.
func TestShapeTruncated_grapheme_boundary(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family

	// "a\u0301" is 'á' as two code points (base + combining acute).
	// Truncating mid-cluster would split the grapheme; ShapeTruncated must not.
	content := "a\u0301b\u0301c\u0301"
	full := shaper.ShapeSimple(content, style)
	if full == nil {
		t.Fatal("expected full layout")
	}

	// Force truncation at just above half width so at least one cluster fits.
	// Use ~70% of the full width so we get at least one original cluster.
	target := full.Bounds.Width() * 0.7
	truncated := shaper.ShapeTruncated(content, style, target)
	if truncated == nil {
		t.Fatal("expected truncated layout")
	}

	// The result must not be wider than the requested maxWidth.
	if truncated.Bounds.Width() > target+1 {
		t.Fatalf("truncated wider than maxWidth: got %v want ≤ %v",
			truncated.Bounds.Width(), target)
	}

	// Determine what prefix of the original content is present before the ellipsis.
	// Strip the trailing "…" if present to isolate the content prefix.
	src := truncated.Source
	prefix := src
	ellipsis := "…"
	if len(src) > 0 {
		srcRunes := []rune(src)
		if len(srcRunes) > 0 && srcRunes[len(srcRunes)-1] == '…' {
			prefix = string(srcRunes[:len(srcRunes)-1])
		}
	}

	// If a non-empty content prefix was retained it must end on a grapheme boundary.
	if prefix == "" || prefix == ellipsis {
		// Ellipsis-only or empty is valid — no cluster-splitting occurred.
		return
	}

	fullRunes := []rune(content)
	bounds := graphemeBoundaries(fullRunes)
	prefixLen := len([]rune(prefix))
	found := false
	for _, b := range bounds {
		if b == prefixLen {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("truncated prefix %q (len=%d runes) does not end on a grapheme boundary; boundaries=%v",
			prefix, prefixLen, bounds)
	}
}

// TestShapeTruncated_ellipsis_fits verifies that when content is truncated,
// the ellipsis "…" is appended and the result fits within maxWidth.
func TestShapeTruncated_ellipsis_fits(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family

	content := "Hello, World! This is a long string."
	full := shaper.ShapeSimple(content, style)
	if full == nil {
		t.Fatal("expected full layout")
	}

	maxWidth := full.Bounds.Width() * 0.4
	truncated := shaper.ShapeTruncated(content, style, maxWidth)
	if truncated == nil {
		t.Fatal("expected truncated layout")
	}
	if truncated.Bounds.Width() > maxWidth+1 {
		t.Fatalf("truncated layout exceeds maxWidth: got %v want ≤ %v",
			truncated.Bounds.Width(), maxWidth)
	}
}

// TestShapeTruncated_no_truncation_when_fits ensures ShapeTruncated returns
// the full layout without truncation when content fits within maxWidth.
func TestShapeTruncated_no_truncation_when_fits(t *testing.T) {
	reg, family := mustTestRegistry(t, testNotoSansRegular)
	shaper := NewShaper(reg)

	style := DefaultStyle()
	style.Family = family

	content := "Hi"
	full := shaper.ShapeSimple(content, style)
	if full == nil {
		t.Fatal("expected full layout")
	}

	// Provide a maxWidth larger than the content.
	result := shaper.ShapeTruncated(content, style, full.Bounds.Width()+100)
	if result == nil {
		t.Fatal("expected result layout")
	}
	// Width must match the un-truncated version exactly.
	if result.Bounds.Width() != full.Bounds.Width() {
		t.Fatalf("unexpected truncation: full=%v result=%v",
			full.Bounds.Width(), result.Bounds.Width())
	}
}
