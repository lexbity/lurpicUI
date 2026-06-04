package selection

import (
	"image/color"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type sliderRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s sliderRuntimeStub) Schedule(j job.AnyJob)  {}
func (s sliderRuntimeStub) CancelJob(id job.JobID) {}
func (s sliderRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s sliderRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s sliderRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s sliderRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestSliderMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	slider, rt, measureCtx := newSliderTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	slider.SetValue(50)
	slider.Label = "Volume"

	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 220}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 18, result.Size.W, result.Size.H)
	slider.LayoutRole().Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: slider.LayoutRole().Parent,
		ChildGroup:  slider.LayoutRole().Child,
	}, bounds)

	if got := slider.AccessibilityRole(); got != "slider" {
		t.Fatalf("accessibility role = %q, want slider", got)
	}
	if got := slider.AccessibleName(); got != "Volume" {
		t.Fatalf("accessible name = %q, want Volume", got)
	}
	if slider.textRole.Layout == nil {
		t.Fatal("expected value label text layout")
	}
	if slider.cachedTrackBounds.IsEmpty() || slider.cachedThumbBounds.IsEmpty() {
		t.Fatalf("expected track/thumb geometry, got track=%#v thumb=%#v", slider.cachedTrackBounds, slider.cachedThumbBounds)
	}

	thumbHit := slider.HitRole().HitTest(gfx.Point{
		X: (slider.cachedThumbBounds.Min.X + slider.cachedThumbBounds.Max.X) * 0.5,
		Y: (slider.cachedThumbBounds.Min.Y + slider.cachedThumbBounds.Max.Y) * 0.5,
	})
	if !thumbHit.Hit || thumbHit.MarkID != sliderMarkIDThumb {
		t.Fatalf("expected thumb hit, got %#v", thumbHit)
	}
	activeHit := slider.HitRole().HitTest(gfx.Point{
		X: slider.cachedActiveBounds.Min.X + 1,
		Y: slider.cachedActiveBounds.Min.Y + slider.cachedActiveBounds.Height()*0.5,
	})
	if !activeHit.Hit || activeHit.MarkID != sliderMarkIDActive {
		t.Fatalf("expected active-track hit, got %#v", activeHit)
	}
	valueHit := slider.HitRole().HitTest(gfx.Point{
		X: slider.cachedValueLabelBounds.Min.X + slider.cachedValueLabelBounds.Width()*0.5,
		Y: slider.cachedValueLabelBounds.Min.Y + slider.cachedValueLabelBounds.Height()*0.5,
	})
	if !valueHit.Hit || valueHit.MarkID != sliderMarkIDValueLabel {
		t.Fatalf("expected value-label hit, got %#v", valueHit)
	}

	anchors := slider.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := slider.ProjectionRole().Project(facet.ProjectionContext{
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

func TestSliderPointerAndKeyboardInteraction(t *testing.T) {
	slider, rt, measureCtx := newSliderTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	slider.SetValue(50)
	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 220}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	slider.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	centerX := slider.cachedTrackBounds.Min.X + slider.cachedTrackBounds.Width()*0.25
	centerY := slider.cachedTrackBounds.Min.Y + slider.cachedTrackBounds.Height()*0.5
	if !slider.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: centerX, Y: centerY}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if got := slider.displayValue(); got != 25 {
		t.Fatalf("value after pointer press = %v, want 25", got)
	}
	if !slider.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: centerX, Y: centerY}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}

	slider.SetValue(50)
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyLeft}) {
		t.Fatal("expected left key to be handled")
	}
	if got := slider.displayValue(); got != 45 {
		t.Fatalf("value after left key = %v, want 45", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := slider.displayValue(); got != 50 {
		t.Fatalf("value after right key = %v, want 50", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if got := slider.displayValue(); got != 0 {
		t.Fatalf("value after home = %v, want 0", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if got := slider.displayValue(); got != 100 {
		t.Fatalf("value after end = %v, want 100", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyPageDown}) {
		t.Fatal("expected page down to be handled")
	}
	if got := slider.displayValue(); got != 90 {
		t.Fatalf("value after page down = %v, want 90", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyPageUp}) {
		t.Fatal("expected page up to be handled")
	}
	if got := slider.displayValue(); got != 100 {
		t.Fatalf("value after page up = %v, want 100", got)
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !slider.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
}

func TestSliderFocusAndDisabledBehavior(t *testing.T) {
	slider, rt, measureCtx := newSliderTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 220}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	slider.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	slider.onFocusGained()
	if !slider.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !slider.pointInFocusRing(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	slider.Disabled = marks.Const(true)
	if slider.FocusRole().Focusable() {
		t.Fatal("expected disabled slider to be unfocusable")
	}
	if slider.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled slider to ignore pointer input")
	}
	if slider.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyLeft}) {
		t.Fatal("expected disabled slider to ignore keyboard input")
	}
}

func TestSliderStoreInvalidation(t *testing.T) {
	slider, rt, measureCtx := newSliderTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 220}})
	initial := slider.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	slider.Value.Set(75)
	if flags := slider.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updated := slider.Base().SubscribedVersions()
	if updated[0] <= initial[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initial, updated)
	}
}

func TestSliderGoldenDefault(t *testing.T) {
	AssertSliderGolden(t, "default", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {})
}

func TestSliderGoldenCompact(t *testing.T) {
	AssertSliderGolden(t, "compact", defaultSliderTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(s *Slider) {})
}

func TestSliderGoldenComfortable(t *testing.T) {
	AssertSliderGolden(t, "comfortable", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {})
}

func TestSliderGoldenDisabled(t *testing.T) {
	AssertSliderGolden(t, "disabled", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {
		s.Disabled = marks.Const(true)
	})
}

func TestSliderGoldenHighContrast(t *testing.T) {
	AssertSliderGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {})
}

func TestSliderGoldenHovered(t *testing.T) {
	AssertSliderGolden(t, "hovered", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestSliderGoldenPressed(t *testing.T) {
	AssertSliderGolden(t, "pressed", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {
		s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 64, Y: 64}, Button: platform.PointerLeft})
	})
}

func TestSliderGoldenFocused(t *testing.T) {
	AssertSliderGolden(t, "focused", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *Slider) {
		s.onFocusGained()
	})
}

func TestSliderGoldenRTL(t *testing.T) {
	AssertSliderGolden(t, "rtl", defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(s *Slider) {})
}

func TestSliderGoldenSkeuomorphic(t *testing.T) {
	assertSliderSkeuomorphicGolden(t, "skeuomorphic", func(s *Slider) {
		s.Variant = marks.Const(uiinput.SliderSkeuomorphic)
	})
}

func TestSliderGoldenSkeuomorphicPressed(t *testing.T) {
	assertSliderSkeuomorphicGolden(t, "skeuomorphic_pressed", func(s *Slider) {
		s.Variant = marks.Const(uiinput.SliderSkeuomorphic)
		s.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 64, Y: 64}, Button: platform.PointerLeft})
	})
}

func assertSliderSkeuomorphicGolden(t *testing.T, name string, mutate func(*Slider)) {
	t.Helper()
	slider, rt, measureCtx := newSliderTestFixture(t, defaultSliderTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	slider.Label = "Slider"
	slider.SetValue(50)
	if mutate != nil {
		mutate(slider)
	}

	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirection(layout.WritingDirectionLTR),
	}, facet.Constraints{MaxSize: gfx.Size{W: 320, H: 80}})

	surfaceW := 360
	surfaceH := 120
	x := maxFloat(0, float32(surfaceW)-result.Size.W) * 0.5
	y := maxFloat(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)

	slider.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := slider.ProjectionRole().Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}

	surface := testkit.NewMemorySurface(surfaceW, surfaceH)
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}

	bgPath := gfx.RectPath(gfx.RectFromXYWH(0, 0, float32(surfaceW), float32(surfaceH)))
	// Premium matte dark charcoal background color for synthesizer layout
	bgColor := gfx.ColorFromRGBA8(26, 29, 36, 255) // #1a1d24
	bgBrush := gfx.SolidBrush(bgColor)
	bgCmd := gfx.FillPath{Path: bgPath, Brush: bgBrush}

	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      gfx.RectFromXYWH(0, 0, float32(surfaceW), float32(surfaceH)),
			Opacity:     1,
			CommandHash: 1,
			Commands:    gfx.CommandList{Commands: append([]gfx.Command{bgCmd}, cmds.Commands...)},
		}},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "slider_"+name)
}


func AssertSliderGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Slider)) {
	t.Helper()
	slider, rt, measureCtx := newSliderTestFixture(t, tokens, density, direction)
	slider.Label = "Slider"
	slider.SetValue(50)
	if mutate != nil {
		mutate(slider)
	}
	renderSliderToSurface(t, slider, rt, measureCtx, density, direction, name)
}

func renderSliderToSurface(t *testing.T, slider *Slider, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(slider, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := slider.LayoutRole().Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	slider.LayoutRole().Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := slider.ProjectionRole().Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "slider_"+goldenName)
}

func newSliderTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Slider, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustSliderFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	slider := NewSlider("Slider", 0, 100, 5)
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return slider, rt, resolved
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

func defaultSliderTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func colorToGfx(c color.RGBA) gfx.Color {
	return gfx.Color{R: float32(c.R) / 255, G: float32(c.G) / 255, B: float32(c.B) / 255, A: float32(c.A) / 255}
}

func mustSliderFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadSliderFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func mustReadSliderFont(t *testing.T, rel string) []byte {
	t.Helper()
	out, err := exec.Command("go", "env", "GOMODCACHE").Output()
	if err != nil {
		t.Fatalf("go env GOMODCACHE: %v", err)
	}
	path := filepath.Join(string(bytesTrim(out)), rel)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read font %q: %v", path, err)
	}
	return data
}

func bytesTrim(in []byte) []byte {
	for len(in) > 0 {
		switch in[len(in)-1] {
		case '\n', '\r', '\t', ' ':
			in = in[:len(in)-1]
		default:
			return in
		}
	}
	return in
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
