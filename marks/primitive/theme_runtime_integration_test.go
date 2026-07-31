package primitive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestTextResolveThemeTokensInProjection(t *testing.T) {
	sentinel := gfx.Color{R: 18.0 / 255.0, G: 52.0 / 255.0, B: 86.0 / 255.0, A: 1}

	tok := theme.DefaultTokens()
	tok.Color.OnSurface = sentinel

	rootStyle := theme.NewRootStyleContext(nil, tok, nil)
	fonts := testkit.TestFontRegistry(t)
	rt := textRuntimeStub{
		rootStyle: rootStyle,
		fonts:     fonts,
	}

	mark := NewText(marks.Const("Theme token test"))
	mark.Foreground = marks.Const(theme.ColorText)

	ctx := theme.DefaultResolvedContext()
	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: ctx})

	result := mark.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 100}})

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
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
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
		t.Fatal("projected commands do not contain sentinel ColorText — token not resolved at runtime")
	}
}
