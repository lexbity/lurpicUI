package input

import (
	"math"
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	numberFieldMarkIDRoot        facet.MarkID = 1
	numberFieldMarkIDContainer   facet.MarkID = 2
	numberFieldMarkIDLabel       facet.MarkID = 3
	numberFieldMarkIDInputText   facet.MarkID = 4
	numberFieldMarkIDPlaceholder facet.MarkID = 5
	numberFieldMarkIDStepperUp   facet.MarkID = 6
	numberFieldMarkIDStepperDown facet.MarkID = 7
	numberFieldMarkIDHelperText  facet.MarkID = 8
	numberFieldMarkIDErrorText   facet.MarkID = 9
	numberFieldMarkIDCaret       facet.MarkID = 10
	numberFieldMarkIDSelection   facet.MarkID = 11
	numberFieldMarkIDFocusRing   facet.MarkID = 12
)

// NumberFieldValidation controls auxiliary text and validation styling.
type NumberFieldValidation uint8

const (
	NumberFieldValidationDefault NumberFieldValidation = iota
	NumberFieldValidationWarning
	NumberFieldValidationInvalid
)

// NumberField implements the input.number_field standard mark.
type NumberField struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Value *store.ValueStore[float64]

	Label       string
	Placeholder string
	HelperText  string
	WarningText string
	ErrorText   string
	Min         float64
	Max         float64
	Step        float64
	Precision   int
	Validation  NumberFieldValidation
	Required    bool
	Disabled    bool
	ReadOnly    bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	selecting        bool
	parseError       bool
	editing          bool
	editingText      string
	selectionAnchor  text.TextPosition
	caret            text.TextPosition
	pressedStepper   int

	cachedLayout            *text.TextLayout
	cachedLabelLayout       *text.TextLayout
	cachedValueLayout       *text.TextLayout
	cachedPlaceholderLayout *text.TextLayout
	cachedHelperLayout      *text.TextLayout
	cachedErrorLayout       *text.TextLayout
	cachedTokens            theme.Tokens
	cachedRecipe            shared.NumberFieldSlots
	cachedRootBounds        gfx.Rect
	cachedFieldBounds       gfx.Rect
	cachedLabelBounds       gfx.Rect
	cachedValueBounds       gfx.Rect
	cachedHelperBounds      gfx.Rect
	cachedErrorBounds       gfx.Rect
	cachedStepperUpBounds   gfx.Rect
	cachedStepperDownBounds gfx.Rect
	cachedPadX              float32
	cachedPadY              float32
	cachedGap               float32
	cachedRadius            float32
	cachedLineHeight        float32
	cachedCaretWidth        float32
	cachedMinFieldWidth     float32
	cachedStepperWidth      float32
}

var _ facet.FacetImpl = (*NumberField)(nil)
var _ layout.AnchorExporter = (*NumberField)(nil)

// NewNumberField constructs an input.number_field mark with canonical defaults.
func NewNumberField(label string) *NumberField {
	nf := &NumberField{
		Facet:     facet.NewFacet(),
		Value:     store.NewValueStore[float64](0),
		Label:     label,
		Step:      1,
		Precision: -1,
	}
	nf.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: numberFieldGroupPolicy{},
	}
	nf.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := nf.measureIntrinsic(ctx, constraints)
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
	nf.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return nf.measure(ctx, constraints)
	}
	nf.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		nf.layoutRole.ArrangedBounds = bounds
		nf.arrange(ctx, bounds)
	}
	nf.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := nf.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	nf.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := nf.buildCommands(nf.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	nf.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return nf.hitTest(p) }
	nf.inputRole.OnPointer = func(e facet.PointerEvent) bool { return nf.onPointer(e) }
	nf.inputRole.OnKey = func(e facet.KeyEvent) bool { return nf.onKey(e) }
	nf.inputRole.OnText = func(e facet.TextEvent) bool { return nf.onText(e) }
	nf.focusRole.Focusable = func() bool { return !nf.Disabled }
	nf.focusRole.TabIndex = 0
	nf.focusRole.OnFocusGained = func() { nf.onFocusGained() }
	nf.focusRole.OnFocusLost = func() { nf.onFocusLost() }
	nf.textRole.IMEEnabled = true
	nf.AddRole(&nf.layoutRole)
	nf.AddRole(&nf.renderRole)
	nf.AddRole(&nf.projectionRole)
	nf.AddRole(&nf.hitRole)
	nf.AddRole(&nf.inputRole)
	nf.AddRole(&nf.focusRole)
	nf.AddRole(&nf.textRole)
	return nf
}

// Base satisfies facet.FacetImpl.
func (nf *NumberField) Base() *facet.Facet {
	nf.Facet.BindImpl(nf)
	return &nf.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (nf *NumberField) AccessibilityRole() string {
	return "spinbutton"
}

// AccessibleName reports the accessible name source required by the spec.
func (nf *NumberField) AccessibleName() string {
	if nf == nil {
		return ""
	}
	return nf.Label
}

// SetValue stores a new canonical numeric value.
func (nf *NumberField) SetValue(value float64) {
	if nf == nil || nf.Value == nil || nf.Value.Get() == value {
		return
	}
	nf.Value.Set(nf.clampValue(value))
	nf.syncEditingText()
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetLabel updates the field label.
func (nf *NumberField) SetLabel(label string) {
	if nf == nil || nf.Label == label {
		return
	}
	nf.Label = label
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetPlaceholder updates the placeholder text.
func (nf *NumberField) SetPlaceholder(placeholder string) {
	if nf == nil || nf.Placeholder == placeholder {
		return
	}
	nf.Placeholder = placeholder
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetHelperText updates the helper text.
func (nf *NumberField) SetHelperText(helper string) {
	if nf == nil || nf.HelperText == helper {
		return
	}
	nf.HelperText = helper
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetWarningText updates the warning text.
func (nf *NumberField) SetWarningText(warning string) {
	if nf == nil || nf.WarningText == warning {
		return
	}
	nf.WarningText = warning
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetErrorText updates the error text.
func (nf *NumberField) SetErrorText(err string) {
	if nf == nil || nf.ErrorText == err {
		return
	}
	nf.ErrorText = err
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetValidation updates the validation state.
func (nf *NumberField) SetValidation(validation NumberFieldValidation) {
	if nf == nil || nf.Validation == validation {
		return
	}
	nf.Validation = validation
	nf.invalidate(facet.DirtyProjection)
}

// SetMin updates the minimum bound.
func (nf *NumberField) SetMin(min float64) {
	if nf == nil || nf.Min == min {
		return
	}
	nf.Min = min
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetMax updates the maximum bound.
func (nf *NumberField) SetMax(max float64) {
	if nf == nil || nf.Max == max {
		return
	}
	nf.Max = max
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetStep updates the authored step value.
func (nf *NumberField) SetStep(step float64) {
	if nf == nil || nf.Step == step {
		return
	}
	nf.Step = step
	nf.invalidate(facet.DirtyProjection)
}

// SetPrecision updates formatting precision. Negative values use compact formatting.
func (nf *NumberField) SetPrecision(precision int) {
	if nf == nil || nf.Precision == precision {
		return
	}
	nf.Precision = precision
	nf.syncEditingText()
	nf.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles the disabled state.
func (nf *NumberField) SetDisabled(disabled bool) {
	if nf == nil || nf.Disabled == disabled {
		return
	}
	nf.Disabled = disabled
	if disabled {
		nf.hovered = false
		nf.pressed = false
		nf.focusedVisible = false
		nf.focusFromPointer = false
		nf.selecting = false
		nf.pressedStepper = 0
	}
	nf.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetReadOnly toggles read-only behavior.
func (nf *NumberField) SetReadOnly(readOnly bool) {
	if nf == nil || nf.ReadOnly == readOnly {
		return
	}
	nf.ReadOnly = readOnly
	nf.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the field anchor set.
func (nf *NumberField) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if nf == nil {
		return nil
	}
	bounds := nf.layoutRole.ArrangedBounds
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
	if nf.textRole.Layout != nil {
		out["baseline"] = gfx.Point{X: nf.cachedValueBounds.Min.X, Y: nf.cachedValueBounds.Min.Y + nf.textRole.Layout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (nf *NumberField) Children() []facet.GroupChild { return nil }

func (nf *NumberField) OnAttach(ctx facet.AttachContext) {
	if nf.Value == nil {
		nf.Value = store.NewValueStore[float64](0)
	}
	nf.syncEditingText()
	facet.Store(facet.Subscribe(nf), &nf.Value.OnChange, nf.Value.Version, func(signal.Change[float64]) {
		if !nf.editing || !nf.parseError {
			nf.syncEditingText()
		}
		nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

func (nf *NumberField) OnActivate()   {}
func (nf *NumberField) OnDeactivate() {}
func (nf *NumberField) OnDetach() {
	nf.cachedLayout = nil
	nf.cachedLabelLayout = nil
	nf.cachedValueLayout = nil
	nf.cachedPlaceholderLayout = nil
	nf.cachedHelperLayout = nil
	nf.cachedErrorLayout = nil
	nf.cachedTokens = theme.Tokens{}
	nf.cachedRecipe = shared.NumberFieldSlots{}
	nf.cachedRootBounds = gfx.Rect{}
	nf.cachedFieldBounds = gfx.Rect{}
	nf.cachedLabelBounds = gfx.Rect{}
	nf.cachedValueBounds = gfx.Rect{}
	nf.cachedHelperBounds = gfx.Rect{}
	nf.cachedErrorBounds = gfx.Rect{}
	nf.cachedStepperUpBounds = gfx.Rect{}
	nf.cachedStepperDownBounds = gfx.Rect{}
	nf.focusedVisible = false
	nf.hovered = false
	nf.pressed = false
	nf.selecting = false
	nf.parseError = false
	nf.editing = false
	nf.editingText = ""
	nf.caret = text.TextPosition{}
	nf.selectionAnchor = text.TextPosition{}
	nf.pressedStepper = 0
}

func (nf *NumberField) invalidate(flags facet.DirtyFlags) {
	if nf == nil {
		return
	}
	nf.Facet.Invalidate(flags)
}

func (nf *NumberField) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := nf.resolveTheme(ctx)
	if !ok {
		nf.cachedLayout = nil
		nf.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	nf.cachedTokens = resolved.TokenSet()
	nf.cachedRecipe = recipe
	nf.cachedPadX = float32(resolved.Spacing(theme.SpacingM))
	nf.cachedPadY = float32(resolved.Spacing(theme.SpacingS))
	nf.cachedGap = float32(resolved.Spacing(theme.SpacingXS))
	nf.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	nf.cachedCaretWidth = resolved.TokenSet().Spacing.BorderWeight
	if nf.cachedCaretWidth <= 0 {
		nf.cachedCaretWidth = 1
	}
	nf.cachedMinFieldWidth = float32(resolved.Spacing(theme.SpacingXL)) * 6
	nf.cachedStepperWidth = maxFloat(resolved.TokenSet().Spacing.TouchTarget*0.5, float32(resolved.Spacing(theme.SpacingL)))
	layout, labelLayout, valueLayout, placeholderLayout, helperLayout, errorLayout := nf.resolveLayouts(ctx, constraints, resolved)
	if layout == nil {
		nf.cachedLayout = nil
		nf.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	nf.cachedLayout = layout
	nf.cachedLabelLayout = labelLayout
	nf.cachedValueLayout = valueLayout
	nf.cachedPlaceholderLayout = placeholderLayout
	nf.cachedHelperLayout = helperLayout
	nf.cachedErrorLayout = errorLayout
	nf.textRole.Layout = valueLayout
	nf.textRole.Selection = nf.currentSelection(valueLayout)
	nf.textRole.CaretPosition = nf.currentCaret(valueLayout)
	nf.textRole.CaretVisible = nf.shouldShowCaret()
	size := gfx.Size{W: layout.Bounds.Width(), H: layout.Bounds.Height()}
	nf.layoutRole.MeasuredSize = size
	nf.layoutRole.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	return nf.layoutRole.MeasuredResult
}

func (nf *NumberField) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return nf.measure(ctx, constraints).Size
}

func (nf *NumberField) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	nf.cachedRootBounds = bounds
	if nf.cachedLayout == nil {
		nf.layoutRole.ArrangedBounds = bounds
		return
	}
	nf.cachedLabelBounds = gfx.Rect{}
	nf.cachedFieldBounds = gfx.Rect{}
	nf.cachedValueBounds = gfx.Rect{}
	nf.cachedHelperBounds = gfx.Rect{}
	nf.cachedErrorBounds = gfx.Rect{}
	nf.cachedStepperUpBounds = gfx.Rect{}
	nf.cachedStepperDownBounds = gfx.Rect{}
	labelH := float32(0)
	if nf.cachedLabelLayout != nil {
		labelH = nf.cachedLabelLayout.Bounds.Height()
	}
	valueH := text.MaxHeight(nf.cachedValueLayout, nf.cachedPlaceholderLayout)
	helperH := text.MaxHeight(nf.cachedHelperLayout, nf.cachedErrorLayout)
	gap := nf.cachedGap
	labelY := bounds.Min.Y
	if nf.cachedLabelLayout != nil {
		nf.cachedLabelBounds = gfx.RectFromXYWH(bounds.Min.X, labelY, bounds.Width(), labelH)
		labelY += labelH + gap
	}
	fieldH := maxFloat(valueH+nf.cachedPadY*2, resolvedMinFieldHeightFromStyle(theme.DefaultResolvedContext(), resolvedTextStyleFallback(nf.cachedValueLayout)))
	if fieldH <= 0 {
		fieldH = resolvedMinFieldHeight()
	}
	fieldY := labelY
	stepperW := nf.cachedStepperWidth
	if stepperW <= 0 {
		stepperW = fieldH
	}
	contentH := maxFloat(valueH, fieldH-nf.cachedPadY*2)
	contentTop := fieldY + maxFloat(0, (fieldH-contentH)/2)
	nf.cachedFieldBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY, bounds.Width(), fieldH)
	valueW := bounds.Width() - nf.cachedPadX*2 - stepperW - gap
	if valueW < 0 {
		valueW = 0
	}
	nf.cachedValueBounds = gfx.RectFromXYWH(bounds.Min.X+nf.cachedPadX, contentTop, valueW, valueH)
	stepperX := bounds.Max.X - stepperW
	nf.cachedStepperUpBounds = gfx.RectFromXYWH(stepperX, fieldY, stepperW, fieldH*0.5)
	nf.cachedStepperDownBounds = gfx.RectFromXYWH(stepperX, fieldY+fieldH*0.5, stepperW, fieldH*0.5)
	if nf.Validation == NumberFieldValidationWarning && nf.WarningText != "" {
		nf.cachedHelperBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	} else if nf.Validation == NumberFieldValidationInvalid && nf.errorText() != "" {
		nf.cachedErrorBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	} else if nf.cachedHelperLayout != nil {
		nf.cachedHelperBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	}
	nf.layoutRole.ArrangedBounds = bounds
	_ = ctx
}

func (nf *NumberField) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.NumberFieldSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveNumberFieldRecipe(style)
	return resolved, slots, true
}

func (nf *NumberField) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.NumberFieldSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: nf.cachedTokens}, nf.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, nf.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveNumberFieldRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: nf.cachedTokens}, nf.cachedRecipe
}

func (nf *NumberField) resolveLayouts(ctx facet.MeasureContext, constraints facet.Constraints, resolved theme.ResolvedContext) (*text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout) {
	shaper := nf.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil, nil, nil, nil, nil, nil
	}
	shaper.SetContentScale(ctx.ContentScale)
	labelStyle := resolved.TextStyle(theme.TextLabelM)
	valueStyle := resolved.TextStyle(theme.TextBodyM)
	helperStyle := resolved.TextStyle(theme.TextBodyS)
	displayText := nf.displayText()
	labelLayout := shaper.ShapeTruncated(nf.Label, labelStyle, nf.availableWidth(constraints, resolved))
	valueLayout := shaper.ShapeTruncated(displayText, valueStyle, nf.availableWidth(constraints, resolved)-nf.cachedStepperWidth)
	placeholderLayout := shaper.ShapeTruncated(nf.Placeholder, valueStyle, nf.availableWidth(constraints, resolved)-nf.cachedStepperWidth)
	helperText := nf.auxiliaryText()
	helperLayout := shaper.ShapeTruncated(helperText, helperStyle, nf.availableWidth(constraints, resolved))
	errorLayout := shaper.ShapeTruncated(nf.errorText(), helperStyle, nf.availableWidth(constraints, resolved))
	valueLayout = nf.ensureTextLayout(valueLayout, valueStyle, true)
	if valueLayout == nil {
		return nil, nil, nil, nil, nil, nil
	}
	placeholderLayout = nf.ensureTextLayout(placeholderLayout, valueStyle, true)
	labelLayout = nf.ensureTextLayout(labelLayout, labelStyle, false)
	helperLayout = nf.ensureTextLayout(helperLayout, helperStyle, false)
	errorLayout = nf.ensureTextLayout(errorLayout, helperStyle, false)
	fieldInnerWidth := maxFloat(valueLayout.Bounds.Width(), nf.cachedMinFieldWidth-nf.cachedPadX*2)
	if placeholderLayout != nil {
		fieldInnerWidth = maxFloat(fieldInnerWidth, placeholderLayout.Bounds.Width())
	}
	if constraints.MaxSize.W > 0 {
		fieldInnerWidth = minFloat(fieldInnerWidth, constraints.MaxSize.W-nf.cachedPadX*2-nf.cachedStepperWidth-nf.cachedGap)
	}
	if fieldInnerWidth < 0 {
		fieldInnerWidth = 0
	}
	valueLayout = shaper.ShapeTruncated(displayText, valueStyle, fieldInnerWidth)
	placeholderLayout = shaper.ShapeTruncated(nf.Placeholder, valueStyle, fieldInnerWidth)
	valueLayout = nf.ensureTextLayout(valueLayout, valueStyle, true)
	placeholderLayout = nf.ensureTextLayout(placeholderLayout, valueStyle, true)
	fieldH := valueLayout.Bounds.Height() + nf.cachedPadY*2
	if placeholderLayout != nil {
		fieldH = maxFloat(fieldH, placeholderLayout.Bounds.Height()+nf.cachedPadY*2)
	}
	if fieldH < resolvedMinFieldHeightFromStyle(resolved, valueStyle) {
		fieldH = resolvedMinFieldHeightFromStyle(resolved, valueStyle)
	}
	labelH := float32(0)
	if labelLayout != nil {
		labelH = labelLayout.Bounds.Height()
	}
	helperH := float32(0)
	if nf.Validation == NumberFieldValidationInvalid && nf.errorText() != "" && errorLayout != nil {
		helperH = errorLayout.Bounds.Height()
	} else if nf.Validation == NumberFieldValidationWarning && nf.warningText() != "" && helperLayout != nil {
		helperH = helperLayout.Bounds.Height()
	} else if helperLayout != nil {
		helperH = helperLayout.Bounds.Height()
	}
	gapCount := 0
	if labelLayout != nil {
		gapCount++
	}
	if helperH > 0 {
		gapCount++
	}
	totalH := labelH + fieldH + helperH + float32(gapCount)*nf.cachedGap
	if labelLayout == nil {
		totalH -= nf.cachedGap
	}
	if helperH == 0 {
		totalH -= nf.cachedGap
	}
	if totalH < fieldH {
		totalH = fieldH
	}
	width := nf.cachedMinFieldWidth
	if labelLayout != nil {
		width = maxFloat(width, labelLayout.Bounds.Width())
	}
	width = maxFloat(width, fieldInnerWidth+nf.cachedPadX*2+nf.cachedStepperWidth+nf.cachedGap)
	if helperLayout != nil {
		width = maxFloat(width, helperLayout.Bounds.Width())
	}
	if errorLayout != nil {
		width = maxFloat(width, errorLayout.Bounds.Width())
	}
	if constraints.MaxSize.W > 0 {
		width = minFloat(width, constraints.MaxSize.W)
	}
	layout := &text.TextLayout{}
	layout.Bounds = text.RectFromXYWH(0, 0, width, totalH)
	layout.LineHeight = fieldH
	layout.Baseline = valueLayout.Baseline
	return layout, labelLayout, valueLayout, placeholderLayout, helperLayout, errorLayout
}

func (nf *NumberField) availableWidth(constraints facet.Constraints, resolved theme.ResolvedContext) float32 {
	if constraints.MaxSize.W > 0 {
		return constraints.MaxSize.W
	}
	return float32(resolved.Spacing(theme.SpacingXL)) * 6
}

func (nf *NumberField) newShaper(runtime any) *text.Shaper {
	registry := nf.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (nf *NumberField) fontRegistry(runtime any) *text.FontRegistry {
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

func (nf *NumberField) ensureTextLayout(layout *text.TextLayout, style text.TextStyle, allowEmpty bool) *text.TextLayout {
	if layout == nil || len(layout.Lines) > 0 {
		return layout
	}
	if allowEmpty {
		return emptyCaretLayout(style)
	}
	return nil
}

func (nf *NumberField) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if nf == nil || bounds.IsEmpty() {
		return nil
	}
	style, recipe := nf.resolveProjectionTheme(runtime)
	state := nf.interactionState()
	tokens := style.Tokens
	container := recipe.FieldContainer.Resolve(state, tokens)
	root := recipe.Root.Resolve(state, tokens)
	label := recipe.Label.Resolve(state, tokens)
	inputText := recipe.InputText.Resolve(state, tokens)
	placeholder := recipe.Placeholder.Resolve(state, tokens)
	helper := recipe.HelperText.Resolve(state, tokens)
	errorStyle := recipe.ErrorText.Resolve(state, tokens)
	caretStyle := recipe.Caret.Resolve(theme.StateFocused, tokens)
	selectionStyle := recipe.SelectionRange.Resolve(theme.StateFocused, tokens)
	focusRing := recipe.FocusRing.Resolve(theme.StateFocused, tokens)
	if nf.hasErrorState() {
		focusRing = errorRingMaterial(tokens)
	}
	stepperUp := recipe.StepperUp.Resolve(state, tokens)
	stepperDown := recipe.StepperDown.Resolve(state, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	fieldPath := gfx.RoundedRectPath(nf.cachedFieldBounds, nf.cachedRadius)
	if !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(fieldPath, container)...)
	}
	if nf.focusedVisible && !isTransparentMaterial(focusRing) {
		inset := maxFloat(1, nf.cachedFieldBounds.Height()*0.08)
		ringBounds := nf.cachedFieldBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, nf.cachedRadius+inset), focusRing)...)
	}
	if nf.selectionHasContent() && !isTransparentMaterial(selectionStyle) {
		cmds = append(cmds, selectionCommands(nf.selectionRects(), selectionStyle)...)
	}
	if nf.currentDisplayText() == "" {
		if nf.cachedPlaceholderLayout != nil && !isTransparentMaterial(placeholder) {
			cmds = append(cmds, nf.textCommands(nf.cachedPlaceholderLayout, nf.cachedValueBounds, placeholder)...)
		}
	} else if nf.cachedValueLayout != nil && !isTransparentMaterial(inputText) {
		cmds = append(cmds, nf.textCommands(nf.cachedValueLayout, nf.cachedValueBounds, inputText)...)
	}
	if nf.focusedVisible && nf.shouldShowCaret() && nf.cachedValueLayout != nil && !isTransparentMaterial(caretStyle) {
		caretRect := nf.cachedValueLayout.CaretRect(nf.caret)
		cmds = append(cmds, rectCommands(offsetTextRect(caretRect, nf.cachedValueBounds.Min), caretStyle)...)
	}
	if nf.cachedLabelLayout != nil && !isTransparentMaterial(label) {
		cmds = append(cmds, nf.textCommands(nf.cachedLabelLayout, nf.cachedLabelBounds, label)...)
	}
	if nf.cachedHelperBounds != (gfx.Rect{}) && nf.cachedHelperLayout != nil {
		cmds = append(cmds, nf.textCommands(nf.helperLayoutForState(), nf.cachedHelperBounds, nf.helperStyleForState(helper, errorStyle, tokens))...)
	}
	if nf.cachedErrorBounds != (gfx.Rect{}) && nf.cachedErrorLayout != nil {
		cmds = append(cmds, nf.textCommands(nf.cachedErrorLayout, nf.cachedErrorBounds, errorStyle)...)
	}
	if !isTransparentMaterial(stepperUp) {
		cmds = append(cmds, materialCommands(nf.stepperArrowPath(nf.cachedStepperUpBounds, true), stepperUp)...)
	}
	if !isTransparentMaterial(stepperDown) {
		cmds = append(cmds, materialCommands(nf.stepperArrowPath(nf.cachedStepperDownBounds, false), stepperDown)...)
	}
	return cmds
}

func (nf *NumberField) helperLayoutForState() *text.TextLayout {
	switch nf.Validation {
	case NumberFieldValidationWarning:
		if nf.WarningText != "" {
			return nf.cachedHelperLayout
		}
	case NumberFieldValidationInvalid:
		if nf.errorText() != "" {
			return nf.cachedErrorLayout
		}
	}
	return nf.cachedHelperLayout
}

func (nf *NumberField) helperStyleForState(helper, errorStyle theme.Material, tokens theme.Tokens) theme.Material {
	switch nf.Validation {
	case NumberFieldValidationWarning:
		return themedMaterialFromColor(tokens.Color.Warning)
	case NumberFieldValidationInvalid:
		return errorStyle
	default:
		return helper
	}
}

func (nf *NumberField) textCommands(layout *text.TextLayout, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if layout == nil {
		return nil
	}
	color := materialColor(material)
	brush := gfx.SolidBrush(color)
	baseOrigin := gfx.Point{X: bounds.Min.X + layout.Bounds.Min.X, Y: bounds.Min.Y + layout.Bounds.Min.Y}
	cmds := make([]gfx.Command, 0, len(layout.Lines))
	for _, line := range layout.Lines {
		lineOrigin := gfx.Point{X: baseOrigin.X + line.Bounds.Min.X, Y: baseOrigin.Y + line.Bounds.Min.Y + line.Baseline}
		for _, run := range line.Runs {
			runOrigin := gfx.Point{X: lineOrigin.X + run.Bounds.Min.X, Y: lineOrigin.Y + run.Bounds.Min.Y}
			cmds = append(cmds, gfx.DrawGlyphRun{Run: run, Origin: runOrigin, Brush: brush})
		}
	}
	return cmds
}

func (nf *NumberField) stepperArrowPath(bounds gfx.Rect, up bool) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	w := bounds.Width()
	h := bounds.Height()
	midX := bounds.Min.X + w*0.5
	top := bounds.Min.Y + h*0.28
	bottom := bounds.Min.Y + h*0.72
	left := bounds.Min.X + w*0.28
	right := bounds.Min.X + w*0.72
	if up {
		return gfx.NewPath().
			MoveTo(gfx.Point{X: midX, Y: top}).
			LineTo(gfx.Point{X: right, Y: bottom}).
			LineTo(gfx.Point{X: left, Y: bottom}).
			Close().
			Build()
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: left, Y: top}).
		LineTo(gfx.Point{X: right, Y: top}).
		LineTo(gfx.Point{X: midX, Y: bottom}).
		Close().
		Build()
}

func (nf *NumberField) hitTest(p gfx.Point) facet.HitResult {
	if nf == nil || nf.layoutRole.ArrangedBounds.IsEmpty() || !nf.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := nf.cursorShape()
	if nf.cachedStepperUpBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDStepperUp, Cursor: facet.CursorPointer}
	}
	if nf.cachedStepperDownBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDStepperDown, Cursor: facet.CursorPointer}
	}
	if nf.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDLabel, Cursor: cursor}
	}
	if nf.cachedHelperBounds.Contains(p) {
		if nf.Validation == NumberFieldValidationInvalid {
			return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDErrorText, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDHelperText, Cursor: cursor}
	}
	if nf.cachedErrorBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDErrorText, Cursor: cursor}
	}
	if nf.cachedFieldBounds.Contains(p) {
		if nf.selectionHasContent() {
			for _, rect := range nf.selectionRects() {
				if rect.Contains(p) {
					return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDSelection, Cursor: cursor}
				}
			}
		}
		if nf.focusedVisible && nf.cachedValueLayout != nil {
			caretRect := offsetTextRect(nf.cachedValueLayout.CaretRect(nf.caret), nf.cachedValueBounds.Min)
			if caretRect.Contains(p) {
				return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDCaret, Cursor: cursor}
			}
		}
		if nf.cachedPlaceholderLayout != nil && nf.currentDisplayText() == "" {
			return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDPlaceholder, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDInputText, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: numberFieldMarkIDContainer, Cursor: cursor}
}

func (nf *NumberField) cursorShape() facet.CursorShape {
	if nf.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorText
}

func (nf *NumberField) onPointer(e facet.PointerEvent) bool {
	if nf.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		nf.hovered = true
		nf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		nf.hovered = false
		nf.pressed = false
		nf.selecting = false
		nf.pressedStepper = 0
		nf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		nf.pressed = true
		nf.focusFromPointer = true
		nf.focusedVisible = false
		if nf.cachedStepperUpBounds.Contains(e.Position) {
			nf.pressedStepper = 1
			nf.invalidate(facet.DirtyProjection)
			return true
		}
		if nf.cachedStepperDownBounds.Contains(e.Position) {
			nf.pressedStepper = 2
			nf.invalidate(facet.DirtyProjection)
			return true
		}
		nf.selecting = true
		nf.editing = true
		if nf.cachedValueLayout != nil && nf.cachedFieldBounds.Contains(e.Position) {
			local := toTextPoint(gfx.Point{X: e.Position.X - nf.cachedValueBounds.Min.X, Y: e.Position.Y - nf.cachedValueBounds.Min.Y})
			nf.caret = nf.cachedValueLayout.HitTest(local)
		} else {
			nf.caret = nf.endCaret()
		}
		nf.selectionAnchor = nf.caret
		nf.textRole.Selection = nf.currentSelection(nf.cachedValueLayout)
		nf.textRole.CaretPosition = nf.caret
		nf.textRole.CaretVisible = true
		nf.syncEditingText()
		nf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		if nf.pressedStepper != 0 {
			return true
		}
		if nf.pressed && nf.selecting && nf.cachedValueLayout != nil {
			local := toTextPoint(gfx.Point{X: e.Position.X - nf.cachedValueBounds.Min.X, Y: e.Position.Y - nf.cachedValueBounds.Min.Y})
			nf.caret = nf.cachedValueLayout.HitTest(local)
			nf.textRole.Selection = nf.currentSelection(nf.cachedValueLayout)
			nf.textRole.CaretPosition = nf.caret
			nf.textRole.CaretVisible = true
			nf.invalidate(facet.DirtyProjection)
			return true
		}
		return nf.hovered
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		if nf.pressedStepper == 1 && nf.cachedStepperUpBounds.Contains(e.Position) {
			nf.incrementValue(+1)
			nf.pressedStepper = 0
			nf.pressed = false
			nf.invalidate(facet.DirtyProjection)
			return true
		}
		if nf.pressedStepper == 2 && nf.cachedStepperDownBounds.Contains(e.Position) {
			nf.incrementValue(-1)
			nf.pressedStepper = 0
			nf.pressed = false
			nf.invalidate(facet.DirtyProjection)
			return true
		}
		nf.pressed = false
		nf.selecting = false
		nf.pressedStepper = 0
		nf.commitEdit()
		nf.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (nf *NumberField) onKey(e facet.KeyEvent) bool {
	if nf.Disabled {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		if e.Kind == platform.KeyPress {
			nf.revertEdit()
			nf.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.KeyUp:
		if e.Kind == platform.KeyPress {
			nf.incrementValue(+1)
			return true
		}
	case platform.KeyDown:
		if e.Kind == platform.KeyPress {
			nf.incrementValue(-1)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress {
			nf.setValueCanonical(nf.Min)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress {
			nf.setValueCanonical(nf.Max)
			return true
		}
	case platform.KeyBackspace:
		if e.Kind == platform.KeyPress {
			return nf.deleteBackward()
		}
	case platform.KeyEnter:
		if e.Kind == platform.KeyPress {
			nf.commitEdit()
			return true
		}
	case platform.KeyA:
		if e.Kind == platform.KeyPress && e.Modifiers&platform.ModControl != 0 {
			nf.selectAll()
			return true
		}
	}
	return false
}

func (nf *NumberField) onText(e facet.TextEvent) bool {
	if nf.Disabled || nf.ReadOnly || e.Text == "" {
		return false
	}
	nf.editing = true
	nf.insertText(e.Text)
	return true
}

func (nf *NumberField) onFocusGained() {
	nf.focusedVisible = !nf.focusFromPointer
	nf.focusFromPointer = false
	nf.editing = true
	nf.syncEditingText()
	if nf.caret == (text.TextPosition{}) {
		nf.caret = nf.endCaret()
	}
	nf.textRole.CaretVisible = true
	nf.textRole.CaretPosition = nf.caret
	nf.textRole.Selection = nf.currentSelection(nf.cachedValueLayout)
	nf.invalidate(facet.DirtyProjection)
}

func (nf *NumberField) onFocusLost() {
	nf.focusedVisible = false
	nf.pressed = false
	nf.selecting = false
	nf.focusFromPointer = false
	nf.pressedStepper = 0
	nf.commitEdit()
	nf.textRole.CaretVisible = false
	nf.invalidate(facet.DirtyProjection)
}

func (nf *NumberField) interactionState() theme.InteractionState {
	switch {
	case nf.Disabled:
		return theme.StateDisabled
	case nf.pressed:
		return theme.StatePressed
	case nf.hovered:
		return theme.StateHover
	case nf.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (nf *NumberField) currentValue() float64 {
	if nf == nil || nf.Value == nil {
		return 0
	}
	return nf.Value.Get()
}

func (nf *NumberField) syncEditingText() {
	nf.editingText = nf.formatValue(nf.currentValue())
}

func (nf *NumberField) formatValue(value float64) string {
	value = nf.clampValue(value)
	if math.IsNaN(value) || math.IsInf(value, 0) {
		value = 0
	}
	if nf.Precision >= 0 {
		return strconv.FormatFloat(value, 'f', nf.Precision, 64)
	}
	return strconv.FormatFloat(value, 'f', -1, 64)
}

func (nf *NumberField) clampValue(value float64) float64 {
	if nf.Min < nf.Max {
		if value < nf.Min {
			value = nf.Min
		}
		if value > nf.Max {
			value = nf.Max
		}
	}
	if math.IsNaN(value) || math.IsInf(value, 0) {
		return 0
	}
	return value
}

func (nf *NumberField) setValueCanonical(value float64) {
	if nf == nil || nf.Value == nil {
		return
	}
	nf.Value.Set(nf.clampValue(value))
	nf.syncEditingText()
	nf.parseError = false
	nf.editing = false
	nf.textRole.Selection = text.TextRange{}
	nf.textRole.CaretPosition = nf.endCaret()
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (nf *NumberField) incrementValue(direction float64) {
	step := nf.Step
	if step == 0 {
		step = 1
	}
	nf.setValueCanonical(nf.currentValue() + step*direction)
}

func (nf *NumberField) currentDisplayText() string {
	if nf.editing || nf.parseError {
		return nf.editingText
	}
	return nf.formatValue(nf.currentValue())
}

func (nf *NumberField) displayText() string {
	return nf.currentDisplayText()
}

func (nf *NumberField) auxiliaryText() string {
	switch nf.Validation {
	case NumberFieldValidationWarning:
		if nf.WarningText != "" {
			return nf.WarningText
		}
	case NumberFieldValidationInvalid:
		if nf.ErrorText != "" {
			return nf.ErrorText
		}
	}
	if nf.parseError {
		if nf.ErrorText != "" {
			return nf.ErrorText
		}
		return "Invalid number"
	}
	return nf.HelperText
}

func (nf *NumberField) warningText() string { return nf.WarningText }
func (nf *NumberField) errorText() string   { return nf.ErrorText }

func (nf *NumberField) hasErrorState() bool {
	return nf.Validation == NumberFieldValidationInvalid || nf.parseError
}

func (nf *NumberField) selectionHasContent() bool {
	return !nf.currentSelection(nf.cachedValueLayout).IsEmpty()
}

func (nf *NumberField) selectionRects() []gfx.Rect {
	if nf.cachedValueLayout == nil {
		return nil
	}
	rects := nf.cachedValueLayout.SelectionRects(nf.currentSelection(nf.cachedValueLayout))
	out := make([]gfx.Rect, 0, len(rects))
	for _, rect := range rects {
		out = append(out, gfx.Rect{Min: gfx.Point{X: rect.Min.X + nf.cachedValueBounds.Min.X, Y: rect.Min.Y + nf.cachedValueBounds.Min.Y}, Max: gfx.Point{X: rect.Max.X + nf.cachedValueBounds.Min.X, Y: rect.Max.Y + nf.cachedValueBounds.Min.Y}})
	}
	return out
}

func (nf *NumberField) currentSelection(layout *text.TextLayout) text.TextRange {
	if layout == nil {
		return text.TextRange{}
	}
	if nf.selecting {
		start := nf.selectionAnchor.Index
		end := nf.caret.Index
		if start > end {
			start, end = end, start
		}
		return text.GraphemeRange(start, end)
	}
	if !nf.textRole.Selection.IsEmpty() {
		return clampRange(nf.textRole.Selection, layout.GraphemeCount())
	}
	return text.TextRange{}
}

func (nf *NumberField) currentCaret(layout *text.TextLayout) text.TextPosition {
	if layout == nil {
		return text.TextPosition{}
	}
	if nf.caret.Index < 0 {
		return text.GraphemePosition(0, text.AffinityDownstream)
	}
	if nf.caret.Unit == text.TextUnitGrapheme && nf.caret.Index > layout.GraphemeCount() {
		return text.GraphemePosition(layout.GraphemeCount(), text.AffinityUpstream)
	}
	if nf.caret.Unit != text.TextUnitGrapheme && nf.caret.Index > layout.RuneCount() {
		return text.RunePosition(layout.RuneCount(), text.AffinityUpstream)
	}
	return nf.caret
}

func (nf *NumberField) shouldShowCaret() bool {
	return !nf.Disabled && nf.focusedVisible
}

func (nf *NumberField) clearSelection() {
	nf.selecting = false
	nf.textRole.Selection = text.TextRange{}
	nf.caret = nf.endCaret()
	nf.textRole.CaretPosition = nf.caret
	nf.textRole.CaretVisible = true
}

func (nf *NumberField) selectAll() {
	if nf.cachedValueLayout == nil {
		return
	}
	count := nf.cachedValueLayout.GraphemeCount()
	nf.caret = text.GraphemePosition(count, text.AffinityUpstream)
	nf.selectionAnchor = text.GraphemePosition(0, text.AffinityDownstream)
	nf.selecting = true
	nf.textRole.Selection = text.GraphemeRange(0, count)
	nf.textRole.CaretPosition = nf.caret
	nf.textRole.CaretVisible = true
	nf.invalidate(facet.DirtyProjection)
}

func (nf *NumberField) setCaretAtStart(extend bool) {
	nf.ensureCaretLayout()
	nf.caret = text.GraphemePosition(0, text.AffinityDownstream)
	nf.applyCaretMove(extend)
}

func (nf *NumberField) setCaretAtEnd(extend bool) {
	nf.ensureCaretLayout()
	nf.caret = nf.endCaret()
	nf.applyCaretMove(extend)
}

func (nf *NumberField) moveCaret(forward, extend bool) {
	nf.ensureCaretLayout()
	if nf.cachedValueLayout == nil {
		return
	}
	if forward {
		nf.caret = nf.cachedValueLayout.NextPosition(nf.caret)
	} else {
		nf.caret = nf.cachedValueLayout.PrevPosition(nf.caret)
	}
	nf.applyCaretMove(extend)
}

func (nf *NumberField) applyCaretMove(extend bool) {
	if extend {
		if !nf.selecting {
			nf.selectionAnchor = nf.caret
		}
		nf.selecting = true
	} else {
		nf.selecting = false
		nf.selectionAnchor = nf.caret
	}
	nf.textRole.Selection = nf.currentSelection(nf.cachedValueLayout)
	nf.textRole.CaretPosition = nf.caret
	nf.textRole.CaretVisible = true
	nf.invalidate(facet.DirtyProjection)
}

func (nf *NumberField) deleteBackward() bool {
	if nf.ReadOnly {
		return false
	}
	if nf.cachedValueLayout == nil {
		return false
	}
	runes := []rune(nf.currentDisplayText())
	sel := nf.currentSelection(nf.cachedValueLayout).Normalized()
	if !sel.IsEmpty() {
		start, end := text.GraphemeRuneBoundsString(nf.currentDisplayText(), sel)
		runes = append(runes[:start], runes[end:]...)
		nf.caret = text.GraphemePosition(sel.Start, text.AffinityDownstream)
	} else if nf.caret.Index > 0 {
		prevCaret := text.GraphemePosition(nf.caret.Index-1, text.AffinityDownstream)
		prevRune, caretRune := text.GraphemeRuneBoundsString(nf.currentDisplayText(), text.GraphemeRange(prevCaret.Index, nf.caret.Index))
		runes = append(runes[:prevRune], runes[caretRune:]...)
		nf.caret = prevCaret
	}
	nf.editing = true
	nf.editingText = string(runes)
	nf.selectionAnchor = nf.caret
	nf.textRole.Selection = text.TextRange{}
	nf.textRole.CaretPosition = nf.caret
	nf.parseError = !nf.applyEditedText()
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	return true
}

func (nf *NumberField) insertText(textValue string) {
	if nf.ReadOnly {
		return
	}
	current := []rune(nf.currentDisplayText())
	if nf.cachedValueLayout != nil {
		sel := nf.currentSelection(nf.cachedValueLayout).Normalized()
		if sel.IsEmpty() {
			sel.Start = nf.caret.Index
			sel.End = nf.caret.Index
		}
		insert := []rune(textValue)
		start, end := text.GraphemeRuneBoundsString(nf.currentDisplayText(), sel)
		next := append(append([]rune(nil), current[:start]...), append(insert, current[end:]...)...)
		nf.editingText = string(next)
		nf.caret = text.GraphemePosition(sel.Start+text.GraphemeCountString(textValue), text.AffinityUpstream)
		nf.selectionAnchor = nf.caret
		nf.textRole.Selection = text.TextRange{}
		nf.textRole.CaretPosition = nf.caret
		nf.textRole.CaretVisible = true
		nf.parseError = !nf.applyEditedText()
		nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	nf.editingText += textValue
	nf.parseError = !nf.applyEditedText()
	nf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (nf *NumberField) applyEditedText() bool {
	textValue := strings.TrimSpace(nf.editingText)
	if textValue == "" {
		return false
	}
	value, err := strconv.ParseFloat(textValue, 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) {
		return false
	}
	if nf.Value != nil {
		nf.Value.Set(nf.clampValue(value))
	}
	return true
}

func (nf *NumberField) syncEditingTextFromValue() {
	nf.editingText = nf.formatValue(nf.currentValue())
}

func (nf *NumberField) ensureCaretLayout() {
	if nf.cachedValueLayout != nil {
		return
	}
	nf.caret = text.GraphemePosition(0, text.AffinityDownstream)
}

func (nf *NumberField) endCaret() text.TextPosition {
	if nf.cachedValueLayout == nil {
		return text.GraphemePosition(0, text.AffinityDownstream)
	}
	return nf.cachedValueLayout.PositionAtLineEnd(nf.cachedValueLayout.LineCount() - 1)
}

func (nf *NumberField) commitEdit() {
	if nf.parseError {
		nf.revertEdit()
		return
	}
	if nf.editingText != "" {
		if value, err := strconv.ParseFloat(strings.TrimSpace(nf.editingText), 64); err == nil {
			nf.setValueCanonical(value)
			return
		}
	}
	nf.syncEditingTextFromValue()
	nf.editing = false
}

func (nf *NumberField) revertEdit() {
	nf.syncEditingTextFromValue()
	nf.parseError = false
	nf.editing = false
	nf.selecting = false
	nf.caret = nf.endCaret()
	nf.textRole.Selection = text.TextRange{}
	nf.textRole.CaretPosition = nf.caret
}

func (nf *NumberField) currentSelectionText() string {
	if nf.cachedValueLayout == nil {
		return ""
	}
	return nf.currentDisplayText()
}

func (nf *NumberField) stepperActiveValue(delta float64) {
	nf.incrementValue(delta)
}

func (nf *NumberField) hasValue() bool {
	return nf.Value != nil
}

func (nf *NumberField) stepperHitPadding() float32 {
	return maxFloat(4, nf.cachedPadX*0.5)
}

func resolvedTextStyleFallback(layout *text.TextLayout) text.TextStyle {
	_ = layout
	return text.DefaultStyle()
}

type numberFieldGroupPolicy struct{}

func (numberFieldGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (numberFieldGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (numberFieldGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
