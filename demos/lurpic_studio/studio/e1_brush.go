package studio

import "codeburg.org/lexbit/lurpicui/store"

// regionHoverSentinel marks a Hover that refers to a whole bar band rather
// than a single row; HoverRegion carries the region name.
const regionHoverSentinel = store.ItemID(1) << 62

// BrushStores are the shared linked-brushing channels between the chart and
// the spreadsheet (F-hover: cross-facet transient state lives in stores).
type BrushStores struct {
	// Hover is the hovered row id (nil = none). A region-group hover sets it
	// to regionHoverSentinel and HoverRegion to the region name.
	Hover *store.ValueStore[*store.ItemID]
	// HoverRegion is the hovered bar band's region (empty when not a band).
	HoverRegion *store.ValueStore[string]
	// Selection is the selected row id (chart point or spreadsheet row click).
	Selection *store.ValueStore[store.ItemID]
}

// NewBrushStores builds a fresh linked-brushing channel set.
func NewBrushStores() BrushStores {
	return BrushStores{
		Hover:       store.NewValueStore[*store.ItemID](nil),
		HoverRegion: store.NewValueStore(""),
		Selection:   store.NewValueStore[store.ItemID](0),
	}
}
