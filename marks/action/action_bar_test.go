package action

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

type actionBarRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
	icons     runtimepkg.IconResolver
}

func (s actionBarRuntimeStub) Schedule(j job.AnyJob)  {}
func (s actionBarRuntimeStub) CancelJob(id job.JobID) {}
func (s actionBarRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s actionBarRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s actionBarRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s actionBarRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }
func (s actionBarRuntimeStub) IconResolver() runtimepkg.IconResolver {
	return s.icons
}

type actionBarIconResolverStub map[string]runtimepkg.IconAsset

func (r actionBarIconResolverStub) ResolveIcon(ref string) (runtimepkg.IconAsset, bool) {
	asset, ok := r[ref]
	return asset, ok
}

func TestActionBarMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	bar, rt := newActionBarFixture(t)

	facet.Attach(bar, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := bar.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 160}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H)
	bar.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: bar.Layout.Parent, ChildGroup: bar.Layout.Child}, bounds)

	if got := bar.AccessibilityRole(); got != "toolbar" {
		t.Fatalf("accessibility role = %q, want toolbar", got)
	}
	if got := bar.AccessibleName(); got != "224 selected" {
		t.Fatalf("accessible name = %q, want 224 selected", got)
	}
	if len(bar.Children()) != 4 {
		t.Fatalf("expected 4 child facets, got %d", len(bar.Children()))
	}
	if bar.cachedLabelBounds.IsEmpty() || len(bar.cachedActionBounds) != 4 {
		t.Fatalf("expected arranged geometry, got label=%#v actions=%d", bar.cachedLabelBounds, len(bar.cachedActionBounds))
	}

	labelHit := bar.Hit.HitTest(gfx.Point{X: bar.cachedLabelBounds.Min.X + 1, Y: bar.cachedLabelBounds.Min.Y + 1})
	if !labelHit.Hit || labelHit.MarkID != actionBarMarkIDContextLabel {
		t.Fatalf("expected context-label hit, got %#v", labelHit)
	}
	actionHit := bar.Hit.HitTest(gfx.Point{X: bar.cachedActionBounds[0].Min.X + 1, Y: bar.cachedActionBounds[0].Min.Y + 1})
	if !actionHit.Hit || actionHit.MarkID != actionBarMarkIDActionItems {
		t.Fatalf("expected action-items hit, got %#v", actionHit)
	}
	overflowHit := bar.Hit.HitTest(gfx.Point{X: bar.cachedActionBounds[3].Min.X + 1, Y: bar.cachedActionBounds[3].Min.Y + 1})
	if !overflowHit.Hit || overflowHit.MarkID != actionBarMarkIDOverflowMenu {
		t.Fatalf("expected overflow hit, got %#v", overflowHit)
	}

	anchors := bar.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := bar.Projection.Project(facet.ProjectionContext{
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
		t.Fatal("expected glyph commands for label")
	}
	if !sawFillPath {
		t.Fatal("expected fill/stroke commands for bar or child actions")
	}
}

func TestActionBarPointerKeyboardAndDisabledBehavior(t *testing.T) {
	bar, rt := newActionBarFixture(t)

	facet.Attach(bar, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := bar.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 960, H: 160}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	bar.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext()}, bounds)

	activated := ""
	bar.Activated.Subscribe(func(key string) {
		activated = key
	})

	first := gfx.Point{X: bar.cachedActionBounds[0].Min.X + 1, Y: bar.cachedActionBounds[0].Min.Y + 1}
	if !bar.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !bar.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != "edit" {
		t.Fatalf("expected edit activation, got %q", activated)
	}

	bar.onFocusLost()
	bar.focusFromPointer = false
	bar.onFocusGained()
	if !bar.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}
	if !bar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := bar.focusedIndex; got != 1 {
		t.Fatalf("focused index = %d, want 1", got)
	}
	if !bar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected space press to be handled")
	}
	if !bar.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeySpace}) {
		t.Fatal("expected space release to be handled")
	}
	if activated != "copy" {
		t.Fatalf("expected copy activation from keyboard, got %q", activated)
	}

	bar.Disabled = marks.Const(true)
	if bar.Focus.Focusable() {
		t.Fatal("expected disabled action bar to be unfocusable")
	}
	if bar.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: first, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled action bar to ignore pointer input")
	}
	if bar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled action bar to ignore keyboard input")
	}
}

func TestActionBarRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveActionBarRecipe(ctx)
	if !allActionBarFieldsPresent(slots) {
		t.Fatalf("action bar slots contain zero values: %#v", slots)
	}
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 6 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func newActionBarFixture(t *testing.T) (*ActionBar, actionBarRuntimeStub) {
	t.Helper()
	bar := NewActionBar("224 selected", []ActionBarAction{
		{Key: "edit", Label: "Edit", IconRef: "edit"},
		{Key: "copy", Label: "Copy", IconRef: "copy"},
		{Key: "delete", Label: "Delete", IconRef: "delete"},
	})
	bar.Overflow = marks.Const(&ActionBarAction{Key: "more", AccessibleLabel: "More options", IconRef: "more"})
	rt := actionBarRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
		icons: actionBarIconResolverStub{
			"edit":   mustActionBarIconAsset("edit"),
			"copy":   mustActionBarIconAsset("copy"),
			"delete": mustActionBarIconAsset("delete"),
			"more":   mustActionBarIconAsset("more"),
		},
	}
	return bar, rt
}

func mustActionBarIconAsset(ref string) runtimepkg.IconAsset {
	return runtimepkg.NewIconAsset(ref, 1, gfx.RectPath(gfx.RectFromXYWH(0, 0, 24, 24)), gfx.RectFromXYWH(0, 0, 24, 24))
}

func allActionBarFieldsPresent[T any](value T) bool {
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
