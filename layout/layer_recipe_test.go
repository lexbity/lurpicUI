package layout

import "testing"

// TestResolveStandardLayerRecipeModal pins the RX-1 Q4 standard recipe set's
// modal entry: a single 1x1 cell that fills the parent so the child centers
// itself within it (the command palette's centered surface). It replaces the
// old hardcoded "modal" special-case in the runtime layer pass.
func TestResolveStandardLayerRecipeModal(t *testing.T) {
	recipe, ok := ResolveStandardLayerRecipe("modal")
	if !ok {
		t.Fatal("modal standard recipe did not resolve")
	}
	if recipe.PolicyKind != LayerLayoutGrid {
		t.Fatalf("modal PolicyKind = %v, want LayerLayoutGrid", recipe.PolicyKind)
	}
	if recipe.Grid.Columns != 1 || recipe.Grid.Rows != 1 {
		t.Fatalf("modal grid = %dx%d, want 1x1", recipe.Grid.Columns, recipe.Grid.Rows)
	}
}

// TestResolveStandardLayerRecipeKinds pins the remaining standard recipes:
// grid is the 5x5 fallback, free and anchor select their policies.
func TestResolveStandardLayerRecipeKinds(t *testing.T) {
	cases := []struct {
		name string
		kind LayerLayoutKind
	}{
		{"grid", LayerLayoutGrid},
		{"free", LayerLayoutFree},
		{"anchor", LayerLayoutAnchor},
	}
	for _, tc := range cases {
		recipe, ok := ResolveStandardLayerRecipe(tc.name)
		if !ok {
			t.Fatalf("standard recipe %q did not resolve", tc.name)
		}
		if recipe.PolicyKind != tc.kind {
			t.Fatalf("recipe %q PolicyKind = %v, want %v", tc.name, recipe.PolicyKind, tc.kind)
		}
	}
}

// TestResolveStandardLayerRecipeUnknown pins the fail-fast contract (§7.2): an
// unknown standard recipe name does not resolve.
func TestResolveStandardLayerRecipeUnknown(t *testing.T) {
	if _, ok := ResolveStandardLayerRecipe("bogus"); ok {
		t.Fatal("unknown standard recipe resolved")
	}
}
