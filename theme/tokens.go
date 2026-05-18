package theme

import (
	"fmt"
	"strings"
	"time"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
)

// ColorTokens groups semantic color roles.
type ColorTokens struct {
	Background       gfx.Color
	Surface          gfx.Color
	SurfaceVariant   gfx.Color
	SurfaceInverse   gfx.Color
	OnBackground     gfx.Color
	OnSurface        gfx.Color
	OnSurfaceVariant gfx.Color
	Primary          gfx.Color
	PrimaryVariant   gfx.Color
	OnPrimary        gfx.Color
	Secondary        gfx.Color
	SecondaryVariant gfx.Color
	OnSecondary      gfx.Color
	Error            gfx.Color
	Warning          gfx.Color
	Success          gfx.Color
	Info             gfx.Color
	OnError          gfx.Color
	DataPalette      []gfx.Color
	HoverLighten     float32
	PressedDarken    float32
	DisabledOpacity  float32
	SelectedOverlay  float32
}

// TypographyTokens groups text styles by semantic scale.
type TypographyTokens struct {
	DisplayLarge   text.TextStyle
	DisplayMedium  text.TextStyle
	DisplaySmall   text.TextStyle
	HeadlineLarge  text.TextStyle
	HeadlineMedium text.TextStyle
	HeadlineSmall  text.TextStyle
	TitleLarge     text.TextStyle
	TitleMedium    text.TextStyle
	TitleSmall     text.TextStyle
	BodyLarge      text.TextStyle
	BodyMedium     text.TextStyle
	BodySmall      text.TextStyle
	LabelLarge     text.TextStyle
	LabelMedium    text.TextStyle
	LabelSmall     text.TextStyle
	DataLabel      text.TextStyle
	DataAnnotation text.TextStyle
	ChartTitle     text.TextStyle
	ChartSubtitle  text.TextStyle
}

// SpacingTokens defines a logical spacing scale.
type SpacingTokens struct {
	XXS float32
	XS  float32
	SM  float32
	MD  float32
	LG  float32
	XL  float32
	XXL float32

	IconSize      float32
	TouchTarget   float32
	DividerWeight float32
	BorderWeight  float32
}

// RadiusTokens defines a corner radius scale.
type RadiusTokens struct {
	None float32
	XS   float32
	SM   float32
	MD   float32
	LG   float32
	Full float32
}

// ElevationTokens defines shadow-like elevations.
type ElevationTokens struct {
	Level0 MaterialStroke
	Level1 MaterialStroke
	Level2 MaterialStroke
	Level3 MaterialStroke
	Level4 MaterialStroke
}

// MotionTokens defines animation timing tokens.
type MotionTokens struct {
	DurationInstant    time.Duration
	DurationShort      time.Duration
	DurationMedium     time.Duration
	DurationLong       time.Duration
	DurationXLong      time.Duration
	EasingStandard     string
	EasingDecelerate   string
	EasingAccelerate   string
	EasingLinear       string
	EasingSpring       string
	HoverDuration      time.Duration
	PressDuration      time.Duration
	FocusDuration      time.Duration
	SelectDuration     time.Duration
	EnterDuration      time.Duration
	ExitDuration       time.Duration
	DataChangeDuration time.Duration
}

// DensityMode controls the base density preset.
type DensityMode uint8

const (
	DensityComfortable DensityMode = iota
	DensityCompact
	DensityTouch
)

// DensityTokens controls how spacing is scaled.
type DensityTokens struct {
	Mode  DensityMode
	Scale float32
}

// Tokens is the complete design token set.
type Tokens struct {
	Color      ColorTokens
	Typography TypographyTokens
	Spacing    SpacingTokens
	Radius     RadiusTokens
	Elevation  ElevationTokens
	Motion     MotionTokens
	Density    DensityTokens
}

// DefaultTokens returns the default theme token set.
func DefaultTokens() Tokens {
	return Tokens{
		Color: defaultColorTokens(),
		Typography: TypographyTokens{
			DisplayLarge:   textStyle("sans-serif", 57, text.WeightRegular, 1.05),
			DisplayMedium:  textStyle("sans-serif", 45, text.WeightRegular, 1.05),
			DisplaySmall:   textStyle("sans-serif", 36, text.WeightRegular, 1.08),
			HeadlineLarge:  textStyle("sans-serif", 32, text.WeightSemiBold, 1.1),
			HeadlineMedium: textStyle("sans-serif", 28, text.WeightSemiBold, 1.1),
			HeadlineSmall:  textStyle("sans-serif", 24, text.WeightSemiBold, 1.12),
			TitleLarge:     textStyle("sans-serif", 22, text.WeightMedium, 1.12),
			TitleMedium:    textStyle("sans-serif", 16, text.WeightMedium, 1.2),
			TitleSmall:     textStyle("sans-serif", 14, text.WeightMedium, 1.2),
			BodyLarge:      textStyle("sans-serif", 16, text.WeightRegular, 1.35),
			BodyMedium:     textStyle("sans-serif", 14, text.WeightRegular, 1.35),
			BodySmall:      textStyle("sans-serif", 12, text.WeightRegular, 1.35),
			LabelLarge:     textStyle("sans-serif", 14, text.WeightMedium, 1.2),
			LabelMedium:    textStyle("sans-serif", 12, text.WeightMedium, 1.2),
			LabelSmall:     textStyle("sans-serif", 11, text.WeightMedium, 1.18),
			DataLabel:      textStyle("monospace", 13, text.WeightRegular, 1.15),
			DataAnnotation: textStyle("monospace", 11, text.WeightRegular, 1.15),
			ChartTitle:     textStyle("sans-serif", 20, text.WeightSemiBold, 1.12),
			ChartSubtitle:  textStyle("sans-serif", 13, text.WeightRegular, 1.2),
		},
		Spacing: SpacingTokens{
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
		Radius: RadiusTokens{
			None: 0,
			XS:   2,
			SM:   4,
			MD:   8,
			LG:   12,
			Full: 9999,
		},
		Elevation: ElevationTokens{
			Level0: MaterialStroke{},
			Level1: MaterialStroke{Width: 1, BlurRadius: 2, Offset: gfx.Point{Y: 1}},
			Level2: MaterialStroke{Width: 1, BlurRadius: 4, Offset: gfx.Point{Y: 2}},
			Level3: MaterialStroke{Width: 1, BlurRadius: 8, Offset: gfx.Point{Y: 3}},
			Level4: MaterialStroke{Width: 1, BlurRadius: 16, Offset: gfx.Point{Y: 4}},
		},
		Motion: MotionTokens{
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
		Density: DensityTokens{
			Mode:  DensityComfortable,
			Scale: 1.0,
		},
	}
}

// DarkTokens returns a dark variant of the default token set.
func DarkTokens() Tokens {
	tokens := DefaultTokens()
	tokens.Color = darkColorTokens()
	return tokens
}

func (t Tokens) colorFor(role string) gfx.Color {
	switch normalizeTokenName(role) {
	case "background":
		return t.Color.Background
	case "surface":
		return t.Color.Surface
	case "surfacevariant":
		return t.Color.SurfaceVariant
	case "surfaceinverse":
		return t.Color.SurfaceInverse
	case "onbackground":
		return t.Color.OnBackground
	case "onsurface":
		return t.Color.OnSurface
	case "onsurfacevariant":
		return t.Color.OnSurfaceVariant
	case "primary":
		return t.Color.Primary
	case "primaryvariant":
		return t.Color.PrimaryVariant
	case "onprimary":
		return t.Color.OnPrimary
	case "secondary":
		return t.Color.Secondary
	case "secondaryvariant":
		return t.Color.SecondaryVariant
	case "onsecondary":
		return t.Color.OnSecondary
	case "error":
		return t.Color.Error
	case "warning":
		return t.Color.Warning
	case "success":
		return t.Color.Success
	case "info":
		return t.Color.Info
	case "onerror":
		return t.Color.OnError
	default:
		panic(fmt.Sprintf("theme: unknown color role %q", role))
	}
}

func (t Tokens) textStyleFor(scale string) text.TextStyle {
	switch normalizeTokenName(scale) {
	case "displaylarge":
		return t.Typography.DisplayLarge
	case "displaymedium":
		return t.Typography.DisplayMedium
	case "displaysmall":
		return t.Typography.DisplaySmall
	case "headlinelarge":
		return t.Typography.HeadlineLarge
	case "headlinemedium":
		return t.Typography.HeadlineMedium
	case "headlinesmall":
		return t.Typography.HeadlineSmall
	case "titlelarge":
		return t.Typography.TitleLarge
	case "titlemedium":
		return t.Typography.TitleMedium
	case "titlesmall":
		return t.Typography.TitleSmall
	case "bodylarge":
		return t.Typography.BodyLarge
	case "bodymedium":
		return t.Typography.BodyMedium
	case "bodysmall":
		return t.Typography.BodySmall
	case "labellarge":
		return t.Typography.LabelLarge
	case "labelmedium":
		return t.Typography.LabelMedium
	case "labelsmall":
		return t.Typography.LabelSmall
	case "datalabel":
		return t.Typography.DataLabel
	case "dataannotation":
		return t.Typography.DataAnnotation
	case "charttitle":
		return t.Typography.ChartTitle
	case "chartsubtitle":
		return t.Typography.ChartSubtitle
	default:
		panic(fmt.Sprintf("theme: unknown text style %q", scale))
	}
}

func (t Tokens) spacingFor(size string) float32 {
	switch normalizeTokenName(size) {
	case "xxs":
		return t.Spacing.XXS
	case "xs":
		return t.Spacing.XS
	case "sm":
		return t.Spacing.SM
	case "md":
		return t.Spacing.MD
	case "lg":
		return t.Spacing.LG
	case "xl":
		return t.Spacing.XL
	case "xxl":
		return t.Spacing.XXL
	case "iconsize", "icon":
		return t.Spacing.IconSize
	case "touchtarget", "touchtargetmin", "touch":
		return t.Spacing.TouchTarget
	case "dividerweight", "divider":
		return t.Spacing.DividerWeight
	case "borderweight", "border":
		return t.Spacing.BorderWeight
	default:
		panic(fmt.Sprintf("theme: unknown spacing size %q", size))
	}
}

// Scale multiplies the value by the density scale factor.
//
// Touch mode enforces a minimum touch target size when a scaled value would
// otherwise fall below the default touch target.
func (t Tokens) Scale(value float32) float32 {
	scaled := value * t.Density.Scale
	if t.Density.Mode == DensityTouch && scaled > 0 && scaled < 44 {
		return 44
	}
	return scaled
}

func textStyle(family string, size float32, weight text.Weight, lineHeight float32) text.TextStyle {
	return text.TextStyle{
		Family:     family,
		Size:       size,
		Weight:     weight,
		Style:      text.StyleNormal,
		LineHeight: lineHeight,
	}
}

func defaultColorTokens() ColorTokens {
	return ColorTokens{
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
	}
}

func darkColorTokens() ColorTokens {
	return ColorTokens{
		Background:       gfx.ColorFromRGBA8(17, 19, 26, 255),
		Surface:          gfx.ColorFromRGBA8(24, 28, 37, 255),
		SurfaceVariant:   gfx.ColorFromRGBA8(34, 39, 51, 255),
		SurfaceInverse:   gfx.ColorFromRGBA8(244, 245, 248, 255),
		OnBackground:     gfx.ColorFromRGBA8(246, 247, 250, 255),
		OnSurface:        gfx.ColorFromRGBA8(246, 247, 250, 255),
		OnSurfaceVariant: gfx.ColorFromRGBA8(185, 192, 210, 255),
		Primary:          gfx.ColorFromRGBA8(126, 180, 255, 255),
		PrimaryVariant:   gfx.ColorFromRGBA8(90, 145, 240, 255),
		OnPrimary:        gfx.ColorFromRGBA8(12, 16, 24, 255),
		Secondary:        gfx.ColorFromRGBA8(149, 169, 214, 255),
		SecondaryVariant: gfx.ColorFromRGBA8(118, 138, 184, 255),
		OnSecondary:      gfx.ColorFromRGBA8(12, 16, 24, 255),
		Error:            gfx.ColorFromRGBA8(240, 101, 101, 255),
		Warning:          gfx.ColorFromRGBA8(249, 173, 66, 255),
		Success:          gfx.ColorFromRGBA8(96, 194, 126, 255),
		Info:             gfx.ColorFromRGBA8(97, 166, 233, 255),
		OnError:          gfx.ColorFromRGBA8(12, 16, 24, 255),
		DataPalette: []gfx.Color{
			gfx.ColorFromRGBA8(248, 113, 113, 255),
			gfx.ColorFromRGBA8(251, 146, 60, 255),
			gfx.ColorFromRGBA8(163, 230, 53, 255),
			gfx.ColorFromRGBA8(74, 222, 128, 255),
			gfx.ColorFromRGBA8(45, 212, 191, 255),
			gfx.ColorFromRGBA8(96, 165, 250, 255),
			gfx.ColorFromRGBA8(129, 140, 248, 255),
			gfx.ColorFromRGBA8(244, 114, 182, 255),
		},
		HoverLighten:    0.08,
		PressedDarken:   0.12,
		DisabledOpacity: 0.38,
		SelectedOverlay: 0.16,
	}
}

func normalizeTokenName(s string) string {
	s = strings.TrimSpace(strings.ToLower(s))
	s = strings.NewReplacer("-", "", "_", "", " ", "", "\t", "", "\n", "", "\r", "").Replace(s)
	return s
}
