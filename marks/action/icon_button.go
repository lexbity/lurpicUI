package action

import (
	"fmt"
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	svgnorm "codeburg.org/lexbit/lurpicui/gfx/svg"
	"codeburg.org/lexbit/lurpicui/layout"
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
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole

	Activated signal.Signal[signal.Unit]

	Source          primitive.IconSource
	Size            float32
	AccessibleLabel string
	DensityBehavior primitive.IconDensityBehavior
	HitPadding      float32
	Disabled        bool

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

// NewIconButton constructs an action.icon_button mark with canonical defaults.
func NewIconButton(source primitive.IconSource) *IconButton {
	i := &IconButton{
		Facet:           facet.NewFacet(),
		Source:          source,
		DensityBehavior: primitive.IconDensityScaleWithDensity,
	}
	i.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: iconButtonGroupPolicy{},
	}
	i.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
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
	i.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return i.measure(ctx, constraints)
	}
	i.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		i.layoutRole.ArrangedBounds = bounds
		i.arrange(bounds)
	}
	i.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := i.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	i.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := i.buildCommands(i.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	i.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return i.hitTest(p)
	}
	i.inputRole.OnPointer = func(e facet.PointerEvent) bool {
		return i.onPointer(e)
	}
	i.inputRole.OnKey = func(e facet.KeyEvent) bool {
		return i.onKey(e)
	}
	i.focusRole.Focusable = func() bool {
		return !i.Disabled
	}
	i.focusRole.OnFocusGained = func() {
		i.onFocusGained()
	}
	i.focusRole.OnFocusLost = func() {
		i.onFocusLost()
	}
	i.AddRole(&i.layoutRole)
	i.AddRole(&i.renderRole)
	i.AddRole(&i.projectionRole)
	i.AddRole(&i.hitRole)
	i.AddRole(&i.inputRole)
	i.AddRole(&i.focusRole)
	return i
}

// Base satisfies facet.FacetImpl.
func (i *IconButton) Base() *facet.Facet {
	i.Facet.BindImpl(i)
	return &i.Facet
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
	return i.AccessibleLabel
}

// SetSource updates the authored icon source.
func (i *IconButton) SetSource(source primitive.IconSource) {
	if i == nil || i.Source == source {
		return
	}
	i.Source = source
	i.cachedSource = iconButtonResolvedSource{}
	i.cachedSourceKey = ""
	i.cachedProjectionKey = ""
	i.cachedCommands = nil
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetSize updates the authored icon size.
func (i *IconButton) SetSize(size float32) {
	if i == nil || i.Size == size {
		return
	}
	i.Size = size
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetAccessibleName updates the accessible name.
func (i *IconButton) SetAccessibleName(name string) {
	if i == nil || i.AccessibleLabel == name {
		return
	}
	i.AccessibleLabel = name
	i.invalidate(facet.DirtyProjection)
}

// SetDensityBehavior updates how the icon size responds to density.
func (i *IconButton) SetDensityBehavior(behavior primitive.IconDensityBehavior) {
	if i == nil || i.DensityBehavior == behavior {
		return
	}
	i.DensityBehavior = behavior
	i.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetHitPadding updates the explicit extra hit padding around the visual bounds.
func (i *IconButton) SetHitPadding(padding float32) {
	if i == nil || i.HitPadding == padding {
		return
	}
	i.HitPadding = padding
	i.invalidate(facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (i *IconButton) SetDisabled(disabled bool) {
	if i == nil || i.Disabled == disabled {
		return
	}
	i.Disabled = disabled
	if disabled {
		i.hovered = false
		i.pressed = false
		i.focusedVisible = false
		i.focusFromPointer = false
	}
	i.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the icon button anchor set.
func (i *IconButton) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if i == nil {
		return nil
	}
	bounds := i.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
	}
}

func (i *IconButton) OnAttach(ctx facet.AttachContext) {}
func (i *IconButton) OnActivate()                      {}
func (i *IconButton) OnDeactivate()                    {}
func (i *IconButton) OnDetach() {
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
	i.layoutRole.MeasuredSize = size
	i.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	_ = i.resolveSource(ctx.Runtime)
	return i.layoutRole.MeasuredResult
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
	slots, _ := uiinput.ResolveIconButtonRecipe(style)
	return resolved, slots, true
}

func (i *IconButton) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.IconButtonSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: i.cachedTokens}, i.cachedRecipe
	}
	if store := theme.NearestStyleContext(runtime, i.Base().ID()); store != nil {
		style := store.Get()
		slots, _ := uiinput.ResolveIconButtonRecipe(style)
		return style, slots
	}
	return theme.StyleContext{Tokens: i.cachedTokens}, i.cachedRecipe
}

func (i *IconButton) resolveIconSize(ctx facet.MeasureContext, resolved theme.ResolvedContext) gfx.Size {
	base := i.Size
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
		cmds = append(cmds, i.iconCommands(i.cachedIconBounds, iconStyle, src)...)
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
	if i == nil || i.Source == nil {
		return iconButtonResolvedSource{}
	}
	switch src := i.Source.(type) {
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
	if i == nil || i.layoutRole.ArrangedBounds.IsEmpty() {
		return facet.HitResult{}
	}
	rootBounds := i.layoutRole.ArrangedBounds
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
	if i.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (i *IconButton) onPointer(e facet.PointerEvent) bool {
	if i.Disabled {
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
		if wasPressed && i.layoutRole.ArrangedBounds.Contains(e.Position) {
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
	if i.Disabled {
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
	case i.Disabled:
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
