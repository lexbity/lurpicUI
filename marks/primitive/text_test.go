package primitive

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

type textRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s textRuntimeStub) Schedule(j job.AnyJob)  {}
func (s textRuntimeStub) CancelJob(id job.JobID) {}
func (s textRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s textRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s textRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s textRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestTextMark_projects_layout_anchors_and_selection(t *testing.T) {
	mark := NewText(marks.Const("Hello world"))
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{
		MaxSize: gfx.Size{W: 500, H: 200},
	})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	mark.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(10, 20, result.Size.W, result.Size.H))
	mark.textRole.Selection = text.TextRange{Start: 0, End: 5}
	geom := mark.textRole.CollectSelectionGeometry()
	if geom == nil || len(geom.SelectionRects) == 0 {
		t.Fatalf("expected selection geometry, got %#v", geom)
	}

	cmds := mark.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       gfx.RectFromXYWH(10, 20, result.Size.W, result.Size.H),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected glyph commands")
	}
	if _, ok := cmds.Commands[0].(gfx.DrawGlyphRun); !ok {
		t.Fatalf("expected DrawGlyphRun, got %T", cmds.Commands[0])
	}

	anchors := mark.ExportAnchors(layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: gfx.RectFromXYWH(10, 20, result.Size.W, result.Size.H)},
	})
	if len(anchors) == 0 {
		t.Fatal("expected exported anchors")
	}
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	wantBaseline := gfx.Point{X: 10, Y: 20 + mark.cachedLayout.Baseline}
	gotBaseline, ok := anchors["baseline"]
	if !ok {
		t.Fatal("expected baseline anchor")
	}
	if diff := math.Abs(float64(gotBaseline.X - wantBaseline.X)); diff > 0.01 {
		t.Fatalf("baseline anchor x = %v, want %v (diff %v)", gotBaseline.X, wantBaseline.X, diff)
	}
	if diff := math.Abs(float64(gotBaseline.Y - wantBaseline.Y)); diff > 0.01 {
		t.Fatalf("baseline anchor y = %v, want %v (diff %v)", gotBaseline.Y, wantBaseline.Y, diff)
	}
}

func TestTextMark_disabled_uses_disabled_color(t *testing.T) {
	mark := NewText(marks.Const("Disabled"))
	mark.Disabled = marks.Const(true)
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	_ = mark.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{
		MaxSize: gfx.Size{W: 500, H: 200},
	})
	mark.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, mark.Layout.MeasuredSize.W, mark.Layout.MeasuredSize.H))
	cmds := mark.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       gfx.RectFromXYWH(0, 0, mark.Layout.MeasuredSize.W, mark.Layout.MeasuredSize.H),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projection commands")
	}
	draw, ok := cmds.Commands[0].(gfx.DrawGlyphRun)
	if !ok {
		t.Fatalf("expected DrawGlyphRun, got %T", cmds.Commands[0])
	}
	if draw.Brush.Color.A >= 1 {
		t.Fatalf("expected disabled opacity, got %#v", draw.Brush.Color)
	}
}

func TestTextMark_wrap_and_truncate(t *testing.T) {
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	wrap := NewText(marks.Const("Hello world"))
	wrap.Overflow = marks.Const(TextOverflowWrap)
	wrap.MaxWidth = marks.Const[float32](40)
	_ = wrap.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 40, H: 200}})
	if wrap.cachedLayout == nil || wrap.cachedLayout.LineCount() < 2 {
		t.Fatalf("expected wrapped layout, got %#v", wrap.cachedLayout)
	}

	trunc := NewText(marks.Const("Hello world"))
	trunc.Overflow = marks.Const(TextOverflowTruncate)
	trunc.MaxWidth = marks.Const[float32](40)
	_ = trunc.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 40, H: 200}})
	if trunc.cachedLayout == nil || trunc.cachedLayout.Bounds.Width() > 40.5 {
		t.Fatalf("expected truncated width <= 40, got %#v", trunc.cachedLayout)
	}
}

func TestTextLayoutCommands_use_shaped_line_box_and_baseline(t *testing.T) {
	mark := NewText(marks.Const("AaBbCcGgJjQq"))
	mark.Overflow = marks.Const(TextOverflowWrap)
	mark.Alignment = marks.Const(text.AlignCenter)
	mark.MaxWidth = marks.Const[float32](300)

	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 300, H: 120}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	arranged := gfx.RectFromXYWH(17, 23, result.Size.W, result.Size.H)
	mark.Layout.Arrange(facet.ArrangeContext{}, arranged)

	cmds := mark.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       arranged,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projection commands")
	}
	draw, ok := cmds.Commands[0].(gfx.DrawGlyphRun)
	if !ok {
		t.Fatalf("expected DrawGlyphRun, got %T", cmds.Commands[0])
	}
	if mark.cachedLayout == nil || len(mark.cachedLayout.Lines) == 0 || len(mark.cachedLayout.Lines[0].Runs) == 0 {
		t.Fatalf("expected cached shaped layout, got %#v", mark.cachedLayout)
	}
	line := mark.cachedLayout.Lines[0]
	run := line.Runs[0]
	wantX := arranged.Min.X + mark.cachedLayout.Bounds.Min.X + line.Bounds.Min.X + run.Bounds.Min.X
	wantY := arranged.Min.Y + mark.cachedLayout.Bounds.Min.Y + line.Bounds.Min.Y + line.Baseline + run.Bounds.Min.Y
	if draw.Origin.X != wantX || draw.Origin.Y != wantY {
		t.Fatalf("origin = %#v, want (%v,%v)", draw.Origin, wantX, wantY)
	}
}

func TestTextLayoutCommands_stack_multiline_runs_with_baseline_offsets(t *testing.T) {
	mark := NewText(marks.Const("First line\nSecond line"))
	mark.Overflow = marks.Const(TextOverflowWrap)
	mark.MaxWidth = marks.Const[float32](300)

	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 300, H: 200}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	mark.Layout.Arrange(facet.ArrangeContext{}, bounds)
	cmds := mark.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) < 2 {
		t.Fatalf("expected multiline projection commands, got %#v", cmds)
	}
	first, ok := cmds.Commands[0].(gfx.DrawGlyphRun)
	if !ok {
		t.Fatalf("expected first command to be DrawGlyphRun, got %T", cmds.Commands[0])
	}
	second, ok := cmds.Commands[1].(gfx.DrawGlyphRun)
	if !ok {
		t.Fatalf("expected second command to be DrawGlyphRun, got %T", cmds.Commands[1])
	}
	if second.Origin.Y <= first.Origin.Y {
		t.Fatalf("expected second line below first, first=%#v second=%#v", first.Origin, second.Origin)
	}
}


