package selection

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	radioGroupMarkIDRoot       facet.MarkID = 1
	radioGroupMarkIDGroupLabel facet.MarkID = 2
	radioGroupMarkIDRadioItems facet.MarkID = 3
	radioGroupMarkIDControl    facet.MarkID = 4
	radioGroupMarkIDItemLabel  facet.MarkID = 5
	radioGroupMarkIDFocusRing  facet.MarkID = 6
)

// RadioOption describes one mutually exclusive choice.
type RadioOption struct {
	Value string
	Label string
}

// RadioGroup implements the selection.radio_group standard mark.
type RadioGroup struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Value *store.ValueStore[string]

	Label    string
	Options  []RadioOption
	Variant  uiinput.RadioGroupVariant
	Disabled bool

	hoveredIndex     int
	pressedIndex     int
	focusedIndex     int
	focusedVisible   bool
	focusFromPointer bool

	cachedLayout           *text.TextLayout
	cachedGroupLabel       *text.TextLayout
	cachedItemLayouts      []*text.TextLayout
	cachedItemLabelLayouts []*text.TextLayout
	cachedTokens           theme.Tokens
	cachedRecipe           shared.RadioGroupSlots
	cachedRootBounds       gfx.Rect
	cachedGroupLabelRect   gfx.Rect
	cachedItemsRect        gfx.Rect
	cachedItemRows         []gfx.Rect
	cachedItemControls     []gfx.Rect
	cachedItemLabels       []gfx.Rect
	cachedItemFocusRing    []gfx.Rect
	cachedControlSize      float32
	cachedControlGap       float32
	cachedItemGap          float32
	cachedGroupGap         float32
	cachedGroupLabelStyle  text.TextStyle
	cachedItemLabelStyle   text.TextStyle
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*RadioGroup)(nil)
var _ layout.AnchorExporter = (*RadioGroup)(nil)

// NewRadioGroup constructs a selection.radio_group mark with canonical defaults.
func NewRadioGroup(label string, options []RadioOption) *RadioGroup {
	rg := &RadioGroup{
		Facet:   facet.NewFacet(),
		Label:   label,
		Variant: uiinput.RadioGroupStandard,
	}
	rg.SetOptions(options)
	rg.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: radioGroupPolicy{},
	}
	rg.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := rg.measureIntrinsic(ctx, constraints)
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
	rg.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return rg.measure(ctx, constraints)
	}
	rg.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		rg.layoutRole.ArrangedBounds = bounds
		rg.arrange(ctx, bounds)
	}
	rg.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := rg.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	rg.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := rg.buildCommands(rg.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	rg.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return rg.hitTest(p)
	}
	rg.inputRole.OnPointer = func(e facet.PointerEvent) bool { return rg.onPointer(e) }
	rg.inputRole.OnKey = func(e facet.KeyEvent) bool { return rg.onKey(e) }
	rg.focusRole.Focusable = func() bool { return !rg.Disabled && len(rg.Options) > 0 }
	rg.focusRole.TabIndex = 0
	rg.focusRole.OnFocusGained = func() { rg.onFocusGained() }
	rg.focusRole.OnFocusLost = func() { rg.onFocusLost() }
	rg.textRole.IMEEnabled = false
	rg.AddRole(&rg.layoutRole)
	rg.AddRole(&rg.renderRole)
	rg.AddRole(&rg.projectionRole)
	rg.AddRole(&rg.hitRole)
	rg.AddRole(&rg.inputRole)
	rg.AddRole(&rg.focusRole)
	rg.AddRole(&rg.textRole)
	return rg
}

// Base satisfies facet.FacetImpl.
func (rg *RadioGroup) Base() *facet.Facet {
	rg.Facet.BindImpl(rg)
	return &rg.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (rg *RadioGroup) AccessibilityRole() string { return "radiogroup" }

// AccessibleName reports the semantic name source required by the spec.
func (rg *RadioGroup) AccessibleName() string {
	if rg == nil {
		return ""
	}
	return rg.Label
}

// SetLabel updates the authored group label.
func (rg *RadioGroup) SetLabel(label string) {
	if rg == nil || rg.Label == label {
		return
	}
	rg.Label = label
	rg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetOptions updates the available radio options.
func (rg *RadioGroup) SetOptions(options []RadioOption) {
	if rg == nil {
		return
	}
	next := append([]RadioOption(nil), options...)
	if len(next) > 0 {
		for i := range next {
			next[i].Value = strings.TrimSpace(next[i].Value)
			next[i].Label = strings.TrimSpace(next[i].Label)
		}
	}
	rg.Options = next
	if rg.Value == nil {
		if len(next) > 0 {
			rg.Value = store.NewValueStore[string](next[0].Value)
		} else {
			rg.Value = store.NewValueStore[string]("")
		}
	}
	if len(next) > 0 {
		if rg.optionIndexByValue(rg.Value.Get()) < 0 {
			rg.Value.Set(next[0].Value)
		}
	}
	rg.focusedIndex = rg.selectedIndex()
	rg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetValue updates the canonical selected value.
func (rg *RadioGroup) SetValue(value string) {
	if rg == nil {
		return
	}
	value = strings.TrimSpace(value)
	if len(rg.Options) > 0 {
		if idx := rg.optionIndexByValue(value); idx >= 0 {
			value = rg.Options[idx].Value
		} else {
			value = rg.Options[0].Value
		}
	}
	if rg.Value == nil {
		rg.Value = store.NewValueStore[string](value)
		rg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	if rg.Value.Get() == value {
		return
	}
	rg.Value.Set(value)
	rg.focusedIndex = rg.selectedIndex()
	rg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (rg *RadioGroup) SetDisabled(disabled bool) {
	if rg == nil || rg.Disabled == disabled {
		return
	}
	rg.Disabled = disabled
	if disabled {
		rg.hoveredIndex = -1
		rg.pressedIndex = -1
		rg.focusedVisible = false
		rg.focusFromPointer = false
	}
	rg.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the radio-group anchor set.
func (rg *RadioGroup) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if rg == nil {
		return nil
	}
	bounds := rg.layoutRole.ArrangedBounds
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
	if rg.cachedGroupLabel != nil {
		out["baseline"] = gfx.Point{X: rg.cachedGroupLabelRect.Min.X, Y: rg.cachedGroupLabelRect.Min.Y + rg.cachedGroupLabel.Baseline}
	} else if len(rg.cachedItemLabelLayouts) > 0 && rg.cachedItemLabelLayouts[0] != nil && len(rg.cachedItemLabels) > 0 {
		out["baseline"] = gfx.Point{X: rg.cachedItemLabels[0].Min.X, Y: rg.cachedItemLabels[0].Min.Y + rg.cachedItemLabelLayouts[0].Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (rg *RadioGroup) Children() []facet.GroupChild { return nil }

// OnAttach wires store invalidation for the bound value store.
func (rg *RadioGroup) OnAttach(ctx facet.AttachContext) {
	if rg.Value == nil {
		if len(rg.Options) > 0 {
			rg.Value = store.NewValueStore[string](rg.Options[0].Value)
		} else {
			rg.Value = store.NewValueStore[string]("")
		}
	}
	facet.Store(facet.Subscribe(rg), &rg.Value.OnChange, rg.Value.Version, func(signal.Change[string]) {
		rg.focusedIndex = rg.selectedIndex()
		rg.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (rg *RadioGroup) OnActivate() {}

// OnDeactivate is unused.
func (rg *RadioGroup) OnDeactivate() {}

// OnDetach clears cached projection state.
func (rg *RadioGroup) OnDetach() {
	rg.cachedLayout = nil
	rg.cachedGroupLabel = nil
	rg.cachedItemLayouts = nil
	rg.cachedItemLabelLayouts = nil
	rg.cachedTokens = theme.Tokens{}
	rg.cachedRecipe = shared.RadioGroupSlots{}
	rg.cachedRootBounds = gfx.Rect{}
	rg.cachedGroupLabelRect = gfx.Rect{}
	rg.cachedItemsRect = gfx.Rect{}
	rg.cachedItemRows = nil
	rg.cachedItemControls = nil
	rg.cachedItemLabels = nil
	rg.cachedItemFocusRing = nil
	rg.cachedControlSize = 0
	rg.cachedControlGap = 0
	rg.cachedItemGap = 0
	rg.cachedGroupGap = 0
	rg.cachedGroupLabelStyle = text.TextStyle{}
	rg.cachedItemLabelStyle = text.TextStyle{}
}

func (rg *RadioGroup) invalidate(flags facet.DirtyFlags) {
	if rg == nil {
		return
	}
	rg.Base().Invalidate(flags)
}

func (rg *RadioGroup) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveRadioGroupRecipe(style, rg.Variant)
	rg.cachedTokens = resolved.TokenSet()
	rg.cachedRecipe = slots
	rg.cachedWritingDirection = ctx.WritingDirection
	rg.cachedControlSize = radioControlSize(resolved)
	rg.cachedControlGap = float32(resolved.Spacing(theme.SpacingS))
	rg.cachedItemGap = float32(resolved.Spacing(theme.SpacingS))
	rg.cachedGroupGap = float32(resolved.Spacing(theme.SpacingXS))
	rg.cachedGroupLabelStyle = resolved.TextStyle(theme.TextLabelM)
	rg.cachedItemLabelStyle = resolved.TextStyle(theme.TextBodyM)
	shaper := rg.newShaper(ctx.Runtime)

	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = radioGroupDefaultMaxWidth(resolved)
	}
	groupLabelLayout := shaper.ShapeTruncated(rg.Label, rg.cachedGroupLabelStyle, maxWidth)
	groupLabelH := text.Height(groupLabelLayout)
	itemLayouts := make([]*text.TextLayout, len(rg.Options))
	itemWidths := make([]float32, len(rg.Options))
	itemHeights := make([]float32, len(rg.Options))
	controlTarget := maxFloat(rg.cachedControlSize, resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget))
	itemMaxTextWidth := maxFloat(0, maxWidth-rg.cachedControlSize-rg.cachedControlGap)
	for i, opt := range rg.Options {
		if shaper != nil {
			shaper.SetContentScale(ctx.ContentScale)
			itemLayouts[i] = shaper.ShapeTruncated(opt.Label, rg.cachedItemLabelStyle, itemMaxTextWidth)
		}
		if itemLayouts[i] != nil {
			itemWidths[i] = itemLayouts[i].Bounds.Width()
			itemHeights[i] = itemLayouts[i].Bounds.Height()
		}
	}
	rowHeight := maxFloat(controlTarget, rg.cachedControlSize)
	for i := range itemHeights {
		if itemHeights[i] > rowHeight {
			rowHeight = itemHeights[i]
		}
	}
	totalItemHeight := float32(0)
	maxItemWidth := float32(0)
	for i := range rg.Options {
		rowWidth := rg.cachedControlSize + rg.cachedControlGap
		if itemLayouts[i] != nil {
			rowWidth += itemWidths[i]
		}
		if rowWidth > maxItemWidth {
			maxItemWidth = rowWidth
		}
		totalItemHeight += rowHeight
		if i+1 < len(rg.Options) {
			totalItemHeight += rg.cachedItemGap
		}
	}
	width := maxFloat(text.Width(groupLabelLayout), maxItemWidth)
	if width <= 0 {
		width = maxWidth
	}
	height := groupLabelH
	if groupLabelH > 0 && len(rg.Options) > 0 {
		height += rg.cachedGroupGap
	}
	height += totalItemHeight
	if height <= 0 {
		height = controlTarget
	}
	rg.cachedLayout = &text.TextLayout{Bounds: text.RectFromXYWH(0, 0, width, height), LineHeight: height, Baseline: 0}
	rg.cachedGroupLabel = groupLabelLayout
	rg.cachedItemLayouts = itemLayouts
	rg.cachedItemLabelLayouts = itemLayouts
	rg.textRole.Layout = groupLabelLayout
	rg.textRole.Selection = text.TextRange{}
	rg.textRole.CaretVisible = false
	rg.textRole.CaretPosition = text.TextPosition{}
	size := gfx.Size{W: width, H: height}
	rg.layoutRole.MeasuredSize = size
	rg.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return rg.layoutRole.MeasuredResult
}

func (rg *RadioGroup) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return rg.measure(ctx, constraints).Size
}

func (rg *RadioGroup) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	rg.cachedRootBounds = bounds
	rg.cachedGroupLabelRect = gfx.Rect{}
	rg.cachedItemsRect = gfx.Rect{}
	rg.cachedItemRows = nil
	rg.cachedItemControls = nil
	rg.cachedItemLabels = nil
	rg.cachedItemFocusRing = nil
	rg.layoutRole.ArrangedBounds = bounds
	if rg.cachedLayout == nil || bounds.IsEmpty() {
		return
	}
	selected := rg.selectedIndex()
	if rg.focusedIndex < 0 || rg.focusedIndex >= len(rg.Options) {
		rg.focusedIndex = selected
	}
	groupLabelH := text.Height(rg.cachedGroupLabel)
	rowHeight := maxFloat(rg.cachedControlSize, rg.cachedControlGap)
	items := make([]gfx.Rect, 0, len(rg.Options))
	controls := make([]gfx.Rect, 0, len(rg.Options))
	labels := make([]gfx.Rect, 0, len(rg.Options))
	focusRects := make([]gfx.Rect, 0, len(rg.Options))
	totalItemHeight := float32(0)
	for i := range rg.Options {
		totalItemHeight += rowHeight
		if i+1 < len(rg.Options) {
			totalItemHeight += rg.cachedItemGap
		}
	}
	stack := layout.ArrangeVerticalFlow(bounds, 0, rg.cachedGroupGap, []gfx.Size{
		{W: bounds.Width(), H: groupLabelH},
		{W: bounds.Width(), H: totalItemHeight},
	}, rg.cachedWritingDirection == facet.WritingDirectionRTL)
	if rg.cachedGroupLabel != nil {
		rg.cachedGroupLabelRect = stack[0]
	}
	itemsTop := stack[1].Min.Y
	for i, opt := range rg.Options {
		labelLayout := rg.cachedItemLayouts[i]
		labelW := float32(0)
		labelH := float32(0)
		if labelLayout != nil {
			labelW = labelLayout.Bounds.Width()
			labelH = labelLayout.Bounds.Height()
		}
		rowY := itemsTop
		controlY := rowY + (rowHeight-rg.cachedControlSize)*0.5
		labelY := rowY + (rowHeight-labelH)*0.5
		controlX := bounds.Min.X
		labelX := bounds.Min.X + rg.cachedControlSize + rg.cachedControlGap
		if rg.cachedWritingDirection == facet.WritingDirectionRTL {
			controlX = bounds.Max.X - rg.cachedControlSize
			labelX = bounds.Max.X - rg.cachedControlSize - rg.cachedControlGap - labelW
		}
		controlRect := gfx.RectFromXYWH(controlX, controlY, rg.cachedControlSize, rg.cachedControlSize)
		labelRect := gfx.RectFromXYWH(labelX, labelY, labelW, labelH)
		rowRect := gfx.RectFromXYWH(bounds.Min.X, rowY, bounds.Width(), rowHeight)
		items = append(items, rowRect)
		controls = append(controls, controlRect)
		labels = append(labels, labelRect)
		focusRects = append(focusRects, rowRect.Inset(-2, -2))
		itemsTop += rowHeight
		if i+1 < len(rg.Options) {
			itemsTop += rg.cachedItemGap
		}
		_ = opt
	}
	if len(items) > 0 {
		rg.cachedItemsRect = items[0]
		for _, rect := range items[1:] {
			rg.cachedItemsRect = rg.cachedItemsRect.Union(rect)
		}
	}
	rg.cachedItemRows = items
	rg.cachedItemControls = controls
	rg.cachedItemLabels = labels
	rg.cachedItemFocusRing = focusRects
	rg.layoutRole.ArrangedBounds = bounds
}

func (rg *RadioGroup) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.RadioGroupSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: rg.cachedTokens}, rg.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, rg.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveRadioGroupRecipe(style, rg.Variant)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: rg.cachedTokens}, rg.cachedRecipe
}

func (rg *RadioGroup) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if rg == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := rg.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	interaction := rg.interactionState()
	root := slots.Root.Resolve(interaction, tokens)
	groupLabel := slots.GroupLabel.Resolve(rg.labelState(), tokens)
	radioItems := slots.RadioItems.Resolve(interaction, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(radioItems) && !rg.cachedItemsRect.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RectPath(rg.cachedItemsRect), radioItems)...)
	}
	if rg.cachedGroupLabel != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(rg.cachedGroupLabel, rg.cachedGroupLabelRect, gfx.SolidBrush(materialColor(groupLabel)))...)
	}
	for i := range rg.Options {
		state := rg.optionState(i)
		controlMaterial := slots.RadioControl.Resolve(state, tokens)
		labelMaterial := slots.ItemLabel.Resolve(state, tokens)
		if !rg.cachedItemControls[i].IsEmpty() && !isTransparentMaterial(controlMaterial) {
			cmds = append(cmds, materialCommands(gfx.CirclePath(rectCenterPoint(rg.cachedItemControls[i]), rg.cachedItemControls[i].Width()*0.5), controlMaterial)...)
		}
		if rg.isSelectedIndex(i) && !rg.cachedItemControls[i].IsEmpty() {
			dotRadius := maxFloat(1, rg.cachedItemControls[i].Width()*0.22)
			cmds = append(cmds, materialCommands(gfx.CirclePath(rectCenterPoint(rg.cachedItemControls[i]), dotRadius), theme.FromToken(tokens.Color.OnPrimary))...)
		}
		if i < len(rg.cachedItemLayouts) && rg.cachedItemLayouts[i] != nil {
			cmds = append(cmds, primitive.TextLayoutCommands(rg.cachedItemLayouts[i], rg.cachedItemLabels[i], gfx.SolidBrush(materialColor(labelMaterial)))...)
		}
		if rg.focusedVisible && i == rg.focusedIndex && !isTransparentMaterial(focus) {
			inset := maxFloat(1, rg.cachedItemGap*0.5)
			ringBounds := rg.cachedItemFocusRing[i].Inset(-inset, -inset)
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, rg.cachedControlSize*0.5+inset), focus)...)
		}
	}
	return cmds
}

func (rg *RadioGroup) hitTest(p gfx.Point) facet.HitResult {
	if rg == nil || rg.layoutRole.ArrangedBounds.IsEmpty() || !rg.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := rg.cursorShape()
	if rg.focusedVisible && rg.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDFocusRing, Cursor: cursor}
	}
	if rg.cachedGroupLabelRect.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDGroupLabel, Cursor: cursor}
	}
	for i := range rg.cachedItemRows {
		if !rg.cachedItemRows[i].Contains(p) {
			continue
		}
		if rg.cachedItemControls[i].Contains(p) {
			return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDControl, Cursor: cursor}
		}
		if rg.cachedItemLabels[i].Contains(p) {
			return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDItemLabel, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDRadioItems, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: radioGroupMarkIDRoot, Cursor: cursor}
}

func (rg *RadioGroup) pointInFocusRing(p gfx.Point) bool {
	if rg.focusedIndex < 0 || rg.focusedIndex >= len(rg.cachedItemFocusRing) {
		return false
	}
	ring := rg.cachedItemFocusRing[rg.focusedIndex]
	if ring.IsEmpty() || !ring.Contains(p) {
		return false
	}
	inner := ring.Inset(maxFloat(1, rg.cachedItemGap*0.5), maxFloat(1, rg.cachedItemGap*0.5))
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (rg *RadioGroup) cursorShape() facet.CursorShape {
	if rg.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (rg *RadioGroup) onPointer(e facet.PointerEvent) bool {
	if rg.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		if idx := rg.itemIndexAt(e.Position); idx >= 0 {
			rg.hoveredIndex = idx
			rg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerLeave:
		rg.hoveredIndex = -1
		rg.pressedIndex = -1
		rg.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		if idx := rg.itemIndexAt(e.Position); idx >= 0 {
			rg.hoveredIndex = idx
			if rg.pressedIndex >= 0 {
				rg.focusedIndex = idx
			}
			rg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if idx := rg.itemIndexAt(e.Position); idx >= 0 {
			rg.hoveredIndex = idx
			rg.pressedIndex = idx
			rg.focusFromPointer = true
			rg.focusedVisible = false
			rg.focusedIndex = idx
			rg.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		idx := rg.itemIndexAt(e.Position)
		wasPressed := rg.pressedIndex >= 0
		pressed := rg.pressedIndex
		rg.pressedIndex = -1
		rg.invalidate(facet.DirtyProjection)
		if wasPressed && idx >= 0 && idx == pressed {
			rg.selectIndex(idx)
			return true
		}
		return wasPressed
	default:
		return false
	}
}

func (rg *RadioGroup) onKey(e facet.KeyEvent) bool {
	if rg.Disabled || len(rg.Options) == 0 {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			rg.pressedIndex = rg.focusIndexOrSelected()
			rg.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := rg.pressedIndex >= 0
			rg.pressedIndex = -1
			rg.invalidate(facet.DirtyProjection)
			if wasPressed {
				rg.selectIndex(rg.focusIndexOrSelected())
			}
			return wasPressed
		}
	case platform.KeyLeft, platform.KeyUp:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			rg.moveSelection(-1)
			return true
		}
	case platform.KeyRight, platform.KeyDown:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			rg.moveSelection(1)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			rg.selectIndex(0)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			rg.selectIndex(len(rg.Options) - 1)
			return true
		}
	}
	return false
}

func (rg *RadioGroup) onFocusGained() {
	rg.focusedVisible = !rg.focusFromPointer
	rg.focusFromPointer = false
	if rg.focusedIndex < 0 {
		rg.focusedIndex = rg.selectedIndex()
	}
	rg.invalidate(facet.DirtyProjection)
}

func (rg *RadioGroup) onFocusLost() {
	rg.focusedVisible = false
	rg.pressedIndex = -1
	rg.focusFromPointer = false
	rg.invalidate(facet.DirtyProjection)
}

func (rg *RadioGroup) interactionState() theme.InteractionState {
	switch {
	case rg.Disabled:
		return theme.StateDisabled
	case rg.pressedIndex >= 0:
		return theme.StatePressed
	case rg.hoveredIndex >= 0:
		return theme.StateHover
	case rg.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (rg *RadioGroup) labelState() theme.InteractionState {
	if rg.Disabled {
		return theme.StateDisabled
	}
	return theme.StateDefault
}

func (rg *RadioGroup) optionState(idx int) theme.InteractionState {
	switch {
	case rg.Disabled:
		return theme.StateDisabled
	case rg.pressedIndex == idx:
		return theme.StatePressed
	case rg.hoveredIndex == idx:
		return theme.StateHover
	case idx == rg.focusedIndex && rg.focusedVisible:
		return theme.StateFocused
	case rg.isSelectedIndex(idx):
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (rg *RadioGroup) selectedIndex() int {
	if rg == nil {
		return -1
	}
	if len(rg.Options) == 0 {
		return -1
	}
	value := rg.currentValue()
	if idx := rg.optionIndexByValue(value); idx >= 0 {
		return idx
	}
	return -1
}

func (rg *RadioGroup) currentValue() string {
	if rg == nil || rg.Value == nil {
		return ""
	}
	return rg.Value.Get()
}

func (rg *RadioGroup) isSelectedIndex(idx int) bool {
	return idx >= 0 && idx == rg.selectedIndex()
}

func (rg *RadioGroup) focusIndexOrSelected() int {
	if rg.focusedIndex >= 0 && rg.focusedIndex < len(rg.Options) {
		return rg.focusedIndex
	}
	if idx := rg.selectedIndex(); idx >= 0 {
		return idx
	}
	return 0
}

func (rg *RadioGroup) selectIndex(idx int) {
	if idx < 0 || idx >= len(rg.Options) {
		return
	}
	rg.focusedIndex = idx
	rg.SetValue(rg.Options[idx].Value)
}

func (rg *RadioGroup) moveSelection(delta int) {
	if len(rg.Options) == 0 {
		return
	}
	idx := rg.focusIndexOrSelected()
	idx += delta
	if idx < 0 {
		idx = 0
	}
	if idx >= len(rg.Options) {
		idx = len(rg.Options) - 1
	}
	rg.selectIndex(idx)
}

func (rg *RadioGroup) itemIndexAt(p gfx.Point) int {
	for i := range rg.cachedItemRows {
		if rg.cachedItemRows[i].Contains(p) {
			return i
		}
	}
	return -1
}

func (rg *RadioGroup) optionIndexByValue(value string) int {
	for i := range rg.Options {
		if rg.Options[i].Value == value {
			return i
		}
	}
	return -1
}

func (rg *RadioGroup) newShaper(runtime any) *text.Shaper {
	registry := rg.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (rg *RadioGroup) fontRegistry(runtime any) *text.FontRegistry {
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

func radioControlSize(resolved theme.ResolvedContext) float32 {
	size := resolved.Density.Scale(18)
	if size < 16 {
		size = 16
	}
	return size
}

func radioGroupDefaultMaxWidth(resolved theme.ResolvedContext) float32 {
	width := resolved.Density.Scale(340)
	if width < 240 {
		width = 240
	}
	return width
}

type radioGroupPolicy struct{}

func (radioGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (radioGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (radioGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
