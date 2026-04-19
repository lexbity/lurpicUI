//go:build rolenegative

package roleexternal

import "codeburg.org/lexbit/lurpicui/facet"

type externalRole struct{}

func (externalRole) onAttach(*facet.Facet)     {}
func (externalRole) onActivate(*facet.Facet)   {}
func (externalRole) onDeactivate(*facet.Facet) {}
func (externalRole) onDispose(*facet.Facet)    {}

var _ facet.Role = externalRole{}
