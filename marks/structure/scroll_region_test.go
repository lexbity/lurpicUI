package structure

import (
	"strings"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestScrollRegionGoldenVerticalOverflow(t *testing.T) {
	AssertScrollRegionGolden(t, "vertical_overflow", ScrollDirectionVertical, func(sr *ScrollRegion) {
		sr.SetChildren(scrollRegionVerticalChildren())
	}, func(sr *ScrollRegion) {
		if sr.cachedVerticalTrack.IsEmpty() || sr.cachedVerticalThumb.IsEmpty() {
			t.Fatalf("expected vertical scrollbar geometry, got track=%#v thumb=%#v", sr.cachedVerticalTrack, sr.cachedVerticalThumb)
		}
		if !sr.cachedHorizontalTrack.IsEmpty() || !sr.cachedHorizontalThumb.IsEmpty() {
			t.Fatalf("did not expect horizontal scrollbar geometry, got track=%#v thumb=%#v", sr.cachedHorizontalTrack, sr.cachedHorizontalThumb)
		}
	})
}

func TestScrollRegionGoldenHorizontalOverflow(t *testing.T) {
	AssertScrollRegionGolden(t, "horizontal_overflow", ScrollDirectionHorizontal, func(sr *ScrollRegion) {
		sr.SetChildren(scrollRegionHorizontalChildren())
	}, func(sr *ScrollRegion) {
		if sr.cachedHorizontalTrack.IsEmpty() || sr.cachedHorizontalThumb.IsEmpty() {
			t.Fatalf("expected horizontal scrollbar geometry, got track=%#v thumb=%#v", sr.cachedHorizontalTrack, sr.cachedHorizontalThumb)
		}
		if !sr.cachedVerticalTrack.IsEmpty() || !sr.cachedVerticalThumb.IsEmpty() {
			t.Fatalf("did not expect vertical scrollbar geometry, got track=%#v thumb=%#v", sr.cachedVerticalTrack, sr.cachedVerticalThumb)
		}
	})
}

func AssertScrollRegionGolden(t *testing.T, name string, direction ScrollDirection, mutate func(*ScrollRegion), assert func(*ScrollRegion)) {
	t.Helper()
	sr := NewScrollRegion("Scrollable region")
	sr.Direction = marks.Const(direction)
	sr.Gap = marks.Const[float32](10)
	if mutate != nil {
		mutate(sr)
	}
	rt := cardRuntimeStub{fonts: mustCardFontRegistry(t)}
	ctx := listResolvedContext(listTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sr, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	_ = sr.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	sr.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: sr.Layout.Parent,
		ChildGroup:  sr.Layout.Child,
	}, canvas)
	if assert != nil {
		assert(sr)
	}
	cmds := sr.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       canvas,
		ContentScale: 1,
	})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(272, 192)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      canvas,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "scroll_region_"+name)
}

func scrollRegionVerticalChildren() []ScrollRegionChild {
	out := make([]ScrollRegionChild, 0, 12)
	for i := 0; i < 12; i++ {
		out = append(out, ScrollRegionChild{
			Facet:     primitive.NewText(marks.Const("Row " + string(rune('A'+i%26)))),
			MarkID:    facet.MarkID(i + 10),
			Placement: facet.Placement{Mode: facet.PlacementFree},
		})
	}
	return out
}

func scrollRegionHorizontalChildren() []ScrollRegionChild {
	return []ScrollRegionChild{
		{
			Facet:     primitive.NewText(marks.Const(strings.Repeat("Wide content segment ", 8))),
			MarkID:    facet.MarkID(10),
			Placement: facet.Placement{Mode: facet.PlacementFree},
		},
	}
}
