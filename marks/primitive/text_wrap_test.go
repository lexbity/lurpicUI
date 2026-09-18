package primitive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

func newTextMeasureCtx(t *testing.T) (facet.MeasureContext, theme.ResolvedContext) {
	t.Helper()
	ctx := theme.DefaultResolvedContext()
	return facet.MeasureContext{
		Runtime:          textRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, ctx
}

func measureText(t *testing.T, mark *Text, maxWidth float32) gfx.Size {
	t.Helper()
	measureCtx, _ := newTextMeasureCtx(t)
	result := mark.Layout.Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: maxWidth, H: 1000}})
	return result.Size
}

// TestTextMultiLine_wrapsByDefault pins RX-1 FR-9: a MultiLine text wraps by
// default when width-constrained (TextOverflowWrap), so a long paragraph in a
// 280dp pane grows vertically instead of overflowing or clipping.
func TestTextMultiLine_wrapsByDefault(t *testing.T) {
	mark := NewText(marks.Const("lurpic UI is a reactive, render-neutral interface framework whose facets compose marks into surfaces; this sentence is long enough to wrap several times at 280dp."))
	mark.MultiLine = marks.Const(true)

	single := measureText(t, mark, 560)
	wrapped := measureText(t, mark, 280)

	if wrapped.W > 280+0.5 {
		t.Fatalf("wrapped text did not constrain to 280dp: W=%v", wrapped.W)
	}
	if wrapped.H <= single.H {
		t.Fatalf("wrapped text did not grow vertically: single H=%v wrapped H=%v", single.H, wrapped.H)
	}
	// The shaped layout must have multiple lines.
	mark.Layout.Measure(facet.MeasureContext{
		Runtime:          textRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:            theme.DefaultResolvedContext(),
		ContentScale:     1,
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 280, H: 1000}})
	layout := mark.textRole.Layout
	if layout == nil || len(layout.Lines) < 2 {
		t.Fatalf("expected multiple shaped lines, got %d", func() int {
			if layout == nil {
				return 0
			}
			return len(layout.Lines)
		}())
	}
}

// TestTextSingleLine_keepsDeclaredOverflow pins the other half of FR-9: a
// single-line text keeps its declared overflow (Clip by default), so a long
// single line stays on one line and clips at the arranged bounds.
func TestTextSingleLine_keepsDeclaredOverflow(t *testing.T) {
	mark := NewText(marks.Const("a single very long line of text that does not wrap"))
	if mark.MultiLine.Get() {
		t.Fatal("NewText should default MultiLine to false")
	}
	size := measureText(t, mark, 120)
	// Single-line shaping is unconstrained horizontally (ShapeSimple).
	if size.W <= 120 {
		t.Fatalf("single-line text was constrained to %v, want its natural width", size.W)
	}
	if size.H <= 0 {
		t.Fatalf("single-line text has no height: %v", size.H)
	}
}

// TestTextMultiLine_explicitTruncateOptsOut pins the FR-9 opt-out: a MultiLine
// text that explicitly declares TextOverflowTruncate truncates rather than
// wraps (ellipsize/truncate is the explicit opt-out).
func TestTextMultiLine_explicitTruncateOptsOut(t *testing.T) {
	mark := NewText(marks.Const("a very long sentence that must be truncated to a single line and ellipsized"))
	mark.MultiLine = marks.Const(true)
	mark.Overflow = marks.Const(TextOverflowTruncate)

	mark.Layout.Measure(facet.MeasureContext{
		Runtime:          textRuntimeStub{fonts: testkit.TestFontRegistry(t)},
		Theme:            theme.DefaultResolvedContext(),
		ContentScale:     1,
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 120, H: 100}})
	layout := mark.textRole.Layout
	if layout == nil || len(layout.Lines) != 1 {
		t.Fatalf("truncate opted-out of wrapping: expected 1 line, got %d", func() int {
			if layout == nil {
				return 0
			}
			return len(layout.Lines)
		}())
	}
}
