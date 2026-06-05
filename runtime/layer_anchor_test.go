package runtime

import (
	"testing"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

type runtimeAnchorFacet struct {
	facet.Facet
	layout facet.LayoutRole
}

func (f *runtimeAnchorFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

func TestRuntime_anchorExportsMarkDependentsDirtyOnMovement(t *testing.T) {
	root := &runtimeTestFacet{Facet: facet.NewFacet(), name: "root"}
	exporter := &runtimeLayerFacet{
		Facet: facet.NewFacet(),
		anchors: layout.AnchorSet{
			"mark": {X: 10, Y: 20},
		},
	}
	dependent := &runtimeAnchorFacet{Facet: facet.NewFacet()}
	dependent.layout.OnMeasure = func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: 20, H: 10}}
	}
	dependent.layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		dependent.layout.ArrangedBounds = bounds
	}
	dependent.layout.Child.SupportedPlacement = facet.SupportsAnchor

	rt := mustRuntimeTree(t, root)
	reg := testLayerRegistry(t)
	rt.config.LayerRegistry = reg
	rt.layerRegistry = reg
	rt.anchorCaches[root.Base().ID()] = layout.NewAnchorPositionCache()
	rt.AddFacet(root, exporter, facet.Attachment{LayerID: facet.LayerID(1)})
	rt.AddFacet(root, dependent, facet.Attachment{
		LayerID: facet.LayerID(2),
		Placement: facet.Placement{
			Mode: facet.PlacementAnchor,
			Anchor: facet.AnchorPlacement{
				AnchorRef: facet.AnchorID("mark"),
				Side:      facet.AnchorRight,
			},
		},
	})
	dependentID := dependent.Base().ID()

	rt.resolveAnchorExports()
	if snap, ok := rt.AnchorSnapshot(root.Base().ID()); !ok || snap.Version == 0 || len(snap.Entries) != 1 {
		t.Fatalf("anchor snapshot after initial export = %#v, ok=%t", snap, ok)
	}
	dependent.Base().ClearDirty(facet.DirtyLayout)
	delete(rt.dirtyFacets, dependentID)

	exporter.anchors["mark"] = gfx.Point{X: 40, Y: 60}
	rt.resolveAnchorExports()

	if rt.dirtyFacets[dependentID]&facet.DirtyLayout == 0 {
		t.Fatalf("anchor movement did not re-dirty dependent %v (dirty=%v)", dependentID, rt.dirtyFacets[dependentID])
	}

	if snap, ok := rt.AnchorSnapshot(root.Base().ID()); !ok || snap.Version < 2 || len(snap.Entries) != 1 {
		t.Fatalf("anchor snapshot after movement = %#v, ok=%t", snap, ok)
	}
}
