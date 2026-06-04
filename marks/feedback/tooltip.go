package feedback

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uifeedback"
)

const (
	tooltipMarkIDRoot        facet.MarkID = 1
	tooltipMarkIDSurface     facet.MarkID = 2
	tooltipMarkIDContent     facet.MarkID = 3
	tooltipMarkIDAnchorArrow facet.MarkID = 4
)

// Tooltip implements the feedback.tooltip canonical mark.
type Tooltip struct {
	marks.Core

	textRole facet.TextRole

	Dismissed signal.Signal[signal.Unit]

	Content   marks.Binding[string]
	Open      marks.Binding[bool]
	Disabled  marks.Binding[bool]
	Placement facet.AnchorPlacement

	hovered bool
	pressed bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.FeedbackTooltipSlots
	cachedBounds           gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedContentBounds    gfx.Rect
	cachedArrowBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedArrowSize        float32
	cachedWritingDirection facet.WritingDirection
	cachedContentFacet     *primitive.Text
}

var _ facet.FacetImpl = (*Tooltip)(nil)
var _ layout.AnchorExporter = (*Tooltip)(nil)
var _ marks.Mark = (*Tooltip)(nil)

// NewTooltip constructs a feedback.tooltip mark with canonical defaults.
func NewTooltip(content string) *Tooltip {
	t := &Tooltip{
		Content:   marks.Const(content),
		Open:      marks.Const(true),
		Disabled:  marks.Const(false),
		Placement: facet.AnchorPlacement{Side: facet.AnchorAbove},
	}
	t.Core.Facet = facet.NewFacet()
	t.AddBinding(t.Content)
	t.AddBinding(t.Open)
	t.AddBinding(t.Disabled)

	t.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutLinearVertical,
		Policy:   tooltipGroupPolicy{tooltip: t},
		Children: t,
		Overflow: facet.OverflowClip,
		Clipping: facet.GroupClipBounds,
	}
	t.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := t.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionTruncate,
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
	t.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.Layout.ArrangedBounds = bounds
		t.arrange(ctx, bounds)
	}
	t.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := t.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	t.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return t.hitTest(p) }
	t.Input.OnPointer = func(e facet.PointerEvent) bool { return t.onPointer(e) }
	t.Input.OnKey = func(e facet.KeyEvent) bool { return t.onKey(e) }
	t.Input.OnDismiss = func(e facet.DismissEvent) bool { return t.onDismiss(e) }
	t.textRole.IMEEnabled = false
	t.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return t.buildCommands(t.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	t.RegisterRoles()
	t.AddRole(&t.textRole)
	t.syncChildren()
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Tooltip) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

// Descriptor satisfies marks.Mark.
func (t *Tooltip) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "feedback", TypeName: "tooltip"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *Tooltip) AccessibilityRole() string { return "tooltip" }

// AccessibleName reports the semantic name source required by the spec.
func (t *Tooltip) AccessibleName() string {
	if t == nil {
		return ""
	}
	return strings.TrimSpace(t.Content.Get())
}

// Children returns the tooltip's immediate child list.
func (t *Tooltip) Children() []facet.GroupChild {
	if t == nil || !t.Open.Get() {
		return nil
	}
	t.syncChildren()
	if t.cachedContentFacet == nil {
		return nil
	}
	return []facet.GroupChild{tooltipGroupChild(t.cachedContentFacet.Base(), tooltipMarkIDContent, 0)}
}

// ExportAnchors publishes the tooltip anchor set.
func (t *Tooltip) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if t == nil {
		return nil
	}
	bounds := t.Layout.ArrangedBounds
	out := t.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if t.cachedContentFacet != nil && t.cachedContentFacet.Base().LayoutRole() != nil && t.cachedContentFacet.Base().LayoutRole().ArrangedBounds.Width() > 0 {
		contentBounds := t.cachedContentFacet.Base().LayoutRole().ArrangedBounds
		if tr := t.cachedContentFacet.Base().TextRole(); tr != nil && tr.Layout != nil {
			out["baseline"] = gfx.Point{X: contentBounds.Min.X, Y: contentBounds.Min.Y + tr.Layout.Baseline}
		} else {
			out["baseline"] = gfx.Point{X: contentBounds.Min.X, Y: contentBounds.Min.Y}
		}
	}
	switch t.Placement.Side {
	case facet.AnchorBelow:
		out["content_anchor"] = gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: bounds.Min.Y}
	case facet.AnchorLeft:
		out["content_anchor"] = gfx.Point{X: bounds.Max.X, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5}
	case facet.AnchorRight:
		out["content_anchor"] = gfx.Point{X: bounds.Min.X, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5}
	case facet.AnchorCenter:
		out["content_anchor"] = gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5}
	default:
		out["content_anchor"] = gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: bounds.Max.Y}
	}
	return out
}

// OnAttach delegates to Core.
func (t *Tooltip) OnAttach(ctx facet.AttachContext) { t.Core.OnAttach() }

// OnActivate delegates to Core.
func (t *Tooltip) OnActivate() { t.Core.OnActivate() }

// OnDeactivate delegates to Core.
func (t *Tooltip) OnDeactivate() { t.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (t *Tooltip) OnDetach() {
	t.Core.OnDetach()
	t.cachedTokens = theme.Tokens{}
	t.cachedRecipe = shared.FeedbackTooltipSlots{}
	t.cachedBounds = gfx.Rect{}
	t.cachedSurfaceBounds = gfx.Rect{}
	t.cachedContentBounds = gfx.Rect{}
	t.cachedArrowBounds = gfx.Rect{}
	t.cachedPadX = 0
	t.cachedPadY = 0
	t.cachedGap = 0
	t.cachedArrowSize = 0
	t.cachedContentFacet = nil
}

func (t *Tooltip) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Facet.Invalidate(flags)
}

func (t *Tooltip) syncChildren() {
	if t == nil {
		return
	}
	content := strings.TrimSpace(t.Content.Get())
	if content == "" {
		t.cachedContentFacet = nil
		return
	}
	if t.cachedContentFacet == nil {
		t.cachedContentFacet = primitive.NewText(marks.Const(content))
	} else {
		t.cachedContentFacet.Content = marks.Const(content)
		t.cachedContentFacet.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	t.cachedContentFacet.Typography = marks.Const(theme.TextBodyS)
	t.cachedContentFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	t.cachedContentFacet.Alignment = marks.Const(text.AlignCenter)
}

func (t *Tooltip) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if !t.Open.Get() {
		size := constraints.Constrain(gfx.Size{})
		t.Layout.MeasuredSize = size
		t.Layout.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		return t.Layout.MeasuredResult
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uifeedback.ResolveTooltipRecipe(style, t.tooltipVariant())
	t.cachedTokens = resolved.TokenSet()
	t.cachedRecipe = slots
	t.cachedWritingDirection = ctx.WritingDirection
	textStyle := resolved.TextStyle(theme.TextBodyS)
	if textStyle.Size <= 0 {
		textStyle = resolved.TextStyle(theme.TextBodyM)
	}
	fontSize := textStyle.Size
	if fontSize <= 0 {
		fontSize = theme.DefaultTokens().Typography.BodySmall.Size
	}
	lineHeight := textStyle.LineHeight
	if lineHeight <= 0 {
		lineHeight = maxFloat(fontSize*1.35, 1)
	}
	t.cachedPadX = maxFloat(maxFloat(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingM))), fontSize*0.55)
	t.cachedPadY = maxFloat(maxFloat(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS))), lineHeight*0.12)
	t.cachedGap = maxFloat(resolved.Density.Scale(4), float32(resolved.Spacing(theme.SpacingXS)))
	t.cachedArrowSize = maxFloat(maxFloat(resolved.Density.Scale(8), float32(resolved.Spacing(theme.SpacingS))), fontSize*0.5)
	t.syncChildren()
	if t.cachedContentFacet == nil || t.cachedContentFacet.Base().LayoutRole() == nil {
		size := constraints.Constrain(gfx.Size{})
		t.Layout.MeasuredSize = size
		t.Layout.MeasuredResult = facet.MeasureResult{
			Size:        size,
			Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
			Constraints: constraints,
		}
		return t.Layout.MeasuredResult
	}
	contentConstraints := facet.Constraints{MaxSize: gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H}}
	if contentConstraints.MaxSize.W > 0 {
		contentConstraints.MaxSize.W = maxFloat(0, contentConstraints.MaxSize.W-t.cachedPadX*2)
	}
	if contentConstraints.MaxSize.H > 0 {
		contentConstraints.MaxSize.H = maxFloat(0, contentConstraints.MaxSize.H-t.cachedPadY*2)
	}
	contentSize := t.cachedContentFacet.Base().LayoutRole().Measure(facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}, contentConstraints).Size
	minContentWidth := contentSize.W + fontSize*0.75
	minContentHeight := contentSize.H
	size := gfx.Size{
		W: maxFloat(contentSize.W+t.cachedPadX*2, minContentWidth+t.cachedPadX*2),
		H: maxFloat(contentSize.H+t.cachedPadY*2, minContentHeight+t.cachedPadY*2),
	}
	switch t.Placement.Side {
	case facet.AnchorLeft, facet.AnchorRight:
		size.W += t.cachedArrowSize
	default:
		size.H += t.cachedArrowSize
	}
	measured := constraints.Constrain(size)
	t.Layout.MeasuredSize = measured
	t.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return t.Layout.MeasuredResult
}

func (t *Tooltip) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	t.cachedBounds = bounds
	t.cachedSurfaceBounds = gfx.Rect{}
	t.cachedContentBounds = gfx.Rect{}
	t.cachedArrowBounds = gfx.Rect{}
	t.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() || !t.Open.Get() {
		return
	}
	t.syncChildren()
	contentFacet := t.cachedContentFacet
	if contentFacet == nil || contentFacet.Base().LayoutRole() == nil {
		return
	}

	w := t.Layout.MeasuredSize.W
	h := t.Layout.MeasuredSize.H
	if w <= 0 || h <= 0 {
		w = bounds.Width()
		h = bounds.Height()
	}
	if w > bounds.Width() {
		w = bounds.Width()
	}
	if h > bounds.Height() {
		h = bounds.Height()
	}
	activeBounds := text.CenterRect(bounds, w, h)

	arrow := t.cachedArrowSize
	surface := activeBounds
	switch t.Placement.Side {
	case facet.AnchorBelow:
		surface = gfx.RectFromXYWH(activeBounds.Min.X, activeBounds.Min.Y+arrow, activeBounds.Width(), activeBounds.Height()-arrow)
		t.cachedArrowBounds = gfx.RectFromXYWH(activeBounds.Min.X+activeBounds.Width()*0.5-arrow*0.5, activeBounds.Min.Y, arrow, arrow)
	case facet.AnchorLeft:
		surface = gfx.RectFromXYWH(activeBounds.Min.X, activeBounds.Min.Y, activeBounds.Width()-arrow, activeBounds.Height())
		t.cachedArrowBounds = gfx.RectFromXYWH(activeBounds.Max.X-arrow, activeBounds.Min.Y+activeBounds.Height()*0.5-arrow*0.5, arrow, arrow)
	case facet.AnchorRight:
		surface = gfx.RectFromXYWH(activeBounds.Min.X+arrow, activeBounds.Min.Y, activeBounds.Width()-arrow, activeBounds.Height())
		t.cachedArrowBounds = gfx.RectFromXYWH(activeBounds.Min.X, activeBounds.Min.Y+activeBounds.Height()*0.5-arrow*0.5, arrow, arrow)
	default:
		surface = gfx.RectFromXYWH(activeBounds.Min.X, activeBounds.Min.Y, activeBounds.Width(), activeBounds.Height()-arrow)
		t.cachedArrowBounds = gfx.RectFromXYWH(activeBounds.Min.X+activeBounds.Width()*0.5-arrow*0.5, activeBounds.Max.Y-arrow, arrow, arrow)
	}
	t.cachedSurfaceBounds = surface
	contentBounds := surface.Inset(t.cachedPadX, t.cachedPadY)
	if contentBounds.IsEmpty() {
		contentBounds = surface
	}
	contentFacet.Base().LayoutRole().Arrange(ctx, contentBounds)
	t.cachedContentBounds = contentBounds
}

func (t *Tooltip) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if t == nil || bounds.IsEmpty() || !t.Open.Get() {
		return nil
	}
	style, slots := t.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateSelected
	switch {
	case t.Disabled.Get():
		state = theme.StateDisabled
	case t.pressed:
		state = theme.StatePressed
	case t.hovered:
		state = theme.StateHover
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.TooltipSurface.Resolve(state, tokens)
	arrow := slots.AnchorArrow.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 16)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(t.cachedSurfaceBounds, float32(tokens.Radius.MD)), surface)...)
	}
	if !isTransparentMaterial(arrow) && !t.cachedArrowBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(tooltipArrowPath(t.cachedArrowBounds, t.Placement.Side), arrow)...)
	}
	if !t.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: t.cachedSurfaceBounds})
		if contentFacet := t.cachedContentFacet; contentFacet != nil && t.cachedContentBounds.Width() > 0 {
			if projected := contentFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       t.cachedContentBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	return cmds
}

func (t *Tooltip) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.FeedbackTooltipSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, t.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uifeedback.ResolveTooltipRecipe(style, t.tooltipVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
}

func (t *Tooltip) tooltipVariant() uifeedback.TooltipVariant {
	if t != nil && t.Disabled.Get() {
		return uifeedback.TooltipDisabled
	}
	if t != nil && !t.Open.Get() {
		return uifeedback.TooltipDefault
	}
	if t != nil && t.pressed {
		return uifeedback.TooltipActive
	}
	if t != nil && t.hovered {
		return uifeedback.TooltipHover
	}
	return uifeedback.TooltipOpen
}

func (t *Tooltip) hitTest(p gfx.Point) facet.HitResult {
	if t == nil || !t.Open.Get() || t.cachedBounds.IsEmpty() || !t.cachedBounds.Contains(p) {
		return facet.HitResult{}
	}
	switch {
	case !t.cachedArrowBounds.IsEmpty() && t.cachedArrowBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: tooltipMarkIDAnchorArrow}
	case !t.cachedContentBounds.IsEmpty() && t.cachedContentBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: tooltipMarkIDContent}
	case !t.cachedSurfaceBounds.IsEmpty() && t.cachedSurfaceBounds.Contains(p):
		return facet.HitResult{Hit: true, MarkID: tooltipMarkIDSurface}
	default:
		return facet.HitResult{Hit: true, MarkID: tooltipMarkIDRoot}
	}
}

func (t *Tooltip) onPointer(e facet.PointerEvent) bool {
	if t == nil || t.Disabled.Get() || !t.Open.Get() {
		return false
	}
	if !t.cachedBounds.Contains(e.Position) {
		if e.Kind == platform.PointerLeave {
			t.hovered = false
			t.pressed = false
			t.invalidate(facet.DirtyProjection)
		}
		return false
	}
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		if !t.hovered {
			t.hovered = true
			t.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerPress:
		if e.Button == platform.PointerLeft {
			t.pressed = true
			t.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerRelease:
		if t.pressed {
			t.pressed = false
			t.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.PointerLeave:
		t.hovered = false
		t.pressed = false
		t.invalidate(facet.DirtyProjection)
		return true
	}
	return false
}

func (t *Tooltip) onKey(e facet.KeyEvent) bool {
	if t == nil || t.Disabled.Get() || !t.Open.Get() {
		return false
	}
	if e.Kind == platform.KeyPress && e.Key == platform.KeyEscape {
		t.openFalseAndDismiss()
		return true
	}
	return false
}

func (t *Tooltip) onDismiss(e facet.DismissEvent) bool {
	_ = e
	if t == nil || t.Disabled.Get() || !t.Open.Get() {
		return false
	}
	t.openFalseAndDismiss()
	return true
}

func (t *Tooltip) openFalseAndDismiss() {
	if t == nil {
		return
	}
	t.Open = marks.Const(false)
	t.hovered = false
	t.pressed = false
	t.Dismissed.Emit(signal.Unit{})
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func tooltipGroupChild(base *facet.Facet, markID facet.MarkID, order int) facet.GroupChild {
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

func tooltipArrowPath(bounds gfx.Rect, side facet.AnchorSide) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	midX := (bounds.Min.X + bounds.Max.X) * 0.5
	midY := (bounds.Min.Y + bounds.Max.Y) * 0.5
	switch side {
	case facet.AnchorBelow:
		return gfx.NewPath().
			MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: midX, Y: bounds.Max.Y}).
			Close().
			Build()
	case facet.AnchorLeft:
		return gfx.NewPath().
			MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: bounds.Max.X, Y: midY}).
			LineTo(gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y}).
			Close().
			Build()
	case facet.AnchorRight:
		return gfx.NewPath().
			MoveTo(gfx.Point{X: bounds.Min.X, Y: midY}).
			LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y}).
			Close().
			Build()
	default:
		return gfx.NewPath().
			MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y}).
			LineTo(gfx.Point{X: midX, Y: bounds.Max.Y}).
			Close().
			Build()
	}
}

type tooltipGroupPolicy struct {
	tooltip *Tooltip
}

func (tooltipGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (p tooltipGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.tooltip == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.tooltip.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p tooltipGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.tooltip == nil {
		return nil, nil
	}
	p.tooltip.arrange(ctx.ArrangeContext, ctx.Bounds)
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
