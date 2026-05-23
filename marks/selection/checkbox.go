package selection

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gfxsvg "codeburg.org/lexbit/lurpicui/gfx/svg"
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
	checkboxMarkIDRoot       facet.MarkID = 1
	checkboxMarkIDControlBox facet.MarkID = 2
	checkboxMarkIDCheckmark  facet.MarkID = 3
	checkboxMarkIDLabel      facet.MarkID = 4
	checkboxMarkIDHelperText facet.MarkID = 5
	checkboxMarkIDFocusRing  facet.MarkID = 6
	checkboxMarkIDStateLayer facet.MarkID = 7
)

// CheckboxState encodes the authored checkbox value.
type CheckboxState uint8

const (
	CheckboxStateOff CheckboxState = iota
	CheckboxStateOn
	CheckboxStateMixed
)

func (s CheckboxState) String() string {
	switch s {
	case CheckboxStateOff:
		return "off"
	case CheckboxStateOn:
		return "on"
	case CheckboxStateMixed:
		return "mixed"
	default:
		return "unknown"
	}
}

// Checkbox implements the selection.checkbox standard mark.
type Checkbox struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Value *store.ValueStore[CheckboxState]

	Label      string
	HelperText string
	Variant    uiinput.CheckboxVariant
	Disabled   bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool

	cachedLayout           *text.TextLayout
	cachedLabelLayout      *text.TextLayout
	cachedHelperLayout     *text.TextLayout
	cachedTokens           theme.Tokens
	cachedRecipe           shared.CheckboxSlots
	cachedRootBounds       gfx.Rect
	cachedControlBounds    gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedHelperBounds     gfx.Rect
	cachedControlRadius    float32
	cachedControlSize      float32
	cachedRowGap           float32
	cachedBlockGap         float32
	cachedLabelStyle       text.TextStyle
	cachedHelperStyle      text.TextStyle
	cachedWritingDirection facet.WritingDirection
	cachedTextRowSize      gfx.Size
	cachedTextRow          *ListItem
	cachedTickFacet        *gfxsvg.SVGFacet
	cachedMixedFacet       *gfxsvg.SVGFacet
}

var _ facet.FacetImpl = (*Checkbox)(nil)
var _ layout.AnchorExporter = (*Checkbox)(nil)

var (
	checkboxTickDocument  = mustParseCheckboxTickDocument()
	checkboxMixedDocument = mustParseCheckboxMixedDocument()
)

// NewCheckbox constructs a selection.checkbox mark with canonical defaults.
func NewCheckbox(label string) *Checkbox {
	c := &Checkbox{
		Facet:   facet.NewFacet(),
		Value:   store.NewValueStore[CheckboxState](CheckboxStateOff),
		Label:   label,
		Variant: uiinput.CheckboxStandard,
	}
	c.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: checkboxGroupPolicy{},
	}
	c.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := c.measureIntrinsic(ctx, constraints)
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
	c.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return c.measure(ctx, constraints)
	}
	c.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		c.layoutRole.ArrangedBounds = bounds
		c.arrange(ctx, bounds)
	}
	c.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := c.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	c.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := c.buildCommands(c.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	c.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return c.hitTest(p)
	}
	c.inputRole.OnPointer = func(e facet.PointerEvent) bool {
		return c.onPointer(e)
	}
	c.inputRole.OnKey = func(e facet.KeyEvent) bool {
		return c.onKey(e)
	}
	c.focusRole.Focusable = func() bool {
		return !c.Disabled
	}
	c.focusRole.TabIndex = 0
	c.focusRole.OnFocusGained = func() {
		c.onFocusGained()
	}
	c.focusRole.OnFocusLost = func() {
		c.onFocusLost()
	}
	c.textRole.IMEEnabled = false
	c.AddRole(&c.layoutRole)
	c.AddRole(&c.renderRole)
	c.AddRole(&c.projectionRole)
	c.AddRole(&c.hitRole)
	c.AddRole(&c.inputRole)
	c.AddRole(&c.focusRole)
	c.AddRole(&c.textRole)
	return c
}

// Base satisfies facet.FacetImpl.
func (c *Checkbox) Base() *facet.Facet {
	c.Facet.BindImpl(c)
	return &c.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (c *Checkbox) AccessibilityRole() string {
	return "checkbox"
}

// AccessibleName reports the semantic name source required by the spec.
func (c *Checkbox) AccessibleName() string {
	if c == nil {
		return ""
	}
	return c.Label
}

// SetLabel updates the authored label text.
func (c *Checkbox) SetLabel(label string) {
	if c == nil || c.Label == label {
		return
	}
	c.Label = label
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetHelperText updates the authored helper text.
func (c *Checkbox) SetHelperText(helper string) {
	if c == nil || c.HelperText == helper {
		return
	}
	c.HelperText = helper
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetVariant updates the authored checkbox variant.
func (c *Checkbox) SetVariant(variant uiinput.CheckboxVariant) {
	if c == nil || c.Variant == variant {
		return
	}
	c.Variant = variant
	c.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (c *Checkbox) SetDisabled(disabled bool) {
	if c == nil || c.Disabled == disabled {
		return
	}
	c.Disabled = disabled
	if disabled {
		c.hovered = false
		c.pressed = false
		c.focusedVisible = false
		c.focusFromPointer = false
	}
	c.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetState updates the canonical checkbox state.
func (c *Checkbox) SetState(state CheckboxState) {
	if c == nil {
		return
	}
	state = normalizeCheckboxState(state)
	if c.Value == nil {
		c.Value = store.NewValueStore[CheckboxState](state)
		c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	if c.Value.Get() == state {
		return
	}
	c.Value.Set(state)
	c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetChecked updates the checkbox to the on/off state.
func (c *Checkbox) SetChecked(checked bool) {
	if checked {
		c.SetState(CheckboxStateOn)
		return
	}
	c.SetState(CheckboxStateOff)
}

// ExportAnchors publishes the checkbox anchor set.
func (c *Checkbox) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if c == nil {
		return nil
	}
	bounds := c.layoutRole.ArrangedBounds
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
	if c.cachedLabelLayout != nil {
		out["baseline"] = gfx.Point{X: c.cachedLabelBounds.Min.X, Y: c.cachedLabelBounds.Min.Y + c.cachedLabelLayout.Baseline}
	} else if c.cachedHelperLayout != nil {
		out["baseline"] = gfx.Point{X: c.cachedHelperBounds.Min.X, Y: c.cachedHelperBounds.Min.Y + c.cachedHelperLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (c *Checkbox) Children() []facet.GroupChild { return nil }

// OnAttach wires store invalidation for the bound value store.
func (c *Checkbox) OnAttach(ctx facet.AttachContext) {
	if c.Value == nil {
		c.Value = store.NewValueStore[CheckboxState](CheckboxStateOff)
	}
	facet.Store(facet.Subscribe(c), &c.Value.OnChange, c.Value.Version, func(signal.Change[CheckboxState]) {
		c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (c *Checkbox) OnActivate() {}

// OnDeactivate is unused.
func (c *Checkbox) OnDeactivate() {}

// OnDetach clears cached projection state.
func (c *Checkbox) OnDetach() {
	c.cachedLayout = nil
	c.cachedLabelLayout = nil
	c.cachedHelperLayout = nil
	c.cachedTokens = theme.Tokens{}
	c.cachedRecipe = shared.CheckboxSlots{}
	c.cachedRootBounds = gfx.Rect{}
	c.cachedControlBounds = gfx.Rect{}
	c.cachedLabelBounds = gfx.Rect{}
	c.cachedHelperBounds = gfx.Rect{}
	c.cachedControlRadius = 0
	c.cachedControlSize = 0
	c.cachedRowGap = 0
	c.cachedBlockGap = 0
	c.cachedLabelStyle = text.TextStyle{}
	c.cachedHelperStyle = text.TextStyle{}
	c.cachedTextRowSize = gfx.Size{}
	c.cachedTextRow = nil
	c.cachedTickFacet = nil
	c.cachedMixedFacet = nil
}

func (c *Checkbox) invalidate(flags facet.DirtyFlags) {
	if c == nil {
		return
	}
	c.Base().Invalidate(flags)
}

func (c *Checkbox) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveCheckboxRecipe(style, c.Variant)
	cachedTokens := resolved.TokenSet()
	c.cachedTokens = cachedTokens
	c.cachedRecipe = slots
	c.cachedWritingDirection = ctx.WritingDirection
	c.cachedControlSize = checkboxControlSize(resolved)
	c.cachedRowGap = float32(resolved.Spacing(theme.SpacingS))
	c.cachedBlockGap = float32(resolved.Spacing(theme.SpacingXS))
	c.cachedControlRadius = checkboxControlRadius(resolved)
	c.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	c.cachedHelperStyle = resolved.TextStyle(theme.TextBodyS)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = checkboxDefaultMaxWidth(resolved)
	}
	row := c.ensureTextRow()
	row.Label = c.Label
	row.SupportingText = c.HelperText
	row.Variant = uiinput.ListItemStandard
	row.Disabled = c.Disabled
	row.Selected = false
	row.Active = false
	row.ShowContainer = false
	row.ShowLeadingIcon = false
	row.ShowSelectionIndicator = false
	row.ShowFocusRing = false

	textMaxWidth := maxFloat(0, maxWidth-c.cachedControlSize-c.cachedRowGap)
	rowResult := row.measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: textMaxWidth, H: constraints.MaxSize.H}})
	c.cachedTextRowSize = rowResult.Size
	c.cachedLabelLayout = row.cachedLabelLayout
	c.cachedHelperLayout = row.cachedSupportingLayout
	labelH := text.Height(c.cachedLabelLayout)
	helperH := text.Height(c.cachedHelperLayout)
	rowH := maxFloat(c.cachedControlSize, labelH)
	if rowH <= 0 {
		rowH = c.cachedControlSize
	}
	width := c.cachedControlSize
	if rowResult.Size.W > 0 {
		width = maxFloat(width, c.cachedControlSize+c.cachedRowGap+rowResult.Size.W)
	}
	if c.cachedHelperLayout != nil {
		width = maxFloat(width, c.cachedControlSize+c.cachedRowGap+c.cachedHelperLayout.Bounds.Width())
	}
	if width <= 0 {
		width = c.cachedControlSize
	}
	height := rowH
	if rowResult.Size.H > rowH {
		height = rowResult.Size.H
	}
	if helperH > 0 {
		height = maxFloat(height, rowH+c.cachedBlockGap+helperH)
	}
	if height <= 0 {
		height = c.cachedControlSize
	}
	size := gfx.Size{W: width, H: height}
	c.cachedLayout = &text.TextLayout{
		Bounds:     text.RectFromXYWH(0, 0, width, height),
		LineHeight: height,
		Baseline:   0,
	}
	c.textRole.Layout = c.cachedLabelLayout
	c.textRole.Selection = text.TextRange{}
	c.textRole.CaretVisible = false
	c.textRole.CaretPosition = text.TextPosition{}
	c.layoutRole.MeasuredSize = size
	c.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return c.layoutRole.MeasuredResult
}

func (c *Checkbox) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return c.measure(ctx, constraints).Size
}

func (c *Checkbox) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	c.cachedRootBounds = bounds
	c.cachedControlBounds = gfx.Rect{}
	c.cachedLabelBounds = gfx.Rect{}
	c.cachedHelperBounds = gfx.Rect{}
	c.layoutRole.ArrangedBounds = bounds
	if c.cachedLayout == nil || bounds.IsEmpty() {
		return
	}
	row := c.ensureTextRow()
	row.ShowContainer = false
	row.ShowLeadingIcon = false
	row.ShowSelectionIndicator = false
	row.ShowFocusRing = false
	row.Disabled = c.Disabled
	row.Label = c.Label
	row.SupportingText = c.HelperText
	row.Variant = uiinput.ListItemStandard
	labelH := text.Height(c.cachedLabelLayout)
	helperH := text.Height(c.cachedHelperLayout)
	rowH := maxFloat(c.cachedControlSize, labelH)
	if rowH <= 0 {
		rowH = c.cachedControlSize
	}
	textWidth := maxFloat(0, bounds.Width()-c.cachedControlSize-c.cachedRowGap)
	textHeight := labelH + helperH
	if helperH > 0 {
		textHeight += c.cachedBlockGap
	}
	if textHeight <= 0 {
		textHeight = rowH
	}
	textBlockH := maxFloat(bounds.Height(), c.cachedTextRowSize.H)
	if textBlockH <= 0 {
		textBlockH = maxFloat(textHeight, rowH)
	}
	textBounds := gfx.Rect{}
	if c.cachedWritingDirection == facet.WritingDirectionRTL {
		textBounds = gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, textWidth, textBlockH)
		row.arrange(ctx, textBounds)
	} else {
		textBounds = gfx.RectFromXYWH(bounds.Min.X+c.cachedControlSize+c.cachedRowGap, bounds.Min.Y, textWidth, textBlockH)
		row.arrange(ctx, textBounds)
	}
	c.cachedLabelBounds = row.cachedLabelBounds
	c.cachedHelperBounds = row.cachedSupportingBounds

	var controlX float32
	if c.cachedWritingDirection == facet.WritingDirectionRTL {
		controlX = bounds.Max.X - c.cachedControlSize
	} else {
		controlX = bounds.Min.X
	}
	controlY := c.cachedLabelBounds.Min.Y + (c.cachedLabelBounds.Height()-c.cachedControlSize)*0.5
	c.cachedControlBounds = gfx.RectFromXYWH(controlX, controlY, c.cachedControlSize, c.cachedControlSize)
	c.cachedTickBounds()
	c.layoutRole.ArrangedBounds = bounds
}

func (c *Checkbox) ensureTextRow() *ListItem {
	if c.cachedTextRow == nil {
		row := NewListItem("")
		row.ShowContainer = false
		row.ShowLeadingIcon = false
		row.ShowSelectionIndicator = false
		row.ShowFocusRing = false
		c.cachedTextRow = row
	}
	return c.cachedTextRow
}

func (c *Checkbox) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.CheckboxSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: c.cachedTokens}, c.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, c.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveCheckboxRecipe(style, c.Variant)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: c.cachedTokens}, c.cachedRecipe
}

func (c *Checkbox) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if c == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := c.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	interaction := c.interactionState()
	selectedState := c.selectedState()
	root := slots.Root.Resolve(interaction, tokens)
	control := slots.ControlBox.Resolve(selectedState, tokens)
	check := slots.Checkmark.Resolve(theme.StateDefault, tokens)
	label := slots.Label.Resolve(c.labelState(), tokens)
	helper := slots.HelperText.Resolve(c.labelState(), tokens)
	stateLayer := slots.StateLayer.Resolve(c.stateLayerState(), tokens)

	cmds := make([]gfx.Command, 0, 24)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(stateLayer) && !c.cachedControlBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(c.cachedControlBounds, c.cachedControlRadius), stateLayer)...)
	}
	if !isTransparentMaterial(control) && !c.cachedControlBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(c.cachedControlBounds, c.cachedControlRadius), control)...)
	}
	if !isTransparentMaterial(check) {
		if c.isSemanticallyMixed() {
			if mixed := c.mixedFacet(); mixed != nil {
				mixed.SetCurrentColor(materialColor(check))
				if mixedCmds := mixed.Project(c.cachedTickBounds()); mixedCmds != nil {
					cmds = append(cmds, mixedCmds.Commands...)
				}
			}
		} else if c.isSemanticallySelected() {
			if tick := c.tickFacet(); tick != nil {
				tick.SetCurrentColor(materialColor(check))
				if tickCmds := tick.Project(c.cachedTickBounds()); tickCmds != nil {
					cmds = append(cmds, tickCmds.Commands...)
				}
			}
		}
	}
	if c.cachedLabelLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(c.cachedLabelLayout, c.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
	}
	if c.cachedHelperLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(c.cachedHelperLayout, c.cachedHelperBounds, gfx.SolidBrush(materialColor(helper)))...)
	}
	return cmds
}

func (c *Checkbox) hitTest(p gfx.Point) facet.HitResult {
	if c == nil || c.layoutRole.ArrangedBounds.IsEmpty() || !c.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := c.cursorShape()
	if c.focusedVisible && c.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: checkboxMarkIDFocusRing, Cursor: cursor}
	}
	if c.cachedHelperBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: checkboxMarkIDHelperText, Cursor: cursor}
	}
	if c.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: checkboxMarkIDLabel, Cursor: cursor}
	}
	if c.cachedControlBounds.Contains(p) {
		if c.pointInCheckmark(p) {
			return facet.HitResult{Hit: true, MarkID: checkboxMarkIDCheckmark, Cursor: cursor}
		}
		if c.stateLayerVisible() {
			return facet.HitResult{Hit: true, MarkID: checkboxMarkIDStateLayer, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: checkboxMarkIDControlBox, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: checkboxMarkIDRoot, Cursor: cursor}
}

func (c *Checkbox) pointInFocusRing(p gfx.Point) bool {
	bounds := c.layoutRole.ArrangedBounds
	if bounds.IsEmpty() || !bounds.Contains(p) {
		return false
	}
	ring := maxFloat(1, c.cachedRowGap*0.5)
	inner := gfx.Rect{
		Min: gfx.Point{X: bounds.Min.X + ring, Y: bounds.Min.Y + ring},
		Max: gfx.Point{X: bounds.Max.X - ring, Y: bounds.Max.Y - ring},
	}
	if inner.IsEmpty() {
		return true
	}
	if inner.Contains(p) {
		return false
	}
	return true
}

func (c *Checkbox) pointInCheckmark(p gfx.Point) bool {
	if c.cachedControlBounds.IsEmpty() || !c.cachedControlBounds.Contains(p) {
		return false
	}
	return c.cachedTickBounds().Contains(p)
}

func (c *Checkbox) cachedTickBounds() gfx.Rect {
	if c == nil || c.cachedControlBounds.IsEmpty() {
		return gfx.Rect{}
	}
	inset := maxFloat(1, c.cachedControlBounds.Width()*0.06)
	return c.cachedControlBounds.Inset(inset, inset)
}

func (c *Checkbox) tickFacet() *gfxsvg.SVGFacet {
	if c == nil {
		return nil
	}
	if c.cachedTickFacet == nil {
		c.cachedTickFacet = gfxsvg.NewSVGFacet(checkboxTickDocument)
	}
	return c.cachedTickFacet
}

func (c *Checkbox) mixedFacet() *gfxsvg.SVGFacet {
	if c == nil {
		return nil
	}
	if c.cachedMixedFacet == nil {
		c.cachedMixedFacet = gfxsvg.NewSVGFacet(checkboxMixedDocument)
	}
	return c.cachedMixedFacet
}

func (c *Checkbox) isSemanticallyMixed() bool {
	return c.state() == CheckboxStateMixed
}

func (c *Checkbox) cursorShape() facet.CursorShape {
	if c.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (c *Checkbox) onPointer(e facet.PointerEvent) bool {
	if c.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		c.hovered = true
		c.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		c.hovered = false
		if !c.pressed {
			c.focusFromPointer = false
		}
		c.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		c.hovered = true
		c.pressed = true
		c.focusFromPointer = true
		c.focusedVisible = false
		c.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := c.pressed
		c.pressed = false
		c.invalidate(facet.DirtyProjection)
		if wasPressed && c.layoutRole.ArrangedBounds.Contains(e.Position) {
			c.toggleState()
			c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		}
		return wasPressed
	case platform.PointerMove:
		return c.hovered
	default:
		return false
	}
}

func (c *Checkbox) onKey(e facet.KeyEvent) bool {
	if c.Disabled {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			c.pressed = true
			c.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := c.pressed
			c.pressed = false
			c.invalidate(facet.DirtyProjection)
			if wasPressed {
				c.toggleState()
				c.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			}
			return wasPressed
		}
	}
	return false
}

func (c *Checkbox) onFocusGained() {
	c.focusedVisible = !c.focusFromPointer
	c.focusFromPointer = false
	c.invalidate(facet.DirtyProjection)
}

func (c *Checkbox) onFocusLost() {
	c.focusedVisible = false
	c.pressed = false
	c.focusFromPointer = false
	c.invalidate(facet.DirtyProjection)
}

func (c *Checkbox) interactionState() theme.InteractionState {
	switch {
	case c.Disabled:
		return theme.StateDisabled
	case c.pressed:
		return theme.StatePressed
	case c.hovered:
		return theme.StateHover
	case c.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (c *Checkbox) selectedState() theme.InteractionState {
	if c.isSemanticallySelected() {
		return theme.StateSelected
	}
	return c.interactionState()
}

func (c *Checkbox) stateLayerState() theme.InteractionState {
	switch {
	case c.Disabled:
		return theme.StateDisabled
	case c.pressed:
		return theme.StatePressed
	case c.hovered:
		return theme.StateHover
	case c.isSemanticallySelected():
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (c *Checkbox) labelState() theme.InteractionState {
	if c.Disabled {
		return theme.StateDisabled
	}
	return theme.StateDefault
}

func (c *Checkbox) state() CheckboxState {
	if c == nil || c.Value == nil {
		return CheckboxStateOff
	}
	return normalizeCheckboxState(c.Value.Get())
}

func (c *Checkbox) isSemanticallySelected() bool {
	switch c.state() {
	case CheckboxStateOn, CheckboxStateMixed:
		return true
	default:
		return false
	}
}

func (c *Checkbox) stateLayerVisible() bool {
	return c.hovered || c.pressed || c.isSemanticallySelected()
}

func (c *Checkbox) toggleState() {
	switch c.state() {
	case CheckboxStateOn:
		c.SetState(CheckboxStateOff)
	case CheckboxStateMixed:
		c.SetState(CheckboxStateOn)
	default:
		c.SetState(CheckboxStateOn)
	}
}

func (c *Checkbox) newShaper(runtime any) *text.Shaper {
	registry := c.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (c *Checkbox) fontRegistry(runtime any) *text.FontRegistry {
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

func checkboxControlSize(resolved theme.ResolvedContext) float32 {
	size := resolved.Density.Scale(18)
	if size < 16 {
		size = 16
	}
	return size
}

func checkboxControlRadius(resolved theme.ResolvedContext) float32 {
	radius := float32(resolved.Radius(theme.RadiusS))
	if radius <= 0 {
		radius = 4
	}
	return radius
}

func checkboxDefaultMaxWidth(resolved theme.ResolvedContext) float32 {
	width := resolved.Density.Scale(320)
	if width < 240 {
		width = 240
	}
	return width
}

func normalizeCheckboxState(state CheckboxState) CheckboxState {
	switch state {
	case CheckboxStateOn, CheckboxStateMixed:
		return state
	default:
		return CheckboxStateOff
	}
}

func mustParseCheckboxTickDocument() gfxsvg.SVGDocument {
	doc, err := gfxsvg.ParseSVGString(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
  <path fill="currentColor" fill-rule="evenodd" d="M5.5 12.25 8.5 15.25 18.5 5.25 20.25 7 8.5 18.75 3.75 14Z"/>
</svg>`)
	if err != nil {
		panic(err)
	}
	return doc
}

func mustParseCheckboxMixedDocument() gfxsvg.SVGDocument {
	doc, err := gfxsvg.ParseSVGString(`<svg xmlns="http://www.w3.org/2000/svg" width="24" height="24" fill="none" viewBox="0 0 24 24">
  <path fill="currentColor" d="M6 11h12v2H6z"/>
</svg>`)
	if err != nil {
		panic(err)
	}
	return doc
}

type checkboxGroupPolicy struct{}

func (checkboxGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (checkboxGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (checkboxGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
