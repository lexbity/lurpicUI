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
	"github.com/go-text/typesetting/di"
)

func TestMenuButtonMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	btn, rt := newMenuButtonFixture(t)

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

	if got := btn.AccessibilityRole(); got != "button_with_menu" {
		t.Fatalf("accessibility role = %q, want button_with_menu", got)
	}
	if got := btn.AccessibleName(); got != "Settings" {
		t.Fatalf("accessible name = %q, want Settings", got)
	}
	if btn.cachedTriggerBounds.IsEmpty() || btn.cachedChevronBounds.IsEmpty() || btn.cachedTriggerLabelBounds.IsEmpty() {
		t.Fatalf("expected trigger geometry, got trigger=%#v label=%#v chevron=%#v", btn.cachedTriggerBounds, btn.cachedTriggerLabelBounds, btn.cachedChevronBounds)
	}

	triggerLabelHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedTriggerLabelBounds.Min.X + 1, Y: btn.cachedTriggerLabelBounds.Min.Y + 1})
	if !triggerLabelHit.Hit || triggerLabelHit.MarkID != menuButtonMarkIDTriggerLabel {
		t.Fatalf("expected trigger label hit, got %#v", triggerLabelHit)
	}
	chevronHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedChevronBounds.Min.X + 1, Y: btn.cachedChevronBounds.Min.Y + 1})
	if !chevronHit.Hit || chevronHit.MarkID != menuButtonMarkIDChevron {
		t.Fatalf("expected chevron hit, got %#v", chevronHit)
	}
	iconHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedTriggerIconBounds.Min.X + 1, Y: btn.cachedTriggerIconBounds.Min.Y + 1})
	if !iconHit.Hit || iconHit.MarkID != menuButtonMarkIDTriggerIcon {
		t.Fatalf("expected trigger icon hit, got %#v", iconHit)
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

	if btn.cachedMenuBounds.IsEmpty() || len(btn.cachedEntryLayouts) == 0 {
		t.Fatalf("expected open menu geometry, got menu=%#v entries=%d", btn.cachedMenuBounds, len(btn.cachedEntryLayouts))
	}
	itemHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedEntryLayouts[2].bounds.Min.X + 1, Y: btn.cachedEntryLayouts[2].bounds.Min.Y + 1})
	if !itemHit.Hit || itemHit.MarkID != menuButtonMarkIDMenuItems {
		t.Fatalf("expected menu item hit, got %#v", itemHit)
	}
	surfaceHit := btn.hitRole.HitTest(gfx.Point{X: btn.cachedMenuBounds.Min.X + 2, Y: btn.cachedMenuBounds.Min.Y + 2})
	if !surfaceHit.Hit || (surfaceHit.MarkID != menuButtonMarkIDMenuItems && surfaceHit.MarkID != menuButtonMarkIDFloatingMenuSurface) {
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

func TestMenuButtonPointerKeyboardAndDisabledBehavior(t *testing.T) {
	btn, rt := newMenuButtonFixture(t)
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

	itemCenter := gfx.Point{X: btn.cachedEntryLayouts[5].bounds.Min.X + btn.cachedEntryLayouts[5].bounds.Width()*0.5, Y: btn.cachedEntryLayouts[5].bounds.Min.Y + btn.cachedEntryLayouts[5].bounds.Height()*0.5}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected item press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected item release to be handled")
	}
	if activated != "rename" {
		t.Fatalf("expected rename activation, got %q", activated)
	}
	if btn.Open {
		t.Fatal("expected menu to close after activation")
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
		t.Fatal("expected disabled menu button to be unfocusable")
	}
	if btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: triggerCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled menu button to ignore pointer input")
	}
	if btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled menu button to ignore keyboard input")
	}
}

func TestMenuButtonRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveMenuButtonRecipe(ctx)
	if !allMenuButtonFieldsPresent(slots) {
		t.Fatalf("menu button slots contain zero values: %#v", slots)
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

func newMenuButtonFixture(t *testing.T) (*MenuButton, buttonRuntimeStub) {
	t.Helper()
	btn := NewMenuButton("Settings", []MenuButtonEntry{
		{Kind: MenuButtonEntrySection, Label: "Notifications"},
		{Key: "push", Label: "Push notifications", IconRef: "push"},
		{Key: "badges", Label: "Badges", IconRef: "badge", Selected: true},
		{Kind: MenuButtonEntryDivider},
		{Kind: MenuButtonEntrySection, Label: "Actions"},
		{Key: "rename", Label: "Rename app", IconRef: "rename"},
		{Key: "restart", Label: "Restart app", Shortcut: "Cmd+R"},
		{Key: "stop", Label: "Stop app"},
		{Key: "delete", Label: "Delete app", Destructive: true},
	})
	btn.SetTriggerIconRef("settings")
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"settings": mustMenuButtonIconAsset("settings"),
			"push":     mustMenuButtonIconAsset("push"),
			"badge":    mustMenuButtonIconAsset("badge"),
			"rename":   mustMenuButtonIconAsset("rename"),
		},
	}
	return btn, rt
}

func mustMenuButtonIconAsset(ref string) runtimepkg.IconAsset {
	return runtimepkg.NewIconAsset(ref, 1, gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 7), gfx.RectFromXYWH(0, 0, 24, 24))
}

func allMenuButtonFieldsPresent[T any](value T) bool {
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

func TestMenuButtonGoldenDefault(t *testing.T) {
	AssertMenuButtonGolden(t, "default", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {})
}

func TestMenuButtonGoldenCompact(t *testing.T) {
	AssertMenuButtonGolden(t, "compact", defaultMenuButtonTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(btn *MenuButton) {})
}

func TestMenuButtonGoldenComfortable(t *testing.T) {
	AssertMenuButtonGolden(t, "comfortable", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {})
}

func TestMenuButtonGoldenDisabled(t *testing.T) {
	AssertMenuButtonGolden(t, "disabled", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.SetDisabled(true)
	})
}

func TestMenuButtonGoldenHighContrast(t *testing.T) {
	AssertMenuButtonGolden(t, "high_contrast", highContrastMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {})
}

func TestMenuButtonGoldenHovered(t *testing.T) {
	AssertMenuButtonGolden(t, "hovered", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 2, Y: 2}})
	})
}

func TestMenuButtonGoldenPressed(t *testing.T) {
	AssertMenuButtonGolden(t, "pressed", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 2, Y: 2}, Button: platform.PointerLeft})
	})
}

func TestMenuButtonGoldenFocused(t *testing.T) {
	AssertMenuButtonGolden(t, "focused", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.onFocusGained()
	})
}

func TestMenuButtonGoldenRTL(t *testing.T) {
	AssertMenuButtonGolden(t, "rtl", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(btn *MenuButton) {})
}

func TestMenuButtonGoldenOpen(t *testing.T) {
	AssertMenuButtonGolden(t, "open", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.SetOpen(true)
	})
}

func TestMenuButtonGoldenDestructiveHover(t *testing.T) {
	AssertMenuButtonGolden(t, "destructive_hover", defaultMenuButtonTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(btn *MenuButton) {
		btn.SetOpen(true)
		btn.hoveredIndex = 8
	})
}

func TestMenuButtonMixedDirectionLabelAndEntriesUseSharedTextLayout(t *testing.T) {
	btn, rt := newMenuButtonFixture(t)
	btn.SetLabel("הגדרות Settings")
	btn.SetEntries([]MenuButtonEntry{
		{Kind: MenuButtonEntrySection, Label: "פעולות Actions"},
		{Key: "rename", Label: "שינוי שם Rename", Shortcut: "Cmd+R"},
	})

	ctx := theme.DefaultResolvedContext()
	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := btn.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        ctx,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 720}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	btn.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: ctx, ParentGroup: btn.layoutRole.Parent, ChildGroup: btn.layoutRole.Child}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	if btn.cachedTriggerLabelLayout == nil || len(btn.cachedTriggerLabelLayout.Lines) == 0 {
		t.Fatal("expected trigger label layout")
	}
	if got := btn.cachedTriggerLabelLayout.Lines[0].Direction; got != di.DirectionRTL {
		t.Fatalf("trigger label direction = %v, want RTL", got)
	}
	if len(btn.cachedEntryLayouts) == 0 || btn.cachedEntryLayouts[1].labelLayout == nil {
		t.Fatal("expected entry label layout")
	}
	if got := btn.cachedEntryLayouts[1].labelLayout.Lines[0].Direction; got != di.DirectionRTL {
		t.Fatalf("entry label direction = %v, want RTL", got)
	}
	if btn.cachedTriggerIconBounds.IsEmpty() || btn.cachedTriggerLabelBounds.IsEmpty() {
		t.Fatalf("expected icon-plus-text trigger geometry, got icon=%#v label=%#v", btn.cachedTriggerIconBounds, btn.cachedTriggerLabelBounds)
	}
}

func AssertMenuButtonGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*MenuButton)) {
	t.Helper()
	btn, rt, measureCtx := newMenuButtonGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(btn)
	}
	renderMenuButtonToSurface(t, btn, rt, measureCtx, density, direction, name)
}

func renderMenuButtonToSurface(t *testing.T, btn *MenuButton, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
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
	testkit.AssertGolden(t, surface, "menu_button_"+goldenName)
}

func newMenuButtonGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*MenuButton, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	btn := NewMenuButton("Settings", []MenuButtonEntry{
		{Kind: MenuButtonEntrySection, Label: "Notifications"},
		{Key: "push", Label: "Push notifications", IconRef: "push"},
		{Key: "badges", Label: "Badges", IconRef: "badge", Selected: true},
		{Kind: MenuButtonEntryDivider},
		{Kind: MenuButtonEntrySection, Label: "Actions"},
		{Key: "rename", Label: "Rename app", IconRef: "rename"},
		{Key: "restart", Label: "Restart app", Shortcut: "Cmd+R"},
		{Key: "stop", Label: "Stop app"},
		{Key: "delete", Label: "Delete app", Destructive: true},
	})
	btn.SetTriggerIconRef("settings")
	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"settings": mustMenuButtonIconAsset("settings"),
			"push":     mustMenuButtonIconAsset("push"),
			"badge":    mustMenuButtonIconAsset("badge"),
			"rename":   mustMenuButtonIconAsset("rename"),
		},
	}
	return btn, rt, resolved
}

func defaultMenuButtonTokens() theme.Tokens {
	return theme.DefaultTokens()
}

func highContrastMenuButtonTokens() theme.Tokens {
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
