package theme

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/text"
)

// ColorToken is a named color in the theme palette.
type ColorToken uint8

const (
	ColorBackground ColorToken = iota
	ColorSurface
	ColorSurfaceVariant
	ColorPrimary
	ColorOnPrimary
	ColorText
	ColorTextSecondary
	ColorTextDisabled
	ColorBorder
	ColorBorderStrong
	ColorSelection
	ColorCaret
	ColorError
	ColorSuccess
	ColorWarning
)

// SpacingToken is a named spacing unit.
type SpacingToken uint8

const (
	SpacingXS SpacingToken = iota
	SpacingS
	SpacingM
	SpacingL
	SpacingXL
	SpacingXXL
)

// TextToken is a named text style preset.
type TextToken uint8

const (
	TextBodyM TextToken = iota
	TextBodyS
	TextLabelM
	TextLabelS
	TextHeadingS
	TextMonoM
	TextMonoS
)

// RadiusToken is a named corner radius.
type RadiusToken uint8

const (
	RadiusNone RadiusToken = iota
	RadiusS
	RadiusM
	RadiusL
)

type defaultContext struct {
	tokens Tokens
}

func (c defaultContext) Color(t ColorToken) gfx.Color {
	switch t {
	case ColorBackground:
		return c.tokens.Color.Background
	case ColorSurface:
		return c.tokens.Color.Surface
	case ColorSurfaceVariant:
		return c.tokens.Color.SurfaceVariant
	case ColorPrimary:
		return c.tokens.Color.Primary
	case ColorOnPrimary:
		return c.tokens.Color.OnPrimary
	case ColorText:
		return c.tokens.Color.OnSurface
	case ColorTextSecondary:
		return c.tokens.Color.OnSurfaceVariant
	case ColorTextDisabled:
		return colorWithAlpha(c.tokens.Color.OnSurfaceVariant, c.tokens.Color.DisabledOpacity)
	case ColorBorder:
		return colorWithAlpha(c.tokens.Color.SurfaceVariant, 0.35)
	case ColorBorderStrong:
		return colorWithAlpha(c.tokens.Color.OnSurfaceVariant, 0.45)
	case ColorSelection:
		return colorWithAlpha(c.tokens.Color.Primary, c.tokens.Color.SelectedOverlay)
	case ColorCaret:
		return c.tokens.Color.Primary
	case ColorError:
		return c.tokens.Color.Error
	case ColorSuccess:
		return c.tokens.Color.Success
	case ColorWarning:
		return c.tokens.Color.Warning
	default:
		return gfx.ColorFromRGBA8(0, 0, 0, 255)
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

func (c defaultContext) Spacing(t SpacingToken) layout.ResolvedScalar {
	switch t {
	case SpacingXS:
		return layout.ResolvedScalar(c.tokens.Spacing.XS)
	case SpacingS:
		return layout.ResolvedScalar(c.tokens.Spacing.SM)
	case SpacingM:
		return layout.ResolvedScalar(c.tokens.Spacing.MD)
	case SpacingL:
		return layout.ResolvedScalar(c.tokens.Spacing.LG)
	case SpacingXL:
		return layout.ResolvedScalar(c.tokens.Spacing.XL)
	case SpacingXXL:
		return layout.ResolvedScalar(c.tokens.Spacing.XXL)
	default:
		return 0
	}
}

func (c defaultContext) TextStyle(t TextToken) text.TextStyle {
	switch t {
	case TextBodyS:
		return c.tokens.Typography.BodySmall
	case TextLabelM:
		return c.tokens.Typography.LabelMedium
	case TextLabelS:
		return c.tokens.Typography.LabelSmall
	case TextHeadingS:
		return c.tokens.Typography.HeadlineSmall
	case TextMonoM:
		return c.tokens.Typography.DataLabel
	case TextMonoS:
		return c.tokens.Typography.DataAnnotation
	case TextBodyM:
		fallthrough
	default:
		return c.tokens.Typography.BodyMedium
	}
}

func (c defaultContext) Radius(t RadiusToken) layout.ResolvedScalar {
	switch t {
	case RadiusNone:
		return layout.ResolvedScalar(c.tokens.Radius.None)
	case RadiusS:
		return layout.ResolvedScalar(c.tokens.Radius.SM)
	case RadiusM:
		return layout.ResolvedScalar(c.tokens.Radius.MD)
	case RadiusL:
		return layout.ResolvedScalar(c.tokens.Radius.LG)
	default:
		return 0
	}
}
