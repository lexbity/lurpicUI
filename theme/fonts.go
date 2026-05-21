package theme

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/text"
)

// FontRole describes one logical font role and its ordered fallback chain.
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

// Resolve selects the first available family from the fallback chain.
func (r FontRole) Resolve(reg *text.FontRegistry) text.TextStyle {
	return r.ResolveStyle(r.DefaultStyle, reg)
}

// ResolveStyle selects the first available family from the fallback chain while preserving style metrics.
func (r FontRole) ResolveStyle(style text.TextStyle, reg *text.FontRegistry) text.TextStyle {
	if len(r.PreferredFamilies) == 0 {
		return style
	}
	candidate := style
	for _, family := range r.PreferredFamilies {
		candidate.Family = family
		if reg == nil {
			return candidate
		}
		if face := reg.Resolve(candidate); !face.IsZero() {
			return candidate
		}
	}
	if reg != nil {
		firstFam := reg.FirstFamily()
		if firstFam != "" {
			candidate.Family = firstFam
			return candidate
		}
		panic(fmt.Sprintf("theme: FontRegistry contains no registered font faces to resolve role with PreferredFamilies=%v", r.PreferredFamilies))
	}
	candidate.Family = r.PreferredFamilies[0]
	return candidate
}

// Validate checks that the role has an explicit fallback policy.
func (r FontRole) Validate(name string) error {
	if len(r.PreferredFamilies) == 0 {
		return fmt.Errorf("theme: font role %s has no preferred families", name)
	}
	for _, family := range r.PreferredFamilies {
		if isGenericFamilyName(family) {
			return fmt.Errorf("theme: font role %s uses generic family %q", name, family)
		}
	}
	return nil
}

// FontRoles groups the canonical text-role bundles.
type FontRoles struct {
	UISans FontRole
	Mono   FontRole
}

// Validate checks that all required roles have explicit fallback chains.
func (r FontRoles) Validate() error {
	if err := r.UISans.Validate("UISans"); err != nil {
		return err
	}
	if err := r.Mono.Validate("Mono"); err != nil {
		return err
	}
	return nil
}

// ResolveTextStyle resolves the supplied style through the canonical role bundle.
func (r FontRoles) ResolveTextStyle(token TextToken, style text.TextStyle, reg *text.FontRegistry) text.TextStyle {
	role := r.roleFor(token)
	if role == nil {
		return style
	}
	return role.ResolveStyle(style, reg)
}

func (r FontRoles) roleFor(token TextToken) *FontRole {
	switch token {
	case TextMonoM, TextMonoS:
		return &r.Mono
	case TextBodyM, TextBodyS, TextLabelM, TextLabelS, TextHeadingS:
		fallthrough
	default:
		return &r.UISans
	}
}

// DefaultFontRoles returns the explicit fallback policy used by the core theme.
func DefaultFontRoles() FontRoles {
	return FontRoles{
		UISans: FontRole{
			PreferredFamilies: []string{
				"Noto Sans",
				"Inter",
				"Segoe UI",
				"Roboto",
				"Helvetica Neue",
				"Arial",
				"Liberation Sans",
			},
			DefaultStyle: text.TextStyle{Size: 14, Weight: text.WeightRegular, Style: text.StyleNormal, LineHeight: 22},
		},
		Mono: FontRole{
			PreferredFamilies: []string{
				"Noto Sans Mono",
				"JetBrains Mono",
				"IBM Plex Mono",
				"Cascadia Mono",
				"Menlo",
				"Consolas",
				"Liberation Mono",
			},
			DefaultStyle: text.TextStyle{Size: 13, Weight: text.WeightRegular, Style: text.StyleNormal, LineHeight: 20},
		},
	}
}

func isGenericFamilyName(name string) bool {
	switch normalizeTokenName(name) {
	case "sansserif", "serif", "monospace", "systemui", "uiserif", "uisansserif", "uimonospace", "emoji", "math", "fangsong":
		return true
	default:
		return false
	}
}
