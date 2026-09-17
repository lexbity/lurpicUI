package ll035_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/store"
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
		store.NewValueStore(0).Set(1) // store write inside projection
	}
	return m
}
