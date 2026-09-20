package studio

import (
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// StudioTokens returns the demo's pinned design token set (RX-1 FR-18). The
// studio defines its own complete token table — colors, type, spacing, radii,
// density — with zero platform/OS reads, so the documentation renders
// identically under any desktop theme. The values pin the blue-family palette
// the studio shipped (the colors are lifted from the framework default so
// visual continuity holds; the set is declared here so the studio's look never
// drifts with the framework default).
func StudioTokens() theme.Tokens {
	return theme.Tokens{
		Color: theme.ColorTokens{
			Background:       gfx.ColorFromRGBA8(248, 248, 250, 255),
			Surface:          gfx.ColorFromRGBA8(255, 255, 255, 255),
			SurfaceVariant:   gfx.ColorFromRGBA8(244, 245, 248, 255),
			SurfaceInverse:   gfx.ColorFromRGBA8(26, 26, 46, 255),
			OnBackground:     gfx.ColorFromRGBA8(26, 26, 46, 255),
			OnSurface:        gfx.ColorFromRGBA8(26, 26, 46, 255),
			OnSurfaceVariant: gfx.ColorFromRGBA8(84, 90, 115, 255),
			Primary:          gfx.ColorFromRGBA8(59, 111, 228, 255),
			PrimaryVariant:   gfx.ColorFromRGBA8(34, 84, 190, 255),
			OnPrimary:        gfx.ColorFromRGBA8(255, 255, 255, 255),
			Secondary:        gfx.ColorFromRGBA8(86, 115, 172, 255),
			SecondaryVariant: gfx.ColorFromRGBA8(59, 92, 144, 255),
			OnSecondary:      gfx.ColorFromRGBA8(255, 255, 255, 255),
			Error:            gfx.ColorFromRGBA8(208, 66, 66, 255),
			Warning:          gfx.ColorFromRGBA8(191, 120, 30, 255),
			Success:          gfx.ColorFromRGBA8(44, 146, 83, 255),
			Info:             gfx.ColorFromRGBA8(43, 118, 194, 255),
			OnError:          gfx.ColorFromRGBA8(255, 255, 255, 255),
			DataPalette: []gfx.Color{
				gfx.ColorFromRGBA8(239, 68, 68, 255),
				gfx.ColorFromRGBA8(249, 115, 22, 255),
				gfx.ColorFromRGBA8(132, 204, 22, 255),
				gfx.ColorFromRGBA8(34, 197, 94, 255),
				gfx.ColorFromRGBA8(20, 184, 166, 255),
				gfx.ColorFromRGBA8(59, 130, 246, 255),
				gfx.ColorFromRGBA8(99, 102, 241, 255),
				gfx.ColorFromRGBA8(236, 72, 153, 255),
			},
			HoverLighten:    0.08,
			PressedDarken:   0.12,
			DisabledOpacity: 0.38,
			SelectedOverlay: 0.16,
		},
		Typography: theme.TypographyTokens{
			DisplayLarge:   studioTextStyle(57, text.WeightRegular, 1.05),
			DisplayMedium:  studioTextStyle(45, text.WeightRegular, 1.05),
			DisplaySmall:   studioTextStyle(36, text.WeightRegular, 1.08),
			HeadlineLarge:  studioTextStyle(32, text.WeightSemiBold, 1.1),
			HeadlineMedium: studioTextStyle(28, text.WeightSemiBold, 1.1),
			HeadlineSmall:  studioTextStyle(24, text.WeightSemiBold, 1.12),
			TitleLarge:     studioTextStyle(22, text.WeightMedium, 1.12),
			TitleMedium:    studioTextStyle(16, text.WeightMedium, 1.2),
			TitleSmall:     studioTextStyle(14, text.WeightMedium, 1.2),
			BodyLarge:      studioTextStyle(16, text.WeightRegular, 1.35),
			BodyMedium:     studioTextStyle(14, text.WeightRegular, 1.35),
			BodySmall:      studioTextStyle(12, text.WeightRegular, 1.35),
			LabelLarge:     studioTextStyle(14, text.WeightMedium, 1.2),
			LabelMedium:    studioTextStyle(12, text.WeightMedium, 1.2),
			LabelSmall:     studioTextStyle(11, text.WeightMedium, 1.18),
			DataLabel:      studioTextStyle(13, text.WeightRegular, 1.15),
			DataAnnotation: studioTextStyle(11, text.WeightRegular, 1.15),
			ChartTitle:     studioTextStyle(20, text.WeightSemiBold, 1.12),
			ChartSubtitle:  studioTextStyle(13, text.WeightRegular, 1.2),
		},
		Fonts: theme.DefaultFontRoles(),
		Spacing: theme.SpacingTokens{
			XXS:           2,
			XS:            4,
			SM:            8,
			MD:            12,
			LG:            16,
			XL:            24,
			XXL:           32,
			IconSize:      20,
			TouchTarget:   44,
			DividerWeight: 1,
			BorderWeight:  1,
		},
		Radius: theme.RadiusTokens{
			None: 0,
			XS:   2,
			SM:   4,
			MD:   8,
			LG:   12,
			Full: 9999,
		},
		Elevation: theme.ElevationTokens{
			Level0: theme.MaterialStroke{},
			Level1: theme.MaterialStroke{Width: 1, BlurRadius: 2, Offset: gfx.Point{Y: 1}},
			Level2: theme.MaterialStroke{Width: 1, BlurRadius: 4, Offset: gfx.Point{Y: 2}},
			Level3: theme.MaterialStroke{Width: 1, BlurRadius: 8, Offset: gfx.Point{Y: 3}},
			Level4: theme.MaterialStroke{Width: 1, BlurRadius: 16, Offset: gfx.Point{Y: 4}},
		},
		Motion: theme.MotionTokens{
			DurationInstant:    0,
			DurationShort:      100 * time.Millisecond,
			DurationMedium:     250 * time.Millisecond,
			DurationLong:       400 * time.Millisecond,
			DurationXLong:      600 * time.Millisecond,
			EasingStandard:     "standard",
			EasingDecelerate:   "decelerate",
			EasingAccelerate:   "accelerate",
			EasingLinear:       "linear",
			EasingSpring:       "spring",
			HoverDuration:      100 * time.Millisecond,
			PressDuration:      60 * time.Millisecond,
			FocusDuration:      120 * time.Millisecond,
			SelectDuration:     180 * time.Millisecond,
			EnterDuration:      250 * time.Millisecond,
			ExitDuration:       200 * time.Millisecond,
			DataChangeDuration: 300 * time.Millisecond,
		},
		Density: theme.DensityTokens{
			Mode:  theme.DensityComfortable,
			Scale: 1.0,
		},
	}
}

// studioTextStyle is the studio's text-style constructor (pinned line heights).
func studioTextStyle(size float32, weight text.Weight, lineHeight float32) text.TextStyle {
	return text.TextStyle{
		Size:       size,
		Weight:     weight,
		Style:      text.StyleNormal,
		LineHeight: lineHeight,
	}
}
