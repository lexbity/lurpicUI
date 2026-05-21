package templates

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
)

// RecipeContext groups the resolved template inputs used to build recipe bundles.
type RecipeContext struct {
	ThemeName string
	Density   DensityMode

	Tokens     Tokens
	Typography TypographyTokens
	Metrics    ResolvedMetricTokens
	Shape      ShapeTokens
	Motion     MotionTokens
	Fonts      theme.FontRoles
	Chart      ChartTokens

	Materials *theme.MaterialRegistry
}

// RecipeDiagnostics describes the active template and bundle provenance.
type RecipeDiagnostics struct {
	ThemeName           string
	Density             DensityMode
	MissingTokens       []string
	MissingRecipes      []string
	InvalidDensityScale []string
	BundleNames         []string
	ChartOverride       bool
	ChartFallbackFields []string
	ChartUsesFallback   bool
	ChartPaletteSize    int
	HasMaterialRegistry bool
}

// ResolveInputs resolves the template into density-scaled inputs.
func (t TemplateTheme) ResolveInputs(mode DensityMode) RecipeContext {
	resolvedCharts := t.Charts.Resolve(t.Tokens.Color)
	return RecipeContext{
		ThemeName:  t.Name,
		Density:    mode,
		Tokens:     t.Tokens,
		Typography: ScaleTypographyForDensity(t.Tokens.Typography, mode),
		Metrics:    ScaleMetricsForDensity(t.Tokens.Metrics, mode),
		Shape:      t.Tokens.Shape,
		Motion:     t.Tokens.Motion,
		Fonts:      t.Fonts,
		Chart:      resolvedCharts,
		Materials:  t.Materials,
	}
}

// Diagnostics returns a compact summary for logging and debugging.
func (t TemplateTheme) Diagnostics(mode DensityMode) RecipeDiagnostics {
	report := t.ValidationReport(mode)
	return RecipeDiagnostics{
		ThemeName:           t.Name,
		Density:             mode,
		MissingTokens:       append([]string(nil), report.MissingTokens...),
		MissingRecipes:      append([]string(nil), report.MissingRecipes...),
		InvalidDensityScale: append([]string(nil), report.InvalidDensityScaling...),
		BundleNames:         t.Recipes.BundleNames(),
		ChartOverride:       t.Charts.HasOverrides(),
		ChartFallbackFields: append([]string(nil), report.ChartFallbackFields...),
		ChartUsesFallback:   len(report.ChartFallbackFields) > 0,
		ChartPaletteSize:    len(t.Charts.Resolve(t.Tokens.Color).DataPalette),
		HasMaterialRegistry: t.Materials != nil,
	}
}

// DefaultRecipeBundle returns the canonical phase-3 family bundle set.
func DefaultRecipeBundle() RecipeBundle {
	return NewRecipeBundle(
		AnnotationRecipeBundle(),
		UIInputRecipeBundle(),
		UINavRecipeBundle(),
		UINotificationRecipeBundle(),
		ChartRecipeBundle(),
	)
}

// AnnotationRecipeBundle returns the canonical annotation bundle catalog.
func AnnotationRecipeBundle() FamilyRecipeBundle {
	return NewFamilyRecipeBundle("annotation",
		"label/standard",
		"label/compact",
		"handle/standard",
	)
}

// UIInputRecipeBundle returns the canonical input bundle catalog.
func UIInputRecipeBundle() FamilyRecipeBundle {
	return NewFamilyRecipeBundle("uiinput",
		"button/filled",
		"button/outlined",
		"button/text",
		"button/tonal",
		"checkbox/standard",
		"radiogroup/standard",
		"select/standard",
		"slider/standard",
		"slider/compact",
		"switch/standard",
		"textinput/outlined",
		"textinput/filled",
		"textinput/underlined",
	)
}

// UINavRecipeBundle returns the canonical navigation bundle catalog.
func UINavRecipeBundle() FamilyRecipeBundle {
	return NewFamilyRecipeBundle("uinav",
		"tabs/standard",
		"tabs/compact",
		"menu/standard",
		"menu/dense",
		"drawer/standard",
		"nav-drawer/standard",
		"nav-rail/standard",
		"tree-navigator/standard",
		"breadcrumbs/standard",
		"pagination/standard",
		"speeddial/standard",
		"scrollbar/standard",
	)
}

// UINotificationRecipeBundle returns the canonical notification bundle catalog.
func UINotificationRecipeBundle() FamilyRecipeBundle {
	return NewFamilyRecipeBundle("uinotification",
		"snackbar/standard",
		"dialog/standard",
		"dialog/destructive",
		"dialog/fullscreen",
		"notification/standard",
		"progress/standard",
	)
}

// ChartRecipeBundle returns the canonical chart bundle catalog.
func ChartRecipeBundle() FamilyRecipeBundle {
	return NewFamilyRecipeBundle("chart",
		"axis/standard",
		"axis/compact",
	)
}

// DefaultTemplateTheme returns a canonical template theme with phase-3 defaults.
func DefaultTemplateTheme(name string) TemplateTheme {
	return TemplateTheme{
		Name:      name,
		Tokens:    DefaultTemplateTokens(),
		Fonts:     theme.DefaultFontRoles(),
		Materials: theme.NewMaterialRegistry(),
		Recipes:   DefaultRecipeBundle(),
		Metadata: ThemeMetadata{
			BaselineDensity:     DensityRegular,
			SupportsCompact:     true,
			SupportsRegular:     true,
			SupportsTouchspread: true,
		},
	}
}

// DefaultTemplateTokens returns the canonical token set used by the shipped templates.
func DefaultTemplateTokens() Tokens {
	return Tokens{
		Color: ColorTokens{
			Background:           gfx.ColorFromRGBA8(255, 255, 255, 255),
			Surface:              gfx.ColorFromRGBA8(243, 243, 243, 255),
			SurfaceVariant:       gfx.ColorFromRGBA8(236, 236, 236, 255),
			SurfaceContainerLow:  gfx.ColorFromRGBA8(255, 255, 255, 255),
			SurfaceContainer:     gfx.ColorFromRGBA8(243, 243, 243, 255),
			SurfaceContainerHigh: gfx.ColorFromRGBA8(231, 231, 231, 255),
			SurfaceInverse:       gfx.ColorFromRGBA8(44, 44, 44, 255),
			OnBackground:         gfx.ColorFromRGBA8(0, 0, 0, 255),
			OnSurface:            gfx.ColorFromRGBA8(51, 51, 51, 255),
			OnSurfaceVariant:     gfx.ColorFromRGBA8(97, 97, 97, 255),
			Outline:              gfx.ColorFromRGBA8(200, 200, 200, 255),
			OutlineVariant:       gfx.ColorFromRGBA8(213, 213, 213, 255),
			Primary:              gfx.ColorFromRGBA8(0, 122, 204, 255),
			OnPrimary:            gfx.ColorFromRGBA8(255, 255, 255, 255),
			PrimaryContainer:     gfx.ColorFromRGBA8(214, 235, 255, 255),
			OnPrimaryContainer:   gfx.ColorFromRGBA8(0, 62, 115, 255),
			Secondary:            gfx.ColorFromRGBA8(95, 106, 121, 255),
			OnSecondary:          gfx.ColorFromRGBA8(255, 255, 255, 255),
			SecondaryContainer:   gfx.ColorFromRGBA8(228, 230, 241, 255),
			OnSecondaryContainer: gfx.ColorFromRGBA8(47, 54, 64, 255),
			Tertiary:             gfx.ColorFromRGBA8(0, 106, 177, 255),
			OnTertiary:           gfx.ColorFromRGBA8(255, 255, 255, 255),
			TertiaryContainer:    gfx.ColorFromRGBA8(214, 235, 255, 255),
			OnTertiaryContainer:  gfx.ColorFromRGBA8(0, 58, 99, 255),
			Error:                gfx.ColorFromRGBA8(229, 20, 0, 255),
			OnError:              gfx.ColorFromRGBA8(255, 255, 255, 255),
			Warning:              gfx.ColorFromRGBA8(233, 167, 0, 255),
			OnWarning:            gfx.ColorFromRGBA8(28, 20, 0, 255),
			Success:              gfx.ColorFromRGBA8(0, 113, 0, 255),
			OnSuccess:            gfx.ColorFromRGBA8(255, 255, 255, 255),
			Info:                 gfx.ColorFromRGBA8(117, 190, 255, 255),
			OnInfo:               gfx.ColorFromRGBA8(10, 39, 64, 255),
			HoverOpacity:         0.06,
			PressedOpacity:       0.12,
			FocusOpacity:         0.18,
			DisabledOpacity:      0.36,
			SelectionOpacity:     0.20,
			DataPalette: []gfx.Color{
				gfx.ColorFromRGBA8(0, 122, 204, 255),
				gfx.ColorFromRGBA8(229, 20, 0, 255),
				gfx.ColorFromRGBA8(0, 113, 0, 255),
				gfx.ColorFromRGBA8(137, 85, 3, 255),
				gfx.ColorFromRGBA8(95, 106, 121, 255),
				gfx.ColorFromRGBA8(117, 190, 255, 255),
				gfx.ColorFromRGBA8(104, 33, 122, 255),
			},
			AxisStrong: gfx.ColorFromRGBA8(51, 51, 51, 255),
			AxisSubtle: gfx.ColorFromRGBA8(97, 97, 97, 255),
			GridStrong: gfx.ColorFromRGBA8(211, 211, 211, 255),
			GridSubtle: gfx.ColorFromRGBA8(236, 236, 236, 255),
		},
		Typography: DefaultTypographyTokens(),
		Metrics:    DefaultMetricTokens(),
		Shape: ShapeTokens{
			RadiusNone: 0,
			RadiusXS:   2,
			RadiusSM:   4,
			RadiusMD:   8,
			RadiusLG:   12,
			RadiusXL:   16,
			RadiusFull: 9999,
		},
		Motion: MotionTokens{
			DurationFast:     120 * time.Millisecond,
			DurationMedium:   180 * time.Millisecond,
			DurationSlow:     260 * time.Millisecond,
			EasingStandard:   "cubic-bezier(0.2, 0.0, 0.0, 1.0)",
			EasingEmphasized: "cubic-bezier(0.2, 0.0, 0.0, 1.2)",
			EasingExit:       "cubic-bezier(0.4, 0.0, 1.0, 1.0)",
			SpringLight:      SpringToken{Tension: 170, Friction: 24},
			SpringMedium:     SpringToken{Tension: 220, Friction: 28},
		},
	}
}
