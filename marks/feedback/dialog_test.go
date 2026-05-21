package feedback

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type dialogRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s dialogRuntimeStub) Schedule(j job.AnyJob)  {}
func (s dialogRuntimeStub) CancelJob(id job.JobID) {}
func (s dialogRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s dialogRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s dialogRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s dialogRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestDialogMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	dialog := newDialogFixture()
	tokens := dialogTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := dialogRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}

	facet.Attach(dialog, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := dialog.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 420, H: 280}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	dialog.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: dialog.layoutRole.Parent,
		ChildGroup:  dialog.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, bounds)

	if got := dialog.AccessibilityRole(); got != "dialog" {
		t.Fatalf("accessibility role = %q, want dialog", got)
	}
	if got := dialog.AccessibleName(); got != "Confirm destructive action" {
		t.Fatalf("accessible name = %q", got)
	}
	if len(dialog.Children()) != 4 {
		t.Fatalf("expected title, body, actions, and close children, got %d", len(dialog.Children()))
	}
	if dialog.cachedSurfaceBounds.IsEmpty() || dialog.cachedTitleBounds.IsEmpty() || dialog.cachedBodyBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got surface=%#v title=%#v body=%#v", dialog.cachedSurfaceBounds, dialog.cachedTitleBounds, dialog.cachedBodyBounds)
	}
	anchors := dialog.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := dialog.projectionRole.Project(facet.ProjectionContext{
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

func TestDialogInteractionsEmitActionAndDismiss(t *testing.T) {
	dialog := newDialogFixture()
	tokens := dialogTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := dialogRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}

	facet.Attach(dialog, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = dialog.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 420, H: 280}})
	dialog.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: dialog.layoutRole.Parent,
		ChildGroup:  dialog.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, gfx.RectFromXYWH(0, 0, dialog.layoutRole.MeasuredSize.W, dialog.layoutRole.MeasuredSize.H))

	var actioned, dismissed int
	dialog.Actioned.Subscribe(func(i int) { actioned = i + 1 })
	dialog.Dismissed.Subscribe(func(signal.Unit) { dismissed++ })

	actions := dialog.cachedActionsFacet
	if actions == nil || len(actions.Buttons) < 2 {
		t.Fatal("expected action buttons")
	}
	actionBounds := actions.Buttons[1].Base().LayoutRole().ArrangedBounds
	if !actions.Buttons[1].Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action press to be handled")
	}
	if !actions.Buttons[1].Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action release to be handled")
	}
	if actioned != 2 {
		t.Fatalf("expected second action emission, got %d", actioned)
	}
	if dismissed != 1 {
		t.Fatalf("expected dismiss emission after action, got %d", dismissed)
	}

	dialog.SetOpen(true)
	if !dialog.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected escape to be handled")
	}
	if dismissed != 2 {
		t.Fatalf("expected second dismiss emission, got %d", dismissed)
	}
}

func TestDialogRecipe_exposes_expected_slots(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uifeedback.ResolveDialogRecipe(ctx, uifeedback.DialogDefault)
	if report.Family != "uifeedback" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Backdrop", "ModalSurface", "Title", "Body", "Actions", "CloseButton", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if slots.Backdrop.Base.Fills == nil || slots.ModalSurface.Base.Fills == nil {
		t.Fatal("expected backdrop and surface fills")
	}
}

func TestDialogCustomContentVertical(t *testing.T) {
	dialog := newDialogFixture()
	dialog.SetContentChildren([]DialogContentChild{
		{Key: "alpha", Facet: primitive.NewText("Alpha")},
		{Key: "beta", Facet: primitive.NewText("Beta")},
	})
	assertDialogContentLayout(t, dialog, DialogContentLayoutVertical)
	if got := len(dialog.cachedBodyGroup.cachedMeasuredChildren); got != 3 {
		t.Fatalf("expected 3 measured body children, got %d", got)
	}
	if dialog.cachedBodyGroup.cachedMeasuredChildren[1].size.H <= 0 || dialog.cachedBodyGroup.cachedMeasuredChildren[2].size.H <= 0 {
		t.Fatal("expected custom body child sizes")
	}
	if !(dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[1].facet.ID()].Min.Y < dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[2].facet.ID()].Min.Y) {
		t.Fatal("expected vertical ordering for custom body content")
	}
}

func TestDialogCustomContentHorizontal(t *testing.T) {
	dialog := newDialogFixture()
	dialog.SetContentLayoutMode(DialogContentLayoutHorizontal)
	dialog.SetContentChildren([]DialogContentChild{
		{Key: "alpha", Facet: primitive.NewText("Alpha")},
		{Key: "beta", Facet: primitive.NewText("Beta")},
	})
	assertDialogContentLayout(t, dialog, DialogContentLayoutHorizontal)
	if got := len(dialog.cachedBodyGroup.cachedMeasuredChildren); got != 3 {
		t.Fatalf("expected 3 measured body children, got %d", got)
	}
	if !(dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[1].facet.ID()].Min.X < dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[2].facet.ID()].Min.X) {
		t.Fatal("expected horizontal ordering for custom body content")
	}
}

func TestDialogCustomContentGrid(t *testing.T) {
	dialog := newDialogFixture()
	dialog.SetContentLayoutMode(DialogContentLayoutGrid)
	dialog.SetContentGrid(2, 2)
	dialog.SetContentChildren([]DialogContentChild{
		{Key: "alpha", Facet: primitive.NewText("Alpha")},
		{Key: "beta", Facet: primitive.NewText("Beta")},
		{Key: "gamma", Facet: primitive.NewText("Gamma")},
	})
	assertDialogContentLayout(t, dialog, DialogContentLayoutGrid)
	if got := len(dialog.cachedBodyGroup.cachedMeasuredChildren); got != 4 {
		t.Fatalf("expected 4 measured body children, got %d", got)
	}
	first := dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[0].facet.ID()]
	second := dialog.cachedBodyGroup.cachedChildrenMap[dialog.cachedBodyGroup.cachedMeasuredChildren[1].facet.ID()]
	if !(first.Min.X <= second.Min.X) {
		t.Fatal("expected grid ordering to place the second child in the same or later column")
	}
}

func TestDialogGoldenDefault(t *testing.T) {
	AssertDialogGolden(t, "default", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {})
}

func TestDialogGoldenCompact(t *testing.T) {
	AssertDialogGolden(t, "compact", dialogTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(d *Dialog) {})
}

func TestDialogGoldenComfortable(t *testing.T) {
	AssertDialogGolden(t, "comfortable", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {})
}

func TestDialogGoldenDisabled(t *testing.T) {
	AssertDialogGolden(t, "disabled", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.SetDisabled(true)
	})
}

func TestDialogGoldenHighContrast(t *testing.T) {
	AssertDialogGolden(t, "high_contrast", dialogHighContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {})
}

func TestDialogGoldenHovered(t *testing.T) {
	AssertDialogGolden(t, "hovered", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.hovered = true
	})
}

func TestDialogGoldenPressed(t *testing.T) {
	AssertDialogGolden(t, "pressed", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.pressed = true
	})
}

func TestDialogGoldenFocused(t *testing.T) {
	AssertDialogGolden(t, "focused", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.focusedVisible = true
	})
}

func TestDialogGoldenRTL(t *testing.T) {
	AssertDialogGolden(t, "rtl", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(d *Dialog) {})
}

func TestDialogGoldenOpen(t *testing.T) {
	AssertDialogGolden(t, "open", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.SetOpen(true)
	})
}

func TestDialogGoldenCustomContentGrid(t *testing.T) {
	AssertDialogGolden(t, "content_grid", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.SetContentLayoutMode(DialogContentLayoutGrid)
		d.SetContentGrid(2, 2)
		d.SetContentChildren([]DialogContentChild{
			{Key: "alpha", Facet: primitive.NewText("Alpha")},
			{Key: "beta", Facet: primitive.NewText("Beta")},
			{Key: "gamma", Facet: primitive.NewText("Gamma")},
		})
	})
}

func TestDialogGoldenDismissed(t *testing.T) {
	AssertDialogGolden(t, "dismissed", dialogTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(d *Dialog) {
		d.SetOpen(false)
	})
}

func AssertDialogGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Dialog)) {
	t.Helper()
	dialog := newDialogFixture()
	rt := dialogRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}
	resolved := alertResolvedContext(tokens, density, direction)
	facet.Attach(dialog, facet.AttachContext{Runtime: rt, Theme: resolved})
	canvas := gfx.RectFromXYWH(0, 0, 420, 280)
	_ = dialog.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	dialog.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: dialog.layoutRole.Parent,
		ChildGroup:  dialog.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, canvas)
	if mutate != nil {
		mutate(dialog)
	}
	cmds := dialog.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		cmds = &gfx.CommandList{}
	}
	surface := testkit.NewMemorySurface(420, 280)
	renderer := softwarerenderer.NewSoftwareRenderer()
	if err := renderer.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{{
			ID:          1,
			Bounds:      canvas,
			Opacity:     1,
			Commands:    *cmds,
			CommandHash: 1,
		}},
	}
	if err := renderer.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "dialog_"+name)
}

func newDialogFixture() *Dialog {
	d := NewDialog(
		"Confirm destructive action",
		"This will permanently remove the selected items.",
		[]DialogAction{
			{Label: "Cancel", Variant: uiinput.ButtonOutlined},
			{Label: "Delete", Variant: uiinput.ButtonFilled},
		},
	)
	d.SetCloseButtonLabel("Close")
	return d
}

func assertDialogContentLayout(t *testing.T, dialog *Dialog, mode DialogContentLayoutMode) {
	t.Helper()
	tokens := dialogTokens()
	resolved := alertResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := dialogRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustAlertFontRegistry(t),
	}
	facet.Attach(dialog, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = dialog.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 420, H: 280}})
	dialog.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: dialog.layoutRole.Parent,
		ChildGroup:  dialog.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, gfx.RectFromXYWH(0, 0, dialog.layoutRole.MeasuredSize.W, dialog.layoutRole.MeasuredSize.H))
	if dialog.cachedBodyGroup == nil {
		t.Fatal("expected body group")
	}
	if dialog.cachedBodyBounds.IsEmpty() || dialog.cachedBodyGroup.cachedContentBounds.IsEmpty() {
		t.Fatalf("expected arranged body bounds, got body=%#v content=%#v", dialog.cachedBodyBounds, dialog.cachedBodyGroup.cachedContentBounds)
	}
	if len(dialog.cachedBodyGroup.cachedMeasuredChildren) == 0 {
		t.Fatal("expected measured body children")
	}
	_ = mode
}

func dialogTokens() theme.Tokens {
	tokens := theme.DefaultTokens()
	tokens.Color.Background = gfx.ColorFromRGBA8(232, 236, 244, 255)
	tokens.Color.Surface = gfx.ColorFromRGBA8(255, 255, 255, 255)
	tokens.Color.SurfaceVariant = gfx.ColorFromRGBA8(242, 245, 250, 255)
	tokens.Color.OnBackground = gfx.ColorFromRGBA8(27, 31, 39, 255)
	tokens.Color.OnSurface = gfx.ColorFromRGBA8(27, 31, 39, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(94, 101, 117, 255)
	tokens.Color.Primary = gfx.ColorFromRGBA8(61, 97, 228, 255)
	tokens.Color.OnPrimary = gfx.ColorFromRGBA8(255, 255, 255, 255)
	tokens.Color.DisabledOpacity = 0.42
	return tokens
}

func dialogHighContrastTokens() theme.Tokens {
	tokens := dialogTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(0, 61, 145, 255)
	tokens.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(0, 0, 0, 255)
	return tokens
}
