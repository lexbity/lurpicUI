package ll034_good

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
	content *Overlay
}

func newHost() *Host {
	h := &Host{overlay: &Overlay{Facet: facet.NewFacet()}, content: &Overlay{Facet: facet.NewFacet()}}
	h.Facet = facet.NewFacet()
	h.AddChild(h.content.Base())
	facet.AttachLayer(h, h.overlay, facet.LayerAttachment{Band: facet.ZBandModal})
	return h
}

// The layer overlay is NOT a group child; only the content child is.
func (h *Host) Children() []facet.GroupChild {
	return []facet.GroupChild{{FacetID: h.content.Base().ID()}}
}
