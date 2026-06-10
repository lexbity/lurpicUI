package action

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type iconButtonRuntimeStub struct {
	rootStyle any
	icons     map[string]runtimepkg.IconAsset
}

func (s iconButtonRuntimeStub) Schedule(j job.AnyJob)  {}
func (s iconButtonRuntimeStub) CancelJob(id job.JobID) {}
func (s iconButtonRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s iconButtonRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s iconButtonRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s iconButtonRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := s.icons[ref]
	return asset, ok
}

const iconButtonTestSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M12 4v16M4 12h16"/></svg>`

func TestIconButtonMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	btn := NewIconButton(primitive.IconSVG(iconButtonTestSVG))
	btn.AccessibleLabel = marks.Const("Add")
	btn.Size = marks.Const(float32(24))
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
	}

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 24, result.Size.W, result.Size.H)
	btn.Layout.Arrange(facet.ArrangeContext{}, bounds)

	if got := btn.AccessibilityRole(); got != "button" {
		t.Fatalf("accessibility role = %q, want button", got)
	}
	if got := btn.AccessibleName(); got != "Add" {
		t.Fatalf("accessible name = %q, want Add", got)
	}

	if btn.cachedIconBounds.IsEmpty() {
		t.Fatal("expected icon bounds to be arranged")
	}
	if btn.cachedIconBounds.Width() >= bounds.Width() {
		t.Fatalf("expected icon bounds smaller than container, got icon=%#v container=%#v", btn.cachedIconBounds, bounds)
	}

	cmds := btn.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawFillPath, sawStrokePath bool
	for _, cmd := range cmds.Commands {
		switch cmd.(type) {
		case gfx.FillPath:
			sawFillPath = true
		case gfx.StrokePath:
			sawStrokePath = true
		}
	}
	if !sawFillPath {
		t.Fatal("expected fill path commands")
	}
	if !sawStrokePath {
		t.Fatal("expected stroke path commands")
	}

	anchors := btn.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	iconHit := btn.Hit.HitTest(gfx.Point{X: btn.cachedIconBounds.Min.X + 1, Y: btn.cachedIconBounds.Min.Y + 1})
	if !iconHit.Hit || iconHit.MarkID != iconButtonMarkIDIcon {
		t.Fatalf("expected icon hit, got %#v", iconHit)
	}
	containerHit := btn.Hit.HitTest(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1})
	if !containerHit.Hit || containerHit.MarkID != iconButtonMarkIDContainer {
		t.Fatalf("expected container hit, got %#v", containerHit)
	}
}

func TestIconButtonActivatesFocusAndDisabledBehavior(t *testing.T) {
	btn := NewIconButton(primitive.IconSVG(iconButtonTestSVG))
	btn.AccessibleLabel = marks.Const("Add")
	btn.Size = marks.Const(float32(24))
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
	}

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	btn.Layout.Arrange(facet.ArrangeContext{}, bounds)

	var activated int
	btn.Activated.Subscribe(func(signal.Unit) {
		activated++
	})

	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}}) {
		t.Fatal("expected pointer enter to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != 1 {
		t.Fatalf("expected one pointer activation, got %d", activated)
	}

	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space key press to be handled")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space key release to be handled")
	}
	if activated != 2 {
		t.Fatalf("expected two activations, got %d", activated)
	}

	btn.onFocusLost()
	btn.onFocusGained()
	if !btn.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}
	if btn.cursorShape() != facet.CursorPointer {
		t.Fatalf("expected pointer cursor, got %v", btn.cursorShape())
	}

	btn.Disabled = marks.Const(true)
	if btn.Focus.Focusable() {
		t.Fatal("expected disabled icon button to be unfocusable")
	}
	if btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled button to ignore pointer input")
	}
	if btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled button to ignore keyboard input")
	}
	if btn.cursorShape() != facet.CursorDefault {
		t.Fatalf("expected default cursor when disabled, got %v", btn.cursorShape())
	}
}

func TestIconButtonDensityBehaviorChangesSize(t *testing.T) {
	btn := NewIconButton(primitive.IconSVG(iconButtonTestSVG))
	btn.Size = marks.Const(float32(24))
	comfortable := theme.DefaultResolvedContext()
	compact := comfortable.WithDensity(theme.DefaultDensityScale(theme.DensityIDCompact, theme.DefaultTokens()))
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
	}

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: comfortable})
	comfortableSize := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        comfortable,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size

	compactSize := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        compact,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size

	if !(compactSize.W < comfortableSize.W) {
		t.Fatalf("expected compact size to shrink, comfortable=%#v compact=%#v", comfortableSize, compactSize)
	}
}

func TestIconButtonRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiinput.ResolveIconButtonRecipe(ctx, uiinput.IconButtonStandard)
	if !allIconButtonFieldsPresent(slots) {
		t.Fatalf("icon button slots contain zero values: %#v", slots)
	}
	if report.Family != "uiinput" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 5 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func TestIconButtonGoldenDefault(t *testing.T) {
	assertIconButtonGolden(t, "default", func(btn *IconButton) {})
}

func TestIconButtonGoldenHovered(t *testing.T) {
	assertIconButtonGolden(t, "hovered", func(btn *IconButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerEnter, Position: gfx.Point{X: 1, Y: 1}})
	})
}

func TestIconButtonGoldenPressed(t *testing.T) {
	assertIconButtonGolden(t, "pressed", func(btn *IconButton) {
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
	})
}

func TestIconButtonGoldenFocused(t *testing.T) {
	assertIconButtonGolden(t, "focused", func(btn *IconButton) {
		btn.onFocusGained()
	})
}

func TestIconButtonGoldenDisabled(t *testing.T) {
	assertIconButtonGolden(t, "disabled", func(btn *IconButton) {
		btn.Disabled = marks.Const(true)
	})
}

func TestIconButtonGoldenSkeuomorphic(t *testing.T) {
	assertIconButtonSkeuomorphicGolden(t, "skeuomorphic", func(btn *IconButton) {
		btn.Variant = marks.Const(uiinput.IconButtonSkeuomorphic)
	})
}

func TestIconButtonGoldenSkeuomorphicPressed(t *testing.T) {
	assertIconButtonSkeuomorphicGolden(t, "skeuomorphic_pressed", func(btn *IconButton) {
		btn.Variant = marks.Const(uiinput.IconButtonSkeuomorphic)
		btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft})
	})
}

func assertIconButtonSkeuomorphicGolden(t *testing.T, name string, mutate func(*IconButton)) {
	t.Helper()
	btn := NewIconButton(primitive.IconSVG(iconButtonTestSVG))
	btn.AccessibleLabel = marks.Const("Add")
	btn.Size = marks.Const(float32(24))
	if mutate != nil {
		mutate(btn)
	}

	measureCtx := theme.DefaultResolvedContext()
	rt := iconButtonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
	}
	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: measureCtx})

	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirection(layout.WritingDirectionLTR),
	}, facet.Constraints{MaxSize: gfx.Size{W: 320, H: 120}})

	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	surfaceW := 96
	surfaceH := 96
	x := mathutil.Max(0, float32(surfaceW)-result.Size.W) * 0.5
	y := mathutil.Max(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)

	btn.Layout.Arrange(facet.ArrangeContext{
		Runtime: rt,
		Theme:   measureCtx,
	}, bounds)

	cmds := btn.Projection.Project(facet.ProjectionContext{
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
	bgColor := measureCtx.TokenSet().Color.Background
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

	testkit.AssertGolden(t, surface, "icon_button_"+name)
}

func assertIconButtonGolden(t *testing.T, name string, mutate func(*IconButton)) {
	t.Helper()
	btn := NewIconButton(primitive.IconSVG(iconButtonTestSVG))
	btn.AccessibleLabel = marks.Const("Add")
	btn.Size = marks.Const(float32(24))
	if mutate != nil {
		mutate(btn)
	}
	cfg := testkit.HarnessConfig{
		Width:         96,
		Height:        96,
		LayerRegistry: mustIconButtonLayerRegistry(t),
	}
	h := testkit.NewHarness(t, cfg, btn)
	h.RunFrame()
	h.RunFrame()
	testkit.AssertGolden(t, h.Surface(), "icon_button_"+name)
}

func mustIconButtonLayerRegistry(t *testing.T) *layout.LayerRegistry {
	t.Helper()
	reg, err := layout.StandardLayerRegistry()
	if err != nil {
		t.Fatalf("standard layer registry: %v", err)
	}
	return reg
}

func allIconButtonFieldsPresent[T any](value T) bool {
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
