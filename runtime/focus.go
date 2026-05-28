package runtime

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
)

func (rt *Runtime) syncFocusTraps() {
	if rt.focusManager == nil {
		return
	}
	if rt.layerRegistry == nil || len(rt.projectionLayers) == 0 {
		rt.focusManager.SyncFocusTraps(nil)
		return
	}
	type trapEntry struct {
		order   int32
		layerID facet.FacetID
		restore facet.FocusRestoreMode
	}
	traps := make([]trapEntry, 0, len(rt.projectionLayers))
	for facetID, layer := range rt.projectionLayers {
		if layer.LayerID == 0 {
			continue
		}
		desc, ok := rt.layerRegistry.Lookup(layout.LayerID(layer.LayerID))
		if !ok || !desc.FocusTrap {
			continue
		}
		traps = append(traps, trapEntry{
			order:   int32(desc.Order),
			layerID: facetID,
			restore: desc.FocusRestore,
		})
	}
	sort.SliceStable(traps, func(i, j int) bool {
		if traps[i].order != traps[j].order {
			return traps[i].order < traps[j].order
		}
		return traps[i].layerID < traps[j].layerID
	})
	stack := make([]facet.FocusTrapState, len(traps))
	for i, trap := range traps {
		stack[i] = facet.FocusTrapState{
			Scope:   trap.layerID,
			Restore: trap.restore,
		}
	}
	rt.focusManager.SyncFocusTraps(stack)
}
