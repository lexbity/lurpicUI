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
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

func TestActionGroupMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	group, rt := newActionGroupFixture(t)

	facet.Attach(group, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := group.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1080, H: 640}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H)
	group.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: group.layoutRole.Parent, ChildGroup: group.layoutRole.Child}, bounds)

	if got := group.AccessibilityRole(); got != "group" {
		t.Fatalf("accessibility role = %q, want group", got)
	}
	if got := group.AccessibleName(); got != "Action group" {
		t.Fatalf("accessible name = %q, want Action group", got)
	}
	if len(group.cachedActionBounds) != 4 || len(group.cachedSeparatorBounds) != 3 {
		t.Fatalf("expected item and separator geometry, got actions=%d separators=%d", len(group.cachedActionBounds), len(group.cachedSeparatorBounds))
	}

	actionHit := group.hitRole.HitTest(gfx.Point{X: group.cachedActionBounds[0].Min.X + 1, Y: group.cachedActionBounds[0].Min.Y + 1})
	if !actionHit.Hit || actionHit.MarkID != actionGroupMarkIDActionItems {
		t.Fatalf("expected action item hit, got %#v", actionHit)
	}
	sepHit := group.hitRole.HitTest(gfx.Point{X: group.cachedSeparatorBounds[0].Min.X + 1, Y: group.cachedSeparatorBounds[0].Min.Y + 1})
	if !sepHit.Hit || sepHit.MarkID != actionGroupMarkIDSeparators {
		t.Fatalf("expected separator hit, got %#v", sepHit)
	}

	anchors := group.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	activated := ""
	group.Activated.Subscribe(func(key string) {
		activated = key
	})
	first := gfx.Point{X: group.cachedActionBounds[0].Min.X + 1, Y: group.cachedActionBounds[0].Min.Y + 1}
	if !group.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !group.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != "edit" {
		t.Fatalf("expected edit activation, got %q", activated)
	}

	group.onFocusLost()
	group.focusFromPointer = false
	group.onFocusGained()
	if !group.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}
	if !group.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := group.focusedIndex; got != 1 {
		t.Fatalf("focused index = %d, want 1", got)
	}
	if !group.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter press to be handled")
	}
	if !group.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter release to be handled")
	}

	group.SetDisabled(true)
	if group.focusRole.Focusable() {
		t.Fatal("expected disabled action group to be unfocusable")
	}
	if group.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled action group to ignore pointer input")
	}
	if group.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled action group to ignore keyboard input")
	}
}

func TestActionGroupRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveActionGroupRecipe(ctx)
	if !allActionGroupFieldsPresent(slots) {
		t.Fatalf("action group slots contain zero values: %#v", slots)
	}
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 5 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func newActionGroupFixture(t *testing.T) (*ActionGroup, buttonRuntimeStub) {
	t.Helper()
	group := NewActionGroup("Action group", []ActionGroupAction{
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "copy", Label: "Copy", IconRef: "copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
		{Key: "more", AccessibleLabel: "More options", IconRef: "more"},
	})
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"edit":   mustActionGroupIconAsset("edit"),
			"copy":   mustActionGroupIconAsset("copy"),
			"delete": mustActionGroupIconAsset("delete"),
			"more":   mustActionGroupIconAsset("more"),
		},
	}
	return group, rt
}

func mustActionGroupIconAsset(ref string) runtimepkg.IconAsset {
	var path gfx.Path
	switch ref {
	case "edit":
		path = actionGroupEditIconPath()
	case "copy":
		path = actionGroupCopyIconPath()
	case "delete":
		path = actionGroupDeleteIconPath()
	default:
		path = actionGroupMoreIconPath()
	}
	return runtimepkg.NewIconAsset(ref, 1, path, gfx.RectFromXYWH(0, 0, 24, 24))
}

func actionGroupEditIconPath() gfx.Path {
	return gfx.NewPath().
		MoveTo(gfx.Point{X: 5, Y: 18}).
		LineTo(gfx.Point{X: 6.5, Y: 19.5}).
		LineTo(gfx.Point{X: 18, Y: 8}).
		LineTo(gfx.Point{X: 16.5, Y: 6.5}).
		Close().
		MoveTo(gfx.Point{X: 17, Y: 5}).
		LineTo(gfx.Point{X: 19, Y: 7}).
		LineTo(gfx.Point{X: 20, Y: 6}).
		LineTo(gfx.Point{X: 18, Y: 4}).
		Close().
		Build()
}

func actionGroupCopyIconPath() gfx.Path {
	return combineActionGroupPaths(
		gfx.RectPath(gfx.RectFromXYWH(6, 8, 9, 10)),
		gfx.RectPath(gfx.RectFromXYWH(9, 5, 9, 10)),
	)
}

func actionGroupDeleteIconPath() gfx.Path {
	return combineActionGroupPaths(
		gfx.RectPath(gfx.RectFromXYWH(7, 8, 10, 10)),
		gfx.RectPath(gfx.RectFromXYWH(8, 6, 8, 2)),
		gfx.RectPath(gfx.RectFromXYWH(10, 4, 4, 2)),
	)
}

func actionGroupMoreIconPath() gfx.Path {
	return combineActionGroupPaths(
		gfx.CirclePath(gfx.Point{X: 8, Y: 12}, 1.6),
		gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 1.6),
		gfx.CirclePath(gfx.Point{X: 16, Y: 12}, 1.6),
	)
}

func combineActionGroupPaths(paths ...gfx.Path) gfx.Path {
	var out gfx.Path
	for _, path := range paths {
		out.Segments = append(out.Segments, path.Segments...)
	}
	return out
}

func TestActionGroupGoldenDefault(t *testing.T) {
	AssertActionGroupGolden(t, "default", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {})
}

func TestActionGroupGoldenCompact(t *testing.T) {
	AssertActionGroupGolden(t, "compact", defaultActionGroupTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(group *ActionGroup) {})
}

func TestActionGroupGoldenComfortable(t *testing.T) {
	AssertActionGroupGolden(t, "comfortable", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {})
}

func TestActionGroupGoldenDisabled(t *testing.T) {
	AssertActionGroupGolden(t, "disabled", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {
		group.SetDisabled(true)
	})
}

func TestActionGroupGoldenHighContrast(t *testing.T) {
	AssertActionGroupGolden(t, "high_contrast", highContrastActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {})
}

func TestActionGroupGoldenHovered(t *testing.T) {
	AssertActionGroupGolden(t, "hovered", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {
		group.hoveredIndex = 0
	})
}

func TestActionGroupGoldenPressed(t *testing.T) {
	AssertActionGroupGolden(t, "pressed", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {
		group.pressedIndex = 0
	})
}

func TestActionGroupGoldenFocused(t *testing.T) {
	AssertActionGroupGolden(t, "focused", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(group *ActionGroup) {
		group.onFocusGained()
	})
}

func TestActionGroupGoldenRTL(t *testing.T) {
	AssertActionGroupGolden(t, "rtl", defaultActionGroupTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(group *ActionGroup) {})
}

func AssertActionGroupGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*ActionGroup)) {
	t.Helper()
	group, rt, measureCtx := newActionGroupGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(group)
	}
	renderActionGroupToSurface(t, group, rt, measureCtx, density, direction, name)
}

func renderActionGroupToSurface(t *testing.T, group *ActionGroup, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(group, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := group.layoutRole.Measure(facet.MeasureContext{
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
	group.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: group.layoutRole.Parent, ChildGroup: group.layoutRole.Child}, bounds)

	cmds := group.projectionRole.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "action_group_"+goldenName)
}

func newActionGroupGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*ActionGroup, buttonRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	group := NewActionGroup("Action group", []ActionGroupAction{
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "copy", Label: "Copy", IconRef: "copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
		{Key: "more", AccessibleLabel: "More options", IconRef: "more"},
	})
	rt := buttonRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		icons: buttonIconResolverStub{
			"edit":   mustActionGroupIconAsset("edit"),
			"copy":   mustActionGroupIconAsset("copy"),
			"delete": mustActionGroupIconAsset("delete"),
			"more":   mustActionGroupIconAsset("more"),
		},
	}
	return group, rt, resolved
}

func defaultActionGroupTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastActionGroupTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func allActionGroupFieldsPresent[T any](value T) bool {
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
