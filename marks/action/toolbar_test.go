package action

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

func TestToolbarMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	toolbar, rt := newToolbarFixture(t)

	facet.Attach(toolbar, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := toolbar.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 320}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H)
	toolbar.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: toolbar.Layout.Parent, ChildGroup: toolbar.Layout.Child}, bounds)

	if got := toolbar.AccessibilityRole(); got != "toolbar" {
		t.Fatalf("accessibility role = %q, want toolbar", got)
	}
	if got := toolbar.AccessibleName(); got != "Actions" {
		t.Fatalf("accessible name = %q, want Actions", got)
	}
	if len(toolbar.Children()) != 3 {
		t.Fatalf("expected 3 child facets, got %d", len(toolbar.Children()))
	}
	if len(toolbar.cachedChildBounds) != 3 || toolbar.cachedChildBounds[0].IsEmpty() || toolbar.cachedChildBounds[1].IsEmpty() || toolbar.cachedChildBounds[2].IsEmpty() {
		t.Fatalf("expected arranged child bounds, got %#v", toolbar.cachedChildBounds)
	}

	itemBounds := toolbar.cachedChildren[0].group.cachedActionBounds[0]
	itemHit := toolbar.Hit.HitTest(gfx.Point{X: itemBounds.Min.X + 1, Y: itemBounds.Min.Y + 1})
	if !itemHit.Hit || itemHit.MarkID != toolbarMarkIDActionItems {
		t.Fatalf("expected action-item hit, got %#v", itemHit)
	}
	sepPoint := gfx.Point{X: toolbar.cachedSeparatorBounds[0].Min.X + 1, Y: toolbar.cachedSeparatorBounds[0].Min.Y + 1}
	sepHit := toolbar.Hit.HitTest(sepPoint)
	if !sepHit.Hit || sepHit.MarkID != toolbarMarkIDSeparators {
		t.Fatalf("expected separator hit, got %#v", sepHit)
	}
	overflowHit := toolbar.Hit.HitTest(gfx.Point{X: toolbar.cachedChildren[2].overflow.cachedTriggerBounds.Min.X + 1, Y: toolbar.cachedChildren[2].overflow.cachedTriggerBounds.Min.Y + 1})
	if !overflowHit.Hit || overflowHit.MarkID != toolbarMarkIDOverflowMenu {
		t.Fatalf("expected overflow hit, got %#v", overflowHit)
	}

	anchors := toolbar.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := toolbar.Projection.Project(facet.ProjectionContext{
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

func TestToolbarPointerKeyboardAndDisabledBehavior(t *testing.T) {
	toolbar, rt := newToolbarFixture(t)

	facet.Attach(toolbar, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := toolbar.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1280, H: 320}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	toolbar.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: toolbar.Layout.Parent, ChildGroup: toolbar.Layout.Child}, bounds)

	var activated string
	toolbar.Activated.Subscribe(func(key string) {
		activated = key
	})

	firstActionBounds := toolbar.cachedChildren[0].group.cachedActionBounds[0]
	if !toolbar.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: firstActionBounds.Min.X + 1, Y: firstActionBounds.Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !toolbar.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: gfx.Point{X: firstActionBounds.Min.X + 1, Y: firstActionBounds.Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if activated != "close" {
		t.Fatalf("expected close activation, got %q", activated)
	}

	toolbar.onFocusLost()
	toolbar.focusFromPointer = false
	toolbar.onFocusGained()
	if !toolbar.focusedVisible {
		t.Fatal("expected focus-visible state after focus gain")
	}
	if !toolbar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := toolbar.focusedIndex; got != 0 {
		t.Fatalf("focused child index = %d, want 0", got)
	}
	if got := toolbar.cachedChildren[0].group.focusedIndex; got != 1 {
		t.Fatalf("focused action index = %d, want 1", got)
	}
	if !toolbar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnter}) {
		t.Fatal("expected enter press to be handled")
	}
	if !toolbar.onKey(facet.KeyEvent{Kind: platform.KeyRelease, Key: platform.KeyEnter}) {
		t.Fatal("expected enter release to be handled")
	}
	if activated != "edit" {
		t.Fatalf("expected edit activation from keyboard, got %q", activated)
	}

	toolbar.Disabled = marks.Const(true)
	if toolbar.Focus.Focusable() {
		t.Fatal("expected disabled toolbar to be unfocusable")
	}
	if toolbar.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: gfx.Point{X: firstActionBounds.Min.X + 1, Y: firstActionBounds.Min.Y + 1}, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled toolbar to ignore pointer input")
	}
	if toolbar.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled toolbar to ignore keyboard input")
	}
}

func TestToolbarRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveToolbarRecipe(ctx)
	if !allToolbarFieldsPresent(slots) {
		t.Fatalf("toolbar slots contain zero values: %#v", slots)
	}
	if report.Family != "uiaction" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 7 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func newToolbarFixture(t *testing.T) (*Toolbar, buttonRuntimeStub) {
	t.Helper()
	toolbar := NewToolbar(marks.Const("Actions"), []ToolbarGroup{
		{
			Key: "primary",
			Actions: []ActionGroupAction{
				{Key: "close", AccessibleLabel: "Close", IconRef: "close"},
				{Key: "edit", Label: "Edit", IconRef: "edit", Active: true},
			},
		},
		{
			Key: "secondary",
			Actions: []ActionGroupAction{
				{Key: "copy", Label: "Copy", IconRef: "copy"},
				{Key: "delete", Label: "Delete", IconRef: "delete"},
			},
		},
	}, &ToolbarOverflow{
		AccessibleLabel: "More options",
		TriggerIconRef:  "more",
		Entries: []MenuButtonEntry{
			{Key: "rename", Label: "Rename", IconRef: "edit"},
			{Key: "duplicate", Label: "Duplicate", IconRef: "copy"},
		},
	})
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, theme.DefaultTokens(), nil),
		fonts:     testkit.TestFontRegistry(t),
		icons: buttonIconResolverStub{
			"close":    mustActionGroupIconAsset("edit"),
			"edit":     mustActionGroupIconAsset("edit"),
			"copy":     mustActionGroupIconAsset("copy"),
			"delete":   mustActionGroupIconAsset("delete"),
			"more":     mustActionGroupIconAsset("more"),
			"rename":   mustActionGroupIconAsset("edit"),
			"duplicate": mustActionGroupIconAsset("copy"),
		},
	}
	return toolbar, rt
}

func allToolbarFieldsPresent[T any](value T) bool {
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
