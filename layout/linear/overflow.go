package linear

import "codeburg.org/lexbit/lurpicui/gfx"

// Overflow reports the linear group's main-axis content extent against the
// arranged main-axis extent (RX-1 FR-5 per-axis overflow reporting). Container
// marks use it to apply their declared OverflowPolicy; the policy stays pure
// geometry.
type Overflow struct {
	// Need is the total content extent along the main axis (child measured
	// sizes plus gaps).
	Need float32
	// Size is the arranged main-axis extent.
	Size float32
}

// MainAxisOverflow reports whether the group's content overflows the arranged
// main-axis extent. Need > Size means the content extends beyond the bounds
// along the main axis.
func (p *Policy) MainAxisOverflow(children []Child, bounds gfx.Rect) Overflow {
	if p == nil || len(children) == 0 {
		return Overflow{Size: mainExtent(bounds, p.cfg.Axis)}
	}
	ordered := sortedChildren(children)
	need := float32(0)
	count := 0
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		size := measuredSize(child)
		need += mainSize(size, p.cfg.Axis)
		count++
	}
	if count > 1 {
		need += p.cfg.Gap * float32(count-1)
	}
	return Overflow{Need: need, Size: mainExtent(bounds, p.cfg.Axis)}
}
