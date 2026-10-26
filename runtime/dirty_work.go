package runtime

import "codeburg.org/lexbit/lurpicui/facet"

// recordInvalidation merges one source into the generation that can still be
// consumed safely. Once the frame has captured its current cut, later work is
// retained for the following frame instead of being erased at frame end.
func (rt *Runtime) recordInvalidation(id facet.FacetID, flags facet.DirtyFlags, source string) {
	if rt == nil || id == 0 || flags == 0 {
		return
	}
	if flags&facet.DirtyLayout != 0 {
		rt.layerTreeDirty = true
	}
	facets, sources := rt.dirtyFacets, rt.dirtySources
	if rt.dirtyCaptured {
		facets, sources = rt.nextDirtyFacets, rt.nextDirtySources
	}
	facets[id] |= flags
	if source != "" {
		sources[id] = source
	}
}

func (rt *Runtime) beginNextDirtyGeneration() {
	rt.dirtyCaptured = true
	clear(rt.nextDirtyFacets)
	clear(rt.nextDirtySources)
}

func (rt *Runtime) sealDirtyGeneration() {
	clear(rt.dirtyFacets)
	clear(rt.dirtySources)
	rt.dirtyFacets, rt.nextDirtyFacets = rt.nextDirtyFacets, rt.dirtyFacets
	rt.dirtySources, rt.nextDirtySources = rt.nextDirtySources, rt.dirtySources
	rt.dirtyCaptured = false
}
