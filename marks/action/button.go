package action

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
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
	buttonMarkIDRoot       facet.MarkID = 1
	buttonMarkIDContainer  facet.MarkID = 2
	buttonMarkIDLabel      facet.MarkID = 3
	buttonMarkIDLeading    facet.MarkID = 4
	buttonMarkIDTrailing   facet.MarkID = 5
	buttonMarkIDFocusRing  facet.MarkID = 6
	buttonMarkIDStateLayer facet.MarkID = 7
)

// Button implements the action.button standard mark.
type Button struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[signal.Unit]

	Label           string
	Variant         uiinput.ButtonVariant
	LeadingIconRef  string
	TrailingIconRef string
	Disabled        bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	spaceDown        bool
	enterDown        bool

	cachedLayout        *text.TextLayout
	cachedLabelStyle    text.TextStyle
	cachedLabelBounds   gfx.Rect
	cachedLeadingBox    gfx.Rect
	cachedTrailingBox   gfx.Rect
	cachedPadX          float32
	cachedPadY          float32
	cachedGap           float32
	cachedRadius        float32
	cachedTokens        theme.Tokens
	cachedRecipe        shared.ButtonSlots
	cachedLeadingAsset  runtimepkg.IconAsset
	cachedTrailingAsset runtimepkg.IconAsset
}

var _ facet.FacetImpl = (*Button)(nil)
var _ layout.AnchorExporter = (*Button)(nil)

// NewButton constructs a button with canonical defaults.
func NewButton(label string, variant uiinput.ButtonVariant) *Button {
	b := &Button{
		Facet:   facet.NewFacet(),
		Label:   label,
		Variant: variant,
	}
	b.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: buttonGroupPolicy{},
	}
	b.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := b.measureIntrinsic(ctx, constraints)
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
	b.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return b.measure(ctx, constraints)
	}
	b.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		b.layoutRole.ArrangedBounds = bounds
		b.arrange(ctx, bounds)
	}
	b.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := b.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	b.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := b.buildCommands(b.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	b.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return b.hitTest(p)
	}
	b.inputRole.OnPointer = func(e facet.PointerEvent) bool {
		return b.onPointer(e)
	}
	b.inputRole.OnKey = func(e facet.KeyEvent) bool {
		return b.onKey(e)
	}
	b.focusRole.Focusable = func() bool {
		return !b.Disabled
	}
	b.focusRole.OnFocusGained = func() {
		b.onFocusGained()
	}
	b.focusRole.OnFocusLost = func() {
		b.onFocusLost()
	}
	b.AddRole(&b.layoutRole)
	b.AddRole(&b.renderRole)
	b.AddRole(&b.projectionRole)
	b.AddRole(&b.hitRole)
	b.AddRole(&b.inputRole)
	b.AddRole(&b.focusRole)
	b.AddRole(&b.textRole)
	return b
}

// Base satisfies facet.FacetImpl.
func (b *Button) Base() *facet.Facet {
	b.Facet.BindImpl(b)
	return &b.Facet
}

// AccessibilityRole reports the semantic role required by the mark spec.
func (b *Button) AccessibilityRole() string {
	return "button"
}

// AccessibleName reports the semantic name source required by the mark spec.
func (b *Button) AccessibleName() string {
	if b == nil {
		return ""
	}
	return b.Label
}

// SetLabel updates the authored label text.
func (b *Button) SetLabel(label string) {
	if b == nil || b.Label == label {
		return
	}
	b.Label = label
	b.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetVariant updates the authored button variant.
func (b *Button) SetVariant(variant uiinput.ButtonVariant) {
	if b == nil || b.Variant == variant {
		return
	}
	b.Variant = variant
	b.invalidate(facet.DirtyProjection)
}

// SetLeadingIconRef updates the leading icon asset reference.
func (b *Button) SetLeadingIconRef(ref string) {
	if b == nil || b.LeadingIconRef == ref {
		return
	}
	b.LeadingIconRef = ref
	b.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetTrailingIconRef updates the trailing icon asset reference.
func (b *Button) SetTrailingIconRef(ref string) {
	if b == nil || b.TrailingIconRef == ref {
		return
	}
	b.TrailingIconRef = ref
	b.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles the disabled state.
func (b *Button) SetDisabled(disabled bool) {
	if b == nil || b.Disabled == disabled {
		return
	}
	b.Disabled = disabled
	if disabled {
		b.hovered = false
		b.pressed = false
		b.spaceDown = false
		b.enterDown = false
		b.focusedVisible = false
	}
	b.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the button's semantic anchor set.
func (b *Button) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if b == nil {
		return nil
	}
	bounds := b.layoutRole.ArrangedBounds
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
	if b.textRole.Layout != nil {
		out["baseline"] = gfx.Point{
			X: bounds.Min.X,
			Y: b.cachedLabelBounds.Min.Y + b.textRole.Layout.Baseline,
		}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (b *Button) Children() []facet.GroupChild {
	return nil
}

func (b *Button) OnAttach(ctx facet.AttachContext) {}
func (b *Button) OnActivate()                      {}
func (b *Button) OnDeactivate()                    {}
func (b *Button) OnDetach() {
	b.cachedLayout = nil
	b.cachedLabelStyle = text.TextStyle{}
	b.cachedLabelBounds = gfx.Rect{}
	b.cachedLeadingBox = gfx.Rect{}
	b.cachedTrailingBox = gfx.Rect{}
	b.cachedPadX = 0
	b.cachedPadY = 0
	b.cachedGap = 0
	b.cachedRadius = 0
	b.cachedTokens = theme.Tokens{}
	b.cachedRecipe = shared.ButtonSlots{}
	b.cachedLeadingAsset = runtimepkg.IconAsset{}
	b.cachedTrailingAsset = runtimepkg.IconAsset{}
}

func (b *Button) invalidate(flags facet.DirtyFlags) {
	if b == nil {
		return
	}
	b.Facet.Invalidate(flags)
}

func (b *Button) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := b.resolveTheme(ctx)
	if !ok {
		b.cachedLayout = nil
		b.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	b.cachedTokens = resolved.TokenSet()
	b.cachedRecipe = recipe
	labelLayout, labelStyle := b.resolveLabelLayout(ctx, constraints, resolved, recipe)
	if labelLayout == nil {
		b.cachedLayout = nil
		b.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	b.cachedLayout = labelLayout
	b.cachedLabelStyle = labelStyle
	b.textRole.Layout = labelLayout
	b.cachedLabelBounds = gfx.RectFromXYWH(0, 0, labelLayout.Bounds.Width(), labelLayout.Bounds.Height())

	leadingBox, leadingAsset := b.resolveIconBox(ctx, b.LeadingIconRef, recipe.OptionalLeadingIcon)
	trailingBox, trailingAsset := b.resolveIconBox(ctx, b.TrailingIconRef, recipe.OptionalTrailingIcon)
	b.cachedLeadingBox = leadingBox
	b.cachedTrailingBox = trailingBox
	b.cachedLeadingAsset = leadingAsset
	b.cachedTrailingAsset = trailingAsset

	padX := float32(resolved.Spacing(theme.SpacingM))
	padY := float32(resolved.Spacing(theme.SpacingXS))
	gap := float32(resolved.Spacing(theme.SpacingS))
	radius := float32(resolved.Radius(theme.RadiusM))
	b.cachedPadX = padX
	b.cachedPadY = padY
	b.cachedGap = gap
	b.cachedRadius = radius

	content := layout.InlineFlowSize([]gfx.Size{
		{W: leadingBox.Width(), H: leadingBox.Height()},
		{W: labelLayout.Bounds.Width(), H: labelLayout.Bounds.Height()},
		{W: trailingBox.Width(), H: trailingBox.Height()},
	}, gap)
	naturalWidth := content.W + padX*2
	naturalHeight := content.H + padY*2
	measured := constraints.Constrain(gfx.Size{W: naturalWidth, H: naturalHeight})
	b.layoutRole.MeasuredSize = measured
	b.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return b.layoutRole.MeasuredResult
}

func (b *Button) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	result := b.measure(ctx, constraints)
	return result.Size
}

func (b *Button) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	padX := b.cachedPadX
	gap := b.cachedGap
	labelSize := gfx.Size{W: b.cachedLabelBounds.Width(), H: 0}
	if b.cachedLayout != nil {
		labelSize.H = b.cachedLayout.Bounds.Height()
		labelSize.W = b.cachedLayout.Bounds.Width()
	}
	writingDirection := layout.WritingDirectionLTR
	if ctx.Theme != nil {
		if resolved, ok := ctx.Theme.(theme.ResolvedContext); ok {
			writingDirection = resolved.WritingDirection
		}
	}
	rtl := writingDirection == layout.WritingDirectionRTL
	rects := layout.ArrangeInlineFlow(bounds, padX, gap, []gfx.Size{
		{W: b.cachedLeadingBox.Width(), H: b.cachedLeadingBox.Height()},
		labelSize,
		{W: b.cachedTrailingBox.Width(), H: b.cachedTrailingBox.Height()},
	}, rtl)
	b.cachedLeadingBox = rects[0]
	b.cachedLabelBounds = rects[1]
	b.cachedTrailingBox = rects[2]

	b.layoutRole.ArrangedBounds = bounds
}

func (b *Button) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.ButtonSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiinput.ResolveButtonRecipe(style, b.Variant)
	return resolved, slots, true
}

func (b *Button) resolveLabelLayout(ctx facet.MeasureContext, constraints facet.Constraints, resolved theme.ResolvedContext, recipe shared.ButtonSlots) (*text.TextLayout, text.TextStyle) {
	if b == nil {
		return nil, text.TextStyle{}
	}
	style := resolved.TextStyle(theme.TextLabelM)
	padX := float32(resolved.Spacing(theme.SpacingM))
	gap := float32(resolved.Spacing(theme.SpacingS))
	leadingBox, trailingBox := b.resolveIconBoxes(ctx, recipe)
	maxWidth := constraints.MaxSize.W - padX*2
	if !leadingBox.IsEmpty() {
		maxWidth -= leadingBox.Width()
		if b.Label != "" {
			maxWidth -= gap
		}
	}
	if !trailingBox.IsEmpty() {
		maxWidth -= trailingBox.Width()
		if b.Label != "" || !leadingBox.IsEmpty() {
			maxWidth -= gap
		}
	}
	if maxWidth < 0 {
		maxWidth = 0
	}
	shaper := b.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil, text.TextStyle{}
	}
	shaper.SetContentScale(ctx.ContentScale)
	layout := shaper.ShapeTruncated(b.Label, style, maxWidth)
	if layout == nil {
		return nil, text.TextStyle{}
	}
	return layout, style
}

func (b *Button) resolveIconBoxes(ctx facet.MeasureContext, recipe shared.ButtonSlots) (gfx.Rect, gfx.Rect) {
	leading, _ := b.resolveIconBox(ctx, b.LeadingIconRef, recipe.OptionalLeadingIcon)
	trailing, _ := b.resolveIconBox(ctx, b.TrailingIconRef, recipe.OptionalTrailingIcon)
	return leading, trailing
}

func (b *Button) resolveIconBox(ctx facet.MeasureContext, ref string, style theme.MarkStyle) (gfx.Rect, runtimepkg.IconAsset) {
	if ref == "" {
		return gfx.Rect{}, runtimepkg.IconAsset{}
	}
	iconSize := resolvedIconSize(ctx)
	if resolver := b.iconResolver(ctx.Runtime); resolver != nil {
		if asset, ok := resolver.ResolveIcon(ref); ok {
			asset = asset.Clone()
			if asset.ViewBox.IsEmpty() {
				return gfx.RectFromXYWH(0, 0, iconSize, iconSize), asset
			}
			scale := float32(1)
			if asset.ViewBox.Width() > 0 && asset.ViewBox.Height() > 0 {
				scale = minFloat(iconSize/asset.ViewBox.Width(), iconSize/asset.ViewBox.Height())
			}
			return gfx.RectFromXYWH(0, 0, asset.ViewBox.Width()*scale, asset.ViewBox.Height()*scale), asset
		}
	}
	_ = style
	return gfx.RectFromXYWH(0, 0, iconSize, iconSize), runtimepkg.IconAsset{}
}

func resolvedIconSize(ctx facet.MeasureContext) float32 {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	return resolved.Density.Scale(resolved.TokenSet().Spacing.IconSize)
}

func (b *Button) iconResolver(runtime any) runtimepkg.IconResolver {
	if runtime == nil {
		return nil
	}
	type provider interface {
		IconResolver() runtimepkg.IconResolver
	}
	if p, ok := runtime.(provider); ok {
		return p.IconResolver()
	}
	return nil
}

func (b *Button) newShaper(runtime any) *text.Shaper {
	registry := b.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (b *Button) fontRegistry(runtime any) *text.FontRegistry {
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

func (b *Button) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if b == nil || bounds.IsEmpty() || b.cachedLayout == nil {
		return nil
	}
	state := b.interactionState()
	slots := b.cachedRecipe
	tokens := b.cachedTokens
	container := slots.Container.Resolve(state, tokens)
	root := slots.Root.Resolve(state, tokens)
	label := slots.Label.Resolve(state, tokens)
	leading := slots.OptionalLeadingIcon.Resolve(state, tokens)
	trailing := slots.OptionalTrailingIcon.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	stateLayer := slots.StateLayer.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 16)
	path := gfx.RoundedRectPath(bounds, b.cachedRadius)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(path, root)...)
	}
	if !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(path, container)...)
	}
	if !isTransparentMaterial(stateLayer) {
		cmds = append(cmds, materialCommands(path, stateLayer)...)
	}
	if !isTransparentMaterial(label) {
		cmds = append(cmds, primitive.TextLayoutCommands(b.cachedLayout, b.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
	}
	cmds = append(cmds, b.iconCommands(leading, true)...)
	cmds = append(cmds, b.iconCommands(trailing, false)...)
	if b.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, b.cachedPadY*0.5)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, b.cachedRadius+inset), focus)...)
	}
	return cmds
}
func (b *Button) iconCommands(style theme.Material, leading bool) []gfx.Command {
	ref := b.LeadingIconRef
	box := b.cachedLeadingBox
	asset := b.cachedLeadingAsset
	if !leading {
		ref = b.TrailingIconRef
		box = b.cachedTrailingBox
		asset = b.cachedTrailingAsset
	}
	if ref == "" || box.IsEmpty() || len(asset.Path.Segments) == 0 {
		return nil
	}
	if isTransparentMaterial(style) {
		return nil
	}
	iconColor := materialColor(style)
	brush := gfx.SolidBrush(iconColor)
	transform := iconTransform(asset.ViewBox, box)
	return []gfx.Command{
		gfx.PushTransform{Matrix: transform},
		gfx.FillPath{Path: asset.Path, Brush: brush},
		gfx.PopTransform{},
	}
}

func iconTransform(viewBox gfx.Rect, target gfx.Rect) gfx.Transform {
	if target.IsEmpty() {
		return gfx.Identity()
	}
	if viewBox.IsEmpty() {
		return gfx.Translation(target.Min.X, target.Min.Y)
	}
	scaleX := target.Width() / viewBox.Width()
	scaleY := target.Height() / viewBox.Height()
	scale := minFloat(scaleX, scaleY)
	width := viewBox.Width() * scale
	height := viewBox.Height() * scale
	offsetX := target.Min.X + (target.Width()-width)/2 - viewBox.Min.X*scale
	offsetY := target.Min.Y + (target.Height()-height)/2 - viewBox.Min.Y*scale
	return gfx.Transform{
		A:  scale,
		D:  scale,
		TX: offsetX,
		TY: offsetY,
	}
}

func (b *Button) hitTest(p gfx.Point) facet.HitResult {
	if b == nil || b.layoutRole.ArrangedBounds.IsEmpty() {
		return facet.HitResult{}
	}
	if !b.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	if b.cachedLeadingBox.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: buttonMarkIDLeading, Cursor: b.cursorShape()}
	}
	if b.cachedTrailingBox.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: buttonMarkIDTrailing, Cursor: b.cursorShape()}
	}
	if b.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: buttonMarkIDLabel, Cursor: b.cursorShape()}
	}
	return facet.HitResult{Hit: true, MarkID: buttonMarkIDContainer, Cursor: b.cursorShape()}
}

func (b *Button) cursorShape() facet.CursorShape {
	if b.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (b *Button) onPointer(e facet.PointerEvent) bool {
	if b.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		b.hovered = true
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		b.hovered = false
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		b.pressed = true
		b.focusFromPointer = true
		b.focusedVisible = false
		b.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := b.pressed
		b.pressed = false
		b.invalidate(facet.DirtyProjection)
		if wasPressed && b.layoutRole.ArrangedBounds.Contains(e.Position) {
			b.Activated.Emit(signal.Fired)
			return true
		}
		return wasPressed
	case platform.PointerMove:
		return b.hovered
	default:
		return false
	}
}

func (b *Button) onKey(e facet.KeyEvent) bool {
	if b.Disabled {
		return false
	}
	switch e.Key {
	case platform.KeySpace:
		switch e.Kind {
		case platform.KeyPress:
			b.spaceDown = true
			b.pressed = true
			b.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasDown := b.spaceDown
			b.spaceDown = false
			b.pressed = false
			b.invalidate(facet.DirtyProjection)
			if wasDown {
				b.Activated.Emit(signal.Fired)
			}
			return wasDown
		}
	case platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress:
			b.enterDown = true
			b.pressed = true
			b.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasDown := b.enterDown
			b.enterDown = false
			b.pressed = false
			b.invalidate(facet.DirtyProjection)
			if wasDown {
				b.Activated.Emit(signal.Fired)
			}
			return wasDown
		}
	}
	return false
}

func (b *Button) onFocusGained() {
	b.focusedVisible = !b.focusFromPointer
	b.focusFromPointer = false
	b.invalidate(facet.DirtyProjection)
}

func (b *Button) onFocusLost() {
	b.focusedVisible = false
	b.pressed = false
	b.spaceDown = false
	b.enterDown = false
	b.focusFromPointer = false
	b.invalidate(facet.DirtyProjection)
}

func (b *Button) interactionState() theme.InteractionState {
	switch {
	case b.Disabled:
		return theme.StateDisabled
	case b.pressed:
		return theme.StatePressed
	case b.hovered:
		return theme.StateHover
	case b.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func materialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	if isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes))
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
			Path:  path,
			Brush: gfx.SolidBrush(stroke.Paint.Color),
			Stroke: gfx.StrokeStyle{
				Width:      stroke.Width,
				Cap:        gfx.LineCapRound,
				Join:       gfx.LineJoinRound,
				MiterLimit: 10,
				Dash:       append([]float32(nil), stroke.Dash...),
				DashOffset: stroke.DashOffset,
			},
		})
	}
	return cmds
}

func materialColor(material theme.Material) gfx.Color {
	for _, fill := range material.Fills {
		if fill.Type == theme.FillSolid && fill.Color.A > 0 {
			return fill.Color
		}
	}
	for _, stroke := range material.Strokes {
		if stroke.Paint.Type == theme.FillSolid && stroke.Paint.Color.A > 0 {
			return stroke.Paint.Color
		}
	}
	return gfx.Color{}
}

func isTransparentMaterial(material theme.Material) bool {
	if material.Opacity <= 0 {
		return true
	}
	for _, fill := range material.Fills {
		if fill.Type == theme.FillSolid && fill.Color.A > 0 && fill.Opacity > 0 {
			return false
		}
	}
	for _, stroke := range material.Strokes {
		if stroke.Paint.Type == theme.FillSolid && stroke.Paint.Color.A > 0 && stroke.Width > 0 {
			return false
		}
	}
	return true
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

type buttonGroupPolicy struct{}

func (buttonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (buttonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (buttonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
