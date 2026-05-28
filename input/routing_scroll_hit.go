package input

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/projection"
)

func (s *System) processScroll(e platform.EventScroll, hitMap *projection.HitMap) []RoutedEvent {
	if s == nil {
		return nil
	}
	s.SetInputModality(facet.InputModalityPointer)
	ptr := s.getOrCreatePointer(0)
	var targetID facet.FacetID
	var markID facet.MarkID
	if ptr != nil && ptr.PressTarget != nil && ptr.DragActive {
		targetID = ptr.PressTarget.FacetID
		markID = ptr.PressTarget.MarkID
	} else if hitMap != nil {
		if hit := s.resolveHitTarget(hitMap, e.Position); hit != nil {
			targetID = hit.FacetID
			markID = hit.MarkID
		}
	}
	if targetID == 0 {
		return nil
	}
	local := s.transformToLocal(e.Position, targetID, hitMap)
	_ = markID
	return []RoutedEvent{{
		Target: targetID,
		Event: ScrollEvent{
			Position:  local,
			DeltaX:    e.DeltaX * s.config.ScrollMultiplier,
			DeltaY:    e.DeltaY * s.config.ScrollMultiplier,
			Precise:   e.Precise,
			Modifiers: e.Modifiers,
		},
	}}
}

func (s *System) resolveHitTarget(hitMap *projection.HitMap, screenPos gfx.Point) *resolvedHitTarget {
	if s == nil || hitMap == nil {
		return nil
	}
	entries := hitMap.Entries()
	if len(entries) == 0 {
		return nil
	}
	var passthrough *resolvedHitTarget
	for _, entry := range entries {
		if entry.HitPolicy == facet.HitDisabled {
			continue
		}
		local := screenPos
		if inv, ok := entry.Transform.Inverse(); ok {
			local = inv.TransformPoint(screenPos)
		}
		if !entry.ClipRect.IsEmpty() {
			clip := entry.ClipRect
			if inv, ok := entry.Transform.Inverse(); ok {
				clip = inv.TransformRect(clip)
			}
			if !clip.Contains(local) {
				if entry.HitPolicy == facet.HitBlockBelow {
					return passthrough
				}
				continue
			}
		}
		matched := false
		for _, region := range entry.Regions {
			if !projection.HitRegionContains(region, local) {
				continue
			}
			matched = true
			markID, accepted := s.resolveHitMark(entry.FacetID, local, region.MarkID)
			if !accepted {
				if entry.HitPolicy == facet.HitBlockBelow {
					return passthrough
				}
				continue
			}
			target := &resolvedHitTarget{
				FacetID: entry.FacetID,
				MarkID:  markID,
				Local:   local,
			}
			if entry.HitPolicy == facet.HitPassThrough || region.PassThrough {
				if passthrough == nil {
					passthrough = target
				}
				continue
			}
			return target
		}
		if entry.HitPolicy == facet.HitBlockBelow {
			return passthrough
		}
		if !matched && entry.HitPolicy == facet.HitPassThrough && passthrough == nil {
			// Continue looking below. If nothing consumes, the first pass-through hit
			// stays as the fallback target.
		}
	}
	return passthrough
}

func (s *System) resolveHitMark(target facet.FacetID, localPos gfx.Point, regionMark facet.MarkID) (facet.MarkID, bool) {
	if s == nil {
		return regionMark, true
	}
	result := refineHitTest(s.focusTree, target, localPos)
	if result == nil {
		return regionMark, true
	}
	if !result.Hit {
		return 0, false
	}
	if result.MarkID != 0 {
		return result.MarkID, true
	}
	return regionMark, true
}
