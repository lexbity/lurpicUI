package navigation

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

type breadcrumbRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s breadcrumbRuntimeStub) Schedule(j job.AnyJob)  {}
func (s breadcrumbRuntimeStub) CancelJob(id job.JobID) {}
func (s breadcrumbRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s breadcrumbRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s breadcrumbRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s breadcrumbRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestBreadcrumbsMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	bc, rt, measureCtx := newBreadcrumbsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(bc, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bc.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 900, H: 240}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(10, 16, result.Size.W, result.Size.H)
	bc.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: bc.layoutRole.Parent,
		ChildGroup:  bc.layoutRole.Child,
	}, bounds)

	if got := bc.AccessibilityRole(); got != "navigation" {
		t.Fatalf("accessibility role = %q, want navigation", got)
	}
	if got := bc.AccessibleName(); got != "Project trail" {
		t.Fatalf("accessible name = %q, want Project trail", got)
	}
	if bc.textRole.Layout == nil {
		t.Fatal("expected current breadcrumb text layout")
	}
	if bc.cachedSegmentListBounds.IsEmpty() || len(bc.cachedItemBounds) != len(bc.Items) {
		t.Fatalf("expected breadcrumb geometry, got list=%#v items=%d", bc.cachedSegmentListBounds, len(bc.cachedItemBounds))
	}

	linkHit := bc.hitRole.HitTest(gfx.Point{
		X: bc.cachedItemBounds[0].Min.X + bc.cachedItemBounds[0].Width()*0.5,
		Y: bc.cachedItemBounds[0].Min.Y + bc.cachedItemBounds[0].Height()*0.5,
	})
	if !linkHit.Hit || linkHit.MarkID != breadcrumbsMarkIDSegmentLink {
		t.Fatalf("expected link hit, got %#v", linkHit)
	}
	sepHit := bc.hitRole.HitTest(gfx.Point{
		X: bc.cachedSeparatorBounds[0].Min.X + bc.cachedSeparatorBounds[0].Width()*0.5,
		Y: bc.cachedSeparatorBounds[0].Min.Y + bc.cachedSeparatorBounds[0].Height()*0.5,
	})
	if !sepHit.Hit || sepHit.MarkID != breadcrumbsMarkIDSeparator {
		t.Fatalf("expected separator hit, got %#v", sepHit)
	}
	currentHit := bc.hitRole.HitTest(gfx.Point{
		X: bc.cachedItemBounds[3].Min.X + bc.cachedItemBounds[3].Width()*0.5,
		Y: bc.cachedItemBounds[3].Min.Y + bc.cachedItemBounds[3].Height()*0.5,
	})
	if !currentHit.Hit || currentHit.MarkID != breadcrumbsMarkIDCurrentSegment {
		t.Fatalf("expected current segment hit, got %#v", currentHit)
	}

	anchors := bc.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := bc.projectionRole.Project(facet.ProjectionContext{
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

func TestBreadcrumbsPointerAndKeyboardInteraction(t *testing.T) {
	bc, rt, measureCtx := newBreadcrumbsTestFixture(t, defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)
	facet.Attach(bc, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bc.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 900, H: 240}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bc.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)

	activated := -1
	bc.Activated.Subscribe(func(index int) {
		activated = index
	})

	linkCenter := gfx.Point{
		X: bc.cachedItemBounds[1].Min.X + bc.cachedItemBounds[1].Width()*0.5,
		Y: bc.cachedItemBounds[1].Min.Y + bc.cachedItemBounds[1].Height()*0.5,
	}
	if !bc.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: linkCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !bc.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: linkCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != 1 {
		t.Fatalf("expected activated link 1, got %d", activated)
	}

	currentCenter := gfx.Point{
		X: bc.cachedItemBounds[3].Min.X + bc.cachedItemBounds[3].Width()*0.5,
		Y: bc.cachedItemBounds[3].Min.Y + bc.cachedItemBounds[3].Height()*0.5,
	}
	if bc.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: currentCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected current breadcrumb press to be ignored")
	}

	bc.onFocusLost()
	bc.onFocusGained()
	if !bc.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	if !bc.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if !bc.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected second right key to be handled")
	}
	if bc.clampedFocusedIndex() != 2 {
		t.Fatalf("expected focused index to move to 2, got %d", bc.clampedFocusedIndex())
	}
	if !bc.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter press to be handled")
	}
	if !bc.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter release to be handled")
	}
	if activated != 2 {
		t.Fatalf("expected activated link 2 after keyboard, got %d", activated)
	}
}

func TestBreadcrumbsGoldenDefault(t *testing.T) {
	AssertBreadcrumbsGolden(t, "default", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {})
}

func TestBreadcrumbsGoldenCompact(t *testing.T) {
	AssertBreadcrumbsGolden(t, "compact", defaultTabsTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(b *Breadcrumbs) {})
}

func TestBreadcrumbsGoldenComfortable(t *testing.T) {
	AssertBreadcrumbsGolden(t, "comfortable", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {})
}

func TestBreadcrumbsGoldenDisabled(t *testing.T) {
	AssertBreadcrumbsGolden(t, "disabled", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {
		b.SetDisabled(true)
	})
}

func TestBreadcrumbsGoldenHighContrast(t *testing.T) {
	AssertBreadcrumbsGolden(t, "high_contrast", highContrastTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {})
}

func TestBreadcrumbsGoldenHovered(t *testing.T) {
	AssertBreadcrumbsGolden(t, "hovered", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {
		b.hoveredIndex = 1
	})
}

func TestBreadcrumbsGoldenFocused(t *testing.T) {
	AssertBreadcrumbsGolden(t, "focused", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *Breadcrumbs) {
		b.onFocusGained()
	})
}

func TestBreadcrumbsGoldenRTL(t *testing.T) {
	AssertBreadcrumbsGolden(t, "rtl", defaultTabsTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(b *Breadcrumbs) {})
}

func AssertBreadcrumbsGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Breadcrumbs)) {
	t.Helper()
	bc, rt, measureCtx := newBreadcrumbsTestFixture(t, tokens, density, direction)
	if mutate != nil {
		mutate(bc)
	}
	renderBreadcrumbsToSurface(t, bc, rt, measureCtx, density, direction, name)
}

func renderBreadcrumbsToSurface(t *testing.T, bc *Breadcrumbs, rt breadcrumbRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(bc, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := bc.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1800, H: 1200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bc.layoutRole.Arrange(facet.ArrangeContext{Runtime: rt, Theme: measureCtx}, bounds)
	cmds := bc.projectionRole.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "breadcrumbs_"+goldenName)
}

func newBreadcrumbsTestFixture(t *testing.T, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) (*Breadcrumbs, breadcrumbRuntimeStub, theme.ResolvedContext) {
	t.Helper()
	fonts := mustTabsFontRegistry(t)
	rtTokens := tokens
	rtTokens.Density.Mode = densityToTemplateMode(density)
	rootStyle := theme.NewRootStyleContext(nil, rtTokens, nil)
	resolved := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction)
	bc := NewBreadcrumbs("Project trail", []BreadcrumbItem{
		{Label: "Breadcrumb 1"},
		{Label: "Breadcrumb 2"},
		{Label: "Breadcrumb 3"},
		{Label: "Breadcrumb 4"},
	})
	rt := breadcrumbRuntimeStub{rootStyle: rootStyle, fonts: fonts}
	return bc, rt, resolved
}

func defaultBreadcrumbTokens() theme.Tokens {
	return defaultTabsTokens()
}

func highContrastBreadcrumbTokens() theme.Tokens {
	t := defaultBreadcrumbTokens()
	t.Color.Surface = gfx.ColorFromRGBA8(255, 255, 255, 255)
	t.Color.OnSurface = gfx.ColorFromRGBA8(0, 0, 0, 255)
	t.Color.OnSurfaceVariant = gfx.ColorFromRGBA8(40, 40, 40, 255)
	t.Color.Primary = gfx.ColorFromRGBA8(0, 90, 220, 255)
	return t
}
