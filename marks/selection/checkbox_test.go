package selection

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
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

func TestCheckboxMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	cb, rt, measureCtx := newCheckboxTestFixture(t, defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	cb.Label = marks.Const("Enable beta features")
	cb.HelperText = marks.Const("This setting applies after restart.")
	cb.SetState(CheckboxStateOn)

	facet.Attach(cb, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := cb.Layout.Measure(facet.MeasureContext{
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
	cb.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: cb.Layout.Parent,
		ChildGroup:  cb.Layout.Child,
	}, bounds)
	if !cb.isSemanticallySelected() {
		t.Fatal("expected checkbox to be semantically selected")
	}

	if got := cb.AccessibilityRole(); got != "checkbox" {
		t.Fatalf("accessibility role = %q, want checkbox", got)
	}
	if got := cb.AccessibleName(); got != "Enable beta features" {
		t.Fatalf("accessible name = %q, want Enable beta features", got)
	}
	if cb.textRole.Layout == nil {
		t.Fatal("expected label text layout")
	}
	if cb.cachedControlBounds.IsEmpty() || cb.cachedLabelBounds.IsEmpty() || cb.cachedHelperBounds.IsEmpty() {
		t.Fatalf("expected checkbox geometry, got control=%#v label=%#v helper=%#v", cb.cachedControlBounds, cb.cachedLabelBounds, cb.cachedHelperBounds)
	}

	labelHit := cb.Hit.HitTest(gfx.Point{
		X: cb.cachedLabelBounds.Min.X + cb.cachedLabelBounds.Width()*0.5,
		Y: cb.cachedLabelBounds.Min.Y + cb.cachedLabelBounds.Height()*0.5,
	})
	if !labelHit.Hit || labelHit.MarkID != checkboxMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", labelHit)
	}
	helperHit := cb.Hit.HitTest(gfx.Point{
		X: cb.cachedHelperBounds.Min.X + cb.cachedHelperBounds.Width()*0.5,
		Y: cb.cachedHelperBounds.Min.Y + cb.cachedHelperBounds.Height()*0.5,
	})
	if !helperHit.Hit || helperHit.MarkID != checkboxMarkIDHelperText {
		t.Fatalf("expected helper hit, got %#v", helperHit)
	}
	controlHit := cb.Hit.HitTest(gfx.Point{
		X: cb.cachedControlBounds.Min.X + 1,
		Y: cb.cachedControlBounds.Min.Y + 1,
	})
	if !controlHit.Hit || (controlHit.MarkID != checkboxMarkIDStateLayer && controlHit.MarkID != checkboxMarkIDControlBox) {
		t.Fatalf("expected control/state hit, got %#v", controlHit)
	}

	anchors := cb.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := cb.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var capture testkit.CommandCapture
	capture.Capture(cmds)
	capture.AssertHasGlyphRun(t)
	capture.AssertHasFillPath(t)
}

func TestCheckboxPointerAndKeyboardInteraction(t *testing.T) {
	cb, rt, measureCtx := newCheckboxTestFixture(t, defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	cb.Label = marks.Const("Enable beta features")
	facet.Attach(cb, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := cb.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	cb.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	center := gfx.Point{
		X: cb.cachedControlBounds.Min.X + cb.cachedControlBounds.Width()*0.5,
		Y: cb.cachedControlBounds.Min.Y + cb.cachedControlBounds.Height()*0.5,
	}
	if !cb.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: center}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !cb.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !cb.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := cb.state(); got != CheckboxStateOn {
		t.Fatalf("state after pointer toggle = %v, want on", got)
	}

	if !cb.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !cb.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
	if got := cb.state(); got != CheckboxStateOff {
		t.Fatalf("state after keyboard toggle = %v, want off", got)
	}

	cb.SetState(CheckboxStateMixed)
	if !cb.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key press to be handled")
	}
	if !cb.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key release to be handled")
	}
	if got := cb.state(); got != CheckboxStateOn {
		t.Fatalf("state after mixed toggle = %v, want on", got)
	}
}

func TestCheckboxFocusAndDisabledBehavior(t *testing.T) {
	cb, rt, measureCtx := newCheckboxTestFixture(t, defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(cb, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := cb.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	cb.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cb.onFocusGained()
	if !cb.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !cb.pointInFocusRing(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1}) {
		t.Fatal("expected edge point to land in focus ring")
	}

	cb.Disabled = marks.Const(true)
	if cb.Focus.Focusable() {
		t.Fatal("expected disabled checkbox to be unfocusable")
	}
	if cb.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: centerPoint(bounds), Button: platform.PointerLeft}) {
		t.Fatal("expected disabled checkbox to ignore pointer input")
	}
	if cb.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled checkbox to ignore keyboard input")
	}
}

func TestCheckboxStoreInvalidation(t *testing.T) {
	cb, rt, measureCtx := newCheckboxTestFixture(t, defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(cb, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	_ = cb.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	initial := cb.Base().SubscribedVersions()
	if len(initial) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initial))
	}
	cb.Value.Set(CheckboxStateOn)
	if flags := cb.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updated := cb.Base().SubscribedVersions()
	if updated[0] <= initial[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initial, updated)
	}
}

func TestCheckboxGoldenDefault(t *testing.T) {
	AssertCheckboxGolden(t, "default", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {})
}

func TestCheckboxGoldenCompact(t *testing.T) {
	AssertCheckboxGolden(t, "compact", defaultCheckboxTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(c *Checkbox) {})
}

func TestCheckboxGoldenComfortable(t *testing.T) {
	AssertCheckboxGolden(t, "comfortable", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {})
}

func TestCheckboxGoldenDisabled(t *testing.T) {
	AssertCheckboxGolden(t, "disabled", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.Disabled = marks.Const(true)
	})
}

func TestCheckboxGoldenHighContrast(t *testing.T) {
	AssertCheckboxGolden(t, "high_contrast", highContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {})
}

func TestCheckboxGoldenHovered(t *testing.T) {
	AssertCheckboxGolden(t, "hovered", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestCheckboxGoldenPressed(t *testing.T) {
	AssertCheckboxGolden(t, "pressed", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 10, Y: 10}, Button: platform.PointerLeft})
	})
}

func TestCheckboxGoldenFocused(t *testing.T) {
	AssertCheckboxGolden(t, "focused", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.onFocusGained()
	})
}

func TestCheckboxGoldenRTL(t *testing.T) {
	AssertCheckboxGolden(t, "rtl", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(c *Checkbox) {})
}

func TestCheckboxGoldenSelected(t *testing.T) {
	AssertCheckboxGolden(t, "selected", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.SetState(CheckboxStateOn)
	})
}

func TestCheckboxGoldenMixed(t *testing.T) {
	AssertCheckboxGolden(t, "mixed", defaultCheckboxTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(c *Checkbox) {
		c.SetState(CheckboxStateMixed)
	})
}

func AssertCheckboxGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Checkbox)) {
	t.Helper()
	cb, rt, measureCtx := newCheckboxTestFixture(t, tokens, density, direction)
	cb.Label = marks.Const("Checkbox")
	cb.SetState(CheckboxStateOff)
	if mutate != nil {
		mutate(cb)
	}
	renderCheckboxToSurface(t, cb, rt, measureCtx, density, direction, name)
}

func renderCheckboxToSurface(t *testing.T, cb *Checkbox, rt sliderRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(cb, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := cb.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 720, H: 260}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	cb.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	cmds := cb.Projection.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "checkbox_"+goldenName)
}

func newCheckboxTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Checkbox, sliderRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustCheckboxFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	cb := NewCheckbox("Checkbox")
	rt := sliderRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return cb, rt, resolved
}

func mustCheckboxFontRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	return mustSliderFontRegistry(t)
}

func defaultCheckboxTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func centerPoint(bounds gfx.Rect) gfx.Point {
	return gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Min.Y + bounds.Height()*0.5}
}
