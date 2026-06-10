package feedback

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
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type alertRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s alertRuntimeStub) Schedule(j job.AnyJob)  {}
func (s alertRuntimeStub) CancelJob(id job.JobID) {}
func (s alertRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s alertRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s alertRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s alertRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestAlertMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	alert := newAlertFixture()
	tokens := alertTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(alert, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := alert.Layout.Measure(facet.MeasureContext{
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
	alert.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: alert.Layout.Parent,
		ChildGroup:  alert.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, bounds)
	if got := alert.AccessibilityRole(); got != "alert" {
		t.Fatalf("accessibility role = %q, want alert", got)
	}
	if got := alert.AccessibleName(); got != "Network unavailable The system will retry automatically." {
		t.Fatalf("accessible name = %q", got)
	}
	if len(alert.Children()) != 5 {
		t.Fatalf("expected five child facets, got %d", len(alert.Children()))
	}
	if alert.cachedIconBounds.IsEmpty() || alert.cachedTitleBounds.IsEmpty() || alert.cachedMessageBounds.IsEmpty() {
		t.Fatalf("expected arranged core geometry, got icon=%#v title=%#v message=%#v", alert.cachedIconBounds, alert.cachedTitleBounds, alert.cachedMessageBounds)
	}
	anchors := alert.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "alert_surface", "icon", "title", "message", "action", "close_button"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := alert.Projection.Project(facet.ProjectionContext{
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

func TestAlertInteractionsEmitActionAndDismiss(t *testing.T) {
	alert := newAlertFixture()
	tokens := alertTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(alert, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = alert.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 240}})
	alert.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: alert.Layout.Parent,
		ChildGroup:  alert.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, gfx.RectFromXYWH(0, 0, alert.Layout.MeasuredSize.W, alert.Layout.MeasuredSize.H))

	var actioned, dismissed int
	alert.Actioned.Subscribe(func(signal.Unit) { actioned++ })
	alert.Dismissed.Subscribe(func(signal.Unit) { dismissed++ })

	actionButton := alert.cachedActionButton
	if actionButton == nil {
		t.Fatal("expected action button")
	}
	actionBounds := alert.cachedActionBounds
	if !actionButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action press to be handled")
	}
	if !actionButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action release to be handled")
	}
	if actioned != 1 {
		t.Fatalf("expected one action emission, got %d", actioned)
	}

	if !alert.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to be handled")
	}
	if dismissed != 1 {
		t.Fatalf("expected one dismiss emission, got %d", dismissed)
	}
}

func TestAlertRecipe_exposes_expected_slots(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uifeedback.ResolveAlertRecipe(ctx, uifeedback.AlertDefault)
	if report.Family != "uifeedback" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	for _, name := range []string{"Root", "AlertSurface", "Icon", "Title", "Message", "Action", "CloseButton"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if slots.Root.Base.Opacity != 0 {
		t.Fatal("expected transparent root slot")
	}
	if slots.AlertSurface.Base.Fills == nil {
		t.Fatal("expected alert surface fill")
	}
}

func TestAlertGoldenDefault(t *testing.T) {
	AssertAlertGolden(t, "default", alertTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *Alert) {})
}

func TestAlertGoldenCompact(t *testing.T) {
	AssertAlertGolden(t, "compact", alertTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(a *Alert) {})
}

func TestAlertGoldenDisabled(t *testing.T) {
	AssertAlertGolden(t, "disabled", alertTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *Alert) {
		a.Disabled = marks.Const(true)
	})
}

func TestAlertGoldenHighContrast(t *testing.T) {
	AssertAlertGolden(t, "high_contrast", alertHighContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *Alert) {})
}

func TestAlertGoldenHovered(t *testing.T) {
	AssertAlertGolden(t, "hovered", alertTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *Alert) {
		a.hovered = true
	})
}

func TestAlertGoldenPressed(t *testing.T) {
	AssertAlertGolden(t, "pressed", alertTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(a *Alert) {
		a.pressed = true
	})
}

func TestAlertGoldenRTL(t *testing.T) {
	AssertAlertGolden(t, "rtl", alertTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(a *Alert) {})
}

func AssertAlertGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Alert)) {
	t.Helper()
	alert := newAlertFixture()
	if mutate != nil {
		mutate(alert)
	}
	rt := alertRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	resolved := alertResolvedContext(tokens, density, direction)
	facet.Attach(alert, facet.AttachContext{Runtime: rt, Theme: resolved})
	canvas := gfx.RectFromXYWH(16, 16, 360, 240)
	_ = alert.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	alert.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: alert.Layout.Parent,
		ChildGroup:  alert.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, canvas)
	cmds := alert.Projection.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(392, 272)
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
	testkit.AssertGolden(t, surface, "alert_"+name)
}

func newAlertFixture() *Alert {
	alert := NewAlert("Network unavailable", "The system will retry automatically.")
	alert.ActionLabel = marks.Const("Retry")
	alert.CloseButtonLabel = marks.Const("Dismiss")
	return alert
}

func alertTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func alertHighContrastTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func alertResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
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
