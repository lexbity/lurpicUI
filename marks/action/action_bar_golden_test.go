package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestActionBarGoldenDefault(t *testing.T) {
	AssertActionBarGolden(t, "default", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {})
}

func TestActionBarGoldenCompact(t *testing.T) {
	AssertActionBarGolden(t, "compact", defaultActionBarTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(a *ActionBar) {})
}

func TestActionBarGoldenComfortable(t *testing.T) {
	AssertActionBarGolden(t, "comfortable", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {})
}

func TestActionBarGoldenDisabled(t *testing.T) {
	AssertActionBarGolden(t, "disabled", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {
		a.SetDisabled(true)
	})
}

func TestActionBarGoldenHighContrast(t *testing.T) {
	AssertActionBarGolden(t, "high_contrast", highContrastActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {})
}

func TestActionBarGoldenHovered(t *testing.T) {
	AssertActionBarGolden(t, "hovered", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {
		a.hovered = true
	})
}

func TestActionBarGoldenPressed(t *testing.T) {
	AssertActionBarGolden(t, "pressed", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {
		a.pressed = true
	})
}

func TestActionBarGoldenFocused(t *testing.T) {
	AssertActionBarGolden(t, "focused", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *ActionBar) {
		a.onFocusGained()
	})
}

func TestActionBarGoldenRTL(t *testing.T) {
	AssertActionBarGolden(t, "rtl", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(a *ActionBar) {})
}

func TestActionBarComposesWithOtherGroupMarks(t *testing.T) {
	tokens := defaultActionBarTokens()
	rt := actionBarRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustButtonTextRegistry(t),
		icons: actionBarIconResolverStub{
			"close": mustActionBarIconAsset("close"),
			"edit":  mustActionBarIconAsset("edit"),
		},
	}
	card := structure.NewCard("")
	card.SetGrid(1, 2)
	bar, _, _ := newActionBarGoldenFixture(t, tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	group := newButtonGroupGroupFixture(t)
	card.SetChildren([]structure.CardChild{
		{
			Key:    "action_bar",
			Facet:  bar.Base(),
			MarkID: 100,
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1},
		},
		{
			Key:    "button_group",
			Facet:  group.Base(),
			MarkID: 101,
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1},
		},
	})

	cardFacet := card.Base()
	cardLayout := cardFacet.LayoutRole()
	facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := cardLayout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            theme.DefaultResolvedContext(),
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 480}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable composite size, got %#v", result.Size)
	}
	cardLayout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       theme.DefaultResolvedContext(),
		ParentGroup: cardLayout.Parent,
		ChildGroup:  cardLayout.Child,
	}, gfx.RectFromXYWH(24, 24, result.Size.W, result.Size.H))
	cmds := cardFacet.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       gfx.RectFromXYWH(24, 24, result.Size.W, result.Size.H),
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected composite commands")
	}
	if len(card.Children()) != 2 {
		t.Fatalf("expected 2 child facets, got %d", len(card.Children()))
	}
	if cardLayout.ArrangedBounds.IsEmpty() {
		t.Fatal("expected composed parent to arrange")
	}
}

func AssertActionBarGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*ActionBar)) {
	t.Helper()
	bar, rt, measureCtx := newActionBarGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(bar)
	}
	renderActionBarToSurface(t, bar, rt, measureCtx, density, direction, name)
}

func renderActionBarToSurface(t *testing.T, bar *ActionBar, rt actionBarRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(bar, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bar.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 320}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	surfaceW := 1920
	surfaceH := 528
	x := maxFloat(0, float32(surfaceW)-result.Size.W) * 0.5
	y := maxFloat(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)
	bar.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: bar.layoutRole.Parent,
		ChildGroup:  bar.layoutRole.Child,
	}, bounds)

	cmds := bar.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}

	surface := testkit.NewMemorySurface(surfaceW, surfaceH)
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    gfx.CommandList{Commands: cmds.Commands},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "action_bar_"+goldenName)
}

func newActionBarGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*ActionBar, actionBarRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	bar := NewActionBar("224 selected", []ActionBarAction{
		{Key: "close", AccessibleLabel: "Close", IconRef: "close"},
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "copy", Label: "Copy", IconRef: "copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
	})
	rt := actionBarRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: actionBarIconResolverStub{
			"close":  mustActionBarIconAsset("close"),
			"edit":   mustActionBarIconAsset("edit"),
			"copy":   mustActionBarIconAsset("copy"),
			"delete": mustActionBarIconAsset("delete"),
		},
	}
	return bar, rt, resolved
}

func newButtonGroupGroupFixture(t *testing.T) *selection.ButtonGroup {
	t.Helper()
	bg := selection.NewButtonGroup("Choices", []selection.ButtonGroupOption{
		{Key: "one", Label: "One"},
		{Key: "two", Label: "Two"},
	})
	return bg
}

func defaultActionBarTokens() theme.Tokens {
	return theme.DefaultTokens()
}

func highContrastActionBarTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Background = gfx.Color{R: 0, G: 0, B: 0, A: 1}
	tokens.Color.Surface = gfx.Color{R: 0, G: 0, B: 0, A: 1}
	tokens.Color.SurfaceVariant = gfx.Color{R: 0.12, G: 0.12, B: 0.12, A: 1}
	tokens.Color.OnBackground = gfx.Color{R: 1, G: 1, B: 1, A: 1}
	tokens.Color.OnSurface = gfx.Color{R: 1, G: 1, B: 1, A: 1}
	tokens.Color.OnSurfaceVariant = gfx.Color{R: 1, G: 1, B: 1, A: 1}
	tokens.Color.Primary = gfx.Color{R: 1, G: 1, B: 0, A: 1}
	tokens.Color.OnPrimary = gfx.Color{R: 0, G: 0, B: 0, A: 1}
	return tokens
}

func actionBarDensityToTemplateMode(density theme.DensityID) theme.DensityMode {
	switch density {
	case theme.DensityIDCompact:
		return theme.DensityCompact
	case theme.DensityIDTouch:
		return theme.DensityTouch
	default:
		return theme.DensityComfortable
	}
}
