package status

import (
	"testing"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestProgressRingMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	ring := newProgressRingFixture()
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, badgeTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR)

	facet.Attach(ring, facet.AttachContext{Runtime: rt, Theme: ctx})
	result := ring.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 240, H: 220}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(16, 16, result.Size.W, result.Size.H)
	ring.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: ring.Layout.Parent,
		ChildGroup:  ring.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, bounds)
	if got := ring.AccessibilityRole(); got != "progressbar" {
		t.Fatalf("accessibility role = %q, want progressbar", got)
	}
	if got := ring.AccessibleName(); got != "" {
		t.Fatalf("accessible name = %q, want empty", got)
	}
	if ring.cachedTrackBounds.IsEmpty() || ring.cachedIndicatorBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got track=%#v indicator=%#v", ring.cachedTrackBounds, ring.cachedIndicatorBounds)
	}
	anchors := ring.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := ring.ProjectionRole().Project(facet.ProjectionContext{
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

func TestProgressRingTickRequestsAnimation(t *testing.T) {
	ring := newProgressRingFixture()
	ring.Value = marks.Const(float32(0.82))
	ring.startPulse()
	if !ring.Tick.IsActive() {
		t.Fatal("expected tick role to request animation after value update")
	}
	before := ring.pulseRemaining
	ring.Tick.OnTick(16 * time.Millisecond)
	if ring.pulseRemaining >= before {
		t.Fatalf("expected pulse remaining to decrease, before=%v after=%v", before, ring.pulseRemaining)
	}
}

func TestProgressRingGoldenDefault(t *testing.T) {
	AssertProgressRingGolden(t, "default", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *ProgressRing) {})
}

func TestProgressRingGoldenCompact(t *testing.T) {
	AssertProgressRingGolden(t, "compact", badgeTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(r *ProgressRing) {})
}

func TestProgressRingGoldenDisabled(t *testing.T) {
	AssertProgressRingGolden(t, "disabled", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *ProgressRing) {
		r.Disabled = marks.Const(true)
	})
}

func TestProgressRingGoldenHighContrast(t *testing.T) {
	AssertProgressRingGolden(t, "high_contrast", highContrastBadgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *ProgressRing) {})
}

func TestProgressRingGoldenRTL(t *testing.T) {
	AssertProgressRingGolden(t, "rtl", badgeTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(r *ProgressRing) {})
}

func AssertProgressRingGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*ProgressRing)) {
	t.Helper()
	ring := newProgressRingFixture()
	if mutate != nil {
		mutate(ring)
	}
	rt := badgeRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	ctx := badgeResolvedContext(tokens, density, direction)
	facet.Attach(ring, facet.AttachContext{Runtime: rt, Theme: ctx})
	canvas := gfx.RectFromXYWH(16, 16, 240, 220)
	_ = ring.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            ctx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	ring.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       ctx,
		ParentGroup: ring.Layout.Parent,
		ChildGroup:  ring.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementGrid},
	}, canvas)
	cmds := ring.ProjectionRole().Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		t.Fatal("expected projected commands")
	}
	surface := testkit.NewMemorySurface(272, 252)
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
	testkit.AssertGolden(t, surface, "progress_ring_"+name)
}

func newProgressRingFixture() *ProgressRing {
	ring := NewProgressRing("Syncing data")
	ring.Value = marks.Const(float32(0.75))
	return ring
}
