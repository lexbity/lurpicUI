package ui

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	catalogstore "codeburg.org/lexbit/ui_catalog/store"
)

// NewCatalogThemeContext returns a live theme context that tracks the catalog theme store.
func NewCatalogThemeContext() theme.Context {
	return catalogThemeContext{}
}

type catalogThemeContext struct{}

func (catalogThemeContext) Color(t theme.ColorToken) gfx.Color {
	tokens := currentCatalogTokens()
	switch t {
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
		return tokens.Color.OnSurfaceVariant
	case theme.ColorBorder:
		return tokens.Color.SurfaceVariant.WithAlpha(0.35)
	case theme.ColorBorderStrong:
		return tokens.Color.OnSurfaceVariant.WithAlpha(0.45)
	case theme.ColorSelection:
		return tokens.Color.Primary.WithAlpha(tokens.Color.SelectedOverlay)
	case theme.ColorCaret:
		return tokens.Color.Primary
	case theme.ColorError:
		return tokens.Color.Error
	case theme.ColorSuccess:
		return tokens.Color.Success
	case theme.ColorWarning:
		return tokens.Color.Warning
	default:
		return gfx.ColorFromRGBA8(0, 0, 0, 255)
	}
}

func (catalogThemeContext) Spacing(t theme.SpacingToken) float32 {
	tokens := currentCatalogTokens()
	switch t {
	case theme.SpacingXS:
		return tokens.Spacing.XS
	case theme.SpacingS:
		return tokens.Spacing.SM
	case theme.SpacingM:
		return tokens.Spacing.MD
	case theme.SpacingL:
		return tokens.Spacing.LG
	case theme.SpacingXL:
		return tokens.Spacing.XL
	case theme.SpacingXXL:
		return tokens.Spacing.XXL
	default:
		return 0
	}
}

func (catalogThemeContext) TextStyle(t theme.TextToken) text.TextStyle {
	tokens := currentCatalogTokens()
	switch t {
	case theme.TextBodyS:
		return tokens.Typography.BodySmall
	case theme.TextLabelM:
		return tokens.Typography.LabelMedium
	case theme.TextLabelS:
		return tokens.Typography.LabelSmall
	case theme.TextHeadingS:
		return tokens.Typography.HeadlineSmall
	case theme.TextMonoM:
		return tokens.Typography.DataLabel
	case theme.TextMonoS:
		return tokens.Typography.DataAnnotation
	case theme.TextBodyM:
		fallthrough
	default:
		return tokens.Typography.BodyMedium
	}
}

func (catalogThemeContext) Radius(t theme.RadiusToken) float32 {
	tokens := currentCatalogTokens()
	switch t {
	case theme.RadiusNone:
		return tokens.Radius.None
	case theme.RadiusS:
		return tokens.Radius.SM
	case theme.RadiusM:
		return tokens.Radius.MD
	case theme.RadiusL:
		return tokens.Radius.LG
	default:
		return 0
	}
}

func currentCatalogTokens() theme.Tokens {
	if catalogstore.GetTheme() == catalogstore.ThemeDark {
		return theme.DarkTokens()
	}
	return theme.DefaultTokens()
}
