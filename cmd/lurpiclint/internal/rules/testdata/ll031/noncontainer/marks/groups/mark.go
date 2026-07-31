package mark

import (
	facet "codeburg.org/lexbit/lurpicui/facet"
)

type SoloGroup struct{}

func (m *SoloGroup) Children() []facet.GroupChild {
	return nil
}

func NewSoloGroup() *SoloGroup {
	return &SoloGroup{}
}
