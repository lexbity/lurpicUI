package action

import (
	"math"
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

func TestSplitButtonMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	btn, rt := newSplitButtonFixture(t)

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 640}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H)
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: btn.layoutRole.Parent, ChildGroup: btn.layoutRole.Child}, bounds)

	if got := btn.AccessibilityRole(); got != "split_button" {
		t.Fatalf("accessibility role = %q, want split_button", got)
	}
	if got := btn.AccessibleName(); got != "Label" {
		t.Fatalf("accessible name = %q, want Label", got)
	}
	if btn.cachedPrimaryBounds.IsEmpty() || btn.cachedTriggerBounds.IsEmpty() || btn.cachedPrimaryLabel.IsEmpty() {
		t.Fatalf("expected control geometry, got primary=%#v trigger=%#v label=%#v", btn.cachedPrimaryBounds, btn.cachedTriggerBounds, btn.cachedPrimaryLabel)
	}

	labelHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedPrimaryLabel.Min.X + 1, Y: btn.cachedPrimaryLabel.Min.Y + 1})
	if !labelHit.Hit || labelHit.MarkID != splitButtonMarkIDPrimaryLabel {
		t.Fatalf("expected primary label hit, got %#v", labelHit)
	}
	triggerHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedTriggerBounds.Min.X + 1, Y: btn.cachedTriggerBounds.Min.Y + 1})
	if !triggerHit.Hit || triggerHit.MarkID != splitButtonMarkIDMenuTrigger {
		t.Fatalf("expected menu trigger hit, got %#v", triggerHit)
	}
	chevronHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedChevronBounds.Min.X + 1, Y: btn.cachedChevronBounds.Min.Y + 1})
	if !chevronHit.Hit || chevronHit.MarkID != splitButtonMarkIDChevron {
		t.Fatalf("expected chevron hit, got %#v", chevronHit)
	}
	primaryHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedPrimaryBounds.Min.X + 1, Y: btn.cachedPrimaryBounds.Min.Y + 1})
	if !primaryHit.Hit || primaryHit.MarkID != splitButtonMarkIDPrimaryButton {
		t.Fatalf("expected primary button hit, got %#v", primaryHit)
	}

	anchors := btn.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	btn.SetOpen(true)
	result = btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 720}})
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: btn.layoutRole.Parent, ChildGroup: btn.layoutRole.Child}, gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H))

	if btn.cachedMenuBounds.IsEmpty() || len(btn.cachedItemLayouts) == 0 {
		t.Fatalf("expected open menu geometry, got menu=%#v items=%d", btn.cachedMenuBounds, len(btn.cachedItemLayouts))
	}
	itemHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedItemLayouts[0].bounds.Min.X + 1, Y: btn.cachedItemLayouts[0].bounds.Min.Y + 1})
	if !itemHit.Hit || itemHit.MarkID != splitButtonMarkIDMenuItems {
		t.Fatalf("expected menu item hit, got %#v", itemHit)
	}
	surfaceHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedMenuBounds.Min.X + 2, Y: btn.cachedMenuBounds.Min.Y + 2})
	if !surfaceHit.Hit || (surfaceHit.MarkID != splitButtonMarkIDMenuItems && surfaceHit.MarkID != splitButtonMarkIDFloatingMenuSurface) {
		t.Fatalf("expected menu surface hit, got %#v", surfaceHit)
	}

	cmds := btn.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
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
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill/stroke commands")
	}
}

func TestSplitButtonPointerKeyboardAndDisabledBehavior(t *testing.T) {
	btn, rt := newSplitButtonFixture(t)
	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 720}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext()}, bounds)

	var activated string
	btn.Activated.Subscribe(func(key string) {
		activated = key
	})

	primaryCenter := gfx.Point{X: btn.cachedPrimaryBounds.Min.X + btn.cachedPrimaryBounds.Width()*0.5, Y: btn.cachedPrimaryBounds.Min.Y + btn.cachedPrimaryBounds.Height()*0.5}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: primaryCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected primary press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: primaryCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected primary release to be handled")
	}
	if activated != "create" {
		t.Fatalf("expected primary activation, got %q", activated)
	}

	triggerCenter := gfx.Point{X: btn.cachedTriggerBounds.Min.X + btn.cachedTriggerBounds.Width()*0.5, Y: btn.cachedTriggerBounds.Min.Y + btn.cachedTriggerBounds.Height()*0.5}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: triggerCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected trigger press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: triggerCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected trigger release to be handled")
	}
	if !btn.Open {
		t.Fatal("expected trigger click to open menu")
	}

	btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 720}})
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext()}, gfx.RectFromXYWH(0, 0, btn.layoutRole.MeasuredSize.W, btn.layoutRole.MeasuredSize.H))

	itemCenter := gfx.Point{X: btn.cachedItemLayouts[1].bounds.Min.X + btn.cachedItemLayouts[1].bounds.Width()*0.5, Y: btn.cachedItemLayouts[1].bounds.Min.Y + btn.cachedItemLayouts[1].bounds.Height()*0.5}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected item press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected item release to be handled")
	}
	if activated != "duplicate" {
		t.Fatalf("expected duplicate activation, got %q", activated)
	}
	if btn.Open {
		t.Fatal("expected menu to close after item activation")
	}

	btn.onFocusLost()
	btn.focusFromPointer = false
	btn.onFocusGained()
	if !btn.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to open menu")
	}
	if !btn.Open {
		t.Fatal("expected menu open after down key")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to navigate")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter press to be handled")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter release to be handled")
	}
	if activated == "" {
		t.Fatal("expected keyboard activation to emit a key")
	}

	btn.SetDisabled(true)
	if btn.focusRole.Focusable() {
		t.Fatal("expected disabled split button to be unfocusable")
	}
	if btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: primaryCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled split button to ignore pointer input")
	}
	if btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled split button to ignore keyboard input")
	}
}

func TestSplitButtonRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveSplitButtonRecipe(ctx)
	if !allSplitButtonFieldsPresent(slots) {
		t.Fatalf("split button slots contain zero values: %#v", slots)
	}
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 8 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func newSplitButtonFixture(t *testing.T) (*SplitButton, buttonRuntimeStub) {
	t.Helper()
	btn := NewSplitButton("Label", []SplitButtonItem{
		{Key: "create", Label: "Create new"},
		{Key: "duplicate", Label: "Duplicate"},
		{Key: "archive", Label: "Archive"},
	})
	btn.SetKey("create")
	btn.SetPrimaryIconRef("star")
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"star": mustSplitButtonIconAsset("star"),
		},
	}
	return btn, rt
}

func mustSplitButtonIconAsset(ref string) runtimepkg.IconAsset {
	circle := gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 9)
	star := splitButtonStarPath(gfx.Point{X: 12, Y: 12}, 5.8, 2.4)
	combined := combineIconPaths(circle, star)
	return runtimepkg.NewIconAsset(ref, 1, combined, gfx.RectFromXYWH(0, 0, 24, 24))
}

func splitButtonStarPath(center gfx.Point, outerRadius, innerRadius float32) gfx.Path {
	points := []gfx.Point{
		{X: center.X, Y: center.Y - outerRadius},
		{X: center.X - innerRadius*0.35, Y: center.Y - innerRadius*0.35},
		{X: center.X - outerRadius, Y: center.Y - innerRadius*0.10},
		{X: center.X - innerRadius*0.55, Y: center.Y + innerRadius*0.35},
		{X: center.X - outerRadius*0.62, Y: center.Y + outerRadius},
		{X: center.X, Y: center.Y + innerRadius*0.65},
		{X: center.X + outerRadius*0.62, Y: center.Y + outerRadius},
		{X: center.X + innerRadius*0.55, Y: center.Y + innerRadius*0.35},
		{X: center.X + outerRadius, Y: center.Y - innerRadius*0.10},
		{X: center.X + innerRadius*0.35, Y: center.Y - innerRadius*0.35},
	}
	return gfx.NewPath().
		MoveTo(points[0]).
		LineTo(points[1]).
		LineTo(points[2]).
		LineTo(points[3]).
		LineTo(points[4]).
		LineTo(points[5]).
		LineTo(points[6]).
		LineTo(points[7]).
		LineTo(points[8]).
		LineTo(points[9]).
		Close().
		Build()
}

func combineIconPaths(paths ...gfx.Path) gfx.Path {
	var out gfx.Path
	for _, path := range paths {
		out.Segments = append(out.Segments, path.Segments...)
	}
	return out
}

func TestSplitButtonGoldenDefault(t *testing.T) {
	AssertSplitButtonGolden(t, "default", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {})
}

func TestSplitButtonGoldenCompact(t *testing.T) {
	AssertSplitButtonGolden(t, "compact", defaultSplitButtonTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(btn *SplitButton) {})
}

func TestSplitButtonGoldenComfortable(t *testing.T) {
	AssertSplitButtonGolden(t, "comfortable", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {})
}

func TestSplitButtonGoldenDisabled(t *testing.T) {
	AssertSplitButtonGolden(t, "disabled", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {
		btn.SetDisabled(true)
	})
}

func TestSplitButtonGoldenHighContrast(t *testing.T) {
	AssertSplitButtonGolden(t, "high_contrast", highContrastSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {})
}

func TestSplitButtonGoldenHovered(t *testing.T) {
	AssertSplitButtonGolden(t, "hovered", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 2, Y: 2}})
	})
}

func TestSplitButtonGoldenPressed(t *testing.T) {
	AssertSplitButtonGolden(t, "pressed", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 2, Y: 2}, Button: platform.PointerLeft})
	})
}

func TestSplitButtonGoldenFocused(t *testing.T) {
	AssertSplitButtonGolden(t, "focused", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {
		btn.onFocusGained()
	})
}

func TestSplitButtonGoldenRTL(t *testing.T) {
	AssertSplitButtonGolden(t, "rtl", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(btn *SplitButton) {})
}

func TestSplitButtonGoldenOpen(t *testing.T) {
	AssertSplitButtonGolden(t, "open", defaultSplitButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *SplitButton) {
		btn.SetOpen(true)
	})
}

func AssertSplitButtonGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*SplitButton)) {
	t.Helper()
	btn, rt, measureCtx := newSplitButtonGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(btn)
	}
	renderSplitButtonToSurface(t, btn, rt, measureCtx, density, direction, name)
}

func renderSplitButtonToSurface(t *testing.T, btn *SplitButton, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 760}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: btn.layoutRole.Parent, ChildGroup: btn.layoutRole.Child}, bounds)

	cmds := btn.projectionRole.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "split_button_"+goldenName)
}

func newSplitButtonGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*SplitButton, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	btn := NewSplitButton("Label", []SplitButtonItem{
		{Key: "create", Label: "Create new"},
		{Key: "duplicate", Label: "Duplicate"},
		{Key: "archive", Label: "Archive"},
	})
	btn.SetKey("create")
	btn.SetPrimaryIconRef("star")
	btn.SetOpen(false)
	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"star": mustSplitButtonIconAsset("star"),
		},
	}
	return btn, rt, resolved
}

func defaultSplitButtonTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Primary = gfx.Color{R: 103.0 / 255.0, G: 80.0 / 255.0, B: 164.0 / 255.0, A: 1}
	tokens.Color.OnPrimary = gfx.Color{R: 1, G: 1, B: 1, A: 1}
	return tokens
}

func highContrastSplitButtonTokens() theme.Tokens {
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

func allSplitButtonFieldsPresent[T any](value T) bool {
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
