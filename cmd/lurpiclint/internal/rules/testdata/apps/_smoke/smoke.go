package smoke

import "codeburg.org/lexbit/lurpicui/facet"

type Simple struct {
	facet.Facet
}

func (s *Simple) Base() *facet.Facet     { return &s.Facet }
func (s *Simple) OnAttach(facet.AttachContext) {}
func (s *Simple) OnDetach()                     {}
func (s *Simple) OnActivate()                   {}
func (s *Simple) OnDeactivate()                 {}
