package shell

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// ThemeMode is the shell-local visual preset used for cycling the shell theme.
type ThemeMode int

const (
	ThemeModeDefault ThemeMode = iota
	ThemeModeNight
	ThemeModeWarm
)

func (m ThemeMode) String() string {
	switch m {
	case ThemeModeNight:
		return "Night"
	case ThemeModeWarm:
		return "Warm"
	default:
		return "Default"
	}
}

// DensityMode is the shell-local density preset used for cycling scene density.
type DensityMode int

const (
	DensityModeCompact DensityMode = iota
	DensityModeNormal
	DensityModeComfortable
)

func (m DensityMode) String() string {
	switch m {
	case DensityModeCompact:
		return "Compact"
	case DensityModeComfortable:
		return "Comfortable"
	default:
		return "Normal"
	}
}

func (m DensityMode) Scale() float32 {
	switch m {
	case DensityModeCompact:
		return 0.9
	case DensityModeComfortable:
		return 1.15
	default:
		return 1.0
	}
}

func nextThemeMode(mode ThemeMode) ThemeMode {
	switch mode {
	case ThemeModeDefault:
		return ThemeModeNight
	case ThemeModeNight:
		return ThemeModeWarm
	default:
		return ThemeModeDefault
	}
}

func nextDensityMode(mode DensityMode) DensityMode {
	switch mode {
	case DensityModeCompact:
		return DensityModeNormal
	case DensityModeNormal:
		return DensityModeComfortable
	default:
		return DensityModeCompact
	}
}

type shellThemeContext struct {
	base  theme.Context
	mode  ThemeMode
	label string
}

var _ theme.Context = shellThemeContext{}

func newShellThemeContext(base theme.Context, mode ThemeMode) shellThemeContext {
	return shellThemeContext{
		base:  base,
		mode:  mode,
		label: mode.String(),
	}
}

func (c shellThemeContext) Color(t theme.ColorToken) gfx.Color {
	base := c.base.Color(t)
	switch c.mode {
	case ThemeModeNight:
		switch t {
		case theme.ColorBackground:
			return tintColor(base, gfx.ColorFromRGBA8(10, 14, 24, 255), 0.25)
		case theme.ColorSurface:
			return tintColor(base, gfx.ColorFromRGBA8(20, 24, 38, 255), 0.2)
		case theme.ColorPrimary:
			return gfx.ColorFromRGBA8(120, 190, 255, 255)
		case theme.ColorSelection:
			return gfx.ColorFromRGBA8(70, 110, 170, 110)
		}
	case ThemeModeWarm:
		switch t {
		case theme.ColorBackground:
			return tintColor(base, gfx.ColorFromRGBA8(36, 28, 18, 255), 0.2)
		case theme.ColorSurface:
			return tintColor(base, gfx.ColorFromRGBA8(54, 42, 26, 255), 0.2)
		case theme.ColorPrimary:
			return gfx.ColorFromRGBA8(220, 150, 95, 255)
		case theme.ColorSelection:
			return gfx.ColorFromRGBA8(180, 110, 70, 110)
		}
	}
	return base
}

func (c shellThemeContext) Spacing(t theme.SpacingToken) float32 {
	return c.base.Spacing(t)
}

func (c shellThemeContext) TextStyle(t theme.TextToken) text.TextStyle {
	return c.base.TextStyle(t)
}

func (c shellThemeContext) Radius(t theme.RadiusToken) float32 {
	return c.base.Radius(t)
}

func tintColor(a, b gfx.Color, mix float32) gfx.Color {
	if mix <= 0 {
		return a
	}
	if mix >= 1 {
		return b
	}
	return gfx.Color{
		R: a.R*(1-mix) + b.R*mix,
		G: a.G*(1-mix) + b.G*mix,
		B: a.B*(1-mix) + b.B*mix,
		A: a.A*(1-mix) + b.A*mix,
	}
}
