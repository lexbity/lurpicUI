package status

import (
	"math"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistatus"
)

const (
	progressRingMarkIDRoot          facet.MarkID = 1
	progressRingMarkIDTrackArc      facet.MarkID = 2
	progressRingMarkIDIndicatorArc  facet.MarkID = 3
	progressRingMarkIDOptionalLabel facet.MarkID = 4
)

// ProgressRing implements the status.progress_ring canonical mark.
type ProgressRing struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	tickRole       facet.TickRole

	Label    string
	Value    float32
	Disabled bool

	cachedTokens           theme.Tokens
	cachedRecipe           shared.StatusProgressRingSlots
	cachedBounds           gfx.Rect
	cachedRootRadius       float32
	cachedRingSide         float32
	cachedRingOuterRadius  float32
	cachedRingInnerRadius  float32
	cachedTrackBounds      gfx.Rect
	cachedIndicatorBounds  gfx.Rect
	cachedLabelBounds      gfx.Rect
	cachedPadX             float32
	cachedPadY             float32
	cachedGap              float32
	cachedRingThickness    float32
	cachedShowLabel        bool
	cachedWritingDirection facet.WritingDirection
	cachedLabelFacet       *primitive.Text

	pulseDuration  time.Duration
	pulseRemaining time.Duration
	pulsePhase     float32
}

var _ facet.FacetImpl = (*ProgressRing)(nil)
var _ layout.AnchorExporter = (*ProgressRing)(nil)

// NewProgressRing constructs a status.progress_ring mark with canonical defaults.
func NewProgressRing(label string) *ProgressRing {
	p := &ProgressRing{
		Facet: facet.NewFacet(),
		Label: label,
	}
	p.layoutRole.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	p.layoutRole.Child = facet.GroupChildContract{
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
	p.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return p.measure(ctx, constraints)
	}
	p.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		p.layoutRole.ArrangedBounds = bounds
		p.arrange(ctx, bounds)
	}
	p.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := p.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	p.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := p.buildCommands(p.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	p.tickRole.OnTick = func(dt time.Duration) {
		p.onTick(dt)
	}
	p.AddRole(&p.layoutRole)
	p.AddRole(&p.renderRole)
	p.AddRole(&p.projectionRole)
	p.AddRole(&p.tickRole)
	p.syncLabelFacet()
	return p
}

// Base satisfies facet.FacetImpl.
func (p *ProgressRing) Base() *facet.Facet {
	p.Facet.BindImpl(p)
	return &p.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *ProgressRing) AccessibilityRole() string { return "progressbar" }

// AccessibleName reports the semantic name source required by the spec.
func (p *ProgressRing) AccessibleName() string { return "" }

// SetLabel updates the authored optional label.
func (p *ProgressRing) SetLabel(label string) {
	if p == nil || p.Label == label {
		return
	}
	p.Label = label
	p.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetValue updates the authored progress value and starts a short pulse.
func (p *ProgressRing) SetValue(value float32) {
	if p == nil {
		return
	}
	value = clamp01(value)
	if p.Value == value {
		return
	}
	p.Value = value
	if !p.Disabled {
		p.startPulse()
	}
	p.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (p *ProgressRing) SetDisabled(disabled bool) {
	if p == nil || p.Disabled == disabled {
		return
	}
	p.Disabled = disabled
	if disabled {
		p.pulseDuration = 0
		p.pulseRemaining = 0
		p.pulsePhase = 0
		p.tickRole.Reset()
	}
	p.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the progress-ring anchor set.
func (p *ProgressRing) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.layoutRole.ArrangedBounds
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
	return out
}

// OnAttach is unused.
func (p *ProgressRing) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (p *ProgressRing) OnActivate() {}

// OnDeactivate is unused.
func (p *ProgressRing) OnDeactivate() {}

// OnDetach clears cached projection state.
func (p *ProgressRing) OnDetach() {
	p.cachedTokens = theme.Tokens{}
	p.cachedRecipe = shared.StatusProgressRingSlots{}
	p.cachedBounds = gfx.Rect{}
	p.cachedRootRadius = 0
	p.cachedRingSide = 0
	p.cachedRingOuterRadius = 0
	p.cachedRingInnerRadius = 0
	p.cachedTrackBounds = gfx.Rect{}
	p.cachedIndicatorBounds = gfx.Rect{}
	p.cachedLabelBounds = gfx.Rect{}
	p.cachedPadX = 0
	p.cachedPadY = 0
	p.cachedGap = 0
	p.cachedRingThickness = 0
	p.cachedShowLabel = false
	p.cachedWritingDirection = facet.WritingDirectionLTR
	p.cachedLabelFacet = nil
	p.pulseDuration = 0
	p.pulseRemaining = 0
	p.pulsePhase = 0
}

func (p *ProgressRing) invalidate(flags facet.DirtyFlags) {
	if p == nil {
		return
	}
	p.Facet.Invalidate(flags)
}

func (p *ProgressRing) syncLabelFacet() {
	if p == nil {
		return
	}
	label := strings.TrimSpace(p.Label)
	if !p.cachedShowLabel || label == "" {
		p.cachedLabelFacet = nil
		return
	}
	if p.cachedLabelFacet == nil {
		p.cachedLabelFacet = primitive.NewText(label)
	} else {
		p.cachedLabelFacet.SetContent(label)
	}
	p.cachedLabelFacet.SetTypography(theme.TextLabelM)
	p.cachedLabelFacet.SetOverflow(primitive.TextOverflowTruncate)
	p.cachedLabelFacet.SetAlignment(text.AlignCenter)
	if p.Disabled {
		p.cachedLabelFacet.SetForeground(theme.ColorTextDisabled)
		p.cachedLabelFacet.SetDisabled(true)
	} else {
		p.cachedLabelFacet.SetForeground(theme.ColorText)
		p.cachedLabelFacet.SetDisabled(false)
	}
}

func (p *ProgressRing) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistatus.ResolveProgressRingRecipe(style, p.progressRingVariant())
	p.cachedTokens = resolved.TokenSet()
	p.cachedRecipe = slots
	p.cachedWritingDirection = ctx.WritingDirection
	p.cachedPadX = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedPadY = maxFloat(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedGap = maxFloat(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	p.cachedRingThickness = maxFloat(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingS))*0.75)
	if p.cachedRingThickness > resolved.Density.Scale(12) {
		p.cachedRingThickness = resolved.Density.Scale(12)
	}
	p.cachedRootRadius = p.cachedRingThickness * 0.5
	p.cachedShowLabel = strings.TrimSpace(p.Label) != "" && resolved.Density.ID != theme.DensityIDCompact
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

	ringSide := resolved.Density.Scale(32)
	if ringSide < 28 {
		ringSide = 28
	}
	if constraints.MaxSize.W > 0 {
		if maxWidth := constraints.MaxSize.W - p.cachedPadX*2; maxWidth > 0 && maxWidth < ringSide {
			ringSide = maxWidth
		}
	}
	if constraints.MaxSize.H > 0 {
		if maxHeight := constraints.MaxSize.H - p.cachedPadY*2; maxHeight > 0 {
			if p.cachedShowLabel {
				maxHeight -= labelSize.H + p.cachedGap
			}
			if maxHeight > 0 && maxHeight < ringSide {
				ringSide = maxHeight
			}
		}
	}
	if ringSide < 24 {
		ringSide = 24
	}

	width := maxFloat(ringSide, labelSize.W) + p.cachedPadX*2
	height := ringSide + p.cachedPadY*2
	if p.cachedLabelFacet != nil {
		height += labelSize.H + p.cachedGap
	}
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
	p.layoutRole.MeasuredSize = measured
	p.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: measured,
		Intrinsic: facet.IntrinsicSize{
			Min:       measured,
			Preferred: measured,
			Max:       measured,
		},
		Constraints: constraints,
	}
	return p.layoutRole.MeasuredResult
}

func (p *ProgressRing) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	p.cachedBounds = bounds
	p.cachedTrackBounds = gfx.Rect{}
	p.cachedIndicatorBounds = gfx.Rect{}
	p.cachedLabelBounds = gfx.Rect{}
	p.layoutRole.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	p.syncLabelFacet()
	inner := bounds.Inset(p.cachedPadX, p.cachedPadY)
	if inner.IsEmpty() {
		inner = bounds
	}

	labelHeight := float32(0)
	if p.cachedLabelFacet != nil {
		labelSize := p.cachedLabelFacet.Base().LayoutRole().MeasuredSize
		if labelSize == (gfx.Size{}) {
			labelSize = p.cachedLabelFacet.Base().LayoutRole().MeasuredResult.Size
		}
		labelHeight = labelSize.H
		ringAreaHeight := inner.Height() - labelHeight - p.cachedGap
		if ringAreaHeight < 0 {
			ringAreaHeight = 0
		}
		ringSide := minFloat(inner.Width(), ringAreaHeight)
		if ringSide <= 0 {
			ringSide = minFloat(inner.Width(), inner.Height())
		}
		ringX := inner.Min.X + (inner.Width()-ringSide)*0.5
		ringY := inner.Min.Y
		p.cachedTrackBounds = gfx.RectFromXYWH(ringX, ringY, ringSide, ringSide)
		if p.cachedWritingDirection == facet.WritingDirectionRTL {
			p.cachedLabelBounds = gfx.RectFromXYWH(inner.Min.X, inner.Max.Y-labelHeight, inner.Width(), labelHeight)
		} else {
			p.cachedLabelBounds = gfx.RectFromXYWH(inner.Min.X, inner.Max.Y-labelHeight, inner.Width(), labelHeight)
		}
	} else {
		ringSide := minFloat(inner.Width(), inner.Height())
		ringX := inner.Min.X + (inner.Width()-ringSide)*0.5
		ringY := text.CenterY(inner, ringSide)
		p.cachedTrackBounds = gfx.RectFromXYWH(ringX, ringY, ringSide, ringSide)
	}
	if p.cachedTrackBounds.IsEmpty() {
		return
	}
	p.cachedRingSide = p.cachedTrackBounds.Width()
	p.cachedRingOuterRadius = p.cachedRingSide * 0.5
	p.cachedRingInnerRadius = maxFloat(1, p.cachedRingOuterRadius-p.cachedRingThickness)
	center := gfx.Point{X: p.cachedTrackBounds.Min.X + p.cachedTrackBounds.Width()*0.5, Y: p.cachedTrackBounds.Min.Y + p.cachedTrackBounds.Height()*0.5}
	progress := clamp01(p.Value)
	p.cachedIndicatorBounds = ringSegmentBounds(center, p.cachedRingOuterRadius, p.cachedRingInnerRadius, progress)
	if p.cachedLabelFacet != nil {
		p.cachedLabelFacet.Base().LayoutRole().ArrangedBounds = p.cachedLabelBounds
	}
	if p.pulseRemaining > 0 {
		p.tickRole.RequestTick()
	}
}

func (p *ProgressRing) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if p == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := p.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if p.Disabled {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	track := slots.TrackArc.Resolve(state, tokens)
	indicator := slots.IndicatorArc.Resolve(state, tokens)
	labelStyle := slots.OptionalLabel.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 24)
	center := gfx.Point{X: p.cachedTrackBounds.Min.X + p.cachedTrackBounds.Width()*0.5, Y: p.cachedTrackBounds.Min.Y + p.cachedTrackBounds.Height()*0.5}
	if !isTransparentMaterial(root) && !p.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, progressMaterialCommands(gfx.CirclePath(center, p.cachedRingOuterRadius), root)...)
	}
	if !isTransparentMaterial(track) && !p.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, progressMaterialCommands(ringPath(center, float64(p.cachedRingOuterRadius), float64(p.cachedRingInnerRadius), 0, 2*math.Pi), track)...)
	}
	progress := clamp01(p.Value)
	if !isTransparentMaterial(indicator) && !p.cachedTrackBounds.IsEmpty() && progress > 0 {
		start := -math.Pi * 0.5
		sweep := float64(progress) * 2 * math.Pi
		cmds = append(cmds, progressMaterialCommands(ringPath(center, float64(p.cachedRingOuterRadius), float64(p.cachedRingInnerRadius), start, sweep), indicator)...)
	}
	if !p.Disabled && p.pulseRemaining > 0 && !p.cachedTrackBounds.IsEmpty() && progress > 0 {
		start := -math.Pi * 0.5
		sweep := float64(progress) * 2 * math.Pi
		highlightSweep := math.Max(sweep*0.18, math.Pi/18)
		if highlightSweep > sweep {
			highlightSweep = sweep
		}
		if highlightSweep > 0 {
			highlightStart := start + float64(p.pulsePhase)*(sweep-highlightSweep)
			alpha := float32(0.25)
			if p.pulseDuration > 0 {
				alpha *= clamp01(float32(p.pulseRemaining) / float32(p.pulseDuration))
			}
			overlay := scaleMaterialOpacity(indicator, alpha)
			cmds = append(cmds, progressMaterialCommands(ringPath(center, float64(p.cachedRingOuterRadius), float64(p.cachedRingInnerRadius), highlightStart, highlightSweep), overlay)...)
		}
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

func (p *ProgressRing) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.StatusProgressRingSlots) {
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
			slots, _ := uistatus.ResolveProgressRingRecipe(style, p.progressRingVariant())
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: p.cachedTokens}, p.cachedRecipe
}

func (p *ProgressRing) progressRingVariant() uistatus.ProgressRingVariant {
	if p != nil && p.Disabled {
		return uistatus.ProgressRingDisabled
	}
	return uistatus.ProgressRingDefault
}

func (p *ProgressRing) startPulse() {
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
	p.tickRole.RequestTick()
}

func (p *ProgressRing) onTick(dt time.Duration) {
	if p == nil || p.Disabled || p.pulseRemaining <= 0 {
		p.tickRole.Reset()
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
		p.tickRole.RequestTick()
	} else {
		p.tickRole.Reset()
	}
}

func ringPath(center gfx.Point, outerRadius, innerRadius, startAngle, sweep float64) gfx.Path {
	if outerRadius <= 0 || innerRadius <= 0 || sweep <= 0 {
		return gfx.Path{}
	}
	if innerRadius >= outerRadius {
		innerRadius = math.Max(1, outerRadius*0.72)
	}
	segments := int(math.Ceil(math.Max(12, math.Abs(sweep)/(2*math.Pi)*64)))
	if segments < 12 {
		segments = 12
	}
	outer := arcPoints(center, outerRadius, startAngle, sweep, segments)
	inner := arcPoints(center, innerRadius, startAngle+sweep, -sweep, segments)
	if len(outer) == 0 || len(inner) == 0 {
		return gfx.Path{}
	}
	b := gfx.NewPath()
	b.MoveTo(outer[0])
	for i := 1; i < len(outer); i++ {
		b.LineTo(outer[i])
	}
	b.LineTo(inner[0])
	for i := 1; i < len(inner); i++ {
		b.LineTo(inner[i])
	}
	b.Close()
	return b.Build()
}

func arcPoints(center gfx.Point, radius, startAngle, sweep float64, segments int) []gfx.Point {
	if radius <= 0 || sweep == 0 {
		return nil
	}
	if segments < 2 {
		segments = 2
	}
	pts := make([]gfx.Point, 0, segments+1)
	for i := 0; i <= segments; i++ {
		t := float64(i) / float64(segments)
		a := startAngle + sweep*t
		pts = append(pts, gfx.Point{
			X: center.X + float32(math.Cos(a))*float32(radius),
			Y: center.Y + float32(math.Sin(a))*float32(radius),
		})
	}
	return pts
}

func ringSegmentBounds(center gfx.Point, outerRadius, innerRadius, progress float32) gfx.Rect {
	if outerRadius <= 0 || progress <= 0 {
		return gfx.Rect{}
	}
	return gfx.RectFromXYWH(center.X-outerRadius, center.Y-outerRadius, outerRadius*2, outerRadius*2)
}
