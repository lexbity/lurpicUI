package feedback

import (
	"reflect"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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
	marks.Core

	textRole facet.TextRole

	Actioned  signal.Signal[signal.Unit]
	Dismissed signal.Signal[signal.Unit]

	Title              marks.Binding[string]
	Message            marks.Binding[string]
	IconRef            marks.Binding[string]
	ActionLabel        marks.Binding[string]
	ActionIconRef      marks.Binding[string]
	CloseButtonLabel   marks.Binding[string]
	CloseButtonIconRef marks.Binding[string]
	Disabled           marks.Binding[bool]

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
var _ marks.Mark = (*Alert)(nil)

const (
	alertDefaultIconSVG  = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="currentColor" stroke="currentColor" stroke-width="1.8" stroke-linecap="round" stroke-linejoin="round"><path d="M12 3 2.6 20h18.8L12 3z"/><path d="M12 8v5"/><circle cx="12" cy="16.5" r="1"/></svg>`
	alertDefaultCloseSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12"/><path d="M18 6 6 18"/></svg>`
)

// NewAlert constructs a feedback.alert mark with canonical defaults.
func NewAlert(title, message string) *Alert {
	a := &Alert{
		Title:              marks.Const(title),
		Message:            marks.Const(message),
		IconRef:            marks.Const(""),
		ActionLabel:        marks.Const(""),
		ActionIconRef:      marks.Const(""),
		CloseButtonLabel:   marks.Const(""),
		CloseButtonIconRef: marks.Const(""),
		Disabled:           marks.Const(false),
	}
	a.Facet = facet.NewFacet()
	a.AddBinding(a.Title)
	a.AddBinding(a.Message)
	a.AddBinding(a.IconRef)
	a.AddBinding(a.ActionLabel)
	a.AddBinding(a.ActionIconRef)
	a.AddBinding(a.CloseButtonLabel)
	a.AddBinding(a.CloseButtonIconRef)
	a.AddBinding(a.Disabled)

	a.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   alertGroupPolicy{alert: a},
		Children: a,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	a.Layout.Child = facet.GroupChildContract{
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
	a.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return a.measure(ctx, constraints)
	}
	a.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		a.Layout.ArrangedBounds = bounds
		a.arrange(ctx, bounds)
	}
	a.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := a.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	a.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return a.hitTest(p) }
	a.Input.OnPointer = func(e facet.PointerEvent) bool { return a.onPointer(e) }
	a.Input.OnKey = func(e facet.KeyEvent) bool { return a.onKey(e) }
	a.textRole.IMEEnabled = false
	a.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return a.buildCommands(a.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	a.RegisterRoles()
	a.AddRole(&a.textRole)
	a.syncChildren()
	return a
}

// Base satisfies facet.FacetImpl.
func (a *Alert) Base() *facet.Facet {
	a.BindImpl(a)
	return &a.Facet
}

// Descriptor satisfies marks.Mark.
func (a *Alert) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "feedback", TypeName: "alert"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (a *Alert) AccessibilityRole() string { return "alert" }

// AccessibleName reports the semantic name source required by the spec.
func (a *Alert) AccessibleName() string {
	if a == nil {
		return ""
	}
	parts := []string{strings.TrimSpace(a.Title.Get()), strings.TrimSpace(a.Message.Get())}
	out := strings.TrimSpace(strings.Join(parts, " "))
	if out != "" {
		return out
	}
	if strings.TrimSpace(a.ActionLabel.Get()) != "" {
		return strings.TrimSpace(a.ActionLabel.Get())
	}
	return strings.TrimSpace(a.CloseButtonLabel.Get())
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
	bounds := a.Layout.ArrangedBounds
	out := a.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
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

// OnAttach delegates to Core.
func (a *Alert) OnAttach(ctx facet.AttachContext) { a.Core.OnAttach() }

// OnActivate delegates to Core.
func (a *Alert) OnActivate() { a.Core.OnActivate() }

// OnDeactivate delegates to Core.
func (a *Alert) OnDeactivate() { a.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (a *Alert) OnDetach() {
	a.Core.OnDetach()
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
	a.Invalidate(flags)
}

func (a *Alert) syncChildren() {
	if a == nil {
		return
	}
	iconRef := strings.TrimSpace(a.IconRef.Get())
	if iconRef == "" {
		iconRef = alertDefaultIconSVG
	}
	if a.cachedIconFacet == nil {
		a.cachedIconFacet = primitive.NewIcon(primitive.IconSVG(iconRef))
	} else {
		a.cachedIconFacet.Source = primitive.IconSVG(iconRef)
	}
	a.cachedIconFacet.Decorative = marks.Const(true)
	a.cachedIconFacet.ColorSlot = marks.Const(theme.ColorPrimary)
	if a.Disabled.Get() {
		a.cachedIconFacet.ColorSlot = marks.Const(theme.ColorTextDisabled)
	}

	if a.cachedTitleFacet == nil {
		a.cachedTitleFacet = primitive.NewText(marks.Const(strings.TrimSpace(a.Title.Get())))
	} else {
		a.cachedTitleFacet.Content = marks.Const(strings.TrimSpace(a.Title.Get()))
		a.cachedTitleFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	a.cachedTitleFacet.Typography = marks.Const(theme.TextHeadingS)
	a.cachedTitleFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	a.cachedTitleFacet.Foreground = marks.Const(theme.ColorText)
	if a.cachedDensity == theme.DensityIDCompact {
		a.cachedTitleFacet.Typography = marks.Const(theme.TextLabelM)
	}
	if a.Disabled.Get() {
		a.cachedTitleFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		a.cachedTitleFacet.Disabled = marks.Const(true)
	} else {
		a.cachedTitleFacet.Disabled = marks.Const(false)
	}

	if a.cachedMessageFacet == nil {
		a.cachedMessageFacet = primitive.NewText(marks.Const(strings.TrimSpace(a.Message.Get())))
	} else {
		a.cachedMessageFacet.Content = marks.Const(strings.TrimSpace(a.Message.Get()))
		a.cachedMessageFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	a.cachedMessageFacet.Typography = marks.Const(theme.TextBodyM)
	a.cachedMessageFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	a.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextSecondary)
	if a.cachedDensity == theme.DensityIDCompact {
		a.cachedMessageFacet.Typography = marks.Const(theme.TextBodyS)
	}
	if a.Disabled.Get() {
		a.cachedMessageFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		a.cachedMessageFacet.Disabled = marks.Const(true)
	} else {
		a.cachedMessageFacet.Disabled = marks.Const(false)
	}

	actionLabel := strings.TrimSpace(a.ActionLabel.Get())
	if actionLabel == "" {
		a.cachedActionButton = nil
	} else {
		if a.cachedActionButton == nil {
			a.cachedActionButton = action.NewButton(marks.Const(actionLabel), marks.Const(uiinput.ButtonText))
			a.cachedActionButton.Activated.Subscribe(func(signal.Unit) {
				if a != nil {
					a.Actioned.Emit(signal.Unit{})
				}
			})
		} else {
			a.cachedActionButton.Label = marks.Const(actionLabel)
			a.cachedActionButton.Variant = marks.Const(uiinput.ButtonText)
		}
		if iconRef := strings.TrimSpace(a.ActionIconRef.Get()); iconRef != "" {
			a.cachedActionButton.LeadingIconRef = marks.Const(iconRef)
		} else {
			a.cachedActionButton.LeadingIconRef = marks.Const("")
		}
		a.cachedActionButton.Disabled = marks.Const(a.Disabled.Get())
	}

	closeLabel := strings.TrimSpace(a.CloseButtonLabel.Get())
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
		if iconRef := strings.TrimSpace(a.CloseButtonIconRef.Get()); iconRef != "" {
			a.cachedCloseButton.Icon = primitive.IconSVG(iconRef)
		} else {
			a.cachedCloseButton.Icon = primitive.IconSVG(alertDefaultCloseSVG)
		}
		a.cachedCloseButton.AccessibleLabel = marks.Const(closeLabel)
		a.cachedCloseButton.Disabled = marks.Const(a.Disabled.Get())
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
	a.cachedPadX = mathutil.Max(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingM)))
	a.cachedPadY = mathutil.Max(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	a.cachedGap = mathutil.Max(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingXS)))
	a.cachedRowGap = mathutil.Max(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	a.cachedIconSize = mathutil.Max(resolved.Density.Scale(20), float32(resolved.Spacing(theme.SpacingM)))
	a.cachedCloseSize = mathutil.Max(resolved.Density.Scale(24), float32(resolved.Spacing(theme.SpacingM))*1.2)
	a.syncChildren()
	children := a.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{})
		a.Layout.MeasuredSize = size
		a.Layout.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		return a.Layout.MeasuredResult
	}
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(480)
	}
	innerWidth := mathutil.Max(0, maxWidth-a.cachedPadX*2)
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
		a.cachedIconFacet.Size = marks.Const(a.cachedIconSize)
		iconSize := a.cachedIconFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: a.cachedIconSize, H: a.cachedIconSize}}).Size
		contentWidth = mathutil.Max(contentWidth, iconSize.W)
	}
	titleSize := a.cachedTitleFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: titleWidth, H: constraints.MaxSize.H}}).Size
	messageSize := a.cachedMessageFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: innerWidth, H: constraints.MaxSize.H}}).Size
	contentWidth = mathutil.Max(contentWidth, titleSize.W+a.cachedGap)
	if a.cachedCloseButton != nil {
		a.cachedCloseButton.Size = marks.Const(a.cachedCloseSize)
		a.cachedCloseButton.HitPadding = mathutil.Max(resolved.Density.Scale(4), float32(resolved.Spacing(theme.SpacingXS)))
		closeSize := a.cachedCloseButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: a.cachedCloseSize, H: a.cachedCloseSize}}).Size
		contentWidth = mathutil.Max(contentWidth, closeSize.W)
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
	contentWidth = mathutil.Max(contentWidth, titleSize.W)
	contentWidth = mathutil.Max(contentWidth, messageSize.W)
	size := gfx.Size{W: contentWidth + a.cachedPadX*2, H: innerHeight + a.cachedPadY*2}
	if size.W < a.cachedIconSize+a.cachedPadX*2 {
		size.W = a.cachedIconSize + a.cachedPadX*2
	}
	measured := constraints.Constrain(size)
	a.Layout.MeasuredSize = measured
	a.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return a.Layout.MeasuredResult
}

func (a *Alert) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	a.cachedBounds = bounds
	a.cachedSurfaceBounds = bounds.Inset(0, 0)
	a.cachedIconBounds = gfx.Rect{}
	a.cachedTitleBounds = gfx.Rect{}
	a.cachedMessageBounds = gfx.Rect{}
	a.cachedActionBounds = gfx.Rect{}
	a.cachedCloseBounds = gfx.Rect{}
	a.Layout.ArrangedBounds = bounds
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
		headerH = mathutil.Max(headerH, iconSize)
		headerH = mathutil.Max(headerH, closeSize)
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
		titleW := mathutil.Max(0, inner.Max.X-titleX)
		if a.cachedCloseButton != nil && a.cachedWritingDirection != facet.WritingDirectionRTL {
			titleW = mathutil.Max(0, closeX-a.cachedGap-titleX)
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
	if a.Disabled.Get() {
		state = theme.StateDisabled
	} else if a.pressed {
		state = theme.StatePressed
	} else if a.hovered {
		state = theme.StateHover
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.AlertSurface.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 64)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(surface) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds, float32(tokens.Radius.LG)), surface)...)
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
	if a != nil && a.Disabled.Get() {
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
	if a == nil || a.Disabled.Get() {
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
	if a == nil || a.Disabled.Get() {
		return false
	}
	if e.Kind == platform.KeyPress && e.Key == platform.KeyEscape {
		if strings.TrimSpace(a.CloseButtonLabel.Get()) != "" {
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
	services, ok := runtime.(facet.RuntimeServices)
	if !ok {
		return nil
	}
	v := reflect.ValueOf(services)
	switch v.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Slice, reflect.Chan, reflect.Func, reflect.Interface:
		if v.IsNil() {
			return nil
		}
	}
	return services
}
