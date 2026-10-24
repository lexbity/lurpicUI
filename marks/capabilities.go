package marks

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

// Focusable is implemented by marks that can receive keyboard focus.
type Focusable interface {
	Focusable() bool
}

// AnchorExporting is an alias for layout.AnchorExporter.
type AnchorExporting = layout.AnchorExporter

// Accessible is implemented by marks that expose accessibility metadata.
type Accessible interface {
	AccessibilityRole() string
	AccessibleName() string
}

// Customizable is implemented by marks that support theme variant/recipe
// overrides beyond the standard token set.
type Customizable any

// theme variant/recipe surface (reserved for future use)

// KeyedItem is implemented by collection-child marks (list items, tree rows,
// table cells) so LDP locators can address them by stable key — the item's
// ItemID, node path, or row:column key. Implemented once per mark; the key
// returns "" when the host assigned none.
type KeyedItem interface{ ItemKey() string }

// HostsChildren is implemented by group marks whose children are hosted
// virtually (projected by the host rather than attached as facet-tree
// children — tree rows, table cells, dropdown entries). LDP locators walk
// hosted children so keyed addressing reaches every visible row.
type HostsChildren interface {
	HostedChildren() []facet.FacetImpl
}

// DataBound is implemented by marks that bind a store.CollectionStore
// together with associated scales.
type DataBound interface {
	// BoundData returns the mark's bound data source.
	// Implementations return the *store.CollectionStore or similar.
	BoundData() any
}

// Channel describes a single encoding channel (x, y, color, size, shape, etc.)
// that a data-bound mark maps from domain to visual range.
type Channel struct {
	Name string
}

// Encoding is implemented by marks that expose their visual encoding channels.
type Encoding interface {
	Channels() []Channel
}
