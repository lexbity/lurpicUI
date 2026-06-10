package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

type navDrawerRuntimeStub struct {
	tabsRuntimeStub
	icons map[string]runtimepkg.IconAsset
}

func (s navDrawerRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := s.icons[ref]
	return asset, ok
}

func TestNavDrawerMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	drawer, rt, measureCtx := newNavDrawerTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(drawer, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := drawer.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1200}})
	if result.Size.W != 1440 || result.Size.H != 1200 {
		t.Fatalf("expected overlay-sized measure, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	drawer.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: drawer.Layout.Parent,
		ChildGroup:  drawer.Layout.Child,
	}, bounds)

	if got := drawer.AccessibilityRole(); got != "navigation" {
		t.Fatalf("accessibility role = %q, want navigation", got)
	}
	if got := drawer.AccessibleName(); got != "Main navigation" {
		t.Fatalf("accessible name = %q, want Main navigation", got)
	}
	if !drawer.Focus.Focusable() {
		t.Fatal("expected open drawer to be focusable")
	}
	children := drawer.Children()
	if len(children) != 2 {
		t.Fatalf("expected 2 structural children, got %d", len(children))
	}
	if children[0].MarkID != navDrawerMarkIDHeader || children[1].MarkID != navDrawerMarkIDNavItems {
		t.Fatalf("unexpected structural child order: %#v", children)
	}
	if children[0].Layout == nil || children[1].Layout == nil {
		t.Fatalf("expected structural child layouts, got %#v", children)
	}
	if drawer.cachedDrawerBounds.IsEmpty() || drawer.cachedItemBounds[0].IsEmpty() {
		t.Fatalf("expected drawer geometry, got drawer=%#v item0=%#v", drawer.cachedDrawerBounds, drawer.cachedItemBounds[0])
	}
	if drawer.cachedHeaderBounds.IsEmpty() || drawer.cachedNavBounds.IsEmpty() {
		t.Fatalf("expected structural group bounds, got header=%#v nav=%#v", drawer.cachedHeaderBounds, drawer.cachedNavBounds)
	}

	scrimHit := drawer.Hit.HitTest(gfx.Point{X: bounds.Max.X - 4, Y: bounds.Max.Y - 4})
	if !scrimHit.Hit || scrimHit.MarkID != navDrawerMarkIDScrimOptional {
		t.Fatalf("expected scrim hit, got %#v", scrimHit)
	}
	itemHit := drawer.Hit.HitTest(gfx.Point{
		X: drawer.cachedItemBounds[0].Min.X + drawer.cachedItemBounds[0].Width()*0.5,
		Y: drawer.cachedItemBounds[0].Min.Y + drawer.cachedItemBounds[0].Height()*0.5,
	})
	if !itemHit.Hit || itemHit.MarkID != navDrawerMarkIDNavItems {
		t.Fatalf("expected nav item hit, got %#v", itemHit)
	}
	focusHit := drawer.Hit.HitTest(gfx.Point{
		X: drawer.cachedItemBounds[0].Min.X + 2,
		Y: drawer.cachedItemBounds[0].Min.Y + 2,
	})
	if !focusHit.Hit {
		t.Fatalf("expected drawer hit, got %#v", focusHit)
	}

	anchors := drawer.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := drawer.Projection.Project(facet.ProjectionContext{
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

func TestNavDrawerPointerKeyboardDismissalAndFocus(t *testing.T) {
	drawer, rt, measureCtx := newNavDrawerTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(drawer, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := drawer.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1200}})
	drawer.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	activated := -1
	drawer.Activated.Subscribe(func(index int) {
		activated = index
	})

	itemCenter := gfx.Point{
		X: drawer.cachedItemBounds[1].Min.X + drawer.cachedItemBounds[1].Width()*0.5,
		Y: drawer.cachedItemBounds[1].Min.Y + drawer.cachedItemBounds[1].Height()*0.5,
	}
	if !drawer.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !drawer.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != 1 {
		t.Fatalf("expected activation for index 1, got %d", activated)
	}
	if drawer.Open.Get() {
		t.Fatal("expected drawer to close after item activation")
	}

	drawer.Open = marks.Const(true)
	drawer.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1200}})
	drawer.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, 1440, 1200))

	if !drawer.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerPointer}) {
		t.Fatal("expected dismiss event to close drawer")
	}
	if drawer.Open.Get() {
		t.Fatal("expected drawer closed after dismissal")
	}

	drawer.Open = marks.Const(true)
	drawer.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1200}})
	drawer.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, 1440, 1200))
	drawer.onFocusLost()
	drawer.onFocusGained()
	if !drawer.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !drawer.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
	if drawer.clampedFocusedIndex() != 1 {
		t.Fatalf("expected focused index 1, got %d", drawer.clampedFocusedIndex())
	}
	if !drawer.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter press to be handled")
	}
	if !drawer.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter release to be handled")
	}
	if activated != 1 {
		t.Fatalf("expected activation to remain 1, got %d", activated)
	}

	drawer.Disabled = marks.Const(true)
	if drawer.Focus.Focusable() {
		t.Fatal("expected disabled drawer to be unfocusable")
	}
	if drawer.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: itemCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled drawer to ignore pointer input")
	}
	if drawer.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected disabled drawer to ignore keyboard input")
	}
}

func TestNavDrawerGoldenDefault(t *testing.T) {
	AssertNavDrawerGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {})
}

func TestNavDrawerGoldenCompact(t *testing.T) {
	AssertNavDrawerGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(d *NavDrawer) {})
}

func TestNavDrawerGoldenDisabled(t *testing.T) {
	AssertNavDrawerGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.Disabled = marks.Const(true)
	})
}

func TestNavDrawerGoldenHighContrast(t *testing.T) {
	AssertNavDrawerGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {})
}

func TestNavDrawerGoldenHovered(t *testing.T) {
	AssertNavDrawerGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.hoveredIndex = 1
	})
}

func TestNavDrawerGoldenPressed(t *testing.T) {
	AssertNavDrawerGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.pressedIndex = 2
	})
}

func TestNavDrawerGoldenFocused(t *testing.T) {
	AssertNavDrawerGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.onFocusGained()
	})
}

func TestNavDrawerGoldenRTL(t *testing.T) {
	AssertNavDrawerGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(d *NavDrawer) {})
}

func TestNavDrawerGoldenOpen(t *testing.T) {
	AssertNavDrawerGolden(t, "open", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.CurrentIndex = marks.Const(2)
	})
}

func TestNavDrawerGoldenDismissed(t *testing.T) {
	AssertNavDrawerGolden(t, "dismissed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *NavDrawer) {
		d.Open = marks.Const(false)
	})
}

func AssertNavDrawerGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*NavDrawer)) {
	t.Helper()
	drawer, rt, measureCtx := newNavDrawerTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(drawer)
	}
	renderNavDrawerToSurface(t, drawer, rt, measureCtx, density, direction, name)
}

func renderNavDrawerToSurface(t *testing.T, drawer *NavDrawer, rt navDrawerRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(drawer, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := drawer.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 1200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	drawer.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := drawer.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})

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
	commands := gfx.CommandList{}
	if cmds != nil {
		commands = *cmds
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    commands,
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "nav_drawer_"+goldenName)
}

func newNavDrawerTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*NavDrawer, navDrawerRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	drawer := NewNavDrawer("Main navigation", []NavDrawerSection{
		{
			Label: "Primary",
			Items: []NavDrawerItem{
				{Key: "inbox", Label: "Inbox", IconRef: "inbox"},
				{Key: "outbox", Label: "Outbox", IconRef: "outbox"},
				{Key: "favorites", Label: "Favorites", IconRef: "heart"},
				{Key: "archive", Label: "Archive", IconRef: "archive"},
				{Key: "trash", Label: "Trash", IconRef: "trash"},
				{Key: "spam", Label: "Spam", IconRef: "exclamation-circle"},
			},
		},
		{
			Label: "Labels",
			Items: []NavDrawerItem{
				{Key: "family", Label: "Family", IconRef: "bookmark"},
				{Key: "friends", Label: "Friends", IconRef: "bookmark"},
				{Key: "work", Label: "Work", IconRef: "bookmark"},
				{Key: "account", Label: "Settings & account", IconRef: "cog"},
			},
		},
	})
	rt := navDrawerRuntimeStub{
		tabsRuntimeStub: tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts},
		icons:           navDrawerTestIcons(),
	}
	return drawer, rt, resolved
}

func navDrawerTestIcons() map[string]runtimepkg.IconAsset {
	return map[string]runtimepkg.IconAsset{
		"inbox":              navDrawerIconAsset("inbox", gfx.NewPath().MoveTo(gfx.Point{X: 4, Y: 7}).LineTo(gfx.Point{X: 20, Y: 7}).LineTo(gfx.Point{X: 20, Y: 17}).LineTo(gfx.Point{X: 4, Y: 17}).Close().Build()),
		"outbox":             navDrawerIconAsset("outbox", gfx.PolylinePath([]gfx.Point{{X: 4, Y: 12}, {X: 20, Y: 6}, {X: 17, Y: 12}, {X: 20, Y: 18}}, true)),
		"heart":              navDrawerIconAsset("heart", gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 6)),
		"archive":            navDrawerIconAsset("archive", gfx.RoundedRectPath(gfx.RectFromXYWH(5, 6, 14, 12), 2)),
		"trash":              navDrawerIconAsset("trash", gfx.RectPath(gfx.RectFromXYWH(7, 5, 10, 14))),
		"exclamation-circle": navDrawerIconAsset("exclamation-circle", gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 7)),
		"bookmark":           navDrawerIconAsset("bookmark", gfx.PolylinePath([]gfx.Point{{X: 7, Y: 4}, {X: 17, Y: 4}, {X: 17, Y: 20}, {X: 12, Y: 16}, {X: 7, Y: 20}}, true)),
		"cog":                navDrawerIconAsset("cog", gfx.CirclePath(gfx.Point{X: 12, Y: 12}, 7)),
	}
}

func navDrawerIconAsset(ref string, path gfx.Path) runtimepkg.IconAsset {
	return runtimepkg.NewIconAsset(ref, 1, path, gfx.RectFromXYWH(0, 0, 24, 24))
}
