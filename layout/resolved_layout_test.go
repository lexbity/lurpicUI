package layout

import "testing"

func TestDefaultLayerLayoutRecipe_usesFiveByFiveGrid(t *testing.T) {
	recipe := DefaultLayerLayoutRecipe()
	if recipe.PolicyKind != LayerLayoutGrid {
		t.Fatalf("policy kind = %v", recipe.PolicyKind)
	}
	if recipe.Grid.Columns != 5 || recipe.Grid.Rows != 5 {
		t.Fatalf("grid = %#v", recipe.Grid)
	}
	if recipe.Grid.ColumnGap != 0 || recipe.Grid.RowGap != 0 {
		t.Fatalf("expected zero gaps, got %#v", recipe.Grid)
	}
}

func TestResolvedOptionalScalar_behavesAsExpected(t *testing.T) {
	if got, ok := (ResolvedOptionalScalar{}).Float32(); ok || got != 0 {
		t.Fatalf("zero optional scalar should be invalid, got %v %v", got, ok)
	}
	value := OptionalScalar(12.5)
	if got, ok := value.Float32(); !ok || got != 12.5 {
		t.Fatalf("optional scalar = %v %v", got, ok)
	}
	if got := value.OrZero(); got != 12.5 {
		t.Fatalf("optional scalar OrZero = %v", got)
	}
}

func TestGroupLayoutRecipeRef_String(t *testing.T) {
	ref := GroupLayoutRecipeRef{Family: "app", Name: "panel"}
	if got := ref.String(); got != "app/panel" {
		t.Fatalf("string = %q", got)
	}
	if !(GroupLayoutRecipeRef{}).IsZero() {
		t.Fatal("expected zero ref to report zero")
	}
}
