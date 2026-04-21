package facet

// Tree exposes the minimum lookup surface needed by tree-aware helpers.
type Tree interface {
	FacetByID(id FacetID) FacetImpl
	RootStyleContext() any
}
