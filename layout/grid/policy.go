package grid

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// TrackSizing selects how a track gets its size.
type TrackSizing uint8

const (
	TrackFixed TrackSizing = iota
	TrackIntrinsic
	TrackFlex
)

// TrackDef describes a single row or column track.
type TrackDef struct {
	Sizing TrackSizing
	Value  float32
	Min    float32
	Max    float32
}

// AutoPlacementMode determines the order auto-placed children follow.
type AutoPlacementMode uint8

const (
	AutoRowFirst AutoPlacementMode = iota
	AutoColumnFirst
)

// Config configures the grid policy.
type Config struct {
	Columns       []TrackDef
	Rows          []TrackDef
	ColumnGap     float32
	RowGap        float32
	AutoPlacement AutoPlacementMode
}

// Policy arranges children in a 2D track grid.
type Policy struct {
	cfg Config
}

var (
	defaultColumnTracks = []TrackDef{{Sizing: TrackIntrinsic}}
	defaultRowTracks    = []TrackDef{{Sizing: TrackIntrinsic}}
)

// New constructs a grid policy.
func New(cfg Config) *Policy {
	return &Policy{cfg: cfg}
}

// Measure computes the preferred grid size.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	if p == nil {
		return gfx.Size{}
	}
	if len(children) == 0 {
		return gfx.Size{}
	}
	colCount := gridTrackCount(p.cfg.Columns)
	rowCount := gridTrackCount(p.cfg.Rows)
	colSizes := p.resolveAxisSizes(children, true, colCount, axisAvailable(constraints, true))
	rowSizes := p.resolveAxisSizes(children, false, rowCount, axisAvailable(constraints, false))
	return gfx.Size{
		W: sumTrackSizes(colSizes, p.cfg.ColumnGap),
		H: sumTrackSizes(rowSizes, p.cfg.RowGap),
	}
}

// Arrange positions children in the resolved layer.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}

	colCount := gridTrackCount(p.cfg.Columns)
	rowCount := gridTrackCount(p.cfg.Rows)
	if colCount == 0 {
		colCount = 1
	}
	if rowCount == 0 {
		rowCount = 1
	}

	if len(children) <= 16 && colCount <= 16 && rowCount <= 16 {
		p.arrangeNoAlloc(children, layer, colCount, rowCount)
		return
	}
	p.arrangeAlloc(children, layer, colCount, rowCount)
}

func (p *Policy) arrangeNoAlloc(children []layout.ChildNode, layer layout.ResolvedLayer, colCount, rowCount int) {
	var orderBuf [16]int
	var placementBuf [16]gridPlacement
	var occupiedBuf [256]bool
	order := orderBuf[:len(children)]
	for i := range order {
		order[i] = i
	}
	insertionSortChildren(order, children)

	nextCol, nextRow := 0, 0
	occupied := occupiedBuf[:colCount*rowCount]
	for _, idx := range order {
		placement := p.resolvePlacement(children[idx], colCount, rowCount, &nextCol, &nextRow, occupied)
		placementBuf[idx] = placement
		markOccupied(occupied, colCount, placement)
	}

	var colSizesBuf [16]float32
	var rowSizesBuf [16]float32
	var colOffsetsBuf [16]float32
	var rowOffsetsBuf [16]float32
	colSizes := colSizesBuf[:colCount]
	rowSizes := rowSizesBuf[:rowCount]
	p.fillAxisSizes(children, true, colSizes, axisRectAvail(layer.Bounds, true))
	p.fillAxisSizes(children, false, rowSizes, axisRectAvail(layer.Bounds, false))

	colOffsets := prefixOffsets(colSizes, p.cfg.ColumnGap, &colOffsetsBuf)
	rowOffsets := prefixOffsets(rowSizes, p.cfg.RowGap, &rowOffsetsBuf)

	for i := range children {
		placement := placementBuf[i]
		cellX, cellW := spanExtent(colOffsets, colSizes, p.cfg.ColumnGap, placement.ColStart, placement.ColSpan)
		cellY, cellH := spanExtent(rowOffsets, rowSizes, p.cfg.RowGap, placement.RowStart, placement.RowSpan)
		rect := alignInCell(children[i], placement.Align, gfx.RectFromXYWH(cellX, cellY, cellW, cellH))
		children[i].SetArrangedBounds(rect)
	}
}

func (p *Policy) arrangeAlloc(children []layout.ChildNode, layer layout.ResolvedLayer, colCount, rowCount int) {
	ordered := p.sortedChildren(children)
	placements := make([]gridPlacement, len(children))
	occupied := make([]bool, colCount*rowCount)
	nextCol, nextRow := 0, 0
	for _, idx := range ordered {
		placement := p.resolvePlacement(children[idx], colCount, rowCount, &nextCol, &nextRow, occupied)
		placements[idx] = placement
		markOccupied(occupied, colCount, placement)
	}

	colSizes := p.resolveAxisSizes(children, true, colCount, axisRectAvail(layer.Bounds, true))
	rowSizes := p.resolveAxisSizes(children, false, rowCount, axisRectAvail(layer.Bounds, false))
	colOffsets := cumulativeOffsets(colSizes, p.cfg.ColumnGap)
	rowOffsets := cumulativeOffsets(rowSizes, p.cfg.RowGap)

	for i := range children {
		placement := placements[i]
		cellX, cellW := spanExtent(colOffsets, colSizes, p.cfg.ColumnGap, placement.ColStart, placement.ColSpan)
		cellY, cellH := spanExtent(rowOffsets, rowSizes, p.cfg.RowGap, placement.RowStart, placement.RowSpan)
		rect := alignInCell(children[i], placement.Align, gfx.RectFromXYWH(cellX, cellY, cellW, cellH))
		children[i].SetArrangedBounds(rect)
	}
}

type gridPlacement struct {
	ColStart int
	ColSpan  int
	RowStart int
	RowSpan  int
	Align    layout.Alignment
}

func (p *Policy) resolvePlacement(child layout.ChildNode, colCount, rowCount int, nextCol, nextRow *int, occupied []bool) gridPlacement {
	colStart := clampTrackIndex(child.Attachment.Placement.ColStart, colCount)
	rowStart := clampTrackIndex(child.Attachment.Placement.RowStart, rowCount)
	colSpan := clampSpan(child.Attachment.Placement.ColSpan, colCount-colStart)
	rowSpan := clampSpan(child.Attachment.Placement.RowSpan, rowCount-rowStart)
	if child.Attachment.Placement.ColStart == 0 && child.Attachment.Placement.RowStart == 0 {
		colStart, rowStart = p.autoPlace(colCount, rowCount, colSpan, rowSpan, nextCol, nextRow, occupied)
	}
	return gridPlacement{
		ColStart: colStart,
		ColSpan:  colSpan,
		RowStart: rowStart,
		RowSpan:  rowSpan,
		Align:    child.Attachment.Placement.Align,
	}
}

func (p *Policy) autoPlace(colCount, rowCount, colSpan, rowSpan int, nextCol, nextRow *int, occupied []bool) (int, int) {
	col := *nextCol
	row := *nextRow
	if p.cfg.AutoPlacement == AutoColumnFirst {
		for r := row; r < rowCount; r++ {
			for c := col; c < colCount; c++ {
				if c+colSpan > colCount || !cellSpanFree(occupied, colCount, c, r, colSpan, rowSpan) {
					continue
				}
				*nextRow = r
				*nextCol = c + colSpan
				return c, r
			}
			col = 0
		}
		*nextRow = 0
		*nextCol = 0
		return 0, 0
	}
	for r := row; r < rowCount; r++ {
		for c := col; c < colCount; c++ {
			if c+colSpan > colCount || !cellSpanFree(occupied, colCount, c, r, colSpan, rowSpan) {
				continue
			}
			*nextCol = c + colSpan
			*nextRow = r
			return c, r
		}
		col = 0
	}
	*nextCol = 0
	*nextRow = 0
	return 0, 0
}

func markOccupied(occupied []bool, colCount int, placement gridPlacement) {
	for r := 0; r < placement.RowSpan; r++ {
		for c := 0; c < placement.ColSpan; c++ {
			idx := (placement.RowStart+r)*colCount + placement.ColStart + c
			if idx >= 0 && idx < len(occupied) {
				occupied[idx] = true
			}
		}
	}
}

func cellSpanFree(occupied []bool, colCount, col, row, colSpan, rowSpan int) bool {
	for r := 0; r < rowSpan; r++ {
		for c := 0; c < colSpan; c++ {
			idx := (row+r)*colCount + col + c
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

func (p *Policy) sortedChildren(children []layout.ChildNode) []int {
	indices := make([]int, len(children))
	for i := range indices {
		indices[i] = i
	}
	for i := 1; i < len(indices); i++ {
		j := i
		for j > 0 && lessChild(children, indices[j], indices[j-1]) {
			indices[j], indices[j-1] = indices[j-1], indices[j]
			j--
		}
	}
	return indices
}

func (p *Policy) resolveAxisSizes(children []layout.ChildNode, horizontal bool, trackCount int, available float32) []float32 {
	if trackCount <= 0 {
		return nil
	}
	if trackCount <= 16 {
		var track [16]float32
		sizes := track[:trackCount]
		p.fillAxisSizes(children, horizontal, sizes, available)
		out := make([]float32, trackCount)
		copy(out, sizes)
		return out
	}
	sizes := make([]float32, trackCount)
	p.fillAxisSizes(children, horizontal, sizes, available)
	return sizes
}

func (p *Policy) fillAxisSizes(children []layout.ChildNode, horizontal bool, sizes []float32, available float32) {
	if len(sizes) == 0 {
		return
	}
	trackDefs := p.trackDefs(horizontal)
	for i := range sizes {
		sizes[i] = 0
	}

	for i := range sizes {
		def := trackDefs[i]
		switch def.Sizing {
		case TrackFixed:
			sizes[i] = clamp(def.Value, def.Min, def.Max)
		case TrackIntrinsic:
			sizes[i] = clamp(def.Min, def.Min, def.Max)
		case TrackFlex:
			sizes[i] = clamp(def.Min, def.Min, def.Max)
		}
	}

	for i := range children {
		child := children[i]
		start, span := childAxisSpan(child, horizontal, len(sizes))
		need := childAxisNeed(child, horizontal) / float32(span)
		for t := start; t < start+span; t++ {
			if trackDefs[t].Sizing == TrackIntrinsic && need > sizes[t] {
				sizes[t] = need
			}
			if trackDefs[t].Sizing == TrackFlex && need > sizes[t] {
				sizes[t] = max(sizes[t], trackDefs[t].Min)
			}
		}
	}

	gaps := gapTotal(len(sizes), gapFor(horizontal, p.cfg))
	fixedTotal := float32(0)
	flexWeightTotal := float32(0)
	flexMinTotal := float32(0)
	for i := range sizes {
		switch trackDefs[i].Sizing {
		case TrackFixed, TrackIntrinsic:
			fixedTotal += sizes[i]
		case TrackFlex:
			flexWeightTotal += max(trackDefs[i].Value, 0)
			flexMinTotal += sizes[i]
		}
	}

	remaining := available - fixedTotal - flexMinTotal - gaps
	if remaining < 0 {
		remaining = 0
	}

	if flexWeightTotal > 0 {
		low := float32(0)
		high := remaining / flexWeightTotal
		for iter := 0; iter < 32; iter++ {
			mid := (low + high) / 2
			sum := float32(0)
			for i := range sizes {
				if trackDefs[i].Sizing != TrackFlex {
					sum += sizes[i]
					continue
				}
				size := sizes[i] + trackDefs[i].Value*mid
				if trackDefs[i].Max > 0 && size > trackDefs[i].Max {
					size = trackDefs[i].Max
				}
				sum += size
			}
			sum += gaps
			if sum > available {
				high = mid
			} else {
				low = mid
			}
		}
		for i := range sizes {
			if trackDefs[i].Sizing != TrackFlex {
				continue
			}
			sizes[i] = sizes[i] + trackDefs[i].Value*low
			if trackDefs[i].Max > 0 && sizes[i] > trackDefs[i].Max {
				sizes[i] = trackDefs[i].Max
			}
		}
	}
}

func (p *Policy) trackDefs(horizontal bool) []TrackDef {
	if horizontal {
		if len(p.cfg.Columns) == 0 {
			return defaultColumnTracks
		}
		return p.cfg.Columns
	}
	if len(p.cfg.Rows) == 0 {
		return defaultRowTracks
	}
	return p.cfg.Rows
}

func childAxisNeed(child layout.ChildNode, horizontal bool) float32 {
	if horizontal {
		return max(child.IntrinsicSize.W, child.MinSize.W)
	}
	return max(child.IntrinsicSize.H, child.MinSize.H)
}

func childAxisSpan(child layout.ChildNode, horizontal bool, trackCount int) (int, int) {
	if trackCount <= 0 {
		return 0, 1
	}
	if horizontal {
		start := clampTrackIndex(child.Attachment.Placement.ColStart, trackCount)
		span := clampSpan(child.Attachment.Placement.ColSpan, trackCount-start)
		return start, span
	}
	start := clampTrackIndex(child.Attachment.Placement.RowStart, trackCount)
	span := clampSpan(child.Attachment.Placement.RowSpan, trackCount-start)
	return start, span
}

func alignInCell(child layout.ChildNode, align layout.Alignment, cell gfx.Rect) gfx.Rect {
	childW := child.IntrinsicSize.W
	childH := child.IntrinsicSize.H
	if align == layout.AlignStretch {
		return cell
	}
	if childW > cell.Width() {
		childW = cell.Width()
	}
	if childH > cell.Height() {
		childH = cell.Height()
	}
	x := cell.Min.X
	y := cell.Min.Y
	switch align {
	case layout.AlignCenter, layout.AlignTopCenter, layout.AlignBottomCenter:
		x += (cell.Width() - childW) / 2
	case layout.AlignEnd, layout.AlignTopRight, layout.AlignCenterRight, layout.AlignBottomRight:
		x += cell.Width() - childW
	}
	switch align {
	case layout.AlignCenter, layout.AlignCenterLeft, layout.AlignCenterRight:
		y += (cell.Height() - childH) / 2
	case layout.AlignEnd, layout.AlignBottomLeft, layout.AlignBottomCenter, layout.AlignBottomRight:
		y += cell.Height() - childH
	}
	return gfx.RectFromXYWH(x, y, childW, childH)
}

func sumTrackSizes(sizes []float32, gap float32) float32 {
	if len(sizes) == 0 {
		return 0
	}
	total := float32(0)
	for i := range sizes {
		total += sizes[i]
	}
	total += gapTotal(len(sizes), gap)
	return total
}

func prefixOffsets(sizes []float32, gap float32, buf *[16]float32) []float32 {
	offsets := buf[:len(sizes)]
	cur := float32(0)
	for i := range sizes {
		offsets[i] = cur
		cur += sizes[i] + gap
	}
	return offsets
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

func spanExtent(offsets, sizes []float32, gap float32, start, span int) (float32, float32) {
	if len(sizes) == 0 {
		return 0, 0
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
	pos := offsets[start]
	end := start + span
	size := float32(0)
	for i := start; i < end; i++ {
		size += sizes[i]
	}
	if span > 1 {
		size += gap * float32(span-1)
	}
	return pos, size
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

func gridTrackCount(defs []TrackDef) int {
	if len(defs) == 0 {
		return 1
	}
	return len(defs)
}

func clampTrackIndex(v, count int) int {
	if count <= 0 {
		return 0
	}
	if v <= 0 {
		return 0
	}
	if v-1 >= count {
		return count - 1
	}
	return v - 1
}

func clampSpan(v, remaining int) int {
	if v <= 0 {
		v = 1
	}
	if remaining <= 0 {
		return 1
	}
	if v > remaining {
		return remaining
	}
	return v
}

func axisAvailable(size gfx.Size, horizontal bool) float32 {
	if horizontal {
		return size.W
	}
	return size.H
}

func axisRectAvail(rect gfx.Rect, horizontal bool) float32 {
	if horizontal {
		return rect.Width()
	}
	return rect.Height()
}

func clamp(v, min, max float32) float32 {
	if v < min {
		v = min
	}
	if max > 0 && v > max {
		v = max
	}
	return v
}

func max(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}

func lessChild(children []layout.ChildNode, a, b int) bool {
	za := children[a].Attachment.ZPriority
	zb := children[b].Attachment.ZPriority
	if za == zb {
		return a < b
	}
	return za < zb
}

func insertionSortChildren(indices []int, children []layout.ChildNode) {
	for i := 1; i < len(indices); i++ {
		j := i
		for j > 0 && lessChild(children, indices[j], indices[j-1]) {
			indices[j], indices[j-1] = indices[j-1], indices[j]
			j--
		}
	}
}
