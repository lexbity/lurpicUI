package structure

import (
	"reflect"
	"testing"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type cardRuntimeStub struct {
	fonts *text.FontRegistry
}

func (s cardRuntimeStub) Schedule(j job.AnyJob)                                              {}
func (s cardRuntimeStub) CancelJob(id job.JobID)                                             {}
func (s cardRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (s cardRuntimeStub) FontRegistry() *text.FontRegistry                                   { return s.fonts }

func TestCardMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	card := newCardFixture()
	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := cardResolvedContext(cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := card.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 720}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	card.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: card.Layout.Parent,
		ChildGroup:  card.Layout.Child,
	}, bounds)

	if got := card.AccessibilityRole(); got != "group" {
		t.Fatalf("accessibility role = %q, want group", got)
	}
	if got := card.AccessibleName(); got != "Default size card" {
		t.Fatalf("accessible name = %q, want Default size card", got)
	}
	if card.LayoutMode.Get() != CardLayoutGrid {
		t.Fatalf("layout mode = %v, want grid", card.LayoutMode)
	}
	if len(card.Children()) < 5 {
		t.Fatalf("expected composed child facets, got %d", len(card.Children()))
	}
	if card.cachedBounds.IsEmpty() || len(card.cachedChildBounds) == 0 {
		t.Fatalf("expected arranged geometry, got card=%#v children=%#v", card.cachedBounds, card.cachedChildBounds)
	}

	anchors := card.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	expectBoundsAnchors(t, anchors, bounds)

	cmds := card.Projection.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawGlyphRun, sawFillPath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected text glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected surface/fill commands")
	}
}

func TestCardLayoutModes(t *testing.T) {
	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := cardResolvedContext(cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	for _, mode := range []CardLayoutMode{CardLayoutGrid, CardLayoutVertical, CardLayoutHorizontal} {
		card := newCardFixture()
		card.LayoutMode = marks.Const(mode)
		facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
		result := card.Layout.Measure(facet.MeasureContext{
			Runtime:          rt,
			Theme:            ctx,
			ContentScale:     1,
			Density:          facet.DensityID(theme.DensityIDComfortable),
			WritingDirection: facet.WritingDirectionLTR,
		}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 720}})
		if result.Size.W <= 0 || result.Size.H <= 0 {
			t.Fatalf("mode %v produced invalid size %#v", mode, result.Size)
		}
	}
}

func TestCardGoldenDefault(t *testing.T) {
	AssertCardGolden(t, "default", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenCompact(t *testing.T) {
	AssertCardGolden(t, "compact", cardTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenDisabled(t *testing.T) {
	AssertCardGolden(t, "disabled", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {
		c.Disabled = marks.Const(true)
	})
}

func TestCardGoldenHighContrast(t *testing.T) {
	AssertCardGolden(t, "high_contrast", highContrastCardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
}

func TestCardGoldenRTL(t *testing.T) {
	// Capture LTR surface for diff comparison; "default" golden already exists.
	ltr := AssertCardGolden(t, "default", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Card) {})
	rtl := AssertCardGolden(t, "rtl", cardTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(c *Card) {})
	testkit.AssertGoldenPair(t, ltr, rtl, "card")
}

func AssertCardGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Card)) *testkit.MemorySurface {
	t.Helper()
	card := newCardFixture()
	if mutate != nil {
		mutate(card)
	}
	rt := cardRuntimeStub{fonts: testkit.TestFontRegistry(t)}
	ctx := cardResolvedContext(tokens, density, direction)
	facet.Attach(card, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(12, 12, 616, 336)
	_ = card.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bounds := canvas
	card.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: ctx, ParentGroup: card.Layout.Parent, ChildGroup: card.Layout.Child}, bounds)
	cmds := card.Projection.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(640, 360)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      bounds,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "card_"+name)
	return surface
}

func newCardFixture() *Card {
	card := NewCard("Default size card")
	card.GridColumns = marks.Const(3)
	card.GridRows = marks.Const(3)
	card.ChildrenContent = []CardChild{
		{
			Key:    "icon",
			Facet:  primitive.NewIcon(cardTestHeaderIcon()),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 0, ColSpan: 1, RowSpan: 1},
			MarkID: cardMarkIDFirstChild,
		},
		{
			Key:    "title",
			Facet:  primitive.NewText(marks.Const("Default size card")),
			Grid:   facet.GridPlacement{ColStart: 1, RowStart: 0, ColSpan: 2, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 1,
		},
		{
			Key:    "action",
			Facet:  action.NewButton(marks.Const("Action"), marks.Const(uiinput.ButtonOutlined)),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 1, ColSpan: 1, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 2,
		},
		{
			Key:    "field",
			Facet:  input.NewTextField("Notes", uiinput.TextInputOutlined),
			Grid:   facet.GridPlacement{ColStart: 1, RowStart: 1, ColSpan: 2, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 3,
		},
		{
			Key:    "body",
			Facet:  primitive.NewText(marks.Const("Card content\nCard content\nCard content")),
			Grid:   facet.GridPlacement{ColStart: 0, RowStart: 2, ColSpan: 3, RowSpan: 1},
			MarkID: cardMarkIDFirstChild + 4,
		},
	}
	return card
}

func cardTestHeaderIcon() primitive.IconSource {
	return primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="10"/></svg>`)
}

func cardTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastCardTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func cardResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
	ctx := theme.DefaultResolvedContext()
	rv := reflect.ValueOf(&ctx).Elem()
	field := rv.FieldByName("defaultContext")
	fieldCopy := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	tokensField := fieldCopy.FieldByName("tokens")
	reflect.NewAt(tokensField.Type(), unsafe.Pointer(tokensField.UnsafeAddr())).Elem().Set(reflect.ValueOf(tokens))
	ctx = ctx.WithDensity(theme.DefaultDensityScale(density, tokens))
	ctx = ctx.WithWritingDirection(direction)
	return ctx
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
