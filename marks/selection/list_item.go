package selection

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	listItemMarkIDRoot               facet.MarkID = 1
	listItemMarkIDItemContainer      facet.MarkID = 2
	listItemMarkIDLeadingIcon        facet.MarkID = 3
	listItemMarkIDLabel              facet.MarkID = 4
	listItemMarkIDSupportingText     facet.MarkID = 5
	listItemMarkIDSelectionIndicator facet.MarkID = 6
	listItemMarkIDFocusRing          facet.MarkID = 7
)

// ListItem implements the selection.list_item canonical mark.
type ListItem struct {
	marks.Core

	Activated signal.Signal[signal.Unit]

	LeadingIconRef         marks.Binding[string]
	Label                  marks.Binding[string]
	SupportingText         marks.Binding[string]
	Variant                marks.Binding[uiinput.ListItemVariant]
	Selected               marks.Binding[bool]
	Active                 marks.Binding[bool]
	Disabled               marks.Binding[bool]
	ShowLabel              marks.Binding[bool]
	ShowContainer          marks.Binding[bool]
	ShowLeadingIcon        marks.Binding[bool]
	ShowSelectionIndicator marks.Binding[bool]
	ShowFocusRing          marks.Binding[bool]

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.ListItemSlots
	cachedRootBounds       gfx.Rect
	cachedItemBounds       gfx.Rect
	cachedLeadingBounds    gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedSupportingBounds gfx.Rect
	cachedSelectionBounds  gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedLabelLayout      *text.TextLayout
	cachedSupportingLayout *text.TextLayout
	cachedLabelStyle       text.TextStyle
	cachedSupportingStyle  text.TextStyle
	cachedRadius           float32
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*ListItem)(nil)
var _ layout.AnchorExporter = (*ListItem)(nil)
var _ marks.Mark = (*ListItem)(nil)

// NewListItem constructs a selection.list_item mark with canonical defaults.
func NewListItem(label marks.Binding[string]) *ListItem {
	li := &ListItem{
		Label:                  label,
		LeadingIconRef:         marks.Const(""),
		SupportingText:         marks.Const(""),
		Variant:                marks.Const(uiinput.ListItemStandard),
		Selected:               marks.Const(false),
		Active:                 marks.Const(false),
		Disabled:               marks.Const(false),
		ShowLabel:              marks.Const(true),
		ShowContainer:          marks.Const(true),
		ShowLeadingIcon:        marks.Const(true),
		ShowSelectionIndicator: marks.Const(true),
		ShowFocusRing:          marks.Const(true),
	}
	li.Core.Facet = facet.NewFacet()
	li.AddBinding(li.Label)
	li.AddBinding(li.LeadingIconRef)
	li.AddBinding(li.SupportingText)
	li.AddBinding(li.Variant)
	li.AddBinding(li.Selected)
	li.AddBinding(li.Active)
	li.AddBinding(li.Disabled)
	li.AddBinding(li.ShowLabel)
	li.AddBinding(li.ShowContainer)
	li.AddBinding(li.ShowLeadingIcon)
	li.AddBinding(li.ShowSelectionIndicator)
	li.AddBinding(li.ShowFocusRing)

	li.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: listItemGroupPolicy{},
	}
	li.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsLinear,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := li.measureIntrinsic(ctx, constraints)
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
	li.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return li.measure(ctx, constraints)
	}
	li.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		li.Layout.ArrangedBounds = bounds
		li.arrange(ctx, bounds)
	}
	li.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return li.hitTest(p) }
	li.Input.OnPointer = func(e facet.PointerEvent) bool { return li.onPointer(e) }
	li.Input.OnKey = func(e facet.KeyEvent) bool { return li.onKey(e) }
	li.Focus.Focusable = func() bool { return !li.Disabled.Get() }
	li.Focus.TabIndex = 0
	li.Focus.OnFocusGained = func() { li.onFocusGained() }
	li.Focus.OnFocusLost = func() { li.onFocusLost() }
	li.textRole.IMEEnabled = false
	li.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return li.buildCommands(li.Layout.ArrangedBounds, ctx.Runtime)
	}
	li.RegisterRoles()
	li.AddRole(&li.textRole)
	return li
}

// Base satisfies facet.FacetImpl.
func (li *ListItem) Base() *facet.Facet {
	li.Facet.BindImpl(li)
	return &li.Facet
}

// Descriptor satisfies marks.Mark.
func (li *ListItem) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "selection", TypeName: "list_item"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (li *ListItem) AccessibilityRole() string { return "option" }

// AccessibleName reports the semantic name source required by the spec.
func (li *ListItem) AccessibleName() string {
	if li == nil {
		return ""
	}
	return li.Label.Get()
}

// ExportAnchors publishes the list-item anchor set.
func (li *ListItem) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := li.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	anchors := li.Core.DefaultAnchors(bounds, ctx)
	if li.textRole.Layout != nil {
		anchors["baseline"] = gfx.Point{X: li.cachedLabelBounds.Min.X, Y: li.cachedLabelBounds.Min.Y + li.textRole.Layout.Baseline}
	} else {
		anchors["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return anchors
}

// Children returns the facet's immediate child list.
func (li *ListItem) Children() []facet.GroupChild { return nil }

func (li *ListItem) OnAttach(ctx facet.AttachContext) { li.Core.OnAttach() }
func (li *ListItem) OnActivate()                      { li.Core.OnActivate() }
func (li *ListItem) OnDeactivate()                    { li.Core.OnDeactivate() }
func (li *ListItem) OnDetach() {
	li.Core.OnDetach()
	li.cachedTokens = theme.Tokens{}
	li.cachedRecipe = shared.ListItemSlots{}
	li.cachedRootBounds = gfx.Rect{}
	li.cachedItemBounds = gfx.Rect{}
	li.cachedLeadingBounds = gfx.Rect{}
	li.cachedLabelBounds = gfx.Rect{}
	li.cachedSupportingBounds = gfx.Rect{}
	li.cachedSelectionBounds = gfx.Rect{}
	li.cachedFocusBounds = gfx.Rect{}
	li.cachedLabelLayout = nil
	li.cachedSupportingLayout = nil
	li.cachedLabelStyle = text.TextStyle{}
	li.cachedSupportingStyle = text.TextStyle{}
	li.cachedRadius = 0
	li.cachedPadX = 0
	li.cachedPadY = 0
	li.cachedGap = 0
}

func (li *ListItem) invalidate(flags facet.DirtyFlags) {
	if li == nil {
		return
	}
	li.Facet.Invalidate(flags)
}

func (li *ListItem) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveListItemRecipe(style, li.Variant.Get())
	li.cachedTokens = resolved.TokenSet()
	li.cachedRecipe = slots
	li.cachedWritingDirection = ctx.WritingDirection
	li.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	li.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(10))
	li.cachedGap = float32(resolved.Spacing(theme.SpacingXS))
	li.cachedRadius = float32(resolved.Radius(theme.RadiusS))
	shaper := li.newShaper(ctx.Runtime)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(320)
	}
	labelLayout := (*text.TextLayout)(nil)
	supportingLayout := (*text.TextLayout)(nil)
	if shaper != nil {
		shaper.SetContentScale(ctx.ContentScale)
		if li.ShowLabel.Get() {
			labelLayout = shaper.ShapeTruncated(li.Label.Get(), resolved.TextStyle(theme.TextBodyM), maxWidth)
		}
		if strings.TrimSpace(li.SupportingText.Get()) != "" {
			supportingLayout = shaper.ShapeTruncated(li.SupportingText.Get(), resolved.TextStyle(theme.TextBodyS), maxWidth)
		}
	}
	li.cachedLabelLayout = labelLayout
	li.cachedSupportingLayout = supportingLayout
	li.cachedLabelStyle = resolved.TextStyle(theme.TextBodyM)
	li.cachedSupportingStyle = resolved.TextStyle(theme.TextBodyS)
	li.textRole.Layout = labelLayout
	li.textRole.Selection = text.TextRange{}
	li.textRole.CaretVisible = false
	li.textRole.CaretPosition = text.TextPosition{}
	contentW := text.Width(supportingLayout)
	if li.ShowLabel.Get() {
		contentW = mathutil.Max(contentW, text.Width(labelLayout))
	}
	leadingReserve := float32(0)
	if li.ShowLeadingIcon.Get() && li.LeadingIconRef.Get() != "" {
		leadingReserve = mathutil.Max(resolved.Density.Scale(16), resolved.Density.Scale(16)*0.42) + li.cachedGap
	}
	selectionReserve := float32(0)
	if li.ShowSelectionIndicator.Get() {
		selectionReserve = mathutil.Max(resolved.Density.Scale(24), resolved.Density.Scale(12))
	}
	minWidth := resolved.Density.Scale(160)
	minHeight := resolved.Density.Scale(32)
	if !li.ShowLabel.Get() && !li.ShowContainer.Get() && !li.ShowSelectionIndicator.Get() {
		minWidth = mathutil.Max(resolved.Density.Scale(56), leadingReserve+li.cachedPadX*2)
		minHeight = mathutil.Max(resolved.Density.Scale(56), resolved.Density.Scale(20)+li.cachedPadY*2)
	}
	if !li.ShowContainer.Get() && !li.ShowLeadingIcon.Get() && !li.ShowSelectionIndicator.Get() && li.ShowLabel.Get() {
		minWidth = contentW + li.cachedPadX*2
		minHeight = text.Height(supportingLayout) + li.cachedPadY*2
		if supportingLayout != nil {
			minHeight += li.cachedGap
		}
		if labelLayout != nil {
			minHeight += text.Height(labelLayout)
		}
	}
	itemSize := gfx.Size{
		W: mathutil.Max(minWidth, contentW+li.cachedPadX*2+leadingReserve+selectionReserve),
		H: mathutil.Max(minHeight, text.Height(supportingLayout)+li.cachedPadY*2+li.cachedGap),
	}
	if itemSize.W <= 0 {
		itemSize.W = resolved.Density.Scale(160)
	}
	if itemSize.H <= 0 {
		itemSize.H = resolved.Density.Scale(32)
	}
	li.Layout.MeasuredSize = itemSize
	li.Layout.MeasuredResult = facet.MeasureResult{
		Size: itemSize,
		Intrinsic: facet.IntrinsicSize{
			Min:       itemSize,
			Preferred: itemSize,
			Max:       itemSize,
		},
		Constraints: constraints,
	}
	return li.Layout.MeasuredResult
}

func (li *ListItem) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return li.measure(ctx, constraints).Size
}

func (li *ListItem) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	li.cachedRootBounds = bounds
	li.cachedItemBounds = gfx.Rect{}
	li.cachedLeadingBounds = gfx.Rect{}
	li.cachedLabelBounds = gfx.Rect{}
	li.cachedSupportingBounds = gfx.Rect{}
	li.cachedSelectionBounds = gfx.Rect{}
	li.cachedFocusBounds = gfx.Rect{}
	li.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	leadingSize := mathutil.Max(resolved.Density.Scale(16), bounds.Height()*0.42)
	indicatorSize := mathutil.Max(8, bounds.Height()*0.18)
	inner := bounds.Inset(li.cachedPadX, li.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	labelH := float32(0)
	if li.ShowLabel.Get() {
		labelH = text.Height(li.cachedLabelLayout)
	}
	supportingH := text.Height(li.cachedSupportingLayout)
	contentH := labelH + supportingH
	if supportingH > 0 {
		contentH += li.cachedGap
	}
	textY := text.AlignRectY(gfx.RectFromXYWH(inner.Min.X, inner.Min.Y, inner.Width(), contentH), inner.Min.Y, inner.Height()).Min.Y
	textX := inner.Min.X
	labelW := float32(0)
	if li.ShowLabel.Get() {
		labelW = text.Width(li.cachedLabelLayout)
	}
	if labelW <= 0 {
		labelW = mathutil.Max(0, inner.Width()-li.cachedPadX*2)
	}
	leadingReserve := float32(0)
	if li.ShowLeadingIcon.Get() && li.LeadingIconRef.Get() != "" {
		leadingReserve = leadingSize + li.cachedGap
	}
	textWidth := mathutil.Max(0, inner.Width()-leadingReserve)
	if li.cachedWritingDirection == facet.WritingDirectionRTL {
		textX = inner.Max.X - labelW
		if li.ShowLeadingIcon.Get() && li.LeadingIconRef.Get() != "" {
			textX = inner.Max.X - li.cachedPadX - leadingReserve - labelW
		}
	} else if li.ShowLeadingIcon.Get() && li.LeadingIconRef.Get() != "" {
		textX = inner.Min.X + leadingReserve
	}
	textRects := layout.ArrangeVerticalFlow(
		gfx.RectFromXYWH(textX, textY, textWidth, contentH),
		0,
		li.cachedGap,
		[]gfx.Size{
			{W: textWidth, H: labelH},
			{W: textWidth, H: supportingH},
		},
		li.cachedWritingDirection == facet.WritingDirectionRTL,
	)
	li.cachedLabelBounds = gfx.Rect{}
	li.cachedSupportingBounds = gfx.Rect{}
	if li.ShowLabel.Get() && li.cachedLabelLayout != nil {
		li.cachedLabelBounds = textRects[0]
	}
	if supportingH > 0 {
		li.cachedSupportingBounds = textRects[1]
	}
	if li.ShowSelectionIndicator.Get() {
		indicatorBounds := text.AlignRectY(gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, indicatorSize, indicatorSize), bounds.Min.Y, bounds.Height())
		if li.cachedWritingDirection == facet.WritingDirectionRTL {
			li.cachedSelectionBounds = gfx.RectFromXYWH(bounds.Min.X+li.cachedPadX, indicatorBounds.Min.Y, indicatorSize, indicatorSize)
		} else {
			li.cachedSelectionBounds = gfx.RectFromXYWH(bounds.Max.X-li.cachedPadX-indicatorSize, indicatorBounds.Min.Y, indicatorSize, indicatorSize)
		}
	}
	if li.ShowLeadingIcon.Get() && li.LeadingIconRef.Get() != "" {
		if li.cachedWritingDirection == facet.WritingDirectionRTL {
			li.cachedLeadingBounds = text.AlignRectY(gfx.RectFromXYWH(bounds.Max.X-li.cachedPadX-leadingSize, bounds.Min.Y, leadingSize, leadingSize), bounds.Min.Y, bounds.Height())
		} else {
			li.cachedLeadingBounds = text.AlignRectY(gfx.RectFromXYWH(bounds.Min.X+li.cachedPadX, bounds.Min.Y, leadingSize, leadingSize), bounds.Min.Y, bounds.Height())
		}
	}
	li.cachedItemBounds = inner
	li.cachedFocusBounds = bounds.Inset(mathutil.Max(1, bounds.Height()*0.08), mathutil.Max(1, bounds.Height()*0.08))
}

func (li *ListItem) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ListItemSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: li.cachedTokens}, li.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, li.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveListItemRecipe(style, li.Variant.Get())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: li.cachedTokens}, li.cachedRecipe
}

func (li *ListItem) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if li == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := li.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := li.interactionState()
	if li.Selected.Get() {
		state = theme.StateSelected
	}
	if li.Active.Get() {
		state = theme.StateFocused
	}
	root := slots.Root.Resolve(state, tokens)
	container := slots.ItemContainer.Resolve(state, tokens)
	label := slots.Label.Resolve(state, tokens)
	supporting := slots.SupportingText.Resolve(state, tokens)
	indicator := slots.SelectionIndicator.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	cmds := make([]gfx.Command, 0, 24)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if li.ShowContainer.Get() && !theme.IsTransparentMaterial(container) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(bounds, li.cachedRadius), container)...)
	}
	if li.ShowLabel.Get() && !theme.IsTransparentMaterial(label) && li.cachedLabelLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(li.cachedLabelLayout, li.cachedLabelBounds, gfx.SolidBrush(theme.MaterialColor(label)))...)
	}
	if !theme.IsTransparentMaterial(supporting) && li.cachedSupportingLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(li.cachedSupportingLayout, li.cachedSupportingBounds, gfx.SolidBrush(theme.MaterialColor(supporting)))...)
	}
	if li.ShowSelectionIndicator.Get() && li.Selected.Get() && !theme.IsTransparentMaterial(indicator) && !li.cachedSelectionBounds.IsEmpty() {
		r := mathutil.Max(3, li.cachedSelectionBounds.Width()*0.5)
		center := gfx.Point{X: li.cachedSelectionBounds.Min.X + li.cachedSelectionBounds.Width()*0.5, Y: li.cachedSelectionBounds.Min.Y + li.cachedSelectionBounds.Height()*0.5}
		cmds = append(cmds, theme.MaterialCommands(gfx.CirclePath(center, r), indicator)...)
	}
	if li.ShowFocusRing.Get() && li.focusedVisible && !theme.IsTransparentMaterial(focus) {
		inset := mathutil.Max(1, li.cachedFocusBounds.Height()*0.08)
		ringBounds := li.cachedFocusBounds.Inset(-inset, -inset)
		cmds = append(cmds, theme.MaterialCommands(gfx.RoundedRectPath(ringBounds, li.cachedRadius+inset), focus)...)
	}
	if li.LeadingIconRef.Get() != "" {
		if iconCmds := li.leadingIconCommands(runtime, slots.LeadingIcon.Resolve(state, tokens)); len(iconCmds) > 0 {
			cmds = append(cmds, iconCmds...)
		}
	}
	return cmds
}

func (li *ListItem) leadingIconCommands(runtime any, material theme.Material) []gfx.Command {
	if li.LeadingIconRef.Get() == "" || theme.IsTransparentMaterial(material) {
		return nil
	}
	iconRect := li.cachedLeadingBounds
	if iconRect.IsEmpty() {
		return nil
	}
	asset, ok := li.resolveIcon(runtime)
	if !ok || len(asset.Path.Segments) == 0 {
		return nil
	}
	box := asset.ViewBox
	if box.IsEmpty() {
		box = gfxsvg.Bounds(asset.Path)
	}
	if box.IsEmpty() {
		return nil
	}
	if box.Width() == 0 || box.Height() == 0 {
		return nil
	}
	sx := iconRect.Width() / box.Width()
	sy := iconRect.Height() / box.Height()
	scale := mathutil.Min(sx, sy)
	if scale <= 0 {
		return nil
	}
	target := gfxsvg.Transformed(asset.Path, gfx.Identity().Multiply(gfx.Translation(iconRect.Min.X-box.Min.X*scale, iconRect.Min.Y-box.Min.Y*scale)).Multiply(gfx.Scale(scale, scale)))
	cmds := make([]gfx.Command, 0, 8)
	cmds = append(cmds, gfx.FillPath{Path: target, Brush: gfx.SolidBrush(theme.MaterialColor(material))})
	return cmds
}

func (li *ListItem) resolveIcon(runtime any) (runtimepkg.IconAsset, bool) {
	type iconProvider interface {
		IconResolver() runtimepkg.IconResolver
	}
	if runtime == nil {
		return runtimepkg.IconAsset{}, false
	}
	if provider, ok := runtime.(iconProvider); ok {
		if resolver := provider.IconResolver(); resolver != nil {
			return resolver.ResolveIcon(li.LeadingIconRef.Get())
		}
	}
	if resolver, ok := runtime.(interface {
		ResolveIcon(string) (runtimepkg.IconAsset, bool)
	}); ok {
		return resolver.ResolveIcon(li.LeadingIconRef.Get())
	}
	return runtimepkg.IconAsset{}, false
}

func (li *ListItem) hitTest(p gfx.Point) facet.HitResult {
	if li == nil || li.Layout.ArrangedBounds.IsEmpty() || !li.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := li.cursorShape()
	if li.focusedVisible && li.cachedFocusBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDFocusRing, Cursor: cursor}
	}
	if li.cachedSelectionBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDSelectionIndicator, Cursor: cursor}
	}
	if li.cachedLeadingBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDLeadingIcon, Cursor: cursor}
	}
	if li.cachedSupportingBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDSupportingText, Cursor: cursor}
	}
	if li.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDLabel, Cursor: cursor}
	}
	if li.cachedItemBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: listItemMarkIDItemContainer, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: listItemMarkIDRoot, Cursor: cursor}
}

func (li *ListItem) cursorShape() facet.CursorShape {
	if li.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (li *ListItem) onPointer(e facet.PointerEvent) bool {
	if li.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		li.hovered = true
		li.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		li.hovered = false
		if !li.pressed {
			li.focusFromPointer = false
		}
		li.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		li.hovered = true
		li.pressed = true
		li.focusFromPointer = true
		li.focusedVisible = false
		li.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := li.pressed
		li.pressed = false
		li.invalidate(facet.DirtyProjection)
		if wasPressed {
			li.Activated.Emit(signal.Unit{})
		}
		return wasPressed
	default:
		return false
	}
}

func (li *ListItem) onKey(e facet.KeyEvent) bool {
	if li.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			li.pressed = true
			li.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := li.pressed
			li.pressed = false
			li.invalidate(facet.DirtyProjection)
			if wasPressed {
				li.Activated.Emit(signal.Unit{})
			}
			return wasPressed
		}
	}
	return false
}

func (li *ListItem) onFocusGained() {
	li.focusedVisible = !li.focusFromPointer
	li.focusFromPointer = false
	li.invalidate(facet.DirtyProjection)
}

func (li *ListItem) onFocusLost() {
	li.focusedVisible = false
	li.pressed = false
	li.focusFromPointer = false
	li.invalidate(facet.DirtyProjection)
}

func (li *ListItem) interactionState() theme.InteractionState {
	switch {
	case li.Disabled.Get():
		return theme.StateDisabled
	case li.pressed:
		return theme.StatePressed
	case li.hovered:
		return theme.StateHover
	case li.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (li *ListItem) newShaper(runtime any) *text.Shaper {
	registry := li.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (li *ListItem) fontRegistry(runtime any) *text.FontRegistry {
	if runtime == nil {
		return nil
	}
	type fontRegistryProvider interface {
		FontRegistry() *text.FontRegistry
	}
	if provider, ok := runtime.(fontRegistryProvider); ok {
		return provider.FontRegistry()
	}
	return nil
}

type listItemGroupPolicy struct{}

func (listItemGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (listItemGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (listItemGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
