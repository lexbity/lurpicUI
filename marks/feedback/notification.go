package feedback

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
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
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	textRole       facet.TextRole

	Actioned  signal.Signal[signal.Unit]
	Dismissed signal.Signal[signal.Unit]

	Title              string
	Message            string
	IconRef            string
	ActionLabel        string
	ActionDisabled     bool
	CloseButtonLabel   string
	ContentLayoutMode  NotificationContentLayoutMode
	ContentGridColumns int
	ContentGridRows    int
	ContentChildren    []NotificationContentChild
	Disabled           bool
	Open               bool

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

const notificationDefaultIconSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"><path d="M12 2.8 3.5 19.5h17L12 2.8z"/><path d="M12 9v4.5"/><circle cx="12" cy="16.5" r="1"/></svg>`

// NewNotification constructs a feedback.notification mark with canonical defaults.
func NewNotification(title, message string) *Notification {
	n := &Notification{
		Facet:             facet.NewFacet(),
		Title:             title,
		Message:           message,
		ContentLayoutMode: NotificationContentLayoutVertical,
		Open:              true,
	}
	n.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   notificationGroupPolicy{notification: n},
		Children: n,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	n.layoutRole.Child = facet.GroupChildContract{
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
	n.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return n.measure(ctx, constraints)
	}
	n.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		n.layoutRole.ArrangedBounds = bounds
		n.arrange(ctx, bounds)
	}
	n.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := n.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	n.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := n.buildCommands(n.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	n.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return n.hitTest(p) }
	n.inputRole.OnPointer = func(e facet.PointerEvent) bool { return n.onPointer(e) }
	n.inputRole.OnKey = func(e facet.KeyEvent) bool { return false }
	n.textRole.IMEEnabled = false
	n.AddRole(&n.layoutRole)
	n.AddRole(&n.renderRole)
	n.AddRole(&n.projectionRole)
	n.AddRole(&n.hitRole)
	n.AddRole(&n.inputRole)
	n.AddRole(&n.textRole)
	n.syncChildren()
	return n
}

// Base satisfies facet.FacetImpl.
func (n *Notification) Base() *facet.Facet {
	n.Facet.BindImpl(n)
	return &n.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (n *Notification) AccessibilityRole() string { return "status" }

// AccessibleName reports the semantic name source required by the spec.
func (n *Notification) AccessibleName() string {
	if n == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(n.Title), strings.TrimSpace(n.Message)}
	return strings.TrimSpace(strings.Join(parts, " "))
}

// SetTitle updates the authored notification title.
func (n *Notification) SetTitle(title string) {
	if n == nil || n.Title == title {
		return
	}
	n.Title = title
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetMessage updates the authored notification message.
func (n *Notification) SetMessage(message string) {
	if n == nil || n.Message == message {
		return
	}
	n.Message = message
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetIconRef updates the authored notification icon source.
func (n *Notification) SetIconRef(ref string) {
	if n == nil || n.IconRef == ref {
		return
	}
	n.IconRef = ref
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetActionLabel updates the authored action label.
func (n *Notification) SetActionLabel(label string) {
	if n == nil || n.ActionLabel == label {
		return
	}
	n.ActionLabel = label
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetActionDisabled toggles the action affordance.
func (n *Notification) SetActionDisabled(disabled bool) {
	if n == nil || n.ActionDisabled == disabled {
		return
	}
	n.ActionDisabled = disabled
	n.syncChildren()
	n.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetCloseButtonLabel updates the authored close-button label.
func (n *Notification) SetCloseButtonLabel(label string) {
	if n == nil || n.CloseButtonLabel == label {
		return
	}
	n.CloseButtonLabel = label
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetContentLayoutMode updates how custom body content is arranged.
func (n *Notification) SetContentLayoutMode(mode NotificationContentLayoutMode) {
	if n == nil || n.ContentLayoutMode == mode {
		return
	}
	n.ContentLayoutMode = mode
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetContentGrid defines the grid used when the body content layout is grid-based.
func (n *Notification) SetContentGrid(columns, rows int) {
	if n == nil {
		return
	}
	if columns < 1 {
		columns = 1
	}
	if rows < 1 {
		rows = 1
	}
	if n.ContentGridColumns == columns && n.ContentGridRows == rows {
		return
	}
	n.ContentGridColumns = columns
	n.ContentGridRows = rows
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetContentChildren updates the reusable body content facet list.
func (n *Notification) SetContentChildren(children []NotificationContentChild) {
	if n == nil {
		return
	}
	next := append([]NotificationContentChild(nil), children...)
	for i := range next {
		next[i].Key = strings.TrimSpace(next[i].Key)
	}
	n.ContentChildren = next
	n.syncChildren()
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles the disabled state.
func (n *Notification) SetDisabled(disabled bool) {
	if n == nil || n.Disabled == disabled {
		return
	}
	n.Disabled = disabled
	if disabled {
		n.hovered = false
		n.pressed = false
		n.focusedVisible = false
	}
	n.syncChildren()
	n.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetOpen toggles the notification visibility.
func (n *Notification) SetOpen(open bool) {
	if n == nil || n.Open == open {
		return
	}
	n.Open = open
	if !open {
		n.hovered = false
		n.pressed = false
		n.focusedVisible = false
	}
	n.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// Children returns the notification's immediate semantic children.
func (n *Notification) Children() []facet.GroupChild {
	if n == nil || !n.Open {
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
	bounds := n.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	out := layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
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

// OnAttach is unused.
func (n *Notification) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (n *Notification) OnActivate() {}

// OnDeactivate is unused.
func (n *Notification) OnDeactivate() {}

// OnFocusGained is unused.
func (n *Notification) OnFocusGained() {}

// OnFocusLost is unused.
func (n *Notification) OnFocusLost() {}

// OnDetach clears cached projection state.
func (n *Notification) OnDetach() {
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
	n.Base().Invalidate(flags)
}

func (n *Notification) syncChildren() {
	if n == nil {
		return
	}
	iconSource := strings.TrimSpace(n.IconRef)
	if iconSource == "" {
		iconSource = notificationDefaultIconSVG
	}
	if n.cachedIconFacet == nil {
		n.cachedIconFacet = primitive.NewIcon(primitive.IconSVG(iconSource))
	} else {
		n.cachedIconFacet.SetSource(primitive.IconSVG(iconSource))
	}
	n.cachedIconFacet.SetDecorative(true)
	title := strings.TrimSpace(n.Title)
	if n.cachedTitleFacet == nil {
		n.cachedTitleFacet = primitive.NewText(title)
	} else {
		n.cachedTitleFacet.SetContent(title)
	}
	n.cachedTitleFacet.SetTypography(theme.TextHeadingS)
	n.cachedTitleFacet.SetOverflow(primitive.TextOverflowTruncate)
	n.cachedTitleFacet.SetForeground(theme.ColorText)
	if n.Disabled {
		n.cachedTitleFacet.SetForeground(theme.ColorTextDisabled)
		n.cachedTitleFacet.SetDisabled(true)
	} else {
		n.cachedTitleFacet.SetDisabled(false)
	}
	message := strings.TrimSpace(n.Message)
	if n.cachedMessageFacet == nil {
		n.cachedMessageFacet = primitive.NewText(message)
	} else {
		n.cachedMessageFacet.SetContent(message)
	}
	n.cachedMessageFacet.SetTypography(theme.TextBodyM)
	n.cachedMessageFacet.SetOverflow(primitive.TextOverflowTruncate)
	n.cachedMessageFacet.SetForeground(theme.ColorTextSecondary)
	if n.Disabled {
		n.cachedMessageFacet.SetForeground(theme.ColorTextDisabled)
		n.cachedMessageFacet.SetDisabled(true)
	} else {
		n.cachedMessageFacet.SetDisabled(false)
	}
	if n.cachedContentGroup == nil {
		n.cachedContentGroup = newNotificationContentGroup(n)
	}
	n.cachedContentGroup.syncContent()
	if strings.TrimSpace(n.ActionLabel) == "" {
		n.cachedActionButton = nil
	} else {
		if n.cachedActionButton == nil {
			n.cachedActionButton = action.NewButton(strings.TrimSpace(n.ActionLabel), uiinput.ButtonText)
			n.cachedActionButton.Activated.Subscribe(func(signal.Unit) {
				if n != nil && !n.Disabled && n.Open {
					n.Actioned.Emit(signal.Unit{})
				}
			})
		}
		n.cachedActionButton.SetLabel(strings.TrimSpace(n.ActionLabel))
		n.cachedActionButton.SetVariant(uiinput.ButtonText)
		n.cachedActionButton.SetDisabled(n.Disabled || n.ActionDisabled)
	}
	if strings.TrimSpace(n.CloseButtonLabel) == "" {
		n.cachedCloseButton = nil
	} else {
		if n.cachedCloseButton == nil {
			n.cachedCloseButton = action.NewIconButton(primitive.IconSVG(dialogDefaultCloseSVG))
			n.cachedCloseButton.Activated.Subscribe(func(signal.Unit) {
				if n != nil && !n.Disabled && n.Open {
					n.closeAndDismiss()
				}
			})
		}
		n.cachedCloseButton.SetSource(primitive.IconSVG(dialogDefaultCloseSVG))
		n.cachedCloseButton.SetAccessibleName(strings.TrimSpace(n.CloseButtonLabel))
		n.cachedCloseButton.SetDisabled(n.Disabled)
	}
}

func (n *Notification) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if !n.Open {
		size := constraints.Constrain(gfx.Size{})
		n.layoutRole.MeasuredSize = size
		n.layoutRole.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return n.layoutRole.MeasuredResult
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
	n.cachedPadX = maxFloat(resolved.Density.Scale(14), float32(resolved.Spacing(theme.SpacingL)))
	n.cachedPadY = maxFloat(resolved.Density.Scale(12), float32(resolved.Spacing(theme.SpacingM)))
	n.cachedGap = maxFloat(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	n.cachedRowGap = maxFloat(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingS)))
	n.cachedSurfaceRadius = maxFloat(float32(resolved.Radius(theme.RadiusL).Float32()), float32(resolved.Radius(theme.RadiusM).Float32()))
	n.syncChildren()
	innerMaxW := constraints.MaxSize.W
	if innerMaxW > 0 {
		innerMaxW = maxFloat(0, innerMaxW-n.cachedPadX*2)
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
		target := maxFloat(resolved.Density.Scale(20), float32(resolved.TokenSet().Spacing.TouchTarget)*0.35)
		iconSize = n.cachedIconFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target, H: target}}).Size
	}
	actionSize := gfx.Size{}
	if n.cachedActionButton != nil {
		target := maxFloat(resolved.Density.Scale(24), float32(resolved.TokenSet().Spacing.TouchTarget)*0.55)
		actionSize = n.cachedActionButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target * 4, H: target}}).Size
	}
	closeSize := gfx.Size{}
	if n.cachedCloseButton != nil {
		target := maxFloat(resolved.Density.Scale(24), float32(resolved.TokenSet().Spacing.TouchTarget)*0.55)
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
		bodyMaxW = maxFloat(0, innerMaxW-controlsW-n.cachedGap*2)
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
	contentH := maxFloat(iconSize.H, bodySize.H)
	contentH = maxFloat(contentH, actionSize.H)
	contentH = maxFloat(contentH, closeSize.H)
	surfaceSize := gfx.Size{
		W: contentW + n.cachedPadX*2,
		H: contentH + n.cachedPadY*2,
	}
	surfaceSize.W = maxFloat(surfaceSize.W, resolved.Density.Scale(280))
	surfaceSize.H = maxFloat(surfaceSize.H, resolved.Density.Scale(72))
	if constraints.MaxSize.W > 0 {
		surfaceSize.W = minFloat(surfaceSize.W, constraints.MaxSize.W)
	}
	if constraints.MaxSize.H > 0 {
		surfaceSize.H = minFloat(surfaceSize.H, constraints.MaxSize.H)
	}
	size := constraints.Constrain(surfaceSize)
	n.layoutRole.MeasuredSize = size
	n.layoutRole.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	return n.layoutRole.MeasuredResult
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
	n.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() || !n.Open {
		return
	}
	n.syncChildren()
	margin := maxFloat(n.cachedPadX, n.cachedPadY)
	contentWidth := float32(0)
	contentHeight := float32(0)
	if n.cachedIconFacet != nil {
		size := n.cachedIconFacet.Base().LayoutRole().MeasuredSize
		contentWidth = maxFloat(contentWidth, size.W)
		contentHeight = maxFloat(contentHeight, size.H)
	}
	if n.cachedContentGroup != nil {
		size := n.cachedContentGroup.Base().LayoutRole().MeasuredSize
		contentWidth = maxFloat(contentWidth, size.W)
		contentHeight = maxFloat(contentHeight, size.H)
	}
	if n.cachedActionButton != nil {
		size := n.cachedActionButton.Base().LayoutRole().MeasuredSize
		contentWidth = maxFloat(contentWidth, size.W)
		contentHeight = maxFloat(contentHeight, size.H)
	}
	if n.cachedCloseButton != nil {
		size := n.cachedCloseButton.Base().LayoutRole().MeasuredSize
		contentWidth = maxFloat(contentWidth, size.W)
		contentHeight = maxFloat(contentHeight, size.H)
	}
	surfaceSize := gfx.Size{
		W: maxFloat(n.cachedPadX*2+contentWidth, 280),
		H: maxFloat(n.cachedPadY*2+contentHeight, 72),
	}
	surfaceSize.W = minFloat(surfaceSize.W, maxFloat(0, bounds.Width()-margin*2))
	surfaceSize.H = minFloat(surfaceSize.H, maxFloat(0, bounds.Height()-margin*2))
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
	rowH := maxFloat(iconSize.H, bodySize.H)
	rowH = maxFloat(rowH, actionSize.H)
	rowH = maxFloat(rowH, closeSize.H)
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
			bodyRect := gfx.RectFromXYWH(content.Min.X, content.Min.Y, maxFloat(0, x-content.Min.X), bodySize.H)
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
			bodyRect := gfx.RectFromXYWH(x, content.Min.Y, maxFloat(0, content.Max.X-x), bodySize.H)
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
	if n == nil || bounds.IsEmpty() || !n.Open {
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
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) && !n.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(n.cachedSurfaceBounds, n.cachedSurfaceRadius), surface)...)
	}
	if !n.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: n.cachedSurfaceBounds})
		if !isTransparentMaterial(icon) && n.cachedIconFacet != nil && !n.cachedIconBounds.IsEmpty() {
			cmds = append(cmds, materialCommands(gfx.RectPath(n.cachedIconBounds), icon)...)
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
			if !isTransparentMaterial(actionSlot) {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(n.cachedActionBounds, n.cachedSurfaceRadius*0.5), actionSlot)...)
			}
			if projected := n.cachedActionButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedActionBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if n.cachedCloseButton != nil && !n.cachedCloseBounds.IsEmpty() {
			if !isTransparentMaterial(closeSlot) {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(n.cachedCloseBounds, n.cachedSurfaceRadius*0.5), closeSlot)...)
			}
			if projected := n.cachedCloseButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: n.cachedCloseBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	if !isTransparentMaterial(title) && !n.cachedTitleBounds.IsEmpty() {
		_ = title
	}
	if !isTransparentMaterial(message) && !n.cachedMessageBounds.IsEmpty() {
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
	if n.Disabled {
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
	if n.Disabled {
		return uifeedback.NotificationDisabled
	}
	if n.pressed {
		return uifeedback.NotificationActive
	}
	if n.hovered {
		return uifeedback.NotificationHover
	}
	if n.Open {
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
	if n == nil || !n.Open || n.cachedBounds.IsEmpty() || !n.cachedBounds.Contains(p) {
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
	if n == nil || n.Disabled || !n.Open {
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

func (n *Notification) onDismiss(e facet.DismissEvent) bool {
	_ = e
	return false
}

func (n *Notification) closeAndDismiss() {
	if n == nil || !n.Open {
		return
	}
	n.Open = false
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
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	textRole       facet.TextRole

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
		Facet:  facet.NewFacet(),
		parent: parent,
	}
	g.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   notificationContentGroupPolicy{group: g},
		Children: g,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	g.layoutRole.Child = facet.GroupChildContract{
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
	g.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		size := g.measure(ctx, constraints)
		return facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	}
	g.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		g.layoutRole.ArrangedBounds = bounds
		g.arrange(ctx, bounds)
	}
	g.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := g.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	g.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := g.buildCommands(g.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	g.textRole.IMEEnabled = false
	g.AddRole(&g.layoutRole)
	g.AddRole(&g.renderRole)
	g.AddRole(&g.projectionRole)
	g.AddRole(&g.textRole)
	return g
}

func (g *notificationContentGroup) Base() *facet.Facet {
	g.Facet.BindImpl(g)
	return &g.Facet
}

func (g *notificationContentGroup) Children() []facet.GroupChild {
	if g == nil || g.parent == nil || !g.parent.Open {
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

func (g *notificationContentGroup) OnAttach(ctx facet.AttachContext) {}
func (g *notificationContentGroup) OnActivate()                      {}
func (g *notificationContentGroup) OnDeactivate()                    {}
func (g *notificationContentGroup) OnDetach()                        {}

func (g *notificationContentGroup) syncContent() {
	if g == nil || g.parent == nil {
		return
	}
	g.cachedLayoutMode = g.parent.ContentLayoutMode
	g.cachedGridColumns = g.parent.ContentGridColumns
	g.cachedGridRows = g.parent.ContentGridRows
	if g.cachedGridColumns < 1 {
		g.cachedGridColumns = 1
	}
	if g.cachedGridRows < 1 {
		g.cachedGridRows = 1
	}
	if g.layoutRole.Parent.Policy != nil {
		g.layoutRole.Parent.Kind = g.groupKind()
	}
	title := strings.TrimSpace(g.parent.Title)
	if g.cachedTitleFacet == nil {
		g.cachedTitleFacet = primitive.NewText(title)
	} else {
		g.cachedTitleFacet.SetContent(title)
	}
	g.cachedTitleFacet.SetTypography(theme.TextHeadingS)
	g.cachedTitleFacet.SetOverflow(primitive.TextOverflowTruncate)
	g.cachedTitleFacet.SetForeground(theme.ColorText)
	if g.parent.Disabled {
		g.cachedTitleFacet.SetForeground(theme.ColorTextDisabled)
		g.cachedTitleFacet.SetDisabled(true)
	} else {
		g.cachedTitleFacet.SetDisabled(false)
	}
	message := strings.TrimSpace(g.parent.Message)
	if g.cachedMessageFacet == nil {
		g.cachedMessageFacet = primitive.NewText(message)
	} else {
		g.cachedMessageFacet.SetContent(message)
	}
	g.cachedMessageFacet.SetTypography(theme.TextBodyM)
	g.cachedMessageFacet.SetOverflow(primitive.TextOverflowTruncate)
	g.cachedMessageFacet.SetForeground(theme.ColorTextSecondary)
	if g.parent.Disabled {
		g.cachedMessageFacet.SetForeground(theme.ColorTextDisabled)
		g.cachedMessageFacet.SetDisabled(true)
	} else {
		g.cachedMessageFacet.SetDisabled(false)
	}
	g.cachedChildren = append(g.cachedChildren[:0], g.parent.ContentChildren...)
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
	if g == nil || g.parent == nil || !g.parent.Open {
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
			height = maxFloat(height, children[i].size.H)
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
			width = maxFloat(width, children[i].size.W)
			height += children[i].size.H
		}
		return gfx.Size{W: width, H: height}
	}
}

func (g *notificationContentGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open {
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
			rect := gfx.RectFromXYWH(x, bounds.Min.Y, children[i].size.W, maxFloat(bounds.Height(), children[i].size.H))
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
		out = append(out, layoutgrid.Child{
			FacetID: child.facet.ID(),
			Attachment: facet.Attachment{
				Placement: facet.Placement{
					Mode: facet.PlacementGrid,
					Grid: child.grid,
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
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open {
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
