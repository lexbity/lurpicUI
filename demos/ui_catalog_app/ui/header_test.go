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

func TestHeaderFacet_ControlsMutateStores(t *testing.T) {
	th := theme.Default()
	shaper := newTestShaper(t)
	meta := model.DefaultBuildMetadata()
	header := NewHeaderFacet(th, shaper, meta)
	header.layout.Arrange(gfx.RectFromXYWH(0, 0, 1280, 48))

	t.Cleanup(func() {
		store.SetTheme(store.ThemeSystem)
		store.SetDensity(store.DensityNormal)
		store.SetCompareMode(store.CompareOff)
		store.SetCompareTheme(store.ThemeDark)
	})

	click := func(kind string) {
		for _, control := range header.layoutControls(header.layout.ArrangedBounds) {
			if control.kind != kind {
				continue
			}
			center := gfx.Point{
				X: control.rect.Min.X + control.rect.Width()/2,
				Y: control.rect.Min.Y + control.rect.Height()/2,
			}
			if !header.hit.HitTest(center).Hit {
				t.Fatalf("expected hit for control %q", kind)
			}
			if !header.input.OnPointer(pointerRelease(center)) {
				t.Fatalf("expected pointer handler to accept control %q", kind)
			}
			return
		}
		t.Fatalf("control %q not found", kind)
	}

	t.Run("theme", func(t *testing.T) {
		store.SetTheme(store.ThemeSystem)
		store.SetDensity(store.DensityNormal)
		click("theme")
		if got := store.GetTheme(); got != store.ThemeLight {
			t.Fatalf("theme = %v, want %v", got, store.ThemeLight)
		}
	})

	t.Run("density", func(t *testing.T) {
		store.SetTheme(store.ThemeSystem)
		store.SetDensity(store.DensityNormal)
		click("density")
		if got := store.GetDensity(); got != store.DensityComfortable {
			t.Fatalf("density = %v, want %v", got, store.DensityComfortable)
		}
	})
}

func pointerRelease(center gfx.Point) facet.PointerEvent {
	return facet.PointerEvent{
		Kind:     platform.PointerRelease,
		Position: center,
		Button:   platform.PointerLeft,
	}
}
