package studio

import (
	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/state"
	"codeburg.org/lexbit/lurpicui/facet"
)

// ExhibitID identifies a gallery exhibit.
type ExhibitID string

// The gallery's exhibit catalog. Later slices fill the set (E1, E2, E3, E5,
// E6, Capability Index); Slice P4 ships E4 plus the stage that hosts them.
const (
	ExhibitCapabilities ExhibitID = "capabilities"
	ExhibitRealtime     ExhibitID = "realtime"
	ExhibitLayers       ExhibitID = "layers"
	ExhibitAnchors      ExhibitID = "anchors"
	ExhibitPolicies     ExhibitID = "policies"
	ExhibitPropagation  ExhibitID = "propagation"
	ExhibitPlayground   ExhibitID = "playground"
)

// Exhibit is one self-contained gallery exhibit: a facet factory that builds
// the exhibit's root facet from the shared app state.
//
// F-exhibits-pkg: the exhibits live in the studio package rather than a
// studio/exhibits subpackage because E4 reuses the studio split host
// (GallerySplit); a subpackage importing studio would create an import cycle
// with the shell's stage wiring (studio/root.go imports the stage which
// imports the exhibits).
type Exhibit interface {
	ID() ExhibitID
	Title() string
	Build(state *state.AppState) facet.FacetImpl
}
