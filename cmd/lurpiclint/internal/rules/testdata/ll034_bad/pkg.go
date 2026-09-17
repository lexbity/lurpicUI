package ll034_bad

import "codeburg.org/lexbit/lurpicui/facet"

type Overlay struct{ facet.Facet }

func (o *Overlay) Base() *facet.Facet           { return &o.Facet }
func (o *Overlay) OnAttach(facet.AttachContext) {}
func (o *Overlay) OnDetach()                    {}
func (o *Overlay) OnActivate()                  {}
func (o *Overlay) OnDeactivate()                {}

type Host struct {
	facet.Facet
	overlay *Overlay
}

func newHost() *Host {
	h := &Host{overlay: &Overlay{Facet: facet.NewFacet()}}
	h.Facet = facet.NewFacet()
	facet.AttachLayer(h, h.overlay, facet.LayerAttachment{Band: facet.ZBandModal})
	return h
}

func (h *Host) Children() []facet.GroupChild {
	return []facet.GroupChild{{FacetID: h.overlay.Base().ID()}}
}
