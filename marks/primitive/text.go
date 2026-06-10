package primitive

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
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
	marks.Core

	Content    marks.Binding[string]
	Typography marks.Binding[theme.TextToken]
	Foreground marks.Binding[theme.ColorToken]
	Disabled   marks.Binding[bool]
	Overflow   marks.Binding[TextOverflow]
	Alignment  marks.Binding[text.TextAlignment]
	MaxWidth   marks.Binding[float32]

	textRole facet.TextRole

	cachedLayout *text.TextLayout
	cachedStyle  text.TextStyle
	cachedBrush  gfx.Brush
}

var _ facet.FacetImpl = (*Text)(nil)
var _ layout.AnchorExporter = (*Text)(nil)
var _ marks.Mark = (*Text)(nil)

// NewText constructs a primitive.text mark with the canonical defaults.
func NewText(content marks.Binding[string]) *Text {
	t := &Text{
		Content:    content,
		Typography: marks.Const(theme.TextBodyM),
		Foreground: marks.Const(theme.ColorText),
		Disabled:   marks.Const(false),
		Overflow:   marks.Const(TextOverflowClip),
		Alignment:  marks.Const(text.AlignLeft),
		MaxWidth:   marks.Const[float32](0),
	}
	t.Facet = facet.NewFacet()
	t.AddBinding(t.Content)
	t.AddBinding(t.Typography)
	t.AddBinding(t.Foreground)
	t.AddBinding(t.Disabled)
	t.AddBinding(t.Overflow)
	t.AddBinding(t.Alignment)
	t.AddBinding(t.MaxWidth)

	t.Layout.Parent = facet.GroupParentContract{Kind: facet.GroupLayoutNone}
	t.Layout.Child = facet.GroupChildContract{
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
	t.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.Layout.ArrangedBounds = bounds
	}
	t.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return t.buildCommands(ctx)
	}
	t.RegisterRoles()
	t.AddRole(&t.textRole)
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Text) Base() *facet.Facet {
	t.BindImpl(t)
	return &t.Facet
}

// Descriptor satisfies marks.Mark.
func (t *Text) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "primitive", TypeName: "text"}
}

// ExportAnchors publishes the primitive text anchor set.
func (t *Text) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	bounds := t.Layout.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	anchors := t.DefaultAnchors(bounds, ctx)
	if t.textRole.Layout != nil {
		anchors["baseline"] = gfx.Point{
			X: bounds.Min.X,
			Y: bounds.Min.Y + t.textRole.Layout.Baseline,
		}
	} else {
		anchors["baseline"] = gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y}
	}
	return anchors
}

func (t *Text) OnAttach(ctx facet.AttachContext) { t.Core.OnAttach() }
func (t *Text) OnDetach() {
	t.Core.OnDetach()
	t.cachedLayout = nil
	t.cachedStyle = text.TextStyle{}
	t.cachedBrush = gfx.Brush{}
}
func (t *Text) OnActivate()   { t.Core.OnActivate() }
func (t *Text) OnDeactivate() { t.Core.OnDeactivate() }

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
	t.Layout.MeasuredSize = size
	t.Layout.MeasuredResult = facet.MeasureResult{
		Size: size,
		Intrinsic: facet.IntrinsicSize{
			Min:       size,
			Preferred: size,
			Max:       size,
		},
		Constraints: constraints,
	}
	return t.Layout.MeasuredResult
}

func (t *Text) measureSize(ctx facet.MeasureContext, constraints facet.Constraints) gfx.Size {
	result := t.measure(ctx, constraints)
	return result.Size
}

func (t *Text) resolveLayout(ctx facet.MeasureContext, constraints facet.Constraints) (*text.TextLayout, text.TextStyle, bool) {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := resolved.TextStyle(t.Typography.Get())
	maxWidth := t.effectiveMaxWidth(constraints)
	shaper := t.newShaper(ctx.Runtime)
	if shaper == nil {
		return nil, text.TextStyle{}, false
	}
	shaper.SetContentScale(ctx.ContentScale)
	switch t.Overflow.Get() {
	case TextOverflowWrap:
		l := shaper.Shape(text.Paragraph{
			Spans:     []text.TextSpan{{Text: t.Content.Get(), Style: style}},
			MaxWidth:  maxWidth,
			Alignment: t.Alignment.Get(),
		})
		return l, style, l != nil
	case TextOverflowTruncate:
		l := shaper.ShapeTruncated(t.Content.Get(), style, maxWidth)
		return l, style, l != nil
	case TextOverflowScroll, TextOverflowClip:
		fallthrough
	default:
		l := shaper.ShapeSimple(t.Content.Get(), style)
		return l, style, l != nil
	}
}

func (t *Text) effectiveMaxWidth(constraints facet.Constraints) float32 {
	maxWidth := constraints.MaxSize.W
	if t.MaxWidth.Get() > 0 {
		if maxWidth <= 0 {
			maxWidth = t.MaxWidth.Get()
		} else if t.MaxWidth.Get() < maxWidth {
			maxWidth = t.MaxWidth.Get()
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

func (t *Text) buildCommands(ctx facet.ProjectionContext) []gfx.Command {
	if t.cachedLayout == nil {
		return nil
	}
	color := t.resolveBrushColor(ctx.Runtime)
	t.cachedBrush = gfx.SolidBrush(color)
	cmds := TextLayoutCommands(t.cachedLayout, t.Layout.ArrangedBounds, t.cachedBrush)
	if len(cmds) == 0 {
		return nil
	}
	t.cachedLayout = t.textRole.Layout
	return cmds
}

// TextLayoutCommands converts a shaped text layout into draw commands positioned within bounds.
//
// Each draw origin is resolved from the arranged content box plus the shaped
// line box and baseline. The projection path does not recompute its own text
// metrics or baseline placement.
func TextLayoutCommands(l *text.TextLayout, bounds gfx.Rect, brush gfx.Brush) []gfx.Command {
	if l == nil || bounds.IsEmpty() || brush.Color.A == 0 {
		return nil
	}
	if len(l.Lines) == 0 {
		return nil
	}
	cmds := make([]gfx.Command, 0, len(l.Lines))
	for _, line := range l.Lines {
		lineOrigin := gfx.Point{
			X: bounds.Min.X + l.Bounds.Min.X + line.Bounds.Min.X,
			Y: bounds.Min.Y + l.Bounds.Min.Y + line.Bounds.Min.Y + line.Baseline,
		}
		for _, run := range line.Runs {
			runOrigin := gfx.Point{
				X: lineOrigin.X + run.Bounds.Min.X,
				Y: lineOrigin.Y + run.Bounds.Min.Y,
			}
			cmds = append(cmds, gfx.DrawGlyphRun{
				Run:    run,
				Origin: runOrigin,
				Brush:  brush,
			})
		}
	}
	return cmds
}

func (t *Text) resolveBrushColor(runtime any) gfx.Color {
	style := t.styleTokens(runtime)
	if t.Disabled.Get() {
		return colorWithAlpha(style.Color.OnSurfaceVariant, style.Color.DisabledOpacity)
	}
	return colorForToken(style, t.Foreground.Get())
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
