package action

import (
	"fmt"
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	svgnorm "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	runtimepkg "codeburg.org/lexbit/lurpicui/runtime"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	iconButtonMarkIDRoot       facet.MarkID = 1
	iconButtonMarkIDContainer  facet.MarkID = 2
	iconButtonMarkIDIcon       facet.MarkID = 3
	iconButtonMarkIDFocusRing  facet.MarkID = 4
	iconButtonMarkIDStateLayer facet.MarkID = 5
)

// IconButton implements the action.icon_button standard mark.
type IconButton struct {
	marks.Core

	Activated signal.Signal[signal.Unit]

	Icon            primitive.IconSource
	DensityBehavior primitive.IconDensityBehavior
	HitPadding      float32

	Label           marks.Binding[string]
	AccessibleLabel marks.Binding[string]
	Variant         marks.Binding[uiinput.IconButtonVariant]
	Disabled        marks.Binding[bool]
	Size            marks.Binding[float32]
	ColorSlot       marks.Binding[theme.ColorToken]

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool

	cachedSize            gfx.Size
	cachedIconSize        gfx.Size
	cachedTokens          theme.Tokens
	cachedRecipe          shared.IconButtonSlots
	cachedRootBounds      gfx.Rect
	cachedContainerBounds gfx.Rect
	cachedIconBounds      gfx.Rect
	cachedTouchPad        float32
	cachedSource          iconButtonResolvedSource
	cachedSourceKey       string
	cachedProjectionKey   string
	cachedCommands        []gfx.Command
	cachedColor           gfx.Color
}

type iconButtonResolvedSourceKind uint8

const (
	iconButtonSourceNone iconButtonResolvedSourceKind = iota
	iconButtonSourceAsset
	iconButtonSourceSVG
)

type iconButtonResolvedSource struct {
	kind  iconButtonResolvedSourceKind
	asset runtimepkg.IconAsset
	doc   svgnorm.SVGDocument
	box   gfx.Rect
	key   string
}

var _ facet.FacetImpl = (*IconButton)(nil)
var _ layout.AnchorExporter = (*IconButton)(nil)
var _ marks.Mark = (*IconButton)(nil)

// NewIconButton constructs an action.icon_button mark with canonical defaults.
func NewIconButton(source primitive.IconSource) *IconButton {
	i := &IconButton{
		Icon:            source,
		DensityBehavior: primitive.IconDensityScaleWithDensity,
		Label:           marks.Const(""),
		AccessibleLabel: marks.Const(""),
		Variant:         marks.Const[uiinput.IconButtonVariant](0),
		Disabled:        marks.Const(false),
		Size:            marks.Const[float32](0),
		ColorSlot:       marks.Const(theme.ColorText),
	}
	i.Core.Facet = facet.NewFacet()
	i.AddBinding(i.Label)
	i.AddBinding(i.AccessibleLabel)
	i.AddBinding(i.Variant)
	i.AddBinding(i.Disabled)
	i.AddBinding(i.Size)
	i.AddBinding(i.ColorSlot)

	i.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: iconButtonGroupPolicy{},
	}
	i.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsRadial,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := i.measureIntrinsic(ctx, constraints)
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
	i.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return i.measure(ctx, constraints)
	}
	i.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		i.Layout.ArrangedBounds = bounds
		i.arrange(bounds)
	}
	i.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return i.hitTest(p)
	}
	i.Input.OnPointer = func(e facet.PointerEvent) bool {
		return i.onPointer(e)
	}
	i.Input.OnKey = func(e facet.KeyEvent) bool {
		return i.onKey(e)
	}
	i.Focus.Focusable = func() bool {
		return !i.Disabled.Get()
	}
	i.Focus.OnFocusGained = func() {
		i.onFocusGained()
	}
	i.Focus.OnFocusLost = func() {
		i.onFocusLost()
	}
	i.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return i.buildCommands(i.Layout.ArrangedBounds, ctx.Runtime)
	}
	i.RegisterRoles()
	return i
}

// Base satisfies facet.FacetImpl.
func (i *IconButton) Base() *facet.Facet {
	i.Facet.BindImpl(i)
	return &i.Facet
}

// Descriptor satisfies marks.Mark.
func (i *IconButton) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "icon_button"}
}

// AccessibilityRole reports the semantic role required by the mark spec.
func (i *IconButton) AccessibilityRole() string {
	return "button"
}

// AccessibleName reports the semantic name required by the mark spec.
func (i *IconButton) AccessibleName() string {
	if i == nil {
		return ""
	}
	return i.AccessibleLabel.Get()
}

// ExportAnchors publishes the icon button anchor set.
func (i *IconButton) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := i.Layout.ArrangedBounds
	return i.Core.DefaultAnchors(bounds, ctx)
}

func (i *IconButton) OnAttach(ctx facet.AttachContext) { i.Core.OnAttach() }
func (i *IconButton) OnActivate()                      { i.Core.OnActivate() }
func (i *IconButton) OnDeactivate()                    { i.Core.OnDeactivate() }
func (i *IconButton) OnDetach() {
	i.Core.OnDetach()
	i.cachedSize = gfx.Size{}
	i.cachedIconSize = gfx.Size{}
	i.cachedTokens = theme.Tokens{}
	i.cachedRecipe = shared.IconButtonSlots{}
	i.cachedRootBounds = gfx.Rect{}
	i.cachedContainerBounds = gfx.Rect{}
	i.cachedIconBounds = gfx.Rect{}
	i.cachedTouchPad = 0
	i.cachedSource = iconButtonResolvedSource{}
	i.cachedSourceKey = ""
	i.cachedProjectionKey = ""
	i.cachedCommands = nil
	i.cachedColor = gfx.Color{}
}

func (i *IconButton) invalidate(flags facet.DirtyFlags) {
	if i == nil {
		return
	}
	i.Facet.Invalidate(flags)
}

func (i *IconButton) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := i.resolveTheme(ctx)
	if !ok {
		i.cachedCommands = nil
		return facet.MeasureResult{}
	}
	i.cachedTokens = resolved.TokenSet()
	i.cachedRecipe = recipe
	iconSize := i.resolveIconSize(ctx, resolved)
	size := i.resolveSize(ctx, resolved, iconSize)
	i.cachedIconSize = iconSize
	i.cachedSize = size
	i.cachedTouchPad = i.computeTouchPadding(ctx, size, resolved)
	i.Layout.MeasuredSize = size
	i.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	_ = i.resolveSource(ctx.Runtime)
	return i.Layout.MeasuredResult
}

func (i *IconButton) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return i.measure(ctx, constraints).Size
}

func (i *IconButton) arrange(bounds gfx.Rect) {
	i.cachedRootBounds = bounds
	i.cachedContainerBounds = bounds
	if bounds.IsEmpty() {
		i.cachedIconBounds = gfx.Rect{}
		return
	}
	iconSize := i.cachedIconSize.W
	if iconSize <= 0 {
		iconSize = bounds.Width()
	}
	if iconSize <= 0 {
		iconSize = bounds.Height()
	}
	iconSize = minFloat(iconSize, minFloat(bounds.Width(), bounds.Height()))
	iconX := bounds.Min.X + (bounds.Width()-iconSize)/2
	iconY := bounds.Min.Y + (bounds.Height()-iconSize)/2
	i.cachedIconBounds = gfx.RectFromXYWH(iconX, iconY, iconSize, iconSize)
}

func (i *IconButton) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.IconButtonSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiinput.ResolveIconButtonRecipe(style, i.Variant.Get())
	return resolved, slots, true
}

func (i *IconButton) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.IconButtonSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: i.cachedTokens}, i.cachedRecipe
	}
	if store := theme.NearestStyleContext(runtime, i.Base().ID()); store != nil {
		style := store.Get()
		slots, _ := uiinput.ResolveIconButtonRecipe(style, i.Variant.Get())
		return style, slots
	}
	return theme.StyleContext{Tokens: i.cachedTokens}, i.cachedRecipe
}

func (i *IconButton) resolveIconSize(ctx facet.MeasureContext, resolved theme.ResolvedContext) gfx.Size {
	base := i.Size.Get()
	if base <= 0 {
		base = resolved.TokenSet().Spacing.IconSize
	}
	switch i.DensityBehavior {
	case primitive.IconDensityLockLogicalSize:
	case primitive.IconDensityTouchAware, primitive.IconDensityScaleWithDensity, primitive.IconDensitySnapToDevicePixels:
		base = resolved.Density.Scale(base)
	}
	if base <= 0 {
		base = resolved.TokenSet().Spacing.IconSize
	}
	if i.DensityBehavior == primitive.IconDensitySnapToDevicePixels {
		scale := ctx.ContentScale
		if scale <= 0 {
			scale = 1
		}
		base = float32(math.Round(float64(base*scale))) / scale
	}
	if base < 0 {
		base = 0
	}
	return gfx.Size{W: base, H: base}
}

func (i *IconButton) resolveSize(ctx facet.MeasureContext, resolved theme.ResolvedContext, iconSize gfx.Size) gfx.Size {
	padding := float32(resolved.Spacing(theme.SpacingS))
	if padding < 0 {
		padding = 0
	}
	size := iconSize.W + padding*2
	if size <= 0 {
		size = iconSize.W
	}
	if size < 0 {
		size = 0
	}
	return gfx.Size{W: size, H: size}
}

func (i *IconButton) computeTouchPadding(ctx facet.MeasureContext, size gfx.Size, resolved theme.ResolvedContext) float32 {
	touch := resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget)
	longest := size.W
	if size.H > longest {
		longest = size.H
	}
	padding := i.HitPadding
	if touch > longest {
		padding = maxFloat(padding, (touch-longest)*0.5)
	}
	if padding < 0 {
		padding = 0
	}
	_ = ctx
	return padding
}

func (i *IconButton) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if i == nil || bounds.IsEmpty() {
		return nil
	}
	style, recipe := i.resolveProjectionTheme(runtime)
	src := i.resolveSource(runtime)
	if src.kind == iconButtonSourceNone {
		return nil
	}
	state := i.interactionState()
	tokens := style.Tokens
	root := recipe.Root.Resolve(state, tokens)
	container := recipe.Container.Resolve(state, tokens)
	iconStyle := recipe.Icon.Resolve(state, tokens)
	focus := recipe.FocusRing.Resolve(theme.StateFocused, tokens)
	stateLayer := recipe.StateLayer.Resolve(state, tokens)

	iconBounds := i.cachedIconBounds

	if i.Variant.Get() == uiinput.IconButtonSkeuomorphic && state == theme.StatePressed {
		strokesCopy := make([]theme.MaterialStroke, len(container.Strokes))
		for idx, stroke := range container.Strokes {
			stroke.Offset.X = -stroke.Offset.X
			stroke.Offset.Y = -stroke.Offset.Y
			if !stroke.Inner {
				stroke.Inner = true
			}
			strokesCopy[idx] = stroke
		}
		container.Strokes = strokesCopy

		iconBounds = iconBounds.Offset(1.5, 1.5)
	}

	cmds := make([]gfx.Command, 0, 16)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	containerPath := gfx.RoundedRectPath(i.cachedContainerBounds, minFloat(i.cachedContainerBounds.Width(), i.cachedContainerBounds.Height())*0.5)
	if !isTransparentMaterial(container) {
		cmds = append(cmds, materialCommands(containerPath, container)...)
	}
	if !isTransparentMaterial(stateLayer) {
		cmds = append(cmds, materialCommands(containerPath, stateLayer)...)
	}
	if !isTransparentMaterial(iconStyle) {
		cmds = append(cmds, i.iconCommands(iconBounds, iconStyle, src)...)
	}
	if i.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, i.cachedTouchPad*0.25)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, minFloat(i.cachedContainerBounds.Width(), i.cachedContainerBounds.Height())*0.5+inset), focus)...)
	}
	if len(cmds) == 0 {
		return nil
	}
	cacheKey := fmt.Sprintf("%v|%s|%v|%v|%v|%d", bounds, src.key, state, style.Tokens.Color.Primary, i.cachedIconBounds, len(cmds))
	if cacheKey == i.cachedProjectionKey && len(i.cachedCommands) > 0 {
		return append([]gfx.Command(nil), i.cachedCommands...)
	}
	i.cachedColor = materialColor(iconStyle)
	i.cachedCommands = append([]gfx.Command(nil), cmds...)
	i.cachedProjectionKey = cacheKey
	i.cachedSource = src
	i.cachedSourceKey = src.key
	return cmds
}

func (i *IconButton) iconCommands(target gfx.Rect, style theme.Material, src iconButtonResolvedSource) []gfx.Command {
	if src.kind == iconButtonSourceNone || src.box.IsEmpty() || target.IsEmpty() {
		return nil
	}
	if isTransparentMaterial(style) {
		return nil
	}
	brush := gfx.SolidBrush(materialColor(style))
	transform := iconFitTransform(src.box, target, svgnorm.SVGPreserveAspectRatio{
		Align:       svgnorm.SVGAspectRatioAlignXMidYMid,
		MeetOrSlice: svgnorm.SVGMeetOrSliceMeet,
	})
	switch src.kind {
	case iconButtonSourceAsset:
		if len(src.asset.Path.Segments) == 0 {
			return nil
		}
		return []gfx.Command{
			gfx.PushTransform{Matrix: transform},
			gfx.FillPath{Path: src.asset.Path, Brush: brush},
			gfx.PopTransform{},
		}
	case iconButtonSourceSVG:
		cmds := make([]gfx.Command, 0, len(src.doc.Elements)*3)
		for _, el := range src.doc.Elements {
			cmds = append(cmds, gfx.PushTransform{Matrix: transform})
			if el.Fill.Kind != svgnorm.SVGPaintNone {
				cmds = append(cmds, gfx.FillPath{Path: el.Path, Brush: brush})
			}
			if el.Stroke != nil && el.Stroke.Width > 0 {
				cmds = append(cmds, gfx.StrokePath{
					Path:   el.Path,
					Stroke: convertStrokeStyle(*el.Stroke),
					Brush:  brush,
				})
			}
			cmds = append(cmds, gfx.PopTransform{})
		}
		return cmds
	default:
		return nil
	}
}

func (i *IconButton) resolveSource(runtime any) iconButtonResolvedSource {
	if i == nil || i.Icon == nil {
		return iconButtonResolvedSource{}
	}
	switch src := i.Icon.(type) {
	case primitive.IconRef:
		ref := strings.TrimSpace(string(src))
		if ref == "" {
			return iconButtonResolvedSource{}
		}
		if runtime == nil {
			if i.cachedSource.kind != iconButtonSourceNone {
				return i.cachedSource
			}
			return iconButtonResolvedSource{}
		}
		type resolver interface {
			ResolveIcon(ref string) (runtimepkg.IconAsset, bool)
		}
		provider, ok := runtime.(resolver)
		if !ok {
			if i.cachedSource.kind != iconButtonSourceNone {
				return i.cachedSource
			}
			return iconButtonResolvedSource{}
		}
		asset, ok := provider.ResolveIcon(ref)
		if !ok {
			if i.cachedSource.kind != iconButtonSourceNone {
				return i.cachedSource
			}
			return iconButtonResolvedSource{}
		}
		asset = asset.Clone()
		box := asset.ViewBox
		if box.IsEmpty() && len(asset.Path.Segments) > 0 {
			box = svgnorm.Bounds(asset.Path)
		}
		key := fmt.Sprintf("ref:%s:%d:%0.4f:%0.4f:%0.4f:%0.4f:%d", asset.SourceRef, asset.Revision, box.Min.X, box.Min.Y, box.Max.X, box.Max.Y, len(asset.Path.Segments))
		if key == i.cachedSourceKey {
			return i.cachedSource
		}
		resolved := iconButtonResolvedSource{kind: iconButtonSourceAsset, asset: asset, box: box, key: key}
		i.cachedSource = resolved
		i.cachedSourceKey = key
		i.cachedCommands = nil
		return resolved
	case primitive.IconSVG:
		srcText := strings.TrimSpace(string(src))
		if srcText == "" {
			return iconButtonResolvedSource{}
		}
		key := fmt.Sprintf("svg:%x", hashString(srcText))
		if key == i.cachedSourceKey {
			return i.cachedSource
		}
		doc, err := svgnorm.ParseSVG([]byte(srcText))
		if err != nil {
			return iconButtonResolvedSource{}
		}
		box := doc.ViewBox
		if box.IsEmpty() {
			box = doc.Bounds
		}
		resolved := iconButtonResolvedSource{kind: iconButtonSourceSVG, doc: doc, box: box, key: key}
		i.cachedSource = resolved
		i.cachedSourceKey = key
		i.cachedCommands = nil
		return resolved
	default:
		return iconButtonResolvedSource{}
	}
}

func (i *IconButton) hitTest(p gfx.Point) facet.HitResult {
	if i == nil || i.Layout.ArrangedBounds.IsEmpty() {
		return facet.HitResult{}
	}
	rootBounds := i.Layout.ArrangedBounds
	hitBounds := rootBounds
	if pad := i.effectiveHitPadding(); pad > 0 {
		hitBounds = hitBounds.Inset(-pad, -pad)
	}
	if !hitBounds.Contains(p) {
		return facet.HitResult{}
	}
	if i.cachedIconBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: iconButtonMarkIDIcon, Cursor: i.cursorShape()}
	}
	if rootBounds.Contains(p) {
		if i.cachedContainerBounds.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: iconButtonMarkIDContainer, Cursor: i.cursorShape()}
		}
		return facet.HitResult{Hit: true, MarkID: iconButtonMarkIDRoot, Cursor: i.cursorShape()}
	}
	return facet.HitResult{Hit: true, MarkID: iconButtonMarkIDRoot, Cursor: i.cursorShape()}
}

func (i *IconButton) effectiveHitPadding() float32 {
	padding := i.HitPadding
	if i.cachedTouchPad > padding {
		padding = i.cachedTouchPad
	}
	return padding
}

func (i *IconButton) cursorShape() facet.CursorShape {
	if i.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (i *IconButton) onPointer(e facet.PointerEvent) bool {
	if i.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		i.hovered = true
		i.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		i.hovered = false
		i.pressed = false
		i.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		i.pressed = true
		i.focusFromPointer = true
		i.focusedVisible = false
		i.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := i.pressed
		i.pressed = false
		i.invalidate(facet.DirtyProjection)
		hitBounds := i.Layout.ArrangedBounds
		if pad := i.effectiveHitPadding(); pad > 0 {
			hitBounds = hitBounds.Inset(-pad, -pad)
		}
		if wasPressed && hitBounds.Contains(e.Position) {
			i.Activated.Emit(signal.Fired)
			return true
		}
		return wasPressed
	case platform.PointerMove:
		return i.hovered
	default:
		return false
	}
}

func (i *IconButton) onKey(e facet.KeyEvent) bool {
	if i.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress:
			i.pressed = true
			i.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := i.pressed
			i.pressed = false
			i.invalidate(facet.DirtyProjection)
			if wasPressed {
				i.Activated.Emit(signal.Fired)
			}
			return wasPressed
		}
	}
	return false
}

func (i *IconButton) onFocusGained() {
	i.focusedVisible = !i.focusFromPointer
	i.focusFromPointer = false
	i.invalidate(facet.DirtyProjection)
}

func (i *IconButton) onFocusLost() {
	i.focusedVisible = false
	i.pressed = false
	i.focusFromPointer = false
	i.invalidate(facet.DirtyProjection)
}

func (i *IconButton) interactionState() theme.InteractionState {
	switch {
	case i.Disabled.Get():
		return theme.StateDisabled
	case i.pressed:
		return theme.StatePressed
	case i.hovered:
		return theme.StateHover
	case i.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func hashString(value string) uint64 {
	var h uint64 = 1469598103934665603
	for i := 0; i < len(value); i++ {
		h ^= uint64(value[i])
		h *= 1099511628211
	}
	return h
}

func iconFitTransform(srcBox, target gfx.Rect, par svgnorm.SVGPreserveAspectRatio) gfx.Transform {
	if target.IsEmpty() {
		return gfx.Identity()
	}
	if srcBox.IsEmpty() {
		return gfx.Translation(target.Min.X, target.Min.Y)
	}
	align := par.Align
	if align == svgnorm.SVGAspectRatioAlignUnspecified {
		align = svgnorm.SVGAspectRatioAlignXMidYMid
	}
	if align == svgnorm.SVGAspectRatioAlignNone {
		scaleX := target.Width() / srcBox.Width()
		scaleY := target.Height() / srcBox.Height()
		return gfx.Transform{
			A:  scaleX,
			D:  scaleY,
			TX: target.Min.X - srcBox.Min.X*scaleX,
			TY: target.Min.Y - srcBox.Min.Y*scaleY,
		}
	}
	scaleX := target.Width() / srcBox.Width()
	scaleY := target.Height() / srcBox.Height()
	scale := minFloat(scaleX, scaleY)
	scaledW := srcBox.Width() * scale
	scaledH := srcBox.Height() * scale
	var offsetX, offsetY float32
	switch align {
	case svgnorm.SVGAspectRatioAlignXMinYMin:
		offsetX = target.Min.X - srcBox.Min.X*scale
		offsetY = target.Min.Y - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMidYMin:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*scale
		offsetY = target.Min.Y - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMaxYMin:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*scale
		offsetY = target.Min.Y - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMinYMid:
		offsetX = target.Min.X - srcBox.Min.X*scale
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMidYMid:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*scale
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMaxYMid:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*scale
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMinYMax:
		offsetX = target.Min.X - srcBox.Min.X*scale
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMidYMax:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*scale
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*scale
	case svgnorm.SVGAspectRatioAlignXMaxYMax:
		offsetX = target.Max.X - scaledW - srcBox.Min.X*scale
		offsetY = target.Max.Y - scaledH - srcBox.Min.Y*scale
	default:
		offsetX = target.Min.X + (target.Width()-scaledW)/2 - srcBox.Min.X*scale
		offsetY = target.Min.Y + (target.Height()-scaledH)/2 - srcBox.Min.Y*scale
	}
	return gfx.Transform{A: scale, D: scale, TX: offsetX, TY: offsetY}
}

func convertStrokeStyle(st svgnorm.SVGStroke) gfx.StrokeStyle {
	return gfx.StrokeStyle{
		Width:      st.Width,
		Cap:        st.Cap,
		Join:       st.Join,
		MiterLimit: st.MiterLimit,
		Dash:       append([]float32(nil), st.Dash...),
		DashOffset: st.DashOffset,
	}
}

type iconButtonGroupPolicy struct{}

func (iconButtonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }

func (iconButtonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (iconButtonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
