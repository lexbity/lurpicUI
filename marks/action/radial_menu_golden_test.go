package action

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	input "codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRadialMenuGoldenDefault(t *testing.T) {
	AssertRadialMenuGolden(t, "default", defaultRadialMenuTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*RadialMenu) {})
}

func TestRadialMenuGoldenCompact(t *testing.T) {
	AssertRadialMenuGolden(t, "compact", defaultRadialMenuTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(*RadialMenu) {})
}

func TestRadialMenuGoldenHighContrast(t *testing.T) {
	AssertRadialMenuGolden(t, "high_contrast", highContrastRadialMenuTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*RadialMenu) {})
}

func TestRadialMenuGoldenFocused(t *testing.T) {
	AssertRadialMenuGolden(t, "focused", defaultRadialMenuTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(menu *RadialMenu) {
		menu.onFocusGained()
	})
}

func TestRadialMenuGoldenRTL(t *testing.T) {
	AssertRadialMenuGolden(t, "rtl", defaultRadialMenuTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(*RadialMenu) {})
}

func AssertRadialMenuGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*RadialMenu)) {
	t.Helper()
	menu, rt, measureCtx := newRadialMenuGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(menu)
	}
	renderRadialMenuToSurface(t, menu, rt, measureCtx, density, direction, name)
}

func renderRadialMenuToSurface(t *testing.T, menu *RadialMenu, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(menu, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := menu.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1440}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	menu.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: menu.layoutRole.Parent, ChildGroup: menu.layoutRole.Child}, bounds)

	cmds := menu.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}

	surface := testkit.NewMemorySurface(int(math.Ceil(float64(bounds.Width()))), int(math.Ceil(float64(bounds.Height()))))
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      bounds,
			Opacity:     1,
			CommandHash: 1,
			Commands:    gfx.CommandList{Commands: cmds.Commands},
		}},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "radial_menu_"+goldenName)
}

func newRadialMenuGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*RadialMenu, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)

	center := input.NewColorPicker("Palette")
	center.SetColor(gfx.Color{R: 0.17, G: 0.56, B: 0.93, A: 1})

	split := NewSplitButton("Brush", []SplitButtonItem{
		{Key: "soft", Label: "Soft"},
		{Key: "hard", Label: "Hard"},
	})
	split.SetKey("soft")
	split.SetPrimaryIconRef("star")

	group := NewActionGroup("Canvas", []ActionGroupAction{
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "copy", Label: "Copy", IconRef: "copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
	})

	menu := NewMenuButton("Tools", []MenuButtonEntry{
		{Kind: MenuButtonEntrySection, Label: "Actions"},
		{Key: "mirror", Label: "Mirror", IconRef: "mirror"},
		{Key: "canvas", Label: "Canvas", IconRef: "canvas"},
	})
	menu.SetTriggerIconRef("more")

	radial := NewRadialMenu("Radial menu", center, []RadialChild{
		{Child: split, Placement: facet.RadialPlacement{Angle: -math.Pi / 2, RadiusTrack: 128}},
		{Child: group, Placement: facet.RadialPlacement{Angle: math.Pi / 6, RadiusTrack: 128}},
		{Child: menu, Placement: facet.RadialPlacement{Angle: 5 * math.Pi / 6, RadiusTrack: 128}},
	})
	radial.DefaultTrackRadius = 128

	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"star":   mustSplitButtonIconAsset("star"),
			"more":   mustActionGroupIconAsset("more"),
			"edit":   mustActionGroupIconAsset("edit"),
			"copy":   mustActionGroupIconAsset("copy"),
			"delete": mustActionGroupIconAsset("delete"),
			"mirror": mustMenuButtonIconAsset("mirror"),
			"canvas": mustMenuButtonIconAsset("canvas"),
		},
	}
	return radial, rt, resolved
}

func defaultRadialMenuTokens() theme.Tokens {
	return theme.DefaultTokens()
}

func highContrastRadialMenuTokens() theme.Tokens {
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
