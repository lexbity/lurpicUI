package input

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type textFieldRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s textFieldRuntimeStub) Schedule(j job.AnyJob)  {}
func (s textFieldRuntimeStub) CancelJob(id job.JobID) {}
func (s textFieldRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s textFieldRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s textFieldRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s textFieldRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestTextFieldMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	tf := NewTextField("Email", uiinput.TextInputOutlined)
	tf.SetPlaceholder("name@example.com")
	tf.SetHelperText("We will not share this address.")
	rt := textFieldRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustTextFieldRegistry(t),
	}

	facet.Attach(tf, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := tf.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 480, H: 220}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(18, 24, result.Size.W, result.Size.H)
	tf.layoutRole.Arrange(facet.ArrangeContext{}, bounds)

	if got := tf.AccessibilityRole(); got != "textbox" {
		t.Fatalf("accessibility role = %q, want textbox", got)
	}
	if got := tf.AccessibleName(); got != "Email" {
		t.Fatalf("accessible name = %q, want Email", got)
	}
	if !tf.textRole.IMEEnabled {
		t.Fatal("expected IME to be enabled")
	}

	labelHit := tf.hitRole.HitTest(gfx.Point{X: tf.cachedLabelBounds.Min.X + 1, Y: tf.cachedLabelBounds.Min.Y + 1})
	if !labelHit.Hit || labelHit.MarkID != textFieldMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", labelHit)
	}
	fieldHit := tf.hitRole.HitTest(gfx.Point{X: tf.cachedFieldBounds.Min.X + 1, Y: tf.cachedFieldBounds.Min.Y + 1})
	if !fieldHit.Hit || fieldHit.MarkID != textFieldMarkIDPlaceholder {
		t.Fatalf("expected placeholder hit in empty field, got %#v", fieldHit)
	}
	containerHit := tf.hitRole.HitTest(gfx.Point{X: bounds.Max.X - 2, Y: tf.cachedLabelBounds.Max.Y + tf.cachedGap*0.5})
	if !containerHit.Hit || containerHit.MarkID != textFieldMarkIDContainer {
		t.Fatalf("expected container hit, got %#v", containerHit)
	}

	anchors := tf.ExportAnchors(layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: bounds},
	})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := tf.projectionRole.Project(facet.ProjectionContext{
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
		t.Fatal("expected text glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected surface/fill commands")
	}
}

func TestTextFieldStoreChangeAndEditing(t *testing.T) {
	tf := NewTextField("Name", uiinput.TextInputFilled)
	rt := textFieldRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     mustTextFieldRegistry(t),
	}

	facet.Attach(tf, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	_ = tf.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 180}})
	tf.layoutRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, tf.layoutRole.MeasuredSize.W, tf.layoutRole.MeasuredSize.H))

	initialVersions := tf.Base().SubscribedVersions()
	if len(initialVersions) != 1 {
		t.Fatalf("expected one tracked store version, got %d", len(initialVersions))
	}

	tf.Value.Set("Alice")
	if flags := tf.Base().DirtyFlags(); flags&(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit) == 0 {
		t.Fatalf("expected dirty flags after store update, got %#v", flags)
	}
	updatedVersions := tf.Base().SubscribedVersions()
	if updatedVersions[0] <= initialVersions[0] {
		t.Fatalf("expected tracked version to advance, before=%v after=%v", initialVersions, updatedVersions)
	}

	_ = tf.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 360, H: 180}})
	tf.layoutRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, tf.layoutRole.MeasuredSize.W, tf.layoutRole.MeasuredSize.H))

	if !tf.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: tf.cachedFieldBounds.Min.X + 4, Y: tf.cachedFieldBounds.Min.Y + 4}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !tf.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: tf.cachedFieldBounds.Min.X + 4, Y: tf.cachedFieldBounds.Min.Y + 4}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	tf.setCaretAtEnd(false)
	if !tf.onText(facet.TextEvent{Text: "!"}) {
		t.Fatal("expected text input to be handled")
	}
	if got := tf.currentValue(); got != "Alice!" {
		t.Fatalf("value = %q, want Alice!", got)
	}

	tf.focusFromPointer = false
	tf.onFocusGained()
	if !tf.shouldShowCaret() {
		t.Fatal("expected focused caret visibility")
	}
	if !tf.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyBackspace}) {
		t.Fatal("expected backspace to be handled")
	}
	if got := tf.currentValue(); got != "Alice" {
		t.Fatalf("value after backspace = %q, want Alice", got)
	}
}

func TestTextFieldGraphemeBackspaceDeletesWholeCluster(t *testing.T) {
	tf := NewTextField("Name", uiinput.TextInputFilled)
	content := "a\u0301b"
	tf.Value.Set(content)
	tf.cachedValueLayout = textLayoutForTest(t, content)
	tf.caret = text.GraphemePosition(1, text.AffinityDownstream)
	if !tf.deleteBackward() {
		t.Fatal("expected deleteBackward to handle grapheme cluster")
	}
	if got := tf.currentValue(); got != "b" {
		t.Fatalf("value = %q, want b", got)
	}
	if tf.caret.Unit != text.TextUnitGrapheme || tf.caret.Index != 0 {
		t.Fatalf("caret = %#v", tf.caret)
	}
}

func TestTextFieldGoldenDefault(t *testing.T) {
	AssertTextFieldGolden(t, "default", func(tf *TextField) {})
}

func TestTextFieldGoldenFocused(t *testing.T) {
	AssertTextFieldGolden(t, "focused", func(tf *TextField) {
		tf.onFocusGained()
	})
}

func TestTextFieldGoldenDisabled(t *testing.T) {
	AssertTextFieldGolden(t, "disabled", func(tf *TextField) {
		tf.SetDisabled(true)
	})
}

func AssertTextFieldGolden(t *testing.T, name string, mutate func(*TextField)) {
	t.Helper()
	tf := NewTextField("Email", uiinput.TextInputOutlined)
	tf.SetPlaceholder("name@example.com")
	tf.SetHelperText("We will never share this value.")
	tf.SetValidation(TextFieldValidationWarning)
	tf.SetWarningText("This will be visible in the golden state.")
	if mutate != nil {
		mutate(tf)
	}
	fontData := mustReadTextFieldFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	cfg := testkit.HarnessConfig{
		Width:         420,
		Height:        220,
		LayerRegistry: mustTextFieldLayerRegistry(t),
		Fonts:         []text.FontSource{{Name: "noto-sans-regular", Data: fontData}},
	}
	h := testkit.NewHarness(t, cfg, tf)
	h.RunFrame()
	testkit.AssertGolden(t, h.Surface(), "text_field_"+name)
}

func mustTextFieldLayerRegistry(t *testing.T) *layout.LayerRegistry {
	t.Helper()
	reg, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	return reg
}

func mustTextFieldRegistry(t *testing.T) *text.FontRegistry {
	t.Helper()
	reg, err := text.NewFontRegistry()
	if err != nil {
		t.Fatalf("new font registry: %v", err)
	}
	data := mustReadTextFieldFont(t, "github.com/go-text/render@v0.2.0/testdata/NotoSans-Regular.ttf")
	if err := reg.LoadFontBytes(data, "noto-sans-regular"); err != nil {
		t.Fatalf("load font: %v", err)
	}
	return reg
}

func textLayoutForTest(t *testing.T, content string) *text.TextLayout {
	t.Helper()
	reg := mustTextFieldRegistry(t)
	shaper := text.NewShaper(reg)
	style := text.DefaultStyle()
	style.Family = "noto-sans-regular"
	return shaper.ShapeSimple(content, style)
}

func mustReadTextFieldFont(t *testing.T, rel string) []byte {
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
