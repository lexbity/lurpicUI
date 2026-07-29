package ll019_bad

import (
	"codeburg.org/lexbit/lurpicui/facet"
)

type Child struct {
	facet.Facet
	layout facet.LayoutRole
}

type ParentNoRole struct {
	facet.Facet
	child *Child
}

func newParentNoRole() *ParentNoRole {
	p := &ParentNoRole{child: &Child{}}
	p.Facet = facet.NewFacet()
	p.Facet.AddChild(p.child.Base())
	// NO AddRole call — this is the bug: children are registered but no
	// LayoutRole is registered, so children are never measured/arranged.
	return p
}
