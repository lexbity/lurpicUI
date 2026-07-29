package ll019_good_core

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/marks"
)

type Child struct {
	facet.Facet
	layout facet.LayoutRole
}

type ParentWithCore struct {
	marks.Core
	child *Child
}

func newParentWithCore() *ParentWithCore {
	p := &ParentWithCore{child: &Child{}}
	p.Core.RegisterRoles()
	p.Facet.AddChild(p.child.Base())
	return p
}
