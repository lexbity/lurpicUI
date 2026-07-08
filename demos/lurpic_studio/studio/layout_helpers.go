package studio

import "codeburg.org/lexbit/lurpicui/facet"

func allowLinear(f facet.FacetImpl) {
	if f == nil {
		return
	}
	lr := f.Base().LayoutRole()
	if lr == nil {
		return
	}
	lr.Child.SupportedPlacement = 0
}
