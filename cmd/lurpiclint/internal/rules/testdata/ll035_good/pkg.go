package ll035_good

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

type Mark struct{ facet.Facet }

func (m *Mark) Base() *facet.Facet           { return &m.Facet }
func (m *Mark) OnAttach(facet.AttachContext) {}
func (m *Mark) OnDetach()                    {}
func (m *Mark) OnActivate()                  {}
func (m *Mark) OnDeactivate()                {}

func newMark() *Mark {
	m := &Mark{Facet: facet.NewFacet()}
	m.Projection.OnProject = func(ctx facet.ProjectionContext) {
		_ = ctx.ContentScale // read-only projection
	}
	return m
}
