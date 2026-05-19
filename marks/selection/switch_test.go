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

func TestSwitchMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	sw, rt, measureCtx := newSwitchTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	sw.SetLabel("Label")
	sw.SetChecked(true)

	facet.Attach(sw, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sw.layoutRole.Measure(facet.MeasureContext{
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
	sw.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: sw.layoutRole.Parent,
		ChildGroup:  sw.layoutRole.Child,
	}, bounds)

	if got := sw.AccessibilityRole(); got != "switch" {
		t.Fatalf("accessibility role = %q, want switch", got)
	}
	if got := sw.AccessibleName(); got != "Label" {
		t.Fatalf("accessible name = %q, want Label", got)
	}
	if sw.textRole.Layout == nil {
		t.Fatal("expected label text layout")
	}
	if sw.cachedLabelBounds.IsEmpty() || sw.cachedTrackBounds.IsEmpty() || sw.cachedThumbBounds.IsEmpty() {
		t.Fatalf("expected switch geometry, got label=%#v track=%#v thumb=%#v", sw.cachedLabelBounds, sw.cachedTrackBounds, sw.cachedThumbBounds)
	}

	labelHit := sw.hitRole.HitTest(gfx.Point{
		X: sw.cachedLabelBounds.Min.X + sw.cachedLabelBounds.Width()*0.5,
		Y: sw.cachedLabelBounds.Min.Y + sw.cachedLabelBounds.Height()*0.5,
	})
	if !labelHit.Hit || labelHit.MarkID != switchMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", labelHit)
	}
	thumbHit := sw.hitRole.HitTest(gfx.Point{
		X: sw.cachedThumbBounds.Min.X + sw.cachedThumbBounds.Width()*0.5,
		Y: sw.cachedThumbBounds.Min.Y + sw.cachedThumbBounds.Height()*0.5,
	})
	if !thumbHit.Hit || thumbHit.MarkID != switchMarkIDThumb {
		t.Fatalf("expected thumb hit, got %#v", thumbHit)
	}
	trackHit := sw.hitRole.HitTest(gfx.Point{
		X: sw.cachedTrackBounds.Min.X + 1,
		Y: sw.cachedTrackBounds.Min.Y + sw.cachedTrackBounds.Height()*0.5,
	})
	if !trackHit.Hit || trackHit.MarkID != switchMarkIDStateLayer {
		t.Fatalf("expected state-layer hit for checked switch, got %#v", trackHit)
	}

	anchors := sw.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := sw.projectionRole.Project(facet.ProjectionContext{
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

func TestSwitchPointerAndKeyboardInteraction(t *testing.T) {
	sw, rt, measureCtx := newSwitchTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	sw.SetLabel("Label")
	facet.Attach(sw, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sw.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sw.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	center := gfx.Point{
		X: sw.cachedTrackBounds.Min.X + sw.cachedTrackBounds.Width()*0.5,
		Y: sw.cachedTrackBounds.Min.Y + sw.cachedTrackBounds.Height()*0.5,
	}
	if !sw.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: center}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !sw.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !sw.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := sw.isChecked(); !got {
		t.Fatalf("value after pointer toggle = %v, want true", got)
	}

	if !sw.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !sw.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
	if got := sw.isChecked(); got {
		t.Fatalf("value after space toggle = %v, want false", got)
	}

	if !sw.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key press to be handled")
	}
	if !sw.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key release to be handled")
	}
	if got := sw.isChecked(); !got {
		t.Fatalf("value after enter toggle = %v, want true", got)
	}
}

func TestSwitchFocusAndDisabledBehavior(t *testing.T) {
	sw, rt, measureCtx := newSwitchTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sw, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sw.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sw.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	sw.onFocusGained()
	if !sw.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !sw.pointInFocusRing(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	sw.SetDisabled(true)
	if sw.focusRole.Focusable() {
		t.Fatal("expected disabled switch to be unfocusable")
	}
	if sw.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled switch to ignore pointer input")
	}
	if sw.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled switch to ignore keyboard input")
	}
}

func TestSwitchStoreInvalidation(t *testing.T) {
	sw, rt, measureCtx := newSwitchTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(sw, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = sw.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	initial := sw.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	sw.Value.Set(true)
	if flags := sw.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updated := sw.Base().SubscribedVersions()
	if updated[0] <= initial[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initial, updated)
	}
}

func TestSwitchGoldenDefault(t *testing.T) {
	AssertSwitchGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {})
}

func TestSwitchGoldenCompact(t *testing.T) {
	AssertSwitchGolden(t, "compact", defaultSliderTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(s *Switch) {})
}

func TestSwitchGoldenComfortable(t *testing.T) {
	AssertSwitchGolden(t, "comfortable", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {})
}

func TestSwitchGoldenDisabled(t *testing.T) {
	AssertSwitchGolden(t, "disabled", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {
		s.SetDisabled(true)
	})
}

func TestSwitchGoldenHighContrast(t *testing.T) {
	AssertSwitchGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {})
}

func TestSwitchGoldenHovered(t *testing.T) {
	AssertSwitchGolden(t, "hovered", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestSwitchGoldenPressed(t *testing.T) {
	AssertSwitchGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 10, Y: 10}, Button: platform.PointerLeft})
	})
}

func TestSwitchGoldenFocused(t *testing.T) {
	AssertSwitchGolden(t, "focused", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Switch) {
		s.onFocusGained()
	})
}

func TestSwitchGoldenRTL(t *testing.T) {
	AssertSwitchGolden(t, "rtl", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(s *Switch) {})
}

func AssertSwitchGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Switch)) {
	t.Helper()
	sw, rt, measureCtx := newSwitchTestFixture(t, tokens, density, direction)
	sw.SetLabel("Label")
	sw.SetChecked(true)
	if mutate != nil {
		mutate(sw)
	}
	renderSwitchToSurface(t, sw, rt, measureCtx, density, direction, name)
}

func renderSwitchToSurface(t *testing.T, sw *Switch, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(sw, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := sw.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	sw.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := sw.projectionRole.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "switch_"+goldenName)
}

func newSwitchTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Switch, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustSliderFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	sw := NewSwitch("Label")
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return sw, rt, resolved
}
