package action

import (
	"math"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

type buttonRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
	icons     runtimepkg.IconResolver
}

func (s buttonRuntimeStub) Schedule(j job.AnyJob)  {}
func (s buttonRuntimeStub) CancelJob(id job.JobID) {}
func (s buttonRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s buttonRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s buttonRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s buttonRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }
func (s buttonRuntimeStub) IconResolver() runtimepkg.IconResolver {
	return s.icons
}

type buttonIconResolverStub map[string]runtimepkg.IconAsset

func (r buttonIconResolverStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := r[ref]
	return asset, ok
}

func TestButtonMeasureProjectHitAndAnchors(t *testing.T) {
	btn, rt := newTestButton(t, true)
	if got := btn.AccessibilityRole(); got != "button" {
		t.Fatalf("accessibility role = %q, want button", got)
	}
	if got := btn.AccessibleName(); got != "Save changes" {
		t.Fatalf("accessible name = %q, want Save changes", got)
	}

	facet.Attach(btn, facet.AttachContext{
		Runtime: rt,
		Theme:   theme.DefaultResolvedContext(),
	})

	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 800, H: 200}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 24, result.Size.W, result.Size.H)
	btn.Layout.Arrange(facet.ArrangeContext{Theme: theme.DefaultResolvedContext()}, bounds)

	if btn.cachedLeadingBox.IsEmpty() || btn.cachedTrailingBox.IsEmpty() {
		t.Fatalf("expected icon hit boxes, got leading=%#v trailing=%#v", btn.cachedLeadingBox, btn.cachedTrailingBox)
	}

	cmds := btn.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var capture testkit.CommandCapture
	capture.Capture(cmds)
	capture.AssertHasGlyphRun(t)
	capture.AssertHasFillPath(t)
	capture.AssertGlyphRunText(t, "Save")

	anchors := btn.ExportAnchors(layoutAnchorContext(bounds))
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	leading := btn.Hit.HitTest(gfx.Point{X: btn.cachedLeadingBox.Min.X + 1, Y: btn.cachedLeadingBox.Min.Y + 1})
	if !leading.Hit || leading.MarkID != buttonMarkIDLeading {
		t.Fatalf("expected leading hit, got %#v", leading)
	}
	label := btn.Hit.HitTest(gfx.Point{X: btn.cachedLabelBounds.Min.X + 1, Y: btn.cachedLabelBounds.Min.Y + 1})
	if !label.Hit || label.MarkID != buttonMarkIDLabel {
		t.Fatalf("expected label hit, got %#v", label)
	}
	trailing := btn.Hit.HitTest(gfx.Point{X: btn.cachedTrailingBox.Min.X + 1, Y: btn.cachedTrailingBox.Min.Y + 1})
	if !trailing.Hit || trailing.MarkID != buttonMarkIDTrailing {
		t.Fatalf("expected trailing hit, got %#v", trailing)
	}
	container := btn.Hit.HitTest(gfx.Point{X: bounds.Min.X + 1, Y: bounds.Min.Y + 1})
	if !container.Hit || container.MarkID != buttonMarkIDContainer {
		t.Fatalf("expected container hit, got %#v", container)
	}
}

func TestButtonAndSplitButtonLabelBaselineReference(t *testing.T) {
	btn, btnRT := newTestButton(t, false)
	btn.Label = marks.Const("Label")

	split, splitRT := newSplitButtonFixture(t)
	split.Label = marks.Const("Label")
	split.PrimaryIconRef = marks.Const("")

	ctx := theme.DefaultResolvedContext()

	facet.Attach(btn, facet.AttachContext{Runtime: btnRT, Theme: ctx})
	facet.Attach(split, facet.AttachContext{Runtime: splitRT, Theme: ctx})

	btnResult := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      btnRT,
		Theme:        ctx,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 800, H: 240}})
	splitResult := split.Layout.Measure(facet.MeasureContext{
		Runtime:      splitRT,
		Theme:        ctx,
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 800, H: 240}})
	if btnResult.Size.W <= 0 || btnResult.Size.H <= 0 || splitResult.Size.W <= 0 || splitResult.Size.H <= 0 {
		t.Fatalf("expected measurable layouts, button=%#v split=%#v", btnResult.Size, splitResult.Size)
	}

	btn.Layout.Arrange(facet.ArrangeContext{Theme: ctx}, gfx.RectFromXYWH(0, 0, btnResult.Size.W, btnResult.Size.H))
	split.Layout.Arrange(facet.ArrangeContext{Runtime: splitRT, Theme: ctx, ParentGroup: split.Layout.Parent, ChildGroup: split.Layout.Child}, gfx.RectFromXYWH(0, 0, splitResult.Size.W, splitResult.Size.H))

	if btn.cachedLayout == nil || split.cachedPrimaryLayout == nil {
		t.Fatalf("expected shaped label layouts, button=%#v split=%#v", btn.cachedLayout, split.cachedPrimaryLayout)
	}
	if diff := math.Abs(float64(btn.cachedLayout.Baseline - split.cachedPrimaryLayout.Baseline)); diff > 0.01 {
		t.Fatalf("label baseline mismatch: button=%v split=%v (diff %v)", btn.cachedLayout.Baseline, split.cachedPrimaryLayout.Baseline, diff)
	}
	if diff := math.Abs(float64(btn.cachedLayout.LineHeight - split.cachedPrimaryLayout.LineHeight)); diff > 0.01 {
		t.Fatalf("label line height mismatch: button=%v split=%v (diff %v)", btn.cachedLayout.LineHeight, split.cachedPrimaryLayout.LineHeight, diff)
	}
}

func TestButtonActivatesFromPointerAndKeyboard(t *testing.T) {
	btn, rt := newTestButton(t, false)

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	btn.Layout.Arrange(facet.ArrangeContext{Theme: theme.DefaultResolvedContext()}, bounds)

	activated := 0
	btn.Activated.Subscribe(func(signal.Unit) {
		activated++
	})

	center := gfx.Point{X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: center, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !btn.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: center, Button: platform.PointerLeft}) {
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
		t.Fatalf("expected space to activate once, got %d", activated)
	}

	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key press to be handled")
	}
	if !btn.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter key release to be handled")
	}
	if activated != 3 {
		t.Fatalf("expected enter to activate once, got %d", activated)
	}
}

func TestButtonFocusVisibleAndDisabledBehavior(t *testing.T) {
	btn, rt := newTestButton(t, false)

	facet.Attach(btn, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := btn.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 400, H: 200}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	btn.Layout.Arrange(facet.ArrangeContext{Theme: theme.DefaultResolvedContext()}, bounds)

	btn.onFocusGained()
	if !btn.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}
	cmds := btn.Projection.Project(facet.ProjectionContext{
		Runtime:      rt,
		Bounds:       bounds,
		ContentScale: 1,
	})
	if cmds == nil || cmds.Len() == 0 {
		t.Fatal("expected projected commands")
	}
	var sawStroke bool
	for _, cmd := range cmds.Commands {
		if _, ok := cmd.(gfx.StrokePath); ok {
			sawStroke = true
			break
		}
	}
	if !sawStroke {
		t.Fatal("expected focus ring stroke in projection")
	}

	btn.Disabled = marks.Const(true)
	if btn.Focus.Focusable() {
		t.Fatal("expected disabled button to be unfocusable")
	}
	if btn.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: 1, Y: 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled button to ignore pointer input")
	}
	if btn.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled button to ignore keyboard input")
	}
}

func newTestButton(t *testing.T, withIcons bool) (*Button, buttonRuntimeStub) {
	t.Helper()
	btn := NewButton(marks.Const("Save changes"), marks.Const(uiinput.ButtonFilled))
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	if withIcons {
		rt.icons = buttonIconResolverStub{
			"leading":  {Path: gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)), ViewBox: gfx.RectFromXYWH(0, 0, 24, 24)},
			"trailing": {Path: gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)), ViewBox: gfx.RectFromXYWH(0, 0, 24, 24)},
		}
		btn.LeadingIconRef = marks.Const("leading")
		btn.TrailingIconRef = marks.Const("trailing")
	}
	return btn, rt
}

func layoutAnchorContext(bounds gfx.Rect) layout.AnchorExportContext {
	return layout.AnchorExportContext{
		ResolvedLayer: layout.ResolvedLayer{Bounds: bounds},
	}
}
