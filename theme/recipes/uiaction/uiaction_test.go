package uiaction

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestResolveMenuButtonRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveMenuButtonRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("menu button variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Trigger", "TriggerLabel", "TriggerIcon", "Chevron", "FloatingMenuSurface", "MenuItems", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected menu button slot source for %s", name)
		}
	}
	if !allMenuButtonFieldsPresent(slots) {
		t.Fatalf("menu button slots contain zero values: %#v", slots)
	}
}

func TestResolveActionGroupRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveActionGroupRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("action group variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "GroupSurface", "ActionItems", "Separators", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected action group slot source for %s", name)
		}
	}
	if !allActionGroupFieldsPresent(slots) {
		t.Fatalf("action group slots contain zero values: %#v", slots)
	}
}

func TestResolveSplitButtonRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveSplitButtonRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("split button variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "PrimaryButton", "PrimaryLabel", "MenuTrigger", "Chevron", "FloatingMenuSurface", "MenuItems", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected split button slot source for %s", name)
		}
	}
	if !allSplitButtonFieldsPresent(slots) {
		t.Fatalf("split button slots contain zero values: %#v", slots)
	}
}

func allMenuButtonFieldsPresent[T any](value T) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			return false
		}
	}
	return true
}

func allSplitButtonFieldsPresent[T any](value T) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			return false
		}
	}
	return true
}

func allActionGroupFieldsPresent[T any](value T) bool {
	rv := reflect.ValueOf(value)
	if rv.Kind() == reflect.Pointer {
		rv = rv.Elem()
	}
	if rv.Kind() != reflect.Struct {
		return false
	}
	for i := 0; i < rv.NumField(); i++ {
		if rv.Field(i).IsZero() {
			return false
		}
	}
	return true
}
