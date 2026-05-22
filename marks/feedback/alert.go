package feedback

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxmaterial "codeburg.org/lexbit/lurpicui/gfx/material"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/action"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	alertMarkIDRoot        facet.MarkID = 1
	alertMarkIDSurface     facet.MarkID = 2
	alertMarkIDIcon        facet.MarkID = 3
	alertMarkIDTitle       facet.MarkID = 4
	alertMarkIDMessage     facet.MarkID = 5
	alertMarkIDAction      facet.MarkID = 6
	alertMarkIDCloseButton facet.MarkID = 7
)

// Alert implements the feedback.alert canonical mark.
type Alert struct {
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
	ActionIconRef      string
	CloseButtonLabel   string
	CloseButtonIconRef string
	Disabled           bool

	hovered        bool
	pressed        bool
	surfacePressed bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.FeedbackAlertSlots
	cachedBounds           gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedIconBounds       gfx.Rect
	cachedTitleBounds      gfx.Rect
	cachedMessageBounds    gfx.Rect
	cachedActionBounds     gfx.Rect
	cachedCloseBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRowGap           float32
	cachedIconSize         float32
	cachedCloseSize        float32
	cachedWritingDirection facet.WritingDirection
	cachedDensity          theme.DensityID

	cachedIconFacet    *primitive.Icon
	cachedTitleFacet   *primitive.Text
	cachedMessageFacet *primitive.Text
	cachedActionButton *action.Button
	cachedCloseButton  *action.IconButton
}

var _ facet.FacetImpl = (*Alert)(nil)
var _ layout.AnchorExporter = (*Alert)(nil)

const (
	alertDefaultIconSVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2.6 20h18.8L12 3z"/><path d="M12 8v5"/><circle cx="12" cy="16.5" r="1"/></svg>`
	alertDefaultCloseSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12"/><path d="M18 6 6 18"/></svg>`
)

// NewAlert constructs a feedback.alert mark with canonical defaults.
func NewAlert(title, message string) *Alert {
	a := &Alert{
		Facet:   facet.NewFacet(),
		Title:   title,
		Message: message,
	}
	a.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   alertGroupPolicy{alert: a},
		Children: a,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	a.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := a.measure(ctx, constraints).Size
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
			Height: facet.StretchNever,
		},
		Baseline: facet.BaselineNone,
	}
	a.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return a.measure(ctx, constraints)
	}
	a.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		a.layoutRole.ArrangedBounds = bounds
		a.arrange(ctx, bounds)
	}
	a.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := a.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	a.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := a.buildCommands(a.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	a.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return a.hitTest(p) }
	a.inputRole.OnPointer = func(e facet.PointerEvent) bool { return a.onPointer(e) }
	a.inputRole.OnKey = func(e facet.KeyEvent) bool { return a.onKey(e) }
	a.textRole.IMEEnabled = false
	a.AddRole(&a.layoutRole)
	a.AddRole(&a.renderRole)
	a.AddRole(&a.projectionRole)
	a.AddRole(&a.hitRole)
	a.AddRole(&a.inputRole)
	a.AddRole(&a.textRole)
	a.syncChildren()
	return a
}

// Base satisfies facet.FacetImpl.
func (a *Alert) Base() *facet.Facet {
	a.Facet.BindImpl(a)
	return &a.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (a *Alert) AccessibilityRole() string { return "alert" }

// AccessibleName reports the semantic name source required by the spec.
func (a *Alert) AccessibleName() string {
	if a == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(a.Title), strings.TrimSpace(a.Message)}
	out := strings.TrimSpace(strings.Join(parts, " "))
	if out != "" {
		return out
	}
	if strings.TrimSpace(a.ActionLabel) != "" {
		return strings.TrimSpace(a.ActionLabel)
	}
	return strings.TrimSpace(a.CloseButtonLabel)
}

// SetTitle updates the authored title text.
func (a *Alert) SetTitle(title string) {
	if a == nil || a.Title == title {
		return
	}
	a.Title = title
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetMessage updates the authored message text.
func (a *Alert) SetMessage(message string) {
	if a == nil || a.Message == message {
		return
	}
	a.Message = message
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetIconRef updates the authored icon source.
func (a *Alert) SetIconRef(ref string) {
	if a == nil || a.IconRef == ref {
		return
	}
	a.IconRef = ref
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetActionLabel updates the authored action label.
func (a *Alert) SetActionLabel(label string) {
	if a == nil || a.ActionLabel == label {
		return
	}
	a.ActionLabel = label
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetActionIconRef updates the authored action icon source.
func (a *Alert) SetActionIconRef(ref string) {
	if a == nil || a.ActionIconRef == ref {
		return
	}
	a.ActionIconRef = ref
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetCloseButtonLabel updates the authored close-button label.
func (a *Alert) SetCloseButtonLabel(label string) {
	if a == nil || a.CloseButtonLabel == label {
		return
	}
	a.CloseButtonLabel = label
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetCloseButtonIconRef updates the authored close-button icon source.
func (a *Alert) SetCloseButtonIconRef(ref string) {
	if a == nil || a.CloseButtonIconRef == ref {
		return
	}
	a.CloseButtonIconRef = ref
	a.syncChildren()
	a.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles the disabled state.
func (a *Alert) SetDisabled(disabled bool) {
	if a == nil || a.Disabled == disabled {
		return
	}
	a.Disabled = disabled
	if disabled {
		a.hovered = false
		a.pressed = false
		a.surfacePressed = false
	}
	a.syncChildren()
	a.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// Children returns the alert's immediate child list.
func (a *Alert) Children() []facet.GroupChild {
	if a == nil {
		return nil
	}
	a.syncChildren()
	out := make([]facet.GroupChild, 0, 5)
	if a.cachedIconFacet != nil {
		out = append(out, a.alertGroupChild(a.cachedIconFacet.Base(), alertMarkIDIcon, 0))
	}
	if a.cachedTitleFacet != nil {
		order := 1
		if a.cachedIconFacet == nil {
			order = 0
		}
		out = append(out, a.alertGroupChild(a.cachedTitleFacet.Base(), alertMarkIDTitle, order))
	}
	if a.cachedMessageFacet != nil {
		order := 2
		if a.cachedIconFacet == nil {
			order = 1
		}
		if a.cachedTitleFacet == nil {
			order--
		}
		out = append(out, a.alertGroupChild(a.cachedMessageFacet.Base(), alertMarkIDMessage, order))
	}
	if a.cachedActionButton != nil {
		out = append(out, a.alertGroupChild(a.cachedActionButton.Base(), alertMarkIDAction, 3))
	}
	if a.cachedCloseButton != nil {
		out = append(out, a.alertGroupChild(a.cachedCloseButton.Base(), alertMarkIDCloseButton, 4))
	}
	return out
}

// ExportAnchors publishes the alert anchor set.
func (a *Alert) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if a == nil {
		return nil
	}
	bounds := a.layoutRole.ArrangedBounds
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
	if !a.cachedTitleBounds.IsEmpty() {
		out["title"] = gfx.Point{X: (a.cachedTitleBounds.Min.X + a.cachedTitleBounds.Max.X) * 0.5, Y: a.cachedTitleBounds.Min.Y}
	}
	if !a.cachedMessageBounds.IsEmpty() {
		out["message"] = gfx.Point{X: (a.cachedMessageBounds.Min.X + a.cachedMessageBounds.Max.X) * 0.5, Y: a.cachedMessageBounds.Min.Y}
	}
	if !a.cachedIconBounds.IsEmpty() {
		out["icon"] = gfx.Point{X: (a.cachedIconBounds.Min.X + a.cachedIconBounds.Max.X) * 0.5, Y: (a.cachedIconBounds.Min.Y + a.cachedIconBounds.Max.Y) * 0.5}
	}
	if !a.cachedActionBounds.IsEmpty() {
		out["action"] = gfx.Point{X: (a.cachedActionBounds.Min.X + a.cachedActionBounds.Max.X) * 0.5, Y: (a.cachedActionBounds.Min.Y + a.cachedActionBounds.Max.Y) * 0.5}
	}
	if !a.cachedCloseBounds.IsEmpty() {
		out["close_button"] = gfx.Point{X: (a.cachedCloseBounds.Min.X + a.cachedCloseBounds.Max.X) * 0.5, Y: (a.cachedCloseBounds.Min.Y + a.cachedCloseBounds.Max.Y) * 0.5}
	}
	if !a.cachedSurfaceBounds.IsEmpty() {
		out["alert_surface"] = gfx.Point{X: (a.cachedSurfaceBounds.Min.X + a.cachedSurfaceBounds.Max.X) * 0.5, Y: (a.cachedSurfaceBounds.Min.Y + a.cachedSurfaceBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach is unused.
func (a *Alert) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (a *Alert) OnActivate() {}

// OnDeactivate is unused.
func (a *Alert) OnDeactivate() {}

// OnDetach clears cached projection state.
func (a *Alert) OnDetach() {
	a.cachedTokens = theme.Tokens{}
	a.cachedRecipe = shared.FeedbackAlertSlots{}
	a.cachedBounds = gfx.Rect{}
	a.cachedSurfaceBounds = gfx.Rect{}
	a.cachedIconBounds = gfx.Rect{}
	a.cachedTitleBounds = gfx.Rect{}
	a.cachedMessageBounds = gfx.Rect{}
	a.cachedActionBounds = gfx.Rect{}
	a.cachedCloseBounds = gfx.Rect{}
	a.cachedPadX = 0
	a.cachedPadY = 0
	a.cachedGap = 0
	a.cachedRowGap = 0
	a.cachedIconSize = 0
	a.cachedCloseSize = 0
	a.cachedWritingDirection = facet.WritingDirectionLTR
	a.cachedDensity = ""
	a.cachedIconFacet = nil
	a.cachedTitleFacet = nil
	a.cachedMessageFacet = nil
	a.cachedActionButton = nil
	a.cachedCloseButton = nil
}

func (a *Alert) invalidate(flags facet.DirtyFlags) {
	if a == nil {
		return
	}
	a.Base().Invalidate(flags)
}

func (a *Alert) syncChildren() {
	if a == nil {
		return
	}
	iconRef := strings.TrimSpace(a.IconRef)
	if iconRef == "" {
		iconRef = alertDefaultIconSVG
	}
	if a.cachedIconFacet == nil {
		a.cachedIconFacet = primitive.NewIcon(primitive.IconSVG(iconRef))
	} else {
		a.cachedIconFacet.SetSource(primitive.IconSVG(iconRef))
	}
	a.cachedIconFacet.SetDecorative(true)
	a.cachedIconFacet.SetColorSlot(theme.ColorPrimary)
	if a.Disabled {
		a.cachedIconFacet.SetColorSlot(theme.ColorTextDisabled)
	}

	if a.cachedTitleFacet == nil {
		a.cachedTitleFacet = primitive.NewText(strings.TrimSpace(a.Title))
	} else {
		a.cachedTitleFacet.SetContent(strings.TrimSpace(a.Title))
	}
	a.cachedTitleFacet.SetTypography(theme.TextHeadingS)
	a.cachedTitleFacet.SetOverflow(primitive.TextOverflowTruncate)
	a.cachedTitleFacet.SetForeground(theme.ColorText)
	if a.cachedDensity == theme.DensityIDCompact {
		a.cachedTitleFacet.SetTypography(theme.TextLabelM)
	}
	if a.Disabled {
		a.cachedTitleFacet.SetForeground(theme.ColorTextDisabled)
		a.cachedTitleFacet.SetDisabled(true)
	} else {
		a.cachedTitleFacet.SetDisabled(false)
	}

	if a.cachedMessageFacet == nil {
		a.cachedMessageFacet = primitive.NewText(strings.TrimSpace(a.Message))
	} else {
		a.cachedMessageFacet.SetContent(strings.TrimSpace(a.Message))
	}
	a.cachedMessageFacet.SetTypography(theme.TextBodyM)
	a.cachedMessageFacet.SetOverflow(primitive.TextOverflowTruncate)
	a.cachedMessageFacet.SetForeground(theme.ColorTextSecondary)
	if a.cachedDensity == theme.DensityIDCompact {
		a.cachedMessageFacet.SetTypography(theme.TextBodyS)
	}
	if a.Disabled {
		a.cachedMessageFacet.SetForeground(theme.ColorTextDisabled)
		a.cachedMessageFacet.SetDisabled(true)
	} else {
		a.cachedMessageFacet.SetDisabled(false)
	}

	actionLabel := strings.TrimSpace(a.ActionLabel)
	if actionLabel == "" {
		a.cachedActionButton = nil
	} else {
		if a.cachedActionButton == nil {
			a.cachedActionButton = action.NewButton(actionLabel, uiinput.ButtonText)
			a.cachedActionButton.Activated.Subscribe(func(signal.Unit) {
				if a != nil {
					a.Actioned.Emit(signal.Unit{})
				}
			})
		} else {
			a.cachedActionButton.SetLabel(actionLabel)
			a.cachedActionButton.SetVariant(uiinput.ButtonText)
		}
		if iconRef := strings.TrimSpace(a.ActionIconRef); iconRef != "" {
			a.cachedActionButton.SetLeadingIconRef(iconRef)
		} else {
			a.cachedActionButton.SetLeadingIconRef("")
		}
		a.cachedActionButton.SetDisabled(a.Disabled)
	}

	closeLabel := strings.TrimSpace(a.CloseButtonLabel)
	if closeLabel == "" {
		a.cachedCloseButton = nil
	} else {
		if a.cachedCloseButton == nil {
			a.cachedCloseButton = action.NewIconButton(primitive.IconSVG(alertDefaultCloseSVG))
			a.cachedCloseButton.Activated.Subscribe(func(signal.Unit) {
				if a != nil {
					a.Dismissed.Emit(signal.Unit{})
				}
			})
		}
		if iconRef := strings.TrimSpace(a.CloseButtonIconRef); iconRef != "" {
			a.cachedCloseButton.SetSource(primitive.IconSVG(iconRef))
		} else {
			a.cachedCloseButton.SetSource(primitive.IconSVG(alertDefaultCloseSVG))
		}
		a.cachedCloseButton.SetAccessibleName(closeLabel)
		a.cachedCloseButton.SetDisabled(a.Disabled)
	}
}

func (a *Alert) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uifeedback.ResolveAlertRecipe(style, a.alertVariant())
	a.cachedTokens = resolved.TokenSet()
	a.cachedRecipe = slots
	a.cachedWritingDirection = ctx.WritingDirection
	a.cachedDensity = resolved.Density.ID
	a.cachedPadX = maxFloat(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingM)))
	a.cachedPadY = maxFloat(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	a.cachedGap = maxFloat(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingXS)))
	a.cachedRowGap = maxFloat(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	a.cachedIconSize = maxFloat(resolved.Density.Scale(20), float32(resolved.Spacing(theme.SpacingM)))
	a.cachedCloseSize = maxFloat(resolved.Density.Scale(24), float32(resolved.Spacing(theme.SpacingM))*1.2)
	a.syncChildren()
	children := a.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{})
		a.layoutRole.MeasuredSize = size
		a.layoutRole.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		return a.layoutRole.MeasuredResult
	}
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(480)
	}
	innerWidth := maxFloat(0, maxWidth-a.cachedPadX*2)
	measureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	innerHeight := float32(0)
	contentWidth := float32(0)
	titleWidth := innerWidth
	if a.cachedIconFacet != nil {
		titleWidth -= a.cachedIconSize + a.cachedGap
	}
	if a.cachedCloseButton != nil {
		titleWidth -= a.cachedCloseSize + a.cachedGap
	}
	if titleWidth < 0 {
		titleWidth = 0
	}
	if a.cachedIconFacet != nil {
		a.cachedIconFacet.SetSize(a.cachedIconSize)
		iconSize := a.cachedIconFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: a.cachedIconSize, H: a.cachedIconSize}}).Size
		contentWidth = maxFloat(contentWidth, iconSize.W)
	}
	titleSize := a.cachedTitleFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: titleWidth, H: constraints.MaxSize.H}}).Size
	messageSize := a.cachedMessageFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: innerWidth, H: constraints.MaxSize.H}}).Size
	contentWidth = maxFloat(contentWidth, titleSize.W+a.cachedGap)
	if a.cachedCloseButton != nil {
		a.cachedCloseButton.SetSize(a.cachedCloseSize)
		a.cachedCloseButton.SetHitPadding(maxFloat(resolved.Density.Scale(4), float32(resolved.Spacing(theme.SpacingXS))))
		closeSize := a.cachedCloseButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: a.cachedCloseSize, H: a.cachedCloseSize}}).Size
		contentWidth = maxFloat(contentWidth, closeSize.W)
		if closeSize.H > innerHeight {
			innerHeight = closeSize.H
		}
	}
	if titleSize.H > innerHeight {
		innerHeight = titleSize.H
	}
	if messageSize.H > 0 {
		innerHeight += a.cachedRowGap + messageSize.H
	} else {
		innerHeight += messageSize.H
	}
	if a.cachedActionButton != nil {
		actionSize := a.cachedActionButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: innerWidth, H: constraints.MaxSize.H}}).Size
		if innerHeight > 0 {
			innerHeight += a.cachedRowGap
		}
		innerHeight += actionSize.H
		if actionSize.W > contentWidth {
			contentWidth = actionSize.W
		}
	}
	contentWidth = maxFloat(contentWidth, titleSize.W)
	contentWidth = maxFloat(contentWidth, messageSize.W)
	size := gfx.Size{W: contentWidth + a.cachedPadX*2, H: innerHeight + a.cachedPadY*2}
	if size.W < a.cachedIconSize+a.cachedPadX*2 {
		size.W = a.cachedIconSize + a.cachedPadX*2
	}
	measured := constraints.Constrain(size)
	a.layoutRole.MeasuredSize = measured
	a.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return a.layoutRole.MeasuredResult
}

func (a *Alert) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	a.cachedBounds = bounds
	a.cachedSurfaceBounds = bounds.Inset(0, 0)
	a.cachedIconBounds = gfx.Rect{}
	a.cachedTitleBounds = gfx.Rect{}
	a.cachedMessageBounds = gfx.Rect{}
	a.cachedActionBounds = gfx.Rect{}
	a.cachedCloseBounds = gfx.Rect{}
	a.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	a.syncChildren()
	inner := bounds.Inset(a.cachedPadX, a.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	titleSize := a.cachedTitleFacet.Base().LayoutRole().MeasuredSize
	messageSize := a.cachedMessageFacet.Base().LayoutRole().MeasuredSize
	actionSize := gfx.Size{}
	if a.cachedActionButton != nil {
		actionSize = a.cachedActionButton.Base().LayoutRole().MeasuredSize
	}
	headerH := titleSize.H
	if a.cachedIconFacet != nil || a.cachedCloseButton != nil {
		iconSize := a.cachedIconSize
		if a.cachedIconFacet != nil {
			iconSize = a.cachedIconFacet.Base().LayoutRole().MeasuredSize.W
		}
		closeSize := a.cachedCloseSize
		if a.cachedCloseButton != nil {
			closeSize = a.cachedCloseButton.Base().LayoutRole().MeasuredSize.W
		}
		headerH = maxFloat(headerH, iconSize)
		headerH = maxFloat(headerH, closeSize)
	}
	rowRects := layout.ArrangeVerticalFlowAligned(inner, 0, a.cachedRowGap, []gfx.Size{
		{W: inner.Width(), H: headerH},
		{W: inner.Width(), H: messageSize.H},
		{W: inner.Width(), H: actionSize.H},
	}, a.cachedWritingDirection == facet.WritingDirectionRTL, layout.AlignStart)
	headerRect := rowRects[0]
	messageRect := rowRects[1]
	actionRect := rowRects[2]
	if a.cachedIconFacet != nil || a.cachedCloseButton != nil {
		iconSize := a.cachedIconSize
		if a.cachedIconFacet != nil {
			iconSize = a.cachedIconFacet.Base().LayoutRole().MeasuredSize.W
		}
		closeSize := a.cachedCloseSize
		if a.cachedCloseButton != nil {
			closeSize = a.cachedCloseButton.Base().LayoutRole().MeasuredSize.W
		}
		iconX := headerRect.Min.X
		closeX := headerRect.Max.X - closeSize
		titleX := headerRect.Min.X
		if a.cachedWritingDirection == facet.WritingDirectionRTL {
			iconX = headerRect.Max.X - iconSize
			closeX = headerRect.Min.X
		}
		if a.cachedIconFacet != nil {
			iconRect := gfx.RectFromXYWH(iconX, headerRect.Min.Y+(headerRect.Height()-iconSize)*0.5, iconSize, iconSize)
			a.cachedIconFacet.Base().LayoutRole().Arrange(ctx, iconRect)
			a.cachedIconBounds = iconRect
			if a.cachedWritingDirection == facet.WritingDirectionRTL {
				titleX = headerRect.Min.X + closeSize + a.cachedGap
			} else {
				titleX = headerRect.Min.X + iconSize + a.cachedGap
			}
		}
		if a.cachedCloseButton != nil {
			closeRect := gfx.RectFromXYWH(closeX, headerRect.Min.Y+(headerRect.Height()-closeSize)*0.5, closeSize, closeSize)
			a.cachedCloseButton.Base().LayoutRole().Arrange(ctx, closeRect)
			a.cachedCloseBounds = closeRect
			if a.cachedWritingDirection == facet.WritingDirectionRTL {
				titleX = closeRect.Max.X + a.cachedGap
			}
		}
		titleW := maxFloat(0, inner.Max.X-titleX)
		if a.cachedCloseButton != nil && a.cachedWritingDirection != facet.WritingDirectionRTL {
			titleW = maxFloat(0, closeX-a.cachedGap-titleX)
		}
		titleRect := gfx.RectFromXYWH(titleX, headerRect.Min.Y, titleW, titleSize.H)
		a.cachedTitleFacet.Base().LayoutRole().Arrange(ctx, titleRect)
		a.cachedTitleBounds = titleRect
	}
	if messageSize.W > 0 || messageSize.H > 0 {
		a.cachedMessageFacet.Base().LayoutRole().Arrange(ctx, messageRect)
		a.cachedMessageBounds = messageRect
	}
	if a.cachedActionButton != nil {
		actionRect = gfx.RectFromXYWH(actionRect.Min.X, actionRect.Min.Y, actionSize.W, actionSize.H)
		if a.cachedWritingDirection == facet.WritingDirectionRTL {
			actionRect.Min.X = inner.Min.X
			actionRect.Max.X = actionRect.Min.X + actionSize.W
		} else {
			actionRect.Min.X = inner.Max.X - actionSize.W
			actionRect.Max.X = inner.Max.X
		}
		a.cachedActionButton.Base().LayoutRole().Arrange(ctx, actionRect)
		a.cachedActionBounds = actionRect
	}
}

func (a *Alert) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if a == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := a.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if a.Disabled {
		state = theme.StateDisabled
	} else if a.pressed {
		state = theme.StatePressed
	} else if a.hovered {
		state = theme.StateHover
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.AlertSurface.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 64)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(bounds, float32(tokens.Radius.LG)), surface)...)
	}
	clipBounds := bounds
	cmds = append(cmds, gfx.PushClipRect{Rect: clipBounds})
	if a.cachedIconFacet != nil && !a.cachedIconBounds.IsEmpty() {
		if projected := a.cachedIconFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       a.cachedIconBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	if a.cachedTitleFacet != nil && !a.cachedTitleBounds.IsEmpty() {
		if projected := a.cachedTitleFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       a.cachedTitleBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	if a.cachedMessageFacet != nil && !a.cachedMessageBounds.IsEmpty() {
		if projected := a.cachedMessageFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       a.cachedMessageBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	if a.cachedActionButton != nil && !a.cachedActionBounds.IsEmpty() {
		if projected := a.cachedActionButton.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       a.cachedActionBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	if a.cachedCloseButton != nil && !a.cachedCloseBounds.IsEmpty() {
		if projected := a.cachedCloseButton.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       a.cachedCloseBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	cmds = append(cmds, gfx.PopClip{})
	return cmds
}

func (a *Alert) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.FeedbackAlertSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: a.cachedTokens}, a.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, a.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uifeedback.ResolveAlertRecipe(style, a.alertVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: a.cachedTokens}, a.cachedRecipe
}

func (a *Alert) alertVariant() uifeedback.AlertVariant {
	if a != nil && a.Disabled {
		return uifeedback.AlertDisabled
	}
	if a != nil && a.pressed {
		return uifeedback.AlertActive
	}
	if a != nil && a.hovered {
		return uifeedback.AlertHover
	}
	return uifeedback.AlertDefault
}

func (a *Alert) hitTest(p gfx.Point) facet.HitResult {
	if a == nil || a.cachedBounds.IsEmpty() || !a.cachedBounds.Contains(p) {
		return facet.HitResult{}
	}
	switch {
	case !a.cachedCloseBounds.IsEmpty() && a.cachedCloseBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDCloseButton}
	case !a.cachedActionBounds.IsEmpty() && a.cachedActionBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDAction}
	case !a.cachedIconBounds.IsEmpty() && a.cachedIconBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDIcon}
	case !a.cachedTitleBounds.IsEmpty() && a.cachedTitleBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDTitle}
	case !a.cachedMessageBounds.IsEmpty() && a.cachedMessageBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDMessage}
	case !a.cachedSurfaceBounds.IsEmpty() && a.cachedSurfaceBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: alertMarkIDSurface}
	default:
		return facet.HitResult{Hit: true, MarkID: alertMarkIDRoot}
	}
}

func (a *Alert) onPointer(e facet.PointerEvent) bool {
	if a == nil || a.Disabled {
		return false
	}
	if !a.cachedBounds.Contains(e.Position) {
		if e.Kind == platform.PointerLeave {
			a.hovered = false
			a.pressed = false
			a.surfacePressed = false
			a.invalidate(facet.DirtyProjection)
		}
		return false
	}
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if !a.hovered {
			a.hovered = true
			a.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			a.pressed = true
			a.surfacePressed = true
			a.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerRelease:
		if a.pressed {
			a.pressed = false
			a.surfacePressed = false
			a.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerLeave:
		a.hovered = false
		a.pressed = false
		a.surfacePressed = false
		a.invalidate(facet.DirtyProjection)
		return true
	}
	return false
}

func (a *Alert) onKey(e facet.KeyEvent) bool {
	if a == nil || a.Disabled {
		return false
	}
	if e.Kind == platform.KeyPress && e.Key == platform.KeyEscape {
		if strings.TrimSpace(a.CloseButtonLabel) != "" {
			a.Dismissed.Emit(signal.Unit{})
			return true
		}
	}
	return false
}

func (a *Alert) alertGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  markID,
		Attachment: facet.Attachment{
			Placement: facet.Placement{
				Mode: facet.PlacementLinear,
				Linear: facet.LinearPlacement{
					Order:          order,
					CrossAxisAlign: facet.CrossAxisStart,
				},
			},
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

type alertGroupPolicy struct {
	alert *Alert
}

func (alertGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p alertGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.alert == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.alert.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size}, nil
}

func (p alertGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.alert == nil {
		return nil, nil
	}
	p.alert.arrange(ctx.ArrangeContext, ctx.Bounds)
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

func runtimeServicesOrNil(runtime any) facet.RuntimeServices {
	if runtime == nil {
		return nil
	}
	if services, ok := runtime.(facet.RuntimeServices); ok {
		return services
	}
	return nil
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.Transparent(material)
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return gfxmaterial.Commands(path, material)
}
