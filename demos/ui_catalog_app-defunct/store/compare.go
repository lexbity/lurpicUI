package store

import (
	"codeburg.org/lexbit/lurpicui/store"
)

// CompareMode represents the comparison display mode.
type CompareMode uint8

const (
	CompareOff CompareMode = iota // Single view
	CompareSideBySide             // Two columns (current vs other theme)
	CompareStacked                // Two rows (current vs other theme)
)

func (c CompareMode) String() string {
	switch c {
	case CompareOff:
		return "Single"
	case CompareSideBySide:
		return "Side by Side"
	case CompareStacked:
		return "Stacked"
	default:
		return "Unknown"
	}
}

// AllCompareModes returns all available compare modes.
func AllCompareModes() []CompareMode {
	return []CompareMode{CompareOff, CompareSideBySide, CompareStacked}
}

// CompareStore holds the current compare mode.
var CompareStore = store.NewValueStore[CompareMode](CompareOff)

// SetCompareMode sets the compare mode.
func SetCompareMode(mode CompareMode) {
	CompareStore.Set(mode)
}

// GetCompareMode returns the current compare mode.
func GetCompareMode() CompareMode {
	return CompareStore.Get()
}

// IsCompareMode returns true if we're in any compare mode.
func IsCompareMode() bool {
	return CompareStore.Get() != CompareOff
}

// CompareThemeStore holds the theme to compare against (for side-by-side).
// When in compare mode, we show: current theme | compare theme
var CompareThemeStore = store.NewValueStore[ThemeMode](ThemeDark)

// SetCompareTheme sets the theme to compare against.
func SetCompareTheme(mode ThemeMode) {
	CompareThemeStore.Set(mode)
}

// GetCompareTheme returns the theme being compared.
func GetCompareTheme() ThemeMode {
	return CompareThemeStore.Get()
}
