package selection

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRadioGroupMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	rg, rt, measureCtx := newRadioGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rg.SetLabel("Size")
	rg.SetValue("medium")

	facet.Attach(rg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rg.layoutRole.Measure(facet.MeasureContext{
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
	rg.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: rg.layoutRole.Parent, ChildGroup: rg.layoutRole.Child}, bounds)

	if got := rg.AccessibilityRole(); got != "radiogroup" {
		t.Fatalf("accessibility role = %q, want radiogroup", got)
	}
	if got := rg.AccessibleName(); got != "Size" {
		t.Fatalf("accessible name = %q, want Size", got)
	}
	if rg.textRole.Layout == nil {
		t.Fatal("expected group label text layout")
	}
	if rg.cachedGroupLabelRect.IsEmpty() || len(rg.cachedItemRows) != len(rg.Options) {
		t.Fatalf("expected item geometry, got label=%#v rows=%d", rg.cachedGroupLabelRect, len(rg.cachedItemRows))
	}

	groupHit := rg.hitRole.HitTest(gfx.Point{X: rg.cachedGroupLabelRect.Min.X + 1, Y: rg.cachedGroupLabelRect.Min.Y + 1})
	if !groupHit.Hit || groupHit.MarkID != radioGroupMarkIDGroupLabel {
		t.Fatalf("expected group label hit, got %#v", groupHit)
	}
	controlHit := rg.hitRole.HitTest(gfx.Point{
		X: rg.cachedItemControls[1].Min.X + 1,
		Y: rg.cachedItemControls[1].Min.Y + 1,
	})
	if !controlHit.Hit || controlHit.MarkID != radioGroupMarkIDControl {
		t.Fatalf("expected control hit, got %#v", controlHit)
	}
	labelHit := rg.hitRole.HitTest(gfx.Point{
		X: rg.cachedItemLabels[1].Min.X + rg.cachedItemLabels[1].Width()*0.5,
		Y: rg.cachedItemLabels[1].Min.Y + rg.cachedItemLabels[1].Height()*0.5,
	})
	if !labelHit.Hit || labelHit.MarkID != radioGroupMarkIDItemLabel {
		t.Fatalf("expected item label hit, got %#v", labelHit)
	}
	midX := rg.cachedItemRows[1].Min.X + rg.cachedControlSize + rg.cachedControlGap*0.5
	midY := rg.cachedItemRows[1].Min.Y + rg.cachedItemRows[1].Height()*0.5
	itemsHit := rg.hitRole.HitTest(gfx.Point{X: midX, Y: midY})
	if !itemsHit.Hit || itemsHit.MarkID != radioGroupMarkIDRadioItems {
		t.Fatalf("expected radio items hit, got %#v", itemsHit)
	}

	anchors := rg.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := rg.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
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
		t.Fatal("expected fill commands")
	}
}

func TestRadioGroupPointerAndKeyboardInteraction(t *testing.T) {
	rg, rt, measureCtx := newRadioGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rg.SetLabel("Size")
	facet.Attach(rg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rg.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	rg.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	center := gfx.Point{X: rg.cachedItemControls[2].Min.X + rg.cachedItemControls[2].Width()*0.5, Y: rg.cachedItemControls[2].Min.Y + rg.cachedItemControls[2].Height()*0.5}
	if !rg.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: center}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !rg.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !rg.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := rg.currentValue(); got != "large" {
		t.Fatalf("value after pointer toggle = %q, want large", got)
	}

	if !rg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if got := rg.currentValue(); got != "small" {
		t.Fatalf("value after home = %q, want small", got)
	}
	if !rg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
	if got := rg.currentValue(); got != "medium" {
		t.Fatalf("value after down = %q, want medium", got)
	}
	if !rg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if got := rg.currentValue(); got != "large" {
		t.Fatalf("value after end = %q, want large", got)
	}
	if !rg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !rg.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
}

func TestRadioGroupFocusAndDisabledBehavior(t *testing.T) {
	rg, rt, measureCtx := newRadioGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(rg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rg.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	rg.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	rg.onFocusGained()
	if !rg.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !rg.pointInFocusRing(gfx.Point{X: rg.cachedItemFocusRing[0].Min.X + 1, Y: rg.cachedItemFocusRing[0].Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	rg.SetDisabled(true)
	if rg.focusRole.Focusable() {
		t.Fatal("expected disabled radio group to be unfocusable")
	}
	if rg.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: centerPoint(bounds), Button: platform.PointerLeft}) {
		t.Fatal("expected disabled radio group to ignore pointer input")
	}
	if rg.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled radio group to ignore keyboard input")
	}
}

func TestRadioGroupStoreInvalidation(t *testing.T) {
	rg, rt, measureCtx := newRadioGroupTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(rg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = rg.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	initial := rg.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	rg.Value.Set("large")
	if flags := rg.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updated := rg.Base().SubscribedVersions()
	if updated[0] <= initial[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initial, updated)
	}
}

func TestRadioGroupGoldenDefault(t *testing.T) {
	AssertRadioGroupGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {})
}

func TestRadioGroupGoldenCompact(t *testing.T) {
	AssertRadioGroupGolden(t, "compact", defaultSliderTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(rg *RadioGroup) {})
}

func TestRadioGroupGoldenComfortable(t *testing.T) {
	AssertRadioGroupGolden(t, "comfortable", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {})
}

func TestRadioGroupGoldenDisabled(t *testing.T) {
	AssertRadioGroupGolden(t, "disabled", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {
		rg.SetDisabled(true)
	})
}

func TestRadioGroupGoldenHighContrast(t *testing.T) {
	AssertRadioGroupGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {})
}

func TestRadioGroupGoldenHovered(t *testing.T) {
	AssertRadioGroupGolden(t, "hovered", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {
		rg.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestRadioGroupGoldenPressed(t *testing.T) {
	AssertRadioGroupGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {
		rg.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 10, Y: 10}, Button: platform.PointerLeft})
	})
}

func TestRadioGroupGoldenFocused(t *testing.T) {
	AssertRadioGroupGolden(t, "focused", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {
		rg.onFocusGained()
	})
}

func TestRadioGroupGoldenRTL(t *testing.T) {
	AssertRadioGroupGolden(t, "rtl", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(rg *RadioGroup) {})
}

func TestRadioGroupGoldenSelected(t *testing.T) {
	AssertRadioGroupGolden(t, "selected", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(rg *RadioGroup) {
		rg.SetValue("large")
	})
}

func AssertRadioGroupGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*RadioGroup)) {
	t.Helper()
	rg, rt, measureCtx := newRadioGroupTestFixture(t, tokens, density, direction)
	rg.SetLabel("Size")
	if mutate != nil {
		mutate(rg)
	}
	renderRadioGroupToSurface(t, rg, rt, measureCtx, density, direction, name)
}

func renderRadioGroupToSurface(t *testing.T, rg *RadioGroup, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(rg, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := rg.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	rg.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := rg.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
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
	testkit.AssertGolden(t, surface, "radio_group_"+goldenName)
}

func newRadioGroupTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*RadioGroup, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustSliderFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	rg := NewRadioGroup("Size", []RadioOption{
		{Value: "small", Label: "Small"},
		{Value: "medium", Label: "Medium"},
		{Value: "large", Label: "Large"},
	})
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return rg, rt, resolved
}
