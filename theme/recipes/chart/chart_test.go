package chart

import (
	"reflect"
	"testing"

	"codeburg.org/lexbit/lurpicui/theme"
)

func TestAxisRecipe_all_slots_present(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, report := ResolveAxisRecipe(ctx, AxisStandard)
	if !allFieldsPresent(slots) {
		t.Fatalf("axis slots contain zero values: %#v", slots)
	}
	if report.Family != "chart" {
		t.Fatalf("family = %q", report.Family)
	}
	if report.Variant != theme.VariantKey("standard") {
		t.Fatalf("variant = %q", report.Variant)
	}
}

func TestAxisRecipe_gridline_optional_behavior(t *testing.T) {
	ctx := theme.StyleContext{Tokens: theme.DefaultTokens()}
	slots, _ := ResolveAxisRecipe(ctx, AxisCompact)
	if reflect.ValueOf(slots.GridLine.Base).IsZero() {
		t.Fatal("gridline slot must still be present")
	}
	if len(slots.GridLine.Base.Fills) == 0 {
		t.Fatal("gridline should carry a visible material even if opacity is reduced")
	}
	if slots.GridLine.Base.Fills[0].Opacity != 0 {
		t.Fatalf("expected compact gridline opacity 0, got %v", slots.GridLine.Base.Fills[0].Opacity)
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
