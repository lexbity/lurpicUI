package action

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiaction"
)

const (
	splitButtonMarkIDRoot                facet.MarkID = 1
	splitButtonMarkIDPrimaryButton       facet.MarkID = 2
	splitButtonMarkIDPrimaryLabel        facet.MarkID = 3
	splitButtonMarkIDMenuTrigger         facet.MarkID = 4
	splitButtonMarkIDChevron             facet.MarkID = 5
	splitButtonMarkIDFloatingMenuSurface facet.MarkID = 6
	splitButtonMarkIDMenuItems           facet.MarkID = 7
	splitButtonMarkIDFocusRing           facet.MarkID = 8
)

// SplitButtonItem describes one secondary command in the split-button menu.
type SplitButtonItem struct {
	Key             string
	Label           string
	AccessibleLabel string
	IconRef         string
	Disabled        bool
}

// SplitButton implements the action.split_button standard mark.
type SplitButton struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole

	Activated signal.Signal[string]

	Key            string
	Label          string
	PrimaryIconRef string
	Items          []SplitButtonItem
	Disabled       bool
	Open           bool

	hoveredPrimary   bool
	hoveredTrigger   bool
	pressedPrimary   bool
	pressedTrigger   bool
	focusedVisible   bool
	focusFromPointer bool
	focusedIndex     int
	hoveredIndex     int
	pressedIndex     int

	cachedTokens           theme.Tokens
	cachedRecipe           shared.SplitButtonSlots
	cachedRootBounds       gfx.Rect
	cachedControlBounds    gfx.Rect
	cachedPrimaryBounds    gfx.Rect
	cachedPrimaryLabel     gfx.Rect
	cachedPrimaryIcon      gfx.Rect
	cachedTriggerBounds    gfx.Rect
	cachedChevronBounds    gfx.Rect
	cachedMenuBounds       gfx.Rect
	cachedFocusBounds      gfx.Rect
	cachedItemLayouts      []splitButtonItemLayout
	cachedPrimaryLayout    *text.TextLayout
	cachedPrimaryStyle     text.TextStyle
	cachedItemStyle        text.TextStyle
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRowGap           float32
	cachedRadius           float32
	cachedWritingDirection facet.WritingDirection
	cachedPrimaryHeight    float32
	cachedControlHeight    float32
	cachedControlWidth     float32
	cachedTriggerWidth     float32
	cachedMenuWidth        float32
	cachedMenuHeight       float32
	cachedPrimaryIconSize  float32
	cachedChevronSize      float32
	cachedMenuIconSize     float32
}

type splitButtonItemLayout struct {
	item        SplitButtonItem
	labelLayout *text.TextLayout
	bounds      gfx.Rect
	labelBounds gfx.Rect
	iconBounds  gfx.Rect
	width       float32
	height      float32
}

var _ facet.FacetImpl = (*SplitButton)(nil)
var _ layout.AnchorExporter = (*SplitButton)(nil)

// NewSplitButton constructs an action.split_button mark with canonical defaults.
func NewSplitButton(label string, items []SplitButtonItem) *SplitButton {
	s := &SplitButton{
		Facet:        facet.NewFacet(),
		Key:          strings.TrimSpace(label),
		Label:        label,
		Items:        normalizeSplitButtonItems(items),
		focusedIndex: -1,
		hoveredIndex: -1,
		pressedIndex: -1,
		Activated:    signal.NewSignal[string]("split_button_activated"),
	}
	s.layoutRole.Parent = facet.GroupParentContract{
		Kind:   facet.GroupLayoutLinearHorizontal,
		Policy: splitButtonGroupPolicy{},
	}
	s.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsLinear | facet.SupportsGrid | facet.SupportsAnchor,
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
		s.arrange(bounds)
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
	s.inputRole.OnDismiss = func(e facet.DismissEvent) bool {
		_ = e
		if s.Disabled || !s.Open {
			return false
		}
		s.SetOpen(false)
		return true
	}
	s.focusRole.Focusable = func() bool {
		return !s.Disabled && (strings.TrimSpace(s.Label) != "" || len(s.Items) > 0)
	}
	s.focusRole.TabIndex = 0
	s.focusRole.OnFocusGained = func() { s.onFocusGained() }
	s.focusRole.OnFocusLost = func() { s.onFocusLost() }
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
func (s *SplitButton) Base() *facet.Facet {
	s.Facet.BindImpl(s)
	return &s.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (s *SplitButton) AccessibilityRole() string { return "split_button" }

// AccessibleName reports the semantic name required by the spec.
func (s *SplitButton) AccessibleName() string {
	if s == nil {
		return ""
	}
	return strings.TrimSpace(s.Label)
}

// SetLabel updates the authored label text.
func (s *SplitButton) SetLabel(label string) {
	if s == nil || s.Label == label {
		return
	}
	s.Label = label
	if s.Key == "" {
		s.Key = strings.TrimSpace(label)
	}
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetKey updates the authored activation key for the primary action.
func (s *SplitButton) SetKey(key string) {
	if s == nil || s.Key == key {
		return
	}
	s.Key = strings.TrimSpace(key)
	s.invalidate(facet.DirtyProjection)
}

// SetPrimaryIconRef updates the authored primary icon reference.
func (s *SplitButton) SetPrimaryIconRef(ref string) {
	if s == nil || s.PrimaryIconRef == ref {
		return
	}
	s.PrimaryIconRef = strings.TrimSpace(ref)
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetItems replaces the secondary commands.
func (s *SplitButton) SetItems(items []SplitButtonItem) {
	if s == nil {
		return
	}
	s.Items = normalizeSplitButtonItems(items)
	s.syncFocusIndex()
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetOpen updates the open state.
func (s *SplitButton) SetOpen(open bool) {
	if s == nil || s.Open == open {
		return
	}
	s.Open = open
	if open {
		s.syncFocusIndex()
	} else {
		s.pressedPrimary = false
		s.pressedTrigger = false
		s.hoveredIndex = -1
		s.pressedIndex = -1
	}
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
}

// SetDisabled toggles disabled state.
func (s *SplitButton) SetDisabled(disabled bool) {
	if s == nil || s.Disabled == disabled {
		return
	}
	s.Disabled = disabled
	if disabled {
		s.hoveredPrimary = false
		s.hoveredTrigger = false
		s.pressedPrimary = false
		s.pressedTrigger = false
		s.focusedVisible = false
		s.focusFromPointer = false
		s.hoveredIndex = -1
		s.pressedIndex = -1
		s.Open = false
	}
	s.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// ExportAnchors publishes the split button anchor set.
func (s *SplitButton) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
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
	if !s.cachedPrimaryBounds.IsEmpty() {
		out["content_anchor"] = gfx.Point{
			X: s.cachedPrimaryBounds.Min.X + s.cachedPrimaryBounds.Width()*0.5,
			Y: s.cachedPrimaryBounds.Min.Y + s.cachedPrimaryBounds.Height()*0.5,
		}
	} else {
		out["content_anchor"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	if s.cachedPrimaryLayout != nil {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: s.cachedPrimaryLabel.Min.Y + s.cachedPrimaryLayout.Baseline}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

// Children returns the facet's immediate child list.
func (s *SplitButton) Children() []facet.GroupChild { return nil }

// OnAttach is unused.
func (s *SplitButton) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (s *SplitButton) OnActivate() {}

// OnDeactivate is unused.
func (s *SplitButton) OnDeactivate() {}

// OnDetach clears cached projection state.
func (s *SplitButton) OnDetach() {
	s.cachedTokens = theme.Tokens{}
	s.cachedRecipe = shared.SplitButtonSlots{}
	s.cachedRootBounds = gfx.Rect{}
	s.cachedControlBounds = gfx.Rect{}
	s.cachedPrimaryBounds = gfx.Rect{}
	s.cachedPrimaryLabel = gfx.Rect{}
	s.cachedPrimaryIcon = gfx.Rect{}
	s.cachedTriggerBounds = gfx.Rect{}
	s.cachedChevronBounds = gfx.Rect{}
	s.cachedMenuBounds = gfx.Rect{}
	s.cachedFocusBounds = gfx.Rect{}
	s.cachedItemLayouts = nil
	s.cachedPrimaryLayout = nil
	s.cachedPrimaryStyle = text.TextStyle{}
	s.cachedItemStyle = text.TextStyle{}
	s.cachedPadX = 0
	s.cachedPadY = 0
	s.cachedGap = 0
	s.cachedRowGap = 0
	s.cachedRadius = 0
	s.cachedPrimaryHeight = 0
	s.cachedControlHeight = 0
	s.cachedControlWidth = 0
	s.cachedTriggerWidth = 0
	s.cachedMenuWidth = 0
	s.cachedMenuHeight = 0
	s.cachedPrimaryIconSize = 0
	s.cachedChevronSize = 0
	s.cachedMenuIconSize = 0
}

func (s *SplitButton) invalidate(flags facet.DirtyFlags) {
	if s == nil {
		return
	}
	s.Facet.Invalidate(flags)
}

func (s *SplitButton) resolveTheme(ctx facet.MeasureContext) (theme.ResolvedContext, shared.SplitButtonSlots, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{
		Tokens:    resolved.TokenSet(),
		Materials: resolved.Materials,
		Depth:     resolved.Depth,
	}
	slots, _ := uiaction.ResolveSplitButtonRecipe(style)
	return resolved, slots, true
}

func (s *SplitButton) resolveProjectionTheme(runtime any) shared.SplitButtonSlots {
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, s.Base().ID()); store != nil {
			slots, _ := uiaction.ResolveSplitButtonRecipe(store.Get())
			return slots
		}
	}
	return s.cachedRecipe
}

func (s *SplitButton) newShaper(runtime any) *text.Shaper {
	registry := s.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (s *SplitButton) fontRegistry(runtime any) *text.FontRegistry {
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

func (s *SplitButton) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, recipe, ok := s.resolveTheme(ctx)
	if !ok {
		s.cachedPrimaryLayout = nil
		s.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	s.cachedTokens = resolved.TokenSet()
	s.cachedRecipe = recipe
	s.cachedWritingDirection = ctx.WritingDirection
	s.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	s.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	s.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	s.cachedRowGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(6))
	s.cachedRadius = float32(resolved.Radius(theme.RadiusM))
	s.cachedPrimaryHeight = maxFloat(resolved.Density.Scale(36), resolved.Density.Scale(32))
	s.cachedControlHeight = maxFloat(s.cachedPrimaryHeight, resolved.Density.Scale(40))
	s.cachedPrimaryIconSize = maxFloat(resolved.Density.Scale(18), 14)
	s.cachedChevronSize = maxFloat(resolved.Density.Scale(14), 10)
	s.cachedMenuIconSize = maxFloat(resolved.Density.Scale(16), 12)

	primaryStyle := resolved.TextStyle(theme.TextLabelM)
	itemStyle := resolved.TextStyle(theme.TextBodyM)
	s.cachedPrimaryStyle = primaryStyle
	s.cachedItemStyle = itemStyle
	maxWidth := constraints.MaxSize.W
	if maxWidth <= 0 {
		maxWidth = resolved.Density.Scale(420)
	}
	shaper := s.newShaper(ctx.Runtime)
	var primaryLayout *text.TextLayout
	if shaper != nil && strings.TrimSpace(s.Label) != "" {
		shaper.SetContentScale(ctx.ContentScale)
		primaryLayout = shaper.ShapeTruncated(strings.TrimSpace(s.Label), primaryStyle, maxWidth)
	}
	s.cachedPrimaryLayout = primaryLayout
	s.textRole.Layout = primaryLayout
	s.textRole.Selection = text.TextRange{}
	s.textRole.CaretVisible = false
	s.textRole.CaretPosition = text.TextPosition{}
	if primaryLayout != nil {
		s.cachedPrimaryLabel = gfx.RectFromXYWH(0, 0, primaryLayout.Bounds.Width(), primaryLayout.Bounds.Height())
	} else {
		s.cachedPrimaryLabel = gfx.Rect{}
	}

	layouts := make([]splitButtonItemLayout, len(s.Items))
	maxItemW := float32(0)
	totalMenuH := float32(0)
	for i := range s.Items {
		item := s.Items[i]
		layouts[i].item = item
		label := strings.TrimSpace(item.Label)
		if shaper != nil && label != "" {
			layouts[i].labelLayout = shaper.ShapeTruncated(label, itemStyle, maxWidth)
		}
		leadW := float32(0)
		if strings.TrimSpace(item.IconRef) != "" {
			leadW = s.cachedMenuIconSize + s.cachedGap
		}
		labelW := text.Width(layouts[i].labelLayout)
		layouts[i].width = maxFloat(resolved.Density.Scale(192), s.cachedPadX*2+leadW+labelW)
		layouts[i].height = maxFloat(resolved.Density.Scale(30), text.Height(layouts[i].labelLayout))
		if layouts[i].height < s.cachedMenuIconSize+s.cachedPadY {
			layouts[i].height = s.cachedMenuIconSize + s.cachedPadY
		}
		if layouts[i].width > maxItemW {
			maxItemW = layouts[i].width
		}
		totalMenuH += layouts[i].height
	}
	if len(layouts) > 1 {
		totalMenuH += s.cachedRowGap * float32(len(layouts)-1)
	}
	s.cachedItemLayouts = layouts

	primaryW := s.cachedPadX * 2
	if strings.TrimSpace(s.PrimaryIconRef) != "" {
		primaryW += s.cachedPrimaryIconSize + s.cachedGap
	}
	primaryW += text.Width(primaryLayout)
	if text.Width(primaryLayout) > 0 {
		primaryW += s.cachedGap
	}
	primaryW = maxFloat(primaryW, resolved.Density.Scale(96))
	primaryH := maxFloat(s.cachedControlHeight, maxFloat(text.Height(primaryLayout), s.cachedPrimaryIconSize))
	primaryH += s.cachedPadY * 2
	s.cachedControlHeight = primaryH
	triggerW := maxFloat(resolved.Density.Scale(44), s.cachedChevronSize+s.cachedPadX*2)
	triggerH := primaryH
	s.cachedControlWidth = primaryW + triggerW
	s.cachedTriggerWidth = triggerW
	menuW := maxFloat(s.cachedControlWidth, maxItemW)
	if len(layouts) > 0 {
		menuW = maxFloat(menuW, resolved.Density.Scale(192))
	}
	s.cachedMenuWidth = menuW
	s.cachedMenuHeight = 0
	if s.Open && len(layouts) > 0 {
		s.cachedMenuHeight = totalMenuH
	}
	if s.Open {
		s.syncFocusIndex()
	}

	size := gfx.Size{
		W: maxFloat(s.cachedControlWidth, s.cachedMenuWidth) + s.cachedPadX*2,
		H: s.cachedPadY*2 + triggerH,
	}
	if s.Open && len(layouts) > 0 {
		size.H += s.cachedGap + s.cachedMenuHeight
	}
	size = constraints.Constrain(size)
	s.layoutRole.MeasuredSize = size
	s.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return s.layoutRole.MeasuredResult
}

func (s *SplitButton) measureIntrinsic(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	return s.measure(ctx, constraints).Size
}

func (s *SplitButton) arrange(bounds gfx.Rect) {
	s.cachedRootBounds = bounds
	s.cachedControlBounds = gfx.Rect{}
	s.cachedPrimaryBounds = gfx.Rect{}
	s.cachedPrimaryLabel = gfx.Rect{}
	s.cachedPrimaryIcon = gfx.Rect{}
	s.cachedTriggerBounds = gfx.Rect{}
	s.cachedChevronBounds = gfx.Rect{}
	s.cachedMenuBounds = gfx.Rect{}
	s.cachedFocusBounds = gfx.Rect{}
	s.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	inner := bounds.Inset(s.cachedPadX, s.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	rtl := s.cachedWritingDirection == facet.WritingDirectionRTL
	controlW := s.cachedControlWidth
	if controlW <= 0 {
		controlW = bounds.Width() - s.cachedPadX*2
	}
	controlH := s.cachedControlHeight
	if controlH <= 0 {
		controlH = maxFloat(bounds.Height(), s.cachedPrimaryHeight)
	}
	controlY := inner.Min.Y
	controlX := inner.Min.X
	if rtl {
		controlX = inner.Max.X - controlW
	}
	s.cachedControlBounds = gfx.RectFromXYWH(controlX, controlY, controlW, controlH)
	primaryW := controlW - s.cachedTriggerWidth
	primaryH := controlH
	var primaryBounds, triggerBounds gfx.Rect
	if rtl {
		triggerBounds = gfx.RectFromXYWH(controlX, controlY, s.cachedTriggerWidth, controlH)
		primaryBounds = gfx.RectFromXYWH(controlX+s.cachedTriggerWidth, controlY, primaryW, primaryH)
	} else {
		primaryBounds = gfx.RectFromXYWH(controlX, controlY, primaryW, primaryH)
		triggerBounds = gfx.RectFromXYWH(controlX+primaryW, controlY, s.cachedTriggerWidth, primaryH)
	}
	s.cachedPrimaryBounds = primaryBounds
	s.cachedTriggerBounds = triggerBounds
	if s.cachedPrimaryLayout != nil {
		textH := text.Height(s.cachedPrimaryLayout)
		contentY := text.CenterY(primaryBounds, textH)
		iconY := text.CenterY(primaryBounds, s.cachedPrimaryIconSize)
		if rtl {
			x := primaryBounds.Max.X - s.cachedPadX
			labelW := s.cachedPrimaryLayout.Bounds.Width()
			x -= labelW
			s.cachedPrimaryLabel = gfx.RectFromXYWH(x, contentY, labelW, textH)
			x -= s.cachedGap
			if strings.TrimSpace(s.PrimaryIconRef) != "" {
				x -= s.cachedPrimaryIconSize
				s.cachedPrimaryIcon = gfx.RectFromXYWH(x, iconY, s.cachedPrimaryIconSize, s.cachedPrimaryIconSize)
			}
		} else {
			x := primaryBounds.Min.X + s.cachedPadX
			if strings.TrimSpace(s.PrimaryIconRef) != "" {
				s.cachedPrimaryIcon = gfx.RectFromXYWH(x, iconY, s.cachedPrimaryIconSize, s.cachedPrimaryIconSize)
				x += s.cachedPrimaryIconSize + s.cachedGap
			}
			labelW := s.cachedPrimaryLayout.Bounds.Width()
			s.cachedPrimaryLabel = gfx.RectFromXYWH(x, contentY, labelW, textH)
		}
	} else if strings.TrimSpace(s.PrimaryIconRef) != "" {
		iconY := text.CenterY(primaryBounds, s.cachedPrimaryIconSize)
		if rtl {
			s.cachedPrimaryIcon = gfx.RectFromXYWH(primaryBounds.Max.X-s.cachedPadX-s.cachedPrimaryIconSize, iconY, s.cachedPrimaryIconSize, s.cachedPrimaryIconSize)
		} else {
			s.cachedPrimaryIcon = gfx.RectFromXYWH(primaryBounds.Min.X+s.cachedPadX, iconY, s.cachedPrimaryIconSize, s.cachedPrimaryIconSize)
		}
	}
	chevronX := triggerBounds.Min.X + maxFloat(0, (triggerBounds.Width()-s.cachedChevronSize)*0.5)
	if rtl {
		chevronX = triggerBounds.Min.X + maxFloat(0, (triggerBounds.Width()-s.cachedChevronSize)*0.5)
	}
	s.cachedChevronBounds = text.CenterRect(gfx.RectFromXYWH(chevronX, triggerBounds.Min.Y, s.cachedChevronSize, triggerBounds.Height()), s.cachedChevronSize, s.cachedChevronSize)

	menuY := triggerBounds.Max.Y + s.cachedGap
	if s.Open && len(s.cachedItemLayouts) > 0 {
		menuW := s.cachedMenuWidth
		if menuW <= 0 {
			menuW = maxFloat(controlW, s.cachedControlBounds.Width())
		}
		menuH := s.cachedMenuHeight
		if menuH <= 0 {
			menuH = sumSplitButtonItemHeights(s.cachedItemLayouts, s.cachedRowGap)
		}
		if rtl {
			s.cachedMenuBounds = gfx.RectFromXYWH(triggerBounds.Max.X-menuW, menuY, menuW, menuH)
		} else {
			s.cachedMenuBounds = gfx.RectFromXYWH(triggerBounds.Min.X, menuY, menuW, menuH)
		}
		rowY := s.cachedMenuBounds.Min.Y
		for i := range s.cachedItemLayouts {
			entry := &s.cachedItemLayouts[i]
			entry.bounds = gfx.RectFromXYWH(s.cachedMenuBounds.Min.X, rowY, s.cachedMenuBounds.Width(), entry.height)
			labelH := text.Height(entry.labelLayout)
			labelW := text.Width(entry.labelLayout)
			leadX := entry.bounds.Min.X + s.cachedPadX
			if rtl {
				leadX = entry.bounds.Max.X - s.cachedPadX
				if entry.item.IconRef != "" {
					leadX -= s.cachedMenuIconSize
					entry.iconBounds = text.CenterRect(gfx.RectFromXYWH(leadX, rowY, s.cachedMenuIconSize, entry.height), s.cachedMenuIconSize, s.cachedMenuIconSize)
					leadX -= s.cachedGap
				}
				entry.labelBounds = gfx.RectFromXYWH(leadX-labelW, text.CenterY(gfx.RectFromXYWH(s.cachedMenuBounds.Min.X, rowY, s.cachedMenuBounds.Width(), entry.height), labelH), labelW, labelH)
			} else {
				if entry.item.IconRef != "" {
					entry.iconBounds = text.CenterRect(gfx.RectFromXYWH(leadX, rowY, s.cachedMenuIconSize, entry.height), s.cachedMenuIconSize, s.cachedMenuIconSize)
					leadX += s.cachedMenuIconSize + s.cachedGap
				}
				entry.labelBounds = gfx.RectFromXYWH(leadX, text.CenterY(gfx.RectFromXYWH(s.cachedMenuBounds.Min.X, rowY, s.cachedMenuBounds.Width(), entry.height), labelH), labelW, labelH)
			}
			rowY += entry.height + s.cachedRowGap
		}
	}
	s.cachedFocusBounds = s.cachedControlBounds.Inset(maxFloat(1, s.cachedControlBounds.Height()*0.08), maxFloat(1, s.cachedControlBounds.Height()*0.08))
}

func (s *SplitButton) buildCommands(bounds gfx.Rect, runtime any) []gfx.Command {
	if s == nil || bounds.IsEmpty() {
		return nil
	}
	slots := s.resolveProjectionTheme(runtime)
	tokens := s.cachedTokens
	if runtime != nil {
		if store := theme.NearestStyleContext(runtime, s.Base().ID()); store != nil {
			tokens = store.Get().Tokens
		}
	}
	state := s.interactionState()
	root := slots.Root.Resolve(state, tokens)
	primary := slots.PrimaryButton.Resolve(state, tokens)
	primaryLabel := slots.PrimaryLabel.Resolve(state, tokens)
	trigger := slots.MenuTrigger.Resolve(state, tokens)
	chevron := slots.Chevron.Resolve(state, tokens)
	menuSurface := slots.FloatingMenuSurface.Resolve(state, tokens)
	menuItems := slots.MenuItems.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 64)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(primary) {
		cmds = append(cmds, materialCommands(splitButtonLeftSegmentPath(s.cachedPrimaryBounds, s.cachedRadius), primary)...)
	}
	if !isTransparentMaterial(trigger) {
		cmds = append(cmds, materialCommands(splitButtonRightSegmentPath(s.cachedTriggerBounds, s.cachedRadius), trigger)...)
	}
	if !isTransparentMaterial(primaryLabel) {
		cmds = append(cmds, labelCommands(s.cachedPrimaryLayout, s.cachedPrimaryLabel, primaryLabel)...)
	}
	if !isTransparentMaterial(primaryLabel) && strings.TrimSpace(s.PrimaryIconRef) != "" && !s.cachedPrimaryIcon.IsEmpty() {
		if iconCmds := iconAssetCommands(runtimeServicesOrNil(runtime), s.PrimaryIconRef, s.cachedPrimaryIcon, primaryLabel); len(iconCmds) > 0 {
			cmds = append(cmds, iconCmds...)
		}
	}
	if !isTransparentMaterial(trigger) && !s.cachedPrimaryBounds.IsEmpty() {
		seamX := s.cachedTriggerBounds.Min.X
		if s.cachedWritingDirection == facet.WritingDirectionRTL {
			seamX = s.cachedPrimaryBounds.Min.X
		}
		seam := gfx.RectFromXYWH(seamX-0.5, s.cachedControlBounds.Min.Y, 1, s.cachedControlBounds.Height())
		cmds = append(cmds, materialCommands(gfx.RectPath(seam), theme.MarkStyle{Base: theme.FromToken(tintColor(tokens.Color.OnPrimary, 0.16))}.Resolve(state, tokens))...)
	}
	if !isTransparentMaterial(chevron) {
		cmds = append(cmds, materialCommands(splitButtonChevronPath(s.cachedChevronBounds), chevron)...)
	}
	if s.Open && len(s.cachedItemLayouts) > 0 {
		if !s.cachedMenuBounds.IsEmpty() && !isTransparentMaterial(menuSurface) {
			cmds = append(cmds, materialCommands(gfx.RoundedRectPath(s.cachedMenuBounds, s.cachedRadius), menuSurface)...)
		}
		for i := range s.cachedItemLayouts {
			entry := &s.cachedItemLayouts[i]
			if entry.bounds.IsEmpty() {
				continue
			}
			rowState := s.itemState(i)
			rowMaterial := theme.Material{Opacity: 0}
			switch rowState {
			case theme.StateHover:
				rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.08))
			case theme.StatePressed:
				rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.14))
			case theme.StateFocused:
				rowMaterial = theme.FromToken(tintColor(tokens.Color.Primary, 0.06))
			}
			if !isTransparentMaterial(rowMaterial) {
				cmds = append(cmds, materialCommands(gfx.RoundedRectPath(entry.bounds, maxFloat(0, s.cachedRadius*0.5)), rowMaterial)...)
			}
			if entry.item.IconRef != "" && !entry.iconBounds.IsEmpty() {
				if iconCmds := iconAssetCommands(runtimeServicesOrNil(runtime), entry.item.IconRef, entry.iconBounds, menuItems); len(iconCmds) > 0 {
					cmds = append(cmds, iconCmds...)
				}
			}
			if entry.labelLayout != nil && !isTransparentMaterial(menuItems) {
				cmds = append(cmds, labelCommands(entry.labelLayout, entry.labelBounds, menuItems)...)
			}
		}
	}
	if s.focusedVisible && !isTransparentMaterial(focus) {
		inset := maxFloat(1, s.cachedControlBounds.Height()*0.08)
		ringBounds := s.cachedControlBounds.Inset(-inset, -inset)
		cmds = append(cmds, materialCommands(gfx.RoundedRectPath(ringBounds, s.cachedRadius+inset), focus)...)
	}
	return cmds
}

func (s *SplitButton) hitTest(p gfx.Point) facet.HitResult {
	if s == nil || s.layoutRole.ArrangedBounds.IsEmpty() || !s.layoutRole.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := s.cursorShape()
	if s.focusedVisible && s.pointInFocusRing(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDFocusRing, Cursor: cursor}
	}
	if idx := s.indexAt(p); idx >= 0 {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDMenuItems, Cursor: cursor}
	}
	if s.Open && s.cachedMenuBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDFloatingMenuSurface, Cursor: cursor}
	}
	if s.cachedChevronBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDChevron, Cursor: cursor}
	}
	if s.cachedTriggerBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDMenuTrigger, Cursor: cursor}
	}
	if s.cachedPrimaryLabel.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDPrimaryLabel, Cursor: cursor}
	}
	if s.cachedPrimaryBounds.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDPrimaryButton, Cursor: cursor}
	}
	return facet.HitResult{Hit: true, MarkID: splitButtonMarkIDRoot, Cursor: cursor}
}

func (s *SplitButton) cursorShape() facet.CursorShape {
	if s.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (s *SplitButton) onPointer(e facet.PointerEvent) bool {
	if s.Disabled {
		return false
	}
	idx := s.indexAt(e.Position)
	switch e.Kind {
	case platform.PointerEnter, platform.PointerMove:
		s.hoveredPrimary = s.cachedPrimaryBounds.Contains(e.Position)
		s.hoveredTrigger = s.cachedTriggerBounds.Contains(e.Position)
		if idx != s.hoveredIndex {
			s.hoveredIndex = idx
			s.invalidate(facet.DirtyProjection)
		} else {
			s.invalidate(facet.DirtyProjection)
		}
		return true
	case platform.PointerLeave:
		s.hoveredPrimary = false
		s.hoveredTrigger = false
		s.hoveredIndex = -1
		if !s.pressedPrimary && !s.pressedTrigger {
			s.focusFromPointer = false
		}
		s.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		s.focusFromPointer = true
		s.focusedVisible = false
		s.hoveredPrimary = s.cachedPrimaryBounds.Contains(e.Position)
		s.hoveredTrigger = s.cachedTriggerBounds.Contains(e.Position)
		if s.Open && idx >= 0 && s.entryIsSelectable(idx) {
			s.pressedIndex = idx
			s.invalidate(facet.DirtyProjection)
			return true
		}
		if s.cachedPrimaryBounds.Contains(e.Position) {
			s.pressedPrimary = true
			s.invalidate(facet.DirtyProjection)
			return true
		}
		if s.cachedTriggerBounds.Contains(e.Position) {
			s.pressedTrigger = true
			s.invalidate(facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		if s.Open && idx >= 0 && s.entryIsSelectable(idx) {
			wasPressed := s.pressedIndex == idx
			s.pressedIndex = -1
			s.invalidate(facet.DirtyProjection)
			if wasPressed {
				s.activateItem(idx)
				return true
			}
			return false
		}
		wasPrimary := s.pressedPrimary
		wasTrigger := s.pressedTrigger
		s.pressedPrimary = false
		s.pressedTrigger = false
		s.invalidate(facet.DirtyProjection)
		if wasPrimary && s.cachedPrimaryBounds.Contains(e.Position) {
			s.activatePrimary()
			return true
		}
		if wasTrigger && s.cachedTriggerBounds.Contains(e.Position) {
			s.SetOpen(!s.Open)
			return true
		}
		return wasPrimary || wasTrigger
	default:
		return false
	}
}

func (s *SplitButton) onKey(e facet.KeyEvent) bool {
	if s.Disabled {
		return false
	}
	if s.Open {
		switch e.Key {
		case platform.KeyUp, platform.KeyDown, platform.KeyHome, platform.KeyEnd:
			if e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat {
				s.navigateOpen(e.Key)
				return true
			}
		case platform.KeyEnter, platform.KeySpace:
			if e.Kind == platform.KeyRelease {
				if s.focusedIndex >= 0 {
					s.activateItem(s.focusedIndex)
					return true
				}
			}
			return e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat || e.Kind == platform.KeyRelease
		case platform.KeyEscape:
			if e.Kind == platform.KeyPress {
				s.SetOpen(false)
				return true
			}
		}
	}
	switch e.Key {
	case platform.KeyEnter, platform.KeySpace:
		if e.Kind == platform.KeyRelease {
			s.activatePrimary()
			return true
		}
		return e.Kind == platform.KeyPress || e.Kind == platform.KeyRepeat
	case platform.KeyDown:
		if e.Kind == platform.KeyPress {
			s.SetOpen(true)
			s.focusedIndex = s.firstSelectableIndex()
			s.invalidate(facet.DirtyProjection)
			return true
		}
	case platform.KeyUp:
		if e.Kind == platform.KeyPress {
			s.SetOpen(true)
			s.focusedIndex = s.lastSelectableIndex()
			s.invalidate(facet.DirtyProjection)
			return true
		}
	}
	return false
}

func (s *SplitButton) onFocusGained() {
	s.focusedVisible = !s.focusFromPointer
	s.focusFromPointer = false
	s.invalidate(facet.DirtyProjection)
}

func (s *SplitButton) onFocusLost() {
	s.focusedVisible = false
	s.pressedPrimary = false
	s.pressedTrigger = false
	s.focusFromPointer = false
	s.hoveredIndex = -1
	s.pressedIndex = -1
	s.invalidate(facet.DirtyProjection)
}

func (s *SplitButton) interactionState() theme.InteractionState {
	switch {
	case s.Disabled:
		return theme.StateDisabled
	case s.pressedPrimary || s.pressedTrigger:
		return theme.StatePressed
	case s.hoveredPrimary || s.hoveredTrigger:
		return theme.StateHover
	case s.focusedVisible:
		return theme.StateFocused
	case s.Open:
		return theme.StateSelected
	default:
		return theme.StateDefault
	}
}

func (s *SplitButton) pointInFocusRing(p gfx.Point) bool {
	if !s.cachedControlBounds.Contains(p) {
		return false
	}
	inset := maxFloat(1, s.cachedControlBounds.Height()*0.08)
	inner := s.cachedControlBounds.Inset(inset, inset)
	if inner.IsEmpty() {
		return true
	}
	return !inner.Contains(p)
}

func (s *SplitButton) indexAt(p gfx.Point) int {
	for i := range s.cachedItemLayouts {
		if s.cachedItemLayouts[i].bounds.Contains(p) {
			return i
		}
	}
	return -1
}

func (s *SplitButton) entryIsSelectable(index int) bool {
	if index < 0 || index >= len(s.cachedItemLayouts) {
		return false
	}
	return !s.cachedItemLayouts[index].item.Disabled
}

func (s *SplitButton) activatePrimary() {
	key := s.primaryKey()
	s.Activated.Emit(key)
	s.SetOpen(false)
}

func (s *SplitButton) activateItem(index int) {
	if !s.entryIsSelectable(index) {
		return
	}
	item := s.cachedItemLayouts[index].item
	s.Activated.Emit(splitButtonItemKey(item))
	s.SetOpen(false)
}

func (s *SplitButton) toggleOpen() {
	s.SetOpen(!s.Open)
}

func (s *SplitButton) primaryKey() string {
	if name := strings.TrimSpace(s.Key); name != "" {
		return name
	}
	if name := strings.TrimSpace(s.Label); name != "" {
		return name
	}
	return ""
}

func (s *SplitButton) syncFocusIndex() {
	if !s.Open {
		s.focusedIndex = -1
		return
	}
	if s.focusedIndex >= 0 && s.focusedIndex < len(s.cachedItemLayouts) && s.entryIsSelectable(s.focusedIndex) {
		return
	}
	s.focusedIndex = s.firstSelectableIndex()
}

func (s *SplitButton) firstSelectableIndex() int {
	for i := range s.cachedItemLayouts {
		if s.entryIsSelectable(i) {
			return i
		}
	}
	return -1
}

func (s *SplitButton) lastSelectableIndex() int {
	for i := len(s.cachedItemLayouts) - 1; i >= 0; i-- {
		if s.entryIsSelectable(i) {
			return i
		}
	}
	return -1
}

func (s *SplitButton) navigateOpen(key platform.Key) {
	if len(s.cachedItemLayouts) == 0 {
		return
	}
	if s.focusedIndex < 0 {
		s.focusedIndex = s.firstSelectableIndex()
	}
	switch key {
	case platform.KeyHome:
		s.focusedIndex = s.firstSelectableIndex()
	case platform.KeyEnd:
		s.focusedIndex = s.lastSelectableIndex()
	case platform.KeyUp:
		for i := s.focusedIndex - 1; i >= 0; i-- {
			if s.entryIsSelectable(i) {
				s.focusedIndex = i
				break
			}
		}
	case platform.KeyDown:
		for i := s.focusedIndex + 1; i < len(s.cachedItemLayouts); i++ {
			if s.entryIsSelectable(i) {
				s.focusedIndex = i
				break
			}
		}
	}
	s.invalidate(facet.DirtyProjection)
}

func (s *SplitButton) itemState(index int) theme.InteractionState {
	if index < 0 || index >= len(s.cachedItemLayouts) {
		return theme.StateDefault
	}
	switch {
	case s.cachedItemLayouts[index].item.Disabled:
		return theme.StateDisabled
	case s.pressedIndex == index:
		return theme.StatePressed
	case s.hoveredIndex == index:
		return theme.StateHover
	case s.Open && s.focusedIndex == index:
		return theme.StateFocused
	default:
		return theme.StateDefault
	}
}

func normalizeSplitButtonItems(items []SplitButtonItem) []SplitButtonItem {
	if len(items) == 0 {
		return nil
	}
	out := make([]SplitButtonItem, len(items))
	for i := range items {
		out[i] = normalizeSplitButtonItem(items[i])
	}
	return out
}

func normalizeSplitButtonItem(item SplitButtonItem) SplitButtonItem {
	item.Key = strings.TrimSpace(item.Key)
	item.Label = strings.TrimSpace(item.Label)
	item.AccessibleLabel = strings.TrimSpace(item.AccessibleLabel)
	item.IconRef = strings.TrimSpace(item.IconRef)
	if item.Key == "" {
		switch {
		case item.AccessibleLabel != "":
			item.Key = item.AccessibleLabel
		case item.Label != "":
			item.Key = item.Label
		}
	}
	if item.AccessibleLabel == "" {
		if item.Label != "" {
			item.AccessibleLabel = item.Label
		} else {
			item.AccessibleLabel = item.Key
		}
	}
	return item
}

func splitButtonItemKey(item SplitButtonItem) string {
	if name := strings.TrimSpace(item.Key); name != "" {
		return name
	}
	if name := strings.TrimSpace(item.AccessibleLabel); name != "" {
		return name
	}
	return strings.TrimSpace(item.Label)
}

func splitButtonLeftSegmentPath(bounds gfx.Rect, radius float32) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	if radius <= 0 {
		radius = 0
	}
	maxRadius := minFloat(bounds.Width(), bounds.Height()) * 0.5
	if radius > maxRadius {
		radius = maxRadius
	}
	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X, bounds.Max.Y
	rx := radius
	ry := radius
	return gfx.NewPath().
		MoveTo(gfx.Point{X: minX + rx, Y: minY}).
		LineTo(gfx.Point{X: maxX, Y: minY}).
		LineTo(gfx.Point{X: maxX, Y: maxY}).
		LineTo(gfx.Point{X: minX + rx, Y: maxY}).
		QuadTo(gfx.Point{X: minX, Y: maxY}, gfx.Point{X: minX, Y: maxY - ry}).
		LineTo(gfx.Point{X: minX, Y: minY + ry}).
		QuadTo(gfx.Point{X: minX, Y: minY}, gfx.Point{X: minX + rx, Y: minY}).
		Close().
		Build()
}

func splitButtonRightSegmentPath(bounds gfx.Rect, radius float32) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	if radius <= 0 {
		radius = 0
	}
	maxRadius := minFloat(bounds.Width(), bounds.Height()) * 0.5
	if radius > maxRadius {
		radius = maxRadius
	}
	minX, minY := bounds.Min.X, bounds.Min.Y
	maxX, maxY := bounds.Max.X, bounds.Max.Y
	rx := radius
	ry := radius
	return gfx.NewPath().
		MoveTo(gfx.Point{X: minX, Y: minY}).
		LineTo(gfx.Point{X: maxX - rx, Y: minY}).
		QuadTo(gfx.Point{X: maxX, Y: minY}, gfx.Point{X: maxX, Y: minY + ry}).
		LineTo(gfx.Point{X: maxX, Y: maxY - ry}).
		QuadTo(gfx.Point{X: maxX, Y: maxY}, gfx.Point{X: maxX - rx, Y: maxY}).
		LineTo(gfx.Point{X: minX, Y: maxY}).
		LineTo(gfx.Point{X: minX, Y: minY}).
		Close().
		Build()
}

func splitButtonChevronPath(bounds gfx.Rect) gfx.Path {
	if bounds.IsEmpty() {
		return gfx.Path{}
	}
	return gfx.NewPath().
		MoveTo(gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y + bounds.Height()*0.40}).
		LineTo(gfx.Point{X: bounds.Min.X + bounds.Width()*0.5, Y: bounds.Max.Y - bounds.Height()*0.12}).
		LineTo(gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y + bounds.Height()*0.40}).
		Build()
}

func sumSplitButtonItemHeights(entries []splitButtonItemLayout, gap float32) float32 {
	if len(entries) == 0 {
		return 0
	}
	total := float32(0)
	for i := range entries {
		total += entries[i].height
		if i > 0 {
			total += gap
		}
	}
	return total
}

type splitButtonGroupPolicy struct{}

func (splitButtonGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutLinearHorizontal }
func (splitButtonGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	return facet.GroupMeasureResult{}, nil
}
func (splitButtonGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	return nil, nil
}
