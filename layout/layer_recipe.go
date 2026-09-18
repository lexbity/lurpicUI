package layout

// StandardLayerRecipeName identifies a built-in layer layout recipe (RX-1 Q4).
// A facet.LayerRecipeRef with an empty Family resolves by Name against this
// set; a Family-qualified ref resolves through the theme resolver instead.
// Framework recipes live here so every overlay (menus, palettes, toasts,
// tooltips) declares against a resolvable name instead of a hardcoded runtime
// special-case.
type StandardLayerRecipeName string

const (
	// StandardLayerRecipeModal arranges children into a single cell that fills
	// the parent so each child centers itself within it (the command palette's
	// centered surface is the canonical consumer).
	StandardLayerRecipeModal StandardLayerRecipeName = "modal"
	// StandardLayerRecipeGrid is the 5x5 grid fallback recipe.
	StandardLayerRecipeGrid StandardLayerRecipeName = "grid"
	// StandardLayerRecipeFree uses free placement (children land at their
	// free offsets).
	StandardLayerRecipeFree StandardLayerRecipeName = "free"
	// StandardLayerRecipeAnchor uses anchor placement (children track
	// exported anchors).
	StandardLayerRecipeAnchor StandardLayerRecipeName = "anchor"
)

// ResolveStandardLayerRecipe returns the built-in recipe for name, or false
// when name is not a declared standard recipe.
func ResolveStandardLayerRecipe(name string) (ResolvedLayerLayoutRecipe, bool) {
	switch StandardLayerRecipeName(name) {
	case StandardLayerRecipeModal:
		recipe := DefaultLayerLayoutRecipe()
		recipe.Grid = ResolvedGridConfig{Columns: 1, Rows: 1}
		return recipe, true
	case StandardLayerRecipeGrid:
		return DefaultLayerLayoutRecipe(), true
	case StandardLayerRecipeFree:
		return ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutFree}, true
	case StandardLayerRecipeAnchor:
		return ResolvedLayerLayoutRecipe{PolicyKind: LayerLayoutAnchor}, true
	default:
		return ResolvedLayerLayoutRecipe{}, false
	}
}
