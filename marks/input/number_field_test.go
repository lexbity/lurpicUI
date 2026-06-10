package input

import (
	"reflect"
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
)

func TestNumberFieldMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	nf := NewNumberField("Amount")
	nf.Step = marks.Const(float64(0.5))
	nf.Precision = marks.Const(2)
	nf.Value.Set(12.5)
	rt := textFieldRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(nf, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := nf.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 240}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(18, 24, result.Size.W, result.Size.H)
	nf.Layout.Arrange(facet.ArrangeContext{}, bounds)

	if got := nf.AccessibilityRole(); got != "spinbutton" {
		t.Fatalf("accessibility role = %q, want spinbutton", got)
	}
	if got := nf.AccessibleName(); got != "Amount" {
		t.Fatalf("accessible name = %q, want Amount", got)
	}
	if !nf.textRole.IMEEnabled {
		t.Fatal("expected IME to be enabled")
	}
	if nf.cachedStepperUpBounds.IsEmpty() || nf.cachedStepperDownBounds.IsEmpty() {
		t.Fatalf("expected stepper bounds, got up=%#v down=%#v", nf.cachedStepperUpBounds, nf.cachedStepperDownBounds)
	}

	cmds := nf.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
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
		t.Fatal("expected text glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill path commands")
	}

	anchors := nf.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	stepperUp := nf.Hit.HitTest(gfx.Point{X: nf.cachedStepperUpBounds.Min.X + 1, Y: nf.cachedStepperUpBounds.Min.Y + 1})
	if !stepperUp.Hit || stepperUp.MarkID != numberFieldMarkIDStepperUp {
		t.Fatalf("expected stepper up hit, got %#v", stepperUp)
	}
	stepperDown := nf.Hit.HitTest(gfx.Point{X: nf.cachedStepperDownBounds.Min.X + 1, Y: nf.cachedStepperDownBounds.Min.Y + 1})
	if !stepperDown.Hit || stepperDown.MarkID != numberFieldMarkIDStepperDown {
		t.Fatalf("expected stepper down hit, got %#v", stepperDown)
	}
}

func TestNumberFieldStoreChangeStepperKeyboardAndEditing(t *testing.T) {
	nf := NewNumberField("Amount")
	nf.Step = marks.Const(float64(2))
	nf.Value.Set(10)
	rt := textFieldRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(nf, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	_ = nf.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 180}})
	nf.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, nf.Layout.MeasuredSize.W, nf.Layout.MeasuredSize.H))

	initialVersions := nf.Base().SubscribedVersions()
	if len(initialVersions) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initialVersions))
	}

	nf.Value.Set(12)
	if flags := nf.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updatedVersions := nf.Base().SubscribedVersions()
	if updatedVersions[0] <= initialVersions[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initialVersions, updatedVersions)
	}

	_ = nf.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 180}})
	nf.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, nf.Layout.MeasuredSize.W, nf.Layout.MeasuredSize.H))

	if !nf.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: nf.cachedStepperUpBounds.Min.X + 1, Y: nf.cachedStepperUpBounds.Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected stepper press to be handled")
	}
	if !nf.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: nf.cachedStepperUpBounds.Min.X + 1, Y: nf.cachedStepperUpBounds.Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected stepper release to be handled")
	}
	if got := nf.currentValue(); got != 14 {
		t.Fatalf("value after stepper up = %v, want 14", got)
	}
	if !nf.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyDown}) {
		t.Fatal("expected down key to be handled")
	}
	if got := nf.currentValue(); got != 12 {
		t.Fatalf("value after key down = %v, want 12", got)
	}
	nf.onFocusGained()
	nf.setCaretAtEnd(false)
	if !nf.onText(facet.TextEvent{Text: "5"}) {
		t.Fatal("expected text input to be handled")
	}
	if got := nf.currentDisplayText(); got == "" {
		t.Fatal("expected edited display text to remain visible")
	}
	nf.onFocusLost()
	if got := nf.currentValue(); got != 125 {
		t.Fatalf("value after committing text = %v, want 125", got)
	}

	nf.onText(facet.TextEvent{Text: "abc"})
	if !nf.parseError {
		t.Fatal("expected invalid input to set parse error state")
	}
	if got := nf.auxiliaryText(); got == "" {
		t.Fatal("expected auxiliary error text for invalid state")
	}
}

func TestNumberFieldGraphemeBackspaceDeletesWholeCluster(t *testing.T) {
	nf := NewNumberField("Amount")
	nf.editing = true
	nf.editingText = "12"
	nf.cachedValueLayout = textLayoutForTest(t, "12")
	nf.caret = text.GraphemePosition(2, text.AffinityDownstream)
	if !nf.deleteBackward() {
		t.Fatal("expected deleteBackward to handle grapheme cluster")
	}
	if got := nf.currentDisplayText(); got != "1" {
		t.Fatalf("display text = %q, want 1", got)
	}
	if nf.caret.Unit != text.TextUnitGrapheme || nf.caret.Index != 1 {
		t.Fatalf("caret = %#v", nf.caret)
	}
}

func TestNumberFieldRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiinput.ResolveNumberFieldRecipe(ctx)
	if !allNumberFieldFieldsPresent(slots) {
		t.Fatalf("number field slots contain zero values: %#v", slots)
	}
	if report.Family != "uiinput" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 12 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func TestNumberFieldGoldenDefault(t *testing.T) {
	assertNumberFieldGolden(t, "default", func(nf *NumberField) {})
}

func TestNumberFieldGoldenCompact(t *testing.T) {
	assertNumberFieldGolden(t, "compact", func(nf *NumberField) {
		nf.Layout.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, nf.Layout.MeasuredSize.W, nf.Layout.MeasuredSize.H))
	})
}

func TestNumberFieldGoldenHovered(t *testing.T) {
	assertNumberFieldGolden(t, "hovered", func(nf *NumberField) {
		nf.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestNumberFieldGoldenPressed(t *testing.T) {
	assertNumberFieldGolden(t, "pressed", func(nf *NumberField) {
		nf.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
	})
}

func TestNumberFieldGoldenFocused(t *testing.T) {
	assertNumberFieldGolden(t, "focused", func(nf *NumberField) {
		nf.onFocusGained()
	})
}

func TestNumberFieldGoldenDisabled(t *testing.T) {
	assertNumberFieldGolden(t, "disabled", func(nf *NumberField) {
		nf.Disabled = marks.Const(true)
	})
}

func TestNumberFieldGoldenRTL(t *testing.T) {
	ltr := renderNumberFieldSurface(t, layout.WritingDirectionLTR, "number_field_default", func(nf *NumberField) {})
	rtl := renderNumberFieldSurface(t, layout.WritingDirectionRTL, "number_field_rtl", func(nf *NumberField) {})
	testkit.AssertGoldenPair(t, ltr, rtl, "number_field")
}

func renderNumberFieldSurface(t *testing.T, direction layout.WritingDirection, goldenName string, mutate func(*NumberField)) *testkit.MemorySurface {
	t.Helper()
	nf := NewNumberField("Amount")
	nf.Precision = marks.Const(2)
	nf.Value.Set(123.45)
	nf.HelperText = marks.Const("Enter a quantity.")
	nf.WarningText = marks.Const("Quantity is advisory.")
	if mutate != nil {
		mutate(nf)
	}
	reg := testkit.TestFontRegistry(t)
	rt := numberFieldRuntimeStub{fonts: reg}
	facet.Attach(nf, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext().WithWritingDirection(direction)})
	result := nf.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            theme.DefaultResolvedContext().WithWritingDirection(direction),
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 240}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	nf.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext().WithWritingDirection(direction)}, bounds)
	cmds := nf.Projection.Project(facet.ProjectionContext{Runtime: rt, Bounds: bounds, ContentScale: 1})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands for golden")
	}
	surface := testkit.NewMemorySurface(int(result.Size.W), int(result.Size.H))
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
	return surface
}

type numberFieldRuntimeStub struct {
	fonts *text.FontRegistry
}

func (numberFieldRuntimeStub) Schedule(j job.AnyJob)                                              {}
func (numberFieldRuntimeStub) CancelJob(id job.JobID)                                             {}
func (numberFieldRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {}
func (s numberFieldRuntimeStub) FontRegistry() *text.FontRegistry                                 { return s.fonts }

func assertNumberFieldGolden(t *testing.T, name string, mutate func(*NumberField)) {
	t.Helper()
	surface := renderNumberFieldSurface(t, layout.WritingDirectionLTR, "number_field_"+name, mutate)
	testkit.AssertGolden(t, surface, "number_field_"+name)
}

func allNumberFieldFieldsPresent[T any](value T) bool {
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
