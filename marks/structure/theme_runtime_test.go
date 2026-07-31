package structure

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// listThemeRuntimeStub extends listRuntimeStub with RootStyleContext and FacetByID
// so that child marks (Text, ListItem) can resolve style tokens through the
// theme.NearestStyleContext path during projection.
type listThemeRuntimeStub struct {
	listRuntimeStub
	rootStyle any
	fonts     *text.FontRegistry
}

func (s listThemeRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s listThemeRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s listThemeRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestListResolveThemeTokensInProjection(t *testing.T) {
	sentinel := gfx.Color{R: 18.0 / 255.0, G: 52.0 / 255.0, B: 86.0 / 255.0, A: 1}

	tok := theme.DefaultTokens()
	tok.Color.OnSurface = sentinel

	rootStyle := theme.NewRootStyleContext(nil, tok, nil)
	fonts := testkit.TestFontRegistry(t)
	rt := listThemeRuntimeStub{
		listRuntimeStub: listRuntimeStub{
			cardRuntimeStub: cardRuntimeStub{fonts: fonts},
			icons:           map[string]runtimepkg.IconAsset{},
		},
		rootStyle: rootStyle,
		fonts:     fonts,
	}

	list := NewList("Theme test list", []ListEntry{
		{Key: "row-1", Label: "First item"},
		{Key: "row-2", Label: "Second item"},
		{Key: "row-3", Label: "Third item"},
	})
	list.SectionHeader = marks.Const("Section")
	ctx := theme.DefaultResolvedContext()

	facet.Attach(list, facet.AttachContext{Runtime: rt, Theme: ctx})

	measureCtx := facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}
	result := list.Layout.Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 600}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	list.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: list.Layout.Parent,
		ChildGroup:  list.Layout.Child,
	}, bounds)

	cmds := list.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands from List")
	}

	found := false
	for _, cmd := range cmds.Commands {
		if dg, ok := cmd.(gfx.DrawGlyphRun); ok {
			if dg.Brush.Color == sentinel {
				found = true
				break
			}
		}
	}
	if !found {
		t.Fatal("projected commands do not contain sentinel ColorText — token not resolved at runtime in List")
	}
}
