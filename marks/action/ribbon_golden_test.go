package action

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestRibbonGoldenDefault(t *testing.T) {
	AssertRibbonGolden(t, "default", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {})
}

func TestRibbonGoldenCompact(t *testing.T) {
	AssertRibbonGolden(t, "compact", defaultActionBarTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(r *Ribbon) {})
}

func TestRibbonGoldenComfortable(t *testing.T) {
	AssertRibbonGolden(t, "comfortable", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {})
}

func TestRibbonGoldenDisabled(t *testing.T) {
	AssertRibbonGolden(t, "disabled", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {
		r.Disabled = marks.Const(true)
	})
}

func TestRibbonGoldenHighContrast(t *testing.T) {
	AssertRibbonGolden(t, "high_contrast", highContrastActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {})
}

func TestRibbonGoldenHovered(t *testing.T) {
	AssertRibbonGolden(t, "hovered", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {
		if len(r.cachedTabButtons) > 1 {
			r.cachedTabButtons[1].hovered = true
		}
	})
}

func TestRibbonGoldenPressed(t *testing.T) {
	AssertRibbonGolden(t, "pressed", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {
		if len(r.cachedTabButtons) > 0 {
			r.cachedTabButtons[0].pressed = true
		}
	})
}

func TestRibbonGoldenFocused(t *testing.T) {
	AssertRibbonGolden(t, "focused", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {
		r.onFocusGained()
	})
}

func TestRibbonGoldenRTL(t *testing.T) {
	AssertRibbonGolden(t, "rtl", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(r *Ribbon) {})
}

func TestRibbonGoldenSelected(t *testing.T) {
	AssertRibbonGolden(t, "selected", defaultActionBarTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(r *Ribbon) {
		r.ActiveIndex = 1
	})
}

func AssertRibbonGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Ribbon)) {
	t.Helper()
	ribbon, rt := newRibbonFixture(t, tokens)
	if mutate != nil {
		mutate(ribbon)
	}
	renderRibbonToSurface(t, ribbon, rt, theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(density, tokens)).WithWritingDirection(direction), density, direction, name)
}

func renderRibbonToSurface(t *testing.T, ribbon *Ribbon, rt buttonRuntimeStub, measureCtx theme.ResolvedContext, density theme.DensityID, direction layout.WritingDirection, goldenName string) {
	t.Helper()
	facet.Attach(ribbon, facet.AttachContext{Runtime: rt, Theme: measureCtx})
	result := ribbon.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            measureCtx,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: 1600, H: 560}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	surfaceW := 1920
	surfaceH := 640
	x := maxFloat(0, float32(surfaceW)-result.Size.W) * 0.5
	y := maxFloat(0, float32(surfaceH)-result.Size.H) * 0.5
	bounds := gfx.RectFromXYWH(x, y, result.Size.W, result.Size.H)
	ribbon.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       measureCtx,
		ParentGroup: ribbon.Layout.Parent,
		ChildGroup:  ribbon.Layout.Child,
	}, bounds)

	cmds := ribbon.Projection.Project(facet.ProjectionContext{
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
	testkit.AssertGolden(t, surface, "ribbon_"+goldenName)
}
