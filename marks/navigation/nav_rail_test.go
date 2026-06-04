package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

type navRailRuntimeStub struct {
	tabsRuntimeStub
	icons map[string]runtimepkg.IconAsset
}

func (s navRailRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := s.icons[ref]
	return asset, ok
}

func TestNavRailMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	rail, rt, measureCtx := newNavRailTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(rail, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rail.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 1400}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	rail.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: rail.LayoutRole().Parent,
		ChildGroup:  rail.LayoutRole().Child,
	}, bounds)

	if got := rail.AccessibilityRole(); got != "navigation" {
		t.Fatalf("accessibility role = %q, want navigation", got)
	}
	if got := rail.AccessibleName(); got != "Primary rail" {
		t.Fatalf("accessible name = %q, want Primary rail", got)
	}
	if !rail.FocusRole().Focusable() {
		t.Fatal("expected nav rail to be focusable")
	}
	children := rail.Children()
	if len(children) != len(rail.Items) {
		t.Fatalf("expected %d child facets, got %d", len(rail.Items), len(children))
	}
	if rail.cachedRailBounds.IsEmpty() || len(rail.cachedItemBounds) != len(rail.Items) {
		t.Fatalf("expected arranged rail geometry, got rail=%#v items=%d", rail.cachedRailBounds, len(rail.cachedItemBounds))
	}
	if rail.cachedItemFacets[0] == nil || rail.cachedItemFacets[0].ShowLabel.Get() == false {
		t.Fatal("expected expanded rail to show item labels")
	}

	itemHit := rail.HitRole().HitTest(gfx.Point{
		X: rail.cachedItemBounds[0].Min.X + rail.cachedItemBounds[0].Width()*0.5,
		Y: rail.cachedItemBounds[0].Min.Y + rail.cachedItemBounds[0].Height()*0.5,
	})
	if !itemHit.Hit || (itemHit.MarkID != navRailMarkIDActiveIndicator && itemHit.MarkID != navRailMarkIDNavItems) {
		t.Fatalf("expected item hit, got %#v", itemHit)
	}
	railHit := rail.HitRole().HitTest(gfx.Point{
		X: rail.cachedRailBounds.Min.X + 2,
		Y: rail.cachedRailBounds.Min.Y + 2,
	})
	if !railHit.Hit || railHit.MarkID != navRailMarkIDRailSurface {
		t.Fatalf("expected rail surface hit, got %#v", railHit)
	}

	anchors := rail.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "item_profile", "item_home"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := rail.ProjectionRole().Project(facet.ProjectionContext{
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

func TestNavRailPointerAndKeyboardInteraction(t *testing.T) {
	rail, rt, measureCtx := newNavRailTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(rail, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rail.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 1400}})
	rail.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	activated := -1
	rail.Activated.Subscribe(func(index int) {
		activated = index
	})

	secondCenter := gfx.Point{
		X: rail.cachedItemBounds[1].Min.X + rail.cachedItemBounds[1].Width()*0.5,
		Y: rail.cachedItemBounds[1].Min.Y + rail.cachedItemBounds[1].Height()*0.5,
	}
	if !rail.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !rail.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := rail.ActiveIndex.Get(); got != 1 {
		t.Fatalf("active index after pointer = %d, want 1", got)
	}
	if activated != 1 {
		t.Fatalf("expected activated signal for index 1, got %d", activated)
	}

	if !rail.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
	if !rail.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !rail.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}

	rail.onFocusLost()
	rail.onFocusGained()
	if !rail.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}

	rail.Disabled = marks.Const(true)
	if rail.FocusRole().Focusable() {
		t.Fatal("expected disabled nav rail to be unfocusable")
	}
	if rail.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled nav rail to ignore pointer input")
	}
	if rail.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected disabled nav rail to ignore keyboard input")
	}
}

func TestNavRailCollapsedHidesLabels(t *testing.T) {
	rail, rt, measureCtx := newNavRailTestFixture(t, defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR)
	rail.Collapsed = marks.Const(true)
	facet.Attach(rail, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rail.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDCompact),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 1400}})
	rail.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))
	for i, item := range rail.cachedItemFacets {
		if item == nil {
			t.Fatalf("missing child facet %d", i)
		}
		if item.ShowLabel.Get() {
			t.Fatalf("expected collapsed rail item %d to hide labels", i)
		}
	}
}

func TestNavRailGoldenDefault(t *testing.T) {
	AssertNavRailGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {})
}

func TestNavRailGoldenCompact(t *testing.T) {
	AssertNavRailGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(r *NavRail) {
		r.Collapsed = marks.Const(true)
	})
}

func TestNavRailGoldenComfortable(t *testing.T) {
	AssertNavRailGolden(t, "comfortable", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {})
}

func TestNavRailGoldenDisabled(t *testing.T) {
	AssertNavRailGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {
		r.Disabled = marks.Const(true)
	})
}

func TestNavRailGoldenHighContrast(t *testing.T) {
	AssertNavRailGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {})
}

func TestNavRailGoldenHovered(t *testing.T) {
	AssertNavRailGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {
		r.hoveredIndex = 1
	})
}

func TestNavRailGoldenPressed(t *testing.T) {
	AssertNavRailGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {
		r.pressedIndex = 0
	})
}

func TestNavRailGoldenFocused(t *testing.T) {
	AssertNavRailGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {
		r.onFocusGained()
	})
}

func TestNavRailGoldenRTL(t *testing.T) {
	AssertNavRailGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(r *NavRail) {})
}

func TestNavRailGoldenSelected(t *testing.T) {
	AssertNavRailGolden(t, "selected", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *NavRail) {
		r.ActiveIndex = marks.Const(2)
	})
}

func AssertNavRailGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*NavRail)) {
	t.Helper()
	rail, rt, measureCtx := newNavRailTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(rail)
	}
	renderNavRailToSurface(t, rail, rt, measureCtx, density, direction, name)
}

func renderNavRailToSurface(t *testing.T, rail *NavRail, rt navRailRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(rail, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rail.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 1400}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	rail.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := rail.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}
	surfaceW := int(result.Size.W) + 1
	surfaceH := int(result.Size.H) + 1
	if surfaceW < 1 {
		surfaceW = 1
	}
	if surfaceH < 1 {
		surfaceH = 1
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
				Commands:    *cmds,
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "nav_rail_"+goldenName)
}

func newNavRailTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*NavRail, navRailRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustTabsFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	rail := NewNavRail("Primary rail", []NavRailItem{
		{Key: "profile", Label: "Profile", IconRef: "user"},
		{Key: "home", Label: "Home", IconRef: "home"},
		{Key: "notifications", Label: "Notifications", IconRef: "bell"},
		{Key: "bookmarks", Label: "Bookmarks", IconRef: "bookmark"},
		{Key: "messages", Label: "Messages", IconRef: "envelope"},
		{Key: "theme", Label: "Theme", IconRef: "moon"},
		{Key: "settings", Label: "Settings", IconRef: "cog"},
		{Key: "logout", Label: "Log out", IconRef: "arrow-up-right-from-square"},
	})
	rt := navRailRuntimeStub{
		tabsRuntimeStub: tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts},
		icons:           navRailTestIcons(),
	}
	return rail, rt, resolved
}

func navRailTestIcons() map[string]runtimepkg.IconAsset {
	return map[string]runtimepkg.IconAsset{
		"user":                       navRailIconAsset("user", gfx.CirclePath(gfx.Point{X: 12, Y: 9}, 4.5)),
		"home":                       navRailIconAsset("home", gfx.NewPath().MoveTo(gfx.Point{X: 5, Y: 12}).LineTo(gfx.Point{X: 12, Y: 6}).LineTo(gfx.Point{X: 19, Y: 12}).LineTo(gfx.Point{X: 19, Y: 19}).LineTo(gfx.Point{X: 13, Y: 19}).LineTo(gfx.Point{X: 13, Y: 14}).LineTo(gfx.Point{X: 11, Y: 14}).LineTo(gfx.Point{X: 11, Y: 19}).LineTo(gfx.Point{X: 5, Y: 19}).Close().Build()),
		"bell":                       navRailIconAsset("bell", gfx.CirclePath(gfx.Point{X: 12, Y: 10}, 5.5)),
		"bookmark":                   navRailIconAsset("bookmark", gfx.PolylinePath([]gfx.Point{{X: 7, Y: 5}, {X: 17, Y: 5}, {X: 17, Y: 19}, {X: 12, Y: 15}, {X: 7, Y: 19}}, true)),
		"envelope":                   navRailIconAsset("envelope", gfx.NewPath().MoveTo(gfx.Point{X: 5, Y: 7}).LineTo(gfx.Point{X: 19, Y: 7}).LineTo(gfx.Point{X: 19, Y: 17}).LineTo(gfx.Point{X: 5, Y: 17}).Close().Build()),
		"moon":                       navRailIconAsset("moon", gfx.CirclePath(gfx.Point{X: 13, Y: 12}, 5.5)),
		"cog":                        navRailIconAsset("cog", gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 6.5)),
		"arrow-up-right-from-square": navRailIconAsset("arrow-up-right-from-square", gfx.NewPath().MoveTo(gfx.Point{X: 6, Y: 18}).LineTo(gfx.Point{X: 6, Y: 12}).LineTo(gfx.Point{X: 12, Y: 12}).LineTo(gfx.Point{X: 12, Y: 6}).LineTo(gfx.Point{X: 18, Y: 6}).LineTo(gfx.Point{X: 18, Y: 12}).LineTo(gfx.Point{X: 14, Y: 12}).LineTo(gfx.Point{X: 14, Y: 18}).Close().Build()),
	}
}

func navRailIconAsset(ref string, path gfx.Path) runtimepkg.IconAsset {
	return runtimepkg.NewIconAsset(ref, 1, path, gfx.RectFromXYWH(0, 0, 24, 24))
}
