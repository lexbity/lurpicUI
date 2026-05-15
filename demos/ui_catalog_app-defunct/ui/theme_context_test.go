package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/store"
)

func TestCatalogThemeContext_TracksThemeStore(t *testing.T) {
	ctx := NewCatalogThemeContext()

	t.Cleanup(func() {
		store.SetTheme(store.ThemeSystem)
	})

	store.SetTheme(store.ThemeLight)
	lightBg := ctx.Color(theme.ColorBackground)

	store.SetTheme(store.ThemeDark)
	darkBg := ctx.Color(theme.ColorBackground)

	if lightBg == darkBg {
		t.Fatalf("background color did not change between light and dark themes: %+v", lightBg)
	}
}
