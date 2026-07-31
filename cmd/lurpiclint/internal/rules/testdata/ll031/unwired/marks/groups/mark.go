package mark

import (
	facet "codeburg.org/lexbit/lurpicui/facet"
)

// groupCore simulates marks.Core: it embeds facet.Facet and carries the role
// fields that embedding marks inherit via Go's promotion rules.  The fingerprint
// must traverse this embedded type to see the layout role — a direct-field-only
// walk would miss it.
type groupCore struct {
	facet.Facet
	Layout facet.LayoutRole
}

type GroupMark struct {
	groupCore
}

func (m *GroupMark) Children() []facet.GroupChild {
	return nil
}

func NewGroupMark() *GroupMark {
	return &GroupMark{}
}
