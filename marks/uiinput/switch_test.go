package uiinput

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/store"
	uirecipe "codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
	"codeburg.org/lexbit/lurpicui/theme"
)

func TestSwitch_click_toggles_store(t *testing.T) {
	on := store.NewBinding(false)
	s := &Switch{On: on}
	s.ensureInit()
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerPress}) {
		t.Fatal("expected press to be handled")
	}
	if !s.handlePointer(facet.PointerEvent{Kind: platform.PointerRelease}) {
		t.Fatal("expected release to be handled")
	}
	if got := s.On.Get(); !got {
		t.Fatalf("on = %v, want true", got)
	}
}

func TestSwitch_recipe_slots_track_and_thumb_present(t *testing.T) {
	slots, report := uirecipe.ResolveSwitchRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()}, uirecipe.SwitchStandard)
	if len(slots.Track.Base.Fills) == 0 || len(slots.Thumb.Base.Fills) == 0 {
		t.Fatalf("expected track and thumb styles to be present: %#v", slots)
	}
	if report.Family != "uiinput" {
		t.Fatalf("family = %q, want uiinput", report.Family)
	}
}
