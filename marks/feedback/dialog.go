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
	dialogMarkIDRoot        facet.MarkID = 1
	dialogMarkIDBackdrop    facet.MarkID = 2
	dialogMarkIDSurface     facet.MarkID = 3
	dialogMarkIDTitle       facet.MarkID = 4
	dialogMarkIDBody        facet.MarkID = 5
	dialogMarkIDActions     facet.MarkID = 6
	dialogMarkIDCloseButton facet.MarkID = 7
	dialogMarkIDFocusRing   facet.MarkID = 8
	dialogMarkIDBodyContent facet.MarkID = 9
	dialogMarkIDBodyChild   facet.MarkID = 10
)

// DialogContentLayoutMode controls how authored body content is arranged.
type DialogContentLayoutMode uint8

const (
	DialogContentLayoutVertical DialogContentLayoutMode = iota
	DialogContentLayoutHorizontal
	DialogContentLayoutGrid
)

func (m DialogContentLayoutMode) String() string {
	switch m {
	case DialogContentLayoutVertical:
		return "vertical"
	case DialogContentLayoutHorizontal:
		return "horizontal"
	case DialogContentLayoutGrid:
		return "grid"
	default:
		return "unknown"
	}
}

// DialogAction describes one dialog action button.
type DialogAction struct {
	Label    string
	Variant  uiinput.ButtonVariant
	Disabled bool
}

// DialogContentChild describes one reusable child facet placed inside the dialog body.
type DialogContentChild struct {
	Key       string
	Facet     facet.FacetImpl
	MarkID    facet.MarkID
	Grid      facet.GridPlacement
	ZPriority int32
}

// Dialog implements the feedback.dialog canonical mark.
type Dialog struct {
	marks.Core

	textRole facet.TextRole

	Actioned  signal.Signal[int]
	Dismissed signal.Signal[signal.Unit]

	Title              marks.Binding[string]
	Body               marks.Binding[string]
	ContentLayoutMode  marks.Binding[DialogContentLayoutMode]
	ContentGridColumns marks.Binding[int]
	ContentGridRows    marks.Binding[int]
	Actions            marks.Binding[[]DialogAction]
	ContentChildren    marks.Binding[[]DialogContentChild]
	CloseButtonLabel   marks.Binding[string]
	Disabled           marks.Binding[bool]
	Open               marks.Binding[bool]

	hovered        bool
	pressed        bool
	focusedVisible bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.FeedbackDialogSlots
	cachedBounds           gfx.Rect
	cachedBackdropBounds   gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedTitleBounds      gfx.Rect
	cachedBodyBounds       gfx.Rect
	cachedActionsBounds    gfx.Rect
	cachedCloseBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRowGap           float32
	cachedSurfaceRadius    float32
	cachedWritingDirection facet.WritingDirection
	cachedTitleFacet       *primitive.Text
	cachedBodyGroup        *dialogBodyGroup
	cachedActionsFacet     *dialogActionGroup
	cachedCloseButton      *action.IconButton
}

var _ facet.FacetImpl = (*Dialog)(nil)
var _ layout.AnchorExporter = (*Dialog)(nil)
var _ marks.Mark = (*Dialog)(nil)

const dialogDefaultCloseSVG = `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2.2" stroke-linecap="round" stroke-linejoin="round"><path d="M6 6l12 12"/><path d="M18 6 6 18"/></svg>`

// NewDialog constructs a feedback.dialog mark with canonical defaults.
func NewDialog(title, body string, actions []DialogAction) *Dialog {
	d := &Dialog{
		Title:              marks.Const(title),
		Body:               marks.Const(body),
		ContentLayoutMode:  marks.Const(DialogContentLayoutVertical),
		ContentGridColumns: marks.Const(1),
		ContentGridRows:    marks.Const(1),
		Actions:            marks.Const(append([]DialogAction(nil), actions...)),
		ContentChildren:    marks.Const[[]DialogContentChild](nil),
		CloseButtonLabel:   marks.Const(""),
		Disabled:           marks.Const(false),
		Open:               marks.Const(true),
	}
	d.Facet = facet.NewFacet()
	d.AddBinding(d.Title)
	d.AddBinding(d.Body)
	d.AddBinding(d.ContentLayoutMode)
	d.AddBinding(d.ContentGridColumns)
	d.AddBinding(d.ContentGridRows)
	d.AddBinding(d.Actions)
	d.AddBinding(d.ContentChildren)
	d.AddBinding(d.CloseButtonLabel)
	d.AddBinding(d.Disabled)
	d.AddBinding(d.Open)

	d.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   dialogGroupPolicy{dialog: d},
		Children: d,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	d.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := d.measure(ctx, constraints).Size
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
	d.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return d.measure(ctx, constraints)
	}
	d.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		d.Layout.ArrangedBounds = bounds
		d.arrange(ctx, bounds)
	}
	d.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := d.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	d.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return d.hitTest(p) }
	d.Input.OnPointer = func(e facet.PointerEvent) bool { return d.onPointer(e) }
	d.Input.OnKey = func(e facet.KeyEvent) bool { return d.onKey(e) }
	d.Input.OnDismiss = func(e facet.DismissEvent) bool { return d.onDismiss(e) }
	d.Focus.Focusable = func() bool { return !d.Disabled.Get() && d.Open.Get() }
	d.Focus.TabIndex = 0
	d.Focus.OnFocusGained = func() { d.OnFocusGained() }
	d.Focus.OnFocusLost = func() { d.OnFocusLost() }
	d.textRole.IMEEnabled = false
	d.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return d.buildCommands(d.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	d.RegisterRoles()
	d.AddRole(&d.textRole)
	d.syncChildren()
	return d
}

// Base satisfies facet.FacetImpl.
func (d *Dialog) Base() *facet.Facet {
	d.BindImpl(d)
	return &d.Facet
}

// Descriptor satisfies marks.Mark.
func (d *Dialog) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "feedback", TypeName: "dialog"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (d *Dialog) AccessibilityRole() string { return "dialog" }

// AccessibleName reports the semantic name source required by the spec.
func (d *Dialog) AccessibleName() string {
	if d == nil {
		return ""
	}
	return strings.TrimSpace(d.Title.Get())
}

// Children returns the dialog's immediate semantic children.
func (d *Dialog) Children() []facet.GroupChild {
	if d == nil || !d.Open.Get() {
		return nil
	}
	d.syncChildren()
	out := make([]facet.GroupChild, 0, 4)
	if d.cachedTitleFacet != nil {
		out = append(out, dialogGroupChild(d.cachedTitleFacet.Base(), dialogMarkIDTitle, 0))
	}
	if d.cachedBodyGroup != nil {
		out = append(out, dialogGroupChild(d.cachedBodyGroup.Base(), dialogMarkIDBody, 1))
	}
	if d.cachedActionsFacet != nil {
		out = append(out, dialogGroupChild(d.cachedActionsFacet.Base(), dialogMarkIDActions, 2))
	}
	if d.cachedCloseButton != nil {
		out = append(out, dialogGroupChild(d.cachedCloseButton.Base(), dialogMarkIDCloseButton, 3))
	}
	return out
}

// ExportAnchors publishes the dialog anchor set.
func (d *Dialog) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if d == nil {
		return nil
	}
	bounds := d.Layout.ArrangedBounds
	out := d.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if !d.cachedTitleBounds.IsEmpty() {
		out["baseline"] = gfx.Point{X: d.cachedTitleBounds.Min.X, Y: d.cachedTitleBounds.Min.Y}
	} else if d.cachedBodyGroup != nil && !d.cachedBodyGroup.cachedTextBounds.IsEmpty() {
		out["baseline"] = gfx.Point{X: d.cachedBodyGroup.cachedTextBounds.Min.X, Y: d.cachedBodyGroup.cachedTextBounds.Min.Y}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	if !d.cachedSurfaceBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{X: (d.cachedSurfaceBounds.Min.X + d.cachedSurfaceBounds.Max.X) * 0.5, Y: (d.cachedSurfaceBounds.Min.Y + d.cachedSurfaceBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach delegates to Core.
func (d *Dialog) OnAttach(ctx facet.AttachContext) { d.Core.OnAttach() }

// OnActivate delegates to Core.
func (d *Dialog) OnActivate() { d.Core.OnActivate() }

// OnDeactivate delegates to Core.
func (d *Dialog) OnDeactivate() { d.Core.OnDeactivate() }

// OnFocusGained marks the dialog as focus-visible.
func (d *Dialog) OnFocusGained() {
	d.focusedVisible = true
	d.invalidate(facet.DirtyProjection)
}

// OnFocusLost clears the focus-visible state.
func (d *Dialog) OnFocusLost() {
	d.focusedVisible = false
	d.invalidate(facet.DirtyProjection)
}

// OnDetach clears cached projection state.
func (d *Dialog) OnDetach() {
	d.Core.OnDetach()
	d.cachedTokens = theme.Tokens{}
	d.cachedRecipe = shared.FeedbackDialogSlots{}
	d.cachedBounds = gfx.Rect{}
	d.cachedBackdropBounds = gfx.Rect{}
	d.cachedSurfaceBounds = gfx.Rect{}
	d.cachedTitleBounds = gfx.Rect{}
	d.cachedBodyBounds = gfx.Rect{}
	d.cachedActionsBounds = gfx.Rect{}
	d.cachedCloseBounds = gfx.Rect{}
	d.cachedPadX = 0
	d.cachedPadY = 0
	d.cachedGap = 0
	d.cachedRowGap = 0
	d.cachedSurfaceRadius = 0
	d.cachedTitleFacet = nil
	d.cachedBodyGroup = nil
	d.cachedActionsFacet = nil
	d.cachedCloseButton = nil
}

func (d *Dialog) invalidate(flags facet.DirtyFlags) {
	if d == nil {
		return
	}
	d.Invalidate(flags)
}

func (d *Dialog) syncChildren() {
	if d == nil {
		return
	}
	title := strings.TrimSpace(d.Title.Get())
	if d.cachedTitleFacet == nil {
		d.cachedTitleFacet = primitive.NewText(marks.Const(title))
	} else {
		d.cachedTitleFacet.Content = marks.Const(title)
		d.cachedTitleFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	d.cachedTitleFacet.Typography = marks.Const(theme.TextHeadingS)
	d.cachedTitleFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	d.cachedTitleFacet.Foreground = marks.Const(theme.ColorText)
	if d.Disabled.Get() {
		d.cachedTitleFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		d.cachedTitleFacet.Disabled = marks.Const(true)
	} else {
		d.cachedTitleFacet.Disabled = marks.Const(false)
	}
	if d.cachedBodyGroup == nil {
		d.cachedBodyGroup = newDialogBodyGroup(d)
	}
	d.cachedBodyGroup.syncContent()
	actions := d.Actions.Get()
	if len(actions) == 0 {
		d.cachedActionsFacet = nil
	} else {
		if d.cachedActionsFacet == nil {
			d.cachedActionsFacet = newDialogActionGroup(d)
		}
		d.cachedActionsFacet.syncActions(actions, d.Disabled.Get())
	}
	if strings.TrimSpace(d.CloseButtonLabel.Get()) == "" {
		d.cachedCloseButton = nil
	} else {
		if d.cachedCloseButton == nil {
			d.cachedCloseButton = action.NewIconButton(primitive.IconSVG(dialogDefaultCloseSVG))
			d.cachedCloseButton.Activated.Subscribe(func(signal.Unit) {
				if d != nil && !d.Disabled.Get() && d.Open.Get() {
					d.closeAndDismiss()
				}
			})
		}
		d.cachedCloseButton.Icon = primitive.IconSVG(dialogDefaultCloseSVG)
		d.cachedCloseButton.AccessibleLabel = marks.Const(strings.TrimSpace(d.CloseButtonLabel.Get()))
		d.cachedCloseButton.Disabled = marks.Const(d.Disabled.Get())
	}
}

func (d *Dialog) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if !d.Open.Get() {
		size := constraints.Constrain(gfx.Size{})
		d.Layout.MeasuredSize = size
		d.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return d.Layout.MeasuredResult
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uifeedback.ResolveDialogRecipe(style, d.dialogVariant())
	d.cachedTokens = resolved.TokenSet()
	d.cachedRecipe = slots
	d.cachedWritingDirection = ctx.WritingDirection
	d.cachedPadX = mathutil.Max(resolved.Density.Scale(16), float32(resolved.Spacing(theme.SpacingL)))
	d.cachedPadY = mathutil.Max(resolved.Density.Scale(14), float32(resolved.Spacing(theme.SpacingM)))
	d.cachedGap = mathutil.Max(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS)))
	d.cachedRowGap = mathutil.Max(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingM)))
	d.cachedSurfaceRadius = mathutil.Max(float32(resolved.Radius(theme.RadiusL).Float32()), float32(resolved.Radius(theme.RadiusM).Float32()))
	d.syncChildren()
	innerMaxW := constraints.MaxSize.W
	if innerMaxW > 0 {
		innerMaxW = mathutil.Max(0, innerMaxW-d.cachedPadX*2)
	}
	measureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	closeSize := gfx.Size{}
	if d.cachedCloseButton != nil {
		target := mathutil.Max(resolved.Density.Scale(24), float32(resolved.TokenSet().Spacing.TouchTarget)*0.55)
		closeSize = d.cachedCloseButton.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: target, H: target}}).Size
	}
	titleMaxW := innerMaxW
	if closeSize.W > 0 || closeSize.H > 0 {
		titleMaxW = mathutil.Max(0, innerMaxW-closeSize.W-d.cachedGap)
	}
	titleSize := d.cachedTitleFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: titleMaxW, H: constraints.MaxSize.H}}).Size
	bodySize := gfx.Size{}
	if d.cachedBodyGroup != nil {
		bodySize = d.cachedBodyGroup.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: innerMaxW, H: constraints.MaxSize.H}}).Size
	}
	actionsSize := gfx.Size{}
	if d.cachedActionsFacet != nil {
		actionsSize = d.cachedActionsFacet.Base().LayoutRole().Measure(measureCtx, facet.Constraints{MaxSize: gfx.Size{W: innerMaxW, H: constraints.MaxSize.H}}).Size
	}
	headerH := titleSize.H
	headerW := titleSize.W
	if closeSize.W > 0 || closeSize.H > 0 {
		headerH = mathutil.Max(headerH, closeSize.H)
		if headerW > 0 {
			headerW += d.cachedGap + closeSize.W
		} else {
			headerW = closeSize.W
		}
	} else {
		headerH = mathutil.Max(headerH, resolved.Density.Scale(24))
	}
	contentW := mathutil.Max(headerW, bodySize.W)
	contentW = mathutil.Max(contentW, actionsSize.W)
	contentH := headerH
	if bodySize.H > 0 {
		contentH += d.cachedRowGap + bodySize.H
	}
	if actionsSize.H > 0 {
		contentH += d.cachedRowGap + actionsSize.H
	}
	surfaceSize := gfx.Size{
		W: contentW + d.cachedPadX*2,
		H: contentH + d.cachedPadY*2,
	}
	surfaceSize.W = mathutil.Max(surfaceSize.W, resolved.Density.Scale(280))
	surfaceSize.H = mathutil.Max(surfaceSize.H, resolved.Density.Scale(160))
	if constraints.MaxSize.W > 0 {
		surfaceSize.W = mathutil.Min(surfaceSize.W, constraints.MaxSize.W)
	}
	if constraints.MaxSize.H > 0 {
		surfaceSize.H = mathutil.Min(surfaceSize.H, constraints.MaxSize.H)
	}
	size := surfaceSize
	if constraints.MaxSize.W > 0 {
		size.W = constraints.MaxSize.W
	}
	if constraints.MaxSize.H > 0 {
		size.H = constraints.MaxSize.H
	}
	measured := constraints.Constrain(size)
	d.Layout.MeasuredSize = measured
	d.Layout.MeasuredResult = facet.MeasureResult{Size: measured, Intrinsic: facet.IntrinsicSize{Min: measured, Preferred: measured, Max: measured}, Constraints: constraints}
	return d.Layout.MeasuredResult
}

func (d *Dialog) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	d.cachedBounds = bounds
	d.cachedBackdropBounds = bounds
	d.cachedSurfaceBounds = gfx.Rect{}
	d.cachedTitleBounds = gfx.Rect{}
	d.cachedBodyBounds = gfx.Rect{}
	d.cachedActionsBounds = gfx.Rect{}
	d.cachedCloseBounds = gfx.Rect{}
	d.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || !d.Open.Get() {
		return
	}
	d.syncChildren()
	margin := mathutil.Max(d.cachedPadX, d.cachedPadY)
	surfaceSize := gfx.Size{
		W: mathutil.Max(d.cachedPadX*2+resolvedDialogContentWidth(d), resolvedDialogMinWidth(d)),
		H: mathutil.Max(d.cachedPadY*2+resolvedDialogContentHeight(d), resolvedDialogMinHeight(d)),
	}
	surfaceSize.W = mathutil.Min(surfaceSize.W, mathutil.Max(0, bounds.Width()-margin*2))
	surfaceSize.H = mathutil.Min(surfaceSize.H, mathutil.Max(0, bounds.Height()-margin*2))
	d.cachedSurfaceBounds = text.CenterRect(bounds, surfaceSize.W, surfaceSize.H)
	content := d.cachedSurfaceBounds.Inset(d.cachedPadX, d.cachedPadY)
	if content.IsEmpty() {
		content = d.cachedSurfaceBounds
	}
	closeSize := gfx.Size{}
	if d.cachedCloseButton != nil {
		closeSize = d.cachedCloseButton.Base().LayoutRole().MeasuredSize
	}
	titleSize := d.cachedTitleFacet.Base().LayoutRole().MeasuredSize
	bodySize := gfx.Size{}
	if d.cachedBodyGroup != nil {
		bodySize = d.cachedBodyGroup.Base().LayoutRole().MeasuredSize
	}
	actionsSize := gfx.Size{}
	if d.cachedActionsFacet != nil {
		actionsSize = d.cachedActionsFacet.Base().LayoutRole().MeasuredSize
	}
	headerH := mathutil.Max(titleSize.H, closeSize.H)
	rowRects := layout.ArrangeVerticalFlowAligned(content, 0, d.cachedRowGap, []gfx.Size{
		{W: content.Width(), H: headerH},
		{W: content.Width(), H: bodySize.H},
		{W: content.Width(), H: actionsSize.H},
	}, d.cachedWritingDirection == facet.WritingDirectionRTL, layout.AlignStart)

	totalHeight := headerH
	if bodySize.H > 0 {
		totalHeight += d.cachedRowGap + bodySize.H
	}
	if actionsSize.H > 0 {
		totalHeight += d.cachedRowGap + actionsSize.H
	}

	if content.Height() > totalHeight {
		if len(rowRects) > 2 && actionsSize.H > 0 {
			rowRects[2].Min.Y = content.Max.Y - actionsSize.H
			rowRects[2].Max.Y = content.Max.Y
		}
		if len(rowRects) > 1 && bodySize.H > 0 {
			rowRects[1].Min.Y = content.Min.Y + headerH + d.cachedRowGap
			rowRects[1].Max.Y = rowRects[1].Min.Y + bodySize.H
		}
	}
	headerRect := rowRects[0]
	if closeSize.W > 0 || closeSize.H > 0 {
		closeX := headerRect.Max.X - closeSize.W
		titleX := headerRect.Min.X
		if d.cachedWritingDirection == facet.WritingDirectionRTL {
			closeX = headerRect.Min.X
			titleX = headerRect.Min.X + closeSize.W + d.cachedGap
		}
		closeRect := gfx.RectFromXYWH(closeX, headerRect.Min.Y, closeSize.W, closeSize.H)
		d.cachedCloseButton.Base().LayoutRole().Arrange(ctx, closeRect)
		d.cachedCloseBounds = closeRect
		titleW := mathutil.Max(0, headerRect.Width()-closeSize.W-d.cachedGap)
		titleRect := gfx.RectFromXYWH(titleX, headerRect.Min.Y, titleW, titleSize.H)
		d.cachedTitleFacet.Base().LayoutRole().Arrange(ctx, titleRect)
		d.cachedTitleBounds = titleRect
	} else {
		titleRect := gfx.RectFromXYWH(headerRect.Min.X, headerRect.Min.Y, headerRect.Width(), titleSize.H)
		d.cachedTitleFacet.Base().LayoutRole().Arrange(ctx, titleRect)
		d.cachedTitleBounds = titleRect
	}
	if d.cachedBodyGroup != nil && bodySize.H > 0 {
		bodyRect := gfx.RectFromXYWH(rowRects[1].Min.X, rowRects[1].Min.Y, rowRects[1].Width(), bodySize.H)
		d.cachedBodyGroup.Base().LayoutRole().Arrange(ctx, bodyRect)
		d.cachedBodyBounds = bodyRect
	}
	if d.cachedActionsFacet != nil && actionsSize.H > 0 {
		actionsRect := gfx.RectFromXYWH(rowRects[2].Min.X, rowRects[2].Min.Y, actionsSize.W, actionsSize.H)
		if d.cachedWritingDirection == facet.WritingDirectionRTL {
			actionsRect.Min.X = content.Min.X
			actionsRect.Max.X = actionsRect.Min.X + actionsSize.W
		} else {
			actionsRect.Min.X = content.Max.X - actionsSize.W
			actionsRect.Max.X = content.Max.X
		}
		d.cachedActionsFacet.Base().LayoutRole().Arrange(ctx, actionsRect)
		d.cachedActionsBounds = actionsRect
	}
}

func (d *Dialog) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if d == nil || bounds.IsEmpty() || !d.Open.Get() {
		return nil
	}
	style, slots := d.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := d.dialogState()
	root := slots.Root.Resolve(state, tokens)
	backdrop := slots.Backdrop.Resolve(state, tokens)
	surface := slots.ModalSurface.Resolve(state, tokens)
	focusRing := slots.FocusRing.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(backdrop) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), backdrop)...)
	}
	if !theme.IsTransparentMaterial(surface) && !d.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(d.cachedSurfaceBounds, d.cachedSurfaceRadius), surface)...)
	}
	if !theme.IsTransparentMaterial(focusRing) && d.focusedVisible && !d.cachedSurfaceBounds.IsEmpty() {
		ring := d.cachedSurfaceBounds.Inset(-mathutil.Max(1, d.cachedGap*0.35), -mathutil.Max(1, d.cachedGap*0.35))
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(ring, d.cachedSurfaceRadius+mathutil.Max(1, d.cachedGap*0.35)), focusRing)...)
	}
	if !d.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: d.cachedSurfaceBounds})
		if d.cachedCloseButton != nil && !d.cachedCloseBounds.IsEmpty() {
			if projected := d.cachedCloseButton.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedCloseBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if d.cachedTitleFacet != nil && !d.cachedTitleBounds.IsEmpty() {
			if projected := d.cachedTitleFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedTitleBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if d.cachedBodyGroup != nil && !d.cachedBodyBounds.IsEmpty() {
			if projected := d.cachedBodyGroup.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedBodyBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		if d.cachedActionsFacet != nil && !d.cachedActionsBounds.IsEmpty() {
			if projected := d.cachedActionsFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: d.cachedActionsBounds, ContentScale: contentScale}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	return cmds
}

func (d *Dialog) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.FeedbackDialogSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: d.cachedTokens}, d.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, d.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uifeedback.ResolveDialogRecipe(style, d.dialogVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: d.cachedTokens}, d.cachedRecipe
}

func (d *Dialog) dialogState() theme.InteractionState {
	if d == nil {
		return theme.StateDefault
	}
	if d.Disabled.Get() {
		return theme.StateDisabled
	}
	if d.pressed {
		return theme.StatePressed
	}
	if d.focusedVisible {
		return theme.StateFocused
	}
	if d.hovered {
		return theme.StateHover
	}
	return theme.StateDefault
}

func (d *Dialog) dialogVariant() uifeedback.DialogVariant {
	if d == nil {
		return uifeedback.DialogDefault
	}
	if d.Disabled.Get() {
		return uifeedback.DialogDisabled
	}
	if d.pressed {
		return uifeedback.DialogActive
	}
	if d.focusedVisible {
		return uifeedback.DialogFocused
	}
	if d.hovered {
		return uifeedback.DialogHover
	}
	if d.Open.Get() {
		return uifeedback.DialogOpen
	}
	return uifeedback.DialogDefault
}

func (d *Dialog) hitTest(p gfx.Point) facet.HitResult {
	if d == nil || !d.Open.Get() || d.cachedBounds.IsEmpty() || !d.cachedBounds.Contains(p) {
		return facet.HitResult{}
	}
	switch {
	case !d.cachedCloseBounds.IsEmpty() && d.cachedCloseBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDCloseButton, Cursor: facet.CursorPointer}
	case !d.cachedActionsBounds.IsEmpty() && d.cachedActionsBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDActions, Cursor: facet.CursorPointer}
	case !d.cachedTitleBounds.IsEmpty() && d.cachedTitleBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDTitle}
	case !d.cachedBodyBounds.IsEmpty() && d.cachedBodyBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDBody}
	case !d.cachedSurfaceBounds.IsEmpty() && d.cachedSurfaceBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDSurface}
	default:
		return facet.HitResult{Hit: true, MarkID: dialogMarkIDBackdrop}
	}
}

func (d *Dialog) onPointer(e facet.PointerEvent) bool {
	if d == nil || d.Disabled.Get() || !d.Open.Get() {
		return false
	}
	if !d.cachedBounds.Contains(e.Position) {
		if e.Kind == platform.PointerLeave {
			d.hovered = false
			d.pressed = false
			d.invalidate(facet.DirtyProjection)
		}
		return false
	}
	if !d.cachedSurfaceBounds.IsEmpty() && !d.cachedSurfaceBounds.Contains(e.Position) {
		switch e.Kind {
		case platform.PointerEnter, platform.PointerMove:
			if !d.hovered {
				d.hovered = true
				d.invalidate(facet.DirtyProjection)
			}
			return true
		case platform.PointerPress:
			if e.Button == platform.PointerLeft {
				d.closeAndDismiss()
				return true
			}
		case platform.PointerLeave:
			d.hovered = false
			d.pressed = false
			d.invalidate(facet.DirtyProjection)
			return true
		}
	}
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if !d.hovered {
			d.hovered = true
			d.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			d.pressed = true
			d.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerRelease:
		if d.pressed {
			d.pressed = false
			d.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerLeave:
		d.hovered = false
		d.pressed = false
		d.invalidate(facet.DirtyProjection)
		return true
	}
	return false
}

func (d *Dialog) onKey(e facet.KeyEvent) bool {
	if d == nil || d.Disabled.Get() || !d.Open.Get() {
		return false
	}
	if e.Kind != platform.KeyPress {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		d.closeAndDismiss()
		return true
	case platform.KeyEnter, platform.KeySpace:
		if d.cachedActionsFacet != nil && len(d.cachedActionsFacet.Buttons) > 0 {
			d.onAction(0)
			return true
		}
	}
	return false
}

func (d *Dialog) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if d == nil || d.Disabled.Get() || !d.Open.Get() {
		return false
	}
	d.closeAndDismiss()
	return true
}

func (d *Dialog) onAction(index int) {
	if d == nil || d.Disabled.Get() || !d.Open.Get() {
		return
	}
	d.Actioned.Emit(index)
	d.closeAndDismiss()
}

func (d *Dialog) closeAndDismiss() {
	if d == nil || !d.Open.Get() {
		return
	}
	d.Open = marks.Const(false)
	d.hovered = false
	d.pressed = false
	d.focusedVisible = false
	d.Dismissed.Emit(signal.Unit{})
	d.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func dialogGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
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

type dialogGroupPolicy struct {
	dialog *Dialog
}

func (dialogGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p dialogGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.dialog == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.dialog.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p dialogGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.dialog == nil {
		return nil, nil
	}
	p.dialog.arrange(ctx.ArrangeContext, ctx.Bounds)
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

type dialogActionGroup struct {
	marks.Core

	parent  *Dialog
	Buttons []*action.Button
}

func newDialogActionGroup(parent *Dialog) *dialogActionGroup {
	g := &dialogActionGroup{
		parent: parent,
	}
	g.Facet = facet.NewFacet()
	g.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearHorizontal,
		Policy:   dialogActionGroupPolicy{group: g},
		Children: g,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	g.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := g.measure(ctx, constraints)
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchNever,
			Height: facet.StretchNever,
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
	g.RegisterRoles()
	return g
}

func (g *dialogActionGroup) Base() *facet.Facet {
	g.BindImpl(g)
	return &g.Facet
}

func (g *dialogActionGroup) Children() []facet.GroupChild {
	if g == nil || len(g.Buttons) == 0 {
		return nil
	}
	out := make([]facet.GroupChild, 0, len(g.Buttons))
	for i, btn := range g.Buttons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		out = append(out, dialogGroupChild(btn.Base(), dialogMarkIDActions, i))
	}
	return out
}

func (g *dialogActionGroup) OnAttach(ctx facet.AttachContext) { g.Core.OnAttach() }
func (g *dialogActionGroup) OnActivate()                      { g.Core.OnActivate() }
func (g *dialogActionGroup) OnDeactivate()                    { g.Core.OnDeactivate() }
func (g *dialogActionGroup) OnDetach()                        { g.Core.OnDetach() }

func (g *dialogActionGroup) syncActions(actions []DialogAction, disabled bool) {
	if g == nil {
		return
	}
	if len(actions) == 0 {
		g.Buttons = nil
		return
	}
	if len(g.Buttons) != len(actions) {
		g.Buttons = make([]*action.Button, len(actions))
	}
	for i, spec := range actions {
		btn := g.Buttons[i]
		if btn == nil {
			btn = action.NewButton(marks.Const(strings.TrimSpace(spec.Label)), marks.Const(spec.variant()))
			index := i
			btn.Activated.Subscribe(func(signal.Unit) {
				if g != nil && g.parent != nil {
					g.parent.onAction(index)
				}
			})
			g.Buttons[i] = btn
		} else {
			btn.Label = marks.Const(strings.TrimSpace(spec.Label))
			btn.Variant = marks.Const(spec.variant())
		}
		btn.Disabled = marks.Const(disabled || spec.Disabled)
	}
}

func (g *dialogActionGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if g == nil || len(g.Buttons) == 0 {
		return gfx.Size{}
	}
	var width float32
	var height float32
	spacing := float32(8)
	for i, btn := range g.Buttons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		size := btn.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H}}).Size
		if i > 0 {
			width += spacing
		}
		width += size.W
		height = mathutil.Max(height, size.H)
	}
	return gfx.Size{W: width, H: height}
}

func (g *dialogActionGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if g == nil || bounds.IsEmpty() || len(g.Buttons) == 0 {
		return
	}
	spacing := mathutil.Max[float32](0, 8)
	totalW := float32(0)
	sizes := make([]gfx.Size, len(g.Buttons))
	for i, btn := range g.Buttons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		sizes[i] = btn.Base().LayoutRole().MeasuredSize
		if i > 0 {
			totalW += spacing
		}
		totalW += sizes[i].W
	}
	x := bounds.Min.X
	if g.parent != nil && g.parent.cachedWritingDirection != facet.WritingDirectionRTL {
		x = bounds.Max.X - totalW
	}
	for i, btn := range g.Buttons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		size := sizes[i]
		if i > 0 {
			x += spacing
		}
		rect := gfx.RectFromXYWH(x, bounds.Min.Y, size.W, mathutil.Max(bounds.Height(), size.H))
		btn.Base().LayoutRole().Arrange(ctx, rect)
		x += size.W
	}
}

type dialogActionGroupPolicy struct {
	group *dialogActionGroup
}

func (dialogActionGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (p dialogActionGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	return facet.GroupMeasureResult{Size: p.group.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})}, nil
}

func (p dialogActionGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
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

func (a DialogAction) variant() uiinput.ButtonVariant {
	if a.Variant != 0 {
		return a.Variant
	}
	return uiinput.ButtonOutlined
}

type dialogBodyGroup struct {
	marks.Core

	textRole facet.TextRole

	parent *Dialog

	cachedTextFacet *primitive.Text

	cachedBounds           gfx.Rect
	cachedTextBounds       gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedChildren         []DialogContentChild
	cachedMeasuredChildren []dialogBodyChildMeasure
	cachedChildrenMap      map[facet.FacetID]gfx.Rect
	cachedLayoutMode       DialogContentLayoutMode
	cachedGridColumns      int
	cachedGridRows         int
	cachedWritingDir       facet.WritingDirection
}

func newDialogBodyGroup(parent *Dialog) *dialogBodyGroup {
	g := &dialogBodyGroup{
		parent: parent,
	}
	g.Facet = facet.NewFacet()
	g.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   dialogBodyGroupPolicy{group: g},
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

func (g *dialogBodyGroup) Base() *facet.Facet {
	g.BindImpl(g)
	return &g.Facet
}

func (g *dialogBodyGroup) Children() []facet.GroupChild {
	if g == nil || g.parent == nil || !g.parent.Open.Get() {
		return nil
	}
	g.syncContent()
	out := make([]facet.GroupChild, 0, 1+len(g.cachedChildren))
	if g.cachedTextFacet != nil {
		out = append(out, dialogBodyGroupChild(g.cachedTextFacet.Base(), dialogMarkIDBodyContent, 0, g.bodyPlacement(0, g.cachedTextFacet.Base().LayoutRole().Child, facet.GridPlacement{})))
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
		order := i
		if g.cachedTextFacet != nil {
			order++
		}
		out = append(out, dialogBodyGroupChild(base, dialogMarkIDBodyChild+facet.MarkID(i), order, g.bodyPlacement(order, base.LayoutRole().Child, spec.Grid)))
	}
	return out
}

func (g *dialogBodyGroup) OnAttach(ctx facet.AttachContext) { g.Core.OnAttach() }
func (g *dialogBodyGroup) OnActivate()                      { g.Core.OnActivate() }
func (g *dialogBodyGroup) OnDeactivate()                    { g.Core.OnDeactivate() }
func (g *dialogBodyGroup) OnDetach()                        { g.Core.OnDetach() }

func (g *dialogBodyGroup) syncContent() {
	if g == nil || g.parent == nil {
		return
	}
	g.cachedLayoutMode = g.parent.ContentLayoutMode.Get()
	g.cachedGridColumns = g.parent.ContentGridColumns.Get()
	g.cachedGridRows = g.parent.ContentGridRows.Get()
	g.cachedWritingDir = g.parent.cachedWritingDirection
	if g.cachedGridColumns < 1 {
		g.cachedGridColumns = 1
	}
	if g.cachedGridRows < 1 {
		g.cachedGridRows = 1
	}
	if g.Layout.Parent.Policy != nil {
		g.Layout.Parent.Kind = g.groupKind()
	}
	body := strings.TrimSpace(g.parent.Body.Get())
	if body == "" {
		g.cachedTextFacet = nil
	} else {
		if g.cachedTextFacet == nil {
			g.cachedTextFacet = primitive.NewText(marks.Const(body))
		} else {
			g.cachedTextFacet.Content = marks.Const(body)
			g.cachedTextFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		}
		g.cachedTextFacet.Typography = marks.Const(theme.TextBodyM)
		g.cachedTextFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
		g.cachedTextFacet.Foreground = marks.Const(theme.ColorTextSecondary)
		if g.parent.Disabled.Get() {
			g.cachedTextFacet.Foreground = marks.Const(theme.ColorTextDisabled)
			g.cachedTextFacet.Disabled = marks.Const(true)
		} else {
			g.cachedTextFacet.Disabled = marks.Const(false)
		}
	}
	g.cachedChildren = append(g.cachedChildren[:0], g.parent.ContentChildren.Get()...)
}

func (g *dialogBodyGroup) groupKind() facet.GroupLayoutKind {
	switch g.cachedLayoutMode {
	case DialogContentLayoutHorizontal:
		return facet.GroupLayoutLinearHorizontal
	case DialogContentLayoutGrid:
		return facet.GroupLayoutGrid
	default:
		return facet.GroupLayoutLinearVertical
	}
}

func (g *dialogBodyGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if g == nil || g.parent == nil || !g.parent.Open.Get() {
		return gfx.Size{}
	}
	g.syncContent()
	if g.cachedTextFacet == nil && len(g.cachedChildren) == 0 {
		g.cachedBounds = gfx.Rect{}
		g.cachedTextBounds = gfx.Rect{}
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
	maxSize := constraints.MaxSize
	inner := g.measureChildren(measureCtx, maxSize)
	g.cachedMeasuredChildren = append(g.cachedMeasuredChildren[:0], inner...)
	size := g.measureContentSize(inner, maxSize)
	g.cachedBounds = gfx.RectFromXYWH(0, 0, size.W, size.H)
	g.cachedContentBounds = g.cachedBounds
	return size
}

func (g *dialogBodyGroup) measureChildren(ctx facet.MeasureContext, maxSize gfx.Size) []dialogBodyChildMeasure {
	out := make([]dialogBodyChildMeasure, 0, 1+len(g.cachedChildren))
	if g.cachedTextFacet != nil && g.cachedTextFacet.Base() != nil && g.cachedTextFacet.Base().LayoutRole() != nil {
		size := g.cachedTextFacet.Base().LayoutRole().Measure(ctx, facet.Constraints{MaxSize: maxSize}).Size
		out = append(out, dialogBodyChildMeasure{
			facet: g.cachedTextFacet.Base(),
			size:  size,
		})
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
		out = append(out, dialogBodyChildMeasure{
			facet:     base,
			size:      size,
			grid:      spec.Grid,
			markID:    spec.MarkID,
			zPriority: spec.ZPriority,
		})
	}
	return out
}

func (g *dialogBodyGroup) measureContentSize(children []dialogBodyChildMeasure, maxSize gfx.Size) gfx.Size {
	if len(children) == 0 {
		return gfx.Size{}
	}
	switch g.cachedLayoutMode {
	case DialogContentLayoutHorizontal:
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
	case DialogContentLayoutGrid:
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

func (g *dialogBodyGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open.Get() {
		return
	}
	g.syncContent()
	g.cachedBounds = bounds
	g.cachedTextBounds = gfx.Rect{}
	g.cachedContentBounds = gfx.Rect{}
	if g.cachedTextFacet == nil && len(g.cachedChildren) == 0 {
		g.cachedChildrenMap = nil
		return
	}
	children := g.cachedMeasuredChildren
	arranged := g.arrangeChildren(ctx, bounds, children)
	g.cachedChildrenMap = make(map[facet.FacetID]gfx.Rect, len(arranged))
	for _, child := range arranged {
		g.cachedChildrenMap[child.facet.ID()] = child.bounds
		if g.cachedTextFacet != nil && child.facet.ID() == g.cachedTextFacet.Base().ID() {
			g.cachedTextBounds = child.bounds
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

func (g *dialogBodyGroup) arrangeChildren(ctx facet.ArrangeContext, bounds gfx.Rect, children []dialogBodyChildMeasure) []dialogBodyChildArrange {
	if len(children) == 0 {
		return nil
	}
	switch g.cachedLayoutMode {
	case DialogContentLayoutHorizontal:
		x := bounds.Min.X
		arranged := make([]dialogBodyChildArrange, 0, len(children))
		for i := range children {
			if i > 0 {
				x += g.parent.cachedGap
			}
			rect := gfx.RectFromXYWH(x, bounds.Min.Y, children[i].size.W, mathutil.Max(bounds.Height(), children[i].size.H))
			children[i].facet.LayoutRole().Arrange(ctx, rect)
			arranged = append(arranged, dialogBodyChildArrange{facet: children[i].facet, bounds: rect})
			x += children[i].size.W
		}
		return arranged
	case DialogContentLayoutGrid:
		cfg := g.gridConfig()
		policy := layoutgrid.New(cfg)
		gridChildren := g.gridChildren(children)
		arranged, err := policy.Arrange(gridChildren, bounds)
		if err != nil {
			return nil
		}
		measureByID := make(map[facet.FacetID]dialogBodyChildMeasure, len(children))
		for i := range children {
			measureByID[children[i].facet.ID()] = children[i]
		}
		out := make([]dialogBodyChildArrange, 0, len(arranged))
		for i := range arranged {
			child, ok := measureByID[arranged[i].FacetID]
			if !ok || child.facet == nil || child.facet.LayoutRole() == nil {
				continue
			}
			child.facet.LayoutRole().Arrange(ctx, arranged[i].Bounds)
			out = append(out, dialogBodyChildArrange{facet: child.facet, bounds: arranged[i].Bounds})
		}
		return out
	default:
		y := bounds.Min.Y
		arranged := make([]dialogBodyChildArrange, 0, len(children))
		for i := range children {
			if i > 0 {
				y += g.parent.cachedRowGap
			}
			rect := gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), children[i].size.H)
			children[i].facet.LayoutRole().Arrange(ctx, rect)
			arranged = append(arranged, dialogBodyChildArrange{facet: children[i].facet, bounds: rect})
			y += children[i].size.H
		}
		return arranged
	}
}

func (g *dialogBodyGroup) gridConfig() layoutgrid.Config {
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

func (g *dialogBodyGroup) gridChildren(children []dialogBodyChildMeasure) []layoutgrid.Child {
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

func (g *dialogBodyGroup) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if g == nil || g.parent == nil || bounds.IsEmpty() || !g.parent.Open.Get() {
		return nil
	}
	if g.cachedTextFacet == nil && len(g.cachedChildren) == 0 {
		return nil
	}
	cmds := make([]gfx.Command, 0, 16)
	if !g.cachedBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: g.cachedBounds})
		if g.cachedTextFacet != nil && !g.cachedTextBounds.IsEmpty() {
			if projected := g.cachedTextFacet.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: g.cachedTextBounds, ContentScale: contentScale}); projected != nil {
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

func (g *dialogBodyGroup) bodyPlacement(index int, contract facet.GroupChildContract, grid facet.GridPlacement) facet.Placement {
	switch g.cachedLayoutMode {
	case DialogContentLayoutGrid:
		placement := grid
		if placement == (facet.GridPlacement{}) {
			placement = facet.GridPlacement{ColStart: 0, RowStart: index, ColSpan: 1, RowSpan: 1}
		}
		return facet.Placement{Mode: facet.PlacementGrid, Grid: placement, Align: facet.AlignStretch}
	case DialogContentLayoutHorizontal, DialogContentLayoutVertical:
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
			if g.cachedLayoutMode == DialogContentLayoutVertical {
				placement.ColStart = 0
				placement.RowStart = index
			}
		}
		return facet.Placement{Mode: facet.PlacementGrid, Grid: placement, Align: facet.AlignStretch}
	default:
		return facet.Placement{Mode: facet.PlacementGrid, Grid: grid, Align: facet.AlignStretch}
	}
}

type dialogBodyChildMeasure struct {
	facet     *facet.Facet
	size      gfx.Size
	grid      facet.GridPlacement
	markID    facet.MarkID
	zPriority int32
}

type dialogBodyChildArrange struct {
	facet  *facet.Facet
	bounds gfx.Rect
}

type dialogBodyGroupPolicy struct {
	group *dialogBodyGroup
}

func (p dialogBodyGroupPolicy) Kind() facet.GroupLayoutKind {
	if p.group == nil {
		return facet.GroupLayoutLinearVertical
	}
	return p.group.groupKind()
}

func (p dialogBodyGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.group == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.group.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}})
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p dialogBodyGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
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

func dialogBodyGroupChild(base *facet.Facet, markID facet.MarkID, order int, placement facet.Placement) facet.GroupChild {
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

func flexibleTracks(count int) []layoutgrid.TrackDef {
	if count < 1 {
		count = 1
	}
	out := make([]layoutgrid.TrackDef, count)
	for i := range out {
		out[i] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFlex, Value: 1, Min: 0}
	}
	return out
}

func resolvedDialogContentWidth(d *Dialog) float32 {
	if d == nil {
		return 0
	}
	width := d.cachedTitleFacet.Base().LayoutRole().MeasuredSize.W
	if d.cachedBodyGroup != nil {
		width = mathutil.Max(width, d.cachedBodyGroup.Base().LayoutRole().MeasuredSize.W)
	}
	if d.cachedActionsFacet != nil {
		width = mathutil.Max(width, d.cachedActionsFacet.Base().LayoutRole().MeasuredSize.W)
	}
	if d.cachedCloseButton != nil {
		width = mathutil.Max(width, d.cachedCloseButton.Base().LayoutRole().MeasuredSize.W)
	}
	return width
}

func resolvedDialogContentHeight(d *Dialog) float32 {
	if d == nil {
		return 0
	}
	height := d.cachedTitleFacet.Base().LayoutRole().MeasuredSize.H
	if d.cachedBodyGroup != nil {
		if d.cachedBodyGroup.Base().LayoutRole().MeasuredSize.H > 0 {
			height += d.cachedRowGap + d.cachedBodyGroup.Base().LayoutRole().MeasuredSize.H
		}
	}
	if d.cachedActionsFacet != nil && d.cachedActionsFacet.Base().LayoutRole().MeasuredSize.H > 0 {
		height += d.cachedRowGap + d.cachedActionsFacet.Base().LayoutRole().MeasuredSize.H
	}
	if d.cachedCloseButton != nil {
		height = mathutil.Max(height, d.cachedCloseButton.Base().LayoutRole().MeasuredSize.H)
	}
	return height
}

func resolvedDialogMinWidth(d *Dialog) float32 {
	if d == nil {
		return 0
	}
	return mathutil.Max(280, d.cachedPadX*2+resolvedDialogContentWidth(d))
}

func resolvedDialogMinHeight(d *Dialog) float32 {
	if d == nil {
		return 0
	}
	return mathutil.Max(160, d.cachedPadY*2+resolvedDialogContentHeight(d))
}
