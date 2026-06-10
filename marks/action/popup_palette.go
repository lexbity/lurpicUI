package action

import (
	"fmt"
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	input "codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	popupPaletteMarkIDRoot        facet.MarkID = 1
	popupPaletteMarkIDSurface     facet.MarkID = 2
	popupPaletteMarkIDToolItems   facet.MarkID = 3
	popupPaletteMarkIDToolGroup   facet.MarkID = 4
	popupPaletteMarkIDAnchorArrow facet.MarkID = 5
	popupPaletteMarkIDFocusRing   facet.MarkID = 6
)

type PopupPaletteTool struct {
	Key             string
	Label           string
	AccessibleLabel string
	IconRef         string
	Color           gfx.Color
	Selected        bool
	Disabled        bool
}

type PopupPalette struct {
	marks.Core

	Activated signal.Signal[string]

	Label         marks.Binding[string]
	Tools         []PopupPaletteTool
	History       []gfx.Color
	Open          marks.Binding[bool]
	Disabled      marks.Binding[bool]
	ShowBottomBar marks.Binding[bool]
	Zoom          marks.Binding[float64]
	CanvasOnly    marks.Binding[bool]
	MirrorCanvas  marks.Binding[bool]
	SelectedIndex marks.Binding[int]

	textRole facet.TextRole

	hoveredIndex     int
	pressedIndex     int
	focusedVisible   bool
	focusFromPointer bool
	hoveredControl   popupPaletteControlKind
	pressedControl   popupPaletteControlKind
	draggingZoom     bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.PopupPaletteSlots
	cachedRootBounds       gfx.Rect
	cachedTriggerBounds    gfx.Rect
	cachedSurfaceBounds    gfx.Rect
	cachedToolItemBounds   []gfx.Rect
	cachedHistoryBounds    []gfx.Rect
	cachedToolGroupBounds  gfx.Rect
	cachedAnchorBounds     gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedLabelLayout      *text.TextLayout
	cachedLabelStyle       text.TextStyle
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRingRadius       float32
	cachedRingThickness    float32
	cachedToolSize         float32
	cachedHistorySize      float32
	cachedBottomBarHeight  float32
	cachedWritingDirection facet.WritingDirection
	cachedSliderTrack      gfx.Rect
	cachedSliderThumb      gfx.Rect
	cachedMirrorBounds     gfx.Rect
	cachedCanvasBounds     gfx.Rect
	cachedClearBounds      gfx.Rect
	cachedToggleBounds     gfx.Rect
	cachedZoomBounds       gfx.Rect
	cachedChildren         []*popupPaletteChild

	composition *popupPaletteComposition
}

type popupPaletteControlKind uint8

const (
	popupPaletteControlNone popupPaletteControlKind = iota
	popupPaletteControlMirror
	popupPaletteControlCanvasOnly
	popupPaletteControlZoom
	popupPaletteControlClearHistory
	popupPaletteControlToggleBar
)

type popupPaletteToolLayout struct {
	tool       PopupPaletteTool
	bounds     gfx.Rect
	iconBounds gfx.Rect
}

type popupPaletteChildKind uint8

const (
	popupPaletteChildTool popupPaletteChildKind = iota
	popupPaletteChildMirror
	popupPaletteChildCanvasOnly
	popupPaletteChildClearHistory
	popupPaletteChildToggleBar
)

type popupPaletteChildSpec struct {
	kind            popupPaletteChildKind
	index           int
	iconRef         string
	accessibleLabel string
	disabled        bool
	hitPadding      float32
	innerSize       float32
}

type popupPaletteChild struct {
	parent *PopupPalette
	index  int
	spec   popupPaletteChildSpec

	button *IconButton

	subID signal.SubscriptionID
}

func newPopupPaletteChild(parent *PopupPalette, index int, spec popupPaletteChildSpec) *popupPaletteChild {
	child := &popupPaletteChild{
		parent: parent,
		index:  index,
		spec:   spec,
	}
	child.setSpec(spec)
	return child
}

func (c *popupPaletteChild) dispose() {
	if c == nil {
		return
	}
	if c.button != nil && c.subID != 0 {
		c.button.Activated.Unsubscribe(c.subID)
	}
	c.subID = 0
}

func (c *popupPaletteChild) isTool() bool {
	return c != nil && c.spec.kind == popupPaletteChildTool
}

func (c *popupPaletteChild) base() *facet.Facet {
	if c == nil || c.button == nil {
		return nil
	}
	return c.button.Base()
}

func (c *popupPaletteChild) setSpec(spec popupPaletteChildSpec) {
	if c == nil {
		return
	}
	c.spec = spec
	if c.button == nil {
		c.button = NewIconButton(primitive.IconRef(spec.iconRef))
		c.subID = c.button.Activated.Subscribe(func(signal.Unit) {
			if c.parent == nil {
				return
			}
			switch c.spec.kind {
			case popupPaletteChildTool:
				c.parent.activateTool(c.spec.index)
			case popupPaletteChildMirror:
				c.parent.activateControl(popupPaletteControlMirror, centerOfRect(c.bounds()))
			case popupPaletteChildCanvasOnly:
				c.parent.activateControl(popupPaletteControlCanvasOnly, centerOfRect(c.bounds()))
			case popupPaletteChildClearHistory:
				c.parent.activateControl(popupPaletteControlClearHistory, centerOfRect(c.bounds()))
			case popupPaletteChildToggleBar:
				c.parent.activateControl(popupPaletteControlToggleBar, centerOfRect(c.bounds()))
			}
		})
	}
	c.button.Icon = primitive.IconRef(spec.iconRef)
	c.button.AccessibleLabel = marks.Const(spec.accessibleLabel)
	c.button.Disabled = marks.Const(spec.disabled)
	c.button.Size = marks.Const(spec.innerSize)
	c.button.HitPadding = spec.hitPadding
}

func (c *popupPaletteChild) measure(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	if c == nil || c.button == nil || c.button.Base() == nil || c.button.Base().LayoutRole() == nil {
		return gfx.Size{}
	}
	size := c.button.Base().LayoutRole().Measure(ctx, constraints).Size
	return size
}

func (c *popupPaletteChild) arrange(ctx facet.ArrangeContext) {
	if c == nil || c.button == nil || c.button.Base() == nil || c.button.Base().LayoutRole() == nil {
		return
	}
	bounds := c.bounds()
	if bounds.IsEmpty() {
		return
	}
	measured := c.button.Base().LayoutRole().MeasuredSize
	if measured.W <= 0 || measured.H <= 0 {
		measured = gfx.Size{W: bounds.Width(), H: bounds.Height()}
	}
	inner := gfx.RectFromXYWH(bounds.Min.X+(bounds.Width()-measured.W)*0.5, bounds.Min.Y+(bounds.Height()-measured.H)*0.5, measured.W, measured.H)
	c.button.Base().LayoutRole().Arrange(ctx, inner)
}

func (c *popupPaletteChild) project(runtime any, contentScale float32) *gfx.CommandList {
	if c == nil || c.button == nil || c.button.Base() == nil || c.button.Base().ProjectionRole() == nil {
		return nil
	}
	bounds := c.button.Base().LayoutRole().ArrangedBounds
	if bounds.IsEmpty() {
		bounds = c.bounds()
	}
	if bounds.IsEmpty() {
		return nil
	}
	return c.button.Base().ProjectionRole().Project(facet.ProjectionContext{Runtime: runtimeServicesOrNil(runtime), Bounds: bounds, ContentScale: contentScale})
}

func (c *popupPaletteChild) pointer(e facet.PointerEvent) bool {
	if c == nil || c.button == nil {
		return false
	}
	return c.button.onPointer(e)
}

func (c *popupPaletteChild) key(e facet.KeyEvent) bool {
	if c == nil || c.button == nil {
		return false
	}
	return c.button.onKey(e)
}

func (c *popupPaletteChild) bounds() gfx.Rect {
	if c == nil {
		return gfx.Rect{}
	}
	switch c.spec.kind {
	case popupPaletteChildTool:
		if c.parent != nil && c.index >= 0 && c.index < len(c.parent.cachedToolItemBounds) {
			return c.parent.cachedToolItemBounds[c.index]
		}
	case popupPaletteChildMirror:
		if c.parent != nil {
			return c.parent.cachedMirrorBounds
		}
	case popupPaletteChildCanvasOnly:
		if c.parent != nil {
			return c.parent.cachedCanvasBounds
		}
	case popupPaletteChildClearHistory:
		if c.parent != nil {
			return c.parent.cachedClearBounds
		}
	case popupPaletteChildToggleBar:
		if c.parent != nil {
			return c.parent.cachedToggleBounds
		}
	}
	return gfx.Rect{}
}

var _ facet.FacetImpl = (*PopupPalette)(nil)
var _ layout.AnchorExporter = (*PopupPalette)(nil)
var _ marks.Mark = (*PopupPalette)(nil)

func NewPopupPalette(label string, tools []PopupPaletteTool) *PopupPalette {
	p := &PopupPalette{
		Label:         marks.Const(label),
		Tools:         normalizePopupPaletteTools(tools),
		Open:          marks.Const(true),
		ShowBottomBar: marks.Const(true),
		Zoom:          marks.Const[float64](1),
		CanvasOnly:    marks.Const(false),
		MirrorCanvas:  marks.Const(false),
		SelectedIndex: marks.Const(-1),
		Disabled:      marks.Const(false),
		hoveredIndex:  -1,
		pressedIndex:  -1,
		Activated:     signal.NewSignal[string]("popup_palette_activated"),
	}
	p.Core.Facet = facet.NewFacet()
	p.AddBinding(p.Label)
	p.AddBinding(p.Open)
	p.AddBinding(p.Disabled)
	p.AddBinding(p.ShowBottomBar)
	p.AddBinding(p.Zoom)
	p.AddBinding(p.CanvasOnly)
	p.AddBinding(p.MirrorCanvas)
	p.AddBinding(p.SelectedIndex)

	p.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: popupPaletteGroupPolicy{palette: p},
		Children: p,
	}
	p.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := p.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	p.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return p.measure(ctx, constraints)
	}
	p.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.Layout.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult {
		return p.hitTest(pt)
	}
	p.Input.OnPointer = func(e facet.PointerEvent) bool {
		return p.onPointer(e)
	}
	p.Input.OnKey = func(e facet.KeyEvent) bool {
		return p.onKey(e)
	}
	p.Input.OnDismiss = func(e facet.DismissEvent) bool {
		return p.onDismiss(e)
	}
	p.Focus.Focusable = func() bool {
		return !p.Disabled.Get() && len(p.Tools) > 0
	}
	p.Focus.TabIndex = 0
	p.Focus.OnFocusGained = func() {
		p.onFocusGained()
	}
	p.Focus.OnFocusLost = func() {
		p.onFocusLost()
	}
	p.textRole.IMEEnabled = false
	p.AddRole(&p.textRole)

	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	p.RegisterRoles()

	p.composition = newPopupPaletteComposition(p)
	return p
}

func (p *PopupPalette) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

func (p *PopupPalette) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "action", TypeName: "popup_palette"}
}

func (p *PopupPalette) AccessibilityRole() string { return "toolbar" }

func (p *PopupPalette) AccessibleName() string {
	if p == nil {
		return ""
	}
	if name := strings.TrimSpace(p.Label.Get()); name != "" {
		return name
	}
	return "Popup palette"
}

func (p *PopupPalette) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	if p.composition != nil {
		if anchors := p.composition.exportAnchors(p.Layout.ArrangedBounds); len(anchors) > 0 {
			return anchors
		}
	}
	bounds := p.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	out := p.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if !p.cachedAnchorBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{X: p.cachedAnchorBounds.Min.X + p.cachedAnchorBounds.Width()*0.5, Y: p.cachedAnchorBounds.Min.Y + p.cachedAnchorBounds.Height()*0.5}
	} else if !p.cachedSurfaceBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{X: p.cachedSurfaceBounds.Min.X + p.cachedSurfaceBounds.Width()*0.5, Y: p.cachedSurfaceBounds.Min.Y + p.cachedSurfaceBounds.Height()*0.5}
	} else {
		out["content_anchor"] = bounds.Min
	}
	if p.cachedLabelLayout != nil {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: p.cachedLabelLayout.Baseline + p.cachedPadY}
	} else {
		out["baseline"] = out["content_anchor"]
	}
	return out
}

func (p *PopupPalette) Children() []facet.GroupChild {
	if p == nil || p.Disabled.Get() || !p.Open.Get() {
		return nil
	}
	return nil
}

func (p *PopupPalette) OnAttach(ctx facet.AttachContext) { p.Core.OnAttach() }

func (p *PopupPalette) OnActivate() { p.Core.OnActivate() }

func (p *PopupPalette) OnDeactivate() { p.Core.OnDeactivate() }

func (p *PopupPalette) OnDetach() {
	p.Core.OnDetach()
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.PopupPaletteSlots{}
	p.cachedRootBounds = gfx.Rect{}
	p.cachedTriggerBounds = gfx.Rect{}
	p.cachedSurfaceBounds = gfx.Rect{}
	p.cachedToolItemBounds = nil
	p.cachedHistoryBounds = nil
	p.cachedToolGroupBounds = gfx.Rect{}
	p.cachedAnchorBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	p.cachedLabelLayout = nil
	p.cachedLabelStyle = text.TextStyle{}
	p.cachedPadX = 0
	p.cachedPadY = 0
	p.cachedGap = 0
	p.cachedRingRadius = 0
	p.cachedRingThickness = 0
	p.cachedToolSize = 0
	p.cachedHistorySize = 0
	p.cachedBottomBarHeight = 0
	p.cachedSliderTrack = gfx.Rect{}
	p.cachedSliderThumb = gfx.Rect{}
	p.cachedMirrorBounds = gfx.Rect{}
	p.cachedCanvasBounds = gfx.Rect{}
	p.cachedClearBounds = gfx.Rect{}
	p.cachedToggleBounds = gfx.Rect{}
	p.cachedZoomBounds = gfx.Rect{}
	for _, child := range p.cachedChildren {
		if child != nil {
			child.dispose()
		}
	}
	p.cachedChildren = nil
}

func (p *PopupPalette) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Facet.Invalidate(flags)
}

func (p *PopupPalette) syncChildren() {
	if p == nil {
		return
	}
	if p.Disabled.Get() || !p.Open.Get() {
		for _, child := range p.cachedChildren {
			if child != nil {
				child.dispose()
			}
		}
		p.cachedChildren = nil
		return
	}
	specs := p.childSpecs()
	if len(p.cachedChildren) > len(specs) {
		for _, child := range p.cachedChildren[len(specs):] {
			if child != nil {
				child.dispose()
			}
		}
		p.cachedChildren = p.cachedChildren[:len(specs)]
	}
	if len(p.cachedChildren) < len(specs) {
		next := make([]*popupPaletteChild, len(specs))
		copy(next, p.cachedChildren)
		p.cachedChildren = next
	}
	for i := range specs {
		if p.cachedChildren[i] == nil {
			p.cachedChildren[i] = newPopupPaletteChild(p, i, specs[i])
		}
		p.cachedChildren[i].index = i
		p.cachedChildren[i].setSpec(specs[i])
	}
}

func (p *PopupPalette) syncChildMeasurements(ctx facet.MeasureContext, constraints facet.Constraints) {
	if p == nil || len(p.cachedChildren) == 0 {
		return
	}
	for i := range p.cachedChildren {
		child := p.cachedChildren[i]
		if child == nil {
			continue
		}
		child.measure(ctx, constraints)
	}
}

func (p *PopupPalette) syncChildArrangement(ctx facet.ArrangeContext) {
	if p == nil || len(p.cachedChildren) == 0 {
		return
	}
	for i := range p.cachedChildren {
		child := p.cachedChildren[i]
		if child == nil {
			continue
		}
		child.arrange(ctx)
	}
}

func (p *PopupPalette) childSpecs() []popupPaletteChildSpec {
	if p == nil || p.Disabled.Get() || !p.Open.Get() {
		return nil
	}
	out := make([]popupPaletteChildSpec, 0, len(p.Tools))
	toolInner := maxFloat(p.cachedToolSize*0.42, 18)
	for i, tool := range p.Tools {
		if strings.TrimSpace(tool.IconRef) == "" {
			continue
		}
		out = append(out, popupPaletteChildSpec{
			kind:            popupPaletteChildTool,
			index:           i,
			iconRef:         tool.IconRef,
			accessibleLabel: tool.AccessibleLabel,
			disabled:        p.Disabled.Get() || tool.Disabled,
			hitPadding:      maxFloat(0, (p.cachedToolSize-toolInner)*0.5),
			innerSize:       toolInner,
		})
	}
	if p.ShowBottomBar.Get() {
		controlInner := maxFloat(p.cachedToolSize*0.34, 16)
		controls := []struct {
			kind            popupPaletteChildKind
			iconRef         string
			accessibleLabel string
			disabled        bool
		}{
			{kind: popupPaletteChildMirror, iconRef: "mirror", accessibleLabel: "Mirror canvas"},
			{kind: popupPaletteChildCanvasOnly, iconRef: "canvas", accessibleLabel: "Canvas only"},
			{kind: popupPaletteChildClearHistory, iconRef: "history-clear", accessibleLabel: "Clear history", disabled: len(p.History) == 0},
			{kind: popupPaletteChildToggleBar, iconRef: "chevron-up", accessibleLabel: "Toggle bottom bar"},
		}
		for _, control := range controls {
			out = append(out, popupPaletteChildSpec{
				kind:            control.kind,
				iconRef:         control.iconRef,
				accessibleLabel: control.accessibleLabel,
				disabled:        p.Disabled.Get() || control.disabled,
				hitPadding:      maxFloat(0, (maxFloat(p.cachedToolSize*0.54, 24)-controlInner)*0.5),
				innerSize:       controlInner,
			})
		}
	}
	return out
}

func (p *PopupPalette) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.PopupPaletteSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolvePopupPaletteRecipe(style)
	return resolved, slots, true
}

func (p *PopupPalette) resolveProjectionTheme(runtime any) shared.PopupPaletteSlots {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, p.Base().ID()); store != nil {
			slots, _ := uiaction.ResolvePopupPaletteRecipe(store.Get())
			return slots
		}
	}
	return p.cachedRecipe
}

func (p *PopupPalette) fontRegistry(runtime any) *text.FontRegistry {
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

func (p *PopupPalette) newShaper(runtime any) *text.Shaper {
	registry := p.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (p *PopupPalette) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if p != nil && p.composition != nil {
		return p.composition.measure(ctx, constraints)
	}
	resolved, recipe, _ := p.resolveTheme(ctx)
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = recipe
	p.cachedWritingDirection = ctx.WritingDirection
	p.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	p.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	p.cachedToolSize = maxFloat(resolved.Density.Scale(52), resolved.Density.Scale(40))
	p.cachedHistorySize = maxFloat(resolved.Density.Scale(20), 16)
	p.cachedBottomBarHeight = maxFloat(resolved.Density.Scale(72), resolved.Density.Scale(60))
	p.cachedRingThickness = maxFloat(resolved.Density.Scale(14), 10)
	p.cachedRingRadius = maxFloat(resolved.Density.Scale(120), resolved.Density.Scale(96))
	p.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	p.syncChildren()

	maxW := constraints.MaxSize.W
	maxH := constraints.MaxSize.H
	if maxW <= 0 {
		maxW = resolved.Density.Scale(720)
	}
	if maxH <= 0 {
		maxH = resolved.Density.Scale(720)
	}

	shaper := p.newShaper(ctx.Runtime)
	if shaper != nil && strings.TrimSpace(p.Label.Get()) != "" {
		shaper.SetContentScale(ctx.ContentScale)
		p.cachedLabelLayout = shaper.ShapeTruncated(strings.TrimSpace(p.Label.Get()), p.cachedLabelStyle, maxW)
	} else {
		p.cachedLabelLayout = nil
	}
	p.textRole.Layout = p.cachedLabelLayout
	p.textRole.Selection = text.TextRange{}
	p.textRole.CaretVisible = false
	p.textRole.CaretPosition = text.TextPosition{}

	if p.Disabled.Get() || !p.Open.Get() {
		w := maxFloat(resolved.Density.Scale(72), p.cachedPadX*2+p.cachedToolSize)
		h := maxFloat(resolved.Density.Scale(72), p.cachedPadY*2+p.cachedToolSize)
		if p.cachedLabelLayout != nil {
			w = maxFloat(w, p.cachedLabelLayout.Bounds.Width()+p.cachedPadX*2)
			h += p.cachedLabelLayout.Bounds.Height()
		}
		size := constraints.Constrain(gfx.Size{W: minFloat(w, maxW), H: minFloat(h, maxH)})
		p.Layout.MeasuredSize = size
		p.Layout.MeasuredResult = facet.MeasureResult{
			Size: size,
			Intrinsic: facet.IntrinsicSize{
				Min:       size,
				Preferred: size,
				Max:       size,
			},
			Constraints: constraints,
		}
		p.syncChildMeasurements(ctx, constraints)
		return p.Layout.MeasuredResult
	}

	surfaceDiameter := maxFloat(resolved.Density.Scale(320), p.cachedRingRadius*2+p.cachedToolSize)
	bottomBarH := float32(0)
	if p.ShowBottomBar.Get() {
		bottomBarH = p.cachedBottomBarHeight
	}
	w := maxFloat(surfaceDiameter+2*p.cachedPadX, resolved.Density.Scale(360))
	h := p.cachedPadY + surfaceDiameter + p.cachedGap + bottomBarH + p.cachedPadY
	if p.cachedLabelLayout != nil {
		h += p.cachedLabelLayout.Bounds.Height() + p.cachedGap*0.5
	}
	if p.ShowBottomBar.Get() {
		h += p.cachedGap
	}
	size := constraints.Constrain(gfx.Size{W: minFloat(w, maxW), H: minFloat(h, maxH)})
	p.Layout.MeasuredSize = size
	p.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	p.syncChildMeasurements(ctx, constraints)
	return p.Layout.MeasuredResult
}

func (p *PopupPalette) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if p != nil && p.composition != nil {
		p.composition.arrange(ctx, bounds)
		return
	}
	p.cachedRootBounds = bounds
	p.cachedTriggerBounds = gfx.Rect{}
	p.cachedSurfaceBounds = gfx.Rect{}
	p.cachedToolItemBounds = nil
	p.cachedHistoryBounds = nil
	p.cachedToolGroupBounds = gfx.Rect{}
	p.cachedAnchorBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	p.cachedSliderTrack = gfx.Rect{}
	p.cachedSliderThumb = gfx.Rect{}
	p.cachedMirrorBounds = gfx.Rect{}
	p.cachedCanvasBounds = gfx.Rect{}
	p.cachedClearBounds = gfx.Rect{}
	p.cachedToggleBounds = gfx.Rect{}
	p.cachedZoomBounds = gfx.Rect{}
	p.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}

	if p.Disabled.Get() || !p.Open.Get() {
		triggerSize := minFloat(bounds.Width(), bounds.Height())
		if triggerSize <= 0 {
			triggerSize = maxFloat(p.cachedToolSize, resolvedDefaultTriggerSize())
		}
		center := gfx.RectFromXYWH(bounds.Min.X+(bounds.Width()-triggerSize)*0.5, bounds.Min.Y+(bounds.Height()-triggerSize)*0.5, triggerSize, triggerSize)
		p.cachedTriggerBounds = center
		p.cachedFocusBounds = center.Inset(-maxFloat(2, triggerSize*0.08), -maxFloat(2, triggerSize*0.08))
		p.cachedAnchorBounds = popupPaletteArrowBounds(center, false, p.cachedPadX)
		return
	}

	surfaceDiameter := minFloat(bounds.Width()-2*p.cachedPadX, bounds.Height()-2*p.cachedPadY)
	bottomBarH := float32(0)
	if p.ShowBottomBar.Get() {
		bottomBarH = p.cachedBottomBarHeight
	}
	if p.cachedLabelLayout != nil {
		bottomBarH += p.cachedLabelLayout.Bounds.Height() + p.cachedGap*0.5
	}
	surfaceDiameter = minFloat(surfaceDiameter, bounds.Height()-bottomBarH-p.cachedGap-2*p.cachedPadY)
	surfaceDiameter = maxFloat(surfaceDiameter, minFloat(resolvedDefaultSurfaceSize(), minFloat(bounds.Width(), bounds.Height())))
	if surfaceDiameter > bounds.Width()-2*p.cachedPadX {
		surfaceDiameter = bounds.Width() - 2*p.cachedPadX
	}
	if surfaceDiameter > bounds.Height()-bottomBarH-p.cachedGap-2*p.cachedPadY {
		surfaceDiameter = bounds.Height() - bottomBarH - p.cachedGap - 2*p.cachedPadY
	}
	if surfaceDiameter < resolvedDefaultTriggerSize()*2 {
		surfaceDiameter = resolvedDefaultTriggerSize() * 2
	}
	surfaceX := bounds.Min.X + (bounds.Width()-surfaceDiameter)*0.5
	surfaceY := bounds.Min.Y + p.cachedPadY
	p.cachedSurfaceBounds = gfx.RectFromXYWH(surfaceX, surfaceY, surfaceDiameter, surfaceDiameter)
	p.cachedTriggerBounds = p.cachedSurfaceBounds
	p.cachedFocusBounds = p.cachedSurfaceBounds.Inset(-maxFloat(2, p.cachedSurfaceBounds.Width()*0.05), -maxFloat(2, p.cachedSurfaceBounds.Height()*0.05))

	center := gfx.Point{X: p.cachedSurfaceBounds.Min.X + p.cachedSurfaceBounds.Width()*0.5, Y: p.cachedSurfaceBounds.Min.Y + p.cachedSurfaceBounds.Height()*0.5}
	outerRadius := p.cachedSurfaceBounds.Width() * 0.5
	p.cachedRingRadius = outerRadius * 0.70
	itemSize := p.cachedToolSize
	if itemSize <= 0 {
		itemSize = resolvedDefaultTriggerSize()
	}
	p.cachedToolItemBounds = make([]gfx.Rect, len(p.Tools))
	if len(p.Tools) > 0 {
		startAngle := -math.Pi / 2
		step := 2 * math.Pi / float64(len(p.Tools))
		if p.cachedWritingDirection == facet.WritingDirectionRTL {
			step = -step
		}
		for i := range p.Tools {
			angle := startAngle + step*float64(i)
			if p.cachedWritingDirection == facet.WritingDirectionRTL {
				angle = startAngle - step*float64(i)
			}
			dx := float32(math.Cos(angle)) * p.cachedRingRadius
			dy := float32(math.Sin(angle)) * p.cachedRingRadius
			bounds := gfx.RectFromXYWH(center.X+dx-itemSize*0.5, center.Y+dy-itemSize*0.5, itemSize, itemSize)
			p.cachedToolItemBounds[i] = bounds
		}
	}

	toolGroupTop := p.cachedSurfaceBounds.Max.Y + p.cachedGap
	if p.cachedLabelLayout != nil {
		toolGroupTop += p.cachedLabelLayout.Bounds.Height() + p.cachedGap*0.5
	}
	if p.ShowBottomBar.Get() {
		p.cachedToolGroupBounds = gfx.RectFromXYWH(bounds.Min.X+p.cachedPadX, toolGroupTop, bounds.Width()-2*p.cachedPadX, p.cachedBottomBarHeight)
		inner := p.cachedToolGroupBounds.Inset(p.cachedPadX, p.cachedPadY*0.5)
		sliderTrackW := maxFloat(inner.Width()*0.46, p.cachedToolSize*2.5)
		if sliderTrackW > inner.Width()*0.9 {
			sliderTrackW = inner.Width() * 0.9
		}
		sliderTrackH := maxFloat(p.cachedPadY*0.25, 6)
		p.cachedZoomBounds = gfx.RectFromXYWH(inner.Min.X, inner.Min.Y+(inner.Height()-sliderTrackH)*0.5, sliderTrackW, sliderTrackH)
		thumbX := p.cachedZoomBounds.Min.X + clamp01Float(float32((p.Zoom.Get()-0.1)/3.9))*maxFloat(0, p.cachedZoomBounds.Width()-p.cachedToolSize*0.34)
		p.cachedSliderThumb = gfx.RectFromXYWH(thumbX, p.cachedZoomBounds.Min.Y-(p.cachedToolSize*0.34-p.cachedZoomBounds.Height())*0.5, p.cachedToolSize*0.34, p.cachedToolSize*0.34)
		buttonSize := maxFloat(p.cachedToolSize*0.54, 24)
		buttonY := inner.Min.Y + (inner.Height()-buttonSize)*0.5
		right := inner.Max.X
		p.cachedToggleBounds = gfx.RectFromXYWH(right-buttonSize, buttonY, buttonSize, buttonSize)
		right -= buttonSize + p.cachedGap
		p.cachedClearBounds = gfx.RectFromXYWH(right-buttonSize, buttonY, buttonSize, buttonSize)
		right -= buttonSize + p.cachedGap
		p.cachedCanvasBounds = gfx.RectFromXYWH(right-buttonSize, buttonY, buttonSize, buttonSize)
		right -= buttonSize + p.cachedGap
		p.cachedMirrorBounds = gfx.RectFromXYWH(right-buttonSize, buttonY, buttonSize, buttonSize)
	}

	historySize := p.cachedHistorySize
	if historySize <= 0 {
		historySize = resolvedDefaultHistorySize()
	}
	if len(p.History) > 0 {
		p.cachedHistoryBounds = make([]gfx.Rect, len(p.History))
		originX := p.cachedSurfaceBounds.Min.X + p.cachedPadX*0.6
		originY := p.cachedSurfaceBounds.Min.Y + p.cachedPadY*1.5
		for i := range p.History {
			y := originY + float32(i)*(historySize+p.cachedGap*0.5)
			p.cachedHistoryBounds[i] = gfx.RectFromXYWH(originX, y, historySize, historySize)
		}
	}

	arrowW := maxFloat(p.cachedPadX*0.7, 14)
	arrowH := maxFloat(p.cachedPadY*0.7, 10)
	p.cachedAnchorBounds = gfx.RectFromXYWH(center.X-arrowW*0.5, p.cachedSurfaceBounds.Min.Y-arrowH*0.8, arrowW, arrowH)
	p.syncChildArrangement(ctx)
}

func (p *PopupPalette) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if p != nil && p.composition != nil {
		return p.composition.buildCommands(bounds, runtime, contentScale)
	}
	if p == nil || bounds.IsEmpty() {
		return nil
	}
	slots := p.resolveProjectionTheme(runtime)
	tokens := p.cachedTokens
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, p.Base().ID()); store != nil {
			tokens = store.Get().Tokens
		}
	}
	state := p.interactionState()
	root := slots.Root.Resolve(state, tokens)
	surface := slots.PaletteSurface.Resolve(state, tokens)
	toolItems := slots.ToolItems.Resolve(state, tokens)
	toolGroup := slots.ToolGroup.Resolve(state, tokens)
	anchorArrow := slots.AnchorArrow.Resolve(state, tokens)
	focusRing := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 128)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}

	if p.Disabled.Get() || !p.Open.Get() {
		if !isTransparentMaterial(surface) && !p.cachedTriggerBounds.IsEmpty() {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(p.cachedTriggerBounds, p.cachedTriggerBounds.Height()*0.28), surface)...)
		}
		if p.cachedLabelLayout != nil && !isTransparentMaterial(toolItems) {
			labelRect := gfx.RectFromXYWH(p.cachedTriggerBounds.Min.X+p.cachedPadX*0.5, p.cachedTriggerBounds.Min.Y+p.cachedPadY*0.15, p.cachedLabelLayout.Bounds.Width(), p.cachedLabelLayout.Bounds.Height())
			cmds = append(cmds, primitive.TextLayoutCommands(p.cachedLabelLayout, labelRect, gfx.SolidBrush(materialColor(toolItems)))...)
		}
		if !isTransparentMaterial(focusRing) && p.focusedVisible && !p.cachedFocusBounds.IsEmpty() {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(p.cachedFocusBounds, p.cachedFocusBounds.Height()*0.28), focusRing)...)
		}
		return cmds
	}

	if !isTransparentMaterial(surface) && !p.cachedSurfaceBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.CirclePath(gfx.Point{X: p.cachedSurfaceBounds.Min.X + p.cachedSurfaceBounds.Width()*0.5, Y: p.cachedSurfaceBounds.Min.Y + p.cachedSurfaceBounds.Height()*0.5}, p.cachedSurfaceBounds.Width()*0.5), surface)...)
	}
	if !isTransparentMaterial(surface) && !p.cachedSurfaceBounds.IsEmpty() {
		inner := p.cachedSurfaceBounds.Inset(p.cachedSurfaceBounds.Width()*0.28, p.cachedSurfaceBounds.Height()*0.28)
		wheelColors := dataPaletteFromTokens(tokens)
		for i := 0; i < 12; i++ {
			start := (2 * math.Pi / 12) * float64(i)
			end := (2 * math.Pi / 12) * float64(i+1)
			path := popupPaletteSectorPath(centerOfRect(p.cachedSurfaceBounds), float64(inner.Width()*0.5), float64(p.cachedSurfaceBounds.Width()*0.5), start-math.Pi/2, end-math.Pi/2)
			cmds = append(cmds, materialCommands(path, theme.MarkStyle{Base: theme.FromToken(wheelColors[i%len(wheelColors)])}.Resolve(state, tokens))...)
		}
	}
	if !p.cachedAnchorBounds.IsEmpty() && !isTransparentMaterial(anchorArrow) {
		cmds = append(cmds, materialCommands(popupPaletteArrowPath(p.cachedAnchorBounds), anchorArrow)...)
	}
	if p.cachedLabelLayout != nil && !isTransparentMaterial(toolItems) {
		labelRect := gfx.RectFromXYWH(p.cachedSurfaceBounds.Min.X+p.cachedPadX*0.5, p.cachedSurfaceBounds.Min.Y+p.cachedPadY*0.2, p.cachedLabelLayout.Bounds.Width(), p.cachedLabelLayout.Bounds.Height())
		cmds = append(cmds, primitive.TextLayoutCommands(p.cachedLabelLayout, labelRect, gfx.SolidBrush(materialColor(toolItems)))...)
	}

	for i, tool := range p.Tools {
		bounds := p.cachedToolItemBounds[i]
		if bounds.IsEmpty() {
			continue
		}
		itemState := p.itemState(i)
		itemMaterial := theme.FromToken(tintColor(tokens.Color.SurfaceInverse, 0.64))
		switch itemState {
		case theme.StateHover:
			itemMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.14))
		case theme.StatePressed:
			itemMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.24))
		case theme.StateFocused:
			itemMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.18))
		}
		if tool.Selected || i == p.SelectedIndex.Get() {
			itemMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.26))
		}
		cmds = append(cmds, materialCommands(gfx.CirclePath(centerOfRect(bounds), bounds.Width()*0.5), itemMaterial)...)
		strokeMat := theme.FromToken(tintColor(tokens.Color.OnSurfaceVariant, 0.36))
		if tool.Selected || i == p.SelectedIndex.Get() {
			strokeMat = theme.FromToken(tokens.Color.Primary)
		}
		cmds = append(cmds, materialCommands(gfx.CirclePath(centerOfRect(bounds), bounds.Width()*0.5), theme.MarkStyle{Base: theme.Material{Fills: []theme.Fill{{Type: theme.FillNone, Opacity: 0}}, Strokes: []theme.MaterialStroke{{Paint: theme.Fill{Type: theme.FillSolid, Color: materialColor(strokeMat), Opacity: 1}, Width: maxFloat(1, bounds.Width()*0.08)}}}}.Resolve(theme.StateDefault, tokens))...)
		if tool.Color.A > 0 {
			cmds = append(cmds, gfx.FillPath{Path: gfx.CirclePath(centerOfRect(bounds), bounds.Width()*0.18), Brush: gfx.SolidBrush(tool.Color)})
		}
	}
	for i := range p.cachedChildren {
		child := p.cachedChildren[i]
		if child == nil || !child.isTool() {
			continue
		}
		if projected := child.project(runtimeServicesOrNil(runtime), contentScale); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}

	for i, swatch := range p.History {
		bounds := p.cachedHistoryBounds[i]
		if bounds.IsEmpty() {
			continue
		}
		cmds = append(cmds, gfx.FillPath{Path: gfx.CirclePath(centerOfRect(bounds), bounds.Width()*0.5), Brush: gfx.SolidBrush(swatch)})
		cmds = append(cmds, materialCommands(gfx.CirclePath(centerOfRect(bounds), bounds.Width()*0.5), theme.MarkStyle{Base: theme.Material{Fills: []theme.Fill{{Type: theme.FillNone, Opacity: 0}}, Strokes: []theme.MaterialStroke{{Paint: theme.Fill{Type: theme.FillSolid, Color: materialColor(toolGroup), Opacity: 1}, Width: maxFloat(1, bounds.Width()*0.08)}}}}.Resolve(theme.StateDefault, tokens))...)
	}

	if p.ShowBottomBar.Get() && !p.cachedToolGroupBounds.IsEmpty() {
		if !isTransparentMaterial(toolGroup) {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(p.cachedToolGroupBounds, p.cachedToolGroupBounds.Height()*0.28), toolGroup)...)
		}
		if !p.cachedZoomBounds.IsEmpty() {
			track := theme.FromToken(tintColor(tokens.Color.OnSurfaceVariant, 0.28))
			active := theme.FromToken(tokens.Color.Primary)
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(p.cachedZoomBounds, p.cachedZoomBounds.Height()*0.5), track)...)
			thumbCenter := centerOfRect(p.cachedSliderThumb)
			activeTrack := gfx.RectFromXYWH(p.cachedZoomBounds.Min.X, p.cachedZoomBounds.Min.Y, thumbCenter.X-p.cachedZoomBounds.Min.X, p.cachedZoomBounds.Height())
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(activeTrack, activeTrack.Height()*0.5), active)...)
			cmds = append(cmds, materialCommands(gfx.CirclePath(centerOfRect(p.cachedSliderThumb), p.cachedSliderThumb.Width()*0.5), theme.FromToken(tokens.Color.OnPrimary))...)
		}
	}
	for i := range p.cachedChildren {
		child := p.cachedChildren[i]
		if child == nil || child.isTool() {
			continue
		}
		if projected := child.project(runtimeServicesOrNil(runtime), contentScale); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}

	if p.focusedVisible && !isTransparentMaterial(focusRing) && !p.cachedFocusBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.CirclePath(centerOfRect(p.cachedFocusBounds), p.cachedFocusBounds.Width()*0.5), focusRing)...)
	}
	return cmds
}

func (p *PopupPalette) hitTest(pt gfx.Point) facet.HitResult {
	if p != nil && p.composition != nil {
		return p.composition.hitTest(pt)
	}
	if p == nil || p.Layout.ArrangedBounds.IsEmpty() || !p.Layout.ArrangedBounds.Contains(pt) {
		return facet.HitResult{}
	}
	cursor := p.cursorShape()
	if p.focusedVisible && !p.cachedFocusBounds.IsEmpty() && p.cachedFocusBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDFocusRing, Cursor: cursor}
	}
	for i, bounds := range p.cachedToolItemBounds {
		if bounds.Contains(pt) {
			_ = i
			return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDToolItems, Cursor: cursor}
		}
	}
	if p.cachedToolGroupBounds.Contains(pt) || p.cachedZoomBounds.Contains(pt) || p.cachedMirrorBounds.Contains(pt) || p.cachedCanvasBounds.Contains(pt) || p.cachedClearBounds.Contains(pt) || p.cachedToggleBounds.Contains(pt) || p.cachedHistoryBoundsContains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDToolGroup, Cursor: cursor}
	}
	if p.cachedAnchorBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDAnchorArrow, Cursor: cursor}
	}
	if p.cachedSurfaceBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDSurface, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDRoot, Cursor: cursor}
}

func (p *PopupPalette) cursorShape() facet.CursorShape {
	if p.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (p *PopupPalette) onPointer(e facet.PointerEvent) bool {
	if p != nil && p.composition != nil {
		return p.composition.onPointer(e)
	}
	if p.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		prevIndex := p.hoveredIndex
		prevControl := p.hoveredControl
		p.hoveredIndex = p.toolIndexAt(e.Position)
		p.hoveredControl = p.controlAt(e.Position)
		if p.draggingZoom {
			p.updateZoomFromPoint(e.Position)
		}
		if prevIndex != p.hoveredIndex || prevControl != p.hoveredControl {
			p.forwardPointerToChild(facet.PointerEvent{Kind: platform.PointerLeave, Position: e.Position}, prevIndex, prevControl)
		}
		p.forwardPointerToChild(e, p.hoveredIndex, p.hoveredControl)
		p.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		prevIndex := p.hoveredIndex
		prevControl := p.hoveredControl
		p.hoveredIndex = -1
		p.hoveredControl = popupPaletteControlNone
		if !p.draggingZoom {
			p.focusFromPointer = false
		}
		p.forwardPointerToChild(facet.PointerEvent{Kind: platform.PointerLeave, Position: e.Position}, prevIndex, prevControl)
		p.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		p.focusFromPointer = true
		p.focusedVisible = false
		p.hoveredIndex = p.toolIndexAt(e.Position)
		p.hoveredControl = p.controlAt(e.Position)
		if p.Disabled.Get() {
			return false
		}
		if p.hoveredIndex >= 0 {
			p.pressedIndex = p.hoveredIndex
			p.forwardPointerToChild(e, p.hoveredIndex, popupPaletteControlNone)
			p.invalidate(facet.DirtyProjection)
			return true
		}
		if p.hoveredControl != popupPaletteControlNone {
			p.pressedControl = p.hoveredControl
			if p.hoveredControl == popupPaletteControlZoom {
				p.draggingZoom = true
				p.updateZoomFromPoint(e.Position)
			}
			p.forwardPointerToChild(e, -1, p.hoveredControl)
			p.invalidate(facet.DirtyProjection)
			return true
		}
		if p.Open.Get() && p.cachedSurfaceBounds.Contains(e.Position) {
			p.invalidate(facet.DirtyProjection)
			return true
		}
		p.Open = marks.Const(true)
		p.focusedVisible = !p.Disabled.Get()
		if p.composition != nil {
			p.composition.sync()
		}
		p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		if p.pressedIndex >= 0 {
			wasPressed := p.pressedIndex == p.toolIndexAt(e.Position)
			index := p.pressedIndex
			p.pressedIndex = -1
			forwarded := p.forwardPointerToChild(e, index, popupPaletteControlNone)
			p.invalidate(facet.DirtyProjection)
			if wasPressed && !forwarded {
				p.activateTool(index)
				return true
			}
			return wasPressed || forwarded
		}
		if p.pressedControl != popupPaletteControlNone {
			control := p.pressedControl
			p.pressedControl = popupPaletteControlNone
			p.draggingZoom = false
			forwarded := p.forwardPointerToChild(e, -1, control)
			p.invalidate(facet.DirtyProjection)
			if p.controlAt(e.Position) != control && control != popupPaletteControlZoom {
				return false
			}
			if !forwarded {
				return p.activateControl(control, e.Position)
			}
			return true
		}
		return false
	default:
		return false
	}
}

func (p *PopupPalette) onKey(e facet.KeyEvent) bool {
	if p != nil && p.composition != nil {
		if p.composition.onKey(e) {
			return true
		}
	}
	if p.Disabled.Get() {
		return false
	}
	if !p.Open.Get() {
		switch e.Kind {
		case platform.KeyPress:
			switch e.Key {
			case platform.KeyEnter, platform.KeySpace:
				p.Open = marks.Const(true)
				p.focusedVisible = !p.Disabled.Get()
				if p.composition != nil {
					p.composition.sync()
				}
				p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
				return true
			}
		}
		return false
	}
	switch e.Kind {
	case platform.KeyPress, platform.KeyRepeat:
		switch e.Key {
		case platform.KeyEscape:
			p.Open = marks.Const(false)
			p.hoveredIndex = -1
			p.pressedIndex = -1
			p.hoveredControl = popupPaletteControlNone
			p.pressedControl = popupPaletteControlNone
			p.draggingZoom = false
			if p.composition != nil {
				p.composition.sync()
			}
			p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		case platform.KeyLeft, platform.KeyUp:
			p.navigateTools(-1)
			return true
		case platform.KeyRight, platform.KeyDown:
			p.navigateTools(1)
			return true
		case platform.KeyHome:
			p.SelectedIndex = marks.Const(0)
			if p.composition != nil {
				p.composition.sync()
			}
			p.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyEnd:
			p.SelectedIndex = marks.Const(len(p.Tools) - 1)
			if p.composition != nil {
				p.composition.sync()
			}
			p.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyEnter, platform.KeySpace:
			if p.SelectedIndex.Get() >= 0 && p.SelectedIndex.Get() < len(p.Tools) {
				p.activateTool(p.SelectedIndex.Get())
				return true
			}
		case platform.KeyPageUp:
			p.Zoom = marks.Const(clampPopupZoom(p.Zoom.Get() + 0.05))
			p.Activated.Emit(fmt.Sprintf("zoom:%.0f", p.Zoom.Get()*100))
			if p.composition != nil {
				p.composition.sync()
			}
			p.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyPageDown:
			p.Zoom = marks.Const(clampPopupZoom(p.Zoom.Get() - 0.05))
			p.Activated.Emit(fmt.Sprintf("zoom:%.0f", p.Zoom.Get()*100))
			if p.composition != nil {
				p.composition.sync()
			}
			p.invalidate(facet.DirtyProjection)
			return true
		}
	}
	return false
}

func (p *PopupPalette) onDismiss(e facet.DismissEvent) bool {
	if p != nil && p.composition != nil {
		return p.composition.onDismiss(e)
	}
	_ = e
	if p.Disabled.Get() || !p.Open.Get() {
		return false
	}
	p.Open = marks.Const(false)
	p.hoveredIndex = -1
	p.pressedIndex = -1
	p.hoveredControl = popupPaletteControlNone
	p.pressedControl = popupPaletteControlNone
	p.draggingZoom = false
	if p.composition != nil {
		p.composition.sync()
	}
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	return true
}

func (p *PopupPalette) onFocusGained() {
	if p.Disabled.Get() {
		return
	}
	p.focusedVisible = !p.focusFromPointer
	p.invalidate(facet.DirtyProjection)
}

func (p *PopupPalette) onFocusLost() {
	p.focusedVisible = false
	p.focusFromPointer = false
	p.draggingZoom = false
	p.invalidate(facet.DirtyProjection)
}

func (p *PopupPalette) interactionState() theme.InteractionState {
	switch {
	case p.Disabled.Get():
		return theme.StateDisabled
	case p.pressedIndex >= 0 || p.pressedControl != popupPaletteControlNone:
		return theme.StatePressed
	case p.hoveredIndex >= 0 || p.hoveredControl != popupPaletteControlNone:
		return theme.StateHover
	case p.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (p *PopupPalette) itemState(index int) theme.InteractionState {
	switch {
	case p.Disabled.Get() || index < 0 || index >= len(p.Tools):
		return theme.StateDisabled
	case p.pressedIndex == index:
		return theme.StatePressed
	case p.hoveredIndex == index:
		return theme.StateHover
	case p.focusedVisible && p.SelectedIndex.Get() == index:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (p *PopupPalette) navigateTools(delta int) {
	if len(p.Tools) == 0 {
		return
	}
	next := p.SelectedIndex.Get()
	if next < 0 {
		next = 0
	} else {
		next = (next + delta + len(p.Tools)) % len(p.Tools)
	}
	p.SelectedIndex = marks.Const(next)
	if p.composition != nil {
		p.composition.sync()
	}
	p.invalidate(facet.DirtyProjection)
}

func (p *PopupPalette) activateTool(index int) {
	if p == nil || index < 0 || index >= len(p.Tools) {
		return
	}
	tool := p.Tools[index]
	if tool.Disabled {
		return
	}
	p.SelectedIndex = marks.Const(index)
	if key := strings.TrimSpace(tool.Key); key != "" {
		p.Activated.Emit(key)
	} else if label := strings.TrimSpace(tool.Label); label != "" {
		p.Activated.Emit(label)
	}
	if !tool.Selected {
		p.Tools[index].Selected = true
	}
	p.invalidate(facet.DirtyProjection)
}

func (p *PopupPalette) activateControl(control popupPaletteControlKind, pt gfx.Point) bool {
	switch control {
	case popupPaletteControlMirror:
		p.MirrorCanvas = marks.Const(!p.MirrorCanvas.Get())
		p.Activated.Emit("mirror")
		p.invalidate(facet.DirtyProjection)
		return true
	case popupPaletteControlCanvasOnly:
		p.CanvasOnly = marks.Const(!p.CanvasOnly.Get())
		p.Activated.Emit("canvas_only")
		p.invalidate(facet.DirtyProjection)
		return true
	case popupPaletteControlZoom:
		p.updateZoomFromPoint(pt)
		p.Activated.Emit(fmt.Sprintf("zoom:%.0f", p.Zoom.Get()*100))
		return true
	case popupPaletteControlClearHistory:
		p.History = nil
		p.Activated.Emit("history_clear")
		p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	case popupPaletteControlToggleBar:
		p.ShowBottomBar = marks.Const(!p.ShowBottomBar.Get())
		p.Activated.Emit("bottom_bar")
		p.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	default:
		return false
	}
}

func (p *PopupPalette) updateZoomFromPoint(pt gfx.Point) {
	if p == nil || p.cachedZoomBounds.IsEmpty() {
		return
	}
	t := (pt.X - p.cachedZoomBounds.Min.X) / p.cachedZoomBounds.Width()
	p.Zoom = marks.Const(0.1 + float64(clamp01Float(t))*3.9)
}

func (p *PopupPalette) controlAt(pt gfx.Point) popupPaletteControlKind {
	switch {
	case p.cachedMirrorBounds.Contains(pt):
		return popupPaletteControlMirror
	case p.cachedCanvasBounds.Contains(pt):
		return popupPaletteControlCanvasOnly
	case p.cachedClearBounds.Contains(pt):
		return popupPaletteControlClearHistory
	case p.cachedToggleBounds.Contains(pt):
		return popupPaletteControlToggleBar
	case p.cachedZoomBounds.Contains(pt) || p.cachedSliderThumb.Contains(pt):
		return popupPaletteControlZoom
	default:
		return popupPaletteControlNone
	}
}

func (p *PopupPalette) forwardPointerToChild(e facet.PointerEvent, toolIndex int, control popupPaletteControlKind) bool {
	if p == nil || len(p.cachedChildren) == 0 {
		return false
	}
	for i := range p.cachedChildren {
		child := p.cachedChildren[i]
		if child == nil {
			continue
		}
		switch child.spec.kind {
		case popupPaletteChildTool:
			if child.spec.index != toolIndex {
				continue
			}
		case popupPaletteChildMirror:
			if control != popupPaletteControlMirror {
				continue
			}
		case popupPaletteChildCanvasOnly:
			if control != popupPaletteControlCanvasOnly {
				continue
			}
		case popupPaletteChildClearHistory:
			if control != popupPaletteControlClearHistory {
				continue
			}
		case popupPaletteChildToggleBar:
			if control != popupPaletteControlToggleBar {
				continue
			}
		}
		_ = child.pointer(e)
		return true
	}
	return false
}

func (p *PopupPalette) toolIndexAt(pt gfx.Point) int {
	for i, bounds := range p.cachedToolItemBounds {
		if bounds.Contains(pt) {
			return i
		}
	}
	return -1
}

func (p *PopupPalette) clampToolIndex(index int) int {
	if len(p.Tools) == 0 {
		return -1
	}
	if index < 0 {
		return -1
	}
	if index >= len(p.Tools) {
		return len(p.Tools) - 1
	}
	return index
}

func (p *PopupPalette) clampIndices() {
	p.SelectedIndex = marks.Const(p.clampToolIndex(p.SelectedIndex.Get()))
	p.hoveredIndex = p.clampToolIndex(p.hoveredIndex)
	p.pressedIndex = p.clampToolIndex(p.pressedIndex)
}

func normalizePopupPaletteTools(tools []PopupPaletteTool) []PopupPaletteTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]PopupPaletteTool, 0, len(tools))
	for _, tool := range tools {
		tool.Key = strings.TrimSpace(tool.Key)
		tool.Label = strings.TrimSpace(tool.Label)
		tool.AccessibleLabel = strings.TrimSpace(tool.AccessibleLabel)
		if tool.AccessibleLabel == "" {
			tool.AccessibleLabel = tool.Label
		}
		out = append(out, tool)
	}
	return out
}

func clampPopupZoom(zoom float64) float64 {
	if zoom < 0.1 {
		return 0.1
	}
	if zoom > 4.0 {
		return 4.0
	}
	return zoom
}

func clamp01Float(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func resolvedDefaultTriggerSize() float32 { return 48 }

func resolvedDefaultSurfaceSize() float32 { return 320 }

func resolvedDefaultHistorySize() float32 { return 20 }

func centerOfRect(r gfx.Rect) gfx.Point {
	return gfx.Point{X: r.Min.X + r.Width()*0.5, Y: r.Min.Y + r.Height()*0.5}
}

func popupPaletteSectorPath(center gfx.Point, innerRadius, outerRadius, startAngle, endAngle float64) gfx.Path {
	startOuter := gfx.Point{X: center.X + float32(math.Cos(startAngle))*float32(outerRadius), Y: center.Y + float32(math.Sin(startAngle))*float32(outerRadius)}
	endOuter := gfx.Point{X: center.X + float32(math.Cos(endAngle))*float32(outerRadius), Y: center.Y + float32(math.Sin(endAngle))*float32(outerRadius)}
	startInner := gfx.Point{X: center.X + float32(math.Cos(endAngle))*float32(innerRadius), Y: center.Y + float32(math.Sin(endAngle))*float32(innerRadius)}
	endInner := gfx.Point{X: center.X + float32(math.Cos(startAngle))*float32(innerRadius), Y: center.Y + float32(math.Sin(startAngle))*float32(innerRadius)}
	return gfx.NewPath().
		MoveTo(startOuter).
		LineTo(endOuter).
		LineTo(startInner).
		LineTo(endInner).
		Close().
		Build()
}

func popupPaletteArrowPath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Min.Y}).
		LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y}).
		Close().
		Build()
}

func popupPaletteArrowBounds(surface gfx.Rect, up bool, pad float32) gfx.Rect {
	w := maxFloat(surface.Width()*0.12, pad)
	h := maxFloat(surface.Height()*0.08, pad*0.8)
	x := surface.Min.X + (surface.Width()-w)*0.5
	y := surface.Min.Y - h*0.9
	if !up {
		y = surface.Max.Y - h*0.1
	}
	return gfx.RectFromXYWH(x, y, w, h)
}

func dataPaletteFromTokens(tokens theme.Tokens) []gfx.Color {
	if len(tokens.Color.DataPalette) > 0 {
		return tokens.Color.DataPalette
	}
	return []gfx.Color{
		tokens.Color.Primary,
		tokens.Color.Secondary,
		tokens.Color.SecondaryVariant,
		tokens.Color.Error,
		tokens.Color.Warning,
		tokens.Color.Success,
		tokens.Color.Info,
		tokens.Color.OnSurfaceVariant,
	}
}

func (p *PopupPalette) cachedHistoryBoundsContains(pt gfx.Point) bool {
	for _, bounds := range p.cachedHistoryBounds {
		if bounds.Contains(pt) {
			return true
		}
	}
	return false
}

type popupPaletteGroupPolicy struct {
	palette *PopupPalette
}

func (popupPaletteGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (popupPaletteGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (popupPaletteGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}

type popupPaletteComposition struct {
	palette *PopupPalette

	menu        *RadialMenu
	center      *input.ColorPicker
	toolButtons []*IconButton
	control     *ActionGroup

	cachedLabelLayout  *text.TextLayout
	cachedLabelStyle    text.TextStyle
	cachedMenuBounds    gfx.Rect
	cachedControlBounds gfx.Rect
	cachedHistoryBounds []gfx.Rect
	cachedPadX         float32
	cachedPadY         float32
	cachedGap          float32
	cachedHistorySize  float32
}

func newPopupPaletteComposition(p *PopupPalette) *popupPaletteComposition {
	c := &popupPaletteComposition{palette: p}
	c.center = input.NewColorPicker("Palette color")
	c.center.Disabled = marks.Const(p.Disabled.Get())
	c.control = NewActionGroup(marks.Const("Palette controls"), marks.Const([]ActionGroupAction{
		{Key: "mirror", AccessibleLabel: "Mirror canvas", IconRef: "mirror"},
		{Key: "canvas_only", AccessibleLabel: "Canvas only", IconRef: "canvas"},
		{Key: "history_clear", AccessibleLabel: "Clear history", IconRef: "history-clear"},
		{Key: "bottom_bar", AccessibleLabel: "Toggle bottom bar", IconRef: "chevron-up"},
	}))
	c.control.Activated.Subscribe(func(key string) {
		if c.palette == nil {
			return
		}
		switch key {
		case "mirror":
			c.palette.activateControl(popupPaletteControlMirror, gfx.Point{})
		case "canvas_only":
			c.palette.activateControl(popupPaletteControlCanvasOnly, gfx.Point{})
		case "history_clear":
			c.palette.activateControl(popupPaletteControlClearHistory, gfx.Point{})
		case "bottom_bar":
			c.palette.activateControl(popupPaletteControlToggleBar, gfx.Point{})
		}
	})
	c.menu = NewRadialMenu(p.Label.Get(), c.center, nil)
	c.menu.DefaultTrackRadius = 120
	c.sync()
	return c
}

func (c *popupPaletteComposition) sync() {
	if c == nil || c.palette == nil || c.menu == nil {
		return
	}
	p := c.palette
	c.menu.Label = marks.Const(p.Label.Get())
	c.menu.Open = p.Open.Get()
	c.menu.Disabled = marks.Const(p.Disabled.Get())
	c.center.Disabled = marks.Const(p.Disabled.Get())
	c.control.Disabled = marks.Const(p.Disabled.Get() || !p.ShowBottomBar.Get())
	c.center.Label = marks.Const(p.Label.Get())
	c.center.SetColor(paletteSelectionColor(p))

	c.cachedLabelStyle = text.TextStyle{}
	c.cachedLabelLayout = nil
	c.cachedPadX = 0
	c.cachedPadY = 0
	c.cachedGap = 0
	c.cachedHistorySize = 0

	toolButtons := make([]*IconButton, 0, len(p.Tools))
	for i := range p.Tools {
		tool := p.Tools[i]
		if strings.TrimSpace(tool.IconRef) == "" {
			continue
		}
		btn := NewIconButton(primitive.IconRef(tool.IconRef))
		btn.Activated.Subscribe(func(signal.Unit) {
			if c.palette != nil {
				c.palette.activateTool(i)
			}
		})
		btn.Icon = primitive.IconRef(tool.IconRef)
		btn.AccessibleLabel = marks.Const(tool.AccessibleLabel)
		btn.Disabled = marks.Const(p.Disabled.Get() || tool.Disabled)
		btn.Size = marks.Const[float32](28)
		btn.HitPadding = 10
		toolButtons = append(toolButtons, btn)
	}
	c.toolButtons = toolButtons

	orbit := make([]RadialChild, 0, len(c.toolButtons))
	for _, btn := range c.toolButtons {
		orbit = append(orbit, RadialChild{
			Child: btn,
			Placement: facet.RadialPlacement{
				Angle:       math.NaN(),
				RadiusTrack: -1,
			},
		})
	}
	c.menu.CenterChild = c.center
	c.menu.RadialChildren = marks.Const(orbit)
}

func (c *popupPaletteComposition) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	if c == nil || c.palette == nil {
		return facet.MeasureResult{}
	}
	c.sync()
	resolved, _, _ := c.palette.resolveTheme(ctx)
	c.palette.cachedTokens = resolved.TokenSet()
	c.palette.cachedWritingDirection = ctx.WritingDirection
	c.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingL)), resolved.Density.Scale(16))
	c.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	c.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	c.cachedHistorySize = maxFloat(resolved.Density.Scale(20), 16)
	controlSize := c.control.measure(ctx, constraints).Size
	if p := c.palette.Label.Get(); p != "" {
		shaper := c.palette.newShaper(ctx.Runtime)
		if shaper != nil {
			style := resolved.TextStyle(theme.TextLabelM)
			shaper.SetContentScale(ctx.ContentScale)
			c.cachedLabelLayout = shaper.ShapeTruncated(strings.TrimSpace(p), style, constraints.MaxSize.W)
			c.cachedLabelStyle = style
		}
	}
	menuSize := c.menu.measure(ctx, constraints).Size
	w := menuSize.W
	h := menuSize.H
	if c.cachedLabelLayout != nil {
		w = maxFloat(w, c.cachedLabelLayout.Bounds.Width()+c.cachedPadX*2)
		h += c.cachedLabelLayout.Bounds.Height() + c.cachedGap
	}
	if len(c.palette.History) > 0 {
		h += c.cachedHistorySize + c.cachedGap
	}
	if c.palette.ShowBottomBar.Get() && controlSize.H > 0 {
		h += controlSize.H + c.cachedGap
	}
	size := constraints.Constrain(gfx.Size{W: w, H: h})
	c.palette.Layout.MeasuredSize = size
	c.palette.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return c.palette.Layout.MeasuredResult
}

func (c *popupPaletteComposition) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if c == nil || c.palette == nil {
		return
	}
	c.sync()
	c.cachedMenuBounds = bounds
	top := bounds.Min.Y + c.cachedPadY
	if c.cachedLabelLayout != nil {
		top += c.cachedLabelLayout.Bounds.Height() + c.cachedGap
	}
	bottomReserve := float32(0)
	if len(c.palette.History) > 0 {
		bottomReserve = c.cachedHistorySize + c.cachedGap
	}
	c.cachedControlBounds = gfx.Rect{}
	if c.palette.ShowBottomBar.Get() {
		controlSize := c.control.Layout.MeasuredSize
		if controlSize.H > 0 {
			controlBounds := gfx.RectFromXYWH(bounds.Min.X+c.cachedPadX, bounds.Max.Y-c.cachedPadY-controlSize.H, bounds.Width()-2*c.cachedPadX, controlSize.H)
			c.cachedControlBounds = controlBounds
			bottomReserve += controlSize.H + c.cachedGap
		}
	}
	menuBounds := gfx.RectFromXYWH(bounds.Min.X, top, bounds.Width(), maxFloat(0, bounds.Height()-top+bounds.Min.Y-bottomReserve-c.cachedPadY))
	c.menu.arrange(ctx, menuBounds)
	if !c.cachedControlBounds.IsEmpty() {
		c.control.arrange(c.cachedControlBounds)
	}
	c.cachedMenuBounds = menuBounds
	c.palette.cachedSurfaceBounds = menuBounds
	c.palette.cachedAnchorBounds = gfx.RectFromXYWH(bounds.Min.X+(bounds.Width()-bounds.Width()*0.08)*0.5, bounds.Min.Y, bounds.Width()*0.08, maxFloat(8, c.cachedPadY*0.8))

	c.palette.cachedToolItemBounds = c.palette.cachedToolItemBounds[:0]
	for _, btn := range c.toolButtons {
		if btn == nil || btn.Base() == nil || btn.Base().LayoutRole() == nil {
			continue
		}
		c.palette.cachedToolItemBounds = append(c.palette.cachedToolItemBounds, btn.Base().LayoutRole().ArrangedBounds)
	}

	c.palette.cachedHistoryBounds = c.palette.cachedHistoryBounds[:0]
	if len(c.palette.History) > 0 {
		sz := maxFloat(c.cachedHistorySize, 14)
		gap := maxFloat(4, c.cachedGap*0.6)
		totalW := float32(len(c.palette.History))*sz + float32(max(0, len(c.palette.History)-1))*gap
		startX := bounds.Min.X + (bounds.Width()-totalW)*0.5
		y := bounds.Max.Y - c.cachedPadY - sz
		for i := range c.palette.History {
			rect := gfx.RectFromXYWH(startX+float32(i)*(sz+gap), y, sz, sz)
			c.palette.cachedHistoryBounds = append(c.palette.cachedHistoryBounds, rect)
		}
	}

	if c.control != nil && c.control.Base() != nil && c.control.Base().LayoutRole() != nil {
		c.palette.cachedMirrorBounds = gfx.Rect{}
		c.palette.cachedCanvasBounds = gfx.Rect{}
		c.palette.cachedClearBounds = gfx.Rect{}
		c.palette.cachedToggleBounds = gfx.Rect{}
		if len(c.control.cachedActionBounds) > 0 {
			c.palette.cachedMirrorBounds = c.control.cachedActionBounds[0]
		}
		if len(c.control.cachedActionBounds) > 1 {
			c.palette.cachedCanvasBounds = c.control.cachedActionBounds[1]
		}
		if len(c.control.cachedActionBounds) > 2 {
			c.palette.cachedClearBounds = c.control.cachedActionBounds[2]
		}
		if len(c.control.cachedActionBounds) > 3 {
			c.palette.cachedToggleBounds = c.control.cachedActionBounds[3]
		}
	}

	if len(c.palette.Tools) > 0 && len(c.palette.cachedToolItemBounds) > 0 {
		c.palette.cachedSliderTrack = gfx.RectFromXYWH(bounds.Min.X+c.cachedPadX, bounds.Max.Y-c.cachedPadY-c.cachedHistorySize*0.5, bounds.Width()-2*c.cachedPadX, maxFloat(8, c.cachedHistorySize*0.28))
		thumbW := maxFloat(12, c.cachedHistorySize*0.9)
		thumbX := c.palette.cachedSliderTrack.Min.X + float32(c.palette.Zoom.Get())*0.25*c.palette.cachedSliderTrack.Width()
		c.palette.cachedSliderThumb = gfx.RectFromXYWH(thumbX, c.palette.cachedSliderTrack.Min.Y-(thumbW-c.palette.cachedSliderTrack.Height())*0.5, thumbW, thumbW)
		c.palette.cachedZoomBounds = c.palette.cachedSliderTrack
	}
}

func (c *popupPaletteComposition) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if c == nil || c.palette == nil {
		return nil
	}
	cmds := c.menu.buildCommands(c.cachedMenuBounds, runtime, contentScale)
	tokens := c.palette.cachedTokens
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, c.palette.Base().ID()); store != nil {
			tokens = store.Get().Tokens
		}
	}
	labelLayout := c.cachedLabelLayout
	if labelLayout == nil && strings.TrimSpace(c.palette.Label.Get()) != "" {
		if shaper := c.palette.newShaper(runtime); shaper != nil {
			shaper.SetContentScale(contentScale)
			labelStyle := c.cachedLabelStyle
			if labelStyle == (text.TextStyle{}) {
				labelStyle = text.DefaultStyle()
			}
			labelLayout = shaper.ShapeTruncated(strings.TrimSpace(c.palette.Label.Get()), labelStyle, bounds.Width())
		}
	}
	if labelLayout != nil {
		labelRect := gfx.RectFromXYWH(bounds.Min.X+c.cachedPadX, bounds.Min.Y+c.cachedPadY, labelLayout.Bounds.Width(), labelLayout.Bounds.Height())
		cmds = append(cmds, primitive.TextLayoutCommands(labelLayout, labelRect, gfx.SolidBrush(tokens.Color.OnSurfaceVariant))...)
	}
	if !c.cachedControlBounds.IsEmpty() && c.palette.ShowBottomBar.Get() {
		if controlCmds := c.control.buildCommands(c.cachedControlBounds, runtime); len(controlCmds) > 0 {
			cmds = append(cmds, controlCmds...)
		}
	}
	for i, rect := range c.palette.cachedHistoryBounds {
		if rect.IsEmpty() || i >= len(c.palette.History) {
			continue
		}
		cmds = append(cmds, gfx.FillPath{Path: gfx.CirclePath(centerOfRect(rect), rect.Width()*0.5), Brush: gfx.SolidBrush(c.palette.History[i])})
	}
	return cmds
}

func (c *popupPaletteComposition) hitTest(pt gfx.Point) facet.HitResult {
	if c == nil || c.palette == nil || (c.cachedMenuBounds.IsEmpty() && c.cachedControlBounds.IsEmpty()) {
		return facet.HitResult{}
	}
	for _, rect := range c.palette.cachedToolItemBounds {
		if rect.Contains(pt) {
			return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDToolItems, Cursor: facet.CursorPointer}
		}
	}
	if !c.cachedControlBounds.IsEmpty() && c.cachedControlBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDToolGroup, Cursor: facet.CursorPointer}
	}
	for _, rect := range c.palette.cachedHistoryBounds {
		if rect.Contains(pt) {
			return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDToolGroup, Cursor: facet.CursorPointer}
		}
	}
	if c.cachedMenuBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDSurface, Cursor: facet.CursorPointer}
	}
	return facet.HitResult{Hit: true, MarkID: popupPaletteMarkIDRoot, Cursor: facet.CursorPointer}
}

func (c *popupPaletteComposition) onPointer(e facet.PointerEvent) bool {
	if c == nil || c.palette == nil {
		return false
	}
	for i, rect := range c.palette.cachedToolItemBounds {
		if rect.Contains(e.Position) && i < len(c.toolButtons) && c.toolButtons[i] != nil {
			return c.toolButtons[i].Base().InputRole().OnPointer(e)
		}
	}
	if !c.cachedControlBounds.IsEmpty() && c.cachedControlBounds.Contains(e.Position) {
		return c.control.onPointer(e)
	}
	if !c.palette.cachedZoomBounds.IsEmpty() && (c.palette.cachedZoomBounds.Contains(e.Position) || c.palette.cachedSliderThumb.Contains(e.Position)) {
		switch e.Kind {
		case platform.PointerPress, platform.PointerMove, platform.PointerRelease:
			c.palette.updateZoomFromPoint(e.Position)
			if e.Kind == platform.PointerRelease {
				c.palette.Activated.Emit(fmt.Sprintf("zoom:%.0f", c.palette.Zoom.Get()*100))
			}
			return true
		}
	}
	return c.menu.onPointer(e)
}

func (c *popupPaletteComposition) onKey(e facet.KeyEvent) bool {
	if c == nil || c.palette == nil {
		return false
	}
	return c.menu.onKey(e)
}

func (c *popupPaletteComposition) onDismiss(e facet.DismissEvent) bool {
	if c == nil || c.palette == nil {
		return false
	}
	return c.menu.onDismiss(e)
}

func (c *popupPaletteComposition) exportAnchors(bounds gfx.Rect) layout.AnchorSet {
	if c == nil || c.palette == nil || bounds.IsEmpty() {
		return nil
	}
	out := c.palette.Core.DefaultAnchors(bounds, layout.AnchorExportContext{})
	if out == nil {
		out = make(layout.AnchorSet)
	}
	out["content_anchor"] = centerOfRect(c.cachedMenuBounds)
	out["baseline"] = centerOfRect(c.cachedMenuBounds)
	return out
}

func paletteSelectionColor(p *PopupPalette) gfx.Color {
	if p == nil {
		return gfx.Color{}
	}
	if p.SelectedIndex.Get() >= 0 && p.SelectedIndex.Get() < len(p.Tools) {
		if color := p.Tools[p.SelectedIndex.Get()].Color; color.A > 0 {
			return color
		}
	}
	for _, tool := range p.Tools {
		if tool.Color.A > 0 {
			return tool.Color
		}
	}
	if len(p.History) > 0 {
		return p.History[0]
	}
	return gfx.ColorFromRGBA8(255, 255, 255, 255)
}
