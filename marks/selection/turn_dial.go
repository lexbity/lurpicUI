package selection

import (
	"fmt"
	"math"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

const (
	turnDialMarkIDRoot       facet.MarkID = 1
	turnDialMarkIDTrack      facet.MarkID = 2
	turnDialMarkIDKnob       facet.MarkID = 3
	turnDialMarkIDDot        facet.MarkID = 4
	turnDialMarkIDLabel      facet.MarkID = 5
	turnDialMarkIDValueLabel facet.MarkID = 6
)

// TurnDial implements a custom skeuomorphic selection.turn_dial mark.
// It acts as a radial rotary knob slider and a mechanical click button.
type TurnDial struct {
	marks.Core

	Value     *store.ValueStore[float64]
	Activated signal.Signal[signal.Unit]

	Label    marks.Binding[string]
	Disabled marks.Binding[bool]

	Min       float64
	Max       float64
	Step      float64
	Precision int
	DialSize  float32

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool

	cachedLayout           *text.TextLayout
	cachedValueLayout      *text.TextLayout
	cachedTokens           theme.Tokens
	cachedRootBounds       gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedDialBounds       gfx.Rect
	cachedValueLabelBounds gfx.Rect

	cachedLabelHeight      float32
	cachedValueHeight      float32
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*TurnDial)(nil)
var _ marks.Mark = (*TurnDial)(nil)

// NewTurnDial constructs a selection.turn_dial mark with defaults.
func NewTurnDial(label string, min, max, step float64) *TurnDial {
	td := &TurnDial{
		Label:     marks.Const(label),
		Disabled:  marks.Const(false),
		Value:     store.NewValueStore[float64](min),
		Min:       min,
		Max:       max,
		Step:      step,
		Precision: 1,
		DialSize:  72,
	}
	td.Facet = facet.NewFacet()
	td.AddBinding(td.Label)
	td.AddBinding(td.Disabled)

	td.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: turnDialGroupPolicy{},
	}
	td.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := td.measureIntrinsic(ctx, constraints)
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
	td.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return td.measure(ctx, constraints)
	}
	td.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		td.Layout.ArrangedBounds = bounds
		td.arrange(ctx, bounds)
	}
	td.Hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		return td.hitTest(p)
	}
	td.Input.OnPointer = func(e facet.PointerEvent) bool {
		return td.onPointer(e)
	}
	td.Input.OnKey = func(e facet.KeyEvent) bool {
		return td.onKey(e)
	}
	td.Focus.Focusable = func() bool {
		return !td.Disabled.Get()
	}
	td.Focus.OnFocusGained = func() {
		td.focusedVisible = !td.focusFromPointer
		td.invalidate(facet.DirtyProjection)
	}
	td.Focus.OnFocusLost = func() {
		td.focusedVisible = false
		td.focusFromPointer = false
		td.invalidate(facet.DirtyProjection)
	}
	td.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return td.buildCommands(td.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	td.RegisterRoles()
	return td
}

// Base satisfies facet.FacetImpl.
func (td *TurnDial) Base() *facet.Facet {
	td.BindImpl(td)
	return &td.Facet
}

// Descriptor satisfies marks.Mark.
func (td *TurnDial) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "selection", TypeName: "turn_dial"}
}

func (td *TurnDial) AccessibilityRole() string {
	return "slider"
}

func (td *TurnDial) AccessibleName() string {
	if td == nil {
		return ""
	}
	return td.Label.Get()
}

func (td *TurnDial) clampValue(v float64) float64 {
	minV, maxV := td.normalizedRange()
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	step := td.stepValue()
	if step > 0 {
		return minV + math.Round((v-minV)/step)*step
	}
	return v
}

func (td *TurnDial) normalizedRange() (float64, float64) {
	minV, maxV := td.Min, td.Max
	if minV > maxV {
		minV, maxV = maxV, minV
	}
	return minV, maxV
}

func (td *TurnDial) stepValue() float64 {
	step := td.Step
	if step <= 0 {
		minV, maxV := td.normalizedRange()
		step = (maxV - minV) * 0.01
	}
	return step
}

func (td *TurnDial) invalidate(flags facet.DirtyFlags) {
	if td == nil {
		return
	}
	td.Base().Invalidate(flags)
}

func (td *TurnDial) OnAttach(ctx facet.AttachContext) {
	td.Core.OnAttach()
	if td.Value == nil {
		td.Value = store.NewValueStore[float64](td.Min)
	}
	facet.Store(facet.Subscribe(td), &td.Value.OnChange, td.Value.Version, func(signal.Change[float64]) {
		td.invalidate(facet.DirtyProjection)
	})
}

func (td *TurnDial) OnActivate()   { td.Core.OnActivate() }
func (td *TurnDial) OnDeactivate() { td.Core.OnDeactivate() }

func (td *TurnDial) OnDetach() {
	td.Core.OnDetach()
	td.cachedLayout = nil
	td.cachedValueLayout = nil
	td.cachedTokens = theme.Tokens{}
	td.cachedRootBounds = gfx.Rect{}
	td.cachedLabelBounds = gfx.Rect{}
	td.cachedDialBounds = gfx.Rect{}
	td.cachedValueLabelBounds = gfx.Rect{}
	td.cachedLabelHeight = 0
	td.cachedValueHeight = 0
}

func (td *TurnDial) newShaper(runtime any) *text.Shaper {
	registry := td.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (td *TurnDial) fontRegistry(runtime any) *text.FontRegistry {
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

func (td *TurnDial) resolveLayouts(ctx facet.MeasureContext, constraints facet.Constraints, resolved theme.ResolvedContext) (*text.TextLayout, *text.TextLayout) {
	labelStyle := resolved.TextStyle(theme.TextLabelM)
	valueStyle := resolved.TextStyle(theme.TextBodyS)

	shaper := td.newShaper(ctx.Runtime)
	if shaper == nil {
		var labelLayout *text.TextLayout
		if td.Label.Get() != "" {
			labelLayout = dummyLayout(td.Label.Get(), labelStyle)
		}
		var valueLayout *text.TextLayout
		valStr := td.formatValue(td.currentValue())
		if valStr != "" {
			valueLayout = dummyLayout(valStr, valueStyle)
		}
		return labelLayout, valueLayout
	}
	shaper.SetContentScale(ctx.ContentScale)

	var labelLayout *text.TextLayout
	if td.Label.Get() != "" {
		labelLayout = shaper.ShapeTruncated(td.Label.Get(), labelStyle, constraints.MaxSize.W)
	}

	var valueLayout *text.TextLayout
	valStr := td.formatValue(td.currentValue())
	if valStr != "" {
		valueLayout = shaper.ShapeTruncated(valStr, valueStyle, constraints.MaxSize.W)
	}

	return labelLayout, valueLayout
}

func dummyLayout(content string, style text.TextStyle) *text.TextLayout {
	if content == "" {
		return nil
	}
	w := float32(len(content)) * (style.Size * 0.6)
	h := style.Size * 1.2
	return &text.TextLayout{
		Bounds:     text.RectFromXYWH(0, 0, w, h),
		LineHeight: h,
	}
}

func (td *TurnDial) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}

	var maxW, totalH float32
	maxW = td.DialSize
	totalH = td.DialSize

	labelLayout, valueLayout := td.resolveLayouts(ctx, constraints, resolved)
	if labelLayout != nil {
		totalH += labelLayout.Bounds.Height() + 6
		if labelLayout.Bounds.Width() > maxW {
			maxW = labelLayout.Bounds.Width()
		}
	}
	if valueLayout != nil {
		totalH += valueLayout.Bounds.Height() + 6
		if valueLayout.Bounds.Width() > maxW {
			maxW = valueLayout.Bounds.Width()
		}
	}

	return gfx.Size{W: maxW, H: totalH}
}

func (td *TurnDial) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	tokens := resolved.TokenSet()
	td.cachedTokens = tokens
	td.cachedWritingDirection = ctx.WritingDirection

	labelLayout, valueLayout := td.resolveLayouts(ctx, constraints, resolved)
	td.cachedLayout = labelLayout
	if labelLayout != nil {
		td.cachedLabelHeight = labelLayout.Bounds.Height()
	} else {
		td.cachedLabelHeight = 0
	}
	td.cachedValueLayout = valueLayout
	if valueLayout != nil {
		td.cachedValueHeight = valueLayout.Bounds.Height()
	} else {
		td.cachedValueHeight = 0
	}

	size := td.measureIntrinsic(ctx, constraints)
	td.Layout.MeasuredSize = size
	td.Layout.MeasuredResult = facet.MeasureResult{
		Size:        size,
		Intrinsic:   facet.IntrinsicSize{Min: size, Preferred: size, Max: size},
		Constraints: constraints,
	}
	return td.Layout.MeasuredResult
}

func (td *TurnDial) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	td.cachedRootBounds = bounds
	td.cachedLabelBounds = gfx.Rect{}
	td.cachedDialBounds = gfx.Rect{}
	td.cachedValueLabelBounds = gfx.Rect{}

	if bounds.IsEmpty() {
		return
	}

	currentY := bounds.Min.Y

	if td.cachedLayout != nil {
		w := td.cachedLayout.Bounds.Width()
		td.cachedLabelBounds = gfx.RectFromXYWH(
			bounds.Min.X+(bounds.Width()-w)*0.5,
			currentY,
			w,
			td.cachedLabelHeight,
		)
		currentY += td.cachedLabelHeight + 6
	}

	td.cachedDialBounds = gfx.RectFromXYWH(
		bounds.Min.X+(bounds.Width()-td.DialSize)*0.5,
		currentY,
		td.DialSize,
		td.DialSize,
	)
	currentY += td.DialSize + 6

	if td.cachedValueLayout != nil {
		w := td.cachedValueLayout.Bounds.Width()
		td.cachedValueLabelBounds = gfx.RectFromXYWH(
			bounds.Min.X+(bounds.Width()-w)*0.5,
			currentY,
			w,
			td.cachedValueHeight,
		)
	}
}

func (td *TurnDial) formatValue(v float64) string {
	prec := td.Precision
	if prec < 0 {
		prec = 1
	}
	return fmt.Sprintf("%.*f", prec, v)
}

func (td *TurnDial) currentValue() float64 {
	if td == nil || td.Value == nil {
		return td.Min
	}
	return td.Value.Get()
}

func (td *TurnDial) interactionState() theme.InteractionState {
	switch {
	case td.Disabled.Get():
		return theme.StateDisabled
	case td.pressed || td.dragging:
		return theme.StatePressed
	case td.hovered:
		return theme.StateHover
	case td.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (td *TurnDial) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if td == nil || bounds.IsEmpty() {
		return nil
	}

	state := td.interactionState()
	tokens := td.cachedTokens
	if tokens.Color.Primary.A == 0 {
		tokens = theme.DefaultTokens()
	}

	slots := defaultTurnDialSlots(tokens)
	track := slots.Track.Resolve(state, tokens)
	knob := slots.Knob.Resolve(state, tokens)
	label := slots.Label.Resolve(state, tokens)
	valueLabel := slots.ValueLabel.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 16)
	isDark := tokens.Color.Background.R < 0.5

	// Draw outer track
	if !td.cachedDialBounds.IsEmpty() {
		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		R := td.cachedDialBounds.Width() * 0.5

		if isDark {
			// Draw inactive track arc (whole 270 deg range)
			inactiveTrackPath := arcPath(gfx.Point{X: centerX, Y: centerY}, R-2, 135.0*math.Pi/180.0, 405.0*math.Pi/180.0)
			cmds = append(cmds, gfx.StrokePath{
				Path:  inactiveTrackPath,
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(26, 32, 48, 255)),
				Stroke: gfx.StrokeStyle{
					Width: 3.5,
					Cap:   gfx.LineCapRound,
				},
			})

			// Map current value to rotation angle
			minV, maxV := td.normalizedRange()
			frac := 0.0
			if maxV > minV {
				frac = (td.currentValue() - minV) / (maxV - minV)
			}
			angleDeg := 135.0 + frac*270.0
			angleRad := angleDeg * math.Pi / 180.0

			// Draw active sweeping track arc (from 135 deg to angleRad)
			if angleRad > 135.0*math.Pi/180.0 {
				activeTrackPath := arcPath(gfx.Point{X: centerX, Y: centerY}, R-2, 135.0*math.Pi/180.0, angleRad)

				// Sweeping gradient active track
				activeStops := []gfx.GradientStop{
					{Offset: 0.0, Color: gfx.ColorFromRGBA8(99, 102, 241, 255)},
					{Offset: 0.5, Color: gfx.ColorFromRGBA8(6, 182, 212, 255)},
					{Offset: 1.0, Color: gfx.ColorFromRGBA8(0, 245, 255, 255)},
				}

				glowStops := make([]gfx.GradientStop, len(activeStops))
				for i, st := range activeStops {
					glowStops[i] = gfx.GradientStop{
						Offset: st.Offset,
						Color:  st.Color.WithAlpha(st.Color.A * 0.35),
					}
				}

				startPt := gfx.Point{X: centerX - R, Y: centerY + R}
				endPt := gfx.Point{X: centerX + R, Y: centerY - R}

				glowBrush := gfx.LinearGradientBrush(startPt, endPt, glowStops)
				coreBrush := gfx.LinearGradientBrush(startPt, endPt, activeStops)

				// 1. Glowing neon active arc under-stroke
				cmds = append(cmds, gfx.StrokePath{
					Path:  activeTrackPath,
					Brush: glowBrush,
					Stroke: gfx.StrokeStyle{
						Width: 8.0,
						Cap:   gfx.LineCapRound,
					},
				})

				// 2. High-intensity neon active arc core
				cmds = append(cmds, gfx.StrokePath{
					Path:  activeTrackPath,
					Brush: coreBrush,
					Stroke: gfx.StrokeStyle{
						Width: 3.5,
						Cap:   gfx.LineCapRound,
					},
				})
			}
		} else {
			if !theme.IsTransparentMaterial(track) {
				cmds = append(cmds, theme.MaterialCommands(gfx.CirclePath(gfx.Point{X: centerX, Y: centerY}, R), track)...)
			}
		}
	}

	// Draw rotating protruding knob
	if !theme.IsTransparentMaterial(knob) && !td.cachedDialBounds.IsEmpty() {
		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		R := td.cachedDialBounds.Width() * 0.5
		knobRadius := R - 6

		// Map current value to rotation angle
		minV, maxV := td.normalizedRange()
		frac := 0.0
		if maxV > minV {
			frac = (td.currentValue() - minV) / (maxV - minV)
		}
		angleDeg := 135.0 + frac*270.0
		angleRad := angleDeg * math.Pi / 180.0

		if len(knob.Fills) > 0 && knob.Fills[0].Type == theme.FillGradient {
			grad := &knob.Fills[0].Gradient
			cosA := float32(math.Cos(angleRad))
			sinA := float32(math.Sin(angleRad))
			grad.Start = gfx.Point{
				X: 0.5 - 0.5*cosA,
				Y: 0.5 - 0.5*sinA,
			}
			grad.End = gfx.Point{
				X: 0.5 + 0.5*cosA,
				Y: 0.5 + 0.5*sinA,
			}
		}

		cmds = append(cmds, theme.MaterialCommands(gfx.CirclePath(gfx.Point{X: centerX, Y: centerY}, knobRadius), knob)...)

		cosA := float32(math.Cos(angleRad))
		sinA := float32(math.Sin(angleRad))

		if isDark {
			reflectionPath := arcPath(gfx.Point{X: centerX, Y: centerY}, knobRadius-1.0, -45.0*math.Pi/180.0, 45.0*math.Pi/180.0)
			cmds = append(cmds, gfx.StrokePath{
				Path:  reflectionPath,
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 245, 255, 255).WithAlpha(0.4)),
				Stroke: gfx.StrokeStyle{
					Width: 1.5,
					Cap:   gfx.LineCapRound,
				},
			})

			dotDistance := knobRadius * 0.72
			dotX := centerX + cosA*dotDistance
			dotY := centerY + sinA*dotDistance

			cmds = append(cmds, gfx.FillPath{
				Path:  gfx.CirclePath(gfx.Point{X: dotX, Y: dotY}, 5.5),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(0, 245, 255, 255).WithAlpha(0.55)),
			})

			cmds = append(cmds, gfx.FillPath{
				Path:  gfx.CirclePath(gfx.Point{X: dotX, Y: dotY}, 2.2),
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 255, 255, 255)),
			})
		} else {
			notchStart := gfx.Point{
				X: centerX + cosA*knobRadius*0.2,
				Y: centerY + sinA*knobRadius*0.2,
			}
			notchEnd := gfx.Point{
				X: centerX + cosA*knobRadius*0.8,
				Y: centerY + sinA*knobRadius*0.8,
			}
			notchLine := gfx.LinePath(notchStart, notchEnd)

			cmds = append(cmds, gfx.StrokePath{
				Path:  notchLine,
				Brush: gfx.SolidBrush(gfx.ColorFromRGBA8(255, 255, 255, 255)),
				Stroke: gfx.StrokeStyle{
					Width: 2.5,
					Cap:   gfx.LineCapRound,
				},
			})
		}
	}

	// Draw labels
	if td.cachedLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(td.cachedLayout, td.cachedLabelBounds, gfx.SolidBrush(theme.MaterialColor(label)))...)
	}
	if td.cachedValueLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(td.cachedValueLayout, td.cachedValueLabelBounds, gfx.SolidBrush(theme.MaterialColor(valueLabel)))...)
	}

	return cmds
}

func (td *TurnDial) hitTest(p gfx.Point) facet.HitResult {
	if td == nil || td.Layout.ArrangedBounds.IsEmpty() || !td.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := td.cursorShape()
	if td.cachedDialBounds.Contains(p) {
		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		dx := p.X - centerX
		dy := p.Y - centerY
		R := td.cachedDialBounds.Width() * 0.5
		if dx*dx+dy*dy <= R*R {
			return facet.HitResult{Hit: true, MarkID: turnDialMarkIDKnob, Cursor: cursor}
		}
	}
	if td.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: turnDialMarkIDLabel, Cursor: cursor}
	}
	if td.cachedValueLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: turnDialMarkIDValueLabel, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: turnDialMarkIDRoot, Cursor: cursor}
}

func (td *TurnDial) cursorShape() facet.CursorShape {
	if td.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorGrab
}

func (td *TurnDial) onPointer(e facet.PointerEvent) bool {
	if td.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		td.hovered = true
		td.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		td.hovered = false
		if !td.dragging {
			td.pressed = false
		}
		td.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		td.hovered = true
		td.pressed = true
		td.focusFromPointer = true
		td.focusedVisible = false

		centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
		centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
		dx := e.Position.X - centerX
		dy := e.Position.Y - centerY
		R := td.cachedDialBounds.Width() * 0.5

		if dx*dx+dy*dy <= R*R {
			td.dragging = true
			td.updateValueFromPoint(e.Position)
		}
		td.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	case platform.PointerMove:
		if td.dragging {
			td.updateValueFromPoint(e.Position)
			td.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		}
		return td.hovered
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := td.pressed
		td.pressed = false
		td.dragging = false
		td.Activated.Emit(signal.Fired)
		td.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return wasPressed
	default:
		return false
	}
}

func (td *TurnDial) updateValueFromPoint(p gfx.Point) {
	if td.cachedDialBounds.IsEmpty() {
		return
	}
	centerX := (td.cachedDialBounds.Min.X + td.cachedDialBounds.Max.X) * 0.5
	centerY := (td.cachedDialBounds.Min.Y + td.cachedDialBounds.Max.Y) * 0.5
	dx := float64(p.X - centerX)
	dy := float64(p.Y - centerY)

	if dx == 0 && dy == 0 {
		return
	}

	phiRad := math.Atan2(dy, dx)
	phiDeg := phiRad * 180.0 / math.Pi

	angle := phiDeg - 135.0
	for angle < 0 {
		angle += 360.0
	}
	if angle > 270.0 {
		if angle > 315.0 {
			angle = 0
		} else {
			angle = 270.0
		}
	}

	frac := angle / 270.0
	minV, maxV := td.normalizedRange()
	td.Value.Set(minV + frac*(maxV-minV))
}

func (td *TurnDial) onKey(e facet.KeyEvent) bool {
	if td.Disabled.Get() {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			td.pressed = true
			td.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := td.pressed
			td.pressed = false
			td.Activated.Emit(signal.Fired)
			td.invalidate(facet.DirtyProjection)
			return wasPressed
		}
	case platform.KeyLeft, platform.KeyDown:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			td.adjustValue(-td.stepValue())
			return true
		}
	case platform.KeyRight, platform.KeyUp:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			td.adjustValue(td.stepValue())
			return true
		}
	}
	return false
}

func (td *TurnDial) adjustValue(delta float64) bool {
	if delta == 0 {
		return true
	}
	td.Value.Set(td.clampValue(td.currentValue() + delta))
	return true
}

type turnDialGroupPolicy struct{}

func (turnDialGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (turnDialGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (turnDialGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}

type TurnDialSlots struct {
	Root       theme.MarkStyle
	Track      theme.MarkStyle
	Knob       theme.MarkStyle
	Dot        theme.MarkStyle
	Label      theme.MarkStyle
	ValueLabel theme.MarkStyle
}

func defaultTurnDialSlots(tokens theme.Tokens) TurnDialSlots {
	isDark := tokens.Color.Background.R < 0.5

	var knobBase theme.Material
	var knobPressed theme.Material

	if isDark {
		knobBase = theme.Material{
			Fills: []theme.Fill{
				{
					Type: theme.FillGradient,
					Gradient: theme.Gradient{
						Type:  theme.GradientLinear,
						Start: gfx.Point{X: 0, Y: 0},
						End:   gfx.Point{X: 0, Y: 1},
						Stops: []theme.GradientStop{
							{Position: 0.0, Color: gfx.ColorFromRGBA8(45, 53, 77, 255)},
							{Position: 0.5, Color: gfx.ColorFromRGBA8(29, 34, 51, 255)},
							{Position: 1.0, Color: gfx.ColorFromRGBA8(17, 20, 32, 255)},
						},
					},
					Opacity: 1,
				},
			},
			Strokes: []theme.MaterialStroke{
				{
					Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.55},
					Width:      0,
					BlurRadius: 6,
					Offset:     gfx.Point{X: 2.0, Y: 2.0},
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 255, 255, 255), Opacity: 0.22},
					Width:  1.0,
					Offset: gfx.Point{X: -0.7, Y: -0.7},
					Inner:  true,
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.4},
					Width:  1.0,
					Offset: gfx.Point{X: 0.7, Y: 0.7},
					Inner:  true,
				},
			},
			Opacity: 1,
		}

		knobPressed = theme.Material{
			Fills: []theme.Fill{
				{
					Type: theme.FillGradient,
					Gradient: theme.Gradient{
						Type:  theme.GradientLinear,
						Start: gfx.Point{X: 0, Y: 0},
						End:   gfx.Point{X: 0, Y: 1},
						Stops: []theme.GradientStop{
							{Position: 0.0, Color: gfx.ColorFromRGBA8(36, 43, 61, 255)},
							{Position: 0.5, Color: gfx.ColorFromRGBA8(23, 27, 41, 255)},
							{Position: 1.0, Color: gfx.ColorFromRGBA8(11, 13, 20, 255)},
						},
					},
					Opacity: 1,
				},
			},
			Strokes: []theme.MaterialStroke{
				{
					Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.55},
					Width:      0,
					BlurRadius: 4,
					Offset:     gfx.Point{X: 1.0, Y: 1.0},
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 255, 255, 255), Opacity: 0.15},
					Width:  1.0,
					Offset: gfx.Point{X: 0.7, Y: 0.7},
					Inner:  true,
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.5},
					Width:  1.0,
					Offset: gfx.Point{X: -0.7, Y: -0.7},
					Inner:  true,
				},
			},
			Opacity: 1,
		}
	} else {
		knobBase = theme.Material{
			Fills: []theme.Fill{
				{
					Type: theme.FillGradient,
					Gradient: theme.Gradient{
						Type:  theme.GradientLinear,
						Start: gfx.Point{X: 0, Y: 0},
						End:   gfx.Point{X: 0, Y: 1},
						Stops: []theme.GradientStop{
							{Position: 0.0, Color: gfx.ColorFromRGBA8(255, 255, 255, 255)},
							{Position: 0.3, Color: gfx.ColorFromRGBA8(216, 216, 216, 255)},
							{Position: 0.7, Color: gfx.ColorFromRGBA8(160, 160, 160, 255)},
							{Position: 1.0, Color: gfx.ColorFromRGBA8(120, 120, 120, 255)},
						},
					},
					Opacity: 1,
				},
			},
			Strokes: []theme.MaterialStroke{
				{
					Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.4},
					Width:      0,
					BlurRadius: 6,
					Offset:     gfx.Point{X: 1.5, Y: 1.5},
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 255, 255, 255), Opacity: 0.8},
					Width:  1.5,
					Offset: gfx.Point{X: -1, Y: -1},
					Inner:  true,
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.3},
					Width:  1.5,
					Offset: gfx.Point{X: 1, Y: 1},
					Inner:  true,
				},
			},
			Opacity: 1,
		}

		knobPressed = theme.Material{
			Fills: []theme.Fill{
				{
					Type: theme.FillGradient,
					Gradient: theme.Gradient{
						Type:  theme.GradientLinear,
						Start: gfx.Point{X: 0, Y: 0},
						End:   gfx.Point{X: 0, Y: 1},
						Stops: []theme.GradientStop{
							{Position: 0.0, Color: gfx.ColorFromRGBA8(210, 210, 210, 255)},
							{Position: 0.5, Color: gfx.ColorFromRGBA8(180, 180, 180, 255)},
							{Position: 1.0, Color: gfx.ColorFromRGBA8(140, 140, 140, 255)},
						},
					},
					Opacity: 1,
				},
			},
			Strokes: []theme.MaterialStroke{
				{
					Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.4},
					Width:      0,
					BlurRadius: 5,
					Offset:     gfx.Point{X: -1.5, Y: -1.5},
					Inner:      true,
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 255, 255, 255), Opacity: 0.8},
					Width:  1.5,
					Offset: gfx.Point{X: 1, Y: 1},
					Inner:  true,
				},
				{
					Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.3},
					Width:  1.5,
					Offset: gfx.Point{X: -1, Y: -1},
					Inner:  true,
				},
			},
			Opacity: 1,
		}
	}

	return TurnDialSlots{
		Root: theme.MarkStyle{
			Base: theme.Material{Opacity: 0},
		},
		Track: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{
					{
						Type: theme.FillGradient,
						Gradient: theme.Gradient{
							Type:  theme.GradientLinear,
							Start: gfx.Point{X: 0, Y: 0},
							End:   gfx.Point{X: 0, Y: 1},
							Stops: []theme.GradientStop{
								{Position: 0.0, Color: gfx.ColorFromRGBA8(12, 12, 12, 255)},
								{Position: 1.0, Color: gfx.ColorFromRGBA8(56, 56, 56, 255)},
							},
						},
						Opacity: 1,
					},
				},
				Strokes: []theme.MaterialStroke{
					{
						Paint:      theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(0, 0, 0, 255), Opacity: 0.6},
						Width:      1.5,
						BlurRadius: 3,
						Offset:     gfx.Point{X: 1, Y: 1},
						Inner:      true,
					},
					{
						Paint:  theme.Fill{Type: theme.FillSolid, Color: gfx.ColorFromRGBA8(255, 255, 255, 255), Opacity: 0.4},
						Width:  1.0,
						Offset: gfx.Point{X: -0.5, Y: -0.5},
						Inner:  true,
					},
				},
				Opacity: 1,
			},
		},
		Knob: theme.MarkStyle{
			Base:    knobBase,
			Pressed: &knobPressed,
		},
		Dot: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.Primary,
					Opacity: 1,
				}},
				Opacity: 1,
			},
		},
		Label: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.OnSurface,
					Opacity: 1,
				}},
				Opacity: 1,
			},
		},
		ValueLabel: theme.MarkStyle{
			Base: theme.Material{
				Fills: []theme.Fill{{
					Type:    theme.FillSolid,
					Color:   tokens.Color.OnSurfaceVariant,
					Opacity: 1,
				}},
				Opacity: 1,
			},
		},
	}
}

func arcPath(center gfx.Point, radius float32, startAngleRad, endAngleRad float64) gfx.Path {
	if radius <= 0 {
		return gfx.Path{}
	}
	numSegments := 40
	builder := gfx.NewPath()
	first := true
	for i := 0; i <= numSegments; i++ {
		t := float64(i) / float64(numSegments)
		angleRad := startAngleRad + t*(endAngleRad-startAngleRad)
		x := center.X + float32(math.Cos(angleRad))*radius
		y := center.Y + float32(math.Sin(angleRad))*radius
		if first {
			builder.MoveTo(gfx.Point{X: x, Y: y})
			first = false
		} else {
			builder.LineTo(gfx.Point{X: x, Y: y})
		}
	}
	return builder.Build()
}

func darkenColor(c gfx.Color, factor float32) gfx.Color {
	r, g, b, a := c.ToRGBA8()
	if a == 0 {
		return c
	}
	scale := 1 - factor
	return gfx.ColorFromRGBA8(
		clampByte(float32(r)*scale),
		clampByte(float32(g)*scale),
		clampByte(float32(b)*scale),
		a,
	)
}

func clampByte(v float32) uint8 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return uint8(v)
}
