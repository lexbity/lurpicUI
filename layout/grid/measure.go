package grid

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Measure computes the preferred grid size.
func (p *Policy) Measure(children []Child, constraints gfx.Size) (gfx.Size, error) {
	if p == nil {
		return gfx.Size{}, nil
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
		return gfx.Size{}, err
	}
	colSizes, err := p.resolveAxisSizes(children, placements, true, colDefs, constraints.W)
	if err != nil {
		return gfx.Size{}, err
	}
	rowSizes, err := p.resolveAxisSizes(children, placements, false, rowDefs, constraints.H)
	if err != nil {
		return gfx.Size{}, err
	}
	return gfx.Size{
		W: sumTrackSizes(colSizes, p.cfg.ColumnGap),
		H: sumTrackSizes(rowSizes, p.cfg.RowGap),
	}, nil
}

func (p *Policy) resolvePlacements(children []Child, cols, rows int) ([]Placement, error) {
	placements := make([]Placement, len(children))
	occupied := make([]bool, cols*rows)
	nextCol, nextRow := 0, 0
	ordered := sortedChildren(children)
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		if child.Attachment.Placement.Mode != facet.PlacementGrid {
			return nil, fmt.Errorf("layout/grid: unsupported placement mode")
		}
		placement, err := normalizePlacement(p.cfg.AutoPlacement, child.Attachment.Placement.Grid, cols, rows, &nextCol, &nextRow, occupied)
		if err != nil {
			return nil, err
		}
		markOccupied(occupied, cols, placement)
		placements[idx] = placement
	}
	return placements, nil
}

func (p *Policy) resolveAxisSizes(children []Child, placements []Placement, horizontal bool, defs []TrackDef, available float32) ([]float32, error) {
	count := len(defs)
	if count == 0 {
		return nil, nil
	}
	sizes := make([]float32, count)
	for i := range defs {
		switch defs[i].Sizing {
		case TrackFixed:
			sizes[i] = clampFloat(defs[i].Value, defs[i].Min, defs[i].Max)
		case TrackIntrinsic, TrackFlex:
			sizes[i] = clampFloat(defs[i].Min, defs[i].Min, defs[i].Max)
		default:
			return nil, fmt.Errorf("layout/grid: unknown track sizing")
		}
	}
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if child.Attachment.Placement.Mode != facet.PlacementGrid {
			return nil, fmt.Errorf("layout/grid: unsupported placement mode")
		}
		start, span, err := childAxisSpan(placements[i], horizontal, count)
		if err != nil {
			return nil, err
		}
		need := childAxisNeed(child, horizontal)
		if span > 0 {
			need /= float32(span)
		}
		for t := start; t < start+span; t++ {
			if t < 0 || t >= len(defs) {
				continue
			}
			switch defs[t].Sizing {
			case TrackIntrinsic:
				if need > sizes[t] {
					sizes[t] = need
				}
			case TrackFixed:
				// Fixed tracks remain fixed; content can overflow them.
			case TrackFlex:
				// Flex tracks distribute remaining space; content does not
				// change their base size in this phase.
			}
		}
	}
	fixedTotal := float32(0)
	flexWeightTotal := float32(0)
	for i := range defs {
		switch defs[i].Sizing {
		case TrackFixed, TrackIntrinsic:
			fixedTotal += sizes[i]
		case TrackFlex:
			flexWeightTotal += maxFloat(defs[i].Value, 1)
			if sizes[i] < defs[i].Min {
				sizes[i] = defs[i].Min
			}
		}
	}
	remaining := available - fixedTotal - gapTotal(count, gapFor(horizontal, p.cfg))
	if remaining < 0 {
		remaining = 0
	}
	if flexWeightTotal > 0 {
		unit := remaining / flexWeightTotal
		for i := range defs {
			if defs[i].Sizing != TrackFlex {
				continue
			}
			sizes[i] += unit * maxFloat(defs[i].Value, 1)
			if defs[i].Max > 0 && sizes[i] > defs[i].Max {
				sizes[i] = defs[i].Max
			}
		}
	}
	return sizes, nil
}

func childAxisNeed(child Child, horizontal bool) float32 {
	if child.Layout == nil {
		return 0
	}
	size := child.Layout.MeasuredSize
	if size == (gfx.Size{}) {
		size = child.Layout.Measure(facet.MeasureContext{}, facet.Constraints{}).Size
	}
	if horizontal {
		return size.W
	}
	return size.H
}

func childAxisSpan(placement Placement, horizontal bool, count int) (int, int, error) {
	if count <= 0 {
		return 0, 0, fmt.Errorf("layout/grid: empty track count")
	}
	start := placement.ColStart
	span := placement.ColSpan
	if !horizontal {
		start = placement.RowStart
		span = placement.RowSpan
	}
	if start < 0 {
		return 0, 0, fmt.Errorf("layout/grid: grid line must be non-negative")
	}
	if span <= 0 {
		return 0, 0, fmt.Errorf("layout/grid: grid span must be positive")
	}
	if start >= count {
		return 0, 0, fmt.Errorf("layout/grid: grid start outside track range")
	}
	if start+span > count {
		span = count - start
	}
	if span <= 0 {
		return 0, 0, fmt.Errorf("layout/grid: grid span collapsed after clamping")
	}
	return start, span, nil
}

func gapFor(horizontal bool, cfg Config) float32 {
	if horizontal {
		return cfg.ColumnGap
	}
	return cfg.RowGap
}

func gapTotal(trackCount int, gap float32) float32 {
	if trackCount <= 1 {
		return 0
	}
	return gap * float32(trackCount-1)
}

func sumTrackSizes(sizes []float32, gap float32) float32 {
	total := float32(0)
	for i := range sizes {
		total += sizes[i]
	}
	total += gapTotal(len(sizes), gap)
	return total
}

func maxFloat(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
