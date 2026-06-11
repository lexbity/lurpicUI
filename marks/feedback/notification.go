package feedback

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	notificationMarkIDRoot        facet.MarkID = 1
	notificationMarkIDSurface     facet.MarkID = 2
	notificationMarkIDIcon        facet.MarkID = 3
	notificationMarkIDTitle       facet.MarkID = 4
	notificationMarkIDMessage     facet.MarkID = 5
	notificationMarkIDAction      facet.MarkID = 6
	notificationMarkIDCloseButton facet.MarkID = 7
	notificationMarkIDContent     facet.MarkID = 8
	notificationMarkIDContentItem facet.MarkID = 9
)

// NotificationContentLayoutMode controls how authored notification body content is arranged.
type NotificationContentLayoutMode uint8

const (
	NotificationContentLayoutVertical NotificationContentLayoutMode = iota
	NotificationContentLayoutHorizontal
	NotificationContentLayoutGrid
)

func (m NotificationContentLayoutMode) String() string {
	switch m {
	case NotificationContentLayoutVertical:
		return "vertical"
	case NotificationContentLayoutHorizontal:
		return "horizontal"
	case NotificationContentLayoutGrid:
		return "grid"
	default:
		return "unknown"
	}
}

// NotificationContentChild describes one reusable child facet placed inside the notification body.
type NotificationContentChild struct {
	Key       string
	Facet     facet.FacetImpl
	MarkID    facet.MarkID
	Grid      facet.GridPlacement
	ZPriority int32
}

// Notification implements the feedback.notification canonical mark.
type Notification struct {
	marks.Core

	textRole facet.TextRole

	Actioned  signal.Signal[signal.Unit]
	Dismissed signal.Signal[signal.Unit]

	Title              marks.Binding[string]
	Message            marks.Binding[string]
	IconRef            marks.Binding[string]
	ActionLabel        marks.Binding[string]
	ActionDisabled     marks.Binding[bool]
	CloseButtonLabel   marks.Binding[string]
	ContentLayoutMode  marks.Binding[NotificationContentLayoutMode]
	ContentGridColumns marks.Binding[int]
	ContentGridRows    marks.Binding[int]
	ContentChildren    marks.Binding[[]NotificationContentChild]
	Disabled           marks.Binding[bool]
	Open               marks.Binding[bool]

	hovered        bool
	pressed        bool
	focusedVisible bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.FeedbackNotificationSlots
	cachedBounds           gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedIconBounds       gfx.Rect
	cachedTitleBounds      gfx.Rect
	cachedMessageBounds    gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedActionBounds     gfx.Rect
	cachedCloseBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRowGap           float32
	cachedSurfaceRadius    float32
	cachedWritingDirection facet.WritingDirection
	cachedIconFacet        *primitive.Icon
	cachedTitleFacet       *primitive.Text
	cachedMessageFacet     *primitive.Text
	cachedContentGroup     *notificationContentGroup
	cachedActionButton     *action.Button
	cachedCloseButton      *action.IconButton
}

var _ facet.FacetImpl = (*Notification)(nil)
var _ layout.AnchorExporter = (*Notification)(nil)
var _ marks.Mark = (*Notification)(nil)

const notificationDefaultIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2.8 3.5 19.5h17L12 2.8z"/><path d="M12 9v4.5"/><circle cx="12" cy="16.5" r="1"/></svg>`

// NewNotification constructs a feedback.notification mark with canonical defaults.
func NewNotification(title, message string) *Notification {
	n := &Notification{
		Title:              marks.Const(title),
		Message:            marks.Const(message),
		IconRef:            marks.Const(""),
		ActionLabel:        marks.Const(""),
		ActionDisabled:     marks.Const(false),
		CloseButtonLabel:   marks.Const(""),
		ContentLayoutMode:  marks.Const(NotificationContentLayoutVertical),
		ContentGridColumns: marks.Const(1),
		ContentGridRows:    marks.Const(1),
		ContentChildren:    marks.Const[[]NotificationContentChild](nil),
		Disabled:           marks.Const(false),
		Open:               marks.Const(true),
	}
	n.Facet = facet.NewFacet()
	n.AddBinding(n.Title)
	n.AddBinding(n.Message)
	n.AddBinding(n.IconRef)
	n.AddBinding(n.ActionLabel)
	n.AddBinding(n.ActionDisabled)
	n.AddBinding(n.CloseButtonLabel)
	n.AddBinding(n.ContentLayoutMode)
	n.AddBinding(n.ContentGridColumns)
	n.AddBinding(n.ContentGridRows)
	n.AddBinding(n.ContentChildren)
	n.AddBinding(n.Disabled)
	n.AddBinding(n.Open)

	n.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   notificationGroupPolicy{notification: n},
		Children: n,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	n.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := n.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	n.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return n.measure(ctx, constraints)
	}
	n.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		n.Layout.ArrangedBounds = bounds
		n.arrange(ctx, bounds)
	}
	n.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := n.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	n.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return n.hitTest(p) }
	n.Input.OnPointer = func(e facet.PointerEvent) bool { return n.onPointer(e) }
	n.Input.OnKey = func(e facet.KeyEvent) bool { return false }
	n.textRole.IMEEnabled = false
	n.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return n.buildCommands(n.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	n.RegisterRoles()
	n.AddRole(&n.textRole)
	n.syncChildren()
	return n
}

// Base satisfies facet.FacetImpl.
func (n *Notification) Base() *facet.Facet {
	n.BindImpl(n)
	return &n.Facet
}

// Descriptor satisfies marks.Mark.
func (n *Notification) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "feedback", TypeName: "notification"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (n *Notification) AccessibilityRole() string { return "status" }

// AccessibleName reports the semantic name source required by the spec.
func (n *Notification) AccessibleName() string {
	if n == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(n.Title.Get()), strings.TrimSpace(n.Message.Get())}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// Children returns the notification's immediate semantic children.
func (n *Notification) Children() []facet.GroupChild {
	if n == nil || !n.Open.Get() {
		return nil
	}
	n.syncChildren()
	out := make([]facet.GroupChild, 0, 4)
	if n.cachedIconFacet != nil {
		out = append(out, notificationGroupChild(n.cachedIconFacet.Base(), notificationMarkIDIcon, 0, facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 0, CrossAxisAlign: facet.CrossAxisStart}}))
	}
	if n.cachedContentGroup != nil {
		out = append(out, notificationGroupChild(n.cachedContentGroup.Base(), notificationMarkIDContent, 1, facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 1, CrossAxisAlign: facet.CrossAxisStart}}))
	}
	if n.cachedActionButton != nil {
		out = append(out, notificationGroupChild(n.cachedActionButton.Base(), notificationMarkIDAction, 2, facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 2, CrossAxisAlign: facet.CrossAxisStart}}))
	}
	if n.cachedCloseButton != nil {
		out = append(out, notificationGroupChild(n.cachedCloseButton.Base(), notificationMarkIDCloseButton, 3, facet.Placement{Mode: facet.PlacementLinear, Linear: facet.LinearPlacement{Order: 3, CrossAxisAlign: facet.CrossAxisStart}}))
	}
	return out
}

// ExportAnchors publishes the notification anchor set.
func (n *Notification) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if n == nil {
		return nil
	}
	bounds := n.Layout.ArrangedBounds
	out := n.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if !n.cachedTitleBounds.IsEmpty() {
		out["baseline"] = gfx.Point{X: n.cachedTitleBounds.Min.X, Y: n.cachedTitleBounds.Min.Y}
	} else if n.cachedContentGroup != nil && !n.cachedContentGroup.cachedTitleBounds.IsEmpty() {
		out["baseline"] = gfx.Point{X: n.cachedContentGroup.cachedTitleBounds.Min.X, Y: n.cachedContentGroup.cachedTitleBounds.Min.Y}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	if !n.cachedSurfaceBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{X: (n.cachedSurfaceBounds.Min.X + n.cachedSurfaceBounds.Max.X) * 0.5, Y: (n.cachedSurfaceBounds.Min.Y + n.cachedSurfaceBounds.Max.Y) * 0.5}
		out["status_surface"] = gfx.Point{X: (n.cachedSurfaceBounds.Min.X + n.cachedSurfaceBounds.Max.X) * 0.5, Y: (n.cachedSurfaceBounds.Min.Y + n.cachedSurfaceBounds.Max.Y) * 0.5}
	}
	if !n.cachedIconBounds.IsEmpty() {
		out["icon"] = gfx.Point{X: (n.cachedIconBounds.Min.X + n.cachedIconBounds.Max.X) * 0.5, Y: (n.cachedIconBounds.Min.Y + n.cachedIconBounds.Max.Y) * 0.5}
	}
	if !n.cachedTitleBounds.IsEmpty() {
		out["title"] = gfx.Point{X: (n.cachedTitleBounds.Min.X + n.cachedTitleBounds.Max.X) * 0.5, Y: (n.cachedTitleBounds.Min.Y + n.cachedTitleBounds.Max.Y) * 0.5}
	}
	if !n.cachedMessageBounds.IsEmpty() {
		out["message"] = gfx.Point{X: (n.cachedMessageBounds.Min.X + n.cachedMessageBounds.Max.X) * 0.5, Y: (n.cachedMessageBounds.Min.Y + n.cachedMessageBounds.Max.Y) * 0.5}
	}
	if !n.cachedActionBounds.IsEmpty() {
		out["action"] = gfx.Point{X: (n.cachedActionBounds.Min.X + n.cachedActionBounds.Max.X) * 0.5, Y: (n.cachedActionBounds.Min.Y + n.cachedActionBounds.Max.Y) * 0.5}
	}
	if !n.cachedCloseBounds.IsEmpty() {
		out["close_button"] = gfx.Point{X: (n.cachedCloseBounds.Min.X + n.cachedCloseBounds.Max.X) * 0.5, Y: (n.cachedCloseBounds.Min.Y + n.cachedCloseBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach delegates to Core.
func (n *Notification) OnAttach(ctx facet.AttachContext) { n.Core.OnAttach() }

// OnActivate delegates to Core.
func (n *Notification) OnActivate() { n.Core.OnActivate() }

// OnDeactivate delegates to Core.
func (n *Notification) OnDeactivate() { n.Core.OnDeactivate() }

// OnFocusGained is unused.
func (n *Notification) OnFocusGained() {}

// OnFocusLost is unused.
func (n *Notification) OnFocusLost() {}

// OnDetach clears cached projection state.
func (n *Notification) OnDetach() {
	n.Core.OnDetach()
	n.cachedTokens = theme.Tokens{}
	n.cachedRecipe = shared.FeedbackNotificationSlots{}
	n.cachedBounds = gfx.Rect{}
	n.cachedSurfaceBounds = gfx.Rect{}
	n.cachedIconBounds = gfx.Rect{}
	n.cachedTitleBounds = gfx.Rect{}
	n.cachedMessageBounds = gfx.Rect{}
	n.cachedContentBounds = gfx.Rect{}
	n.cachedActionBounds = gfx.Rect{}
	n.cachedCloseBounds = gfx.Rect{}
	n.cachedPadX = 0
	n.cachedPadY = 0
	n.cachedGap = 0
	n.cachedRowGap = 0
	n.cachedSurfaceRadius = 0
	n.cachedIconFacet = nil
	n.cachedTitleFacet = nil
	n.cachedMessageFacet = nil
	n.cachedContentGroup = nil
	n.cachedActionButton = nil
	n.cachedCloseButton = nil
}

func (n *Notification) invalidate(flags facet.DirtyFlags) {
	if n == nil {
		return
	}
	n.Invalidate(flags)
}

func (n *Notification) syncChildren() {
	if n == nil {
		return
	}
	iconSource := strings.TrimSpace(n.IconRef.Get())
	if iconSource == "" {
		iconSource = notificationDefaultIconSVG
	}
	if n.cachedIconFacet == nil {
		n.cachedIconFacet = primitive.NewIcon(primitive.IconSVG(iconSource))
	} else {
		n.cachedIconFacet.Source = primitive.IconSVG(iconSource)
	}
	n.cachedIconFacet.Decorative = marks.Const(true)
	title := strings.TrimSpace(n.Title.Get())
	if n.cachedTitleFacet == nil {
		n.cachedTitleFacet = primitive.NewText(marks.Const(title))
	} else {
		n.cachedTitleFacet.Content = marks.Const(title)
		n.cachedTitleFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	n.cachedTitleFacet.Typography = marks.Const(theme.TextHeadingS)
	n.cachedTitleFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	n.cachedTitleFacet.Foreground = marks.Const(theme.ColorText)
	if n.Disabled.Get() {
		n.cachedTitleFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		n.cachedTitleFacet.Disabled = marks.Const(true)
	} else {
		n.cachedTitleFacet.Disabled = marks.Const(false)
	}
	message := strings.TrimSpace(n.Message.Get())
	if n.cachedMessageFacet == nil {
		n.cachedMessageFacet = primitive.NewText(marks.Const(message))
	} else {
		n.cachedMessageFacet.Content = marks.Const(message)
		n.cachedMessageFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	n.cachedMessageFacet.Typography = marks.Const(theme.TextBodyM)
	n.cachedMessageFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	n.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextSecondary)
	if n.Disabled.Get() {
		n.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		n.cachedMessageFacet.Disabled = marks.Const(true)
	} else {
		n.cachedMessageFacet.Disabled = marks.Const(false)
	}
	if n.cachedContentGroup == nil {
		n.cachedContentGroup = newNotificationContentGroup(n)
	}
	n.cachedContentGroup.syncContent()
	if strings.TrimSpace(n.ActionLabel.Get()) == "" {
		n.cachedActionButton = nil
	} else {
		if n.cachedActionButton == nil {
			n.cachedActionButton = action.NewButton(marks.Const(strings.TrimSpace(n.ActionLabel.Get())), marks.Const(uiinput.ButtonText))
			n.cachedActionButton.Activated.Subscribe(func(signal.Unit) {
				if n != nil && !n.Disabled.Get() && n.Open.Get() {
					n.Actioned.Emit(signal.Unit{})
				}
			})
		}
		n.cachedActionButton.Label = marks.Const(strings.TrimSpace(n.ActionLabel.Get()))
		n.cachedActionButton.Variant = marks.Const(uiinput.ButtonText)
		n.cachedActionButton.Disabled = marks.Const(n.Disabled.Get() || n.ActionDisabled.Get())
	}
	if strings.TrimSpace(n.CloseButtonLabel.Get()) == "" {
		n.cachedCloseButton = nil
	} else {
		if n.cachedCloseButton == nil {
			n.cachedCloseButton = action.NewIconButton(primitive.IconSVG(dialogDefaultCloseSVG))
			n.cachedCloseButton.Activated.Subscribe(func(signal.Unit) {
				if n != nil && !n.Disabled.Get() && n.Open.Get() {
					n.closeAndDismiss()
				}
			})
		}
		n.cachedCloseButton.Icon = primitive.IconSVG(dialogDefaultCloseSVG)
		n.cachedCloseButton.AccessibleLabel = marks.Const(strings.TrimSpace(n.CloseButtonLabel.Get()))
		n.cachedCloseButton.Disabled = marks.Const(n.Disabled.Get())
	}
}

func (n *Notification) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if !n.Open.Get() {
		size := constraints.Constrain(gfx.Size{})
		n.Layout.MeasuredSize = size
		n.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return n.Layout.MeasuredResult
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uifeedback.ResolveNotificationRecipe(style, n.notificationVariant())
	n.cachedTokens = resolved.TokenSet()
	n.cachedRecipe = slots
	n.cachedWritingDirection = ctx.WritingDirection
	n.cachedPadX = mathutil.Max(resolved.Density.Scale(14), float32(resolved.Spacing(theme.SpacingL)))
	n.cachedPadY = mathutil.Max(resolved.Density.Scale(12), float32(resolved.Spacing(theme.SpacingM)))
	n.cachedGap = mathutil.Max(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	n.cachedRowGap = mathutil.Max(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingS)))
	n.cachedSurfaceRadius = mathutil.Max(float32(resolved.Radius(theme.RadiusL).Float32()), float32(resolved.Radius(theme.RadiusM).Float32()))
	n.syncChildren()
	innerMaxW := constraints.MaxSize.W
	if innerMaxW > 0 {
		innerMaxW = mathutil.Max(0, innerMaxW-n.cachedPadX*2)
	}
	measureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	iconSize := gfx.Size{}
	if n.cachedIconFacet != nil {
		target := mathutil.Max(resolved.Density.Scale(20), float32(resolved.TokenSet().Spacing.TouchTarget)*0.35)
		iconSize = n.cachedIconFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target, H: target}}).Size
	}
	actionSize := gfx.Size{}
	if n.cachedActionButton != nil {
		target := mathutil.Max(resolved.Density.Scale(24), float32(resolved.TokenSet().Spacing.TouchTarget)*0.55)
		actionSize = n.cachedActionButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target * 4, H: target}}).Size
	}
	closeSize := gfx.Size{}
	if n.cachedCloseButton != nil {
		target := mathutil.Max(resolved.Density.Scale(24), float32(resolved.TokenSet().Spacing.TouchTarget)*0.55)
		closeSize = n.cachedCloseButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target, H: target}}).Size
	}
	controlsW := float32(0)
	if iconSize.W > 0 || iconSize.H > 0 {
		controlsW += iconSize.W
	}
	if actionSize.W > 0 || actionSize.H > 0 {
		if controlsW > 0 {
			controlsW += n.cachedGap
		}
		controlsW += actionSize.W
	}
	if closeSize.W > 0 || closeSize.H > 0 {
		if controlsW > 0 {
			controlsW += n.cachedGap
		}
		controlsW += closeSize.W
	}
	bodyMaxW := innerMaxW
	if controlsW > 0 {
		bodyMaxW = mathutil.Max(0, innerMaxW-controlsW-n.cachedGap*2)
	}
	bodySize := gfx.Size{}
	if n.cachedContentGroup != nil {
		bodySize = n.cachedContentGroup.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: bodyMaxW, H: constraints.MaxSize.H}}).Size
	}
	contentW := controlsW
	if contentW > 0 {
		contentW += n.cachedGap
	}
	contentW += bodySize.W
	contentH := mathutil.Max(iconSize.H, bodySize.H)
	contentH = mathutil.Max(contentH, actionSize.H)
	contentH = mathutil.Max(contentH, closeSize.H)
	surfaceSize := gfx.Size{
		W: contentW + n.cachedPadX*2,
		H: contentH + n.cachedPadY*2,
	}
	surfaceSize.W = mathutil.Max(surfaceSize.W, resolved.Density.Scale(280))
	surfaceSize.H = mathutil.Max(surfaceSize.H, resolved.Density.Scale(72))
	if constraints.MaxSize.W > 0 {
		surfaceSize.W = mathutil.Min(surfaceSize.W, constraints.MaxSize.W)
	}
	if constraints.MaxSize.H > 0 {
		surfaceSize.H = mathutil.Min(surfaceSize.H, constraints.MaxSize.H)
	}
	size := constraints.Constrain(surfaceSize)
	n.Layout.MeasuredSize = size
	n.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	return n.Layout.MeasuredResult
}

func (n *Notification) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	n.cachedBounds = bounds
	n.cachedSurfaceBounds = gfx.Rect{}
	n.cachedIconBounds = gfx.Rect{}
	n.cachedTitleBounds = gfx.Rect{}
	n.cachedMessageBounds = gfx.Rect{}
	n.cachedContentBounds = gfx.Rect{}
	n.cachedActionBounds = gfx.Rect{}
	n.cachedCloseBounds = gfx.Rect{}
	n.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || !n.Open.Get() {
		return
	}
	n.syncChildren()
	margin := mathutil.Max(n.cachedPadX, n.cachedPadY)
	contentWidth := float32(0)
	contentHeight := float32(0)
	if n.cachedIconFacet != nil {
		size := n.cachedIconFacet.Base().LayoutRole().MeasuredSize
		contentWidth = mathutil.Max(contentWidth, size.W)
		contentHeight = mathutil.Max(contentHeight, size.H)
	}
	if n.cachedContentGroup != nil {
		size := n.cachedContentGroup.Base().LayoutRole().MeasuredSize
		contentWidth = mathutil.Max(contentWidth, size.W)
		contentHeight = mathutil.Max(contentHeight, size.H)
	}
	if n.cachedActionButton != nil {
		size := n.cachedActionButton.Base().LayoutRole().MeasuredSize
		contentWidth = mathutil.Max(contentWidth, size.W)
		contentHeight = mathutil.Max(contentHeight, size.H)
	}
	if n.cachedCloseButton != nil {
		size := n.cachedCloseButton.Base().LayoutRole().MeasuredSize
		contentWidth = mathutil.Max(contentWidth, size.W)
		contentHeight = mathutil.Max(contentHeight, size.H)
	}
	surfaceSize := gfx.Size{
		W: mathutil.Max(n.cachedPadX*2+contentWidth, 280),
		H: mathutil.Max(n.cachedPadY*2+contentHeight, 72),
	}
	surfaceSize.W = mathutil.Min(surfaceSize.W, mathutil.Max(0, bounds.Width()-margin*2))
	surfaceSize.H = mathutil.Min(surfaceSize.H, mathutil.Max(0, bounds.Height()-margin*2))
	n.cachedSurfaceBounds = text.CenterRect(bounds, surfaceSize.W, surfaceSize.H)
	content := n.cachedSurfaceBounds.Inset(n.cachedPadX, n.cachedPadY)
	if content.IsEmpty() {
		content = n.cachedSurfaceBounds
	}
	iconSize := gfx.Size{}
	if n.cachedIconFacet != nil {
		iconSize = n.cachedIconFacet.Base().LayoutRole().MeasuredSize
	}
	actionSize := gfx.Size{}
	if n.cachedActionButton != nil {
		actionSize = n.cachedActionButton.Base().LayoutRole().MeasuredSize
	}
	closeSize := gfx.Size{}
	if n.cachedCloseButton != nil {
		closeSize = n.cachedCloseButton.Base().LayoutRole().MeasuredSize
	}
	bodySize := gfx.Size{}
	if n.cachedContentGroup != nil {
		bodySize = n.cachedContentGroup.Base().LayoutRole().MeasuredSize
	}
	rowH := mathutil.Max(iconSize.H, bodySize.H)
	rowH = mathutil.Max(rowH, actionSize.H)
	rowH = mathutil.Max(rowH, closeSize.H)
	x := content.Min.X
	if n.cachedWritingDirection == facet.WritingDirectionRTL {
		x = content.Max.X
	}
	if n.cachedWritingDirection == facet.WritingDirectionRTL {
		if n.cachedCloseButton != nil && (closeSize.W > 0 || closeSize.H > 0) {
			x -= closeSize.W
			rect := gfx.RectFromXYWH(x, content.Min.Y, closeSize.W, rowH)
			n.cachedCloseButton.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedCloseBounds = rect
			x -= n.cachedGap
		}
		if n.cachedActionButton != nil && (actionSize.W > 0 || actionSize.H > 0) {
			x -= actionSize.W
			rect := gfx.RectFromXYWH(x, content.Min.Y, actionSize.W, rowH)
			n.cachedActionButton.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedActionBounds = rect
			x -= n.cachedGap
		}
		if n.cachedContentGroup != nil {
			bodyRect := gfx.RectFromXYWH(content.Min.X, content.Min.Y, mathutil.Max(0, x-content.Min.X), bodySize.H)
			n.cachedContentGroup.Base().LayoutRole().Arrange(ctx, bodyRect)
			n.cachedContentBounds = bodyRect
		}
		if n.cachedIconFacet != nil && (iconSize.W > 0 || iconSize.H > 0) {
			rect := gfx.RectFromXYWH(content.Max.X-iconSize.W, content.Min.Y, iconSize.W, rowH)
			n.cachedIconFacet.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedIconBounds = rect
		}
	} else {
		if n.cachedIconFacet != nil && (iconSize.W > 0 || iconSize.H > 0) {
			rect := gfx.RectFromXYWH(x, content.Min.Y, iconSize.W, rowH)
			n.cachedIconFacet.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedIconBounds = rect
			x += iconSize.W + n.cachedGap
		}
		if n.cachedContentGroup != nil {
			bodyRect := gfx.RectFromXYWH(x, content.Min.Y, mathutil.Max(0, content.Max.X-x), bodySize.H)
			n.cachedContentGroup.Base().LayoutRole().Arrange(ctx, bodyRect)
			n.cachedContentBounds = bodyRect
			x += bodySize.W + n.cachedGap
		}
		if n.cachedActionButton != nil && (actionSize.W > 0 || actionSize.H > 0) {
			rect := gfx.RectFromXYWH(x, content.Min.Y, actionSize.W, rowH)
			n.cachedActionButton.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedActionBounds = rect
			x += actionSize.W + n.cachedGap
		}
		if n.cachedCloseButton != nil && (closeSize.W > 0 || closeSize.H > 0) {
			rect := gfx.RectFromXYWH(x, content.Min.Y, closeSize.W, rowH)
			n.cachedCloseButton.Base().LayoutRole().Arrange(ctx, rect)
			n.cachedCloseBounds = rect
		}
	}
	if n.cachedContentGroup != nil {
		if n.cachedContentGroup.cachedTitleBounds.Width() > 0 {
			n.cachedTitleBounds = n.cachedContentGroup.cachedTitleBounds
		}
		if n.cachedContentGroup.cachedMessageBounds.Width() > 0 {
			n.cachedMessageBounds = n.cachedContentGroup.cachedMessageBounds
		}
	}
}

func (n *Notification) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if n == nil || bounds.IsEmpty() || !n.Open.Get() {
		return nil
	}
	style, slots := n.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := n.notificationState()
	root := slots.Root.Resolve(state, tokens)
	surface := slots.StatusSurface.Resolve(state, tokens)
	icon := slots.Icon.Resolve(state, tokens)
	title := slots.Title.Resolve(state, tokens)
	message := slots.Message.Resolve(state, tokens)
	actionSlot := slots.Action.Resolve(state, tokens)
	closeSlot := slots.CloseButton.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(surface) && !n.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(n.cachedSurfaceBounds, n.cachedSurfaceRadius), surface)...)
	}
	if !n.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: n.cachedSurfaceBounds})
		if !theme.IsTransparentMaterial(icon) && n.cachedIconFacet != nil && !n.cachedIconBounds.IsEmpty() {
			cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(n.cachedIconBounds), icon)...)
		}
		if n.cachedIconFacet != nil && !n.cachedIconBounds.IsEmpty() {
			if projected := n.cachedIconFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedIconBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if n.cachedContentGroup != nil && !n.cachedContentBounds.IsEmpty() {
			if projected := n.cachedContentGroup.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedContentBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if n.cachedActionButton != nil && !n.cachedActionBounds.IsEmpty() {
			if !theme.IsTransparentMaterial(actionSlot) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(n.cachedActionBounds, n.cachedSurfaceRadius*0.5), actionSlot)...)
			}
			if projected := n.cachedActionButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedActionBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if n.cachedCloseButton != nil && !n.cachedCloseBounds.IsEmpty() {
			if !theme.IsTransparentMaterial(closeSlot) {
				cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(n.cachedCloseBounds, n.cachedSurfaceRadius*0.5), closeSlot)...)
			}
			if projected := n.cachedCloseButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedCloseBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	if !theme.IsTransparentMaterial(title) && !n.cachedTitleBounds.IsEmpty() {
		_ = title
	}
	if !theme.IsTransparentMaterial(message) && !n.cachedMessageBounds.IsEmpty() {
		_ = message
	}
	return cmds
}

func (n *Notification) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.FeedbackNotificationSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: n.cachedTokens}, n.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, n.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uifeedback.ResolveNotificationRecipe(style, n.notificationVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: n.cachedTokens}, n.cachedRecipe
}

func (n *Notification) notificationState() theme.InteractionState {
	if n == nil {
		return theme.StateDefault
	}
	if n.Disabled.Get() {
		return theme.StateDisabled
	}
	if n.pressed {
		return theme.StatePressed
	}
	if n.hovered {
		return theme.StateHover
	}
	return theme.StateDefault
}

func (n *Notification) notificationVariant() uifeedback.NotificationVariant {
	if n == nil {
		return uifeedback.NotificationDefault
	}
	if n.Disabled.Get() {
		return uifeedback.NotificationDisabled
	}
	if n.pressed {
		return uifeedback.NotificationActive
	}
	if n.hovered {
		return uifeedback.NotificationHover
	}
	if n.Open.Get() {
		return uifeedback.NotificationOpen
	}
	return uifeedback.NotificationDefault
}

type notificationGroupPolicy struct {
	notification *Notification
}

func (notificationGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p notificationGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.notification == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.notification.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p notificationGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.notification == nil {
		return nil, nil
	}
	p.notification.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for _, child := range children {
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    child.Layout.ArrangedBounds,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}

func (n *Notification) hitTest(p gfx.Point) facet.HitResult {
	if n == nil || !n.Open.Get() || n.cachedBounds.IsEmpty() || !n.cachedBounds.Contains(p) {
		return facet.HitResult{}
	}
	switch {
	case !n.cachedCloseBounds.IsEmpty() && n.cachedCloseBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDCloseButton, Cursor: facet.CursorPointer}
	case !n.cachedActionBounds.IsEmpty() && n.cachedActionBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDAction, Cursor: facet.CursorPointer}
	case !n.cachedIconBounds.IsEmpty() && n.cachedIconBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDIcon}
	case !n.cachedContentBounds.IsEmpty() && n.cachedContentBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDContent}
	case !n.cachedSurfaceBounds.IsEmpty() && n.cachedSurfaceBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDSurface}
	default:
		return facet.HitResult{Hit: true, MarkID: notificationMarkIDRoot}
	}
}

func (n *Notification) onPointer(e facet.PointerEvent) bool {
	if n == nil || n.Disabled.Get() || !n.Open.Get() {
		return false
	}
	if !n.cachedBounds.Contains(e.Position) {
		if e.Kind == platform.PointerLeave {
			n.hovered = false
			n.pressed = false
			n.invalidate(facet.DirtyProjection)
		}
		return false
	}
	if n.cachedActionButton != nil && !n.cachedActionBounds.IsEmpty() && n.cachedActionBounds.Contains(e.Position) {
		if n.cachedActionButton.Base().InputRole() != nil && n.cachedActionButton.Base().InputRole().OnPointer(e) {
			return true
		}
	}
	if n.cachedCloseButton != nil && !n.cachedCloseBounds.IsEmpty() && n.cachedCloseBounds.Contains(e.Position) {
		if n.cachedCloseButton.Base().InputRole() != nil && n.cachedCloseButton.Base().InputRole().OnPointer(e) {
			return true
		}
	}
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if !n.hovered {
			n.hovered = true
			n.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			n.pressed = true
			n.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerRelease:
		if n.pressed {
			n.pressed = false
			n.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerLeave:
		n.hovered = false
		n.pressed = false
		n.invalidate(facet.DirtyProjection)
		return true
	}
	return false
}

func (n *Notification) closeAndDismiss() {
	if n == nil || !n.Open.Get() {
		return
	}
	n.Open = marks.Const(false)
	n.hovered = false
	n.pressed = false
	n.focusedVisible = false
	n.Dismissed.Emit(signal.Unit{})
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func notificationGroupChild(base *facet.Facet, markID facet.MarkID, order int, placement facet.Placement) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: placement,
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func notificationContentGroupChild(base *facet.Facet, markID facet.MarkID, order int, placement facet.Placement) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: placement,
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

type notificationContentGroup struct {
	marks.Core

	textRole facet.TextRole

	parent *Notification

	cachedTitleFacet   *primitive.Text
	cachedMessageFacet *primitive.Text

	cachedBounds        gfx.Rect
	cachedTitleBounds   gfx.Rect
	cachedMessageBounds gfx.Rect
	cachedContentBounds gfx.Rect
	cachedChildren      []NotificationContentChild
	cachedMeasured      []notificationContentMeasure
	cachedChildrenMap   map[facet.FacetID]gfx.Rect
	cachedLayoutMode    NotificationContentLayoutMode
	cachedGridColumns   int
	cachedGridRows      int
}

func newNotificationContentGroup(parent *Notification) *notificationContentGroup {
	g := &notificationContentGroup{
		parent: parent,
	}
	g.Facet = facet.NewFacet()
	g.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   notificationContentGroupPolicy{group: g},
		Children: g,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	g.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := g.measure(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	g.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := g.measure(ctx, constraints)
		return facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	}
	g.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		g.Layout.ArrangedBounds = bounds
		g.arrange(ctx, bounds)
	}
	g.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := g.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	g.textRole.IMEEnabled = false
	g.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return g.buildCommands(g.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	g.RegisterRoles()
	g.AddRole(&g.textRole)
	return g
}

func (g *notificationContentGroup) Base() *facet.Facet {
	g.BindImpl(g)
	return &g.Facet
}

func (g *notificationContentGroup) Children() []facet.GroupChild {
	if g == nil || g.parent == nil || !g.parent.Open.Get() {
		return nil
	}
	g.syncContent()
	out := make([]facet.GroupChild, 0, 2+len(g.cachedChildren))
	if g.cachedTitleFacet != nil {
		out = append(out, notificationContentGroupChild(g.cachedTitleFacet.Base(), notificationMarkIDTitle, 0, g.contentPlacement(0, g.cachedTitleFacet.Base().LayoutRole().Child, facet.GridPlacement{})))
	}
	if g.cachedMessageFacet != nil {
		out = append(out, notificationContentGroupChild(g.cachedMessageFacet.Base(), notificationMarkIDMessage, 1, g.contentPlacement(1, g.cachedMessageFacet.Base().LayoutRole().Child, facet.GridPlacement{})))
	}
	for i := range g.cachedChildren {
		spec := g.cachedChildren[i]
		if spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil || base.LayoutRole() == nil {
			continue
		}
		order := i + 2
		out = append(out, notificationContentGroupChild(base, notificationMarkIDContentItem+facet.MarkID(i), order, g.contentPlacement(order, base.LayoutRole().Child, spec.Grid)))
	}
	return out
}

func (g *notificationContentGroup) OnAttach(ctx facet.AttachContext) { g.Core.OnAttach() }
func (g *notificationContentGroup) OnActivate()                      { g.Core.OnActivate() }
func (g *notificationContentGroup) OnDeactivate()                    { g.Core.OnDeactivate() }
func (g *notificationContentGroup) OnDetach()                        { g.Core.OnDetach() }

func (g *notificationContentGroup) syncContent() {
	if g == nil || g.parent == nil {
		return
	}
	g.cachedLayoutMode = g.parent.ContentLayoutMode.Get()
	g.cachedGridColumns = g.parent.ContentGridColumns.Get()
	g.cachedGridRows = g.parent.ContentGridRows.Get()
	if g.cachedGridColumns < 1 {
		g.cachedGridColumns = 1
	}
	if g.cachedGridRows < 1 {
		g.cachedGridRows = 1
	}
	if g.Layout.Parent.Policy != nil {
		g.Layout.Parent.Kind = g.groupKind()
	}
	title := strings.TrimSpace(g.parent.Title.Get())
	if g.cachedTitleFacet == nil {
		g.cachedTitleFacet = primitive.NewText(marks.Const(title))
	} else {
		g.cachedTitleFacet.Content = marks.Const(title)
		g.cachedTitleFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	g.cachedTitleFacet.Typography = marks.Const(theme.TextHeadingS)
	g.cachedTitleFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	g.cachedTitleFacet.Foreground = marks.Const(theme.ColorText)
	if g.parent.Disabled.Get() {
		g.cachedTitleFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		g.cachedTitleFacet.Disabled = marks.Const(true)
	} else {
		g.cachedTitleFacet.Disabled = marks.Const(false)
	}
	message := strings.TrimSpace(g.parent.Message.Get())
	if g.cachedMessageFacet == nil {
		g.cachedMessageFacet = primitive.NewText(marks.Const(message))
	} else {
		g.cachedMessageFacet.Content = marks.Const(message)
		g.cachedMessageFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	g.cachedMessageFacet.Typography = marks.Const(theme.TextBodyM)
	g.cachedMessageFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	g.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextSecondary)
	if g.parent.Disabled.Get() {
		g.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		g.cachedMessageFacet.Disabled = marks.Const(true)
	} else {
		g.cachedMessageFacet.Disabled = marks.Const(false)
	}
	g.cachedChildren = append(g.cachedChildren[:0], g.parent.ContentChildren.Get()...)
}

func (g *notificationContentGroup) groupKind() facet.GroupLayoutKind {
	switch g.cachedLayoutMode {
	case NotificationContentLayoutHorizontal:
		return facet.GroupLayoutLinearHorizontal
	case NotificationContentLayoutGrid:
		return facet.GroupLayoutGrid
	default:
		return facet.GroupLayoutLinearVertical
	}
}

func (g *notificationContentGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if g == nil || g.parent == nil || !g.parent.Open.Get() {
		return gfx.Size{}
	}
	g.syncContent()
	if g.cachedTitleFacet == nil && g.cachedMessageFacet == nil && len(g.cachedChildren) == 0 {
		g.cachedBounds = gfx.Rect{}
		g.cachedTitleBounds = gfx.Rect{}
		g.cachedMessageBounds = gfx.Rect{}
		g.cachedContentBounds = gfx.Rect{}
		g.cachedChildrenMap = nil
		return gfx.Size{}
	}
	measureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	children := g.measureChildren(measureCtx, constraints.MaxSize)
	g.cachedMeasured = append(g.cachedMeasured[:0], children...)
	size := g.measureContentSize(children, constraints.MaxSize)
	g.cachedBounds = gfx.RectFromXYWH(0, 0, size.W, size.H)
	g.cachedContentBounds = g.cachedBounds
	return size
}

func (g *notificationContentGroup) measureChildren(ctx facet.MeasureContext, maxSize gfx.Size) []notificationContentMeasure {
	out := make([]notificationContentMeasure, 0, 2+len(g.cachedChildren))
	if g.cachedTitleFacet != nil && g.cachedTitleFacet.Base() != nil && g.cachedTitleFacet.Base().LayoutRole() != nil {
		size := g.cachedTitleFacet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: maxSize}).Size
		out = append(out, notificationContentMeasure{facet: g.cachedTitleFacet.Base(), size: size})
	}
	if g.cachedMessageFacet != nil && g.cachedMessageFacet.Base() != nil && g.cachedMessageFacet.Base().LayoutRole() != nil {
		size := g.cachedMessageFacet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: maxSize}).Size
		out = append(out, notificationContentMeasure{facet: g.cachedMessageFacet.Base(), size: size})
	}
	for i := range g.cachedChildren {
		spec := g.cachedChildren[i]
		if spec.Facet == nil {
			continue
		}
		base := spec.Facet.Base()
		if base == nil || base.LayoutRole() == nil {
			continue
		}
		size := base.LayoutRole().Measure(ctx, facet.Constraints{MaxSize: maxSize}).Size
		out = append(out, notificationContentMeasure{
			facet:     base,
			size:      size,
			grid:      spec.Grid,
			markID:    spec.MarkID,
			zPriority: spec.ZPriority,
		})
	}
	return out
}

func (g *notificationContentGroup) measureContentSize(children []notificationContentMeasure, maxSize gfx.Size) gfx.Size {
	if len(children) == 0 {
		return gfx.Size{}
	}
	switch g.cachedLayoutMode {
	case NotificationContentLayoutHorizontal:
		var width float32
		var height float32
		for i := range children {
			if i > 0 {
				width += g.parent.cachedGap
			}
			width += children[i].size.W
			height = mathutil.Max(height, children[i].size.H)
		}
		return gfx.Size{W: width, H: height}
	case NotificationContentLayoutGrid:
		cfg := g.gridConfig()
		policy := layoutgrid.New(cfg)
		gridChildren := g.gridChildren(children)
		size, err := policy.Measure(gridChildren, maxSize)
		if err != nil {
			return gfx.Size{}
		}
		return size
	default:
		var width float32
		var height float32
		for i := range children {
			if i > 0 {
				height += g.parent.cachedRowGap
			}
			width = mathutil.Max(width, children[i].size.W)
			height += children[i].size.H
		}
		return gfx.Size{W: width, H: height}
	}
}

func (g *notificationContentGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open.Get() {
		return
	}
	g.syncContent()
	g.cachedBounds = bounds
	g.cachedTitleBounds = gfx.Rect{}
	g.cachedMessageBounds = gfx.Rect{}
	g.cachedContentBounds = gfx.Rect{}
	if g.cachedTitleFacet == nil && g.cachedMessageFacet == nil && len(g.cachedChildren) == 0 {
		g.cachedChildrenMap = nil
		return
	}
	children := g.cachedMeasured
	arranged := g.arrangeChildren(ctx, bounds, children)
	g.cachedChildrenMap = make(map[facet.FacetID]gfx.Rect, len(arranged))
	for _, child := range arranged {
		g.cachedChildrenMap[child.facet.ID()] = child.bounds
		if g.cachedTitleFacet != nil && child.facet.ID() == g.cachedTitleFacet.Base().ID() {
			g.cachedTitleBounds = child.bounds
		}
		if g.cachedMessageFacet != nil && child.facet.ID() == g.cachedMessageFacet.Base().ID() {
			g.cachedMessageBounds = child.bounds
		}
	}
	if len(arranged) > 0 {
		minX, minY := arranged[0].bounds.Min.X, arranged[0].bounds.Min.Y
		maxX, maxY := arranged[0].bounds.Max.X, arranged[0].bounds.Max.Y
		for _, child := range arranged[1:] {
			if child.bounds.Min.X < minX {
				minX = child.bounds.Min.X
			}
			if child.bounds.Min.Y < minY {
				minY = child.bounds.Min.Y
			}
			if child.bounds.Max.X > maxX {
				maxX = child.bounds.Max.X
			}
			if child.bounds.Max.Y > maxY {
				maxY = child.bounds.Max.Y
			}
		}
		g.cachedContentBounds = gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
	}
}

func (g *notificationContentGroup) arrangeChildren(ctx facet.ArrangeContext, bounds gfx.Rect, children []notificationContentMeasure) []notificationContentArrange {
	if len(children) == 0 {
		return nil
	}
	switch g.cachedLayoutMode {
	case NotificationContentLayoutHorizontal:
		x := bounds.Min.X
		out := make([]notificationContentArrange, 0, len(children))
		for i := range children {
			if i > 0 {
				x += g.parent.cachedGap
			}
			rect := gfx.RectFromXYWH(x, bounds.Min.Y, children[i].size.W, mathutil.Max(bounds.Height(), children[i].size.H))
			children[i].facet.LayoutRole().Arrange(ctx, rect)
			out = append(out, notificationContentArrange{facet: children[i].facet, bounds: rect})
			x += children[i].size.W
		}
		return out
	case NotificationContentLayoutGrid:
		cfg := g.gridConfig()
		policy := layoutgrid.New(cfg)
		gridChildren := g.gridChildren(children)
		arranged, err := policy.Arrange(gridChildren, bounds)
		if err != nil {
			return nil
		}
		measureByID := make(map[facet.FacetID]notificationContentMeasure, len(children))
		for i := range children {
			measureByID[children[i].facet.ID()] = children[i]
		}
		out := make([]notificationContentArrange, 0, len(arranged))
		for i := range arranged {
			child, ok := measureByID[arranged[i].FacetID]
			if !ok || child.facet == nil || child.facet.LayoutRole() == nil {
				continue
			}
			child.facet.LayoutRole().Arrange(ctx, arranged[i].Bounds)
			out = append(out, notificationContentArrange{facet: child.facet, bounds: arranged[i].Bounds})
		}
		return out
	default:
		y := bounds.Min.Y
		out := make([]notificationContentArrange, 0, len(children))
		for i := range children {
			if i > 0 {
				y += g.parent.cachedRowGap
			}
			rect := gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), children[i].size.H)
			children[i].facet.LayoutRole().Arrange(ctx, rect)
			out = append(out, notificationContentArrange{facet: children[i].facet, bounds: rect})
			y += children[i].size.H
		}
		return out
	}
}

func (g *notificationContentGroup) gridConfig() layoutgrid.Config {
	columns := g.cachedGridColumns
	rows := g.cachedGridRows
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}
	return layoutgrid.Config{
		Columns:       flexibleTracks(columns),
		Rows:          flexibleTracks(rows),
		ColumnGap:     g.parent.cachedGap,
		RowGap:        g.parent.cachedRowGap,
		AutoPlacement: layoutgrid.AutoRowFirst,
	}
}

func (g *notificationContentGroup) gridChildren(children []notificationContentMeasure) []layoutgrid.Child {
	out := make([]layoutgrid.Child, 0, len(children))
	for i := range children {
		child := children[i]
		if child.facet == nil || child.facet.LayoutRole() == nil {
			continue
		}
		placement := child.grid
		out = append(out, layoutgrid.Child{
			FacetID: child.facet.ID(),
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementGrid,
					Grid: placement,
				},
				ZPriority: child.zPriority,
			},
			Layout:   child.facet.LayoutRole(),
			Contract: child.facet.LayoutRole().Child,
		})
	}
	return out
}

func (g *notificationContentGroup) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open.Get() {
		return nil
	}
	if g.cachedTitleFacet == nil && g.cachedMessageFacet == nil && len(g.cachedChildren) == 0 {
		return nil
	}
	cmds := make([]gfx.Command, 0, 16)
	if !g.cachedBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: g.cachedBounds})
		if g.cachedTitleFacet != nil && !g.cachedTitleBounds.IsEmpty() {
			if projected := g.cachedTitleFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: g.cachedTitleBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if g.cachedMessageFacet != nil && !g.cachedMessageBounds.IsEmpty() {
			if projected := g.cachedMessageFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: g.cachedMessageBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		for i := range g.cachedChildren {
			spec := g.cachedChildren[i]
			if spec.Facet == nil {
				continue
			}
			b, ok := g.cachedChildrenMap[spec.Facet.Base().ID()]
			if !ok || b.IsEmpty() {
				continue
			}
			if projected := spec.Facet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: b, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	return cmds
}

func (g *notificationContentGroup) contentPlacement(index int, contract facet.GroupChildContract, grid facet.GridPlacement) facet.Placement {
	switch g.cachedLayoutMode {
	case NotificationContentLayoutGrid:
		placement := grid
		if placement == (facet.GridPlacement{}) {
			placement = facet.GridPlacement{ColStart: 0, RowStart: index, ColSpan: 1, RowSpan: 1}
		}
		return facet.Placement{Mode: facet.PlacementGrid, Grid: placement, Align: facet.AlignStretch}
	case NotificationContentLayoutHorizontal, NotificationContentLayoutVertical:
		if contract.SupportedPlacement.Has(facet.PlacementLinear) {
			return facet.Placement{
				Mode: facet.PlacementLinear,
				Linear: facet.LinearPlacement{
					Order:          index,
					CrossAxisAlign: facet.CrossAxisStart,
				},
			}
		}
		placement := grid
		if placement == (facet.GridPlacement{}) {
			placement = facet.GridPlacement{ColStart: index, RowStart: 0, ColSpan: 1, RowSpan: 1}
			if g.cachedLayoutMode == NotificationContentLayoutVertical {
				placement.ColStart = 0
				placement.RowStart = index
			}
		}
		return facet.Placement{Mode: facet.PlacementGrid, Grid: placement, Align: facet.AlignStretch}
	default:
		return facet.Placement{Mode: facet.PlacementGrid, Grid: grid, Align: facet.AlignStretch}
	}
}

type notificationContentMeasure struct {
	facet     *facet.Facet
	size      gfx.Size
	grid      facet.GridPlacement
	markID    facet.MarkID
	zPriority int32
}

type notificationContentArrange struct {
	facet  *facet.Facet
	bounds gfx.Rect
}

type notificationContentGroupPolicy struct {
	group *notificationContentGroup
}

func (p notificationContentGroupPolicy) Kind() facet.GroupLayoutKind {
	if p.group == nil {
		return facet.GroupLayoutLinearVertical
	}
	return p.group.groupKind()
}

func (p notificationContentGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.group.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p notificationContentGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.group == nil {
		return nil, nil
	}
	p.group.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for _, child := range children {
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    child.Layout.ArrangedBounds,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
