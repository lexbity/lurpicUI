package facet

// Role is implemented by opt-in facet capabilities.
//
// The base package defines the lifecycle contract now; concrete role structs
// arrive in the next phase.
type Role interface {
	onAttach(f *Facet)
	onActivate(f *Facet)
	onDeactivate(f *Facet)
	onDispose(f *Facet)
}
