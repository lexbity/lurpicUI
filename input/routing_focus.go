package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

func refineHitTest(tree facet.FacetImpl, id facet.FacetID, localPos gfx.Point) *facet.HitResult {
	path := findFacetPath(tree, id)
	if len(path) == 0 {
		return nil
	}
	hr := path[len(path)-1].Base().HitRole()
	if hr == nil || hr.OnHitTest == nil {
		return nil
	}
	// The facet ID is the id parameter — HitRole carries no identity, and
	// widening its signature is out of scope, so recovery lands here.
	h := currentRecoveryHook()
	if h == nil {
		result := hr.HitTest(localPos)
		return &result
	}
	var result facet.HitResult
	ran := h(id, "hit", func() {
		result = hr.HitTest(localPos)
	})
	if !ran {
		// Quarantined: behave as "no hit here" so input falls through.
		return nil
	}
	return &result
}

func (s *System) requestFocus(targetID facet.FacetID, tree facet.FacetImpl) facet.FacetID {
	if s == nil || targetID == 0 || tree == nil {
		return 0
	}
	path := findFacetPath(tree, targetID)
	if len(path) == 0 {
		return 0
	}
	for i := len(path) - 1; i >= 0; i-- {
		base := path[i].Base()
		if base == nil {
			continue
		}
		role := base.FocusRole()
		if role == nil {
			continue
		}
		focusable := true
		if role.Focusable != nil {
			focusable = role.Focusable()
		}
		if !focusable {
			continue
		}
		if s.focusManager != nil {
			if !s.focusManager.SetFocus(path[i]) {
				continue
			}
			s.focus.SetFocused(s.focusManager.Focused())
		} else {
			s.focus.SetFocused(base.ID())
		}
		s.focusTree = tree
		return s.focus.Focused()
	}
	return 0
}
