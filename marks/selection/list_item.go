package selection

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
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
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[signal.Unit]

	LeadingIconRef         string
	Label                  string
	SupportingText         string
	Variant                uiinput.ListItemVariant
	Selected               bool
	Active                 bool
	Disabled               bool
	ShowLabel              bool
	ShowContainer          bool
	ShowLeadingIcon        bool
	ShowSelectionIndicator bool
	ShowFocusRing          bool

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

// NewListItem constructs a selection.list_item mark with canonical defaults.
func NewListItem(label string) *ListItem {
	li := &ListItem{
		Facet:                  facet.NewFacet(),
		Label:                  label,
		Variant:                uiinput.ListItemStandard,
		ShowLabel:              true,
		ShowContainer:          true,
		ShowLeadingIcon:        true,
		ShowSelectionIndicator: true,
		ShowFocusRing:          true,
	}
	li.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: listItemGroupPolicy{},
	}
	li.layoutRole.Child = facet.GroupChildContract{
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
	li.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return li.measure(ctx, constraints)
	}
	li.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		li.layoutRole.ArrangedBounds = bounds
		li.arrange(ctx, bounds)
	}
	li.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := li.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	li.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := li.buildCommands(li.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	li.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return li.hitTest(p) }
	li.inputRole.OnPointer = func(e facet.PointerEvent) bool { return li.onPointer(e) }
	li.inputRole.OnKey = func(e facet.KeyEvent) bool { return li.onKey(e) }
	li.focusRole.Focusable = func() bool { return !li.Disabled }
	li.focusRole.TabIndex = 0
	li.focusRole.OnFocusGained = func() { li.onFocusGained() }
	li.focusRole.OnFocusLost = func() { li.onFocusLost() }
	li.textRole.IMEEnabled = false
	li.AddRole(&li.layoutRole)
	li.AddRole(&li.renderRole)
	li.AddRole(&li.projectionRole)
	li.AddRole(&li.hitRole)
	li.AddRole(&li.inputRole)
	li.AddRole(&li.focusRole)
	li.AddRole(&li.textRole)
	return li
}

// Base satisfies facet.FacetImpl.
func (li *ListItem) Base() *facet.Facet {
	li.Facet.BindImpl(li)
	return &li.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (li *ListItem) AccessibilityRole() string { return "option" }

// AccessibleName reports the semantic name source required by the spec.
func (li *ListItem) AccessibleName() string {
	if li == nil {
		return ""
	}
	return li.Label
}

// SetLabel updates the authored label.
func (li *ListItem) SetLabel(label string) {
	if li == nil || li.Label == label {
		return
	}
	li.Label = label
	li.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetSupportingText updates the authored supporting text.
func (li *ListItem) SetSupportingText(text string) {
	if li == nil || li.SupportingText == text {
		return
	}
	li.SupportingText = text
	li.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetLeadingIconRef updates the authored leading icon.
func (li *ListItem) SetLeadingIconRef(ref string) {
	if li == nil || li.LeadingIconRef == ref {
		return
	}
	li.LeadingIconRef = ref
	li.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetSelected toggles selected state.
func (li *ListItem) SetSelected(selected bool) {
	if li == nil || li.Selected == selected {
		return
	}
	li.Selected = selected
	li.invalidate(facet.DirtyProjection)
}

// SetActive toggles active-descendant style emphasis.
func (li *ListItem) SetActive(active bool) {
	if li == nil || li.Active == active {
		return
	}
	li.Active = active
	li.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (li *ListItem) SetDisabled(disabled bool) {
	if li == nil || li.Disabled == disabled {
		return
	}
	li.Disabled = disabled
	if disabled {
		li.hovered = false
		li.pressed = false
		li.focusedVisible = false
		li.focusFromPointer = false
	}
	li.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetVariant updates the authored list-item variant.
func (li *ListItem) SetVariant(variant uiinput.ListItemVariant) {
	if li == nil || li.Variant == variant {
		return
	}
	li.Variant = variant
	li.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the list-item anchor set.
func (li *ListItem) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if li == nil {
		return nil
	}
	bounds := li.layoutRole.ArrangedBounds
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
	if li.textRole.Layout != nil {
		out["baseline"] = gfx.Point{X: li.cachedLabelBounds.Min.X, Y: li.cachedLabelBounds.Min.Y + li.textRole.Layout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (li *ListItem) Children() []facet.GroupChild { return nil }

func (li *ListItem) OnAttach(ctx facet.AttachContext) {}
func (li *ListItem) OnActivate()                      {}
func (li *ListItem) OnDeactivate()                    {}
func (li *ListItem) OnDetach() {
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
	slots, _ := uiinput.ResolveListItemRecipe(style, li.Variant)
	li.cachedTokens = resolved.TokenSet()
	li.cachedRecipe = slots
	li.cachedWritingDirection = ctx.WritingDirection
	li.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	li.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(10))
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
		if li.ShowLabel {
			labelLayout = li.shapeTruncated(shaper, resolved.TextStyle(theme.TextBodyM), li.Label, maxWidth)
		}
		if strings.TrimSpace(li.SupportingText) != "" {
			supportingLayout = li.shapeTruncated(shaper, resolved.TextStyle(theme.TextBodyS), li.SupportingText, maxWidth)
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
	contentW := layoutWidth(supportingLayout)
	if li.ShowLabel {
		contentW = maxFloat(contentW, layoutWidth(labelLayout))
	}
	leadingReserve := float32(0)
	if li.ShowLeadingIcon && li.LeadingIconRef != "" {
		leadingReserve = maxFloat(resolved.Density.Scale(16), resolved.Density.Scale(16)*0.42) + li.cachedGap
	}
	selectionReserve := float32(0)
	if li.ShowSelectionIndicator {
		selectionReserve = maxFloat(resolved.Density.Scale(24), resolved.Density.Scale(12))
	}
	minWidth := resolved.Density.Scale(160)
	minHeight := resolved.Density.Scale(32)
	if !li.ShowLabel && !li.ShowContainer && !li.ShowSelectionIndicator {
		minWidth = maxFloat(resolved.Density.Scale(56), leadingReserve+li.cachedPadX*2)
		minHeight = maxFloat(resolved.Density.Scale(56), resolved.Density.Scale(20)+li.cachedPadY*2)
	}
	if !li.ShowContainer && !li.ShowLeadingIcon && !li.ShowSelectionIndicator && li.ShowLabel {
		minWidth = contentW + li.cachedPadX*2
		minHeight = layoutHeight(supportingLayout) + li.cachedPadY*2
		if supportingLayout != nil {
			minHeight += li.cachedGap
		}
		if labelLayout != nil {
			minHeight += layoutHeight(labelLayout)
		}
	}
	itemSize := gfx.Size{
		W: maxFloat(minWidth, contentW+li.cachedPadX*2+leadingReserve+selectionReserve),
		H: maxFloat(minHeight, layoutHeight(supportingLayout)+li.cachedPadY*2+li.cachedGap),
	}
	if itemSize.W <= 0 {
		itemSize.W = resolved.Density.Scale(160)
	}
	if itemSize.H <= 0 {
		itemSize.H = resolved.Density.Scale(32)
	}
	li.layoutRole.MeasuredSize = itemSize
	li.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: itemSize,
		Intrinsic: facet.IntrinsicSize{
			Min:       itemSize,
			Preferred: itemSize,
			Max:       itemSize,
		},
		Constraints: constraints,
	}
	return li.layoutRole.MeasuredResult
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
	li.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	leadingSize := maxFloat(resolved.Density.Scale(16), bounds.Height()*0.42)
	indicatorSize := maxFloat(8, bounds.Height()*0.18)
	inner := bounds.Inset(li.cachedPadX, li.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	labelH := float32(0)
	if li.ShowLabel {
		labelH = layoutHeight(li.cachedLabelLayout)
	}
	supportingH := layoutHeight(li.cachedSupportingLayout)
	contentH := labelH + supportingH
	if supportingH > 0 {
		contentH += li.cachedGap
	}
	textY := inner.Min.Y + maxFloat(0, (inner.Height()-contentH)*0.5)
	textX := inner.Min.X
	labelW := float32(0)
	if li.ShowLabel {
		labelW = layoutWidth(li.cachedLabelLayout)
	}
	if labelW <= 0 {
		labelW = maxFloat(0, inner.Width()-li.cachedPadX*2)
	}
	leadingReserve := float32(0)
	if li.ShowLeadingIcon && li.LeadingIconRef != "" {
		leadingReserve = leadingSize + li.cachedGap
	}
	if li.cachedWritingDirection == facet.WritingDirectionRTL {
		textX = inner.Max.X - labelW
		if li.ShowLeadingIcon && li.LeadingIconRef != "" {
			textX = inner.Max.X - li.cachedPadX - leadingReserve - labelW
		}
	} else if li.ShowLeadingIcon && li.LeadingIconRef != "" {
		textX = inner.Min.X + leadingReserve
	}
	if li.ShowLabel && li.cachedLabelLayout != nil {
		li.cachedLabelBounds = gfx.RectFromXYWH(textX, textY, labelW, labelH)
	}
	if supportingH > 0 {
		supportY := textY + labelH + li.cachedGap
		supportW := layoutWidth(li.cachedSupportingLayout)
		if supportW <= 0 {
			supportW = labelW
		}
		li.cachedSupportingBounds = gfx.RectFromXYWH(textX, supportY, supportW, supportingH)
	}
	if li.ShowSelectionIndicator {
		indicatorY := bounds.Min.Y + (bounds.Height()-indicatorSize)*0.5
		if li.cachedWritingDirection == facet.WritingDirectionRTL {
			li.cachedSelectionBounds = gfx.RectFromXYWH(bounds.Min.X+li.cachedPadX, indicatorY, indicatorSize, indicatorSize)
		} else {
			li.cachedSelectionBounds = gfx.RectFromXYWH(bounds.Max.X-li.cachedPadX-indicatorSize, indicatorY, indicatorSize, indicatorSize)
		}
	}
	if li.ShowLeadingIcon && li.LeadingIconRef != "" {
		if li.cachedWritingDirection == facet.WritingDirectionRTL {
			li.cachedLeadingBounds = gfx.RectFromXYWH(bounds.Max.X-li.cachedPadX-leadingSize, bounds.Min.Y+(bounds.Height()-leadingSize)*0.5, leadingSize, leadingSize)
		} else {
			li.cachedLeadingBounds = gfx.RectFromXYWH(bounds.Min.X+li.cachedPadX, bounds.Min.Y+(bounds.Height()-leadingSize)*0.5, leadingSize, leadingSize)
		}
	}
	li.cachedItemBounds = inner
	li.cachedFocusBounds = bounds.Inset(maxFloat(1, bounds.Height()*0.08), maxFloat(1, bounds.Height()*0.08))
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
			slots, _ := uiinput.ResolveListItemRecipe(style, li.Variant)
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
	if li.Selected {
		state = theme.StateSelected
	}
	if li.Active {
		state = theme.StateFocused
	}
	root := slots.Root.Resolve(state, tokens)
	container := slots.ItemContainer.Resolve(state, tokens)
	label := slots.Label.Resolve(state, tokens)
	supporting := slots.SupportingText.Resolve(state, tokens)
	indicator := slots.SelectionIndicator.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	cmds := make([]gfx.Command, 0, 24)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if li.ShowContainer && !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(bounds, li.cachedRadius), container)...)
	}
	if li.ShowLabel && !isTransparentMaterial(label) && li.cachedLabelLayout != nil {
		cmds = append(cmds, li.textCommands(li.cachedLabelLayout, li.cachedLabelBounds, label)...)
	}
	if !isTransparentMaterial(supporting) && li.cachedSupportingLayout != nil {
		cmds = append(cmds, li.textCommands(li.cachedSupportingLayout, li.cachedSupportingBounds, supporting)...)
	}
	if li.ShowSelectionIndicator && li.Selected && !isTransparentMaterial(indicator) && !li.cachedSelectionBounds.IsEmpty() {
		r := maxFloat(3, li.cachedSelectionBounds.Width()*0.5)
		center := gfx.Point{X: li.cachedSelectionBounds.Min.X + li.cachedSelectionBounds.Width()*0.5, Y: li.cachedSelectionBounds.Min.Y + li.cachedSelectionBounds.Height()*0.5}
		cmds = append(cmds, materialCommands(gfx.CirclePath(center, r), indicator)...)
	}
	if li.ShowFocusRing && li.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, li.cachedFocusBounds.Height()*0.08)
		ringBounds := li.cachedFocusBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, li.cachedRadius+inset), focus)...)
	}
	if li.LeadingIconRef != "" {
		if iconCmds := li.leadingIconCommands(runtime, slots.LeadingIcon.Resolve(state, tokens)); len(iconCmds) > 0 {
			cmds = append(cmds, iconCmds...)
		}
	}
	return cmds
}

func (li *ListItem) textCommands(layout *text.TextLayout, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if layout == nil || bounds.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	brush := gfx.SolidBrush(materialColor(material))
	baseOrigin := gfx.Point{X: bounds.Min.X + layout.Bounds.Min.X, Y: bounds.Min.Y + layout.Bounds.Min.Y}
	cmds := make([]gfx.Command, 0, len(layout.Lines))
	for _, line := range layout.Lines {
		lineOrigin := gfx.Point{X: baseOrigin.X + line.Bounds.Min.X, Y: baseOrigin.Y + line.Bounds.Min.Y}
		for _, run := range line.Runs {
			cmds = append(cmds, gfx.DrawGlyphRun{Run: run, Origin: lineOrigin, Brush: brush})
		}
	}
	return cmds
}

func (li *ListItem) leadingIconCommands(runtime any, material theme.Material) []gfx.Command {
	if li.LeadingIconRef == "" || isTransparentMaterial(material) {
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
	scale := minFloat(sx, sy)
	if scale <= 0 {
		return nil
	}
	target := gfxsvg.Transformed(asset.Path, gfx.Identity().Multiply(gfx.Translation(iconRect.Min.X-box.Min.X*scale, iconRect.Min.Y-box.Min.Y*scale)).Multiply(gfx.Scale(scale, scale)))
	cmds := make([]gfx.Command, 0, 8)
	cmds = append(cmds, gfx.FillPath{Path: target, Brush: gfx.SolidBrush(materialColor(material))})
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
			return resolver.ResolveIcon(li.LeadingIconRef)
		}
	}
	if resolver, ok := runtime.(interface {
		ResolveIcon(string) (runtimepkg.IconAsset, bool)
	}); ok {
		return resolver.ResolveIcon(li.LeadingIconRef)
	}
	return runtimepkg.IconAsset{}, false
}

func (li *ListItem) hitTest(p gfx.Point) facet.HitResult {
	if li == nil || li.layoutRole.ArrangedBounds.IsEmpty() || !li.layoutRole.ArrangedBounds.Contains(p) {
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
	if li.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (li *ListItem) onPointer(e facet.PointerEvent) bool {
	if li.Disabled {
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
	if li.Disabled {
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
	case li.Disabled:
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

func (li *ListItem) shapeTruncated(shaper *text.Shaper, style text.TextStyle, content string, maxWidth float32) *text.TextLayout {
	content = strings.TrimSpace(content)
	if content == "" || shaper == nil {
		return nil
	}
	layout := shaper.ShapeSimple(content, style)
	if layout == nil || maxWidth <= 0 || layout.Bounds.Width() <= maxWidth {
		return layout
	}
	runes := []rune(content)
	ellipsis := shaper.ShapeSimple("…", style)
	if ellipsis == nil {
		return layout
	}
	best := 0
	lo, hi := 0, len(runes)
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := shaper.ShapeSimple(string(runes[:mid]), style)
		if candidate != nil && candidate.Bounds.Width() <= maxWidth {
			best = mid
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if best == 0 {
		return ellipsis
	}
	truncated := shaper.ShapeSimple(string(runes[:best])+"…", style)
	if truncated == nil {
		return ellipsis
	}
	return truncated
}

type listItemGroupPolicy struct{}

func (listItemGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (listItemGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (listItemGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
