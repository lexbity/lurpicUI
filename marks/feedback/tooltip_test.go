package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

func TestTooltipMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	tt := newTooltipFixture()
	tokens := tooltipTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}

	facet.Attach(tt, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := tt.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 240}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	tt.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: tt.layoutRole.Parent,
		ChildGroup:  tt.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, bounds)

	if got := tt.AccessibilityRole(); got != "tooltip" {
		t.Fatalf("accessibility role = %q, want tooltip", got)
	}
	if got := tt.AccessibleName(); got != "Press and hold for more details." {
		t.Fatalf("accessible name = %q", got)
	}
	if len(tt.Children()) != 1 {
		t.Fatalf("expected one child facet, got %d", len(tt.Children()))
	}
	if tt.cachedContentFacet == nil {
		t.Fatal("expected cached text facet")
	}
	if got := tt.cachedContentFacet.Alignment; got != text.AlignCenter {
		t.Fatalf("content alignment = %v, want center", got)
	}
	if textBounds := tt.cachedContentFacet.Base().LayoutRole().MeasuredSize; tt.cachedContentBounds.Width() <= textBounds.W {
		t.Fatalf("expected tooltip content bounds wider than text, bounds=%v text=%v", tt.cachedContentBounds, textBounds)
	}
	if tt.cachedContentBounds.IsEmpty() || tt.cachedSurfaceBounds.IsEmpty() || tt.cachedArrowBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got content=%#v surface=%#v arrow=%#v", tt.cachedContentBounds, tt.cachedSurfaceBounds, tt.cachedArrowBounds)
	}
	anchors := tt.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := tt.projectionRole.Project(facet.ProjectionContext{
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
		case gfx.FillPath:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill path commands")
	}
}

func TestTooltipDismissalAndOpenState(t *testing.T) {
	tt := newTooltipFixture()
	tokens := tooltipTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}

	facet.Attach(tt, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = tt.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 240}})
	tt.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: tt.layoutRole.Parent,
		ChildGroup:  tt.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, gfx.RectFromXYWH(0, 0, tt.layoutRole.MeasuredSize.W, tt.layoutRole.MeasuredSize.H))

	var dismissed int
	tt.Dismissed.Subscribe(func(signal.Unit) { dismissed++ })
	if !tt.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to dismiss tooltip")
	}
	if dismissed != 1 {
		t.Fatalf("expected one dismiss emission, got %d", dismissed)
	}
	if tt.Open {
		t.Fatal("expected tooltip to close after escape")
	}
	if tt.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: gfx.RectFromXYWH(0, 0, 1, 1), ContentScale: 1}) != nil {
		t.Fatal("expected closed tooltip to stop projecting commands")
	}
}

func TestTooltipRecipe_exposes_expected_slots(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uifeedback.ResolveTooltipRecipe(ctx, uifeedback.TooltipOpen)
	if report.Family != "uifeedback" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("open") {
		t.Fatalf("variant = %q", report.Variant)
	}
	for _, name := range []string{"Root", "TooltipSurface", "Content", "AnchorArrow"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.TooltipSurface.Base.Fills == nil {
		t.Fatal("expected tooltip surface fill")
	}
}

func TestTooltipGoldenDefault(t *testing.T) {
	AssertTooltipGolden(t, "default", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {})
}

func TestTooltipGoldenCompact(t *testing.T) {
	AssertTooltipGolden(t, "compact", tooltipTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(tt *Tooltip) {})
}

func TestTooltipGoldenComfortable(t *testing.T) {
	AssertTooltipGolden(t, "comfortable", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {})
}

func TestTooltipGoldenDisabled(t *testing.T) {
	AssertTooltipGolden(t, "disabled", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {
		tt.SetDisabled(true)
	})
}

func TestTooltipGoldenHighContrast(t *testing.T) {
	AssertTooltipGolden(t, "high_contrast", tooltipHighContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {})
}

func TestTooltipGoldenHovered(t *testing.T) {
	AssertTooltipGolden(t, "hovered", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {
		tt.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 8, Y: 8}})
	})
}

func TestTooltipGoldenPressed(t *testing.T) {
	AssertTooltipGolden(t, "pressed", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {
		tt.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 8, Y: 8}, Button: platform.PointerLeft})
	})
}

func TestTooltipGoldenRTL(t *testing.T) {
	AssertTooltipGolden(t, "rtl", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(tt *Tooltip) {})
}

func TestTooltipGoldenOpen(t *testing.T) {
	AssertTooltipGolden(t, "open", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {
		tt.SetOpen(true)
	})
}

func TestTooltipGoldenDismissed(t *testing.T) {
	AssertTooltipGolden(t, "dismissed", tooltipTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(tt *Tooltip) {
		tt.SetOpen(false)
	})
}

func AssertTooltipGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Tooltip)) {
	t.Helper()
	tt := newTooltipFixture()
	if mutate != nil {
		mutate(tt)
	}
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}
	resolved := alertResolvedContext(tokens, density, direction)
	facet.Attach(tt, facet.AttachContext{Runtime: rt, Theme: resolved})
	canvas := gfx.RectFromXYWH(16, 16, 360, 240)
	_ = tt.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	tt.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: tt.layoutRole.Parent,
		ChildGroup:  tt.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, canvas)
	cmds := tt.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	surface := testkit.NewMemorySurface(392, 272)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frameCmds := gfx.CommandList{}
	if cmds != nil {
		frameCmds = *cmds
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      canvas,
			Opacity:     1,
			Commands:    frameCmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "tooltip_"+name)
}

func newTooltipFixture() *Tooltip {
	tt := NewTooltip("Press and hold for more details.")
	tt.SetPlacement(facet.AnchorPlacement{Side: facet.AnchorAbove})
	return tt
}

func tooltipTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func tooltipHighContrastTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}
