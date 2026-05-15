package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

func TestCatalogFacetTextRendering_EmitsGlyphRuns(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)
	meta := model.DefaultBuildMetadata()
	profile := DefaultLayoutProfile()

	resetCatalogStores(t)

	bounds := gfx.RectFromXYWH(0, 0, 1280, 800)

	t.Run("header", func(t *testing.T) {
		header := NewHeaderFacet(th, shaper, meta)
		list := &gfx.CommandList{}
		header.render.OnCollect(list, gfx.RectFromXYWH(0, 0, 1280, profile.HeaderHeight))
		assertHasAnyGlyphRun(t, list)
	})

	t.Run("sidebar", func(t *testing.T) {
		sidebar := NewSidebarFacet(th, shaper)
		list := &gfx.CommandList{}
		sidebar.render.OnCollect(list, gfx.RectFromXYWH(0, 0, profile.SidebarWidthDefault, 600))
		assertHasAnyGlyphRun(t, list)
	})

	t.Run("footer", func(t *testing.T) {
		footer := NewFooterFacet(th, shaper)
		list := &gfx.CommandList{}
		footer.render.OnCollect(list, gfx.RectFromXYWH(0, 0, 1280, profile.FooterHeight))
		assertHasAnyGlyphRun(t, list)
	})

	t.Run("inspector empty", func(t *testing.T) {
		store.ClearSelection()
		inspector := NewInspectorFacet(th, shaper)
		list := &gfx.CommandList{}
		inspector.render.OnCollect(list, gfx.RectFromXYWH(0, 0, profile.InspectorWidthDefault, 600))
		assertHasAnyGlyphRun(t, list)
	})

	t.Run("content grid", func(t *testing.T) {
		store.ClearSelection()
		store.SetCompareMode(store.CompareOff)
		content := NewContentFacet(th, shaper)
		content.layout.Arrange(bounds)
		content.syncCards()

		list := &gfx.CommandList{}
		content.render.OnCollect(list, bounds)
		assertHasAnyGlyphRun(t, list)
	})
}

func TestCatalogFacetTextRendering_DetailAndCompare(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	entry := store.CatalogInstance.AllEntries()[0]
	store.SelectEntry(entry.ID)

	content := NewContentFacet(th, shaper)
	content.layout.Arrange(gfx.RectFromXYWH(0, 0, 1280, 800))
	content.syncCards()

	t.Run("detail", func(t *testing.T) {
		content.SetViewMode(ViewDetail)
		store.SetCompareMode(store.CompareOff)
		list := &gfx.CommandList{}
		content.render.OnCollect(list, gfx.RectFromXYWH(0, 0, 1280, 800))
		assertHasAnyGlyphRun(t, list)
	})

	t.Run("compare", func(t *testing.T) {
		content.SetViewMode(ViewDetail)
		store.SetCompareMode(store.CompareSideBySide)
		store.SetCompareTheme(store.ThemeDark)
		list := &gfx.CommandList{}
		content.render.OnCollect(list, gfx.RectFromXYWH(0, 0, 1280, 800))
		assertHasAnyGlyphRun(t, list)
	})
}

func resetCatalogStores(t *testing.T) {
	t.Helper()

	t.Cleanup(func() {
		store.ResetFilters()
		store.ClearSelection()
		store.SetTheme(store.ThemeSystem)
		store.SetDensity(store.DensityNormal)
		store.SetCompareMode(store.CompareOff)
		store.SetCompareTheme(store.ThemeDark)
	})

	store.ResetFilters()
	store.ClearSelection()
	store.SetTheme(store.ThemeSystem)
	store.SetDensity(store.DensityNormal)
	store.SetCompareMode(store.CompareOff)
	store.SetCompareTheme(store.ThemeDark)
}

func assertHasAnyGlyphRun(t *testing.T, list *gfx.CommandList) {
	t.Helper()

	count := 0
	for _, cmd := range list.Commands {
		if _, ok := cmd.(gfx.DrawGlyphRun); ok {
			count++
		}
	}
	if count == 0 {
		t.Fatal("expected at least one DrawGlyphRun command")
	}
}
