package feedback

import (
	"reflect"
	"testing"
	"unsafe"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/testkit"
	"codeburg.org/lexbit/lurpicui/job"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/render"
	softwarerenderer "codeburg.org/lexbit/lurpicui/render/software"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uinotification"
	"codeburg.org/lexbit/lurpicui/theme/templates"
)

type notificationRuntimeStub struct {
	rootStyle any
	fonts     *text.FontRegistry
}

func (s notificationRuntimeStub) Schedule(j job.AnyJob)  {}
func (s notificationRuntimeStub) CancelJob(id job.JobID) {}
func (s notificationRuntimeStub) Invalidate(id facet.FacetID, flags facet.DirtyFlags, source string) {
}
func (s notificationRuntimeStub) RootStyleContext() any { return s.rootStyle }
func (s notificationRuntimeStub) FacetByID(id facet.FacetID) facet.FacetImpl {
	return nil
}
func (s notificationRuntimeStub) FontRegistry() *text.FontRegistry { return s.fonts }

func TestNotificationMeasureProjectAnchorsAndAccessibility(t *testing.T) {
	notification := newNotificationFixture()
	tokens := notificationTokens()
	resolved := notificationResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := notificationRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(notification, facet.AttachContext{Runtime: rt, Theme: resolved})
	result := 	notification.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 440, H: 180}})
	if result.Size.W <= 0 || result.Size.H <= 0 {
		t.Fatalf("expected measurable size, got %#v", result.Size)
	}
	bounds := gfx.RectFromXYWH(0, 0, result.Size.W, result.Size.H)
	notification.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: notification.Layout.Parent,
		ChildGroup:  notification.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, bounds)
	if got := notification.AccessibilityRole(); got != "status" {
		t.Fatalf("accessibility role = %q, want status", got)
	}
	if got := notification.AccessibleName(); got != "Saved Draft was synced successfully." {
		t.Fatalf("accessible name = %q", got)
	}
	if len(notification.Children()) != 4 {
		t.Fatalf("expected four child facets, got %d", len(notification.Children()))
	}
	if notification.cachedSurfaceBounds.IsEmpty() || notification.cachedTitleBounds.IsEmpty() || notification.cachedMessageBounds.IsEmpty() {
		t.Fatalf("expected arranged geometry, got surface=%#v title=%#v message=%#v", notification.cachedSurfaceBounds, notification.cachedTitleBounds, notification.cachedMessageBounds)
	}
	anchors := notification.ExportAnchors(layout.AnchorExportContext{ResolvedLayer: layout.ResolvedLayer{Bounds: bounds}})
	for _, name := range []layout.AnchorID{"bounds_center", "bounds_top_left", "bounds_top_right", "bounds_bottom_left", "bounds_bottom_right", "baseline", "content_anchor"} {
		if _, ok := anchors[name]; !ok {
			t.Fatalf("missing anchor %q", name)
		}
	}
	cmds := notification.Projection.Project(facet.ProjectionContext{
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

func TestNotificationInteractionsEmitActionAndDismiss(t *testing.T) {
	notification := newNotificationFixture()
	tokens := notificationTokens()
	resolved := notificationResolvedContext(tokens, theme.DensityIDComfortable, layout.WritingDirectionLTR)
	rt := notificationRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}

	facet.Attach(notification, facet.AttachContext{Runtime: rt, Theme: resolved})
	_ = 	notification.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(theme.DensityIDComfortable),
		WritingDirection: facet.WritingDirectionLTR,
	}, facet.Constraints{MaxSize: gfx.Size{W: 440, H: 180}})
	notification.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: notification.Layout.Parent,
		ChildGroup:  notification.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, gfx.RectFromXYWH(0, 0, 	notification.Layout.MeasuredSize.W, 	notification.Layout.MeasuredSize.H))

	var actioned, dismissed int
	notification.Actioned.Subscribe(func(signal.Unit) { actioned++ })
	notification.Dismissed.Subscribe(func(signal.Unit) { dismissed++ })

	if notification.cachedActionButton == nil {
		t.Fatal("expected action button")
	}
	actionBounds := notification.cachedActionBounds
	if !notification.cachedActionButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action press to be handled")
	}
	if !notification.cachedActionButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: actionBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected action release to be handled")
	}
	if actioned != 1 {
		t.Fatalf("expected one action emission, got %d", actioned)
	}

	if notification.cachedCloseButton == nil {
		t.Fatal("expected close button")
	}
	closeBounds := notification.cachedCloseBounds
	if !notification.cachedCloseButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerPress, Position: closeBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected close press to be handled")
	}
	if !notification.cachedCloseButton.Base().InputRole().OnPointer(facet.PointerEvent{Kind: platform.PointerRelease, Position: closeBounds.Min, Button: platform.PointerLeft}) {
		t.Fatal("expected close release to be handled")
	}
	if dismissed != 1 {
		t.Fatalf("expected one dismiss emission, got %d", dismissed)
	}
}

func TestNotificationRecipe_exposes_expected_slots(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := uinotification.ResolveNotificationRecipe(ctx)
	if report.Family != "uinotification" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("variant = %q, want standard", report.Variant)
	}
	for _, name := range []string{"Root", "StatusSurface", "Icon", "Title", "Message", "Action", "CloseButton"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected slot source for %s", name)
		}
	}
	if slots.StatusSurface.Base.Fills == nil || slots.Icon.Base.Fills == nil {
		t.Fatal("expected notification slots to be populated")
	}
}

func TestNotificationGoldenDefault(t *testing.T) {
	AssertNotificationGolden(t, "default", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {})
}

func TestNotificationGoldenCompact(t *testing.T) {
	AssertNotificationGolden(t, "compact", notificationTokens(), theme.DensityIDCompact, layout.WritingDirectionLTR, func(n *Notification) {})
}

func TestNotificationGoldenDisabled(t *testing.T) {
	AssertNotificationGolden(t, "disabled", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {
		n.Disabled = marks.Const(true)
	})
}

func TestNotificationGoldenHighContrast(t *testing.T) {
	AssertNotificationGolden(t, "high_contrast", notificationHighContrastTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {})
}

func TestNotificationGoldenHovered(t *testing.T) {
	AssertNotificationGolden(t, "hovered", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {
		n.hovered = true
	})
}

func TestNotificationGoldenPressed(t *testing.T) {
	AssertNotificationGolden(t, "pressed", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {
		n.pressed = true
	})
}

func TestNotificationGoldenRTL(t *testing.T) {
	AssertNotificationGolden(t, "rtl", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionRTL, func(n *Notification) {})
}

func TestNotificationGoldenOpen(t *testing.T) {
	AssertNotificationGolden(t, "open", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {
		n.Open = marks.Const(true)
	})
}

func TestNotificationGoldenGridContent(t *testing.T) {
	AssertNotificationGolden(t, "content_grid", notificationTokens(), theme.DensityIDComfortable, layout.WritingDirectionLTR, func(n *Notification) {
		n.ContentLayoutMode = marks.Const(NotificationContentLayoutGrid)
		n.ContentGridColumns = marks.Const(2)
		n.ContentGridRows = marks.Const(2)
		n.ContentChildren = marks.Const([]NotificationContentChild{
			{Key: "one", Facet: primitive.NewText(marks.Const("One"))},
			{Key: "two", Facet: primitive.NewText(marks.Const("Two"))},
			{Key: "three", Facet: primitive.NewText(marks.Const("Three"))},
		})
	})
}

func AssertNotificationGolden(t *testing.T, name string, tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection, mutate func(*Notification)) {
	t.Helper()
	notification := newNotificationFixture()
	rt := notificationRuntimeStub{
		rootStyle: theme.NewRootStyleContext(nil, tokens, nil),
		fonts:     testkit.TestFontRegistry(t),
	}
	resolved := notificationResolvedContext(tokens, density, direction)
	facet.Attach(notification, facet.AttachContext{Runtime: rt, Theme: resolved})
	canvas := gfx.RectFromXYWH(0, 0, 440, 180)
	_ = 	notification.Layout.Measure(facet.MeasureContext{
		Runtime:          rt,
		Theme:            resolved,
		ContentScale:     1,
		Density:          facet.DensityID(density),
		WritingDirection: facet.WritingDirection(direction),
	}, facet.Constraints{MaxSize: gfx.Size{W: canvas.Width(), H: canvas.Height()}})
	notification.Layout.Arrange(facet.ArrangeContext{
		Runtime:     rt,
		Theme:       resolved,
		ParentGroup: notification.Layout.Parent,
		ChildGroup:  notification.Layout.Child,
		Placement:   facet.Placement{Mode: facet.PlacementLinear},
	}, canvas)
	if mutate != nil {
		mutate(notification)
	}
	cmds := notification.Projection.Project(facet.ProjectionContext{Runtime: rt, Bounds: canvas, ContentScale: 1})
	if cmds == nil {
		cmds = &gfx.CommandList{}
	}
	surface := testkit.NewMemorySurface(440, 180)
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
	testkit.AssertGolden(t, surface, "notification_"+name)
}

func newNotificationFixture() *Notification {
	notification := NewNotification("Saved", "Draft was synced successfully.")
	notification.ActionLabel = marks.Const("Undo")
	notification.CloseButtonLabel = marks.Const("Dismiss")
	return notification
}

func notificationTokens() theme.Tokens {
	return toThemeTokens(templates.Notes().Tokens)
}

func notificationHighContrastTokens() theme.Tokens {
	return toThemeTokens(templates.UneNuit().Tokens)
}

func notificationResolvedContext(tokens theme.Tokens, density theme.DensityID, direction layout.WritingDirection) theme.ResolvedContext {
	ctx := theme.DefaultResolvedContext()
	rv := reflect.ValueOf(&ctx).Elem()
	field := rv.FieldByName("defaultContext")
	fieldCopy := reflect.NewAt(field.Type(), unsafe.Pointer(field.UnsafeAddr())).Elem()
	tokensField := fieldCopy.FieldByName("tokens")
	reflect.NewAt(tokensField.Type(), unsafe.Pointer(tokensField.UnsafeAddr())).Elem().Set(reflect.ValueOf(tokens))
	ctx = ctx.WithDensity(theme.DefaultDensityScale(density, tokens))
	ctx = ctx.WithWritingDirection(direction)
	return ctx
}


