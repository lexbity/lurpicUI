package status

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistatus"
)

const (
	statusLightMarkIDRoot      facet.MarkID = 1
	statusLightMarkIDIndicator facet.MarkID = 2
	statusLightMarkIDLabel     facet.MarkID = 3
)

// StatusLight implements the status.status_light canonical mark.
type StatusLight struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole

	Label     string
	ShowLabel bool
	Disabled  bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.StatusLightSlots
	cachedBounds           gfx.Rect
	cachedIndicatorBounds  gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedIndicatorSize    float32
	cachedShowLabel        bool
	cachedWritingDirection facet.WritingDirection
	cachedLabelFacet       *primitive.Text
}

var _ facet.FacetImpl = (*StatusLight)(nil)
var _ layout.AnchorExporter = (*StatusLight)(nil)

// NewStatusLight constructs a status.status_light mark with canonical defaults.
func NewStatusLight(label string) *StatusLight {
	s := &StatusLight{
		Facet:     facet.NewFacet(),
		Label:     label,
		ShowLabel: true,
	}
	s.layoutRole.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	s.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := s.measure(ctx, constraints).Size
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
		cmds := s.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	s.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := s.buildCommands(s.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	s.AddRole(&s.layoutRole)
	s.AddRole(&s.renderRole)
	s.AddRole(&s.projectionRole)
	s.syncChildren()
	return s
}

// Base satisfies facet.FacetImpl.
func (s *StatusLight) Base() *facet.Facet {
	s.Facet.BindImpl(s)
	return &s.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (s *StatusLight) AccessibilityRole() string { return "status" }

// AccessibleName reports the semantic name source required by the spec.
func (s *StatusLight) AccessibleName() string { return "" }

// SetLabel updates the authored label text.
func (s *StatusLight) SetLabel(label string) {
	if s == nil || s.Label == label {
		return
	}
	s.Label = label
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetShowLabel toggles label visibility.
func (s *StatusLight) SetShowLabel(show bool) {
	if s == nil || s.ShowLabel == show {
		return
	}
	s.ShowLabel = show
	s.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (s *StatusLight) SetDisabled(disabled bool) {
	if s == nil || s.Disabled == disabled {
		return
	}
	s.Disabled = disabled
	s.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the status-light anchor set.
func (s *StatusLight) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
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
	if !s.cachedIndicatorBounds.IsEmpty() {
		out["indicator"] = gfx.Point{X: (s.cachedIndicatorBounds.Min.X + s.cachedIndicatorBounds.Max.X) * 0.5, Y: (s.cachedIndicatorBounds.Min.Y + s.cachedIndicatorBounds.Max.Y) * 0.5}
	}
	if !s.cachedLabelBounds.IsEmpty() {
		out["label_optional"] = gfx.Point{X: (s.cachedLabelBounds.Min.X + s.cachedLabelBounds.Max.X) * 0.5, Y: (s.cachedLabelBounds.Min.Y + s.cachedLabelBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach subscribes to any attached store.
func (s *StatusLight) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (s *StatusLight) OnActivate() {}

// OnDeactivate is unused.
func (s *StatusLight) OnDeactivate() {}

// OnDetach clears cached projection state.
func (s *StatusLight) OnDetach() {
	s.cachedTokens = theme.Tokens{}
	s.cachedRecipe = shared.StatusLightSlots{}
	s.cachedBounds = gfx.Rect{}
	s.cachedIndicatorBounds = gfx.Rect{}
	s.cachedLabelBounds = gfx.Rect{}
	s.cachedPadX = 0
	s.cachedPadY = 0
	s.cachedGap = 0
	s.cachedIndicatorSize = 0
	s.cachedShowLabel = false
	s.cachedWritingDirection = facet.WritingDirectionLTR
	s.cachedLabelFacet = nil
}

func (s *StatusLight) invalidate(flags facet.DirtyFlags) {
	if s == nil {
		return
	}
	s.Facet.Invalidate(flags)
}

func (s *StatusLight) syncChildren() {
	if s == nil {
		return
	}
	label := strings.TrimSpace(s.Label)
	showLabel := s.cachedShowLabel && label != ""
	if showLabel {
		if s.cachedLabelFacet == nil {
			s.cachedLabelFacet = primitive.NewText(label)
		} else {
			s.cachedLabelFacet.SetContent(label)
		}
		s.cachedLabelFacet.SetTypography(theme.TextLabelS)
		s.cachedLabelFacet.SetOverflow(primitive.TextOverflowTruncate)
		if s.Disabled {
			s.cachedLabelFacet.SetForeground(theme.ColorTextDisabled)
			s.cachedLabelFacet.SetDisabled(true)
		} else {
			s.cachedLabelFacet.SetForeground(theme.ColorTextSecondary)
			s.cachedLabelFacet.SetDisabled(false)
		}
	} else {
		s.cachedLabelFacet = nil
	}
}

func (s *StatusLight) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistatus.ResolveStatusLightRecipe(style, s.statusLightVariant())
	s.cachedTokens = resolved.TokenSet()
	s.cachedRecipe = slots
	s.cachedWritingDirection = ctx.WritingDirection
	s.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(8))
	s.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	s.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	s.cachedIndicatorSize = maxFloat(resolved.Density.Scale(10), float32(resolved.Spacing(theme.SpacingM))*0.75)
	s.cachedShowLabel = s.ShowLabel && resolved.Density.ID != theme.DensityIDCompact
	s.syncChildren()
	innerWidth := constraints.MaxSize.W
	if innerWidth > 0 {
		innerWidth = maxFloat(0, innerWidth-s.cachedPadX*2)
	}
	labelBounds := gfx.Rect{}
	if s.cachedLabelFacet != nil {
		if size := s.cachedLabelFacet.Base().LayoutRole().Measure(facet.MeasureContext{
			Runtime:          ctx.Runtime,
			Theme:            ctx.Theme,
			ContentScale:     ctx.ContentScale,
			Density:          ctx.Density,
			WritingDirection: ctx.WritingDirection,
		}, facet.Constraints{MaxSize: gfx.Size{W: innerWidth, H: constraints.MaxSize.H}}).Size; size != (gfx.Size{}) {
			labelBounds = gfx.RectFromXYWH(0, 0, size.W, size.H)
		}
	}
	width := s.cachedIndicatorSize
	height := s.cachedIndicatorSize
	if !labelBounds.IsEmpty() {
		width += s.cachedGap + labelBounds.Width()
		if labelBounds.Height() > height {
			height = labelBounds.Height()
		}
	}
	width += s.cachedPadX * 2
	height += s.cachedPadY * 2
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
	s.layoutRole.MeasuredSize = measured
	s.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return s.layoutRole.MeasuredResult
}

func (s *StatusLight) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	s.cachedBounds = bounds
	s.cachedIndicatorBounds = gfx.Rect{}
	s.cachedLabelBounds = gfx.Rect{}
	s.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	s.syncChildren()
	inner := bounds.Inset(s.cachedPadX, s.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	labelBounds := gfx.Rect{}
	if s.cachedLabelFacet != nil {
		if size := s.cachedLabelFacet.Base().LayoutRole().MeasuredSize; size != (gfx.Size{}) {
			labelBounds = gfx.RectFromXYWH(0, 0, size.W, size.H)
		}
	}
	rects := layout.ArrangeInlineFlow(inner, 0, s.cachedGap, []gfx.Size{
		{W: s.cachedIndicatorSize, H: s.cachedIndicatorSize},
		{W: labelBounds.Width(), H: labelBounds.Height()},
	}, s.cachedWritingDirection == facet.WritingDirectionRTL)
	s.cachedIndicatorBounds = rects[0]
	if s.cachedLabelFacet != nil {
		s.cachedLabelBounds = rects[1]
	}
	if s.cachedLabelFacet != nil {
		s.cachedLabelFacet.Base().LayoutRole().ArrangedBounds = s.cachedLabelBounds
	}
}

func (s *StatusLight) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if s == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := s.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if s.Disabled {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	indicator := slots.Indicator.Resolve(state, tokens)
	_ = slots.LabelOptional
	cmds := make([]gfx.Command, 0, 16)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(indicator) {
		shape := gfx.CirclePath(gfx.Point{X: s.cachedIndicatorBounds.Min.X + s.cachedIndicatorBounds.Width()*0.5, Y: s.cachedIndicatorBounds.Min.Y + s.cachedIndicatorBounds.Height()*0.5}, s.cachedIndicatorBounds.Width()*0.5)
		cmds = append(cmds, materialCommands(shape, indicator)...)
	}
	if s.cachedLabelFacet != nil && !s.cachedLabelBounds.IsEmpty() {
		if projected := s.cachedLabelFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       s.cachedLabelBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	return cmds
}

func (s *StatusLight) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.StatusLightSlots) {
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
			slots, _ := uistatus.ResolveStatusLightRecipe(style, s.statusLightVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: s.cachedTokens}, s.cachedRecipe
}

func (s *StatusLight) statusLightVariant() uistatus.StatusLightVariant {
	if s != nil && s.Disabled {
		return uistatus.StatusLightDisabled
	}
	return uistatus.StatusLightDefault
}
