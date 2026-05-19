package primitive

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/theme"
)

type iconRuntimeStub struct {
	rootStyle any
	icons     map[string]runtimepkg.IconAsset
}

func (s iconRuntimeStub) Schedule(j job.AnyJob)  {}
func (s iconRuntimeStub) CancelJob(id job.JobID) {}
func (s iconRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s iconRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s iconRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s iconRuntimeStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := s.icons[ref]
	return asset, ok
}

func TestIconMark_defaultsSizeAndMissingSourceFallback(t *testing.T) {
	mark := NewIcon(nil)
	if !mark.Decorative {
		t.Fatal("expected icons to default to decorative")
	}
	if mark.AccessibilityRole() != "presentation" {
		t.Fatalf("accessibility role = %q, want presentation", mark.AccessibilityRole())
	}
	if mark.AccessibleName() != "" {
		t.Fatalf("accessible name = %q, want empty for decorative icon", mark.AccessibleName())
	}
	if mark.ColorSlot != theme.ColorText {
		t.Fatalf("color slot = %v, want ColorText", mark.ColorSlot)
	}
	if mark.DensityBehavior != IconDensityScaleWithDensity {
		t.Fatalf("density behavior = %v, want scale-with-density", mark.DensityBehavior)
	}

	rt := iconRuntimeStub{rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil)}
	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}})
	if !almostEqual(result.Size.W, 20) || !almostEqual(result.Size.H, 20) {
		t.Fatalf("expected default icon size 20, got %#v", result.Size)
	}
	mark.layoutRole.Arrange(facet.ArrangeContext{}, gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H))
	if cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H),
		ContentScale: 1,
	}); cmds != nil {
		t.Fatalf("expected missing source to yield no commands, got %#v", cmds.Commands)
	}
}

func TestIconMark_resolverProjectionColorAnchorsAndHit(t *testing.T) {
	tokens := theme.DefaultTokens()
	tokens.Color.Primary = gfx.ColorFromRGBA8(24, 48, 72, 255)
	rt := iconRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		icons: map[string]runtimepkg.IconAsset{
			"chevron-down": runtimepkg.NewIconAsset(
				"chevron-down",
				17,
				gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)),
				gfx.RectFromXYWH(0, 0, 24, 24),
			),
		},
	}
	mark := NewIcon(IconRef("chevron-down"))
	mark.SetAccessibleName("Chevron down")
	mark.SetColorSlot(theme.ColorPrimary)

	facet.Attach(mark, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := mark.layoutRole.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}})
	bounds := gfx.RectFromXYWH(10, 20, result.Size.W, result.Size.H)
	mark.layoutRole.Arrange(facet.ArrangeContext{}, bounds)

	if got := mark.AccessibilityRole(); got != "img" {
		t.Fatalf("accessibility role = %q, want img", got)
	}
	if got := mark.AccessibleName(); got != "Chevron down" {
		t.Fatalf("accessible name = %q, want Chevron down", got)
	}

	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || len(cmds.Commands) != 3 {
		t.Fatalf("expected transform + fill + pop commands, got %#v", cmds)
	}
	if _, ok := cmds.Commands[1].(gfx.FillPath); !ok {
		t.Fatalf("expected fill path command, got %T", cmds.Commands[1])
	}
	fill := cmds.Commands[1].(gfx.FillPath)
	if r, g, b, a := fill.Brush.Color.ToRGBA8(); r != 24 || g != 48 || b != 72 || a != 255 {
		t.Fatalf("unexpected fill color: %d %d %d %d", r, g, b, a)
	}

	anchors := mark.ExportAnchors(iconLayoutAnchorContext(bounds))
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	hit := mark.hitRole.HitTest(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1})
	if !hit.Hit {
		t.Fatal("expected semantic icon to hit-test inside its bounds")
	}

	updatedTokens := tokens
	updatedTokens.Color.Primary = gfx.ColorFromRGBA8(200, 10, 20, 255)
	rt2 := iconRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, updatedTokens, nil),
		icons:     rt.icons,
	}
	cmds2 := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt2,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds2 == nil || len(cmds2.Commands) != 3 {
		t.Fatalf("expected second projection to produce commands, got %#v", cmds2)
	}
	fill2 := cmds2.Commands[1].(gfx.FillPath)
	if r, g, b, a := fill2.Brush.Color.ToRGBA8(); r == 24 && g == 48 && b == 72 && a == 255 {
		t.Fatal("expected theme color change to update projected icon color")
	}
}

func TestIconMark_densityBehaviorsAndInlineSVG(t *testing.T) {
	inline := IconSVG(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 10 10" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round"><path d="M1 3l4 4 4-4"/></svg>`)
	compact := theme.DefaultResolvedContext().WithDensity(theme.DefaultDensityScale(theme.DensityIDCompact, theme.DefaultTokens()))
	baseRuntime := iconRuntimeStub{rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil)}

	scaled := NewIcon(inline)
	scaled.SetSize(16)
	facet.Attach(scaled, facet.AttachContext{Runtime: baseRuntime, Theme: compact})
	gotScaled := scaled.layoutRole.Measure(facet.MeasureContext{
		Runtime:      baseRuntime,
		Theme:        compact,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size
	if !almostEqual(gotScaled.W, 14.88) || !almostEqual(gotScaled.H, 14.88) {
		t.Fatalf("scale-with-density size = %#v, want 14.88", gotScaled)
	}

	locked := NewIcon(inline)
	locked.SetSize(16)
	locked.SetDensityBehavior(IconDensityLockLogicalSize)
	facet.Attach(locked, facet.AttachContext{Runtime: baseRuntime, Theme: compact})
	gotLocked := locked.layoutRole.Measure(facet.MeasureContext{
		Runtime:      baseRuntime,
		Theme:        compact,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size
	if !almostEqual(gotLocked.W, 16) || !almostEqual(gotLocked.H, 16) {
		t.Fatalf("lock-logical-size = %#v, want 16", gotLocked)
	}

	snapped := NewIcon(inline)
	snapped.SetSize(16)
	snapped.SetDensityBehavior(IconDensitySnapToDevicePixels)
	facet.Attach(snapped, facet.AttachContext{Runtime: baseRuntime, Theme: compact})
	gotSnapped := snapped.layoutRole.Measure(facet.MeasureContext{
		Runtime:      baseRuntime,
		Theme:        compact,
		ContentScale: 2,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size
	if !almostEqual(gotSnapped.W, 15) || !almostEqual(gotSnapped.H, 15) {
		t.Fatalf("snap-to-device size = %#v, want 15", gotSnapped)
	}

	touch := NewIcon(inline)
	touch.SetSize(16)
	touch.SetDensityBehavior(IconDensityTouchAware)
	touch.SetAccessibleName("Chevron")
	touch.SetHitPadding(8)
	facet.Attach(touch, facet.AttachContext{Runtime: baseRuntime, Theme: compact})
	touchSize := touch.layoutRole.Measure(facet.MeasureContext{
		Runtime:      baseRuntime,
		Theme:        compact,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 200, H: 200}}).Size
	touchBounds := gfx.RectFromXYWH(0, 0, touchSize.W, touchSize.H)
	touch.layoutRole.Arrange(facet.ArrangeContext{}, touchBounds)
	gotHit := touch.hitRole.HitTest(gfx.Point{X: touchBounds.Max.X + 2, Y: touchBounds.Min.Y + 2})
	if !gotHit.Hit {
		t.Fatal("expected hit padding to expand icon hit bounds")
	}

	cmds := inlineProjectCommands(t, scaled, baseRuntime, gotScaled)
	if len(cmds) == 0 {
		t.Fatal("expected inline SVG to project draw commands")
	}
	if _, ok := cmds[1].(gfx.StrokePath); !ok {
		t.Fatalf("expected stroke projection for inline SVG, got %T", cmds[1])
	}
}

func inlineProjectCommands(t *testing.T, mark *Icon, rt iconRuntimeStub, size gfx.Size) []gfx.Command {
	t.Helper()
	bounds := gfx.RectFromXYWH(0, 0, size.W, size.H)
	mark.layoutRole.Arrange(facet.ArrangeContext{}, bounds)
	cmds := mark.projectionRole.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil {
		return nil
	}
	return cmds.Commands
}

func almostEqual(a, b float32) bool {
	const eps = 0.01
	if a < b {
		return b-a <= eps
	}
	return a-b <= eps
}

func iconLayoutAnchorContext(bounds gfx.Rect) layout.AnchorExportContext {
	return layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: bounds},
	}
}
