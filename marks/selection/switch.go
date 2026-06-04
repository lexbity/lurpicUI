package selection

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
	switchMarkIDRoot       facet.MarkID = 1
	switchMarkIDTrack      facet.MarkID = 2
	switchMarkIDThumb      facet.MarkID = 3
	switchMarkIDLabel      facet.MarkID = 4
	switchMarkIDFocusRing  facet.MarkID = 5
	switchMarkIDStateLayer facet.MarkID = 6
)

// Switch implements the selection.switch standard mark.
type Switch struct {
	marks.Core

	Value *store.ValueStore[bool]

	Label    string
	Variant  marks.Binding[uiinput.SwitchVariant]
	Disabled marks.Binding[bool]

	textRole facet.TextRole

	hovered          bool
	pressed          bool
	focusedVisible   bool
	focusFromPointer bool

	cachedLayout           *text.TextLayout
	cachedLabelLayout      *text.TextLayout
	cachedTokens           theme.Tokens
	cachedRecipe           shared.SwitchSlots
	cachedRootBounds       gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedControlBounds    gfx.Rect
	cachedTrackBounds      gfx.Rect
	cachedThumbBounds      gfx.Rect
	cachedControlWidth     float32
	cachedControlHeight    float32
	cachedThumbSize        float32
	cachedTrackRadius      float32
	cachedRowGap           float32
	cachedLabelGap         float32
	cachedLabelStyle       text.TextStyle
	cachedWritingDirection facet.WritingDirection
}

var _ facet.FacetImpl = (*Switch)(nil)
var _ layout.AnchorExporter = (*Switch)(nil)
var _ marks.Mark = (*Switch)(nil)

// NewSwitch constructs a selection.switch mark with canonical defaults.
func NewSwitch(label string) *Switch {
	s := &Switch{
		Variant:  marks.Const(uiinput.SwitchStandard),
		Disabled: marks.Const(false),
		Value:    store.NewValueStore[bool](false),
		Label:    label,
	}
	s.Core.Facet = facet.NewFacet()
	s.AddBinding(s.Variant)
	s.AddBinding(s.Disabled)

	s.Layout.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearVertical,
		Policy: switchGroupPolicy{},
	}
	s.Layout.Child = facet.GroupChildContract{
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
	s.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return s.measure(ctx, constraints)
	}
	s.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		s.Layout.ArrangedBounds = bounds
		s.arrange(ctx, bounds)
	}
	s.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return s.buildCommands(s.Layout.ArrangedBounds, ctx.Runtime)
	}
	s.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return s.hitTest(p) }
	s.Input.OnPointer = func(e facet.PointerEvent) bool { return s.onPointer(e) }
	s.Input.OnKey = func(e facet.KeyEvent) bool { return s.onKey(e) }
	s.Focus.Focusable = func() bool { return !s.Disabled.Get() }
	s.Focus.TabIndex = 0
	s.Focus.OnFocusGained = func() { s.onFocusGained() }
	s.Focus.OnFocusLost = func() { s.onFocusLost() }
	s.textRole.IMEEnabled = false
	s.RegisterRoles()
	s.AddRole(&s.textRole)
	return s
}

// Base satisfies facet.FacetImpl.
func (s *Switch) Base() *facet.Facet {
	s.Facet.BindImpl(s)
	return &s.Facet
}

// Descriptor satisfies marks.Mark.
func (s *Switch) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "selection", TypeName: "switch"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (s *Switch) AccessibilityRole() string { return "switch" }

// AccessibleName reports the semantic name source required by the spec.
func (s *Switch) AccessibleName() string {
	if s == nil {
		return ""
	}
	return s.Label
}

// ExportAnchors publishes the switch anchor set.
func (s *Switch) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := s.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	out := s.Core.DefaultAnchors(bounds, ctx)
	if s.cachedLabelLayout != nil {
		out["baseline"] = gfx.Point{X: s.cachedLabelBounds.Min.X, Y: s.cachedLabelBounds.Min.Y + s.cachedLabelLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (s *Switch) Children() []facet.GroupChild { return nil }

// OnAttach wires store invalidation for the bound value store.
func (s *Switch) OnAttach(ctx facet.AttachContext) {
	s.Core.OnAttach()
	if s.Value == nil {
		s.Value = store.NewValueStore[bool](false)
	}
	facet.Store(facet.Subscribe(s), &s.Value.OnChange, s.Value.Version, func(signal.Change[bool]) {
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
	})
}

// OnActivate is unused.
func (s *Switch) OnActivate() { s.Core.OnActivate() }

// OnDeactivate is unused.
func (s *Switch) OnDeactivate() { s.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (s *Switch) OnDetach() {
	s.Core.OnDetach()
	s.cachedLayout = nil
	s.cachedLabelLayout = nil
	s.cachedTokens = theme.Tokens{}
	s.cachedRecipe = shared.SwitchSlots{}
	s.cachedRootBounds = gfx.Rect{}
	s.cachedLabelBounds = gfx.Rect{}
	s.cachedControlBounds = gfx.Rect{}
	s.cachedTrackBounds = gfx.Rect{}
	s.cachedThumbBounds = gfx.Rect{}
	s.cachedControlWidth = 0
	s.cachedControlHeight = 0
	s.cachedThumbSize = 0
	s.cachedTrackRadius = 0
	s.cachedRowGap = 0
	s.cachedLabelGap = 0
	s.cachedLabelStyle = text.TextStyle{}
}

func (s *Switch) invalidate(flags facet.DirtyFlags) {
	if s == nil {
		return
	}
	s.Facet.Invalidate(flags)
}

func (s *Switch) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uiinput.ResolveSwitchRecipe(style, s.Variant.Get())
	s.cachedTokens = resolved.TokenSet()
	s.cachedRecipe = slots
	s.cachedWritingDirection = ctx.WritingDirection
	s.cachedControlWidth = switchControlWidth(resolved)
	s.cachedControlHeight = switchControlHeight(resolved)
	s.cachedThumbSize = switchThumbSize(resolved)
	s.cachedTrackRadius = s.cachedControlHeight * 0.5
	s.cachedRowGap = float32(resolved.Spacing(theme.SpacingS))
	s.cachedLabelGap = float32(resolved.Spacing(theme.SpacingXS))
	s.cachedLabelStyle = resolved.TextStyle(theme.TextLabelM)
	shaper := s.newShaper(ctx.Runtime)
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = switchDefaultMaxWidth(resolved)
	}
	if shaper != nil {
		shaper.SetContentScale(ctx.ContentScale)
		s.cachedLabelLayout = shaper.ShapeTruncated(s.Label, s.cachedLabelStyle, maxWidth)
	} else {
		s.cachedLabelLayout = nil
	}
	labelH := text.Height(s.cachedLabelLayout)
	controlH := maxFloat(s.cachedControlHeight, resolved.Density.Scale(resolved.TokenSet().Spacing.TouchTarget))
	width := s.cachedControlWidth
	if s.cachedLabelLayout != nil {
		width = maxFloat(width, s.cachedLabelLayout.Bounds.Width())
	}
	height := labelH
	if labelH > 0 {
		height += s.cachedLabelGap
	}
	height += controlH
	if width <= 0 {
		width = s.cachedControlWidth
	}
	if height <= 0 {
		height = controlH
	}
	s.cachedLayout = &text.TextLayout{Bounds: text.RectFromXYWH(0, 0, width, height), LineHeight: height, Baseline: 0}
	s.textRole.Layout = s.cachedLabelLayout
	s.textRole.Selection = text.TextRange{}
	s.textRole.CaretVisible = false
	s.textRole.CaretPosition = text.TextPosition{}
	size := gfx.Size{W: width, H: height}
	s.Layout.MeasuredSize = size
	s.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return s.Layout.MeasuredResult
}

func (s *Switch) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return s.measure(ctx, constraints).Size
}

func (s *Switch) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	s.cachedRootBounds = bounds
	s.cachedLabelBounds = gfx.Rect{}
	s.cachedControlBounds = gfx.Rect{}
	s.cachedTrackBounds = gfx.Rect{}
	s.cachedThumbBounds = gfx.Rect{}
	s.Layout.ArrangedBounds = bounds
	if s.cachedLayout == nil || bounds.IsEmpty() {
		return
	}
	labelH := text.Height(s.cachedLabelLayout)
	controlH := maxFloat(s.cachedControlHeight, resolvedTouchHeight(bounds.Height(), s))
	rects := layout.ArrangeVerticalFlow(bounds, 0, s.cachedLabelGap, []gfx.Size{
		{W: bounds.Width(), H: labelH},
		{W: s.cachedControlWidth, H: controlH},
	}, s.cachedWritingDirection == facet.WritingDirectionRTL)
	if s.cachedLabelLayout != nil {
		s.cachedLabelBounds = rects[0]
	}
	s.cachedControlBounds = rects[1]
	s.cachedTrackBounds = text.AlignRectY(gfx.RectFromXYWH(s.cachedControlBounds.Min.X, s.cachedControlBounds.Min.Y, s.cachedControlWidth, s.cachedControlHeight), s.cachedControlBounds.Min.Y, s.cachedControlBounds.Height())
	if s.isChecked() {
		thumbX := s.cachedTrackBounds.Max.X - s.cachedThumbSize - maxFloat(2, s.cachedTrackRadius*0.2)
		s.cachedThumbBounds = text.AlignRectY(gfx.RectFromXYWH(thumbX, s.cachedControlBounds.Min.Y, s.cachedThumbSize, s.cachedThumbSize), s.cachedControlBounds.Min.Y, s.cachedControlBounds.Height())
	} else {
		thumbX := s.cachedTrackBounds.Min.X + maxFloat(2, s.cachedTrackRadius*0.2)
		s.cachedThumbBounds = text.AlignRectY(gfx.RectFromXYWH(thumbX, s.cachedControlBounds.Min.Y, s.cachedThumbSize, s.cachedThumbSize), s.cachedControlBounds.Min.Y, s.cachedControlBounds.Height())
	}
	s.Layout.ArrangedBounds = bounds
}

func (s *Switch) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.SwitchSlots) {
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
			slots, _ := uiinput.ResolveSwitchRecipe(style, s.Variant.Get())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: s.cachedTokens}, s.cachedRecipe
}

func (s *Switch) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if s == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := s.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	interaction := s.interactionState()
	selectedState := s.selectedState()
	root := slots.Root.Resolve(interaction, tokens)
	track := slots.Track.Resolve(selectedState, tokens)
	thumb := slots.Thumb.Resolve(selectedState, tokens)
	label := slots.Label.Resolve(s.labelState(), tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)
	stateLayer := slots.StateLayer.Resolve(interaction, tokens)

	cmds := make([]gfx.Command, 0, 16)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(stateLayer) && !s.cachedControlBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(s.cachedControlBounds, s.cachedTrackRadius), stateLayer)...)
	}
	if !isTransparentMaterial(track) && !s.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(s.cachedTrackBounds, s.cachedTrackRadius), track)...)
	}
	if !isTransparentMaterial(thumb) && !s.cachedThumbBounds.IsEmpty() {
		cmds = append(cmds, materialCommands(gfx.CirclePath(rectCenterPoint(s.cachedThumbBounds), s.cachedThumbBounds.Width()*0.5), thumb)...)
	}
	if s.cachedLabelLayout != nil {
		cmds = append(cmds, primitive.TextLayoutCommands(s.cachedLabelLayout, s.cachedLabelBounds, gfx.SolidBrush(materialColor(label)))...)
	}
	if s.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, s.cachedRowGap*0.5)
		ringBounds := bounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, s.cachedTrackRadius+inset), focus)...)
	}
	return cmds
}

func (s *Switch) hitTest(p gfx.Point) facet.HitResult {
	if s == nil || s.Layout.ArrangedBounds.IsEmpty() || !s.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := s.cursorShape()
	if s.focusedVisible && s.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: switchMarkIDFocusRing, Cursor: cursor}
	}
	if s.cachedLabelBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: switchMarkIDLabel, Cursor: cursor}
	}
	if s.cachedThumbBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: switchMarkIDThumb, Cursor: cursor}
	}
	if s.cachedTrackBounds.Contains(p) {
		if s.hovered || s.pressed || s.isChecked() {
			return facet.HitResult{Hit: true, MarkID: switchMarkIDStateLayer, Cursor: cursor}
		}
		return facet.HitResult{Hit: true, MarkID: switchMarkIDTrack, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: switchMarkIDRoot, Cursor: cursor}
}

func (s *Switch) pointInFocusRing(p gfx.Point) bool {
	bounds := s.Layout.ArrangedBounds
	if bounds.IsEmpty() || !bounds.Contains(p) {
		return false
	}
	ring := maxFloat(1, s.cachedRowGap*0.5)
	inner := bounds.Inset(ring, ring)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (s *Switch) cursorShape() facet.CursorShape {
	if s.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (s *Switch) onPointer(e facet.PointerEvent) bool {
	if s.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		s.hovered = true
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		s.hovered = false
		if !s.pressed {
			s.focusFromPointer = false
		}
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		s.hovered = true
		s.pressed = true
		s.focusFromPointer = true
		s.focusedVisible = false
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		wasPressed := s.pressed
		s.pressed = false
		s.invalidate(facet.DirtyProjection)
		if wasPressed && s.Layout.ArrangedBounds.Contains(e.Position) {
			s.SetChecked(!s.isChecked())
			return true
		}
		return wasPressed
	case platform.PointerMove:
		return s.hovered
	default:
		return false
	}
}

func (s *Switch) onKey(e facet.KeyEvent) bool {
	if s.Disabled.Get() {
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
			if wasPressed {
				s.SetChecked(!s.isChecked())
			}
			return wasPressed
		}
	}
	return false
}

func (s *Switch) onFocusGained() {
	s.focusedVisible = !s.focusFromPointer
	s.focusFromPointer = false
	s.invalidate(facet.DirtyProjection)
}

func (s *Switch) onFocusLost() {
	s.focusedVisible = false
	s.pressed = false
	s.focusFromPointer = false
	s.invalidate(facet.DirtyProjection)
}

func (s *Switch) interactionState() theme.InteractionState {
	switch {
	case s.Disabled.Get():
		return theme.StateDisabled
	case s.pressed:
		return theme.StatePressed
	case s.hovered:
		return theme.StateHover
	case s.focusedVisible:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func (s *Switch) selectedState() theme.InteractionState {
	if s.isChecked() {
		return theme.StateSelected
	}
	return s.interactionState()
}

func (s *Switch) labelState() theme.InteractionState {
	if s.Disabled.Get() {
		return theme.StateDisabled
	}
	return theme.StateDefault
}

// SetChecked updates the canonical switch value.
func (s *Switch) SetChecked(checked bool) {
	if s == nil {
		return
	}
	if s.Value == nil {
		s.Value = store.NewValueStore[bool](checked)
		s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	if s.Value.Get() == checked {
		return
	}
	s.Value.Set(checked)
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

func (s *Switch) isChecked() bool {
	if s == nil || s.Value == nil {
		return false
	}
	return s.Value.Get()
}

func (s *Switch) newShaper(runtime any) *text.Shaper {
	registry := s.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (s *Switch) fontRegistry(runtime any) *text.FontRegistry {
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

func switchControlWidth(resolved theme.ResolvedContext) float32 {
	width := resolved.Density.Scale(44)
	if width < 36 {
		width = 36
	}
	return width
}

func switchControlHeight(resolved theme.ResolvedContext) float32 {
	height := resolved.Density.Scale(24)
	if height < 20 {
		height = 20
	}
	return height
}

func switchThumbSize(resolved theme.ResolvedContext) float32 {
	size := resolved.Density.Scale(18)
	if size < 16 {
		size = 16
	}
	return size
}

func switchDefaultMaxWidth(resolved theme.ResolvedContext) float32 {
	width := resolved.Density.Scale(320)
	if width < 240 {
		width = 240
	}
	return width
}

func resolvedTouchHeight(height float32, s *Switch) float32 {
	if s == nil {
		return height
	}
	if height <= 0 {
		return s.cachedControlHeight
	}
	return height
}

type switchGroupPolicy struct{}

func (switchGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearVertical }

func (switchGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}

func (switchGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}


