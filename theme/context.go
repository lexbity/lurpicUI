package theme

import (
	"codeburg.org/lexbit/lurpicui/gfx"
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

// Context provides named visual tokens.
type Context interface {
	Color(t ColorToken) gfx.Color
	Spacing(t SpacingToken) float32
	TextStyle(t TextToken) text.TextStyle
	Radius(t RadiusToken) float32
}

type defaultContext struct {
	tokens Tokens
}

var _ Context = defaultContext{}

// Default returns the default theme context.
func Default() Context {
	return defaultContext{tokens: DefaultTokens()}
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

func (c defaultContext) Spacing(t SpacingToken) float32 {
	switch t {
	case SpacingXS:
		return c.tokens.Spacing.XS
	case SpacingS:
		return c.tokens.Spacing.SM
	case SpacingM:
		return c.tokens.Spacing.MD
	case SpacingL:
		return c.tokens.Spacing.LG
	case SpacingXL:
		return c.tokens.Spacing.XL
	case SpacingXXL:
		return c.tokens.Spacing.XXL
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

func (c defaultContext) Radius(t RadiusToken) float32 {
	switch t {
	case RadiusNone:
		return c.tokens.Radius.None
	case RadiusS:
		return c.tokens.Radius.SM
	case RadiusM:
		return c.tokens.Radius.MD
	case RadiusL:
		return c.tokens.Radius.LG
	default:
		return 0
	}
}
