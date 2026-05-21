package status

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestProgressBarMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	bar := newProgressBarFixture()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, badgeTokens(), nil),
		fonts:     mustBadgeFontRegistry(t),
	}
	ctx := badgeResolvedContext(badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(bar, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := bar.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 280, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	bar.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: bar.layoutRole.Parent,
		ChildGroup:  bar.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, bounds)
	if got := bar.AccessibilityRole(); got != "progressbar" {
		t.Fatalf("accessibility role = %q, want progressbar", got)
	}
	if got := bar.AccessibleName(); got != "" {
		t.Fatalf("accessible name = %q, want empty", got)
	}
	if bar.cachedTrackBounds.IsEmpty() || bar.cachedIndicatorBounds.IsEmpty() || bar.cachedLabelBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got track=%#v indicator=%#v label=%#v", bar.cachedTrackBounds, bar.cachedIndicatorBounds, bar.cachedLabelBounds)
	}
	anchors := bar.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "track", "indicator", "optional_label"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := bar.projectionRole.Project(facet.ProjectionContext{
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

func TestProgressBarTickRequestsAnimation(t *testing.T) {
	bar := newProgressBarFixture()
	bar.SetValue(0.85)
	if !bar.tickRole.IsActive() {
		t.Fatal("expected tick role to request animation after value update")
	}
	before := bar.pulseRemaining
	bar.tickRole.OnTick(16 * time.Millisecond)
	if bar.pulseRemaining >= before {
		t.Fatalf("expected pulse remaining to decrease, before=%v after=%v", before, bar.pulseRemaining)
	}
}

func TestProgressBarGoldenDefault(t *testing.T) {
	AssertProgressBarGolden(t, "default", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *ProgressBar) {})
}

func TestProgressBarGoldenCompact(t *testing.T) {
	AssertProgressBarGolden(t, "compact", badgeTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(b *ProgressBar) {})
}

func TestProgressBarGoldenComfortable(t *testing.T) {
	AssertProgressBarGolden(t, "comfortable", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *ProgressBar) {})
}

func TestProgressBarGoldenDisabled(t *testing.T) {
	AssertProgressBarGolden(t, "disabled", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *ProgressBar) {
		b.SetDisabled(true)
	})
}

func TestProgressBarGoldenHighContrast(t *testing.T) {
	AssertProgressBarGolden(t, "high_contrast", highContrastBadgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(b *ProgressBar) {})
}

func TestProgressBarGoldenRTL(t *testing.T) {
	AssertProgressBarGolden(t, "rtl", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(b *ProgressBar) {})
}

func AssertProgressBarGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*ProgressBar)) {
	t.Helper()
	bar := newProgressBarFixture()
	if mutate != nil {
		mutate(bar)
	}
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     mustBadgeFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, density, direction)
	facet.Attach(bar, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 280, 160)
	_ = bar.layoutRole.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	bar.layoutRole.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: bar.layoutRole.Parent,
		ChildGroup:  bar.layoutRole.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, canvas)
	cmds := bar.projectionRole.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(312, 192)
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
	testkit.AssertGolden(t, surface, "progress_bar_"+name)
}

func newProgressBarFixture() *ProgressBar {
	bar := NewProgressBar("Uploading files")
	bar.Value = 0.63
	return bar
}
