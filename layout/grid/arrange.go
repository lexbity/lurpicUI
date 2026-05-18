package grid

import (
	"fmt"
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
)

// Arrange positions children in the resolved layer.
func (p *Policy) Arrange(children []Child, layer gfx.Rect) ([]ArrangedChild, error) {
	if p == nil || len(children) == 0 {
		return nil, nil
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
		return nil, err
	}
	colSizes, err := p.resolveAxisSizes(children, placements, true, colDefs, layer.Width())
	if err != nil {
		return nil, err
	}
	rowSizes, err := p.resolveAxisSizes(children, placements, false, rowDefs, layer.Height())
	if err != nil {
		return nil, err
	}
	colOffsets := cumulativeOffsets(colSizes, p.cfg.ColumnGap)
	rowOffsets := cumulativeOffsets(rowSizes, p.cfg.RowGap)
	arranged := make([]ArrangedChild, 0, len(children))
	ordered := sortedChildren(children)
	for _, idx := range ordered {
		child := children[idx]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementGrid) {
			return nil, fmt.Errorf("layout contract violation: facet %d; layer %d; placement grid; violated contract: unsupported placement mode; guidance: set SupportedPlacement to include grid placement", child.FacetID, child.Attachment.LayerID)
		}
		if child.Attachment.Placement.Mode != facet.PlacementGrid {
			return nil, fmt.Errorf("layout contract violation: facet %d; layer %d; placement grid; violated contract: unsupported placement mode; guidance: use facet.PlacementGrid for this child", child.FacetID, child.Attachment.LayerID)
		}
		placement := placements[idx]
		rect := arrangeChildInGrid(layer, colOffsets, rowOffsets, colSizes, rowSizes, child, placement)
		child.Layout.Arrange(facet.ArrangeContext{Placement: child.Attachment.Placement}, rect)
		arranged = append(arranged, ArrangedChild{
			FacetID:   child.FacetID,
			Bounds:    rect,
			Placement: placement,
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	sort.SliceStable(arranged, func(i, j int) bool {
		if arranged[i].Placement.RowStart != arranged[j].Placement.RowStart {
			return arranged[i].Placement.RowStart < arranged[j].Placement.RowStart
		}
		if arranged[i].Placement.ColStart != arranged[j].Placement.ColStart {
			return arranged[i].Placement.ColStart < arranged[j].Placement.ColStart
		}
		if arranged[i].ZPriority != arranged[j].ZPriority {
			return arranged[i].ZPriority > arranged[j].ZPriority
		}
		return arranged[i].FacetID < arranged[j].FacetID
	})
	return arranged, nil
}

func sortedChildren(children []Child) []int {
	indices := make([]int, len(children))
	for i := range indices {
		indices[i] = i
	}
	sort.SliceStable(indices, func(i, j int) bool {
		left := children[indices[i]]
		right := children[indices[j]]
		if left.Attachment.ZPriority != right.Attachment.ZPriority {
			return left.Attachment.ZPriority > right.Attachment.ZPriority
		}
		return left.FacetID < right.FacetID
	})
	return indices
}

func normalizePlacement(mode AutoPlacementMode, grid facet.GridPlacement, cols, rows int, nextCol, nextRow *int, occupied []bool) (Placement, error) {
	placement := Placement{
		ColStart: grid.ColStart,
		RowStart: grid.RowStart,
		ColSpan:  grid.ColSpan,
		RowSpan:  grid.RowSpan,
	}
	auto := placement.ColStart == 0 && placement.RowStart == 0 && placement.ColSpan == 0 && placement.RowSpan == 0
	if auto {
		placement.ColSpan = 1
		placement.RowSpan = 1
		col, row := findNextFreeCell(mode, cols, rows, placement.ColSpan, placement.RowSpan, nextCol, nextRow, occupied)
		if col < 0 || row < 0 {
			return Placement{}, fmt.Errorf("layout/grid: no free cell available")
		}
		placement.ColStart = col
		placement.RowStart = row
		return placement, nil
	}
	if placement.ColSpan <= 0 || placement.RowSpan <= 0 {
		return Placement{}, fmt.Errorf("layout/grid: grid span must be positive")
	}
	if placement.ColStart < 0 || placement.RowStart < 0 {
		return Placement{}, fmt.Errorf("layout/grid: grid line must be non-negative")
	}
	if placement.ColStart >= cols || placement.RowStart >= rows {
		return Placement{}, fmt.Errorf("layout/grid: grid start outside track range")
	}
	if placement.ColStart+placement.ColSpan > cols {
		placement.ColSpan = cols - placement.ColStart
	}
	if placement.RowStart+placement.RowSpan > rows {
		placement.RowSpan = rows - placement.RowStart
	}
	if placement.ColSpan <= 0 || placement.RowSpan <= 0 {
		return Placement{}, fmt.Errorf("layout/grid: grid span collapsed after clamping")
	}
	return placement, nil
}

func findNextFreeCell(mode AutoPlacementMode, cols, rows, colSpan, rowSpan int, nextCol, nextRow *int, occupied []bool) (int, int) {
	if cols <= 0 || rows <= 0 {
		return -1, -1
	}
	startRow := 0
	startCol := 0
	if nextCol != nil {
		startCol = *nextCol
	}
	if nextRow != nil {
		startRow = *nextRow
	}
	scan := func(row, col int) (int, int, bool) {
		if col+colSpan > cols || row+rowSpan > rows {
			return 0, 0, false
		}
		if !cellSpanFree(occupied, cols, col, row, colSpan, rowSpan) {
			return 0, 0, false
		}
		if nextCol != nil {
			*nextCol = col + colSpan
			if *nextCol >= cols {
				*nextCol = 0
				if nextRow != nil {
					*nextRow = row + 1
				}
			} else if nextRow != nil {
				*nextRow = row
			}
		}
		return col, row, true
	}
	if mode == AutoColumnFirst {
		for col := startCol; col < cols; col++ {
			for row := startRow; row < rows; row++ {
				if c, r, ok := scan(row, col); ok {
					return c, r
				}
			}
			startRow = 0
		}
		return -1, -1
	}
	for row := startRow; row < rows; row++ {
		for col := startCol; col < cols; col++ {
			if c, r, ok := scan(row, col); ok {
				return c, r
			}
		}
		startCol = 0
	}
	return -1, -1
}

func cellSpanFree(occupied []bool, cols, col, row, colSpan, rowSpan int) bool {
	for r := 0; r < rowSpan; r++ {
		for c := 0; c < colSpan; c++ {
			idx := (row+r)*cols + col + c
			if idx < 0 || idx >= len(occupied) {
				return false
			}
			if occupied[idx] {
				return false
			}
		}
	}
	return true
}

func markOccupied(occupied []bool, cols int, placement Placement) {
	for r := 0; r < placement.RowSpan; r++ {
		for c := 0; c < placement.ColSpan; c++ {
			idx := (placement.RowStart+r)*cols + placement.ColStart + c
			if idx >= 0 && idx < len(occupied) {
				occupied[idx] = true
			}
		}
	}
}

func arrangeChildInGrid(layer gfx.Rect, colOffsets, rowOffsets, colSizes, rowSizes []float32, child Child, placement Placement) gfx.Rect {
	cellX, cellW := spanExtent(colOffsets, colSizes, placement.ColStart, placement.ColSpan, layer.Min.X, layer.Width())
	cellY, cellH := spanExtent(rowOffsets, rowSizes, placement.RowStart, placement.RowSpan, layer.Min.Y, layer.Height())
	cell := gfx.RectFromXYWH(cellX, cellY, cellW, cellH)
	return alignInCell(cell, child)
}

func spanExtent(offsets, sizes []float32, start, span int, origin, available float32) (float32, float32) {
	if len(sizes) == 0 {
		return origin, available
	}
	if start < 0 {
		start = 0
	}
	if start >= len(sizes) {
		start = len(sizes) - 1
	}
	if span < 1 {
		span = 1
	}
	if start+span > len(sizes) {
		span = len(sizes) - start
	}
	pos := origin + offsets[start]
	size := float32(0)
	for i := start; i < start+span; i++ {
		size += sizes[i]
	}
	return pos, size
}

func cumulativeOffsets(sizes []float32, gap float32) []float32 {
	if len(sizes) == 0 {
		return nil
	}
	offsets := make([]float32, len(sizes))
	cur := float32(0)
	for i := range sizes {
		offsets[i] = cur
		cur += sizes[i] + gap
	}
	return offsets
}

func alignInCell(cell gfx.Rect, child Child) gfx.Rect {
	align := facet.AlignStretch
	if child.Attachment.Placement.Align != 0 {
		align = child.Attachment.Placement.Align
	}
	if align == facet.AlignStretch {
		return cell
	}
	size := child.Layout.MeasuredSize
	if size == (gfx.Size{}) {
		size = gfx.Size{W: cell.Width(), H: cell.Height()}
	}
	childW := size.W
	childH := size.H
	if childW > cell.Width() {
		childW = cell.Width()
	}
	if childH > cell.Height() {
		childH = cell.Height()
	}
	x := cell.Min.X
	y := cell.Min.Y
	switch align {
	case facet.AlignCenter, facet.AlignTopCenter, facet.AlignBottomCenter:
		x += (cell.Width() - childW) / 2
	case facet.AlignEnd, facet.AlignTopRight, facet.AlignCenterRight, facet.AlignBottomRight:
		x += cell.Width() - childW
	}
	switch align {
	case facet.AlignCenter, facet.AlignCenterLeft, facet.AlignCenterRight:
		y += (cell.Height() - childH) / 2
	case facet.AlignEnd, facet.AlignBottomLeft, facet.AlignBottomCenter, facet.AlignBottomRight:
		y += cell.Height() - childH
	}
	return gfx.RectFromXYWH(x, y, childW, childH)
}
