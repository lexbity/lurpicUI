package marks

import "codeburg.org/lexbit/lurpicui/layout"

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
type Customizable interface {
	// theme variant/recipe surface (reserved for future use)
}

// Composite is implemented by marks that contain child marks.
type Composite interface {
	ChildMarks() []Mark
}

// DataBound is implemented by marks that bind a store.CollectionStore
// together with associated scales.
type DataBound interface {
	// binds a *store.CollectionStore + scales (reserved for data-viz)
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
