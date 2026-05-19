package selection

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
	sliderMarkIDRoot       facet.MarkID = 1
	sliderMarkIDTrack      facet.MarkID = 2
	sliderMarkIDActive     facet.MarkID = 3
	sliderMarkIDThumb      facet.MarkID = 4
	sliderMarkIDValueLabel facet.MarkID = 5
	sliderMarkIDTickMarks  facet.MarkID = 6
	sliderMarkIDFocusRing  facet.MarkID = 7
)

// Slider implements the selection.slider standard mark.
type Slider struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Value *store.ValueStore[float64]

	Label     string
	Min       float64
	Max       float64
	Step      float64
	Precision int
	Disabled  bool

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool

	cachedLayout           *text.TextLayout
	cachedLabelLayout      *text.TextLayout
	cachedValueLayout      *text.TextLayout
	cachedTokens           theme.Tokens
	cachedRecipe           shared.SliderSlots
	cachedRootBounds       gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedTrackBounds      gfx.Rect
	cachedActiveBounds     gfx.Rect
	cachedThumbBounds      gfx.Rect
	cachedValueLabelBounds gfx.Rect
	cachedTickRects        []gfx.Rect
	cachedTrackThickness   float32
	cachedThumbSize        float32
	cachedTickSize         float32
	cachedGap              float32
	cachedLabelStyle       text.TextStyle
	cachedValueStyle       text.TextStyle
	cachedWritingDirection facet.WritingDirection
	cachedMinWidth         float32
	cachedMinHeight        float32
}

var _ facet.FacetImpl = (*Slider)(nil)
var _ layout.AnchorExporter = (*Slider)(nil)

// NewSlider constructs a selection.slider mark with canonical defaults.
func NewSlider(label string, min, max, step float64) *Slider {
	s := &Slider{
		Facet:     facet.NewFacet(),
		Value:     store.NewValueStore[float64](min),
		Label:     label,
		Min:       min,
		Max:       max,
		Step:      step,
		Precision: -1,
	}
	s.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: sliderGroupPolicy{},
	}
	s.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := s.measureIntrinsic(ctx, constraints)
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
	s.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return s.measure(ctx, constraints)
	}
	s.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.layoutRole.ArrangedBounds = bounds
		s.arrange(ctx, bounds)
	}
	s.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := s.buildCommands(bounds, nil)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	s.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := s.buildCommands(s.layoutRole.ArrangedBounds, ctx.Runtime)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	s.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult {
		return s.hitTest(p)
	}
	s.inputRole.OnPointer = func(e facet.PointerEvent) bool {
		return s.onPointer(e)
	}
	s.inputRole.OnKey = func(e facet.KeyEvent) bool {
		return s.onKey(e)
	}
	s.focusRole.Focusable = func() bool {
		return !s.Disabled
	}
	s.focusRole.TabIndex = 0
	s.focusRole.OnFocusGained = func() {
		s.onFocusGained()
	}
	s.focusRole.OnFocusLost = func() {
		s.onFocusLost()
	}
	s.textRole.IMEEnabled = false
	s.AddRole(&s.layoutRole)
	s.AddRole(&s.renderRole)
	s.AddRole(&s.projectionRole)
	s.AddRole(&s.hitRole)
	s.AddRole(&s.inputRole)
	s.AddRole(&s.focusRole)
	s.AddRole(&s.textRole)
	return s
}

// Base satisfies facet.FacetImpl.
func (s *Slider) Base() *facet.Facet {
	s.Facet.BindImpl(s)
	return &s.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (s *Slider) AccessibilityRole() string {
	return "slider"
}

// AccessibleName reports the semantic name source required by the spec.
func (s *Slider) AccessibleName() string {
	if s == nil {
		return ""
	}
	return s.Label
}

// SetLabel updates the authored label text.
func (s *Slider) SetLabel(label string) {
	if s == nil || s.Label == label {
		return
	}
	s.Label = label
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetValue updates the canonical numeric value.
func (s *Slider) SetValue(value float64) {
	if s == nil {
		return
	}
	clamped := s.clampValue(value)
	if s.Value == nil {
		s.Value = store.NewValueStore[float64](clamped)
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	if s.Value.Get() == clamped {
		return
	}
	s.Value.Set(clamped)
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetMin updates the minimum selectable value.
func (s *Slider) SetMin(min float64) {
	if s == nil || s.Min == min {
		return
	}
	s.Min = min
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetMax updates the maximum selectable value.
func (s *Slider) SetMax(max float64) {
	if s == nil || s.Max == max {
		return
	}
	s.Max = max
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetStep updates the authored increment.
func (s *Slider) SetStep(step float64) {
	if s == nil || s.Step == step {
		return
	}
	s.Step = step
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetPrecision updates the explicit numeric precision.
func (s *Slider) SetPrecision(precision int) {
	if s == nil || s.Precision == precision {
		return
	}
	s.Precision = precision
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (s *Slider) SetDisabled(disabled bool) {
	if s == nil || s.Disabled == disabled {
		return
	}
	s.Disabled = disabled
	if disabled {
		s.hovered = false
		s.pressed = false
		s.dragging = false
		s.focusedVisible = false
		s.focusFromPointer = false
	}
	s.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the slider anchor set.
func (s *Slider) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if s == nil {
		return nil
	}
	bounds := s.layoutRole.ArrangedBounds
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
	if s.cachedLabelLayout != nil {
		out["baseline"] = gfx.Point{X: s.cachedLabelBounds.Min.X, Y: s.cachedLabelBounds.Min.Y + s.cachedLabelLayout.Baseline}
	} else if s.cachedValueLayout != nil {
		out["baseline"] = gfx.Point{X: s.cachedValueLabelBounds.Min.X, Y: s.cachedValueLabelBounds.Min.Y + s.cachedValueLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (s *Slider) Children() []facet.GroupChild { return nil }

// OnAttach wires store invalidation for the bound value store.
func (s *Slider) OnAttach(ctx facet.AttachContext) {
	if s.Value == nil {
		s.Value = store.NewValueStore[float64](s.clampValue(s.Min))
	}
	facet.Store(facet.Subscribe(s), &s.Value.OnChange, s.Value.Version, func(signal.Change[float64]) {
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (s *Slider) OnActivate() {}

// OnDeactivate is unused.
func (s *Slider) OnDeactivate() {}

// OnDetach clears cached projection state.
func (s *Slider) OnDetach() {
	s.cachedLayout = nil
	s.cachedLabelLayout = nil
	s.cachedValueLayout = nil
	s.cachedTokens = theme.Tokens{}
	s.cachedRecipe = shared.SliderSlots{}
	s.cachedRootBounds = gfx.Rect{}
	s.cachedLabelBounds = gfx.Rect{}
	s.cachedTrackBounds = gfx.Rect{}
	s.cachedActiveBounds = gfx.Rect{}
	s.cachedThumbBounds = gfx.Rect{}
	s.cachedValueLabelBounds = gfx.Rect{}
	s.cachedTickRects = nil
	s.cachedTrackThickness = 0
	s.cachedThumbSize = 0
	s.cachedTickSize = 0
	s.cachedGap = 0
	s.cachedLabelStyle = text.TextStyle{}
	s.cachedValueStyle = text.TextStyle{}
}

func (s *Slider) invalidate(flags facet.DirtyFlags) {
	if s == nil {
		return
	}
	s.Base().Invalidate(flags)
}

func (s *Slider) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	variant := sliderRecipeVariant(resolved)
	slots, _ := uiinput.ResolveSliderRecipe(style, variant)
	s.cachedTokens = resolved.TokenSet()
	s.cachedRecipe = slots
	s.cachedWritingDirection = ctx.WritingDirection
	s.cachedTrackThickness = sliderTrackThickness(resolved)
	s.cachedThumbSize = sliderThumbSize(resolved)
	s.cachedTickSize = sliderTickSize(resolved)
	s.cachedGap = sliderGap(resolved)
	s.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	s.cachedValueStyle = resolved.TextStyle(theme.TextBodyS)
	shaper := s.newShaper(ctx.Runtime)
	if shaper == nil {
		s.cachedLayout = nil
		s.cachedLabelLayout = nil
		s.cachedValueLayout = nil
		return facet.MeasureResult{}
	}
	shaper.SetContentScale(ctx.ContentScale)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = sliderDefaultMinWidth(resolved)
	}
	labelLayout := s.shapeTruncated(shaper, s.cachedLabelStyle, s.Label, maxWidth)
	valueLayout := s.shapeTruncated(shaper, s.cachedValueStyle, s.valueLabelText(), maxWidth)
	labelH := layoutHeight(labelLayout)
	valueH := layoutHeight(valueLayout)
	controlH := maxFloat(s.cachedThumbSize, s.cachedTrackThickness+s.cachedTickSize*2)
	totalH := controlH
	if valueH > 0 {
		totalH += valueH + s.cachedGap
	}
	if labelH > 0 {
		totalH += labelH + s.cachedGap
	}
	minWidth := sliderDefaultMinWidth(resolved)
	if labelLayout != nil {
		minWidth = maxFloat(minWidth, labelLayout.Bounds.Width())
	}
	if valueLayout != nil {
		minWidth = maxFloat(minWidth, valueLayout.Bounds.Width()+sliderThumbInset(resolved)*2)
	}
	if constraints.MaxSize.W > 0 {
		minWidth = minFloat(minWidth, constraints.MaxSize.W)
	}
	width := maxFloat(minWidth, sliderDefaultTrackLength(resolved))
	if constraints.MaxSize.W > 0 {
		width = minFloat(width, constraints.MaxSize.W)
	}
	s.cachedLayout = &text.TextLayout{Bounds: text.RectFromXYWH(0, 0, width, totalH), LineHeight: totalH, Baseline: 0}
	s.cachedLabelLayout = labelLayout
	s.cachedValueLayout = valueLayout
	s.textRole.Layout = valueLayout
	s.textRole.Selection = text.TextRange{}
	s.textRole.CaretVisible = false
	s.textRole.CaretPosition = text.TextPosition{}
	size := gfx.Size{W: width, H: totalH}
	s.layoutRole.MeasuredSize = size
	s.layoutRole.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
	return s.layoutRole.MeasuredResult
}

func (s *Slider) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return s.measure(ctx, constraints).Size
}

func (s *Slider) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	s.cachedRootBounds = bounds
	s.cachedLabelBounds = gfx.Rect{}
	s.cachedTrackBounds = gfx.Rect{}
	s.cachedActiveBounds = gfx.Rect{}
	s.cachedThumbBounds = gfx.Rect{}
	s.cachedValueLabelBounds = gfx.Rect{}
	s.cachedTickRects = nil
	s.layoutRole.ArrangedBounds = bounds
	if s.cachedLayout == nil || bounds.IsEmpty() {
		return
	}
	labelH := layoutHeight(s.cachedLabelLayout)
	valueH := layoutHeight(s.cachedValueLayout)
	trackH := maxFloat(s.cachedTrackThickness, s.cachedThumbSize)
	controlTop := bounds.Min.Y
	if labelH > 0 {
		s.cachedLabelBounds = gfx.RectFromXYWH(bounds.Min.X, controlTop, bounds.Width(), labelH)
		controlTop += labelH + s.cachedGap
	}
	controlH := valueH
	if controlH > 0 {
		controlH += s.cachedGap
	}
	controlH += trackH
	controlBounds := gfx.RectFromXYWH(bounds.Min.X, controlTop, bounds.Width(), controlH)
	trackY := controlBounds.Min.Y + controlBounds.Height() - trackH*0.5
	thumbSize := s.cachedThumbSize
	if thumbSize <= 0 {
		thumbSize = 16
	}
	inset := sliderThumbInsetFromSize(thumbSize)
	trackLeft := bounds.Min.X + inset
	trackRight := bounds.Max.X - inset
	if trackRight < trackLeft {
		trackRight = trackLeft
	}
	trackWidth := trackRight - trackLeft
	value := s.displayValue()
	frac := s.valueFraction(value)
	if s.cachedWritingDirection == facet.WritingDirectionRTL {
		frac = 1 - frac
	}
	thumbCenterX := trackLeft
	if trackWidth > 0 {
		thumbCenterX = trackLeft + trackWidth*frac
	}
	s.cachedTrackBounds = gfx.RectFromXYWH(trackLeft, trackY-s.cachedTrackThickness*0.5, trackWidth, s.cachedTrackThickness)
	if s.cachedTrackBounds.IsEmpty() {
		s.cachedTrackBounds = gfx.RectFromXYWH(trackLeft, trackY-1, trackWidth, 2)
	}
	activeLeft := trackLeft
	activeRight := thumbCenterX
	if s.cachedWritingDirection == facet.WritingDirectionRTL {
		activeLeft, activeRight = thumbCenterX, trackRight
	}
	if activeRight < activeLeft {
		activeLeft, activeRight = activeRight, activeLeft
	}
	s.cachedActiveBounds = gfx.RectFromXYWH(activeLeft, s.cachedTrackBounds.Min.Y, maxFloat(0, activeRight-activeLeft), s.cachedTrackBounds.Height())
	s.cachedThumbBounds = gfx.RectFromXYWH(thumbCenterX-thumbSize*0.5, trackY-thumbSize*0.5, thumbSize, thumbSize)
	if s.cachedValueLayout != nil {
		valueW := s.cachedValueLayout.Bounds.Width()
		valueLeft := thumbCenterX - valueW*0.5
		minLeft := bounds.Min.X
		maxLeft := bounds.Max.X - valueW
		if maxLeft < minLeft {
			maxLeft = minLeft
		}
		valueLeft = clampFloat(valueLeft, minLeft, maxLeft)
		valueY := controlTop
		if valueH > 0 {
			s.cachedValueLabelBounds = gfx.RectFromXYWH(valueLeft, valueY, valueW, valueH)
		}
	}
	tickRects := s.tickRects(trackLeft, trackRight, trackY)
	s.cachedTickRects = tickRects
	s.layoutRole.ArrangedBounds = bounds
}

func (s *Slider) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.SliderSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveSliderRecipe(style, sliderRecipeVariant(resolved))
	return resolved, slots, true
}

func (s *Slider) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.SliderSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: s.cachedTokens}, s.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, s.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uiinput.ResolveSliderRecipe(style, sliderRecipeVariantForDensity(styleContextDensity(style)))
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: s.cachedTokens}, s.cachedRecipe
}

func (s *Slider) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if s == nil || bounds.IsEmpty() || s.cachedLayout == nil {
		return nil
	}
	style, slots := s.resolveProjectionTheme(runtime)
	state := s.interactionState()
	tokens := style.Tokens
	root := slots.Root.Resolve(state, tokens)
	track := slots.Track.Resolve(state, tokens)
	active := slots.ActiveTrack.Resolve(state, tokens)
	thumb := slots.Thumb.Resolve(state, tokens)
	ticks := slots.TickMarks.Resolve(state, tokens)
	valueLabel := slots.ValueLabel.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	cmds := make([]gfx.Command, 0, 32)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(track) {
		cmds = append(cmds, materialCommands(gfx.RectPath(s.cachedTrackBounds), track)...)
	}
	if !isTransparentMaterial(active) {
		cmds = append(cmds, materialCommands(gfx.RectPath(s.cachedActiveBounds), active)...)
	}
	cmds = append(cmds, s.tickCommands(ticks)...)
	if s.cachedThumbBounds.IsEmpty() == false && !isTransparentMaterial(thumb) {
		cmds = append(cmds, materialCommands(gfx.CirclePath(gfx.Point{X: (s.cachedThumbBounds.Min.X + s.cachedThumbBounds.Max.X) * 0.5, Y: (s.cachedThumbBounds.Min.Y + s.cachedThumbBounds.Max.Y) * 0.5}, s.cachedThumbBounds.Width()*0.5), thumb)...)
	}
	if s.cachedLabelLayout != nil {
		cmds = append(cmds, s.textCommands(s.cachedLabelLayout, s.cachedLabelBounds, valueLabel)...)
	}
	if s.cachedValueLayout != nil {
		cmds = append(cmds, s.textCommands(s.cachedValueLayout, s.cachedValueLabelBounds, valueLabel)...)
	}
	if s.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, s.cachedGap*0.5)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, s.cachedThumbSize*0.5+inset), focus)...)
	}
	return cmds
}

func (s *Slider) tickCommands(material theme.Material) []gfx.Command {
	if len(s.cachedTickRects) == 0 || isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(s.cachedTickRects))
	for _, rect := range s.cachedTickRects {
		if rect.IsEmpty() {
			continue
		}
		cmds = append(cmds, materialCommands(gfx.RectPath(rect), material)...)
	}
	return cmds
}

func (s *Slider) textCommands(layout *text.TextLayout, bounds gfx.Rect, material theme.Material) []gfx.Command {
	if layout == nil || isTransparentMaterial(material) {
		return nil
	}
	brush := gfx.SolidBrush(materialColor(material))
	baseOrigin := gfx.Point{X: bounds.Min.X + layout.Bounds.Min.X, Y: bounds.Min.Y + layout.Bounds.Min.Y}
	cmds := make([]gfx.Command, 0, len(layout.Lines))
	for _, line := range layout.Lines {
		lineOrigin := gfx.Point{X: baseOrigin.X + line.Bounds.Min.X, Y: baseOrigin.Y + line.Bounds.Min.Y}
		for _, run := range line.Runs {
			cmds = append(cmds, gfx.DrawGlyphRun{Run: run, Origin: lineOrigin, Brush: brush})
		}
	}
	return cmds
}

func (s *Slider) hitTest(p gfx.Point) facet.HitResult {
	if s == nil || s.layoutRole.ArrangedBounds.IsEmpty() || !s.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := s.cursorShape()
	if s.focusedVisible && s.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDFocusRing, Cursor: cursor}
	}
	if s.cachedValueLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDValueLabel, Cursor: cursor}
	}
	if s.cachedThumbBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDThumb, Cursor: cursor}
	}
	if s.cachedActiveBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDActive, Cursor: cursor}
	}
	if s.cachedTrackBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDTrack, Cursor: cursor}
	}
	for _, rect := range s.cachedTickRects {
		if rect.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: sliderMarkIDTickMarks, Cursor: cursor}
		}
	}
	if s.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: sliderMarkIDRoot, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: sliderMarkIDRoot, Cursor: cursor}
}

func (s *Slider) pointInFocusRing(p gfx.Point) bool {
	bounds := s.layoutRole.ArrangedBounds
	if bounds.IsEmpty() || !bounds.Contains(p) {
		return false
	}
	ring := maxFloat(1, s.cachedGap*0.5)
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

func (s *Slider) cursorShape() facet.CursorShape {
	if s.Disabled {
		return facet.CursorDefault
	}
	if s.dragging || s.pressed {
		return facet.CursorGrabbing
	}
	return facet.CursorGrab
}

func (s *Slider) onPointer(e facet.PointerEvent) bool {
	if s.Disabled {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		s.hovered = true
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		s.hovered = false
		if !s.dragging {
			s.pressed = false
		}
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		s.hovered = true
		s.pressed = true
		s.dragging = true
		s.focusFromPointer = true
		s.focusedVisible = false
		s.updateValueFromPoint(e.Position)
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return true
	case platform.PointerMove:
		if s.dragging {
			s.updateValueFromPoint(e.Position)
			s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
			return true
		}
		return s.hovered
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := s.pressed
		s.pressed = false
		s.dragging = false
		s.updateValueFromPoint(e.Position)
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return wasPressed
	default:
		return false
	}
}

func (s *Slider) onKey(e facet.KeyEvent) bool {
	if s.Disabled {
		return false
	}
	switch e.Key {
	case platform.KeySpace, platform.KeyEnter:
		switch e.Kind {
		case platform.KeyPress, platform.KeyRepeat:
			s.pressed = true
			s.invalidate(facet.DirtyProjection)
			return true
		case platform.KeyRelease:
			wasPressed := s.pressed
			s.pressed = false
			s.invalidate(facet.DirtyProjection)
			return wasPressed
		}
	case platform.KeyLeft:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(-s.stepDeltaForDirection(true))
		}
	case platform.KeyRight:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(s.stepDeltaForDirection(true))
		}
	case platform.KeyUp:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(s.stepDeltaForDirection(false))
		}
	case platform.KeyDown:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(-s.stepDeltaForDirection(false))
		}
	case platform.KeyHome:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			s.SetValue(s.normalizedMin())
			return true
		}
	case platform.KeyEnd:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			s.SetValue(s.normalizedMax())
			return true
		}
	case platform.KeyPageUp:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(s.pageDelta())
		}
	case platform.KeyPageDown:
		if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
			return s.adjustValue(-s.pageDelta())
		}
	}
	return false
}

func (s *Slider) onFocusGained() {
	s.focusedVisible = !s.focusFromPointer
	s.focusFromPointer = false
	s.invalidate(facet.DirtyProjection)
}

func (s *Slider) onFocusLost() {
	s.focusedVisible = false
	s.pressed = false
	s.dragging = false
	s.focusFromPointer = false
	s.invalidate(facet.DirtyProjection)
}

func (s *Slider) interactionState() theme.InteractionState {
	switch {
	case s.Disabled:
		return theme.StateDisabled
	case s.pressed || s.dragging:
		return theme.StatePressed
	case s.hovered:
		return theme.StateHover
	case s.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (s *Slider) currentValue() float64 {
	if s == nil || s.Value == nil {
		return s.normalizedMin()
	}
	return s.Value.Get()
}

func (s *Slider) displayValue() float64 {
	return s.clampValue(s.currentValue())
}

func (s *Slider) valueFraction(value float64) float32 {
	minV, maxV := s.normalizedRange()
	if maxV <= minV {
		return 0.5
	}
	return float32((value - minV) / (maxV - minV))
}

func (s *Slider) normalizedRange() (float64, float64) {
	if s.Min <= s.Max {
		return s.Min, s.Max
	}
	return s.Max, s.Min
}

func (s *Slider) normalizedMin() float64 {
	minV, _ := s.normalizedRange()
	return minV
}

func (s *Slider) normalizedMax() float64 {
	_, maxV := s.normalizedRange()
	return maxV
}

func (s *Slider) clampValue(value float64) float64 {
	minV, maxV := s.normalizedRange()
	if value < minV {
		return minV
	}
	if value > maxV {
		return maxV
	}
	return value
}

func (s *Slider) updateValueFromPoint(p gfx.Point) {
	if s.cachedTrackBounds.IsEmpty() {
		return
	}
	trackLeft := s.cachedTrackBounds.Min.X
	trackRight := s.cachedTrackBounds.Max.X
	if trackRight < trackLeft {
		trackLeft, trackRight = trackRight, trackLeft
	}
	if trackRight <= trackLeft {
		s.SetValue(s.normalizedMin())
		return
	}
	x := clampFloat(p.X, trackLeft, trackRight)
	frac := float64((x - trackLeft) / (trackRight - trackLeft))
	if s.cachedWritingDirection == facet.WritingDirectionRTL {
		frac = 1 - frac
	}
	minV, maxV := s.normalizedRange()
	s.SetValue(minV + frac*(maxV-minV))
}

func (s *Slider) adjustValue(delta float64) bool {
	if delta == 0 {
		return true
	}
	s.SetValue(s.clampValue(s.currentValue() + delta))
	return true
}

func (s *Slider) stepDeltaForDirection(horizontal bool) float64 {
	step := s.stepValue()
	if horizontal && s.cachedWritingDirection == facet.WritingDirectionRTL {
		step = -step
	}
	return step
}

func (s *Slider) stepValue() float64 {
	if s.Step > 0 {
		return s.Step
	}
	minV, maxV := s.normalizedRange()
	if maxV > minV {
		return (maxV - minV) / 100
	}
	return 1
}

func (s *Slider) pageDelta() float64 {
	minV, maxV := s.normalizedRange()
	step := s.stepValue()
	rng := maxV - minV
	page := rng / 10
	if page <= 0 {
		page = step * 10
	}
	if page < step {
		page = step
	}
	return page
}

func (s *Slider) valueLabelText() string {
	return s.formatValue(s.displayValue())
}

func (s *Slider) formatValue(value float64) string {
	precision := s.Precision
	if precision < 0 {
		precision = s.autoPrecision()
	}
	if precision < 0 {
		precision = 0
	}
	return strconv.FormatFloat(value, 'f', precision, 64)
}

func (s *Slider) autoPrecision() int {
	if s.Step <= 0 {
		return 0
	}
	step := strconv.FormatFloat(s.Step, 'f', -1, 64)
	if idx := strings.IndexByte(step, '.'); idx >= 0 {
		return len(strings.TrimRight(step[idx+1:], "0"))
	}
	return 0
}

func (s *Slider) tickRects(trackLeft, trackRight, trackY float32) []gfx.Rect {
	minV, maxV := s.normalizedRange()
	step := s.stepValue()
	if step <= 0 || maxV <= minV {
		return nil
	}
	count := int(math.Round((maxV - minV) / step))
	if count < 1 {
		count = 1
	}
	if count > 200 {
		count = 200
	}
	rects := make([]gfx.Rect, 0, count+1)
	for i := 0; i <= count; i++ {
		frac := float32(i) / float32(count)
		if s.cachedWritingDirection == facet.WritingDirectionRTL {
			frac = 1 - frac
		}
		x := trackLeft
		if trackRight > trackLeft {
			x = trackLeft + (trackRight-trackLeft)*frac
		}
		size := s.cachedTickSize
		if size <= 0 {
			size = 2
		}
		rects = append(rects, gfx.RectFromXYWH(x-size*0.5, trackY-size*0.5, size, size))
	}
	return rects
}

func (s *Slider) newShaper(runtime any) *text.Shaper {
	registry := s.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (s *Slider) fontRegistry(runtime any) *text.FontRegistry {
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

func (s *Slider) shapeTruncated(shaper *text.Shaper, style text.TextStyle, content string, maxWidth float32) *text.TextLayout {
	layout := shaper.ShapeSimple(content, style)
	if layout == nil {
		return nil
	}
	if content == "" || maxWidth <= 0 || layout.Bounds.Width() <= maxWidth {
		return layout
	}
	runes := []rune(content)
	if len(runes) == 0 {
		return layout
	}
	if ellipsis := shaper.ShapeSimple("…", style); ellipsis != nil && ellipsis.Bounds.Width() <= maxWidth {
		best := 0
		lo, hi := 0, len(runes)
		for lo <= hi {
			mid := (lo + hi) / 2
			candidate := shaper.ShapeSimple(string(runes[:mid])+"…", style)
			if candidate != nil && candidate.Bounds.Width() <= maxWidth {
				best = mid
				lo = mid + 1
				continue
			}
			hi = mid - 1
		}
		if best > 0 {
			return shaper.ShapeSimple(string(runes[:best])+"…", style)
		}
		return ellipsis
	}
	best := 0
	lo, hi := 0, len(runes)
	for lo <= hi {
		mid := (lo + hi) / 2
		candidate := shaper.ShapeSimple(string(runes[:mid]), style)
		if candidate != nil && candidate.Bounds.Width() <= maxWidth {
			best = mid
			lo = mid + 1
			continue
		}
		hi = mid - 1
	}
	if best == 0 {
		return layout
	}
	return shaper.ShapeSimple(string(runes[:best]), style)
}

func sliderRecipeVariant(resolved theme.ResolvedContext) uiinput.SliderVariant {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return uiinput.SliderCompact
	default:
		return uiinput.SliderStandard
	}
}

func sliderRecipeVariantForDensity(id theme.DensityID) uiinput.SliderVariant {
	switch id {
	case theme.DensityIDCompact:
		return uiinput.SliderCompact
	default:
		return uiinput.SliderStandard
	}
}

func styleContextDensity(style theme.StyleContext) theme.DensityID {
	switch style.Tokens.Density.Mode {
	case theme.DensityCompact:
		return theme.DensityIDCompact
	case theme.DensityTouch:
		return theme.DensityIDTouch
	default:
		return theme.DensityIDComfortable
	}
}

func sliderTrackThickness(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 3
	case theme.DensityIDTouch:
		return 6
	default:
		return 4
	}
}

func sliderThumbSize(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 14
	case theme.DensityIDTouch:
		return 20
	default:
		return 16
	}
}

func sliderTickSize(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 2
	case theme.DensityIDTouch:
		return 4
	default:
		return 3
	}
}

func sliderGap(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 6
	case theme.DensityIDTouch:
		return 10
	default:
		return 8
	}
}

func sliderDefaultTrackLength(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 220
	case theme.DensityIDTouch:
		return 300
	default:
		return 260
	}
}

func sliderDefaultMinWidth(resolved theme.ResolvedContext) float32 {
	switch resolved.Density.ID {
	case theme.DensityIDCompact:
		return 220
	case theme.DensityIDTouch:
		return 280
	default:
		return 240
	}
}

func sliderThumbInset(resolved theme.ResolvedContext) float32 {
	return sliderThumbInsetFromSize(sliderThumbSize(resolved))
}

func sliderThumbInsetFromSize(size float32) float32 {
	if size <= 0 {
		return 8
	}
	return size * 0.5
}

func layoutHeight(layout *text.TextLayout) float32 {
	if layout == nil {
		return 0
	}
	return layout.Bounds.Height()
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

func clampFloat(v, minV, maxV float32) float32 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
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

type sliderGroupPolicy struct{}

func (sliderGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }
func (sliderGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (sliderGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
