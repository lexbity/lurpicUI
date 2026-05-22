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

func TestResolveToolbarRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveToolbarRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("toolbar variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "ToolbarSurface", "ActionItems", "Groups", "Separators", "OverflowMenu", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected toolbar slot source for %s", name)
		}
	}
	if !allToolbarFieldsPresent(slots) {
		t.Fatalf("toolbar slots contain zero values: %#v", slots)
	}
}

func TestResolveRibbonRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveRibbonRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("ribbon variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "RibbonSurface", "Groups", "GroupLabels", "ActionItems", "OverflowControls", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected ribbon slot source for %s", name)
		}
	}
	if !allRibbonFieldsPresent(slots) {
		t.Fatalf("ribbon slots contain zero values: %#v", slots)
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

func TestResolveCommandPaletteRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveCommandPaletteRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("command palette variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Backdrop", "ModalSurface", "SearchField", "ResultsList", "ResultItem", "ShortcutLabel", "EmptyState", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected command palette slot source for %s", name)
		}
	}
	if !allCommandPaletteFieldsPresent(slots) {
		t.Fatalf("command palette slots contain zero values: %#v", slots)
	}
}

func TestResolvePopupPaletteRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolvePopupPaletteRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("popup palette variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "PaletteSurface", "ToolItems", "ToolGroup", "AnchorArrow", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected popup palette slot source for %s", name)
		}
	}
	if !allPopupPaletteFieldsPresent(slots) {
		t.Fatalf("popup palette slots contain zero values: %#v", slots)
	}
}

func TestResolveRadialMenuRecipe_exposes_expected_slots(t *testing.T) {
	slots, report := ResolveRadialMenuRecipe(theme.StyleContext{Tokens: theme.DefaultTokens()})
	if report.Variant != theme.VariantKey("default") {
		t.Fatalf("radial menu variant = %q, want default", report.Variant)
	}
	for _, name := range []string{"Root", "Surface", "CenterSlot", "RadialTrack", "AnchorArrow", "FocusRing"} {
		if _, ok := report.SlotSource(name); !ok {
			t.Fatalf("expected radial menu slot source for %s", name)
		}
	}
	if !allRadialMenuFieldsPresent(slots) {
		t.Fatalf("radial menu slots contain zero values: %#v", slots)
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

func allToolbarFieldsPresent[T any](value T) bool {
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

func allRibbonFieldsPresent[T any](value T) bool {
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

func allCommandPaletteFieldsPresent[T any](value T) bool {
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

func allPopupPaletteFieldsPresent[T any](value T) bool {
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

func allRadialMenuFieldsPresent[T any](value T) bool {
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
