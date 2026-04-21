package basic

import (
	"os"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/projection"
	"codeburg.org/lexbit/lurpicui/text"
)

func TestText_projects_glyph_runs(t *testing.T) {
	setDefaultTextRegistry(t)
	txt := newTextMark("Hello world", false)
	cmds := renderMark(t, txt)
	if !containsCommandType(cmds, gfx.DrawGlyphRun{}) {
		t.Fatalf("expected glyph run command, got %#v", cmds)
	}
}

func TestText_hit_maps_to_text_position(t *testing.T) {
	setDefaultTextRegistry(t)
	txt := newTextMark("Hello world", false)
	pos := txt.HitPosition(gfx.Point{X: 10, Y: 10})
	if pos.Index < 0 {
		t.Fatalf("unexpected position: %+v", pos)
	}
}

func TestText_selection_geometry_available_when_selectable(t *testing.T) {
	setDefaultTextRegistry(t)
	txt := newTextMark("Hello world", true)
	out := renderFrame(t, txt)
	geom := out.SelectionGeometries[txt.Base().ID()]
	if geom == nil {
		t.Fatal("expected selection geometry")
	}
	if len(geom.SelectionRects) == 0 {
		t.Fatal("expected selection rects")
	}
}

func TestText_baseline_anchors_exported(t *testing.T) {
	setDefaultTextRegistry(t)
	txt := newTextMark("Hello world", false)
	anchors := txt.ExportAnchors(layout.AnchorExportContext{})
	for _, name := range []layout.AnchorID{"baseline-start", "baseline-end"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
}

func TestText_layout_cache_hit_on_static_frame(t *testing.T) {
	setDefaultTextRegistry(t)
	txt := newTextMark("Cache me", false)
	_ = renderMark(t, txt)
	_ = renderMark(t, txt)
	if txt.layoutBuilds != 1 {
		t.Fatalf("expected one layout build, got %d", txt.layoutBuilds)
	}
	if txt.layoutCacheHits == 0 {
		t.Fatal("expected a layout cache hit")
	}
}

func setDefaultTextRegistry(t *testing.T) {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("NewFontRegistry: %v", err)
	}
	for _, path := range []string{
		"/usr/share/fonts/liberation/LiberationSans-Regular.ttf",
		"/usr/share/fonts/Adwaita/AdwaitaSans-Regular.ttf",
	} {
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := reg.LoadFontFile(filepath.Clean(path)); err != nil {
			t.Fatalf("LoadFontFile %s: %v", path, err)
		}
		SetTextRegistry(reg)
		return
	}
	t.Skip("no usable font found for text mark tests")
}

func newTextMark(content string, selectable bool) *Text {
	return &Text{
		Paragraph: text.Paragraph{
			Spans: []text.TextSpan{{Text: content, Style: text.TextStyle{Family: "Liberation Sans", Size: 16, Weight: text.WeightRegular}}},
		},
		Selectable: selectable,
	}
}

func renderFrame(t *testing.T, mark facet.FacetImpl) *projection.FrameOutput {
	t.Helper()
	system := projection.NewSystem()
	return system.Run(mark, projection.FrameInfo{})
}
