package selection

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestDropdownSelectMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	sel, rt, measureCtx := newDropdownSelectTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	sel.Value.Set("canberra")

	facet.Attach(sel, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	sel.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: sel.LayoutRole().Parent,
		ChildGroup:  sel.LayoutRole().Child,
	}, bounds)

	if got := sel.AccessibilityRole(); got != "combobox" {
		t.Fatalf("accessibility role = %q, want combobox", got)
	}
	if got := sel.AccessibleName(); got != "What city do you live in?" {
		t.Fatalf("accessible name = %q, want What city do you live in?", got)
	}
	if sel.textRole.Layout == nil {
		t.Fatal("expected selected-value text layout")
	}
	if sel.cachedTriggerBounds.IsEmpty() || sel.cachedValueBounds.IsEmpty() || sel.cachedChevronBounds.IsEmpty() {
		t.Fatalf("expected trigger geometry, got trigger=%#v value=%#v chevron=%#v", sel.cachedTriggerBounds, sel.cachedValueBounds, sel.cachedChevronBounds)
	}

	triggerHit := sel.HitRole().HitTest(gfx.Point{
		X: sel.cachedTriggerBounds.Min.X + 2,
		Y: sel.cachedTriggerBounds.Min.Y + 2,
	})
	if !triggerHit.Hit || triggerHit.MarkID != dropdownSelectMarkIDTrigger {
		t.Fatalf("expected trigger hit, got %#v", triggerHit)
	}
	valueHit := sel.HitRole().HitTest(gfx.Point{
		X: sel.cachedValueBounds.Min.X + 2,
		Y: sel.cachedValueBounds.Min.Y + 2,
	})
	if !valueHit.Hit || valueHit.MarkID != dropdownSelectMarkIDSelectedValue {
		t.Fatalf("expected value hit, got %#v", valueHit)
	}
	chevronHit := sel.HitRole().HitTest(gfx.Point{
		X: sel.cachedChevronBounds.Min.X + 1,
		Y: sel.cachedChevronBounds.Min.Y + 1,
	})
	if !chevronHit.Hit || chevronHit.MarkID != dropdownSelectMarkIDChevron {
		t.Fatalf("expected chevron hit, got %#v", chevronHit)
	}

	anchors := sel.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := sel.ProjectionRole().Project(facet.ProjectionContext{
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
		t.Fatal("expected fill commands")
	}
}

func TestDropdownSelectPointerAndKeyboardInteraction(t *testing.T) {
	sel, rt, measureCtx := newDropdownSelectTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sel, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sel.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	triggerCenter := gfx.Point{X: sel.cachedTriggerBounds.Min.X + sel.cachedTriggerBounds.Width()*0.5, Y: sel.cachedTriggerBounds.Min.Y + sel.cachedTriggerBounds.Height()*0.5}
	if !sel.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: triggerCenter}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !sel.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: triggerCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !sel.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: triggerCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if !sel.open {
		t.Fatal("expected trigger click to open listbox")
	}

	sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	sel.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	optionCenter := gfx.Point{X: sel.cachedOptionRects[2].Min.X + sel.cachedOptionRects[2].Width()*0.5, Y: sel.cachedOptionRects[2].Min.Y + sel.cachedOptionRects[2].Height()*0.5}
	if !sel.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: optionCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected option press to be handled")
	}
	if !sel.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: optionCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected option release to be handled")
	}
	if got := sel.selectedValue(); got != "canberra" {
		t.Fatalf("selected value after option click = %q, want canberra", got)
	}

	if !sel.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space press to be handled")
	}
	if !sel.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space release to be handled")
	}
	if !sel.open {
		t.Fatal("expected space to reopen listbox")
	}
	if !sel.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
	if !sel.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if !sel.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to be handled")
	}
	if sel.open {
		t.Fatal("expected escape to close listbox")
	}
}

func TestDropdownSelectFocusAndDisabledBehavior(t *testing.T) {
	sel, rt, measureCtx := newDropdownSelectTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sel, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sel.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	sel.onFocusGained()
	if !sel.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !sel.pointInFocusRing(gfx.Point{X: sel.cachedTriggerBounds.Min.X + 1, Y: sel.cachedTriggerBounds.Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	sel.Disabled = marks.Const(true)
	if sel.FocusRole().Focusable() {
		t.Fatal("expected disabled dropdown select to be unfocusable")
	}
	if sel.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: centerPoint(bounds), Button: platform.PointerLeft}) {
		t.Fatal("expected disabled dropdown select to ignore pointer input")
	}
	if sel.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled dropdown select to ignore keyboard input")
	}
}

func TestDropdownSelectStoreInvalidation(t *testing.T) {
	sel, rt, measureCtx := newDropdownSelectTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sel, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	initial := sel.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	sel.Value.Set("perth")
	if flags := sel.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updated := sel.Base().SubscribedVersions()
	if updated[0] <= initial[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initial, updated)
	}
}

func TestDropdownSelectGoldenDefault(t *testing.T) {
	AssertDropdownSelectGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {})
}

func TestDropdownSelectGoldenCompact(t *testing.T) {
	AssertDropdownSelectGolden(t, "compact", defaultSliderTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(s *DropdownSelect) {})
}

func TestDropdownSelectGoldenComfortable(t *testing.T) {
	AssertDropdownSelectGolden(t, "comfortable", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {})
}

func TestDropdownSelectGoldenDisabled(t *testing.T) {
	AssertDropdownSelectGolden(t, "disabled", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.Disabled = marks.Const(true)
	})
}

func TestDropdownSelectGoldenHighContrast(t *testing.T) {
	AssertDropdownSelectGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {})
}

func TestDropdownSelectGoldenHovered(t *testing.T) {
	AssertDropdownSelectGolden(t, "hovered", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestDropdownSelectGoldenPressed(t *testing.T) {
	AssertDropdownSelectGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 10, Y: 10}, Button: platform.PointerLeft})
	})
}

func TestDropdownSelectGoldenFocused(t *testing.T) {
	AssertDropdownSelectGolden(t, "focused", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.onFocusGained()
	})
}

func TestDropdownSelectGoldenRTL(t *testing.T) {
	AssertDropdownSelectGolden(t, "rtl", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(s *DropdownSelect) {})
}

func TestDropdownSelectGoldenOpen(t *testing.T) {
	AssertDropdownSelectGolden(t, "open", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.open = true
	})
}

func TestDropdownSelectGoldenDismissed(t *testing.T) {
	AssertDropdownSelectGolden(t, "dismissed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.open = true
		s.onDismiss(facet.DismissEvent{Trigger: facet.DismissalTriggerPointer})
	})
}

func TestDropdownSelectGoldenSelected(t *testing.T) {
	AssertDropdownSelectGolden(t, "selected", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *DropdownSelect) {
		s.Value.Set("brisbane")
	})
}

func AssertDropdownSelectGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*DropdownSelect)) {
	t.Helper()
	sel, rt, measureCtx := newDropdownSelectTestFixture(t, tokens, density, direction)
	sel.Label = marks.Const("What city do you live in?")
	sel.Value.Set("adelaide")
	if mutate != nil {
		mutate(sel)
	}
	renderDropdownSelectToSurface(t, sel, rt, measureCtx, density, direction, name)
}

func renderDropdownSelectToSurface(t *testing.T, sel *DropdownSelect, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(sel, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sel.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 820, H: 520}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sel.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := sel.ProjectionRole().Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "dropdown_select_"+goldenName)
}

func newDropdownSelectTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*DropdownSelect, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustSliderFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	sel := NewDropdownSelect("What city do you live in?", []DropdownOption{
		{Value: "adelaide", Label: "Adelaide"},
		{Value: "brisbane", Label: "Brisbane"},
		{Value: "canberra", Label: "Canberra"},
		{Value: "darwin", Label: "Darwin"},
		{Value: "hobart", Label: "Hobart"},
		{Value: "melbourne", Label: "Melbourne"},
		{Value: "perth", Label: "Perth"},
		{Value: "sydney", Label: "Sydney"},
	})
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return sel, rt, resolved
}
