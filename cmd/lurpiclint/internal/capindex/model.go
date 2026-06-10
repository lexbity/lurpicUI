// Package capindex implements the uxauthoring-index introspection engine.
// It scans the framework's own packages (marks/, layout/, ...) and builds a
// structured capability model that drives both the LL004 shape-match rule
// and the `lurpiclint capabilities` subcommand.
package capindex

import (
	"fmt"
)

// CapabilityKind classifies a framework capability.
type CapabilityKind int

const (
	KindMark   CapabilityKind = iota // UI widget / mark (e.g. NewCard)
	KindLayout                       // Layout container (e.g. RowLayout)
	KindLayer                        // Standard layer constant (e.g. StandardLayer_Base)
)

func (k CapabilityKind) String() string {
	switch k {
	case KindMark:
		return "mark"
	case KindLayout:
		return "layout"
	case KindLayer:
		return "layer"
	default:
		return fmt.Sprintf("kind(%d)", int(k))
	}
}

// Capability describes a single discoverable framework capability.
type Capability struct {
	Kind        CapabilityKind
	Path        string      // uxauthoring index path (e.g. "marks/structure.Card")
	TypeName    string      // exported type name (e.g. "Card")
	Category    string      // sub-package category (e.g. "structure")
	Constructor string      // constructor function name (e.g. "NewCard"), empty if none
	Intent      string      // first sentence of the type's doc comment
	Fingerprint Fingerprint // structural fingerprint for shape-matching
}

// Fingerprint captures the structural shape of a capability for use in
// shape-match suggestions (LL004).
type Fingerprint struct {
	EmbedsFacet   bool     // struct embeds facet.Facet
	Roles         []string // roles registered via AddRole ("layout", "render", ...)
	HasChildSlice bool     // struct holds a slice of child facets
	IsContainer   bool     // fingerprint classifies as a container (hosts children)
}
