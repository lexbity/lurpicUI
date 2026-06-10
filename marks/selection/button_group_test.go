package selection

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestButtonGroupMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	bg, rt, measureCtx := newButtonGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bg.Label = marks.Const("Choices")
	bg.SetOptions([]ButtonGroupOption{
		{Key: "test-item-1", Label: "test-item-1"},
		{Key: "test-item-2", Label: "test-item-2"},
		{Key: "test-item-3", Label: "test-item-3"},
	})

	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bg.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	bg.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: bg.LayoutRole().Parent, ChildGroup: bg.LayoutRole().Child}, bounds)

	if got := bg.AccessibilityRole(); got != "group" {
		t.Fatalf("accessibility role = %q, want group", got)
	}
	if got := bg.AccessibleName(); got != "Choices" {
		t.Fatalf("accessible name = %q, want Choices", got)
	}
	if len(bg.Children()) != 3 {
		t.Fatalf("expected 3 child facets, got %d", len(bg.Children()))
	}
	if bg.cachedGroupSurface.IsEmpty() || len(bg.cachedOptionBounds) != 3 {
		t.Fatalf("expected arranged geometry, got surface=%#v options=%d", bg.cachedGroupSurface, len(bg.cachedOptionBounds))
	}

	hit := bg.HitRole().HitTest(gfx.Point{X: bg.cachedOptionBounds[1].Min.X + 1, Y: bg.cachedOptionBounds[1].Min.Y + 1})
	if !hit.Hit || (hit.MarkID != buttonGroupMarkIDSelectedIndicator && hit.MarkID != buttonGroupMarkIDOptionButtons) {
		t.Fatalf("expected option hit, got %#v", hit)
	}

	anchors := bg.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := bg.ProjectionRole().Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawGlyphRun, sawFillPath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.DrawGlyphRun:
			sawGlyphRun = true
		case gfx.FillPath, gfx.FillRect:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill commands")
	}
}

func TestButtonGroupPointerKeyboardAndStoreBehavior(t *testing.T) {
	bg, rt, measureCtx := newButtonGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bg.SetOptions([]ButtonGroupOption{
		{Key: "test-item-1", Label: "test-item-1"},
		{Key: "test-item-2", Label: "test-item-2"},
		{Key: "test-item-3", Label: "test-item-3"},
	})
	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bg.LayoutRole().Measure(facet.MeasureContext{Runtime: rt, Theme: measureCtx, ContentScale: 1, Density: facet.DensityID(theme.DensityIDComfortable), WritingDirection: facet.WritingDirectionLTR}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bg.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	center := gfx.Point{X: bg.cachedOptionBounds[0].Min.X + bg.cachedOptionBounds[0].Width()*0.5, Y: bg.cachedOptionBounds[0].Min.Y + bg.cachedOptionBounds[0].Height()*0.5}
	if !bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerEnter, Position: center}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := bg.currentSelection(); len(got) != 1 || got[0] != "test-item-1" {
		t.Fatalf("selection after pointer toggle = %#v, want test-item-1", got)
	}

	if !bg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := bg.focusedIndex; got != 1 {
		t.Fatalf("focused index after right = %d, want 1", got)
	}
	if !bg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !bg.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
	if got := bg.currentSelection(); len(got) != 1 || got[0] != "test-item-2" {
		t.Fatalf("selection after keyboard toggle = %#v, want test-item-2", got)
	}

	bg.Mode = marks.Const(ButtonGroupMultiple)
	bg.SetSelectedKeys("test-item-1")
	bg.SetSelectedKeys("test-item-1", "test-item-3")
	if got := bg.currentSelection(); len(got) != 2 {
		t.Fatalf("expected multi selection, got %#v", got)
	}
}

func TestButtonGroupComposesPrimitiveTextAndIconChildren(t *testing.T) {
	bg, rt, measureCtx := newButtonGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bg.SetOptions([]ButtonGroupOption{
		{
			Key:   "test-item-1",
			Label: "test-item-1",
			Icon:  primitive.IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor"><path d="M5 12l4 4 10-10"/></svg>`),
		},
	})
	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bg.LayoutRole().Measure(facet.MeasureContext{Runtime: rt, Theme: measureCtx, ContentScale: 1, Density: facet.DensityID(theme.DensityIDComfortable), WritingDirection: facet.WritingDirectionLTR}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bg.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	if len(bg.cachedItems) != 1 || bg.cachedItems[0] == nil {
		t.Fatalf("expected one composed child, got %#v", bg.cachedItems)
	}
	item := bg.cachedItems[0]
	if item.labelMark == nil {
		t.Fatal("expected primitive.text label child")
	}
	if item.iconMark == nil {
		t.Fatal("expected primitive.icon child")
	}
	if got := len(item.Children()); got != 2 {
		t.Fatalf("expected 2 real child facets, got %d", got)
	}
	if item.cachedLabelBounds.IsEmpty() || item.cachedIconBounds.IsEmpty() {
		t.Fatalf("expected arranged child bounds, got icon=%#v label=%#v", item.cachedIconBounds, item.cachedLabelBounds)
	}
}

func TestButtonGroupFocusAndDisabledBehavior(t *testing.T) {
	bg, rt, measureCtx := newButtonGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bg.SetOptions([]ButtonGroupOption{
		{Key: "test-item-1", Label: "test-item-1"},
		{Key: "test-item-2", Label: "test-item-2"},
	})
	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bg.LayoutRole().Measure(facet.MeasureContext{Runtime: rt, Theme: measureCtx, ContentScale: 1, Density: facet.DensityID(theme.DensityIDComfortable), WritingDirection: facet.WritingDirectionLTR}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bg.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	bg.onFocusGained()
	if !bg.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !bg.pointInFocusRing(gfx.Point{X: bg.cachedOptionBounds[0].Min.X + 1, Y: bg.cachedOptionBounds[0].Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	bg.Disabled = marks.Const(true)
	if bg.FocusRole().Focusable() {
		t.Fatal("expected disabled button-group to be unfocusable")
	}
	if bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerPress, Position: centerPoint(bounds), Button: platform.PointerLeft}) {
		t.Fatal("expected disabled button-group to ignore pointer input")
	}
	if bg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled button-group to ignore keyboard input")
	}
}

func TestButtonGroupStoreInvalidation(t *testing.T) {
	bg, rt, measureCtx := newButtonGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	bg.SetOptions([]ButtonGroupOption{
		{Key: "test-item-1", Label: "test-item-1"},
		{Key: "test-item-2", Label: "test-item-2"},
	})
	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = bg.LayoutRole().Measure(facet.MeasureContext{Runtime: rt, Theme: measureCtx, ContentScale: 1, Density: facet.DensityID(theme.DensityIDComfortable), WritingDirection: facet.WritingDirectionLTR}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	initial := bg.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	bg.Value.Set([]string{"test-item-2"})
	if flags := bg.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
}

func TestButtonGroupGoldenDefault(t *testing.T) {
	AssertButtonGroupGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {})
}

func TestButtonGroupGoldenCompact(t *testing.T) {
	AssertButtonGroupGolden(t, "compact", defaultSliderTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(bg *ButtonGroup) {})
}

func TestButtonGroupGoldenDisabled(t *testing.T) {
	AssertButtonGroupGolden(t, "disabled", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {
		bg.Disabled = marks.Const(true)
	})
}

func TestButtonGroupGoldenHighContrast(t *testing.T) {
	AssertButtonGroupGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {})
}

func TestButtonGroupGoldenHovered(t *testing.T) {
	AssertButtonGroupGolden(t, "hovered", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {
		bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestButtonGroupGoldenPressed(t *testing.T) {
	AssertButtonGroupGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {
		bg.onChildPointer(0, facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
	})
}

func TestButtonGroupGoldenFocused(t *testing.T) {
	AssertButtonGroupGolden(t, "focused", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {
		bg.onFocusGained()
	})
}

func TestButtonGroupGoldenRTL(t *testing.T) {
	AssertButtonGroupGolden(t, "rtl", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(bg *ButtonGroup) {})
}

func TestButtonGroupGoldenSelected(t *testing.T) {
	AssertButtonGroupGolden(t, "selected", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(bg *ButtonGroup) {
		bg.SetSelectedKeys("test-item-2")
	})
}

func AssertButtonGroupGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*ButtonGroup)) {
	t.Helper()
	bg, rt, measureCtx := newButtonGroupTestFixture(t, tokens, density, direction)
	bg.Label = marks.Const("Choices")
	bg.SetOptions([]ButtonGroupOption{
		{Key: "test-item-1", Label: "test-item-1"},
		{Key: "test-item-2", Label: "test-item-2"},
		{Key: "test-item-3", Label: "test-item-3"},
	})
	if mutate != nil {
		mutate(bg)
	}
	renderButtonGroupToSurface(t, bg, rt, measureCtx, density, direction, name)
}

func renderButtonGroupToSurface(t *testing.T, bg *ButtonGroup, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(bg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bg.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bg.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := bg.ProjectionRole().Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "button_group_"+goldenName)
}

func newButtonGroupTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*ButtonGroup, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	bg := NewButtonGroup("Choices", nil)
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return bg, rt, resolved
}
