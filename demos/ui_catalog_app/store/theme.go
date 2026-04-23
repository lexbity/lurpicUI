package store

import (
	"codeburg.org/lexbit/lurpicui/store"
)

// ThemeMode represents the current theme family.
type ThemeMode uint8

const (
	ThemeLight ThemeMode = iota
	ThemeDark
	ThemeSystem // Uses system preference
)

func (t ThemeMode) String() string {
	switch t {
	case ThemeLight:
		return "Light"
	case ThemeDark:
		return "Dark"
	case ThemeSystem:
		return "System"
	default:
		return "Unknown"
	}
}

// AllThemeModes returns all available theme modes.
func AllThemeModes() []ThemeMode {
	return []ThemeMode{ThemeLight, ThemeDark, ThemeSystem}
}

// ThemeStore holds the currently selected theme mode.
var ThemeStore = store.NewValueStore[ThemeMode](ThemeSystem)

// SetTheme sets the theme mode.
func SetTheme(mode ThemeMode) {
	ThemeStore.Set(mode)
}

// GetTheme returns the current theme mode.
func GetTheme() ThemeMode {
	return ThemeStore.Get()
}

// IsDarkMode returns true if the current theme should use dark colors.
// For ThemeSystem, this would check system preference (stub for now).
func IsDarkMode() bool {
	mode := ThemeStore.Get()
	switch mode {
	case ThemeDark:
		return true
	case ThemeLight:
		return false
	case ThemeSystem:
		// TODO: Check system preference
		return false
	default:
		return false
	}
}
