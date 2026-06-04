package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type tabsRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s tabsRuntimeStub) Schedule(j job.AnyJob)  {}
func (s tabsRuntimeStub) CancelJob(id job.JobID) {}
func (s tabsRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s tabsRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s tabsRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s tabsRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestTabsMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	tabs, rt, measureCtx := newTabsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1400, H: 800}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	tabs.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: tabs.Layout.Parent,
		ChildGroup:  tabs.Layout.Child,
	}, bounds)

	if got := tabs.AccessibilityRole(); got != "tablist" {
		t.Fatalf("accessibility role = %q, want tablist", got)
	}
	if got := tabs.AccessibleName(); got != "Primary navigation" {
		t.Fatalf("accessible name = %q, want Primary navigation", got)
	}
	if len(tabs.cachedTabBounds) != len(tabs.Items) {
		t.Fatalf("cached tab bounds = %d, want %d", len(tabs.cachedTabBounds), len(tabs.Items))
	}
	if tabs.cachedPanelBounds.IsEmpty() || tabs.cachedTabListBounds.IsEmpty() {
		t.Fatalf("expected panel/tab-list geometry, got tablist=%#v panel=%#v", tabs.cachedTabListBounds, tabs.cachedPanelBounds)
	}

	firstHit := tabs.Hit.HitTest(gfx.Point{
		X: tabs.cachedTabBounds[0].Min.X + tabs.cachedTabBounds[0].Width()*0.5,
		Y: tabs.cachedTabBounds[0].Min.Y + tabs.cachedTabBounds[0].Height()*0.5,
	})
	if !firstHit.Hit || (firstHit.MarkID != tabsMarkIDTab && firstHit.MarkID != tabsMarkIDTabLabel) {
		t.Fatalf("expected tab or label hit, got %#v", firstHit)
	}
	panelHit := tabs.Hit.HitTest(gfx.Point{
		X: tabs.cachedPanelBounds.Min.X + tabs.cachedPanelBounds.Width()*0.5,
		Y: tabs.cachedPanelBounds.Min.Y + tabs.cachedPanelBounds.Height()*0.5,
	})
	if !panelHit.Hit || panelHit.MarkID != tabsMarkIDPanelAnchor {
		t.Fatalf("expected panel anchor hit, got %#v", panelHit)
	}

	anchors := tabs.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := tabs.Projection.Project(facet.ProjectionContext{
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

func TestTabsPointerAndKeyboardInteraction(t *testing.T) {
	tabs, rt, measureCtx := newTabsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1400, H: 800}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	tabs.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	activated := -1
	tabs.Activated.Subscribe(func(index int) {
		activated = index
	})

	secondCenter := gfx.Point{
		X: tabs.cachedTabBounds[1].Min.X + tabs.cachedTabBounds[1].Width()*0.5,
		Y: tabs.cachedTabBounds[1].Min.Y + tabs.cachedTabBounds[1].Height()*0.5,
	}
	if !tabs.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !tabs.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := tabs.ActiveIndex.Get(); got != 1 {
		t.Fatalf("active index after pointer = %d, want 1", got)
	}
	if activated != 1 {
		t.Fatalf("expected activated signal for index 1, got %d", activated)
	}

	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := tabs.ActiveIndex.Get(); got != 2 {
		t.Fatalf("active index after right = %d, want 2", got)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if got := tabs.ActiveIndex.Get(); got != 0 {
		t.Fatalf("active index after home = %d, want 0", got)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if got := tabs.ActiveIndex.Get(); got != len(tabs.Items)-1 {
		t.Fatalf("active index after end = %d, want %d", got, len(tabs.Items)-1)
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !tabs.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}

	tabs.onFocusLost()
	tabs.focusFromPointer = false
	tabs.onFocusGained()
	if !tabs.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	tabs.Disabled = marks.Const(true)
	if tabs.Focus.Focusable() {
		t.Fatal("expected disabled tabs to be unfocusable")
	}
	if tabs.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled tabs to ignore pointer input")
	}
	if tabs.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected disabled tabs to ignore keyboard input")
	}
}

func TestTabsGoldenDefault(t *testing.T) {
	AssertTabsGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenCompact(t *testing.T) {
	AssertTabsGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenDisabled(t *testing.T) {
	AssertTabsGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.Disabled = marks.Const(true)
	})
}

func TestTabsGoldenHighContrast(t *testing.T) {
	AssertTabsGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
}

func TestTabsGoldenHovered(t *testing.T) {
	AssertTabsGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.hoveredIndex = 1
	})
}

func TestTabsGoldenPressed(t *testing.T) {
	AssertTabsGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.pressedIndex = 0
	})
}

func TestTabsGoldenFocused(t *testing.T) {
	AssertTabsGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.focusedVisible = true
	})
}

func TestTabsGoldenRTL(t *testing.T) {
	ltr := AssertTabsGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {})
	rtl := AssertTabsGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(t *Tabs) {})
	testkit.AssertGoldenPair(t, ltr, rtl, "tabs")
}

func TestTabsGoldenSelected(t *testing.T) {
	AssertTabsGolden(t, "selected", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(t *Tabs) {
		t.ActiveIndex = marks.Const(1)
	})
}

func AssertTabsGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Tabs)) *testkit.MemorySurface {
	t.Helper()
	tabs, rt, measureCtx := newTabsTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(tabs)
	}
	return renderTabsToSurface(t, tabs, rt, measureCtx, density, direction, name)
}

func renderTabsToSurface(t *testing.T, tabs *Tabs, rt tabsRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) *testkit.MemorySurface {
	t.Helper()
	facet.Attach(tabs, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := tabs.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1800, H: 1200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	tabs.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := tabs.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}
	surface := testkit.NewMemorySurface(int(result.Size.W+1), int(result.Size.H+1))
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
				Commands:    gfx.CommandList{Commands: cmds.Commands},
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "tabs_"+goldenName)
	return surface
}

func newTabsTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Tabs, tabsRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	tabs := NewTabs("Primary navigation", []TabItem{
		{Key: "dashboard", Label: "Dashboard", PanelText: "Tab Panel 1"},
		{Key: "monitoring", Label: "Monitoring", PanelText: "Monitoring panel"},
		{Key: "activity", Label: "Activity", PanelText: "Activity panel"},
		{Key: "settings", Label: "Settings", PanelText: "Settings panel"},
	})
	rt := tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return tabs, rt, resolved
}



func defaultTabsTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastTabsTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func densityToTemplateMode(density theme.DensityID) theme.DensityMode {
	switch density {
	case theme.DensityIDCompact:
		return theme.DensityCompact
	case theme.DensityIDTouch:
		return theme.DensityTouch
	default:
		return theme.DensityComfortable
	}
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
