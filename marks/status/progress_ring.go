package status

import (
	"math"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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
	marks.Core

	Label    marks.Binding[string]
	Value    marks.Binding[float32]
	Disabled marks.Binding[bool]

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
var _ marks.Mark = (*ProgressRing)(nil)

// NewProgressRing constructs a status.progress_ring mark with canonical defaults.
func NewProgressRing(label string) *ProgressRing {
	p := &ProgressRing{
		Label:    marks.Const(label),
		Value:    marks.Const(float32(0)),
		Disabled: marks.Const(false),
	}
	p.Facet = facet.NewFacet()
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
func (p *ProgressRing) Base() *facet.Facet {
	p.BindImpl(p)
	return &p.Facet
}

// Descriptor satisfies marks.Mark.
func (p *ProgressRing) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "status", TypeName: "progress_ring"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (p *ProgressRing) AccessibilityRole() string { return "progressbar" }

// AccessibleName reports the semantic name source required by the spec.
func (p *ProgressRing) AccessibleName() string { return "" }

// ExportAnchors publishes the progress-ring anchor set.
func (p *ProgressRing) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if p == nil {
		return nil
	}
	bounds := p.Layout.ArrangedBounds
	return p.DefaultAnchors(bounds, ctx)
}

// OnAttach is unused.
func (p *ProgressRing) OnAttach(ctx facet.AttachContext) { p.Core.OnAttach() }

// OnActivate is unused.
func (p *ProgressRing) OnActivate() { p.Core.OnActivate() }

// OnDeactivate is unused.
func (p *ProgressRing) OnDeactivate() { p.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (p *ProgressRing) OnDetach() {
	p.Core.OnDetach()
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
	p.Invalidate(flags)
}

func (p *ProgressRing) syncLabelFacet() {
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
	p.cachedLabelFacet.Alignment = marks.Const(text.AlignCenter)
	if p.Disabled.Get() {
		p.cachedLabelFacet.Foreground = marks.Const(theme.ColorTextDisabled)
		p.cachedLabelFacet.Disabled = marks.Const(true)
	} else {
		p.cachedLabelFacet.Foreground = marks.Const(theme.ColorText)
		p.cachedLabelFacet.Disabled = marks.Const(false)
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
	p.cachedPadX = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedPadY = mathutil.Max(float32(resolved.Spacing(theme.SpacingM)), resolved.Density.Scale(12))
	p.cachedGap = mathutil.Max(float32(resolved.Spacing(theme.SpacingXS)), resolved.Density.Scale(4))
	p.cachedRingThickness = mathutil.Max(resolved.Density.Scale(6), float32(resolved.Spacing(theme.SpacingS))*0.75)
	if p.cachedRingThickness > resolved.Density.Scale(12) {
		p.cachedRingThickness = resolved.Density.Scale(12)
	}
	p.cachedRootRadius = p.cachedRingThickness * 0.5
	p.cachedShowLabel = strings.TrimSpace(p.Label.Get()) != "" && resolved.Density.ID != theme.DensityIDCompact
	p.syncLabelFacet()

	availableWidth := constraints.MaxSize.W
	if availableWidth > 0 {
		availableWidth = mathutil.Max(0, availableWidth-p.cachedPadX*2)
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

	width := mathutil.Max(ringSide, labelSize.W) + p.cachedPadX*2
	height := ringSide + p.cachedPadY*2
	if p.cachedLabelFacet != nil {
		height += labelSize.H + p.cachedGap
	}
	measured := constraints.Constrain(gfx.Size{W: width, H: height})
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

func (p *ProgressRing) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
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

	labelHeight := float32(0)
	ringSide := mathutil.Min(inner.Width(), inner.Height())
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
		ringSide = mathutil.Min(inner.Width(), ringAreaHeight)
		if ringSide <= 0 {
			ringSide = mathutil.Min(inner.Width(), inner.Height())
		}
	}
	rows := layout.ArrangeVerticalFlowAligned(inner, 0, p.cachedGap, []gfx.Size{
		{W: ringSide, H: ringSide},
		{W: inner.Width(), H: labelHeight},
	}, p.cachedWritingDirection == facet.WritingDirectionRTL, layout.AlignCenter)
	p.cachedTrackBounds = rows[0]
	if p.cachedLabelFacet != nil {
		p.cachedLabelBounds = rows[1]
	}
	if p.cachedTrackBounds.IsEmpty() {
		return
	}
	p.cachedRingSide = p.cachedTrackBounds.Width()
	p.cachedRingOuterRadius = p.cachedRingSide * 0.5
	p.cachedRingInnerRadius = mathutil.Max(1, p.cachedRingOuterRadius-p.cachedRingThickness)
	center := gfx.Point{X: p.cachedTrackBounds.Min.X + p.cachedTrackBounds.Width()*0.5, Y: p.cachedTrackBounds.Min.Y + p.cachedTrackBounds.Height()*0.5}
	progress := clamp01(p.Value.Get())
	p.cachedIndicatorBounds = ringSegmentBounds(center, p.cachedRingOuterRadius, p.cachedRingInnerRadius, progress)
	if p.cachedLabelFacet != nil {
		p.cachedLabelFacet.Base().LayoutRole().ArrangedBounds = p.cachedLabelBounds
	}
	if p.pulseRemaining > 0 {
		p.Tick.RequestTick()
	}
}

func (p *ProgressRing) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
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
	track := slots.TrackArc.Resolve(state, tokens)
	indicator := slots.IndicatorArc.Resolve(state, tokens)
	labelStyle := slots.OptionalLabel.Resolve(state, tokens)

	cmds := make([]gfx.Command, 0, 24)
	center := gfx.Point{X: p.cachedTrackBounds.Min.X + p.cachedTrackBounds.Width()*0.5, Y: p.cachedTrackBounds.Min.Y + p.cachedTrackBounds.Height()*0.5}
	if !theme.IsTransparentMaterial(root) && !p.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, progressMaterialCommands(gfx.CirclePath(center, p.cachedRingOuterRadius), root)...)
	}
	if !theme.IsTransparentMaterial(track) && !p.cachedTrackBounds.IsEmpty() {
		cmds = append(cmds, progressMaterialCommands(ringPath(center, float64(p.cachedRingOuterRadius), float64(p.cachedRingInnerRadius), 0, 2*math.Pi), track)...)
	}
	progress := clamp01(p.Value.Get())
	if !theme.IsTransparentMaterial(indicator) && !p.cachedTrackBounds.IsEmpty() && progress > 0 {
		start := -math.Pi * 0.5
		sweep := float64(progress) * 2 * math.Pi
		cmds = append(cmds, progressMaterialCommands(ringPath(center, float64(p.cachedRingOuterRadius), float64(p.cachedRingInnerRadius), start, sweep), indicator)...)
	}
	if !p.Disabled.Get() && p.pulseRemaining > 0 && !p.cachedTrackBounds.IsEmpty() && progress > 0 {
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
	if p != nil && p.Disabled.Get() {
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
	p.Tick.RequestTick()
}

func (p *ProgressRing) onTick(dt time.Duration) {
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
