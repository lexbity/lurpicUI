package templates

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	legacytheme "codeburg.org/lexbit/lurpicui/theme"
)

// UneNuit returns the balanced dark shipped template theme.
func UneNuit() TemplateTheme {
	return shippedTheme(ThemeSpec{
		Name:            "uneNuit",
		Dark:            true,
		BaselineDensity: DensityRegular,
		Colors: ColorTokens{
			Background:           colorHex(0x282C34FF),
			Surface:              colorHex(0x21252BFF),
			SurfaceVariant:       colorHex(0x2C313AFF),
			SurfaceContainerLow:  colorHex(0x21252BFF),
			SurfaceContainer:     colorHex(0x2C313AFF),
			SurfaceContainerHigh: colorHex(0x404754FF),
			SurfaceInverse:       colorHex(0xABB2BFFF),
			OnBackground:         colorHex(0xABB2BFFF),
			OnSurface:            colorHex(0xABB2BFFF),
			OnSurfaceVariant:     colorHex(0x7F848EFF),
			Outline:              colorHex(0x3E4452FF),
			OutlineVariant:       colorHex(0x495162FF),
			Primary:              colorHex(0x61AFEFFF),
			OnPrimary:            colorHex(0x11151CFF),
			PrimaryContainer:     colorHex(0x314365FF),
			OnPrimaryContainer:   colorHex(0xD7E9FFFF),
			Secondary:            colorHex(0x56B6C2FF),
			OnSecondary:          colorHex(0x0F1618FF),
			SecondaryContainer:   colorHex(0x1F3A40FF),
			OnSecondaryContainer: colorHex(0xCDEEF2FF),
			Tertiary:             colorHex(0xC678DDFF),
			OnTertiary:           colorHex(0x140F16FF),
			TertiaryContainer:    colorHex(0x3B2542FF),
			OnTertiaryContainer:  colorHex(0xF0D8F6FF),
			Error:                colorHex(0xE06C75FF),
			OnError:              colorHex(0x190E10FF),
			Warning:              colorHex(0xE5C07BFF),
			OnWarning:            colorHex(0x1A1510FF),
			Success:              colorHex(0x98C379FF),
			OnSuccess:            colorHex(0x11170DFF),
			Info:                 colorHex(0x56B6C2FF),
			OnInfo:               colorHex(0x0F1618FF),
			HoverOpacity:         0.08,
			PressedOpacity:       0.14,
			FocusOpacity:         0.18,
			DisabledOpacity:      0.38,
			SelectionOpacity:     0.22,
			DataPalette: []gfx.Color{
				colorHex(0x61AFEFFF),
				colorHex(0xE06C75FF),
				colorHex(0x98C379FF),
				colorHex(0xE5C07BFF),
				colorHex(0xC678DDFF),
				colorHex(0x56B6C2FF),
				colorHex(0xD19A66FF),
			},
			AxisStrong: colorHex(0xABB2BFFF),
			AxisSubtle: colorHex(0x7F848EFF),
			GridStrong: colorHex(0x3E4452FF),
			GridSubtle: colorHex(0x2C313AFF),
		},
		Chart: ChartInheritance{
			DataPalette: []gfx.Color{
				colorHex(0x61AFEFFF),
				colorHex(0xE06C75FF),
				colorHex(0x98C379FF),
				colorHex(0xE5C07BFF),
				colorHex(0xC678DDFF),
				colorHex(0x56B6C2FF),
				colorHex(0xD19A66FF),
			},
			AxisStrong: colorPtr(colorHex(0xABB2BFFF)),
			AxisSubtle: colorPtr(colorHex(0x7F848EFF)),
			GridStrong: colorPtr(colorHex(0x3E4452FF)),
			GridSubtle: colorPtr(colorHex(0x2C313AFF)),
		},
	})
}

// Sythique returns the neon-accent shipped template theme.
func Sythique() TemplateTheme {
	return shippedTheme(ThemeSpec{
		Name:            "sythique",
		Dark:            true,
		BaselineDensity: DensityCompact,
		Colors: ColorTokens{
			Background:           colorHex(0x262335FF),
			Surface:              colorHex(0x241B2FFF),
			SurfaceVariant:       colorHex(0x2A2139FF),
			SurfaceContainerLow:  colorHex(0x171520FF),
			SurfaceContainer:     colorHex(0x2A2139FF),
			SurfaceContainerHigh: colorHex(0x34294FFF),
			SurfaceInverse:       colorHex(0xFFFFFFFF),
			OnBackground:         colorHex(0xFFFFFFFF),
			OnSurface:            colorHex(0xFFFFFFFF),
			OnSurfaceVariant:     colorHex(0xB6B1B1FF),
			Outline:              colorHex(0x495495FF),
			OutlineVariant:       colorHex(0x7059ABFF),
			Primary:              colorHex(0x36F9F6FF),
			OnPrimary:            colorHex(0x091113FF),
			PrimaryContainer:     colorHex(0x113843FF),
			OnPrimaryContainer:   colorHex(0xCFFFFFFF),
			Secondary:            colorHex(0xFF7EDBFF),
			OnSecondary:          colorHex(0x160A14FF),
			SecondaryContainer:   colorHex(0x4A1E40FF),
			OnSecondaryContainer: colorHex(0xFFD5F3FF),
			Tertiary:             colorHex(0x72F1B8FF),
			OnTertiary:           colorHex(0x0B1511FF),
			TertiaryContainer:    colorHex(0x18392EFF),
			OnTertiaryContainer:  colorHex(0xD7FFE9FF),
			Error:                colorHex(0xFE4450FF),
			OnError:              colorHex(0x17090BFF),
			Warning:              colorHex(0xFEDE5DFF),
			OnWarning:            colorHex(0x1A1707FF),
			Success:              colorHex(0x72F1B8FF),
			OnSuccess:            colorHex(0x0B1511FF),
			Info:                 colorHex(0x2EE2FAFF),
			OnInfo:               colorHex(0x081216FF),
			HoverOpacity:         0.10,
			PressedOpacity:       0.16,
			FocusOpacity:         0.20,
			DisabledOpacity:      0.40,
			SelectionOpacity:     0.22,
			DataPalette: []gfx.Color{
				colorHex(0x36F9F6FF),
				colorHex(0xFF7EDBFF),
				colorHex(0x72F1B8FF),
				colorHex(0xFEDE5DFF),
				colorHex(0xF97E72FF),
				colorHex(0xFE4450FF),
				colorHex(0xB893CEFF),
			},
			AxisStrong: colorHex(0xFFFFFFFF),
			AxisSubtle: colorHex(0xB6B1B1FF),
			GridStrong: colorHex(0x7059ABFF),
			GridSubtle: colorHex(0x34294FFF),
		},
		Chart: ChartInheritance{
			DataPalette: []gfx.Color{
				colorHex(0x36F9F6FF),
				colorHex(0xFF7EDBFF),
				colorHex(0x72F1B8FF),
				colorHex(0xFEDE5DFF),
				colorHex(0xF97E72FF),
				colorHex(0xFE4450FF),
				colorHex(0xB893CEFF),
			},
			AxisStrong: colorPtr(colorHex(0xFFFFFFFF)),
			AxisSubtle: colorPtr(colorHex(0xB6B1B1FF)),
			GridStrong: colorPtr(colorHex(0x7059ABFF)),
			GridSubtle: colorPtr(colorHex(0x34294FFF)),
		},
	})
}

// Notes returns the practical light shipped template theme.
func Notes() TemplateTheme {
	return shippedTheme(ThemeSpec{
		Name:            "notes",
		Dark:            false,
		BaselineDensity: DensityRegular,
		Colors: ColorTokens{
			Background:           colorHex(0xFFFFFFFF),
			Surface:              colorHex(0xF3F3F3FF),
			SurfaceVariant:       colorHex(0xECECECFF),
			SurfaceContainerLow:  colorHex(0xFFFFFFFF),
			SurfaceContainer:     colorHex(0xF3F3F3FF),
			SurfaceContainerHigh: colorHex(0xE7E7E7FF),
			SurfaceInverse:       colorHex(0x2C2C2CFF),
			OnBackground:         colorHex(0x000000FF),
			OnSurface:            colorHex(0x333333FF),
			OnSurfaceVariant:     colorHex(0x616161FF),
			Outline:              colorHex(0xC8C8C8FF),
			OutlineVariant:       colorHex(0xD5D5D5FF),
			Primary:              colorHex(0x007ACCFF),
			OnPrimary:            colorHex(0xFFFFFFFF),
			PrimaryContainer:     colorHex(0xD6EBFFFF),
			OnPrimaryContainer:   colorHex(0x003E73FF),
			Secondary:            colorHex(0x5F6A79FF),
			OnSecondary:          colorHex(0xFFFFFFFF),
			SecondaryContainer:   colorHex(0xE4E6F1FF),
			OnSecondaryContainer: colorHex(0x2F3640FF),
			Tertiary:             colorHex(0x006AB1FF),
			OnTertiary:           colorHex(0xFFFFFFFF),
			TertiaryContainer:    colorHex(0xD6EBFFFF),
			OnTertiaryContainer:  colorHex(0x003A63FF),
			Error:                colorHex(0xE51400FF),
			OnError:              colorHex(0xFFFFFFFF),
			Warning:              colorHex(0xE9A700FF),
			OnWarning:            colorHex(0x1C1400FF),
			Success:              colorHex(0x007100FF),
			OnSuccess:            colorHex(0xFFFFFFFF),
			Info:                 colorHex(0x75BEFFFF),
			OnInfo:               colorHex(0x0A2740FF),
			HoverOpacity:         0.06,
			PressedOpacity:       0.12,
			FocusOpacity:         0.18,
			DisabledOpacity:      0.36,
			SelectionOpacity:     0.20,
			DataPalette: []gfx.Color{
				colorHex(0x007ACCFF),
				colorHex(0xE51400FF),
				colorHex(0x007100FF),
				colorHex(0x895503FF),
				colorHex(0x5F6A79FF),
				colorHex(0x75BEFFFF),
				colorHex(0x68217AFF),
			},
			AxisStrong: colorHex(0x333333FF),
			AxisSubtle: colorHex(0x616161FF),
			GridStrong: colorHex(0xD3D3D3FF),
			GridSubtle: colorHex(0xECECECFF),
		},
		Chart: ChartInheritance{
			DataPalette: []gfx.Color{
				colorHex(0x007ACCFF),
				colorHex(0xE51400FF),
				colorHex(0x007100FF),
				colorHex(0x895503FF),
				colorHex(0x5F6A79FF),
				colorHex(0x75BEFFFF),
				colorHex(0x68217AFF),
			},
			AxisStrong: colorPtr(colorHex(0x333333FF)),
			AxisSubtle: colorPtr(colorHex(0x616161FF)),
			GridStrong: colorPtr(colorHex(0xD3D3D3FF)),
			GridSubtle: colorPtr(colorHex(0xECECECFF)),
		},
	})
}

type ThemeSpec struct {
	Name            string
	Dark            bool
	BaselineDensity DensityMode
	Colors          ColorTokens
	Chart           ChartInheritance
}

func shippedTheme(spec ThemeSpec) TemplateTheme {
	return TemplateTheme{
		Name:      spec.Name,
		Tokens:    Tokens{Color: spec.Colors, Typography: DefaultTypographyTokens(), Metrics: DefaultMetricTokens(), Shape: defaultShapeTokens(), Motion: defaultMotionTokens()},
		Fonts:     DefaultFontRoles(),
		Materials: shippedMaterials(spec.Colors),
		Recipes:   DefaultRecipeBundle(),
		Metadata: ThemeMetadata{
			Dark:                spec.Dark,
			BaselineDensity:     spec.BaselineDensity,
			SupportsCompact:     true,
			SupportsRegular:     true,
			SupportsTouchspread: true,
		},
		Charts: spec.Chart,
	}
}

func shippedMaterials(colors ColorTokens) *legacytheme.MaterialRegistry {
	reg := legacytheme.NewMaterialRegistry()
	reg.Define("background", legacytheme.FromToken(colors.Background))
	reg.Define("surface", legacytheme.FromToken(colors.Surface))
	reg.Define("surface-variant", legacytheme.FromToken(colors.SurfaceVariant))
	reg.Define("surface-container", legacytheme.FromToken(colors.SurfaceContainer))
	reg.Define("primary", legacytheme.FromToken(colors.Primary))
	reg.Define("secondary", legacytheme.FromToken(colors.Secondary))
	reg.Define("tertiary", legacytheme.FromToken(colors.Tertiary))
	reg.Define("error", legacytheme.FromToken(colors.Error))
	reg.Define("warning", legacytheme.FromToken(colors.Warning))
	reg.Define("success", legacytheme.FromToken(colors.Success))
	reg.Define("info", legacytheme.FromToken(colors.Info))
	return reg
}

func defaultShapeTokens() ShapeTokens {
	return ShapeTokens{
		RadiusNone: 0,
		RadiusXS:   2,
		RadiusSM:   4,
		RadiusMD:   8,
		RadiusLG:   12,
		RadiusXL:   16,
		RadiusFull: 9999,
	}
}

func defaultMotionTokens() MotionTokens {
	return MotionTokens{
		DurationFast:     120 * 1_000_000,
		DurationMedium:   180 * 1_000_000,
		DurationSlow:     260 * 1_000_000,
		EasingStandard:   "cubic-bezier(0.2, 0.0, 0.0, 1.0)",
		EasingEmphasized: "cubic-bezier(0.2, 0.0, 0.0, 1.2)",
		EasingExit:       "cubic-bezier(0.4, 0.0, 1.0, 1.0)",
		SpringLight:      SpringToken{Tension: 170, Friction: 24},
		SpringMedium:     SpringToken{Tension: 220, Friction: 28},
	}
}

func colorHex(v uint32) gfx.Color {
	return gfx.ColorFromHex(v)
}

func colorPtr(c gfx.Color) *gfx.Color {
	return &c
}
