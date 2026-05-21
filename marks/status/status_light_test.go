package status

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestStatusLightMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	light := newStatusLightFixture()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, badgeTokens(), nil),
		fonts:     mustBadgeFontRegistry(t),
	}
	ctx := badgeResolvedContext(badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(light, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := light.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 240, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	light.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: light.layoutRole.Parent,
		ChildGroup:  light.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, bounds)
	if got := light.AccessibilityRole(); got != "status" {
		t.Fatalf("accessibility role = %q, want status", got)
	}
	if got := light.AccessibleName(); got != "" {
		t.Fatalf("accessible name = %q, want empty", got)
	}
	if light.cachedIndicatorBounds.IsEmpty() {
		t.Fatal("expected indicator bounds")
	}
	if light.cachedLabelBounds.IsEmpty() {
		t.Fatal("expected label bounds")
	}
	anchors := light.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "indicator", "label_optional"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := light.projectionRole.Project(facet.ProjectionContext{
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

func TestStatusLightGoldenDefault(t *testing.T) {
	AssertStatusLightGolden(t, "default", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *StatusLight) {})
}

func TestStatusLightGoldenCompact(t *testing.T) {
	AssertStatusLightGolden(t, "compact", badgeTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(s *StatusLight) {})
}

func TestStatusLightGoldenComfortable(t *testing.T) {
	AssertStatusLightGolden(t, "comfortable", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *StatusLight) {})
}

func TestStatusLightGoldenDisabled(t *testing.T) {
	AssertStatusLightGolden(t, "disabled", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *StatusLight) {
		s.SetDisabled(true)
	})
}

func TestStatusLightGoldenHighContrast(t *testing.T) {
	AssertStatusLightGolden(t, "high_contrast", highContrastBadgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(s *StatusLight) {})
}

func TestStatusLightGoldenRTL(t *testing.T) {
	AssertStatusLightGolden(t, "rtl", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(s *StatusLight) {})
}

func AssertStatusLightGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*StatusLight)) {
	t.Helper()
	light := newStatusLightFixture()
	if mutate != nil {
		mutate(light)
	}
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustBadgeFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, density, direction)
	facet.Attach(light, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 160)
	_ = light.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	light.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: light.layoutRole.Parent,
		ChildGroup:  light.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, canvas)
	cmds := light.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(272, 192)
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
	testkit.AssertGolden(t, surface, "status_light_"+name)
}

func newStatusLightFixture() *StatusLight {
	return NewStatusLight("Online")
}
