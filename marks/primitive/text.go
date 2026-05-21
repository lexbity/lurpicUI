package primitive

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TextOverflow describes how primitive.text resolves content that exceeds bounds.
type TextOverflow uint8

const (
	TextOverflowClip TextOverflow = iota
	TextOverflowTruncate
	TextOverflowWrap
	TextOverflowScroll
)

// TextMarkID values reserve the primitive.text semantic namespace.
const (
	TextMarkIDRoot      facet.MarkID = 1
	TextMarkIDTextRuns  facet.MarkID = 2
	TextMarkIDGlyphs    facet.MarkID = 3
	TextMarkIDSelection facet.MarkID = 4
	TextMarkIDBaseline  facet.MarkID = 5
)

// Text implements the primitive.text standard mark.
type Text struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	textRole       facet.TextRole

	Content    string
	Typography theme.TextToken
	Foreground theme.ColorToken
	Disabled   bool
	Overflow   TextOverflow
	Alignment  text.TextAlignment
	MaxWidth   float32

	cachedLayout *text.TextLayout
	cachedStyle  text.TextStyle
	cachedBrush  gfx.Brush
}

var _ facet.FacetImpl = (*Text)(nil)
var _ layout.AnchorExporter = (*Text)(nil)

// NewText constructs a primitive.text mark with the canonical defaults.
func NewText(content string) *Text {
	t := &Text{
		Facet:      facet.NewFacet(),
		Content:    content,
		Typography: theme.TextBodyM,
		Foreground: theme.ColorText,
		Overflow:   TextOverflowClip,
	}
	t.layoutRole.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	t.layoutRole.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsAnchor | facet.SupportsFree,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := t.measureSize(ctx, constraints)
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
	t.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.layoutRole.ArrangedBounds = bounds
	}
	t.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil || len(t.cachedLayoutCommands()) == 0 {
			return
		}
		list.Commands = append(list.Commands, t.cachedLayoutCommands()...)
	}
	t.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		return t.project(ctx)
	}
	t.AddRole(&t.layoutRole)
	t.AddRole(&t.renderRole)
	t.AddRole(&t.projectionRole)
	t.AddRole(&t.textRole)
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Text) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

// SetContent updates the authored text and invalidates layout/projection.
func (t *Text) SetContent(content string) {
	if t == nil || t.Content == content {
		return
	}
	t.Content = content
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetTypography updates the text token used for shaping.
func (t *Text) SetTypography(token theme.TextToken) {
	if t == nil || t.Typography == token {
		return
	}
	t.Typography = token
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetForeground updates the text color token used at projection time.
func (t *Text) SetForeground(token theme.ColorToken) {
	if t == nil || t.Foreground == token {
		return
	}
	t.Foreground = token
	t.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles the disabled projection state.
func (t *Text) SetDisabled(disabled bool) {
	if t == nil || t.Disabled == disabled {
		return
	}
	t.Disabled = disabled
	t.invalidate(facet.DirtyProjection)
}

// SetOverflow updates the overflow policy.
func (t *Text) SetOverflow(overflow TextOverflow) {
	if t == nil || t.Overflow == overflow {
		return
	}
	t.Overflow = overflow
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetAlignment updates the paragraph alignment.
func (t *Text) SetAlignment(alignment text.TextAlignment) {
	if t == nil || t.Alignment == alignment {
		return
	}
	t.Alignment = alignment
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetMaxWidth updates the authored wrap/truncate bound.
func (t *Text) SetMaxWidth(maxWidth float32) {
	if t == nil || t.MaxWidth == maxWidth {
		return
	}
	t.MaxWidth = maxWidth
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// SetSelection updates the optional text selection state.
func (t *Text) SetSelection(sel text.TextRange) {
	if t == nil || t.textRole.Selection == sel {
		return
	}
	t.textRole.Selection = sel
	t.invalidate(facet.DirtyProjection)
}

// SetCaret updates the optional caret state.
func (t *Text) SetCaret(pos text.TextPosition, visible bool) {
	if t == nil {
		return
	}
	if t.textRole.CaretPosition == pos && t.textRole.CaretVisible == visible {
		return
	}
	t.textRole.CaretPosition = pos
	t.textRole.CaretVisible = visible
	t.invalidate(facet.DirtyProjection)
}

// ExportAnchors publishes the primitive text anchor set.
func (t *Text) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if t == nil {
		return nil
	}
	bounds := t.layoutRole.ArrangedBounds
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
	if t.textRole.Layout != nil {
		out["baseline"] = gfx.Point{
			X: bounds.Min.X,
			Y: bounds.Min.Y + t.textRole.Layout.Baseline,
		}
	} else {
		out["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return out
}

func (t *Text) OnAttach(ctx facet.AttachContext) {}
func (t *Text) OnActivate()                      {}
func (t *Text) OnDeactivate()                    {}
func (t *Text) OnDetach() {
	t.cachedLayout = nil
	t.cachedStyle = text.TextStyle{}
	t.cachedBrush = gfx.Brush{}
}

func (t *Text) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Facet.Invalidate(flags)
}

func (t *Text) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	layout, style, ok := t.resolveLayout(ctx, constraints)
	if !ok {
		t.cachedLayout = nil
		t.cachedStyle = text.TextStyle{}
		t.textRole.Layout = nil
		return facet.MeasureResult{}
	}
	t.cachedLayout = layout
	t.cachedStyle = style
	t.textRole.Layout = layout
	size := gfx.Size{W: layout.Bounds.Width(), H: layout.Bounds.Height()}
	t.layoutRole.MeasuredSize = size
	t.layoutRole.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return t.layoutRole.MeasuredResult
}

func (t *Text) measureSize(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	result := t.measure(ctx, constraints)
	return result.Size
}

func (t *Text) resolveLayout(ctx facet.MeasureContext, constraints facet.Constraints) (*text.TextLayout, text.TextStyle, bool) {
	if t == nil {
		return nil, text.TextStyle{}, false
	}
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := resolved.TextStyle(t.Typography)
	maxWidth := t.effectiveMaxWidth(constraints)
	shaper := t.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil, text.TextStyle{}, false
	}
	shaper.SetContentScale(ctx.ContentScale)
	switch t.Overflow {
	case TextOverflowWrap:
		layout := shaper.Shape(text.Paragraph{
			Spans:     []text.TextSpan{{Text: t.Content, Style: style}},
			MaxWidth:  maxWidth,
			Alignment: t.Alignment,
		})
		return layout, style, layout != nil
	case TextOverflowTruncate:
		layout := shaper.ShapeTruncated(t.Content, style, maxWidth)
		return layout, style, layout != nil
	case TextOverflowScroll, TextOverflowClip:
		fallthrough
	default:
		layout := shaper.ShapeSimple(t.Content, style)
		return layout, style, layout != nil
	}
}

func (t *Text) effectiveMaxWidth(constraints facet.Constraints) float32 {
	maxWidth := constraints.MaxSize.W
	if t.MaxWidth > 0 {
		if maxWidth <= 0 {
			maxWidth = t.MaxWidth
		} else {
			if t.MaxWidth < maxWidth {
				maxWidth = t.MaxWidth
			}
		}
	}
	if maxWidth < 0 {
		return 0
	}
	return maxWidth
}

func (t *Text) newShaper(runtime any) *text.Shaper {
	registry := t.fontRegistry(runtime)
	if registry == nil {
		return nil
	}
	return text.NewShaper(registry)
}

func (t *Text) fontRegistry(runtime any) *text.FontRegistry {
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

func (t *Text) cachedLayoutCommands() []gfx.Command {
	if t == nil || t.cachedLayout == nil {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(t.cachedLayout.Lines))
	color := t.cachedBrush.Color
	if len(t.cachedLayout.Lines) == 0 {
		return nil
	}
	for _, line := range t.cachedLayout.Lines {
		lineOrigin := gfx.Point{
			X: t.layoutRole.ArrangedBounds.Min.X + line.Bounds.Min.X,
			Y: t.layoutRole.ArrangedBounds.Min.Y + line.Bounds.Min.Y + line.Baseline,
		}
		for _, run := range line.Runs {
			runOrigin := gfx.Point{
				X: lineOrigin.X + run.Bounds.Min.X,
				Y: lineOrigin.Y + run.Bounds.Min.Y,
			}
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run:    run,
				Origin: runOrigin,
				Brush:  gfx.SolidBrush(color),
			})
		}
	}
	return cmds
}

func (t *Text) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if t == nil || t.cachedLayout == nil {
		return nil
	}
	color := t.resolveBrushColor(ctx.Runtime)
	t.cachedBrush = gfx.SolidBrush(color)
	cmds := t.cachedLayoutCommands()
	if len(cmds) == 0 {
		return nil
	}
	list := &gfx.CommandList{Commands: cmds}
	t.cachedLayout = t.textRole.Layout
	return list
}

func (t *Text) resolveBrushColor(runtime any) gfx.Color {
	style := t.styleTokens(runtime)
	if t.Disabled {
		return colorWithAlpha(style.Color.OnSurfaceVariant, style.Color.DisabledOpacity)
	}
	return colorForToken(style, t.Foreground)
}

func (t *Text) styleTokens(runtime any) theme.Tokens {
	if runtime == nil {
		return theme.DefaultTokens()
	}
	type styleContextProvider interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if provider, ok := runtime.(styleContextProvider); ok {
		if store := theme.NearestStyleContext(provider, t.Base().ID()); store != nil {
			return store.Get().Tokens
		}
	}
	return theme.DefaultTokens()
}

func colorForToken(tokens theme.Tokens, token theme.ColorToken) gfx.Color {
	switch token {
	case theme.ColorBackground:
		return tokens.Color.Background
	case theme.ColorSurface:
		return tokens.Color.Surface
	case theme.ColorSurfaceVariant:
		return tokens.Color.SurfaceVariant
	case theme.ColorPrimary:
		return tokens.Color.Primary
	case theme.ColorOnPrimary:
		return tokens.Color.OnPrimary
	case theme.ColorText:
		return tokens.Color.OnSurface
	case theme.ColorTextSecondary:
		return tokens.Color.OnSurfaceVariant
	case theme.ColorTextDisabled:
		return colorWithAlpha(tokens.Color.OnSurfaceVariant, tokens.Color.DisabledOpacity)
	case theme.ColorBorder:
		return colorWithAlpha(tokens.Color.SurfaceVariant, 0.35)
	case theme.ColorBorderStrong:
		return colorWithAlpha(tokens.Color.OnSurfaceVariant, 0.45)
	case theme.ColorSelection:
		return colorWithAlpha(tokens.Color.Primary, tokens.Color.SelectedOverlay)
	case theme.ColorCaret:
		return tokens.Color.Primary
	case theme.ColorError:
		return tokens.Color.Error
	case theme.ColorSuccess:
		return tokens.Color.Success
	case theme.ColorWarning:
		return tokens.Color.Warning
	default:
		return tokens.Color.OnSurface
	}
}

func colorWithAlpha(c gfx.Color, a float32) gfx.Color {
	if a <= 0 {
		return gfx.Color{}
	}
	if a >= 1 {
		return c.WithAlpha(1)
	}
	return gfx.Color{
		R: c.R * a,
		G: c.G * a,
		B: c.B * a,
		A: a,
	}
}
