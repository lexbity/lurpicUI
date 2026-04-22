package templates

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/text"
)

// FontRole describes one logical font role and its fallback chain.
type FontRole struct {
	PreferredFamilies []string
	DefaultStyle      text.TextStyle
}

// Families returns a copy of the preferred fallback chain.
func (r FontRole) Families() []string {
	if len(r.PreferredFamilies) == 0 {
		return nil
	}
	out := make([]string, len(r.PreferredFamilies))
	copy(out, r.PreferredFamilies)
	return out
}

// Resolve selects the first installed family from the fallback chain.
//
// If no registered face matches, the first preferred family is returned with
// the default style so callers can preserve the policy while handling the
// missing font explicitly.
func (r FontRole) Resolve(reg *text.FontRegistry) text.TextStyle {
	if reg != nil {
		for _, family := range r.PreferredFamilies {
			style := r.DefaultStyle
			style.Family = family
			if face := reg.Resolve(style); !face.IsZero() {
				return style
			}
		}
	}
	if len(r.PreferredFamilies) > 0 {
		style := r.DefaultStyle
		style.Family = r.PreferredFamilies[0]
		return style
	}
	return r.DefaultStyle
}

// Validate checks that the role has a usable fallback policy.
func (r FontRole) Validate(name string) error {
	if len(r.PreferredFamilies) == 0 {
		return fmt.Errorf("font role %s has no preferred families", name)
	}
	return nil
}

// FontRoles groups the required canonical roles.
type FontRoles struct {
	UISans FontRole
	Mono   FontRole
}

// Validate checks that all required roles have fallback chains.
func (r FontRoles) Validate() error {
	if err := r.UISans.Validate("UISans"); err != nil {
		return err
	}
	if err := r.Mono.Validate("Mono"); err != nil {
		return err
	}
	return nil
}

// DefaultFontRoles returns the phase-2 fallback policy.
func DefaultFontRoles() FontRoles {
	typography := DefaultTypographyTokens()
	return FontRoles{
		UISans: FontRole{
			PreferredFamilies: []string{
				"Inter",
				"Noto Sans",
				"Segoe UI",
				"Roboto",
				"Helvetica",
				"Arial",
			},
			DefaultStyle: typography.BodyMedium,
		},
		Mono: FontRole{
			PreferredFamilies: []string{
				"JetBrains Mono",
				"IBM Plex Mono",
				"Cascadia Mono",
				"Menlo",
				"Consolas",
			},
			DefaultStyle: typography.MonoMedium,
		},
	}
}

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
