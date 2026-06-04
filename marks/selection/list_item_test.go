package selection

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestListItemMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	item, rt, measureCtx := newListItemTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	item.Label = marks.Const("Sydney")
	item.Selected = marks.Const(true)

	facet.Attach(item, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := item.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 420, H: 120}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	item.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: item.LayoutRole().Parent,
		ChildGroup:  item.LayoutRole().Child,
	}, bounds)

	if got := item.AccessibilityRole(); got != "option" {
		t.Fatalf("accessibility role = %q, want option", got)
	}
	if got := item.AccessibleName(); got != "Sydney" {
		t.Fatalf("accessible name = %q, want Sydney", got)
	}
	if item.textRole.Layout == nil {
		t.Fatal("expected label text layout")
	}
	if item.cachedItemBounds.IsEmpty() || item.cachedSelectionBounds.IsEmpty() {
		t.Fatalf("expected item geometry, got item=%#v selection=%#v", item.cachedItemBounds, item.cachedSelectionBounds)
	}

	labelHit := item.HitRole().HitTest(gfx.Point{
		X: item.cachedLabelBounds.Min.X + 1,
		Y: item.cachedLabelBounds.Min.Y + 1,
	})
	if !labelHit.Hit || labelHit.MarkID != listItemMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", labelHit)
	}

	anchors := item.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := item.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
}

func newListItemTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*ListItem, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	item := NewListItem(marks.Const("Sydney"))
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return item, rt, resolved
}
