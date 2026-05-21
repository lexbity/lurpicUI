package templates

import (
	"codeburg.org/lexbit/lurpicui/text"
)

// DefaultTypographyTokens returns the canonical role-based typography scale.
func DefaultTypographyTokens() TypographyTokens {
	return TypographyTokens{
		DisplayLarge:   typeRoleStyle(36, 44, text.WeightBold, 0.0),
		DisplayMedium:  typeRoleStyle(32, 40, text.WeightBold, 0.0),
		DisplaySmall:   typeRoleStyle(28, 36, text.WeightBold, 0.0),
		HeadlineLarge:  typeRoleStyle(24, 32, text.WeightBold, 0.0),
		HeadlineMedium: typeRoleStyle(22, 30, text.WeightSemiBold, 0.0),
		HeadlineSmall:  typeRoleStyle(20, 28, text.WeightSemiBold, 0.0),
		TitleLarge:     typeRoleStyle(18, 26, text.WeightSemiBold, 0.0),
		TitleMedium:    typeRoleStyle(16, 24, text.WeightSemiBold, 0.0),
		TitleSmall:     typeRoleStyle(14, 20, text.WeightSemiBold, 0.0),
		LabelLarge:     typeRoleStyle(14, 20, text.WeightSemiBold, 0.1),
		LabelMedium:    typeRoleStyle(13, 18, text.WeightSemiBold, 0.1),
		LabelSmall:     typeRoleStyle(12, 16, text.WeightSemiBold, 0.1),
		BodyLarge:      typeRoleStyle(16, 24, text.WeightRegular, 0.0),
		BodyMedium:     typeRoleStyle(14, 22, text.WeightRegular, 0.0),
		BodySmall:      typeRoleStyle(12, 18, text.WeightRegular, 0.0),
		MonoLarge:      typeRoleStyle(14, 22, text.WeightRegular, 0.0),
		MonoMedium:     typeRoleStyle(13, 20, text.WeightRegular, 0.0),
		MonoSmall:      typeRoleStyle(12, 18, text.WeightRegular, 0.0),
	}
}

// ScaleTypographyForDensity applies the canonical density multiplier to a role table.
func ScaleTypographyForDensity(base TypographyTokens, mode DensityMode) TypographyTokens {
	scaleStyle := func(style text.TextStyle) text.TextStyle {
		style.Size = ScaleTypographySize(style.Size, mode)
		style.LineHeight = ScaleTypographyLineHeight(style.LineHeight, mode)
		return style
	}
	return TypographyTokens{
		DisplayLarge:   scaleStyle(base.DisplayLarge),
		DisplayMedium:  scaleStyle(base.DisplayMedium),
		DisplaySmall:   scaleStyle(base.DisplaySmall),
		HeadlineLarge:  scaleStyle(base.HeadlineLarge),
		HeadlineMedium: scaleStyle(base.HeadlineMedium),
		HeadlineSmall:  scaleStyle(base.HeadlineSmall),
		TitleLarge:     scaleStyle(base.TitleLarge),
		TitleMedium:    scaleStyle(base.TitleMedium),
		TitleSmall:     scaleStyle(base.TitleSmall),
		LabelLarge:     scaleStyle(base.LabelLarge),
		LabelMedium:    scaleStyle(base.LabelMedium),
		LabelSmall:     scaleStyle(base.LabelSmall),
		BodyLarge:      scaleStyle(base.BodyLarge),
		BodyMedium:     scaleStyle(base.BodyMedium),
		BodySmall:      scaleStyle(base.BodySmall),
		MonoLarge:      scaleStyle(base.MonoLarge),
		MonoMedium:     scaleStyle(base.MonoMedium),
		MonoSmall:      scaleStyle(base.MonoSmall),
	}
}

func typeRoleStyle(size, lineHeight float32, weight text.Weight, letterSpacing float32) text.TextStyle {
	return text.TextStyle{
		Size:          size,
		Weight:        weight,
		Style:         text.StyleNormal,
		LineHeight:    lineHeight,
		LetterSpacing: letterSpacing,
	}
}
