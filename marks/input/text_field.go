package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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
	textFieldMarkIDRoot        facet.MarkID = 1
	textFieldMarkIDContainer   facet.MarkID = 2
	textFieldMarkIDLabel       facet.MarkID = 3
	textFieldMarkIDInputText   facet.MarkID = 4
	textFieldMarkIDPlaceholder facet.MarkID = 5
	textFieldMarkIDHelperText  facet.MarkID = 6
	textFieldMarkIDErrorText   facet.MarkID = 7
	textFieldMarkIDCaret       facet.MarkID = 8
	textFieldMarkIDSelection   facet.MarkID = 9
	textFieldMarkIDFocusRing   facet.MarkID = 10
)

// TextFieldValidation controls auxiliary text and validation styling.
type TextFieldValidation uint8

const (
	TextFieldValidationDefault TextFieldValidation = iota
	TextFieldValidationWarning
	TextFieldValidationInvalid
)

// TextField implements the input.text_field standard mark.
type TextField struct {
	marks.Core

	Value *store.ValueStore[string]

	Label       marks.Binding[string]
	Placeholder marks.Binding[string]
	HelperText  marks.Binding[string]
	WarningText marks.Binding[string]
	ErrorText   marks.Binding[string]
	Variant     marks.Binding[uiinput.TextInputVariant]
	Validation  marks.Binding[TextFieldValidation]
	Required    marks.Binding[bool]
	Disabled    marks.Binding[bool]
	ReadOnly    marks.Binding[bool]

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	selecting        bool
	selectionAnchor  text.TextPosition
	caret            text.TextPosition

	cachedLayout            *text.TextLayout
	cachedLabelLayout       *text.TextLayout
	cachedValueLayout       *text.TextLayout
	cachedPlaceholderLayout *text.TextLayout
	cachedHelperLayout      *text.TextLayout
	cachedErrorLayout       *text.TextLayout
	cachedTokens            theme.Tokens
	cachedRecipe            shared.TextInputSlots
	cachedRootBounds        gfx.Rect
	cachedFieldBounds       gfx.Rect
	cachedLabelBounds       gfx.Rect
	cachedValueBounds       gfx.Rect
	cachedHelperBounds      gfx.Rect
	cachedErrorBounds       gfx.Rect
	cachedPadX              float32
	cachedPadY              float32
	cachedGap               float32
	cachedRadius            float32
	cachedLineHeight        float32
	cachedCaretWidth        float32
	cachedMinFieldWidth     float32
}

var _ facet.FacetImpl = (*TextField)(nil)
var _ layout.AnchorExporter = (*TextField)(nil)
var _ marks.Mark = (*TextField)(nil)

// NewTextField constructs an input.text_field mark with canonical defaults.
func NewTextField(label string, variant uiinput.TextInputVariant) *TextField {
	tf := &TextField{
		Label:       marks.Const(label),
		Placeholder: marks.Const(""),
		HelperText:  marks.Const(""),
		WarningText: marks.Const(""),
		ErrorText:   marks.Const(""),
		Variant:     marks.Const(variant),
		Validation:  marks.Const(TextFieldValidationDefault),
		Required:    marks.Const(false),
		Disabled:    marks.Const(false),
		ReadOnly:    marks.Const(false),
		Value:       store.NewValueStore(""),
	}
	tf.Core.Facet = facet.NewFacet()

	tf.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: textFieldGroupPolicy{},
	}
	tf.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := tf.measureIntrinsic(ctx, constraints)
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
	tf.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return tf.measure(ctx, constraints)
	}
	tf.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		tf.Layout.ArrangedBounds = bounds
		tf.arrange(ctx, bounds)
	}
	tf.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return tf.hitTest(p)
	}
	tf.Input.OnPointer = func(e facet.PointerEvent) bool {
		return tf.onPointer(e)
	}
	tf.Input.OnKey = func(e facet.KeyEvent) bool {
		return tf.onKey(e)
	}
	tf.Input.OnText = func(e facet.TextEvent) bool {
		return tf.onText(e)
	}
	tf.Focus.Focusable = func() bool {
		return !tf.Disabled.Get()
	}
	tf.Focus.TabIndex = 0
	tf.Focus.OnFocusGained = func() {
		tf.onFocusGained()
	}
	tf.Focus.OnFocusLost = func() {
		tf.onFocusLost()
	}
	tf.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return tf.buildCommands(tf.Layout.ArrangedBounds, ctx.Runtime)
	}
	tf.textRole.IMEEnabled = true
	tf.RegisterRoles()
	tf.AddRole(&tf.textRole)
	return tf
}

// Base satisfies facet.FacetImpl.
func (tf *TextField) Base() *facet.Facet {
	tf.Facet.BindImpl(tf)
	return &tf.Facet
}

// Descriptor satisfies marks.Mark.
func (tf *TextField) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "input", TypeName: "text_field"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (tf *TextField) AccessibilityRole() string {
	return "textbox"
}

// AccessibleName reports the accessible name source required by the spec.
func (tf *TextField) AccessibleName() string {
	if tf == nil {
		return ""
	}
	return tf.Label.Get()
}

// ExportAnchors publishes the field anchor set.
func (tf *TextField) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if tf == nil {
		return nil
	}
	out := tf.Core.DefaultAnchors(tf.Layout.ArrangedBounds, ctx)
	if out == nil {
		return nil
	}
	if tf.textRole.Layout != nil {
		out["baseline"] = gfx.Point{
			X: tf.cachedValueBounds.Min.X,
			Y: tf.cachedValueBounds.Min.Y + tf.textRole.Layout.Baseline,
		}
	} else {
		out["baseline"] = out["bounds_top_left"]
	}
	return out
}

// Children returns the facet's immediate child list.
func (tf *TextField) Children() []facet.GroupChild {
	return nil
}

func (tf *TextField) OnAttach(ctx facet.AttachContext) {
	if tf.Value == nil {
		tf.Value = store.NewValueStore("")
	}
	tf.Core.OnAttach()
	facet.Store(facet.Subscribe(tf), &tf.Value.OnChange, tf.Value.Version, func(signal.Change[string]) {
		tf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

func (tf *TextField) OnActivate()   { tf.Core.OnActivate() }
func (tf *TextField) OnDeactivate() { tf.Core.OnDeactivate() }
func (tf *TextField) OnDetach() {
	tf.Core.OnDetach()
	tf.cachedLayout = nil
	tf.cachedLabelLayout = nil
	tf.cachedValueLayout = nil
	tf.cachedPlaceholderLayout = nil
	tf.cachedHelperLayout = nil
	tf.cachedErrorLayout = nil
	tf.cachedTokens = theme.Tokens{}
	tf.cachedRecipe = shared.TextInputSlots{}
	tf.cachedRootBounds = gfx.Rect{}
	tf.cachedFieldBounds = gfx.Rect{}
	tf.cachedLabelBounds = gfx.Rect{}
	tf.cachedValueBounds = gfx.Rect{}
	tf.cachedHelperBounds = gfx.Rect{}
	tf.cachedErrorBounds = gfx.Rect{}
	tf.focusedVisible = false
	tf.hovered = false
	tf.pressed = false
	tf.selecting = false
	tf.caret = text.TextPosition{}
	tf.selectionAnchor = text.TextPosition{}
}

func (tf *TextField) invalidate(flags facet.DirtyFlags) {
	if tf == nil {
		return
	}
	tf.Facet.Invalidate(flags)
}

func (tf *TextField) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := tf.resolveTheme(ctx)
	if !ok {
		tf.cachedLayout = nil
		tf.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	tf.cachedTokens = resolved.TokenSet()
	tf.cachedRecipe = recipe
	tf.cachedPadX = float32(resolved.Spacing(theme.SpacingM))
	tf.cachedPadY = float32(resolved.Spacing(theme.SpacingS))
	tf.cachedGap = float32(resolved.Spacing(theme.SpacingXS))
	tf.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	tf.cachedCaretWidth = resolved.TokenSet().Spacing.BorderWeight
	if tf.cachedCaretWidth <= 0 {
		tf.cachedCaretWidth = 1
	}
	tf.cachedMinFieldWidth = float32(resolved.Spacing(theme.SpacingXL)) * 6
	layout, labelLayout, valueLayout, placeholderLayout, helperLayout, errorLayout := tf.resolveLayouts(ctx, constraints, resolved)
	if layout == nil {
		tf.cachedLayout = nil
		tf.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	tf.cachedLayout = layout
	tf.cachedLabelLayout = labelLayout
	tf.cachedValueLayout = valueLayout
	tf.cachedPlaceholderLayout = placeholderLayout
	tf.cachedHelperLayout = helperLayout
	tf.cachedErrorLayout = errorLayout
	tf.textRole.Layout = valueLayout
	tf.textRole.Selection = tf.currentSelection(valueLayout)
	tf.textRole.CaretPosition = tf.currentCaret(valueLayout)
	tf.textRole.CaretVisible = tf.shouldShowCaret()

	size := gfx.Size{W: layout.Bounds.Width(), H: layout.Bounds.Height()}
	tf.Layout.MeasuredSize = size
	tf.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return tf.Layout.MeasuredResult
}

func (tf *TextField) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return tf.measure(ctx, constraints).Size
}

func (tf *TextField) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	tf.cachedRootBounds = bounds
	if tf.cachedLayout == nil {
		tf.Layout.ArrangedBounds = bounds
		return
	}
	layout := tf.cachedLayout
	tf.cachedLabelBounds = gfx.Rect{}
	tf.cachedFieldBounds = gfx.Rect{}
	tf.cachedValueBounds = gfx.Rect{}
	tf.cachedHelperBounds = gfx.Rect{}
	tf.cachedErrorBounds = gfx.Rect{}

	labelH := float32(0)
	if tf.cachedLabelLayout != nil {
		labelH = tf.cachedLabelLayout.Bounds.Height()
	}
	valueH := text.MaxHeight(tf.cachedValueLayout, tf.cachedPlaceholderLayout)
	helperH := text.MaxHeight(tf.cachedHelperLayout, tf.cachedErrorLayout)
	gap := tf.cachedGap

	labelY := bounds.Min.Y
	if tf.cachedLabelLayout != nil {
		tf.cachedLabelBounds = gfx.RectFromXYWH(bounds.Min.X, labelY, bounds.Width(), labelH)
		labelY += labelH + gap
	}
	fieldH := valueH + tf.cachedPadY*2
	if fieldH <= 0 {
		fieldH = resolvedMinFieldHeight()
	}
	fieldY := labelY
	contentH := valueH
	contentTop := fieldY + maxFloat(0, (fieldH-contentH)/2)
	tf.cachedFieldBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY, bounds.Width(), fieldH)
	tf.cachedValueBounds = gfx.RectFromXYWH(bounds.Min.X+tf.cachedPadX, contentTop, bounds.Width()-tf.cachedPadX*2, valueH)
	if tf.cachedValueBounds.Width() < 0 {
		tf.cachedValueBounds = gfx.RectFromXYWH(bounds.Min.X, contentTop, 0, valueH)
	}
	if tf.Validation.Get() == TextFieldValidationWarning && tf.WarningText.Get() != "" {
		tf.cachedHelperBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	} else if tf.Validation.Get() == TextFieldValidationInvalid && tf.ErrorText.Get() != "" {
		tf.cachedErrorBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	} else if tf.cachedHelperLayout != nil {
		tf.cachedHelperBounds = gfx.RectFromXYWH(bounds.Min.X, fieldY+fieldH+gap, bounds.Width(), helperH)
	}
	tf.Layout.ArrangedBounds = bounds
	_ = layout
}

func (tf *TextField) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.TextInputSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiinput.ResolveTextInputRecipe(style, tf.Variant.Get())
	return resolved, slots, true
}

func (tf *TextField) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.TextInputSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: tf.cachedTokens}, tf.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, tf.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveTextInputRecipe(style, tf.Variant.Get())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: tf.cachedTokens}, tf.cachedRecipe
}

func (tf *TextField) resolveLayouts(ctx facet.MeasureContext, constraints facet.Constraints, resolved theme.ResolvedContext) (*text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout, *text.TextLayout) {
	shaper := tf.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil, nil, nil, nil, nil, nil
	}
	shaper.SetContentScale(ctx.ContentScale)
	labelStyle := resolved.TextStyle(theme.TextLabelM)
	valueStyle := resolved.TextStyle(theme.TextBodyM)
	helperStyle := resolved.TextStyle(theme.TextBodyS)
	labelLayout := shaper.ShapeTruncated(tf.Label.Get(), labelStyle, tf.availableWidth(constraints, resolved))
	valueText := tf.currentValue()
	valueLayout := shaper.ShapeTruncated(valueText, valueStyle, tf.availableWidth(constraints, resolved))
	placeholderLayout := shaper.ShapeTruncated(tf.placeholderText(), valueStyle, tf.availableWidth(constraints, resolved))
	helperText := tf.auxiliaryText()
	helperLayout := shaper.ShapeTruncated(helperText, helperStyle, tf.availableWidth(constraints, resolved))
	errorLayout := shaper.ShapeTruncated(tf.errorText(), helperStyle, tf.availableWidth(constraints, resolved))
	valueLayout = tf.ensureTextLayout(valueLayout, valueStyle, true)
	if valueLayout == nil {
		return nil, nil, nil, nil, nil, nil
	}
	placeholderLayout = tf.ensureTextLayout(placeholderLayout, valueStyle, true)
	labelLayout = tf.ensureTextLayout(labelLayout, labelStyle, false)
	helperLayout = tf.ensureTextLayout(helperLayout, helperStyle, false)
	errorLayout = tf.ensureTextLayout(errorLayout, helperStyle, false)

	fieldInnerWidth := valueLayout.Bounds.Width()
	if placeholderLayout != nil {
		fieldInnerWidth = maxFloat(fieldInnerWidth, placeholderLayout.Bounds.Width())
	}
	if fieldInnerWidth < tf.cachedMinFieldWidth-tf.cachedPadX*2 {
		fieldInnerWidth = tf.cachedMinFieldWidth - tf.cachedPadX*2
	}
	if constraints.MaxSize.W > 0 && fieldInnerWidth+tf.cachedPadX*2 > constraints.MaxSize.W {
		fieldInnerWidth = maxFloat(0, constraints.MaxSize.W-tf.cachedPadX*2)
	}
	if fieldInnerWidth < 0 {
		fieldInnerWidth = 0
	}
	valueLayout = shaper.ShapeTruncated(valueText, valueStyle, fieldInnerWidth)
	placeholderLayout = shaper.ShapeTruncated(tf.placeholderText(), valueStyle, fieldInnerWidth)
	valueLayout = tf.ensureTextLayout(valueLayout, valueStyle, true)
	placeholderLayout = tf.ensureTextLayout(placeholderLayout, valueStyle, true)

	fieldH := valueLayout.Bounds.Height() + tf.cachedPadY*2
	if placeholderLayout != nil {
		fieldH = maxFloat(fieldH, placeholderLayout.Bounds.Height()+tf.cachedPadY*2)
	}
	if fieldH < resolvedMinFieldHeightFromStyle(resolved, valueStyle) {
		fieldH = resolvedMinFieldHeightFromStyle(resolved, valueStyle)
	}
	labelH := float32(0)
	if labelLayout != nil {
		labelH = labelLayout.Bounds.Height()
	}
	helperH := float32(0)
	if tf.Validation.Get() == TextFieldValidationInvalid && tf.errorText() != "" && errorLayout != nil {
		helperH = errorLayout.Bounds.Height()
	} else if tf.Validation.Get() == TextFieldValidationWarning && tf.warningText() != "" && helperLayout != nil {
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
	totalH := labelH + fieldH + helperH + float32(gapCount)*tf.cachedGap
	if labelLayout == nil {
		totalH -= tf.cachedGap
	}
	if helperH == 0 {
		totalH -= tf.cachedGap
	}
	if totalH < fieldH {
		totalH = fieldH
	}
	width := tf.cachedMinFieldWidth
	if labelLayout != nil {
		width = maxFloat(width, labelLayout.Bounds.Width())
	}
	width = maxFloat(width, fieldInnerWidth+tf.cachedPadX*2)
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

func (tf *TextField) availableWidth(constraints facet.Constraints, resolved theme.ResolvedContext) float32 {
	if constraints.MaxSize.W > 0 {
		return constraints.MaxSize.W
	}
	return float32(resolved.Spacing(theme.SpacingXL)) * 6
}

func (tf *TextField) newShaper(runtime any) *text.Shaper {
	registry := tf.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (tf *TextField) fontRegistry(runtime any) *text.FontRegistry {
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

func (tf *TextField) ensureTextLayout(layout *text.TextLayout, style text.TextStyle, allowEmpty bool) *text.TextLayout {
	if layout == nil || len(layout.Lines) > 0 {
		return layout
	}
	if allowEmpty {
		return emptyCaretLayout(style)
	}
	return nil
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func materialColor(material theme.Material) gfx.Color {
	return theme.MaterialColor(material)
}

func isTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}

func offsetTextRect(rect text.Rect, origin gfx.Point) gfx.Rect {
	return gfx.Rect{
		Min: gfx.Point{X: rect.Min.X + origin.X, Y: rect.Min.Y + origin.Y},
		Max: gfx.Point{X: rect.Max.X + origin.X, Y: rect.Max.Y + origin.Y},
	}
}

func toTextPoint(p gfx.Point) text.Point {
	return text.Point{X: p.X, Y: p.Y}
}

func emptyCaretLayout(style text.TextStyle) *text.TextLayout {
	lineHeight := style.Size * 1.2
	if lineHeight <= 0 {
		lineHeight = 16
	}
	baseline := lineHeight * 0.8
	if baseline <= 0 {
		baseline = lineHeight
	}
	line := text.ShapedLine{
		Bounds:    text.RectFromXYWH(0, 0, 0, lineHeight),
		Baseline:  baseline,
		FirstRune: 0,
		RuneCount: 0,
	}
	return &text.TextLayout{
		Lines:      []text.ShapedLine{line},
		Bounds:     text.RectFromXYWH(0, 0, 0, lineHeight),
		LineHeight: lineHeight,
		Baseline:   baseline,
	}
}

func (tf *TextField) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if tf == nil || bounds.IsEmpty() {
		return nil
	}
	style, recipe := tf.resolveProjectionTheme(runtime)
	state := tf.interactionState()
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
	if tf.Validation.Get() == TextFieldValidationInvalid {
		focusRing = errorRingMaterial(tokens)
	}
	cmds := make([]gfx.Command, 0, 24)
	rootPath := gfx.RectPath(bounds)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(rootPath, root)...)
	}
	fieldPath := gfx.RoundedRectPath(tf.cachedFieldBounds, tf.cachedRadius)
	if !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(fieldPath, container)...)
	}
	if tf.focusedVisible && !isTransparentMaterial(focusRing) {
		inset := maxFloat(1, tf.cachedFieldBounds.Height()*0.08)
		ringBounds := tf.cachedFieldBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, tf.cachedRadius+inset), focusRing)...)
	}
	if tf.selectionHasContent() && !isTransparentMaterial(selectionStyle) {
		cmds = append(cmds, selectionCommands(tf.selectionRects(), selectionStyle)...)
	}
	if tf.valueIsEmpty() {
		if tf.cachedPlaceholderLayout != nil && !isTransparentMaterial(placeholder) {
			cmds = append(cmds, primitive.TextLayoutCommands(tf.cachedPlaceholderLayout, tf.cachedValueBounds, gfx.SolidBrush(materialColor(placeholder)))...)
		}
	} else if tf.cachedValueLayout != nil && !isTransparentMaterial(inputText) {
		cmds = append(cmds, primitive.TextLayoutCommands(tf.cachedValueLayout, tf.cachedValueBounds, gfx.SolidBrush(materialColor(inputText)))...)
	}
	if tf.focusedVisible && tf.shouldShowCaret() && tf.cachedValueLayout != nil && !isTransparentMaterial(caretStyle) {
		caretRect := tf.cachedValueLayout.CaretRect(tf.caret)
		cmds = append(cmds, rectCommands(offsetTextRect(caretRect, tf.cachedValueBounds.Min), caretStyle)...)
	}
	if tf.cachedLabelLayout != nil && !isTransparentMaterial(label) {
		cmds = append(cmds, primitive.TextLayoutCommands(tf.cachedLabelLayout, tf.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
	}
	if tf.cachedHelperBounds != (gfx.Rect{}) && tf.cachedHelperLayout != nil {
		helperLayout := tf.cachedHelperLayout
		helperMaterial := helper
		switch tf.Validation.Get() {
		case TextFieldValidationWarning:
			if tf.WarningText.Get() != "" {
				helperMaterial = themedMaterialFromColor(tokens.Color.Warning)
			}
		case TextFieldValidationInvalid:
			if tf.ErrorText.Get() != "" {
				helperLayout = tf.cachedErrorLayout
				helperMaterial = errorStyle
			}
		}
		cmds = append(cmds, primitive.TextLayoutCommands(helperLayout, tf.cachedHelperBounds, gfx.SolidBrush(materialColor(helperMaterial)))...)
	}
	if tf.cachedErrorBounds != (gfx.Rect{}) && tf.cachedErrorLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(tf.cachedErrorLayout, tf.cachedErrorBounds, gfx.SolidBrush(materialColor(errorStyle)))...)
	}
	return cmds
}

func themedMaterialFromColor(color gfx.Color) theme.Material {
	return theme.FromToken(color)
}

func errorRingMaterial(tokens theme.Tokens) theme.Material {
	return theme.Material{
		Strokes: []theme.MaterialStroke{{
			Paint: theme.Fill{
				Type:    theme.FillSolid,
				Color:   tokens.Color.Error,
				Opacity: 1,
			},
			Width: 2,
			Cap:   theme.CapRound,
			Join:  theme.JoinRound,
		}},
		Opacity: 1,
	}
}

func selectionCommands(rects []gfx.Rect, material theme.Material) []gfx.Command {
	if len(rects) == 0 || isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(rects))
	brush := gfx.SolidBrush(materialColor(material))
	for _, rect := range rects {
		if rect.IsEmpty() {
			continue
		}
		cmds = append(cmds, gfx.FillRect{Rect: rect, Brush: brush})
	}
	return cmds
}

func rectCommands(rect gfx.Rect, material theme.Material) []gfx.Command {
	if rect.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes))
	path := gfx.RectPath(rect)
	for _, fill := range material.Fills {
		if fill.Type != theme.FillSolid || fill.Color.A <= 0 || fill.Opacity <= 0 {
			continue
		}
		cmds = append(cmds, gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color)})
	}
	for _, stroke := range material.Strokes {
		if stroke.Paint.Type != theme.FillSolid || stroke.Paint.Color.A <= 0 || stroke.Width <= 0 {
			continue
		}
		cmds = append(cmds, gfx.StrokePath{
			Path:   path,
			Brush:  gfx.SolidBrush(stroke.Paint.Color),
			Stroke: gfx.StrokeStyle{Width: stroke.Width, Cap: gfx.LineCapRound, Join: gfx.LineJoinRound, MiterLimit: 10},
		})
	}
	return cmds
}

func (tf *TextField) hitTest(p gfx.Point) facet.HitResult {
	if tf == nil || tf.Layout.ArrangedBounds.IsEmpty() || !tf.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := tf.cursorShape()
	if tf.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: textFieldMarkIDLabel, Cursor: cursor}
	}
	if tf.cachedHelperBounds.Contains(p) {
		if tf.Validation.Get() == TextFieldValidationInvalid {
			return facet.HitResult{Hit: true, MarkID: textFieldMarkIDErrorText, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: textFieldMarkIDHelperText, Cursor: cursor}
	}
	if tf.cachedErrorBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: textFieldMarkIDErrorText, Cursor: cursor}
	}
	if tf.cachedFieldBounds.Contains(p) {
		if tf.selectionHasContent() {
			for _, rect := range tf.selectionRects() {
				if rect.Contains(p) {
					return facet.HitResult{Hit: true, MarkID: textFieldMarkIDSelection, Cursor: cursor}
				}
			}
		}
		if tf.focusedVisible && tf.cachedValueLayout != nil {
			caretRect := offsetTextRect(tf.cachedValueLayout.CaretRect(tf.caret), tf.cachedValueBounds.Min)
			if caretRect.Contains(p) {
				return facet.HitResult{Hit: true, MarkID: textFieldMarkIDCaret, Cursor: cursor}
			}
		}
		if tf.valueIsEmpty() && tf.cachedPlaceholderLayout != nil {
			return facet.HitResult{Hit: true, MarkID: textFieldMarkIDPlaceholder, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: textFieldMarkIDInputText, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: textFieldMarkIDContainer, Cursor: cursor}
}

func (tf *TextField) cursorShape() facet.CursorShape {
	if tf.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorText
}

func (tf *TextField) onPointer(e facet.PointerEvent) bool {
	if tf.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		tf.hovered = true
		tf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		tf.hovered = false
		tf.pressed = false
		tf.selecting = false
		tf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		tf.pressed = true
		tf.focusFromPointer = true
		tf.focusedVisible = false
		tf.selecting = true
		if tf.cachedValueLayout != nil && tf.cachedFieldBounds.Contains(e.Position) {
			local := toTextPoint(gfx.Point{X: e.Position.X - tf.cachedValueBounds.Min.X, Y: e.Position.Y - tf.cachedValueBounds.Min.Y})
			tf.caret = tf.cachedValueLayout.HitTest(local)
		} else {
			tf.caret = tf.endCaret()
		}
		tf.selectionAnchor = tf.caret
		tf.textRole.Selection = tf.currentSelection(tf.cachedValueLayout)
		tf.textRole.CaretPosition = tf.caret
		tf.textRole.CaretVisible = true
		tf.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerMove:
		if tf.pressed && tf.selecting && tf.cachedValueLayout != nil {
			local := toTextPoint(gfx.Point{X: e.Position.X - tf.cachedValueBounds.Min.X, Y: e.Position.Y - tf.cachedValueBounds.Min.Y})
			tf.caret = tf.cachedValueLayout.HitTest(local)
			tf.textRole.Selection = tf.currentSelection(tf.cachedValueLayout)
			tf.textRole.CaretPosition = tf.caret
			tf.textRole.CaretVisible = true
			tf.invalidate(facet.DirtyProjection)
			return true
		}
		return tf.hovered
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		tf.pressed = false
		tf.selecting = false
		tf.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (tf *TextField) onKey(e facet.KeyEvent) bool {
	if tf.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		if e.Kind == platform.KeyPress {
			tf.clearSelection()
			tf.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress {
			tf.moveCaret(false, e.Modifiers&platform.ModShift != 0)
			return true
		}
	case platform.KeyRight:
		if e.Kind == platform.KeyPress {
			tf.moveCaret(true, e.Modifiers&platform.ModShift != 0)
			return true
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress {
			tf.setCaretAtStart(e.Modifiers&platform.ModShift != 0)
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress {
			tf.setCaretAtEnd(e.Modifiers&platform.ModShift != 0)
			return true
		}
	case platform.KeyBackspace:
		if e.Kind == platform.KeyPress {
			return tf.deleteBackward()
		}
	case platform.KeyA:
		if e.Kind == platform.KeyPress && e.Modifiers&platform.ModControl != 0 {
			tf.selectAll()
			return true
		}
	}
	return false
}

func (tf *TextField) onText(e facet.TextEvent) bool {
	if tf.Disabled.Get() || tf.ReadOnly.Get() || e.Text == "" {
		return false
	}
	tf.insertText(e.Text)
	return true
}

func (tf *TextField) onFocusGained() {
	tf.focusedVisible = !tf.focusFromPointer
	tf.focusFromPointer = false
	if tf.caret == (text.TextPosition{}) {
		tf.caret = tf.endCaret()
	}
	tf.textRole.CaretVisible = true
	tf.textRole.CaretPosition = tf.caret
	tf.textRole.Selection = tf.currentSelection(tf.cachedValueLayout)
	tf.invalidate(facet.DirtyProjection)
}

func (tf *TextField) onFocusLost() {
	tf.focusedVisible = false
	tf.pressed = false
	tf.selecting = false
	tf.focusFromPointer = false
	tf.textRole.CaretVisible = false
	tf.invalidate(facet.DirtyProjection)
}

func (tf *TextField) interactionState() theme.InteractionState {
	switch {
	case tf.Disabled.Get():
		return theme.StateDisabled
	case tf.pressed:
		return theme.StatePressed
	case tf.hovered:
		return theme.StateHover
	case tf.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (tf *TextField) currentValue() string {
	if tf == nil || tf.Value == nil {
		return ""
	}
	return tf.Value.Get()
}

func (tf *TextField) valueIsEmpty() bool {
	return tf.currentValue() == ""
}

func (tf *TextField) placeholderText() string {
	return tf.Placeholder.Get()
}

func (tf *TextField) warningText() string {
	return tf.WarningText.Get()
}

func (tf *TextField) errorText() string {
	return tf.ErrorText.Get()
}

func (tf *TextField) auxiliaryText() string {
	switch tf.Validation.Get() {
	case TextFieldValidationWarning:
		if tf.WarningText.Get() != "" {
			return tf.WarningText.Get()
		}
	case TextFieldValidationInvalid:
		if tf.ErrorText.Get() != "" {
			return tf.ErrorText.Get()
		}
	}
	return tf.HelperText.Get()
}

func (tf *TextField) selectionHasContent() bool {
	return !tf.currentSelection(tf.cachedValueLayout).IsEmpty()
}

func (tf *TextField) selectionRects() []gfx.Rect {
	if tf.cachedValueLayout == nil {
		return nil
	}
	rects := tf.cachedValueLayout.SelectionRects(tf.currentSelection(tf.cachedValueLayout))
	out := make([]gfx.Rect, 0, len(rects))
	for _, rect := range rects {
		out = append(out, gfx.Rect{
			Min: gfx.Point{X: rect.Min.X + tf.cachedValueBounds.Min.X, Y: rect.Min.Y + tf.cachedValueBounds.Min.Y},
			Max: gfx.Point{X: rect.Max.X + tf.cachedValueBounds.Min.X, Y: rect.Max.Y + tf.cachedValueBounds.Min.Y},
		})
	}
	return out
}

func (tf *TextField) currentSelection(layout *text.TextLayout) text.TextRange {
	if layout == nil {
		return text.TextRange{}
	}
	if tf.selecting {
		start := tf.selectionAnchor.Index
		end := tf.caret.Index
		if start > end {
			start, end = end, start
		}
		return text.GraphemeRange(start, end)
	}
	if !tf.textRole.Selection.IsEmpty() {
		return clampRange(tf.textRole.Selection, layout.GraphemeCount())
	}
	return text.TextRange{}
}

func (tf *TextField) currentCaret(layout *text.TextLayout) text.TextPosition {
	if layout == nil {
		return text.TextPosition{}
	}
	if tf.caret.Index < 0 {
		return text.GraphemePosition(0, text.AffinityDownstream)
	}
	if tf.caret.Unit == text.TextUnitGrapheme && tf.caret.Index > layout.GraphemeCount() {
		return text.GraphemePosition(layout.GraphemeCount(), text.AffinityUpstream)
	}
	if tf.caret.Unit != text.TextUnitGrapheme && tf.caret.Index > layout.RuneCount() {
		return text.RunePosition(layout.RuneCount(), text.AffinityUpstream)
	}
	return tf.caret
}

func (tf *TextField) shouldShowCaret() bool {
	return !tf.Disabled.Get() && tf.focusedVisible
}

func (tf *TextField) clearSelection() {
	tf.selecting = false
	tf.textRole.Selection = text.TextRange{}
	tf.caret = tf.endCaret()
	tf.textRole.CaretPosition = tf.caret
	tf.textRole.CaretVisible = true
}

func (tf *TextField) selectAll() {
	if tf.cachedValueLayout == nil {
		return
	}
	count := tf.cachedValueLayout.GraphemeCount()
	tf.caret = text.GraphemePosition(count, text.AffinityUpstream)
	tf.selectionAnchor = text.GraphemePosition(0, text.AffinityDownstream)
	tf.selecting = true
	tf.textRole.Selection = text.GraphemeRange(0, count)
	tf.textRole.CaretPosition = tf.caret
	tf.textRole.CaretVisible = true
	tf.invalidate(facet.DirtyProjection)
}

func (tf *TextField) setCaretAtStart(extend bool) {
	tf.ensureCaretLayout()
	tf.caret = text.GraphemePosition(0, text.AffinityDownstream)
	tf.applyCaretMove(extend)
}

func (tf *TextField) setCaretAtEnd(extend bool) {
	tf.ensureCaretLayout()
	tf.caret = tf.endCaret()
	tf.applyCaretMove(extend)
}

func (tf *TextField) moveCaret(forward, extend bool) {
	tf.ensureCaretLayout()
	if tf.cachedValueLayout == nil {
		return
	}
	if forward {
		tf.caret = tf.cachedValueLayout.NextPosition(tf.caret)
	} else {
		tf.caret = tf.cachedValueLayout.PrevPosition(tf.caret)
	}
	tf.applyCaretMove(extend)
}

func (tf *TextField) applyCaretMove(extend bool) {
	if extend {
		if !tf.selecting {
			tf.selectionAnchor = tf.caret
		}
		tf.selecting = true
	} else {
		tf.selecting = false
		tf.selectionAnchor = tf.caret
	}
	tf.textRole.Selection = tf.currentSelection(tf.cachedValueLayout)
	tf.textRole.CaretPosition = tf.caret
	tf.textRole.CaretVisible = true
	tf.invalidate(facet.DirtyProjection)
}

func (tf *TextField) deleteBackward() bool {
	if tf.ReadOnly.Get() {
		return false
	}
	if tf.cachedValueLayout == nil {
		return false
	}
	value := []rune(tf.currentValue())
	sel := tf.currentSelection(tf.cachedValueLayout).Normalized()
	if !sel.IsEmpty() {
		start, end := text.GraphemeRuneBoundsString(tf.currentValue(), sel)
		tf.setValueRunes(append(value[:start], value[end:]...))
		tf.caret = text.GraphemePosition(sel.Start, text.AffinityDownstream)
		tf.textRole.Selection = text.TextRange{}
		tf.textRole.CaretPosition = tf.caret
		tf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	}
	if tf.caret.Index <= 0 {
		return true
	}
	prevCaret := text.GraphemePosition(tf.caret.Index-1, text.AffinityDownstream)
	prevRune, caretRune := text.GraphemeRuneBoundsString(tf.currentValue(), text.GraphemeRange(prevCaret.Index, tf.caret.Index))
	tf.setValueRunes(append(value[:prevRune], value[caretRune:]...))
	tf.caret = prevCaret
	tf.textRole.Selection = text.TextRange{}
	tf.textRole.CaretPosition = tf.caret
	tf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	return true
}

func (tf *TextField) insertText(textValue string) {
	if tf.ReadOnly.Get() || tf.Value == nil {
		return
	}
	if tf.cachedValueLayout == nil {
		tf.Value.Set(tf.currentValue() + textValue)
		return
	}
	value := []rune(tf.currentValue())
	sel := tf.currentSelection(tf.cachedValueLayout).Normalized()
	if sel.IsEmpty() {
		sel.Start = tf.caret.Index
		sel.End = tf.caret.Index
	}
	insert := []rune(textValue)
	start, end := text.GraphemeRuneBoundsString(tf.currentValue(), sel)
	next := append(append([]rune(nil), value[:start]...), append(insert, value[end:]...)...)
	tf.setValueRunes(next)
	newIndex := sel.Start + text.GraphemeCountString(textValue)
	tf.caret = text.GraphemePosition(newIndex, text.AffinityUpstream)
	tf.textRole.Selection = text.TextRange{}
	tf.textRole.CaretPosition = tf.caret
	tf.textRole.CaretVisible = true
	tf.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (tf *TextField) setValueRunes(runes []rune) {
	if tf == nil || tf.Value == nil {
		return
	}
	tf.Value.Set(string(runes))
}

func (tf *TextField) ensureCaretLayout() {
	if tf.cachedValueLayout != nil {
		return
	}
	tf.caret = text.GraphemePosition(0, text.AffinityDownstream)
}

func (tf *TextField) endCaret() text.TextPosition {
	if tf.cachedValueLayout == nil {
		return text.GraphemePosition(0, text.AffinityDownstream)
	}
	return tf.cachedValueLayout.PositionAtLineEnd(tf.cachedValueLayout.LineCount() - 1)
}

func clampRange(r text.TextRange, max int) text.TextRange {
	if max < 0 {
		max = 0
	}
	if r.Start < 0 {
		r.Start = 0
	}
	if r.End < 0 {
		r.End = 0
	}
	if r.Start > max {
		r.Start = max
	}
	if r.End > max {
		r.End = max
	}
	return r.Normalized()
}

func resolvedMinFieldHeight() float32 {
	return 32
}

func resolvedMinFieldHeightFromStyle(resolved theme.ResolvedContext, style text.TextStyle) float32 {
	lineHeight := style.Size * 1.2
	if lineHeight <= 0 {
		lineHeight = 16
	}
	return lineHeight + float32(resolved.Spacing(theme.SpacingS))*2
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

type textFieldGroupPolicy struct{}

func (textFieldGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (textFieldGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (textFieldGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
