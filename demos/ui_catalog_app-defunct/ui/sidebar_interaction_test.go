package ui

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

func TestSidebarFacet_TogglesFamilyFilterOnClick(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)

	resetCatalogStores(t)

	sidebar := NewSidebarFacet(th, shaper)
	bounds := gfx.RectFromXYWH(0, 0, 280, 800)
	sidebar.layout.Arrange(bounds)

	var list gfx.CommandList
	sidebar.renderSidebar(&list, bounds)

	rect, ok := sidebar.itemRects["family:"+model.FamilyBasic.String()]
	if !ok {
		t.Fatal("expected family item rect to be registered")
	}

	center := gfx.Point{
		X: rect.Min.X + rect.Width()/2,
		Y: rect.Min.Y + rect.Height()/2,
	}
	if !sidebar.input.OnPointer(facet.PointerEvent{
		Kind:     platform.PointerPress,
		Position: center,
		Button:   platform.PointerLeft,
	}) {
		t.Fatal("expected family press to be handled")
	}
	if !sidebar.input.OnPointer(facet.PointerEvent{
		Kind:     platform.PointerRelease,
		Position: center,
		Button:   platform.PointerLeft,
	}) {
		t.Fatal("expected family click to be handled")
	}

	filter := store.FilterStore.Get()
	if len(filter.SelectedFamilies) != 1 || filter.SelectedFamilies[0] != model.FamilyBasic {
		t.Fatalf("selected families = %#v, want only basic", filter.SelectedFamilies)
	}
}
