package action

import (
	"math"
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

func TestPopupPaletteMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	palette, rt, resolved := newPopupPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := palette.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 960}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	palette.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: resolved, ParentGroup: palette.Layout.Parent, ChildGroup: palette.Layout.Child}, bounds)

	if got := palette.AccessibilityRole(); got != "toolbar" {
		t.Fatalf("accessibility role = %q, want toolbar", got)
	}
	if got := palette.AccessibleName(); got != "Popup palette" {
		t.Fatalf("accessible name = %q, want Popup palette", got)
	}
	if palette.cachedSurfaceBounds.IsEmpty() || len(palette.cachedToolItemBounds) == 0 || len(palette.cachedHistoryBounds) == 0 {
		t.Fatalf("expected arranged geometry, got surface=%#v tools=%d history=%d", palette.cachedSurfaceBounds, len(palette.cachedToolItemBounds), len(palette.cachedHistoryBounds))
	}

	toolHit := palette.Hit.HitTest(gfx.Point{X: palette.cachedToolItemBounds[0].Min.X + 1, Y: palette.cachedToolItemBounds[0].Min.Y + 1})
	if !toolHit.Hit || toolHit.MarkID != popupPaletteMarkIDToolItems {
		t.Fatalf("expected tool-item hit, got %#v", toolHit)
	}
	groupHit := palette.Hit.HitTest(gfx.Point{X: palette.cachedMirrorBounds.Min.X + 1, Y: palette.cachedMirrorBounds.Min.Y + 1})
	if !groupHit.Hit || groupHit.MarkID != popupPaletteMarkIDToolGroup {
		t.Fatalf("expected tool-group hit, got %#v", groupHit)
	}
	surfaceHit := palette.Hit.HitTest(gfx.Point{X: palette.cachedSurfaceBounds.Min.X + 2, Y: palette.cachedSurfaceBounds.Min.Y + 2})
	if !surfaceHit.Hit || surfaceHit.MarkID != popupPaletteMarkIDSurface {
		t.Fatalf("expected surface hit, got %#v", surfaceHit)
	}

	anchors := palette.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := palette.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawGlyphRun, sawFillPath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath, gfx.StrokePath:
			sawFillPath = true
		}
	}
	if !sawFillPath {
		t.Fatal("expected fill/stroke commands")
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
}

func TestPopupPalettePointerKeyboardAndDisabledBehavior(t *testing.T) {
	palette, rt, resolved := newPopupPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	// Test enabled palette behavior first.
	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := palette.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 960}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	palette.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: resolved, ParentGroup: palette.Layout.Parent, ChildGroup: palette.Layout.Child}, bounds)

	var activated string
	palette.Activated.Subscribe(func(key string) {
		activated = key
	})

	toolCenter := centerOfRect(palette.cachedToolItemBounds[0])
	if !palette.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: toolCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected tool press to be handled")
	}
	if !palette.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: toolCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected tool release to be handled")
	}
	if activated == "" {
		t.Fatal("expected activation signal from tool release")
	}

	if !palette.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if palette.SelectedIndex.Get() < 0 {
		t.Fatal("expected keyboard navigation to select a tool")
	}
	if !palette.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key to be handled")
	}

	zoomCenter := centerOfRect(palette.cachedSliderThumb)
	if !palette.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: zoomCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected zoom press to be handled")
	}
	if !palette.onPointer(facet.PointerEvent{Kind: platform.PointerMove, Position: gfx.Point{X: palette.cachedZoomBounds.Max.X, Y: zoomCenter.Y}, Button: platform.PointerLeft}) {
		t.Fatal("expected zoom drag to be handled")
	}
	if !palette.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: palette.cachedZoomBounds.Max.X, Y: zoomCenter.Y}, Button: platform.PointerLeft}) {
		t.Fatal("expected zoom release to be handled")
	}
	if palette.Zoom.Get() <= 1 {
		t.Fatalf("expected zoom change, got %v", palette.Zoom.Get())
	}

	palette.onFocusLost()
	palette.focusFromPointer = false
	palette.onFocusGained()
	if !palette.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}

	// Now create a disabled palette and verify it rejects all input.
	palette2, rt2, resolved2 := newPopupPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	palette2.Disabled = marks.Const(true)
	facet.Attach(palette2, facet.AttachContext{Runtime: rt2, Theme: resolved2})
	result2 := palette2.Layout.Measure(facet.MeasureContext{
		Runtime:          rt2,
		Theme:            resolved2,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 960}})
	palette2.Layout.Arrange(facet.ArrangeContext{Runtime: rt2, Theme: resolved2, ParentGroup: palette2.Layout.Parent, ChildGroup: palette2.Layout.Child}, gfx.RectFromXYWH(0, 0, result2.Size.W, result2.Size.H))

	if palette2.Focus.Focusable() {
		t.Fatal("expected disabled palette to be unfocusable")
	}
	var activated2 string
	palette2.Activated.Subscribe(func(key string) {
		activated2 = key
	})
	if palette2.cachedToolItemBounds != nil && len(palette2.cachedToolItemBounds) > 0 {
		center := centerOfRect(palette2.cachedToolItemBounds[0])
		palette2.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft})
		palette2.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft})
	}
	if activated2 != "" {
		t.Fatal("expected disabled palette not to activate on pointer input")
	}
	if palette2.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled palette to ignore keyboard input")
	}
}

func TestPopupPaletteRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolvePopupPaletteRecipe(ctx)
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	for _, name := range []string{"Root", "PaletteSurface", "ToolItems", "ToolGroup", "AnchorArrow", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if !allPopupPaletteFieldsPresent(slots) {
		t.Fatalf("popup palette slots contain zero values: %#v", slots)
	}
}

func TestPopupPaletteGoldenDefault(t *testing.T) {
	AssertPopupPaletteGolden(t, "default", defaultPopupPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*PopupPalette) {})
}

func TestPopupPaletteGoldenCompact(t *testing.T) {
	AssertPopupPaletteGolden(t, "compact", defaultPopupPaletteTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(*PopupPalette) {})
}

func TestPopupPaletteGoldenDisabled(t *testing.T) {
	AssertPopupPaletteGolden(t, "disabled", defaultPopupPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *PopupPalette) {
		p.Disabled = marks.Const(true)
	})
}

func TestPopupPaletteGoldenRTL(t *testing.T) {
	AssertPopupPaletteGolden(t, "rtl", defaultPopupPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(*PopupPalette) {})
}

func AssertPopupPaletteGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*PopupPalette)) {
	t.Helper()
	palette, rt, measureCtx := newPopupPaletteGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(palette)
	}
	renderPopupPaletteToSurface(t, palette, rt, measureCtx, density, direction, name)
}

func renderPopupPaletteToSurface(t *testing.T, palette *PopupPalette, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := palette.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 960}})
	bounds := gfx.RectFromXYWH(0, 0, 1280, 960)
	if result.Size.W > 0 && result.Size.H > 0 {
		bounds = gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	}
	palette.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: palette.Layout.Parent, ChildGroup: palette.Layout.Child}, bounds)

	cmds := palette.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil {
		cmds = &gfx.CommandList{}
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
	testkit.AssertGolden(t, surface, "popup_palette_"+goldenName)
}

func newPopupPaletteFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*PopupPalette, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	palette := NewPopupPalette("Popup palette", []PopupPaletteTool{
		{Key: "brush", Label: "Brush", IconRef: "brush", Color: gfx.ColorFromRGBA8(235, 95, 70, 255)},
		{Key: "eraser", Label: "Eraser", IconRef: "eraser", Color: gfx.ColorFromRGBA8(230, 230, 230, 255)},
		{Key: "dropper", Label: "Dropper", IconRef: "dropper", Color: gfx.ColorFromRGBA8(100, 170, 230, 255)},
		{Key: "shape", Label: "Shape", IconRef: "shape", Color: gfx.ColorFromRGBA8(130, 220, 100, 255)},
		{Key: "fill", Label: "Fill", IconRef: "fill", Color: gfx.ColorFromRGBA8(255, 206, 66, 255)},
		{Key: "lasso", Label: "Lasso", IconRef: "lasso", Color: gfx.ColorFromRGBA8(168, 125, 255, 255)},
		{Key: "slice", Label: "Slice", IconRef: "slice", Color: gfx.ColorFromRGBA8(255, 145, 87, 255)},
		{Key: "hand", Label: "Hand", IconRef: "hand", Color: gfx.ColorFromRGBA8(128, 128, 128, 255)},
	})
	palette.History = []gfx.Color{
		gfx.ColorFromRGBA8(255, 90, 90, 255),
		gfx.ColorFromRGBA8(255, 179, 71, 255),
		gfx.ColorFromRGBA8(103, 208, 118, 255),
	}
	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     testkit.TestFontRegistry(t),
		icons: buttonIconResolverStub{
			"brush":         mustMenuButtonIconAsset("brush"),
			"eraser":        mustMenuButtonIconAsset("eraser"),
			"dropper":       mustMenuButtonIconAsset("dropper"),
			"shape":         mustMenuButtonIconAsset("shape"),
			"fill":          mustMenuButtonIconAsset("fill"),
			"lasso":         mustMenuButtonIconAsset("lasso"),
			"slice":         mustMenuButtonIconAsset("slice"),
			"hand":          mustMenuButtonIconAsset("hand"),
			"mirror":        mustMenuButtonIconAsset("mirror"),
			"canvas":        mustMenuButtonIconAsset("canvas"),
			"history-clear": mustMenuButtonIconAsset("history-clear"),
			"chevron-up":    mustMenuButtonIconAsset("chevron-up"),
		},
	}
	return palette, rt, resolved
}

func newPopupPaletteGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*PopupPalette, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	return newPopupPaletteFixture(t, tokens, density, direction)
}

func defaultPopupPaletteTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.SurfaceInverse = gfx.ColorFromRGBA8(54, 54, 60, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(220, 224, 235, 255)
	tokens.Color.Primary = gfx.ColorFromRGBA8(74, 112, 245, 255)
	tokens.Color.Secondary = gfx.ColorFromRGBA8(109, 120, 159, 255)
	tokens.Color.SecondaryVariant = gfx.ColorFromRGBA8(73, 83, 118, 255)
	tokens.Color.Error = gfx.ColorFromRGBA8(232, 79, 75, 255)
	tokens.Color.Warning = gfx.ColorFromRGBA8(240, 173, 54, 255)
	tokens.Color.Success = gfx.ColorFromRGBA8(84, 188, 116, 255)
	tokens.Color.Info = gfx.ColorFromRGBA8(95, 169, 255, 255)
	tokens.Color.DataPalette = []gfx.Color{
		gfx.ColorFromRGBA8(255, 90, 100, 255),
		gfx.ColorFromRGBA8(253, 164, 56, 255),
		gfx.ColorFromRGBA8(166, 230, 71, 255),
		gfx.ColorFromRGBA8(78, 227, 126, 255),
		gfx.ColorFromRGBA8(42, 214, 208, 255),
		gfx.ColorFromRGBA8(86, 161, 255, 255),
		gfx.ColorFromRGBA8(146, 107, 255, 255),
		gfx.ColorFromRGBA8(236, 113, 190, 255),
	}
	return tokens
}

func allPopupPaletteFieldsPresent[T any](value T) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			return false
		}
	}
	return true
}
