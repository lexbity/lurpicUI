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

type defaultContext struct{}

var _ Context = defaultContext{}

// Default returns the default theme context.
func Default() Context {
	return defaultContext{}
}

func (defaultContext) Color(t ColorToken) gfx.Color {
	switch t {
	case ColorBackground:
		return gfx.ColorFromRGBA8(248, 248, 250, 255)
	case ColorSurface:
		return gfx.ColorFromRGBA8(255, 255, 255, 255)
	case ColorSurfaceVariant:
		return gfx.ColorFromRGBA8(244, 245, 248, 255)
	case ColorPrimary:
		return gfx.ColorFromRGBA8(59, 111, 228, 255)
	case ColorOnPrimary:
		return gfx.ColorFromRGBA8(255, 255, 255, 255)
	case ColorText:
		return gfx.ColorFromRGBA8(26, 26, 46, 255)
	case ColorTextSecondary:
		return gfx.ColorFromRGBA8(84, 90, 115, 255)
	case ColorTextDisabled:
		return gfx.ColorFromRGBA8(132, 137, 158, 255)
	case ColorBorder:
		return gfx.ColorFromRGBA8(0, 0, 0, 31)
	case ColorBorderStrong:
		return gfx.ColorFromRGBA8(0, 0, 0, 61)
	case ColorSelection:
		return gfx.ColorFromRGBA8(59, 111, 228, 51)
	case ColorCaret:
		return gfx.ColorFromRGBA8(59, 111, 228, 255)
	case ColorError:
		return gfx.ColorFromRGBA8(208, 66, 66, 255)
	case ColorSuccess:
		return gfx.ColorFromRGBA8(44, 146, 83, 255)
	case ColorWarning:
		return gfx.ColorFromRGBA8(191, 120, 30, 255)
	default:
		return gfx.ColorFromRGBA8(0, 0, 0, 255)
	}
}

func (defaultContext) Spacing(t SpacingToken) float32 {
	switch t {
	case SpacingXS:
		return 4
	case SpacingS:
		return 8
	case SpacingM:
		return 12
	case SpacingL:
		return 16
	case SpacingXL:
		return 24
	case SpacingXXL:
		return 32
	default:
		return 0
	}
}

func (defaultContext) TextStyle(t TextToken) text.TextStyle {
	switch t {
	case TextBodyS:
		return text.TextStyle{
			Family:     "sans-serif",
			Size:       12,
			Weight:     text.WeightRegular,
			Style:      text.StyleNormal,
			LineHeight: 1.2,
		}
	case TextLabelM:
		return text.TextStyle{
			Family:     "sans-serif",
			Size:       12,
			Weight:     text.WeightMedium,
			Style:      text.StyleNormal,
			LineHeight: 1.2,
		}
	case TextLabelS:
		return text.TextStyle{
			Family:     "sans-serif",
			Size:       11,
			Weight:     text.WeightMedium,
			Style:      text.StyleNormal,
			LineHeight: 1.15,
		}
	case TextHeadingS:
		return text.TextStyle{
			Family:     "sans-serif",
			Size:       18,
			Weight:     text.WeightSemiBold,
			Style:      text.StyleNormal,
			LineHeight: 1.15,
		}
	case TextMonoM:
		return text.TextStyle{
			Family:     "monospace",
			Size:       13,
			Weight:     text.WeightRegular,
			Style:      text.StyleNormal,
			LineHeight: 1.15,
		}
	case TextMonoS:
		return text.TextStyle{
			Family:     "monospace",
			Size:       11,
			Weight:     text.WeightRegular,
			Style:      text.StyleNormal,
			LineHeight: 1.15,
		}
	case TextBodyM:
		fallthrough
	default:
		return text.TextStyle{
			Family:     "sans-serif",
			Size:       14,
			Weight:     text.WeightRegular,
			Style:      text.StyleNormal,
			LineHeight: 1.2,
		}
	}
}

func (defaultContext) Radius(t RadiusToken) float32 {
	switch t {
	case RadiusNone:
		return 0
	case RadiusS:
		return 4
	case RadiusM:
		return 8
	case RadiusL:
		return 12
	default:
		return 0
	}
}
