package action

import (
	"math"
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type commandPaletteRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
	registry  *runtimepkg.CommandRegistry
}

func (s commandPaletteRuntimeStub) Schedule(j job.AnyJob)  {}
func (s commandPaletteRuntimeStub) CancelJob(id job.JobID) {}
func (s commandPaletteRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s commandPaletteRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s commandPaletteRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s commandPaletteRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }
func (s commandPaletteRuntimeStub) CommandRegistry() *runtimepkg.CommandRegistry {
	return s.registry
}

func TestCommandPaletteMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	palette, rt, resolved := newCommandPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := palette.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 720}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	palette.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: resolved, ParentGroup: palette.layoutRole.Parent, ChildGroup: palette.layoutRole.Child}, bounds)

	if got := palette.AccessibilityRole(); got != "dialog_combobox" {
		t.Fatalf("accessibility role = %q, want dialog_combobox", got)
	}
	if got := palette.AccessibleName(); got != "Command palette" {
		t.Fatalf("accessible name = %q, want Command palette", got)
	}
	if len(palette.Children()) != 2 {
		t.Fatalf("expected search field and results list children, got %d", len(palette.Children()))
	}
	if palette.cachedSurfaceBounds.IsEmpty() || palette.cachedSearchBounds.IsEmpty() || palette.cachedResultsBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got surface=%#v search=%#v results=%#v", palette.cachedSurfaceBounds, palette.cachedSearchBounds, palette.cachedResultsBounds)
	}

	searchHit := palette.hitRole.HitTest(gfx.Point{X: palette.cachedSearchBounds.Min.X + 1, Y: palette.cachedSearchBounds.Min.Y + 1})
	if !searchHit.Hit || searchHit.MarkID != commandPaletteMarkIDSearchField {
		t.Fatalf("expected search-field hit, got %#v", searchHit)
	}
	backdropHit := palette.hitRole.HitTest(gfx.Point{X: bounds.Max.X - 1, Y: bounds.Max.Y - 1})
	if !backdropHit.Hit || backdropHit.MarkID != commandPaletteMarkIDBackdrop {
		t.Fatalf("expected backdrop hit, got %#v", backdropHit)
	}

	anchors := palette.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := palette.projectionRole.Project(facet.ProjectionContext{
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
		case gfx.FillPath, gfx.StrokePath:
			sawFillPath = true
		}
	}
	if !sawGlyphRun {
		t.Fatal("expected glyph commands")
	}
	if !sawFillPath {
		t.Fatal("expected fill/stroke commands")
	}
}

func TestCommandPaletteSearchNavigationAndActivation(t *testing.T) {
	palette, rt, resolved := newCommandPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = palette.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        resolved,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 720}})
	palette.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: resolved, ParentGroup: palette.layoutRole.Parent, ChildGroup: palette.layoutRole.Child}, gfx.RectFromXYWH(0, 0, 1280, 720))

	var activated string
	var executed string
	palette.Activated.Subscribe(func(id string) { activated = id })
	if entry, ok := palette.registry.Lookup("edit.find"); ok {
		entry.Execute = func() { executed = entry.ID }
		palette.registry.Register(entry)
	}

	if palette.searchField.Value.Get() != "" {
		t.Fatalf("expected empty query, got %q", palette.searchField.Value.Get())
	}
	palette.searchField.Value.Set("find")
	if len(palette.cachedFiltered) != 1 {
		t.Fatalf("expected one filtered command, got %d", len(palette.cachedFiltered))
	}
	if got := palette.cachedFiltered[0].ID; got != "edit.find" {
		t.Fatalf("filtered command = %q, want edit.find", got)
	}

	palette.searchField.Base().InputRole().OnKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter})
	if activated != "edit.find" {
		t.Fatalf("activated command = %q, want edit.find", activated)
	}
	if executed != "edit.find" {
		t.Fatalf("executed command = %q, want edit.find", executed)
	}
	if palette.Open {
		t.Fatal("expected palette to close after activation")
	}
}

func TestCommandPaletteDisabledBehavior(t *testing.T) {
	palette, rt, resolved := newCommandPaletteFixture(t, theme.DefaultTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: resolved})
	palette.SetDisabled(true)
	if palette.focusRole.Focusable() {
		t.Fatal("expected disabled palette to be unfocusable")
	}
	if palette.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled palette to ignore pointer input")
	}
	if palette.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEscape}) {
		t.Fatal("expected disabled palette to ignore keyboard input")
	}
}

func TestCommandPaletteRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveCommandPaletteRecipe(ctx)
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	for _, name := range []string{"Root", "Backdrop", "ModalSurface", "SearchField", "ResultsList", "ResultItem", "ShortcutLabel", "EmptyState", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if !allCommandPaletteFieldsPresent(slots) {
		t.Fatalf("command palette slots contain zero values: %#v", slots)
	}
}

func TestCommandPaletteGoldenDefault(t *testing.T) {
	AssertCommandPaletteGolden(t, "default", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*CommandPalette) {})
}

func TestCommandPaletteGoldenCompact(t *testing.T) {
	AssertCommandPaletteGolden(t, "compact", defaultCommandPaletteTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(*CommandPalette) {})
}

func TestCommandPaletteGoldenComfortable(t *testing.T) {
	AssertCommandPaletteGolden(t, "comfortable", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*CommandPalette) {})
}

func TestCommandPaletteGoldenDisabled(t *testing.T) {
	AssertCommandPaletteGolden(t, "disabled", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.SetDisabled(true)
	})
}

func TestCommandPaletteGoldenHighContrast(t *testing.T) {
	AssertCommandPaletteGolden(t, "high_contrast", highContrastCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(*CommandPalette) {})
}

func TestCommandPaletteGoldenHovered(t *testing.T) {
	AssertCommandPaletteGolden(t, "hovered", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.hovered = true
	})
}

func TestCommandPaletteGoldenPressed(t *testing.T) {
	AssertCommandPaletteGolden(t, "pressed", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.pressed = true
	})
}

func TestCommandPaletteGoldenFocused(t *testing.T) {
	AssertCommandPaletteGolden(t, "focused", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.focusedVisible = true
	})
}

func TestCommandPaletteGoldenRTL(t *testing.T) {
	AssertCommandPaletteGolden(t, "rtl", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(*CommandPalette) {})
}

func TestCommandPaletteGoldenOpen(t *testing.T) {
	AssertCommandPaletteGolden(t, "open", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.SetOpen(true)
	})
}

func TestCommandPaletteGoldenDismissed(t *testing.T) {
	AssertCommandPaletteGolden(t, "dismissed", defaultCommandPaletteTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *CommandPalette) {
		p.SetOpen(false)
	})
}

func AssertCommandPaletteGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*CommandPalette)) {
	t.Helper()
	palette, rt, measureCtx := newCommandPaletteGoldenFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(palette)
	}
	renderCommandPaletteToSurface(t, palette, rt, measureCtx, density, direction, name)
}

func renderCommandPaletteToSurface(t *testing.T, palette *CommandPalette, rt commandPaletteRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(palette, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := palette.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 720}})
	bounds := gfx.RectFromXYWH(0, 0, 1280, 720)
	if result.Size.W > 0 && result.Size.H > 0 {
		bounds = gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	}
	palette.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx, ParentGroup: palette.layoutRole.Parent, ChildGroup: palette.layoutRole.Child}, bounds)

	cmds := palette.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil {
		cmds = &gfx.CommandList{}
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
	testkit.AssertGolden(t, surface, "command_palette_"+goldenName)
}

func newCommandPaletteFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*CommandPalette, commandPaletteRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	registry := runtimepkg.NewCommandRegistry()
	registry.Register(runtimepkg.CommandEntry{
		ID:       "edit.find",
		Title:    "Find in Files",
		Category: "Edit",
		Shortcut: "Ctrl+Shift+F",
		Execute:  func() {},
	})
	registry.Register(runtimepkg.CommandEntry{
		ID:       "view.toggle",
		Title:    "Toggle Sidebar",
		Category: "View",
		Shortcut: "Ctrl+B",
	})
	registry.Register(runtimepkg.CommandEntry{
		ID:       "git.open",
		Title:    "Open Repository",
		Category: "Git",
		Shortcut: "Ctrl+Shift+G",
	})
	rtTokens := tokens
	rtTokens.Density.Mode = actionBarDensityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	palette := NewCommandPalette("Command palette", registry)
	rt := commandPaletteRuntimeStub{
		rootStyle: rootStyle,
		fonts:     mustButtonTextRegistry(t),
		registry:  registry,
	}
	return palette, rt, resolved
}

func newCommandPaletteGoldenFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*CommandPalette, commandPaletteRuntimeStub, theme.ResolvedContext) {
	return newCommandPaletteFixture(t, tokens, density, direction)
}

func defaultCommandPaletteTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func highContrastCommandPaletteTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func allCommandPaletteFieldsPresent[T any](value T) bool {
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
