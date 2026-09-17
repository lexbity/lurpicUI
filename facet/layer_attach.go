package facet

import "codeburg.org/lexbit/lurpicui/store"

// ZBand names the semantic z-tier a layer-attached facet occupies (RX-1 Q4).
// Bands replace raw z-priority numbers: the framework owns the tier semantics,
// and the Order field breaks ties within a band. Monotonic paint order:
// Base < Content < Popover < Modal < Tooltip < Toast.
type ZBand uint8

const (
	ZBandBase ZBand = iota
	ZBandContent
	ZBandPopover
	ZBandModal
	ZBandTooltip
	ZBandToast

	zBandMax
)

// Valid reports whether the band is a declared constant.
func (b ZBand) Valid() bool {
	return b < zBandMax
}

// LayerRecipeRef points at a named layer layout recipe (centered modal,
// anchored popover, free, grid). When set, it overrides the recipe the layer
// registry descriptor provides.
type LayerRecipeRef struct {
	Family string
	Name   string
}

// LayerAttachment describes the layer contract for a child mounted via
// AttachLayer.
type LayerAttachment struct {
	// Band is the semantic z-tier. Required; AttachLayer panics if invalid.
	Band ZBand
	// Order breaks ties within a band.
	Order int32
	// Recipe overrides the layer's layout recipe when both names are set.
	Recipe LayerRecipeRef
	// Mount gates the layer's visibility: when non-nil and its value is false
	// the layer is skipped entirely (no measure, arrange, projection, or hit).
	// Nil means always mounted.
	Mount *store.ValueStore[bool]
	// Dismissal and HitPolicy are unchanged from the V2 overlay contract.
	Dismissal DismissalScope
	HitPolicy HitPolicy
}

// AttachLayer registers child as a layered child of parent, recording the layer
// contract for the runtime to consume. It panics with a contract message if
// att.Band is not a declared ZBand — callers wanting default placement use
// AddChild.
//
// The child remains a tree child of parent (for coverage, focus, and disposal)
// but is NOT part of the parent's group-measure/arrange set: the layer system
// exclusively measures, arranges, and gates it (RX-1 Q4 exclusivity).
//
// AttachLayer is safe to call during construction (the child lifecycle is
// managed by the runtime once the tree is attached).
func AttachLayer(parent, child FacetImpl, att LayerAttachment) {
	if !att.Band.Valid() {
		panic("facet: AttachLayer requires a valid ZBand; use AddChild for default placement")
	}
	parent.Base().AddChild(child.Base())
	child.Base().layer = att
	child.Base().layerSet = true
}
