package action

import (
	"bytes"
	"image"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/selection"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/templates"
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
		a.Disabled = marks.Const(true)
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
	card.GridColumns = marks.Const(1)
	card.GridRows = marks.Const(2)
	bar, _, _ := newActionBarGoldenFixture(t, tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	group := newButtonGroupGroupFixture(t)
	card.ChildrenContent = []structure.CardChild{
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
	}

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

	baselineSurface, baselineImage := renderActionBarPass(t, bar, rt, measureCtx, density, direction)
	// Exercise the local child graph multiple times using equivalent action
	// slices so layout cache reuse cannot perturb the rendered output.
	variants := []func(*ActionBar){
		func(*ActionBar) {},
		func(a *ActionBar) {
			a.Actions = marks.Const(cloneActionBarActions(a.Actions.Get()))
		},
		func(a *ActionBar) {
			a.Actions = marks.Const(equivalentActionBarActions(a.Actions.Get()))
		},
	}
	for i := 1; i < len(variants); i++ {
		variants[i](bar)
		_, got := renderActionBarPass(t, bar, rt, measureCtx, density, direction)
		if !actionBarImagesEqual(baselineImage, got) {
			t.Fatalf("action bar golden layout pass %d changed rendered output", i+1)
		}
	}

	testkit.AssertGolden(t, baselineSurface, "action_bar_"+goldenName)
}

func renderActionBarPass(t *testing.T, bar *ActionBar, rt actionBarRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection) (*testkit.MemorySurface, *image.RGBA) {
	t.Helper()
	result := bar.Layout.Measure(facet.MeasureContext{
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
	bar.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: bar.Layout.Parent,
		ChildGroup:  bar.Layout.Child,
	}, bounds)

	cmds := bar.Projection.Project(facet.ProjectionContext{
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
	return surface, surface.Capture()
}

func actionBarImagesEqual(a, b *image.RGBA) bool {
	if a == nil || b == nil {
		return a == b
	}
	if !a.Bounds().Eq(b.Bounds()) {
		return false
	}
	return bytes.Equal(a.Pix, b.Pix)
}

func cloneActionBarActions(actions []ActionBarAction) []ActionBarAction {
	if len(actions) == 0 {
		return nil
	}
	out := make([]ActionBarAction, len(actions))
	copy(out, actions)
	return out
}

func equivalentActionBarActions(actions []ActionBarAction) []ActionBarAction {
	out := cloneActionBarActions(actions)
	for i := range out {
		if out[i].Variant == 0 {
			out[i].Variant = defaultActionBarButtonVariant()
		}
	}
	return out
}

func defaultActionBarButtonVariant() uiinput.ButtonVariant {
	return uiinput.ButtonText
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
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastActionBarTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
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

func toThemeTokens(t templates.Tokens) theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Background = t.Color.Background
	tokens.Color.Surface = t.Color.Surface
	tokens.Color.SurfaceVariant = t.Color.SurfaceVariant
	tokens.Color.SurfaceInverse = t.Color.SurfaceInverse
	tokens.Color.OnBackground = t.Color.OnBackground
	tokens.Color.OnSurface = t.Color.OnSurface
	tokens.Color.OnSurfaceVariant = t.Color.OnSurfaceVariant
	tokens.Color.Primary = t.Color.Primary
	tokens.Color.OnPrimary = t.Color.OnPrimary
	tokens.Color.Secondary = t.Color.Secondary
	tokens.Color.OnSecondary = t.Color.OnSecondary
	tokens.Color.Error = t.Color.Error
	tokens.Color.Warning = t.Color.Warning
	tokens.Color.Success = t.Color.Success
	tokens.Color.OnError = t.Color.OnError
	tokens.Color.DisabledOpacity = t.Color.DisabledOpacity
	tokens.Color.HoverLighten = t.Color.HoverOpacity
	tokens.Color.PressedDarken = t.Color.PressedOpacity
	tokens.Color.SelectedOverlay = t.Color.SelectionOpacity

	tokens.Typography.DisplayLarge = t.Typography.DisplayLarge
	tokens.Typography.DisplayMedium = t.Typography.DisplayMedium
	tokens.Typography.DisplaySmall = t.Typography.DisplaySmall
	tokens.Typography.HeadlineLarge = t.Typography.HeadlineLarge
	tokens.Typography.HeadlineMedium = t.Typography.HeadlineMedium
	tokens.Typography.HeadlineSmall = t.Typography.HeadlineSmall
	tokens.Typography.TitleLarge = t.Typography.TitleLarge
	tokens.Typography.TitleMedium = t.Typography.TitleMedium
	tokens.Typography.TitleSmall = t.Typography.TitleSmall
	tokens.Typography.LabelLarge = t.Typography.LabelLarge
	tokens.Typography.LabelMedium = t.Typography.LabelMedium
	tokens.Typography.LabelSmall = t.Typography.LabelSmall
	tokens.Typography.BodyLarge = t.Typography.BodyLarge
	tokens.Typography.BodyMedium = t.Typography.BodyMedium
	tokens.Typography.BodySmall = t.Typography.BodySmall

	tokens.Radius.None = t.Shape.RadiusNone
	tokens.Radius.XS = t.Shape.RadiusXS
	tokens.Radius.SM = t.Shape.RadiusSM
	tokens.Radius.MD = t.Shape.RadiusMD
	tokens.Radius.LG = t.Shape.RadiusLG
	tokens.Radius.Full = t.Shape.RadiusFull

	return tokens
}
