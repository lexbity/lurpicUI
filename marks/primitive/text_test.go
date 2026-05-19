package primitive

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
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
	mark := NewText("Hello world")
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustTextRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{
		MaxSize: gfx.Size{W: 500, H: 200},
	})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	mark.layoutRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(10, 20, result.Size.W, result.Size.H))
	mark.SetSelection(text.TextRange{Start: 0, End: 5})
	geom := mark.textRole.CollectSelectionGeometry()
	if geom == nil || len(geom.SelectionRects) == 0 {
		t.Fatalf("expected selection geometry, got %#v", geom)
	}

	cmds := mark.projectionRole.Project(facet.ProjectionContext{
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
}

func TestTextMark_disabled_uses_disabled_color(t *testing.T) {
	mark := NewText("Disabled")
	mark.SetDisabled(true)
	rt := textRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustTextRegistry(t),
	}

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	_ = mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{
		MaxSize: gfx.Size{W: 500, H: 200},
	})
	mark.layoutRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, mark.layoutRole.MeasuredSize.W, mark.layoutRole.MeasuredSize.H))
	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       gfx.RectFromXYWH(0, 0, mark.layoutRole.MeasuredSize.W, mark.layoutRole.MeasuredSize.H),
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
		fonts:     mustTextRegistry(t),
	}

	wrap := NewText("Hello world")
	wrap.SetOverflow(TextOverflowWrap)
	wrap.SetMaxWidth(40)
	_ = wrap.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 40, H: 200}})
	if wrap.cachedLayout == nil || wrap.cachedLayout.LineCount() < 2 {
		t.Fatalf("expected wrapped layout, got %#v", wrap.cachedLayout)
	}

	trunc := NewText("Hello world")
	trunc.SetOverflow(TextOverflowTruncate)
	trunc.SetMaxWidth(40)
	_ = trunc.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 40, H: 200}})
	if trunc.cachedLayout == nil || trunc.cachedLayout.Bounds.Width() > 40.5 {
		t.Fatalf("expected truncated width <= 40, got %#v", trunc.cachedLayout)
	}
}

func mustTextRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func mustReadFont(t *testing.T, rel string) []byte {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	path := filepath.Join(string(bytesTrim(out)), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font %q: %v", path, err)
	}
	return data
}

func bytesTrim(in []byte) []byte {
	for len(in) > 0 {
		switch in[len(in)-1] {
		case '\n', '\r', '\t', ' ':
			in = in[:len(in)-1]
		default:
			return in
		}
	}
	return in
}
