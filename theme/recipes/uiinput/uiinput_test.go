package uiinput

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
)

func TestButtonRecipe_default_variant_returns_all_slots(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := ResolveButtonRecipe(ctx, ButtonFilled)
	if !allFieldsPresent(slots) {
		t.Fatalf("button slots contain zero values: %#v", slots)
	}
	if report.Family != "uiinput" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("filled") {
		t.Fatalf("variant = %q", report.Variant)
	}
	if len(report.SlotNames()) != 4 {
		t.Fatalf("slot names = %v", report.SlotNames())
	}
}

func TestButtonRecipe_subtree_override_changes_container_only(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	overrideTokens := ctx.Tokens
	overrideTokens.Color.Surface = gfx.Color{R: 0.95, G: 0.85, B: 0.75, A: 1}
	subtree := ctx.Derive(theme.StyleContextOverride{Colors: &overrideTokens.Color})

	base, _ := ResolveButtonRecipe(ctx, ButtonOutlined)
	got, report := ResolveButtonRecipe(subtree, ButtonOutlined)

	if reflect.DeepEqual(base.Container, got.Container) {
		t.Fatal("expected container to change under subtree override")
	}
	if !reflect.DeepEqual(base.Label, got.Label) {
		t.Fatal("label should not change")
	}
	if !reflect.DeepEqual(base.Icon, got.Icon) {
		t.Fatal("icon should not change")
	}
	if source, ok := report.SlotSource("Container"); !ok || source != theme.SlotSourceInstanceOverride && source != theme.SlotSourceVariantDefault {
		t.Fatalf("unexpected container source: %v %v", source, ok)
	}
}

func TestTextInputRecipe_all_slots_present(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := ResolveTextInputRecipe(ctx, TextInputOutlined)
	if !allFieldsPresent(slots) {
		t.Fatalf("text input slots contain zero values: %#v", slots)
	}
	if report.Family != "uiinput" {
		t.Fatalf("family = %q", report.Family)
	}
}

func TestSliderRecipe_variant_changes_thumb_and_track(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	standard, _ := ResolveSliderRecipe(ctx, SliderStandard)
	compact, _ := ResolveSliderRecipe(ctx, SliderCompact)
	if reflect.DeepEqual(standard.Thumb, compact.Thumb) {
		t.Fatal("thumb should differ across variants")
	}
	if reflect.DeepEqual(standard.Track, compact.Track) {
		t.Fatal("track should differ across variants")
	}
}

func allFieldsPresent[T any](value T) bool {
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

var _ shared.ButtonSlots
