package status

import (
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistatus"
)

const (
	progressBarMarkIDRoot          facet.MarkID = 1
	progressBarMarkIDTrack         facet.MarkID = 2
	progressBarMarkIDIndicator     facet.MarkID = 3
	progressBarMarkIDOptionalLabel facet.MarkID = 4
)

// ProgressBar implements the status.progress_bar canonical mark.
type ProgressBar struct {
	marks.Core

	Label    marks.Binding[string]
	Value    marks.Binding[float32]
	Disabled marks.Binding[bool]

	cachedTokens           theme.Tokens
	cachedRecipe           shared.StatusProgressBarSlots
	cachedBounds           gfx.Rect
	cachedRootRadius       float32
	cachedTrackBounds      gfx.Rect
	cachedTrackRadius      float32
	cachedIndicatorBounds  gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedTrackHeight      float32
	cachedShowLabel        bool
	cachedWritingDirection facet.WritingDirection
	cachedLabelFacet       *primitive.Text

	pulseDuration  time.Duration
	pulseRemaining time.Duration
	pulsePhase     float32

	cachedCommands []gfx.Command
}

var _ facet.FacetImpl = (*ProgressBar)(nil)
var _ layout.AnchorExporter = (*ProgressBar)(nil)
var _ marks.Mark = (*ProgressBar)(nil)

// NewProgressBar constructs a status.progress_bar mark with canonical defaults.
func NewProgressBar(label string) *ProgressBar {
	p := &ProgressBar{
		Label:    marks.Const(label),
		Value:    marks.Const(float32(0)),
		Disabled: marks.Const(false),
	}
	p.Core.Facet = facet.NewFacet()
	p.AddBinding(p.Label)
	p.AddBinding(p.Value)
	p.AddBinding(p.Disabled)

	p.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
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
	p.Tick.OnTick = func(dt time.Duration) {
		p.onTick(dt)
	}
	p.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return p.buildCommands(p.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	p.RegisterRoles()
	p.syncLabelFacet()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *ProgressBar) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

// Descriptor satisfies marks.Mark.
func (p *ProgressBar) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "status", TypeName: "progress_bar"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *ProgressBar) AccessibilityRole() string { return "progressbar" }

// AccessibleName reports the semantic name source required by the spec.
func (p *ProgressBar) AccessibleName() string { return "" }

// ExportAnchors publishes the progress-bar anchor set.
func (p *ProgressBar) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.Layout.ArrangedBounds
	out := p.Core.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	if !p.cachedTrackBounds.IsEmpty() {
		out["track"] = gfx.Point{X: (p.cachedTrackBounds.Min.X + p.cachedTrackBounds.Max.X) * 0.5, Y: (p.cachedTrackBounds.Min.Y + p.cachedTrackBounds.Max.Y) * 0.5}
	}
	if !p.cachedIndicatorBounds.IsEmpty() {
		out["indicator"] = gfx.Point{X: (p.cachedIndicatorBounds.Min.X + p.cachedIndicatorBounds.Max.X) * 0.5, Y: (p.cachedIndicatorBounds.Min.Y + p.cachedIndicatorBounds.Max.Y) * 0.5}
	}
	if !p.cachedLabelBounds.IsEmpty() {
		out["optional_label"] = gfx.Point{X: (p.cachedLabelBounds.Min.X + p.cachedLabelBounds.Max.X) * 0.5, Y: (p.cachedLabelBounds.Min.Y + p.cachedLabelBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach is unused.
func (p *ProgressBar) OnAttach(ctx facet.AttachContext) { p.Core.OnAttach() }

// OnActivate is unused.
func (p *ProgressBar) OnActivate() { p.Core.OnActivate() }

// OnDeactivate is unused.
func (p *ProgressBar) OnDeactivate() { p.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (p *ProgressBar) OnDetach() {
	p.Core.OnDetach()
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.StatusProgressBarSlots{}
	p.cachedBounds = gfx.Rect{}
	p.cachedRootRadius = 0
	p.cachedTrackBounds = gfx.Rect{}
	p.cachedTrackRadius = 0
	p.cachedIndicatorBounds = gfx.Rect{}
	p.cachedLabelBounds = gfx.Rect{}
	p.cachedPadX = 0
	p.cachedPadY = 0
	p.cachedGap = 0
	p.cachedTrackHeight = 0
	p.cachedShowLabel = false
	p.cachedWritingDirection = facet.WritingDirectionLTR
	p.cachedLabelFacet = nil
	p.pulseDuration = 0
	p.pulseRemaining = 0
	p.pulsePhase = 0
	p.cachedCommands = nil
}

func (p *ProgressBar) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Facet.Invalidate(flags)
}

func (p *ProgressBar) syncLabelFacet() {
	if p == nil {
		return
	}
	label := strings.TrimSpace(p.Label.Get())
	if !p.cachedShowLabel || label == "" {
		p.cachedLabelFacet = nil
		return
	}
	if p.cachedLabelFacet == nil {
		p.cachedLabelFacet = primitive.NewText(marks.Const(label))
	} else {
		p.cachedLabelFacet.Content = marks.Const(label)
		p.cachedLabelFacet.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
	p.cachedLabelFacet.Typography = marks.Const(theme.TextLabelM)
	p.cachedLabelFacet.Overflow = marks.Const(primitive.TextOverflowTruncate)
	if p.cachedWritingDirection == facet.WritingDirectionRTL {
		p.cachedLabelFacet.Alignment = marks.Const(text.AlignRight)
	} else {
		p.cachedLabelFacet.Alignment = marks.Const(text.AlignLeft)
	}
	if p.Disabled.Get() {
		p.cachedLabelFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		p.cachedLabelFacet.Disabled = marks.Const(true)
	} else {
		p.cachedLabelFacet.Foreground = marks.Const(theme.ColorText)
		p.cachedLabelFacet.Disabled = marks.Const(false)
	}
}

func (p *ProgressBar) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistatus.ResolveProgressBarRecipe(style, p.progressBarVariant())
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = slots
	p.cachedWritingDirection = ctx.WritingDirection
	p.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(8))
	p.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	p.cachedTrackHeight = maxFloat(float32(resolved.Spacing(theme.SpacingS)), resolved.Density.Scale(8))
	p.cachedRootRadius = maxFloat(float32(resolved.Radius(theme.RadiusM).Float32()), p.cachedTrackHeight*0.75)
	p.cachedTrackRadius = p.cachedTrackHeight * 0.5
	p.cachedShowLabel = strings.TrimSpace(p.Label.Get()) != "" && resolved.Density.ID != theme.DensityIDCompact
	p.syncLabelFacet()

	availableWidth := constraints.MaxSize.W
	if availableWidth > 0 {
		availableWidth = maxFloat(0, availableWidth-p.cachedPadX*2)
	}

	labelSize := gfx.Size{}
	if p.cachedLabelFacet != nil {
		labelCtx := facet.MeasureContext{
			Runtime:          ctx.Runtime,
			Theme:            ctx.Theme,
			ContentScale:     ctx.ContentScale,
			Density:          ctx.Density,
			WritingDirection: ctx.WritingDirection,
		}
		labelConstraints := facet.Constraints{MaxSize: gfx.Size{W: availableWidth, H: constraints.MaxSize.H}}
		if size := p.cachedLabelFacet.Base().LayoutRole().Measure(labelCtx, labelConstraints).Size; size != (gfx.Size{}) {
			labelSize = size
		}
	}

	contentWidth := constraints.MaxSize.W
	if contentWidth <= 0 {
		contentWidth = labelSize.W + p.cachedPadX*2
		if contentWidth < resolved.Density.Scale(160) {
			contentWidth = resolved.Density.Scale(160)
		}
	}
	labelHeight := float32(0)
	if p.cachedLabelFacet != nil {
		labelHeight = labelSize.H
	}
	totalHeight := p.cachedPadY * 2
	if p.cachedLabelFacet != nil {
		totalHeight += labelHeight + p.cachedGap + p.cachedTrackHeight
	} else {
		totalHeight += p.cachedTrackHeight
	}
	measured := constraints.Constrain(gfx.Size{W: contentWidth, H: totalHeight})
	p.Layout.MeasuredSize = measured
	p.Layout.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return p.Layout.MeasuredResult
}

func (p *ProgressBar) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	p.cachedBounds = bounds
	p.cachedTrackBounds = gfx.Rect{}
	p.cachedIndicatorBounds = gfx.Rect{}
	p.cachedLabelBounds = gfx.Rect{}
	p.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	p.syncLabelFacet()
	inner := bounds.Inset(p.cachedPadX, p.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}
	trackTop := inner.Min.Y
	if p.cachedLabelFacet != nil {
		labelSize := p.cachedLabelFacet.Base().LayoutRole().MeasuredSize
		if labelSize == (gfx.Size{}) {
			labelSize = p.cachedLabelFacet.Base().LayoutRole().MeasuredResult.Size
		}
		p.cachedLabelBounds = gfx.RectFromXYWH(inner.Min.X, inner.Min.Y, inner.Width(), labelSize.H)
		if p.cachedWritingDirection == facet.WritingDirectionRTL {
			p.cachedLabelBounds = gfx.RectFromXYWH(inner.Max.X-labelSize.W, inner.Min.Y, labelSize.W, labelSize.H)
		} else {
			p.cachedLabelBounds = gfx.RectFromXYWH(inner.Min.X, inner.Min.Y, labelSize.W, labelSize.H)
		}
		trackTop = p.cachedLabelBounds.Max.Y + p.cachedGap
	}
	trackHeight := p.cachedTrackHeight
	if trackHeight <= 0 {
		trackHeight = maxFloat(4, inner.Height()*0.14)
	}
	if trackTop+trackHeight > inner.Max.Y {
		trackTop = maxFloat(inner.Min.Y, inner.Max.Y-trackHeight)
	}
	p.cachedTrackBounds = gfx.RectFromXYWH(inner.Min.X, trackTop, inner.Width(), trackHeight)
	progress := clamp01(p.Value.Get())
	indicatorWidth := p.cachedTrackBounds.Width() * progress
	if progress > 0 && indicatorWidth < 1 {
		indicatorWidth = 1
	}
	if indicatorWidth > p.cachedTrackBounds.Width() {
		indicatorWidth = p.cachedTrackBounds.Width()
	}
	p.cachedIndicatorBounds = gfx.RectFromXYWH(p.cachedTrackBounds.Min.X, p.cachedTrackBounds.Min.Y, indicatorWidth, p.cachedTrackBounds.Height())
	if p.cachedLabelFacet != nil {
		p.cachedLabelFacet.Base().LayoutRole().ArrangedBounds = p.cachedLabelBounds
	}
	if p.pulseRemaining > 0 {
		p.Tick.RequestTick()
	}
}

func (p *ProgressBar) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if p == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := p.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if p.Disabled.Get() {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	track := slots.Track.Resolve(state, tokens)
	indicator := slots.Indicator.Resolve(state, tokens)
	labelStyle := slots.OptionalLabel.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 24)
	if !isTransparentMaterial(root) {
		cmds = append(cmds, progressMaterialCommands(gfx.RoundedRectPath(bounds, p.cachedRootRadius), root)...)
	}
	if !isTransparentMaterial(track) && !p.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, progressMaterialCommands(gfx.RoundedRectPath(p.cachedTrackBounds, p.cachedTrackRadius), track)...)
	}
	if !isTransparentMaterial(indicator) && !p.cachedIndicatorBounds.IsEmpty() {
		indicatorRadius := minFloat(p.cachedIndicatorBounds.Height()*0.5, p.cachedTrackRadius)
		cmds = append(cmds, progressMaterialCommands(gfx.RoundedRectPath(p.cachedIndicatorBounds, indicatorRadius), indicator)...)
	}
	if !p.Disabled.Get() && p.pulseRemaining > 0 && !p.cachedIndicatorBounds.IsEmpty() {
		stripeWidth := maxFloat(2, minFloat(p.cachedIndicatorBounds.Width()*0.18, p.cachedIndicatorBounds.Height()*0.8))
		if stripeWidth > p.cachedIndicatorBounds.Width() {
			stripeWidth = p.cachedIndicatorBounds.Width()
		}
		alpha := float32(0.22)
		if p.pulseDuration > 0 {
			alpha *= clamp01(float32(p.pulseRemaining) / float32(p.pulseDuration))
		}
		stripeX := p.cachedIndicatorBounds.Min.X
		if p.cachedIndicatorBounds.Width() > stripeWidth {
			stripeX += p.pulsePhase * (p.cachedIndicatorBounds.Width() - stripeWidth)
		}
		stripe := gfx.RectFromXYWH(stripeX, p.cachedIndicatorBounds.Min.Y, stripeWidth, p.cachedIndicatorBounds.Height())
		overlay := scaleMaterialOpacity(indicator, alpha)
		cmds = append(cmds, progressMaterialCommands(gfx.RoundedRectPath(stripe, minFloat(stripe.Width(), stripe.Height())*0.5), overlay)...)
	}
	if p.cachedLabelFacet != nil && !p.cachedLabelBounds.IsEmpty() && !progressIsTransparentMaterial(labelStyle) {
		if projected := p.cachedLabelFacet.Base().ProjectionRole().Project(facet.ProjectionContext{
			Runtime:      runtimeServicesOrNil(runtime),
			Bounds:       p.cachedLabelBounds,
			ContentScale: contentScale,
		}); projected != nil {
			cmds = append(cmds, projected.Commands...)
		}
	}
	return cmds
}

func (p *ProgressBar) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.StatusProgressBarSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, p.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uistatus.ResolveProgressBarRecipe(style, p.progressBarVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
}

func (p *ProgressBar) progressBarVariant() uistatus.ProgressBarVariant {
	if p != nil && p.Disabled.Get() {
		return uistatus.ProgressBarDisabled
	}
	return uistatus.ProgressBarDefault
}

func (p *ProgressBar) startPulse() {
	if p == nil {
		return
	}
	duration := p.cachedTokens.Motion.DurationShort
	if duration <= 0 {
		duration = 120 * time.Millisecond
	}
	p.pulseDuration = duration
	p.pulseRemaining = duration
	p.pulsePhase = 0
	p.Tick.RequestTick()
}

func (p *ProgressBar) onTick(dt time.Duration) {
	if p == nil || p.Disabled.Get() || p.pulseRemaining <= 0 {
		p.Tick.Reset()
		return
	}
	p.pulseRemaining -= dt
	if p.pulseRemaining < 0 {
		p.pulseRemaining = 0
	}
	if p.pulseDuration > 0 {
		p.pulsePhase += float32(dt) / float32(p.pulseDuration)
		for p.pulsePhase >= 1 {
			p.pulsePhase -= 1
		}
	}
	p.invalidate(facet.DirtyProjection)
	if p.pulseRemaining > 0 {
		p.Tick.RequestTick()
	} else {
		p.Tick.Reset()
	}
}

func progressMaterialCommands(path gfx.Path, material theme.Material) []gfx.Command {
	if progressIsTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(material.Fills)+len(material.Strokes)*3)
	for _, fill := range material.Fills {
		if fill.Type != theme.FillSolid || fill.Color.A <= 0 {
			continue
		}
		alpha := material.Opacity * fill.Opacity
		if alpha <= 0 {
			continue
		}
		if alpha < 1 {
			cmds = append(cmds, gfx.PushOpacity{Alpha: alpha})
		}
		cmds = append(cmds, gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color)})
		if alpha < 1 {
			cmds = append(cmds, gfx.PopOpacity{})
		}
	}
	for _, stroke := range material.Strokes {
		if stroke.Paint.Type != theme.FillSolid || stroke.Paint.Color.A <= 0 || stroke.Width <= 0 {
			continue
		}
		alpha := material.Opacity * stroke.Paint.Opacity
		if alpha <= 0 {
			continue
		}
		if alpha < 1 {
			cmds = append(cmds, gfx.PushOpacity{Alpha: alpha})
		}
		cmds = append(cmds, gfx.StrokePath{
			Path:   path,
			Brush:  gfx.SolidBrush(stroke.Paint.Color),
			Stroke: gfx.DefaultStroke(stroke.Width),
		})
		if alpha < 1 {
			cmds = append(cmds, gfx.PopOpacity{})
		}
	}
	return cmds
}

func progressIsTransparentMaterial(material theme.Material) bool {
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

func scaleMaterialOpacity(material theme.Material, opacity float32) theme.Material {
	if opacity <= 0 {
		return theme.Material{}
	}
	next := material
	next.Opacity *= opacity
	return next
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func minFloat(a, b float32) float32 {
	if a < b {
		return a
	}
	return b
}
