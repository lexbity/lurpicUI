package input

import (
	"math"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

const (
	colorPickerMarkIDRoot      facet.MarkID = 1
	colorPickerMarkIDWheel     facet.MarkID = 2
	colorPickerMarkIDTriangle  facet.MarkID = 3
	colorPickerMarkIDHandle    facet.MarkID = 4
	colorPickerMarkIDFocusRing facet.MarkID = 5
)

type colorPickerRegion uint8

const (
	colorPickerRegionNone colorPickerRegion = iota
	colorPickerRegionWheel
	colorPickerRegionTriangle
	colorPickerRegionHandle
)

// ColorPicker implements the input.color_picker standard mark.
type ColorPicker struct {
	marks.Core

	ColorChanged signal.Signal[gfx.Color]

	Label         marks.Binding[string]
	SelectedColor gfx.Color
	Hue           float64
	Saturation    float32
	Value         float32
	Alpha         float32
	Disabled      marks.Binding[bool]

	hoveredRegion    colorPickerRegion
	pressedRegion    colorPickerRegion
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool

	cachedTokens         theme.Tokens
	cachedRecipe         shared.ColorPickerSlots
	cachedBounds         gfx.Rect
	cachedWheelBounds    gfx.Rect
	cachedTriangleBounds gfx.Rect
	cachedHandleBounds   gfx.Rect
	cachedFocusBounds    gfx.Rect
	cachedCenter         gfx.Point
	cachedOuterRadius    float32
	cachedInnerRadius    float32
	cachedTriangleRadius float32
	cachedTriangleVerts  [3]gfx.Point
}

var _ facet.FacetImpl = (*ColorPicker)(nil)
var _ marks.Mark = (*ColorPicker)(nil)
var _ layout.AnchorExporter = (*ColorPicker)(nil)

// NewColorPicker constructs a color picker with canonical defaults.
func NewColorPicker(label string) *ColorPicker {
	p := &ColorPicker{
		Label:            marks.Const(strings.TrimSpace(label)),
		Disabled:         marks.Const(false),
		Hue:              0,
		Saturation:       1,
		Value:            1,
		Alpha:            1,
		SelectedColor:    hsvToColor(0, 1, 1, 1),
		focusFromPointer: false,
	}
	p.Core.Facet = facet.NewFacet()

	p.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	p.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsRadial,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := p.measureIntrinsic(ctx, constraints)
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
	p.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return p.measure(ctx, constraints)
	}
	p.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.Layout.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.Hit.OnHitTest = func(pt gfx.Point) facet.HitResult { return p.hitTest(pt) }
	p.Input.OnPointer = func(e facet.PointerEvent) bool { return p.onPointer(e) }
	p.Input.OnKey = func(e facet.KeyEvent) bool { return p.onKey(e) }
	p.Focus.Focusable = func() bool { return !p.Disabled.Get() }
	p.Focus.TabIndex = 0
	p.Focus.OnFocusGained = func() { p.onFocusGained() }
	p.Focus.OnFocusLost = func() { p.onFocusLost() }
	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds, ctx.Runtime)
	}
	p.RegisterRoles()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *ColorPicker) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

// Descriptor satisfies marks.Mark.
func (p *ColorPicker) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "input", TypeName: "color_picker"}
}

// AccessibilityRole reports the semantic role required by the mark spec.
func (p *ColorPicker) AccessibilityRole() string { return "colorpicker" }

// AccessibleName reports the semantic name source required by the mark spec.
func (p *ColorPicker) AccessibleName() string {
	if p == nil {
		return ""
	}
	if label := strings.TrimSpace(p.Label.Get()); label != "" {
		return label
	}
	return "Color picker"
}

// ExportAnchors publishes the color picker anchor set.
func (p *ColorPicker) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	return p.Core.DefaultAnchors(p.Layout.ArrangedBounds, ctx)
}

// CurrentColor returns the resolved selected color.
func (p *ColorPicker) CurrentColor() gfx.Color {
	if p == nil {
		return gfx.Color{}
	}
	return p.SelectedColor
}

// OnAttach wires the binding subscriptions.
func (p *ColorPicker) OnAttach(ctx facet.AttachContext) { p.Core.OnAttach() }

// OnActivate is unused.
func (p *ColorPicker) OnActivate() { p.Core.OnActivate() }

// OnDeactivate is unused.
func (p *ColorPicker) OnDeactivate() { p.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (p *ColorPicker) OnDetach() {
	p.Core.OnDetach()
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.ColorPickerSlots{}
	p.cachedBounds = gfx.Rect{}
	p.cachedWheelBounds = gfx.Rect{}
	p.cachedTriangleBounds = gfx.Rect{}
	p.cachedHandleBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	p.cachedCenter = gfx.Point{}
	p.cachedOuterRadius = 0
	p.cachedInnerRadius = 0
	p.cachedTriangleRadius = 0
	p.cachedTriangleVerts = [3]gfx.Point{}
}

func (p *ColorPicker) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, _ := p.resolveTheme(ctx)
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = recipe
	minSide := resolved.Density.Scale(220)
	if minSide < 180 {
		minSide = 180
	}
	maxSide := constraints.MaxSize.W
	if maxSide <= 0 || (constraints.MaxSize.H > 0 && constraints.MaxSize.H < maxSide) {
		maxSide = constraints.MaxSize.H
	}
	if maxSide <= 0 {
		maxSide = resolved.Density.Scale(280)
	}
	side := mathutil.Min(maxSide, minSide)
	if side <= 0 {
		side = minSide
	}
	size := constraints.Constrain(gfx.Size{W: side, H: side})
	if size.W <= 0 || size.H <= 0 {
		size = gfx.Size{W: side, H: side}
	}
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
	return p.Layout.MeasuredResult
}

func (p *ColorPicker) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return p.measure(ctx, constraints).Size
}

func (p *ColorPicker) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	p.cachedBounds = bounds
	p.cachedWheelBounds = gfx.Rect{}
	p.cachedTriangleBounds = gfx.Rect{}
	p.cachedHandleBounds = gfx.Rect{}
	p.cachedFocusBounds = gfx.Rect{}
	p.cachedCenter = colorPickerRectCenterPoint(bounds)
	p.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	size := mathutil.Min(bounds.Width(), bounds.Height())
	pad := mathutil.Max(10, size*0.06)
	p.cachedOuterRadius = mathutil.Max(0, size*0.5-pad)
	if p.cachedOuterRadius <= 0 {
		return
	}
	wheelThickness := mathutil.Max(size*0.19, 14)
	p.cachedInnerRadius = mathutil.Max(0, p.cachedOuterRadius-wheelThickness)
	p.cachedTriangleRadius = mathutil.Max(0, p.cachedInnerRadius*0.86)
	p.cachedWheelBounds = gfx.RectFromXYWH(p.cachedCenter.X-p.cachedOuterRadius, p.cachedCenter.Y-p.cachedOuterRadius, p.cachedOuterRadius*2, p.cachedOuterRadius*2)
	p.cachedFocusBounds = bounds.Inset(-mathutil.Max(2, size*0.05), -mathutil.Max(2, size*0.05))
	p.syncGeometry()
}

func (p *ColorPicker) syncGeometry() {
	if p == nil || p.cachedOuterRadius <= 0 {
		return
	}
	hueAngle := p.hueAngle()
	p.cachedTriangleVerts[0] = pointOnCircle(p.cachedCenter, p.cachedTriangleRadius, hueAngle)
	p.cachedTriangleVerts[1] = pointOnCircle(p.cachedCenter, p.cachedTriangleRadius, hueAngle+2*math.Pi/3)
	p.cachedTriangleVerts[2] = pointOnCircle(p.cachedCenter, p.cachedTriangleRadius, hueAngle-2*math.Pi/3)
	p.cachedTriangleBounds = boundsForPoints(p.cachedTriangleVerts[:])
	p.cachedHandleBounds = colorPickerCenteredRect(p.selectedPoint(), mathutil.Max(10, p.cachedOuterRadius*0.12))
}

func (p *ColorPicker) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if p == nil || bounds.IsEmpty() {
		return nil
	}
	style, recipe := p.resolveProjectionTheme(runtime)
	state := p.interactionState()
	tokens := style.Tokens
	root := recipe.Root.Resolve(state, tokens)
	wheelStyle := recipe.Wheel.Resolve(state, tokens)
	triangleStyle := recipe.Triangle.Resolve(state, tokens)
	handleStyle := recipe.Handle.Resolve(state, tokens)
	focusRing := recipe.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 128)
	if !colorPickerIsTransparentMaterial(root) {
		cmds = append(cmds, colorPickerMaterialCommands(gfx.RectPath(bounds), root)...)
	}

	if p.cachedOuterRadius > 0 {
		segments := 12
		step := 2 * math.Pi / float64(segments)
		start := -math.Pi / 2
		for i := 0; i < segments; i++ {
			a0 := start + step*float64(i)
			a1 := start + step*float64(i+1)
			path := colorPickerSectorPath(p.cachedCenter, float64(p.cachedInnerRadius), float64(p.cachedOuterRadius), a0, a1)
			midHue := wrapAngle((a0+a1)*0.5) / (2 * math.Pi)
			cmds = append(cmds, gfx.FillPath{
				Path:  path,
				Brush: gfx.SolidBrush(hsvToColor(midHue, 0.88, 0.97, 1)),
			})
		}
		if !colorPickerIsTransparentMaterial(wheelStyle) {
			cmds = append(cmds, colorPickerMaterialCommands(gfx.CirclePath(p.cachedCenter, p.cachedOuterRadius), wheelStyle)...)
			if p.cachedInnerRadius > 0 {
				cmds = append(cmds, colorPickerMaterialCommands(gfx.CirclePath(p.cachedCenter, p.cachedInnerRadius), wheelStyle)...)
			}
		}
	}

	if p.cachedTriangleRadius > 0 {
		baseHue := hsvToColor(p.Hue, 1, 1, p.Alpha)
		trianglePath := gfx.PolylinePath(p.cachedTriangleVerts[:], true)
		cmds = append(cmds, gfx.FillPath{Path: trianglePath, Brush: gfx.SolidBrush(baseHue)})
		cmds = append(cmds, gfx.FillPath{
			Path: trianglePath,
			Brush: gfx.LinearGradientBrush(
				p.cachedTriangleVerts[1],
				p.cachedTriangleVerts[0],
				[]gfx.GradientStop{
					{Offset: 0, Color: gfx.ColorFromRGBA8(255, 255, 255, 230)},
					{Offset: 1, Color: gfx.ColorFromRGBA8(255, 255, 255, 0)},
				},
			),
		})
		cmds = append(cmds, gfx.FillPath{
			Path: trianglePath,
			Brush: gfx.LinearGradientBrush(
				p.cachedTriangleVerts[2],
				p.cachedTriangleVerts[0],
				[]gfx.GradientStop{
					{Offset: 0, Color: gfx.ColorFromRGBA8(0, 0, 0, 230)},
					{Offset: 1, Color: gfx.ColorFromRGBA8(0, 0, 0, 0)},
				},
			),
		})
		if !colorPickerIsTransparentMaterial(triangleStyle) {
			cmds = append(cmds, colorPickerMaterialCommands(trianglePath, triangleStyle)...)
		}
	}

	if !p.cachedHandleBounds.IsEmpty() {
		handlePath := gfx.CirclePath(colorPickerRectCenterPoint(p.cachedHandleBounds), p.cachedHandleBounds.Width()*0.5)
		if !colorPickerIsTransparentMaterial(handleStyle) {
			cmds = append(cmds, colorPickerMaterialCommands(handlePath, handleStyle)...)
		} else {
			cmds = append(cmds, gfx.FillPath{Path: handlePath, Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 255, 255, 255))})
			cmds = append(cmds, gfx.StrokePath{
				Path:  handlePath,
				Brush: gfx.SolidBrush(hsvToColor(p.Hue, 1, 1, 1)),
				Stroke: gfx.StrokeStyle{
					Width:      mathutil.Max(1.5, p.cachedHandleBounds.Width()*0.12),
					Cap:        gfx.LineCapRound,
					Join:       gfx.LineJoinRound,
					MiterLimit: 10,
				},
			})
		}
	}

	if p.focusedVisible && !p.cachedFocusBounds.IsEmpty() && !colorPickerIsTransparentMaterial(focusRing) {
		cmds = append(cmds, colorPickerMaterialCommands(gfx.CirclePath(p.cachedCenter, p.cachedFocusBounds.Width()*0.5), focusRing)...)
	}

	return cmds
}

func (p *ColorPicker) hitTest(pt gfx.Point) facet.HitResult {
	if p == nil || p.cachedBounds.IsEmpty() || !p.cachedBounds.Contains(pt) {
		return facet.HitResult{}
	}
	if p.focusedVisible && !p.cachedFocusBounds.IsEmpty() && p.cachedFocusBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: colorPickerMarkIDFocusRing, Cursor: facet.CursorCrosshair}
	}
	if !p.cachedHandleBounds.IsEmpty() && p.cachedHandleBounds.Contains(pt) {
		return facet.HitResult{Hit: true, MarkID: colorPickerMarkIDHandle, Cursor: facet.CursorCrosshair}
	}
	if pointInTriangle(pt, p.cachedTriangleVerts[0], p.cachedTriangleVerts[1], p.cachedTriangleVerts[2]) {
		return facet.HitResult{Hit: true, MarkID: colorPickerMarkIDTriangle, Cursor: facet.CursorCrosshair}
	}
	if distance(p.cachedCenter, pt) >= p.cachedInnerRadius && distance(p.cachedCenter, pt) <= p.cachedOuterRadius {
		return facet.HitResult{Hit: true, MarkID: colorPickerMarkIDWheel, Cursor: facet.CursorCrosshair}
	}
	return facet.HitResult{Hit: true, MarkID: colorPickerMarkIDRoot, Cursor: facet.CursorCrosshair}
}

func (p *ColorPicker) onPointer(e facet.PointerEvent) bool {
	if p.Disabled.Get() {
		return false
	}
	region := p.regionAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		p.hoveredRegion = region
		if p.dragging {
			p.applyPointerRegion(region, e.Position, true)
		}
		p.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		p.focusFromPointer = true
		p.focusedVisible = false
		p.dragging = true
		p.pressedRegion = region
		if !p.applyPointerRegion(region, e.Position, true) {
			p.dragging = false
			p.pressedRegion = colorPickerRegionNone
			return false
		}
		p.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		dragging := p.dragging
		p.dragging = false
		p.pressedRegion = colorPickerRegionNone
		if region != colorPickerRegionNone {
			p.applyPointerRegion(region, e.Position, true)
		}
		p.invalidate(facet.DirtyProjection)
		return dragging || region != colorPickerRegionNone
	case platform.PointerLeave:
		p.hoveredRegion = colorPickerRegionNone
		if !p.dragging {
			p.focusFromPointer = false
		}
		p.invalidate(facet.DirtyProjection)
		return true
	default:
		return false
	}
}

func (p *ColorPicker) onKey(e facet.KeyEvent) bool {
	if p.Disabled.Get() {
		return false
	}
	if e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
		return false
	}
	switch e.Key {
	case platform.KeyEscape:
		return false
	case platform.KeyLeft:
		if e.Modifiers&platform.ModShift != 0 {
			p.setHSV(p.Hue, p.Saturation-0.05, p.Value, true)
		} else {
			p.setHSV(p.Hue-math.Pi/36, p.Saturation, p.Value, true)
		}
		return true
	case platform.KeyRight:
		if e.Modifiers&platform.ModShift != 0 {
			p.setHSV(p.Hue, p.Saturation+0.05, p.Value, true)
		} else {
			p.setHSV(p.Hue+math.Pi/36, p.Saturation, p.Value, true)
		}
		return true
	case platform.KeyUp:
		p.setHSV(p.Hue, p.Saturation, p.Value+0.05, true)
		return true
	case platform.KeyDown:
		p.setHSV(p.Hue, p.Saturation, p.Value-0.05, true)
		return true
	case platform.KeyPageUp:
		p.setHSV(p.Hue+math.Pi/12, p.Saturation, p.Value, true)
		return true
	case platform.KeyPageDown:
		p.setHSV(p.Hue-math.Pi/12, p.Saturation, p.Value, true)
		return true
	case platform.KeyHome:
		p.setHSV(p.Hue, 0, 1, true)
		return true
	case platform.KeyEnd:
		p.setHSV(p.Hue, p.Saturation, 0, true)
		return true
	default:
		return false
	}
}

func (p *ColorPicker) onFocusGained() {
	if p.Disabled.Get() {
		return
	}
	p.focusedVisible = !p.focusFromPointer
	p.invalidate(facet.DirtyProjection)
}

func (p *ColorPicker) onFocusLost() {
	p.focusedVisible = false
	p.focusFromPointer = false
	p.dragging = false
	p.pressedRegion = colorPickerRegionNone
	p.invalidate(facet.DirtyProjection)
}

func (p *ColorPicker) interactionState() theme.InteractionState {
	switch {
	case p.Disabled.Get():
		return theme.StateDisabled
	case p.pressedRegion != colorPickerRegionNone:
		return theme.StatePressed
	case p.hoveredRegion != colorPickerRegionNone:
		return theme.StateHover
	case p.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (p *ColorPicker) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.ColorPickerSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiinput.ResolveColorPickerRecipe(style, uiinput.ColorPickerStandard)
	return resolved, slots, true
}

func (p *ColorPicker) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.ColorPickerSlots) {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, p.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveColorPickerRecipe(style, uiinput.ColorPickerStandard)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: p.cachedTokens, Materials: nil, Depth: 0}, p.cachedRecipe
}

func (p *ColorPicker) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Facet.Invalidate(flags)
}

func (p *ColorPicker) regionAt(pt gfx.Point) colorPickerRegion {
	switch {
	case !p.cachedHandleBounds.IsEmpty() && p.cachedHandleBounds.Contains(pt):
		return colorPickerRegionHandle
	case pointInTriangle(pt, p.cachedTriangleVerts[0], p.cachedTriangleVerts[1], p.cachedTriangleVerts[2]):
		return colorPickerRegionTriangle
	case distance(p.cachedCenter, pt) >= p.cachedInnerRadius && distance(p.cachedCenter, pt) <= p.cachedOuterRadius:
		return colorPickerRegionWheel
	default:
		return colorPickerRegionNone
	}
}

func (p *ColorPicker) applyPointerRegion(region colorPickerRegion, pt gfx.Point, emit bool) bool {
	switch region {
	case colorPickerRegionWheel:
		angle := math.Atan2(float64(pt.Y-p.cachedCenter.Y), float64(pt.X-p.cachedCenter.X))
		p.setHSV(angle, p.Saturation, p.Value, emit)
		return true
	case colorPickerRegionTriangle:
		a, b, c := barycentric(pt, p.cachedTriangleVerts[0], p.cachedTriangleVerts[1], p.cachedTriangleVerts[2])
		a = clamp01Float(a)
		b = clamp01Float(b)
		c = clamp01Float(c)
		if sum := a + b + c; sum > 0 {
			a /= sum
			b /= sum
			c /= sum
		}
		value := clamp01Float(a + b)
		saturation := 0.0
		if value > 0 {
			saturation = float64(a) / float64(value)
		}
		p.setHSV(p.Hue, float32(saturation), value, emit)
		return true
	case colorPickerRegionHandle:
		// Treat the handle as part of the triangle selection to keep dragging intuitive.
		fallthrough
	case colorPickerRegionNone:
		return false
	default:
		return false
	}
}

// SetColor updates the selected color, synchronizes derived HSV state,
// and invalidates projection. Call this instead of assigning SelectedColor
// directly to ensure all internal state stays consistent.
func (p *ColorPicker) SetColor(color gfx.Color) {
	p.setColor(color, false)
}

func (p *ColorPicker) setColor(color gfx.Color, emit bool) {
	if p == nil {
		return
	}
	if p.SelectedColor == color {
		return
	}
	p.SelectedColor = color
	if r, g, b, a := color.ToRGBA8(); a > 0 {
		h, s, v := rgbToHSV(r, g, b)
		p.Hue = h
		p.Saturation = s
		p.Value = v
		p.Alpha = float32(a) / 255
	} else {
		p.Alpha = 0
	}
	p.syncGeometry()
	if emit {
		p.ColorChanged.Emit(p.SelectedColor)
	}
	p.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

func (p *ColorPicker) setHSV(hue float64, saturation, value float32, emit bool) {
	if p == nil {
		return
	}
	hue = wrapAngle(hue)
	saturation = clamp01Float(saturation)
	value = clamp01Float(value)
	alpha := p.Alpha
	if alpha <= 0 {
		alpha = 1
	}
	color := hsvToColor(hue, saturation, value, alpha)
	if p.Hue == hue && p.Saturation == saturation && p.Value == value && p.SelectedColor == color {
		return
	}
	p.Hue = hue
	p.Saturation = saturation
	p.Value = value
	p.SelectedColor = color
	p.syncGeometry()
	if emit {
		p.ColorChanged.Emit(color)
	}
	p.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

func (p *ColorPicker) selectedPoint() gfx.Point {
	if p == nil {
		return gfx.Point{}
	}
	if p.cachedTriangleRadius <= 0 {
		return p.cachedCenter
	}
	pureW := p.Saturation * p.Value
	whiteW := (1 - p.Saturation) * p.Value
	blackW := 1 - p.Value
	return weightedPoint(p.cachedTriangleVerts[0], p.cachedTriangleVerts[1], p.cachedTriangleVerts[2], pureW, whiteW, blackW)
}

func (p *ColorPicker) hueAngle() float64 {
	return p.Hue
}

func colorPickerSectorPath(center gfx.Point, innerRadius, outerRadius, startAngle, endAngle float64) gfx.Path {
	startOuter := pointOnCircle(center, float32(outerRadius), startAngle)
	endOuter := pointOnCircle(center, float32(outerRadius), endAngle)
	startInner := pointOnCircle(center, float32(innerRadius), endAngle)
	endInner := pointOnCircle(center, float32(innerRadius), startAngle)
	return gfx.NewPath().
		MoveTo(startOuter).
		LineTo(endOuter).
		LineTo(startInner).
		LineTo(endInner).
		Close().
		Build()
}

func colorPickerMaterialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	return theme.MaterialCommands(path, material)
}

func colorPickerIsTransparentMaterial(material theme.Material) bool {
	return theme.IsTransparentMaterial(material)
}

func pointOnCircle(center gfx.Point, radius float32, angle float64) gfx.Point {
	return gfx.Point{
		X: center.X + float32(math.Cos(angle))*radius,
		Y: center.Y + float32(math.Sin(angle))*radius,
	}
}

func weightedPoint(a, b, c gfx.Point, wa, wb, wc float32) gfx.Point {
	sum := wa + wb + wc
	if sum == 0 {
		return gfx.Point{}
	}
	return gfx.Point{
		X: (a.X*wa + b.X*wb + c.X*wc) / sum,
		Y: (a.Y*wa + b.Y*wb + c.Y*wc) / sum,
	}
}

func barycentric(p, a, b, c gfx.Point) (float32, float32, float32) {
	denom := (b.Y-c.Y)*(a.X-c.X) + (c.X-b.X)*(a.Y-c.Y)
	if denom == 0 {
		return 0, 0, 0
	}
	w1 := ((b.Y-c.Y)*(p.X-c.X) + (c.X-b.X)*(p.Y-c.Y)) / denom
	w2 := ((c.Y-a.Y)*(p.X-c.X) + (a.X-c.X)*(p.Y-c.Y)) / denom
	w3 := 1 - w1 - w2
	return w1, w2, w3
}

func pointInTriangle(p, a, b, c gfx.Point) bool {
	w1, w2, w3 := barycentric(p, a, b, c)
	const eps = 1e-4
	return w1 >= -eps && w2 >= -eps && w3 >= -eps
}

func trianglePointToHSV(point gfx.Point, a, b, c gfx.Point, hue float64) (float32, float32) {
	w1, w2, w3 := barycentric(point, a, b, c)
	if w1 < 0 {
		w1 = 0
	}
	if w2 < 0 {
		w2 = 0
	}
	if w3 < 0 {
		w3 = 0
	}
	sum := w1 + w2 + w3
	if sum > 0 {
		w1 /= sum
		w2 /= sum
		w3 /= sum
	}
	value := clamp01Float(w1 + w2)
	saturation := float32(0)
	if value > 0 {
		saturation = w1 / value
	}
	_ = hue
	return saturation, value
}

func boundsForPoints(pts []gfx.Point) gfx.Rect {
	if len(pts) == 0 {
		return gfx.Rect{}
	}
	minX, maxX := pts[0].X, pts[0].X
	minY, maxY := pts[0].Y, pts[0].Y
	for _, pt := range pts[1:] {
		if pt.X < minX {
			minX = pt.X
		}
		if pt.X > maxX {
			maxX = pt.X
		}
		if pt.Y < minY {
			minY = pt.Y
		}
		if pt.Y > maxY {
			maxY = pt.Y
		}
	}
	return gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
}

func colorPickerCenteredRect(center gfx.Point, size float32) gfx.Rect {
	if size <= 0 {
		return gfx.Rect{}
	}
	half := size * 0.5
	return gfx.RectFromXYWH(center.X-half, center.Y-half, size, size)
}

func colorPickerRectCenterPoint(r gfx.Rect) gfx.Point {
	return gfx.Point{X: r.Min.X + r.Width()*0.5, Y: r.Min.Y + r.Height()*0.5}
}

func colorToHSV(color gfx.Color) (float64, float32, float32) {
	r, g, b, a := color.ToRGBA8()
	if a == 0 {
		return 0, 0, 0
	}
	return rgbToHSV(r, g, b)
}

func rgbToHSV(r, g, b uint8) (float64, float32, float32) {
	rr := float64(r) / 255
	gg := float64(g) / 255
	bb := float64(b) / 255
	maxV := math.Max(rr, math.Max(gg, bb))
	minV := math.Min(rr, math.Min(gg, bb))
	delta := maxV - minV
	var hue float64
	switch {
	case delta == 0:
		hue = 0
	case maxV == rr:
		hue = math.Mod((gg-bb)/delta, 6)
	case maxV == gg:
		hue = ((bb - rr) / delta) + 2
	default:
		hue = ((rr - gg) / delta) + 4
	}
	hue *= math.Pi / 3
	if hue < 0 {
		hue += 2 * math.Pi
	}
	saturation := float32(0)
	if maxV > 0 {
		saturation = float32(delta / maxV)
	}
	return hue, saturation, float32(maxV)
}

func hsvToColor(hue float64, saturation, value, alpha float32) gfx.Color {
	hue = wrapAngle(hue)
	saturation = clamp01Float(saturation)
	value = clamp01Float(value)
	alpha = clamp01Float(alpha)
	if alpha <= 0 {
		return gfx.Color{}
	}
	h := hue / (2 * math.Pi)
	if h < 0 {
		h += 1
	}
	h *= 6
	c := float64(value) * float64(saturation)
	x := c * (1 - math.Abs(math.Mod(h, 2)-1))
	m := float64(value) - c
	var rr, gg, bb float64
	switch {
	case h < 1:
		rr, gg, bb = c, x, 0
	case h < 2:
		rr, gg, bb = x, c, 0
	case h < 3:
		rr, gg, bb = 0, c, x
	case h < 4:
		rr, gg, bb = 0, x, c
	case h < 5:
		rr, gg, bb = x, 0, c
	default:
		rr, gg, bb = c, 0, x
	}
	return gfx.ColorFromRGBA8(
		clampByte((rr+m)*255),
		clampByte((gg+m)*255),
		clampByte((bb+m)*255),
		clampByte(float64(alpha)*255),
	)
}

func wrapAngle(angle float64) float64 {
	if math.IsNaN(angle) || math.IsInf(angle, 0) {
		return 0
	}
	angle = math.Mod(angle, 2*math.Pi)
	if angle < 0 {
		angle += 2 * math.Pi
	}
	return angle
}

func clampByte(v float64) uint8 {
	if v < 0 {
		v = 0
	}
	if v > 255 {
		v = 255
	}
	return uint8(v + 0.5)
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

func distance(a, b gfx.Point) float32 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return float32(math.Hypot(float64(dx), float64(dy)))
}
