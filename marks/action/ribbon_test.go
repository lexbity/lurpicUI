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

func TestRibbonMeasureProjectHitAnchorsAndAccessibility(t *testing.T) {
	ribbon, rt := newRibbonFixture(t, defaultActionBarTokens())

	facet.Attach(ribbon, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := ribbon.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1600, H: 560}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}

	bounds := gfx.RectFromXYWH(12, 20, result.Size.W, result.Size.H)
	ribbon.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: ribbon.Layout.Parent, ChildGroup: ribbon.Layout.Child}, bounds)

	if got := ribbon.AccessibilityRole(); got != "toolbar" {
		t.Fatalf("accessibility role = %q, want toolbar", got)
	}
	if got := ribbon.AccessibleName(); got != "Document ribbon" {
		t.Fatalf("accessible name = %q, want Document ribbon", got)
	}
	if len(ribbon.cachedTabButtons) != len(ribbon.Sections) {
		t.Fatalf("tab buttons = %d, want %d", len(ribbon.cachedTabButtons), len(ribbon.Sections))
	}
	if ribbon.cachedRootBounds.IsEmpty() || ribbon.cachedTabBounds[0].IsEmpty() || len(ribbon.cachedToolbarBounds) == 0 {
		t.Fatalf("expected arranged geometry, got root=%#v tabs=%#v toolbars=%#v", ribbon.cachedRootBounds, ribbon.cachedTabBounds, ribbon.cachedToolbarBounds)
	}

	tabHit := ribbon.Hit.HitTest(gfx.Point{
		X: ribbon.cachedTabBounds[1].Min.X + ribbon.cachedTabBounds[1].Width()*0.5,
		Y: ribbon.cachedTabBounds[1].Min.Y + ribbon.cachedTabBounds[1].Height()*0.5,
	})
	if !tabHit.Hit || tabHit.MarkID != ribbonMarkIDGroupLabels {
		t.Fatalf("expected tab-label hit, got %#v", tabHit)
	}
	toolbarHit := ribbon.Hit.HitTest(gfx.Point{
		X: ribbon.cachedToolbarBounds[0].Min.X + ribbon.cachedToolbarBounds[0].Width()*0.5,
		Y: ribbon.cachedToolbarBounds[0].Min.Y + ribbon.cachedToolbarBounds[0].Height()*0.5,
	})
	if !toolbarHit.Hit || toolbarHit.MarkID != ribbonMarkIDActionItems {
		t.Fatalf("expected action-item hit, got %#v", toolbarHit)
	}

	anchors := ribbon.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}

	cmds := ribbon.Projection.Project(facet.ProjectionContext{
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

func TestRibbonPointerKeyboardAndDisabledBehavior(t *testing.T) {
	ribbon, rt := newRibbonFixture(t, defaultActionBarTokens())

	facet.Attach(ribbon, facet.AttachContext{Runtime: rt, Theme: theme.DefaultResolvedContext()})
	result := ribbon.Layout.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        theme.DefaultResolvedContext(),
		ContentScale: 1,
	}, facet.Constraints{MaxSize: gfx.Size{W: 1600, H: 560}})
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	ribbon.Layout.Arrange(facet.ArrangeContext{Runtime: rt, Theme: theme.DefaultResolvedContext(), ParentGroup: ribbon.Layout.Parent, ChildGroup: ribbon.Layout.Child}, bounds)

	activated := -1
	ribbon.Activated.Subscribe(func(index int) {
		activated = index
	})

	secondCenter := gfx.Point{
		X: ribbon.cachedTabBounds[1].Min.X + ribbon.cachedTabBounds[1].Width()*0.5,
		Y: ribbon.cachedTabBounds[1].Min.Y + ribbon.cachedTabBounds[1].Height()*0.5,
	}
	if !ribbon.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer press to be handled")
	}
	if !ribbon.onPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected pointer release to be handled")
	}
	if got := ribbon.ActiveIndex; got != 1 {
		t.Fatalf("active index after pointer = %d, want 1", got)
	}
	if activated != 1 {
		t.Fatalf("expected activated signal for index 1, got %d", activated)
	}

	if !ribbon.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyRight}) {
		t.Fatal("expected right key to be handled")
	}
	if got := ribbon.ActiveIndex; got != 2 {
		t.Fatalf("active index after right = %d, want 2", got)
	}
	if !ribbon.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyHome}) {
		t.Fatal("expected home key to be handled")
	}
	if got := ribbon.ActiveIndex; got != 0 {
		t.Fatalf("active index after home = %d, want 0", got)
	}
	if !ribbon.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeyEnd}) {
		t.Fatal("expected end key to be handled")
	}
	if got := ribbon.ActiveIndex; got != len(ribbon.Sections)-1 {
		t.Fatalf("active index after end = %d, want %d", got, len(ribbon.Sections)-1)
	}

	ribbon.onFocusLost()
	ribbon.focusFromPointer = false
	ribbon.onFocusGained()
	if !ribbon.focusedVisible {
		t.Fatal("expected keyboard focus to show focus ring")
	}

	ribbon.Disabled = marks.Const(true)
	if ribbon.Focus.Focusable() {
		t.Fatal("expected disabled ribbon to be unfocusable")
	}
	if ribbon.onPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: secondCenter, Button: platform.PointerLeft}) {
		t.Fatal("expected disabled ribbon to ignore pointer input")
	}
	if ribbon.onKey(facet.KeyEvent{Kind: platform.KeyPress, Key: platform.KeySpace}) {
		t.Fatal("expected disabled ribbon to ignore keyboard input")
	}
}

func TestRibbonRecipe_allSlotsPresent(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uiaction.ResolveRibbonRecipe(ctx)
	if !allRibbonFieldsPresent(slots) {
		t.Fatalf("ribbon slots contain zero values: %#v", slots)
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

func newRibbonFixture(t *testing.T, tokens theme.Tokens) (*Ribbon, buttonRuntimeStub) {
	t.Helper()
	ribbon := NewRibbon("Document ribbon", []RibbonSection{
		{
			Key:   "home",
			Label: "Home",
			Toolbars: []*Toolbar{
				NewToolbar(marks.Const("Clipboard"), []ToolbarGroup{
					{
						Key: "primary",
						Actions: []ActionGroupAction{
							{Key: "paste", Label: "Paste", IconRef: "paste"},
							{Key: "cut", Label: "Cut", IconRef: "cut"},
						},
					},
				}, &ToolbarOverflow{
					AccessibleLabel: "More clipboard options",
					TriggerIconRef:  "more",
					Entries: []MenuButtonEntry{
						{Key: "copy", Label: "Copy", IconRef: "copy"},
						{Key: "format", Label: "Format", IconRef: "edit"},
					},
				}),
				NewToolbar(marks.Const("Editing"), []ToolbarGroup{
					{
						Key: "secondary",
						Actions: []ActionGroupAction{
							{Key: "find", Label: "Find", IconRef: "search"},
							{Key: "replace", Label: "Replace", IconRef: "edit"},
						},
					},
				}, nil),
			},
		},
		{
			Key:   "insert",
			Label: "Insert",
			Toolbars: []*Toolbar{
				NewToolbar(marks.Const("Illustrations"), []ToolbarGroup{
					{
						Key: "art",
						Actions: []ActionGroupAction{
							{Key: "picture", Label: "Picture", IconRef: "image"},
							{Key: "shape", Label: "Shape", IconRef: "shape"},
						},
					},
				}, nil),
			},
		},
		{
			Key:   "view",
			Label: "View",
			Toolbars: []*Toolbar{
				NewToolbar(marks.Const("Layout"), []ToolbarGroup{
					{
						Key: "view",
						Actions: []ActionGroupAction{
							{Key: "zoom", Label: "Zoom", IconRef: "zoom"},
							{Key: "grid", Label: "Grid", IconRef: "grid"},
						},
					},
				}, nil),
			},
		},
	})
	rt := buttonRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
		icons: buttonIconResolverStub{
			"paste":  mustActionGroupIconAsset("edit"),
			"cut":    mustActionGroupIconAsset("delete"),
			"copy":   mustActionGroupIconAsset("copy"),
			"edit":   mustActionGroupIconAsset("edit"),
			"more":   mustActionGroupIconAsset("more"),
			"search": mustActionGroupIconAsset("search"),
			"image":  mustActionGroupIconAsset("image"),
			"shape":  mustActionGroupIconAsset("shape"),
			"zoom":   mustActionGroupIconAsset("zoom"),
			"grid":   mustActionGroupIconAsset("grid"),
		},
	}
	return ribbon, rt
}

func allRibbonFieldsPresent[T any](value T) bool {
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
