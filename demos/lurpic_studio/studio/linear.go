package studio

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout/linear"
)

// linearChild measures one facet for a linear group-parent host and returns
// its linear.Child. Forwarding the host's constraints lets the child measure
// against real available space; the linear policy reuses the measured size
// when present, so arrange passes must not re-measure.
func linearChild(ctx facet.MeasureContext, max gfx.Size, f facet.FacetImpl, placement facet.LinearPlacement) linear.Child {
	child := linearChildOf(f, placement)
	if child.Layout == nil {
		return child
	}
	child.Layout.Measure(ctx, facet.Constraints{MaxSize: max})
	return child
}

// linearChildOf builds a linear.Child for an already-measured facet.
func linearChildOf(f facet.FacetImpl, placement facet.LinearPlacement) linear.Child {
	if f == nil || f.Base() == nil {
		return linear.Child{}
	}
	role := f.Base().LayoutRole()
	return linear.Child{
		FacetID:    f.Base().ID(),
		Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: placement}},
		Layout:     role,
		Contract:   role.Child,
	}
}

// linearChildContract is the shared child-placement contract for the shell
// hosts. It always declares SupportsGrid so a host stays arrangeable when the
// runtime drives it directly (the app/harness root is arranged with the
// default grid placement), plus SupportsLinear for its linear group parent.
func linearChildContract(stretch facet.StretchPolicy) facet.GroupChildContract {
	return facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid | facet.SupportsLinear,
		Stretch:            stretch,
	}
}

// arrangeChild arranges a facet's layout role to the given bounds using a
// placement its child contract actually supports. Marks disagree on placement
// support (badge declares SupportsLinear only; text/icon_button declare grid),
// so a hand-arranged host picks the first supported mode instead of assuming
// the default grid placement (F-badge-contract).
func arrangeChild(ctx facet.ArrangeContext, f facet.FacetImpl, bounds gfx.Rect) {
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	placement := facet.Placement{Mode: facet.PlacementGrid}
	if role.Child.SupportedPlacement != 0 && !role.Child.SupportedPlacement.Has(facet.PlacementGrid) {
		placement.Mode = facet.PlacementLinear
	}
	ctx.Placement = placement
	role.Arrange(ctx, bounds)
}

// linearGroupChild builds one GroupChild for a linearly placed facet — the
// shared shape of every shell host's Children() method.
func linearGroupChild(placement facet.LinearPlacement, f facet.FacetImpl) facet.GroupChild {
	role := f.Base().LayoutRole()
	return facet.GroupChild{
		FacetID:    f.Base().ID(),
		Attachment: facet.Attachment{Placement: facet.Placement{Mode: facet.PlacementLinear, Linear: placement}},
		Layout:     role,
		Contract:   role.Child,
	}
}

// linearGroupChildren builds the group-child list for an ordered run of
// facets (the hand-arranged hosts' ChildSource).
func linearGroupChildren(items []facet.FacetImpl) []facet.GroupChild {
	out := make([]facet.GroupChild, 0, len(items))
	for i, item := range items {
		if item == nil || item.Base() == nil || item.Base().LayoutRole() == nil {
			continue
		}
		out = append(out, linearGroupChild(facet.LinearPlacement{Order: i}, item))
	}
	return out
}

// invalidateLayout requests a runtime layout pass for f. facet.Facet.Invalidate
// only sets the facet's local dirty bits, which the runtime's layout pass does
// not read (its gate is rt.dirtyFacets); attached hosts must route through
// facet.RuntimeServices.Invalidate so the runtime re-lays them
// (F-dirtylayout-routing). Outside a runtime (construction / standalone tests)
// it falls back to the local bits, which fire synchronously.
func invalidateLayout(f facet.FacetImpl, rt facet.RuntimeServices, source string) {
	if rt != nil {
		rt.Invalidate(f.Base().ID(), facet.DirtyLayout, source)
		return
	}
	f.Base().Invalidate(facet.DirtyLayout)
}

// crossStretch places a child to fill a linear host's cross axis.
func crossStretch(order int) facet.LinearPlacement {
	return facet.LinearPlacement{Order: order, CrossAxisAlign: facet.CrossAxisStretch}
}

// groupPolicy adapts a host facet's LayoutRole to the GroupLayoutPolicy
// contract (the group-parent bridge, §1.6). The runtime drives a host's
// LayoutRole directly, so this policy's job is to keep the declared
// Parent.Kind contract sound and behave correctly if a group driver ever
// consumes it. One shared type serves every shell host.
type groupPolicy struct {
	kind facet.GroupLayoutKind
	host facet.FacetImpl
}

func (p groupPolicy) Kind() facet.GroupLayoutKind { return p.kind }

func (p groupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.host == nil || p.host.Base() == nil {
		return facet.GroupMeasureResult{}, nil
	}
	role := p.host.Base().LayoutRole()
	if role == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := role.Measure(ctx.MeasureContext, facet.Constraints{
		MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()},
	}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p groupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.host == nil || p.host.Base() == nil {
		return nil, nil
	}
	role := p.host.Base().LayoutRole()
	if role == nil {
		return nil, nil
	}
	role.Arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    child.Layout.ArrangedBounds,
			Placement: child.Attachment.Placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
