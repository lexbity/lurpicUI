package grid

import "codeburg.org/lexbit/lurpicui/gfx"

// TrackOverflow reports one grid track whose content need exceeds its
// allocated size after arrangement (RX-1 FR-5 per-track overflow reporting).
// Container marks use the report to apply their declared OverflowPolicy
// (viewport+scroll, clip, or upward min-content report); the policies stay
// pure geometry.
type TrackOverflow struct {
	// Horizontal reports the axis: true for a column track, false for a row.
	Horizontal bool
	// Index is the track's position in the column/row definition.
	Index int
	// Need is the largest per-track content need among children spanning the
	// track (a spanning child's need is divided across its span).
	Need float32
	// Size is the track's final allocated size after arrangement.
	Size float32
}

// OverflowTracks reports every column/row track whose content need exceeds its
// allocated size when the given children are arranged within layer. Intrinsic
// tracks are sized to their need by resolveAxisSizes and never overflow;
// Fixed and Flex tracks can compress below content.
func (p *Policy) OverflowTracks(children []Child, layer gfx.Rect) []TrackOverflow {
	if p == nil || len(children) == 0 {
		return nil
	}
	colDefs := p.cfg.Columns
	rowDefs := p.cfg.Rows
	if len(colDefs) == 0 {
		colDefs = defaultFlexTracks(5)
	}
	if len(rowDefs) == 0 {
		rowDefs = defaultFlexTracks(5)
	}
	placements, err := p.resolvePlacements(children, len(colDefs), len(rowDefs))
	if err != nil {
		return nil
	}
	colSizes, err := p.resolveAxisSizes(children, placements, true, colDefs, layer.Width())
	if err != nil {
		return nil
	}
	rowSizes, err := p.resolveAxisSizes(children, placements, false, rowDefs, layer.Height())
	if err != nil {
		return nil
	}
	var out []TrackOverflow
	out = append(out, p.axisOverflow(children, placements, true, colSizes, colDefs)...)
	out = append(out, p.axisOverflow(children, placements, false, rowSizes, rowDefs)...)
	return out
}

func (p *Policy) axisOverflow(children []Child, placements []Placement, horizontal bool, sizes []float32, defs []TrackDef) []TrackOverflow {
	needs := make([]float32, len(sizes))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		start, span, err := childAxisSpan(placements[i], horizontal, len(sizes))
		if err != nil {
			continue
		}
		need := childAxisNeed(child, horizontal)
		if span > 0 {
			need /= float32(span)
		}
		for t := start; t < start+span; t++ {
			if t < 0 || t >= len(needs) {
				continue
			}
			if need > needs[t] {
				needs[t] = need
			}
		}
	}
	var out []TrackOverflow
	for t := range sizes {
		if defs[t].Sizing == TrackIntrinsic {
			continue
		}
		if needs[t] > sizes[t]+trackOverflowEpsilon {
			out = append(out, TrackOverflow{Horizontal: horizontal, Index: t, Need: needs[t], Size: sizes[t]})
		}
	}
	return out
}

// trackOverflowEpsilon absorbs float noise in the per-track need comparison.
const trackOverflowEpsilon = 0.01
