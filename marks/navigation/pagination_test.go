package navigation

import (
	"strconv"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestPaginationMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	pagination, rt, measureCtx := newPaginationTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(pagination, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := pagination.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 320}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(14, 12, result.Size.W, result.Size.H)
	pagination.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: pagination.Layout.Parent,
		ChildGroup:  pagination.Layout.Child,
	}, bounds)
	if got := pagination.AccessibilityRole(); got != "navigation" {
		t.Fatalf("accessibility role = %q, want navigation", got)
	}
	if got := pagination.AccessibleName(); got != "Pages" {
		t.Fatalf("accessible name = %q, want Pages", got)
	}
	if !pagination.Focus.Focusable() {
		t.Fatal("expected pagination to be focusable")
	}
	if len(pagination.Children()) == 0 {
		t.Fatal("expected pagination child facets")
	}
	if len(pagination.cachedEntryBounds) == 0 {
		t.Fatal("expected arranged entry bounds")
	}
	if idx := pagination.currentVisibleEntryIndex(); idx < 0 {
		t.Fatal("expected current page to be visible")
	}
	pageHit := pagination.Hit.HitTest(gfx.Point{
		X: pagination.cachedEntryBounds[pagination.currentVisibleEntryIndex()].Min.X + 2,
		Y: pagination.cachedEntryBounds[pagination.currentVisibleEntryIndex()].Min.Y + 2,
	})
	if !pageHit.Hit || (pageHit.MarkID != paginationMarkIDCurrentIndicator && pageHit.MarkID != paginationMarkIDPageItems) {
		t.Fatalf("expected page hit, got %#v", pageHit)
	}
	anchors := pagination.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := pagination.Projection.Project(facet.ProjectionContext{
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

func TestPaginationPointerKeyboardAndFocus(t *testing.T) {
	pagination, rt, measureCtx := newPaginationTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(pagination, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := pagination.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 320}})
	pagination.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: pagination.Layout.Parent,
		ChildGroup:  pagination.Layout.Child,
	}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))

	activated := -1
	pagination.Activated.Subscribe(func(index int) { activated = index })

	nextCenter := gfx.Point{
		X: pagination.cachedEntryBounds[len(pagination.cachedEntryBounds)-1].Min.X + pagination.cachedEntryBounds[len(pagination.cachedEntryBounds)-1].Width()*0.5,
		Y: pagination.cachedEntryBounds[len(pagination.cachedEntryBounds)-1].Min.Y + pagination.cachedEntryBounds[len(pagination.cachedEntryBounds)-1].Height()*0.5,
	}
	if !pagination.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: nextCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected next press to be handled")
	}
	if !pagination.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: nextCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected next release to be handled")
	}
	if activated < 0 {
		t.Fatal("expected activation signal on pointer release")
	}
	if !pagination.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if !pagination.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if !pagination.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	pagination.onFocusLost()
	pagination.onFocusGained()
	if !pagination.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	pagination.Disabled = marks.Const(true)
	if pagination.Focus.Focusable() {
		t.Fatal("expected disabled pagination to be unfocusable")
	}
}

func TestPaginationGoldenDefault(t *testing.T) {
	AssertPaginationGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {})
}

func TestPaginationGoldenCompact(t *testing.T) {
	AssertPaginationGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(p *Pagination) {})
}

func TestPaginationGoldenDisabled(t *testing.T) {
	AssertPaginationGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {
		p.Disabled = marks.Const(true)
	})
}

func TestPaginationGoldenHighContrast(t *testing.T) {
	AssertPaginationGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {})
}

func TestPaginationGoldenHovered(t *testing.T) {
	AssertPaginationGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {
		p.hoveredEntryIndex = 2
	})
}

func TestPaginationGoldenPressed(t *testing.T) {
	AssertPaginationGolden(t, "pressed", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {
		p.pressedEntryIndex = 2
	})
}

func TestPaginationGoldenFocused(t *testing.T) {
	AssertPaginationGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(p *Pagination) {
		p.onFocusGained()
	})
}

func TestPaginationGoldenRTL(t *testing.T) {
	AssertPaginationGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(p *Pagination) {})
}

func AssertPaginationGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Pagination)) {
	t.Helper()
	pagination, rt, measureCtx := newPaginationTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(pagination)
	}
	renderPaginationToSurface(t, pagination, rt, measureCtx, density, direction, name)
}

func renderPaginationToSurface(t *testing.T, pagination *Pagination, rt tabsRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(pagination, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := pagination.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1440, H: 320}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	pagination.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: pagination.Layout.Parent,
		ChildGroup:  pagination.Layout.Child,
	}, bounds)
	cmds := pagination.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	surfaceW := int(result.Size.W) + 1
	surfaceH := int(result.Size.H) + 1
	if surfaceW < 1 {
		surfaceW = 1
	}
	if surfaceH < 1 {
		surfaceH = 1
	}
	surface := testkit.NewMemorySurface(surfaceW, surfaceH)
	r := softwarerenderer.NewSoftwareRenderer()
	if err := r.Initialize(surface); err != nil {
		t.Fatalf("initialize renderer: %v", err)
	}
	commands := gfx.CommandList{}
	if cmds != nil {
		commands = *cmds
	}
	frame := &render.Frame{
		RenderBatchs: []render.RenderBatch{
			{
				ID:          1,
				Bounds:      bounds,
				Opacity:     1,
				CommandHash: 1,
				Commands:    commands,
			},
		},
	}
	if err := r.Submit(frame); err != nil {
		t.Fatalf("submit frame: %v", err)
	}
	testkit.AssertGolden(t, surface, "pagination_"+goldenName)
}

func newPaginationTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Pagination, tabsRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := testkit.TestFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	items := make([]PaginationItem, 0, 10)
	for i := 1; i <= 10; i++ {
		items = append(items, PaginationItem{
			Key:   "page-" + strconv.Itoa(i),
			Label: "test-item-" + strconv.Itoa(i),
		})
	}
	pagination := NewPagination("Pages", items)
	pagination.CurrentIndex = marks.Const(4)
	rt := tabsRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return pagination, rt, resolved
}
