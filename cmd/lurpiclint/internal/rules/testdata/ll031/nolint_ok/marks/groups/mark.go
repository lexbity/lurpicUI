package mark

import (
	facet "codeburg.org/lexbit/lurpicui/facet"
)

// groupCore simulates marks.Core.
type groupCore struct {
	facet.Facet
	Layout facet.LayoutRole
}

//nolint:LL031 // deliberate opt-out
type GroupMark struct {
	groupCore
}

func (m *GroupMark) Children() []facet.GroupChild {
	return nil
}

func NewGroupMark() *GroupMark {
	return &GroupMark{}
}
