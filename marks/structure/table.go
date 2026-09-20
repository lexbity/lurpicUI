package structure

import (
	"fmt"
	"math"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/internal/mathutil"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/primitive"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/theme"
	shared "codeburg.org/lexbit/lurpicui/theme/recipes"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uistruct"
)

const (
	tableMarkIDRoot            facet.MarkID = 1
	tableMarkIDTableSurface    facet.MarkID = 2
	tableMarkIDHeaderRow       facet.MarkID = 3
	tableMarkIDHeaderCell      facet.MarkID = 4
	tableMarkIDBodyRows        facet.MarkID = 5
	tableMarkIDBodyCell        facet.MarkID = 6
	tableMarkIDSelectionColumn facet.MarkID = 7
	tableMarkIDSortIndicator   facet.MarkID = 8
	tableMarkIDFocusRing       facet.MarkID = 9
)

// TableColumn describes one table column.
type TableColumn struct {
	Key            string
	Label          string
	Width          float32
	Align          facet.Alignment
	Sortable       bool
	SortDescending bool
}

// TableRow describes one table row.
type TableRow struct {
	Key      string
	Cells    []string
	Disabled bool
}

// TableData describes a complete table snapshot.
type TableData struct {
	Columns        []TableColumn
	Rows           []TableRow
	SortColumnKey  string
	SortDescending bool
}

type tableChildSpec struct {
	Facet     facet.FacetImpl
	MarkID    facet.MarkID
	Placement facet.Placement
	ZOrder    int32
	Key       string
}

// Table implements the structure.table canonical mark.
type Table struct {
	marks.Core

	Activated signal.Signal[int]
	Scrolled  signal.Signal[gfx.Point]

	Label     marks.Binding[string]
	Disabled  marks.Binding[bool]
	Data      *store.ValueStore[TableData]
	Selection *store.ValueStore[string]

	textRole facet.TextRole

	hoveredRowIndex  int
	pressedRowIndex  int
	focusedRowIndex  int
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool
	draggingAxis     ScrollDirection

	scrollOffset gfx.Point

	cachedTokens          theme.Tokens
	cachedRecipe          shared.TableSlots
	cachedBounds          gfx.Rect
	cachedViewportBounds  gfx.Rect
	cachedContentBounds   gfx.Rect
	cachedVerticalTrack   gfx.Rect
	cachedVerticalThumb   gfx.Rect
	cachedHorizontalTrack gfx.Rect
	cachedHorizontalThumb gfx.Rect
	cachedFocusBounds     gfx.Rect
	cachedRowBounds       map[string]gfx.Rect
	cachedColumnBounds    map[string]gfx.Rect
	cachedCellBounds      map[facet.FacetID]gfx.Rect
	cachedChildOrder      []facet.FacetID
	cachedChildSpecs      []tableChildSpec
	cachedHeaderCells     map[string]*primitive.Text
	cachedBodyCells       map[string]map[string]*primitive.Text
	cachedSelectionCells  map[string]*primitive.Text
	cachedSortIndicators  map[string]*primitive.Text
	cachedColumnKeys      []string
	cachedRowKeys         []string
	cachedAllRows         []TableRow
	// cachedWindowRows / cachedRowWindowStart are the row-virtualization window
	// (RX-1 FR-7): the built/measured/arranged/projected body rows and the
	// absolute index of the first one. Rows outside the window are not built.
	cachedWindowRows     []TableRow
	cachedRowWindowStart int
	// cachedBodyRowHeight / cachedHeaderHeight are the fixed row heights from
	// typography metrics (measured once per font/scale), which make the window
	// computable from the scroll offset (FR-7).
	cachedBodyRowHeight       float32
	cachedHeaderHeight        float32
	cachedShowSelectionColumn bool
	cachedColumnWidths        []float32
	cachedRowHeights          []float32
	cachedWritingDirection    facet.WritingDirection
}

var _ facet.FacetImpl = (*Table)(nil)
var _ layout.AnchorExporter = (*Table)(nil)
var _ marks.Mark = (*Table)(nil)

// NewTable constructs a structure.table mark with canonical defaults.
// The selection store is supplied by the caller — the mark never creates its own.
func NewTable(label string, data TableData, selection *store.ValueStore[string]) *Table {
	t := &Table{
		Label:                marks.Const(label),
		Disabled:             marks.Const(false),
		Data:                 store.NewValueStore(cloneTableData(data)),
		Selection:            selection,
		focusedRowIndex:      -1,
		pressedRowIndex:      -1,
		hoveredRowIndex:      -1,
		cachedRowBounds:      make(map[string]gfx.Rect),
		cachedColumnBounds:   make(map[string]gfx.Rect),
		cachedCellBounds:     make(map[facet.FacetID]gfx.Rect),
		cachedHeaderCells:    make(map[string]*primitive.Text),
		cachedBodyCells:      make(map[string]map[string]*primitive.Text),
		cachedSelectionCells: make(map[string]*primitive.Text),
		cachedSortIndicators: make(map[string]*primitive.Text),
		Scrolled:             signal.NewSignal[gfx.Point]("table_scrolled"),
	}
	t.Facet = facet.NewFacet()
	t.AddBinding(t.Label)
	t.AddBinding(t.Disabled)

	t.Layout.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutGrid,
		Policy:   tableGroupPolicy{table: t},
		Children: t,
		Overflow: facet.OverflowScroll,
		Clipping: facet.GroupClipBounds,
	}
	t.Layout.Child = facet.GroupChildContract{
		SupportedPlacement: facet.SupportsGrid,
		Intrinsic: func(ctx facet.MeasureContext, constraints facet.Constraints) facet.IntrinsicSize {
			size := t.measure(ctx, constraints).Size
			return facet.IntrinsicSize{Min: size, Preferred: size, Max: size}
		},
		Constraints: facet.ConstraintPolicy{
			BelowMinWidth:  facet.CompressionClip,
			BelowMinHeight: facet.CompressionClip,
			AboveMaxWidth:  facet.ExpansionClip,
			AboveMaxHeight: facet.ExpansionClip,
		},
		Stretch: facet.StretchPolicy{
			Width:  facet.StretchWhenParentRequests,
			Height: facet.StretchWhenParentRequests,
		},
		Baseline: facet.BaselineNone,
	}
	t.Layout.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.Layout.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.Layout.ArrangedBounds = bounds
		t.arrange(ctx, bounds)
	}
	t.Render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := t.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	t.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return t.buildCommands(t.Layout.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
	}
	t.Hit.OnHitTest = func(p gfx.Point) facet.HitResult { return t.hitTest(p) }
	t.Input.OnPointer = func(e facet.PointerEvent) bool { return t.onPointer(e) }
	t.Input.OnScroll = func(e facet.ScrollEvent) bool { return t.onScroll(e) }
	t.Input.OnKey = func(e facet.KeyEvent) bool { return t.onKey(e) }
	t.Focus.Focusable = func() bool { return !t.Disabled.Get() && len(t.cachedRowKeys) > 0 }
	t.Focus.TabIndex = 0
	t.Focus.OnFocusGained = func() { t.onFocusGained() }
	t.Focus.OnFocusLost = func() { t.onFocusLost() }
	t.Viewport.Transform = gfx.Identity()
	t.textRole.IMEEnabled = false
	t.RegisterRoles()
	t.AddRole(&t.textRole)
	if t.Data != nil {
		t.Data.OnChange.Subscribe(func(_ signal.Change[TableData]) {
			t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		})
	}
	t.syncChildren()
	return t
}

// Base satisfies facet.FacetImpl.
func (t *Table) Base() *facet.Facet {
	t.BindImpl(t)
	return &t.Facet
}

// Descriptor satisfies marks.Mark.
func (t *Table) Descriptor() marks.Descriptor {
	return marks.Descriptor{Family: "structure", TypeName: "table"}
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *Table) AccessibilityRole() string { return "table" }

// AccessibleName reports the semantic name source required by the spec.
func (t *Table) AccessibleName() string { return strings.TrimSpace(t.Label.Get()) }

// Focusable reports whether the table can receive keyboard focus.
func (t *Table) Focusable() bool {
	if t.Focus.Focusable == nil {
		return false
	}
	return t.Focus.Focusable()
}

// Children returns the immediate child list.
func (t *Table) Children() []facet.GroupChild {
	if t == nil {
		return nil
	}
	if t.Data == nil {
		return nil
	}
	out := make([]facet.GroupChild, 0, len(t.cachedChildSpecs))
	for i := range t.cachedChildSpecs {
		child := t.groupChild(t.cachedChildSpecs[i])
		if child.Layout != nil {
			out = append(out, child)
		}
	}
	return out
}

// ExportAnchors publishes the table anchor set.
func (t *Table) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	if t == nil {
		return nil
	}
	bounds := t.Layout.ArrangedBounds
	out := t.DefaultAnchors(bounds, ctx)
	if out == nil {
		return nil
	}
	out["baseline"] = bounds.Min
	if !t.cachedViewportBounds.IsEmpty() {
		out["viewport"] = rectCenter(t.cachedViewportBounds)
	}
	if !t.cachedContentBounds.IsEmpty() {
		out["content"] = rectCenter(t.cachedContentBounds)
	}
	return out
}

func (t *Table) OnAttach(ctx facet.AttachContext) {
	t.Core.OnAttach(ctx)
	if t.Selection != nil {
		facet.Store(facet.Subscribe(t), &t.Selection.OnChange, t.Selection.Version, func(signal.Change[string]) {
			t.InvalidateWithSource(facet.DirtyLayout|facet.DirtyProjection|facet.DirtyHit, "table.Selection")
		})
	}
}
func (t *Table) OnActivate()   { t.Core.OnActivate() }
func (t *Table) OnDeactivate() { t.Core.OnDeactivate() }

// OnDetach clears cached projection state.
func (t *Table) OnDetach() {
	t.Core.OnDetach()
	t.cachedTokens = theme.Tokens{}
	t.cachedRecipe = shared.TableSlots{}
	t.cachedBounds = gfx.Rect{}
	t.cachedViewportBounds = gfx.Rect{}
	t.cachedContentBounds = gfx.Rect{}
	t.cachedVerticalTrack = gfx.Rect{}
	t.cachedVerticalThumb = gfx.Rect{}
	t.cachedHorizontalTrack = gfx.Rect{}
	t.cachedHorizontalThumb = gfx.Rect{}
	t.cachedFocusBounds = gfx.Rect{}
	t.cachedRowBounds = nil
	t.cachedColumnBounds = nil
	t.cachedCellBounds = nil
	t.cachedChildOrder = nil
	t.cachedChildSpecs = nil
	t.cachedHeaderCells = nil
	t.cachedBodyCells = nil
	t.cachedSelectionCells = nil
	t.cachedSortIndicators = nil
	t.cachedColumnKeys = nil
	t.cachedRowKeys = nil
	t.cachedAllRows = nil
	t.cachedWindowRows = nil
	t.cachedRowWindowStart = 0
	t.cachedBodyRowHeight = 0
	t.cachedHeaderHeight = 0
	t.cachedShowSelectionColumn = false
	t.cachedColumnWidths = nil
	t.cachedRowHeights = nil
	t.scrollOffset = gfx.Point{}
	t.dragging = false
	t.draggingAxis = ScrollDirectionVertical
}

func (t *Table) invalidate(flags facet.DirtyFlags) {
	if t == nil {
		return
	}
	t.Invalidate(flags)
}

func (t *Table) data() TableData {
	if t == nil || t.Data == nil {
		return TableData{}
	}
	return cloneTableData(t.Data.Get())
}

func (t *Table) syncChildren() {
	if t == nil {
		return
	}
	if t.Data == nil {
		t.Data = store.NewValueStore(TableData{})
	}
	data := t.data()
	allRows := sortedTableRows(data)
	t.cachedAllRows = allRows
	if t.focusedRowIndex < 0 && len(allRows) > 0 {
		t.focusedRowIndex = 0
	}
	if len(allRows) == 0 {
		t.focusedRowIndex = -1
	}
	if t.focusedRowIndex >= len(allRows) {
		t.focusedRowIndex = len(allRows) - 1
	}
	// Row virtualization (FR-7): only the visible window + overscan is built,
	// measured, arranged, and projected. Rows outside the window are skipped.
	windowStart, windowEnd := t.visibleRowWindow(len(allRows))
	t.cachedRowWindowStart = windowStart
	var windowRows []TableRow
	if len(allRows) > 0 {
		windowRows = allRows[windowStart : windowEnd+1]
	}
	t.cachedWindowRows = windowRows

	headerCells := make(map[string]*primitive.Text, len(data.Columns))
	bodyCells := make(map[string]map[string]*primitive.Text, len(windowRows))
	selectionCells := make(map[string]*primitive.Text, len(windowRows)+1)
	sortIndicators := make(map[string]*primitive.Text)
	hasSelection := t.Selection != nil && t.Selection.Get() != ""
	showSelectionColumn := hasSelection
	selectionOffset := 0
	if showSelectionColumn {
		selectionOffset = 1
	}
	childSpecs := make([]tableChildSpec, 0, len(data.Columns)+len(windowRows)*2+1)
	columnKeys := make([]string, 0, len(data.Columns))
	rowKeys := make([]string, 0, len(windowRows))
	if showSelectionColumn {
		headerKey := "__selection__"
		header := t.cachedSelectionCells[headerKey]
		if header == nil {
			header = primitive.NewText(marks.Const(""))
		} else {
			header.Content = marks.Const("")
			header.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		}
		header.Typography = marks.Const(theme.TextLabelM)
		header.Foreground = marks.Const(theme.ColorTextSecondary)
		header.Overflow = marks.Const(primitive.TextOverflowClip)
		selectionCells[headerKey] = header
		childSpecs = append(childSpecs, tableChildSpec{
			Facet:     header,
			MarkID:    tableMarkIDSelectionColumn,
			Placement: tableSelectionPlacement(0),
			Key:       "selection:header",
		})
	}

	for colIndex := range data.Columns {
		col := data.Columns[colIndex]
		key := stableTableKey(col.Key, col.Label, colIndex)
		columnKeys = append(columnKeys, key)
		label := strings.TrimSpace(col.Label)
		if label == "" {
			label = key
		}
		cell := t.cachedHeaderCells[key]
		if cell == nil {
			cell = primitive.NewText(marks.Const(label))
		} else {
			cell.Content = marks.Const(label)
			cell.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		}
		cell.Typography = marks.Const(theme.TextLabelM)
		cell.Foreground = marks.Const(theme.ColorTextSecondary)
		cell.Overflow = marks.Const(primitive.TextOverflowTruncate)
		headerCells[key] = cell
		childSpecs = append(childSpecs, tableChildSpec{
			Facet:     cell,
			MarkID:    tableMarkIDHeaderCell,
			Placement: tableCellPlacement(colIndex+selectionOffset, 0, col.Align),
			Key:       "header:" + key,
		})
		if data.SortColumnKey == key {
			indicator := t.cachedSortIndicators[key]
			if indicator == nil {
				indicator = primitive.NewText(marks.Const("▼"))
				if data.SortDescending {
					indicator.Content = marks.Const("▼")
					indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				} else {
					indicator.Content = marks.Const("▲")
					indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				}
			} else {
				if data.SortDescending {
					indicator.Content = marks.Const("▼")
					indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				} else {
					indicator.Content = marks.Const("▲")
					indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				}
			}
			indicator.Typography = marks.Const(theme.TextLabelM)
			indicator.Foreground = marks.Const(theme.ColorPrimary)
			indicator.Overflow = marks.Const(primitive.TextOverflowClip)
			sortIndicators[key] = indicator
			childSpecs = append(childSpecs, tableChildSpec{
				Facet:     indicator,
				MarkID:    tableMarkIDSortIndicator,
				Placement: tableSortIndicatorPlacement(colIndex+selectionOffset, 0),
				ZOrder:    1,
				Key:       "sort:" + key,
			})
		}
	}

	for windowIndex := range windowRows {
		row := windowRows[windowIndex]
		rowIndex := windowStart + windowIndex
		key := stableTableKey(row.Key, "", rowIndex)
		rowKeys = append(rowKeys, key)
		rowCells := t.cachedBodyCells[key]
		if rowCells == nil {
			rowCells = make(map[string]*primitive.Text, len(data.Columns))
		}
		if showSelectionColumn {
			selectionKey := stableTableKey(row.Key, "", rowIndex)
			indicator := t.cachedSelectionCells[selectionKey]
			if indicator == nil {
				indicator = primitive.NewText(marks.Const(""))
			}
			indicator.Typography = marks.Const(theme.TextLabelM)
			if t.isRowSelected(key) {
				indicator.Content = marks.Const("✓")
				indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				indicator.Foreground = marks.Const(theme.ColorPrimary)
			} else {
				indicator.Content = marks.Const("")
				indicator.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
				indicator.Foreground = marks.Const(theme.ColorTextSecondary)
			}
			indicator.Overflow = marks.Const(primitive.TextOverflowClip)
			if row.Disabled || t.Disabled.Get() {
				indicator.Disabled = marks.Const(true)
			} else {
				indicator.Disabled = marks.Const(false)
			}
			selectionCells[selectionKey] = indicator
			childSpecs = append(childSpecs, tableChildSpec{
				Facet:     indicator,
				MarkID:    tableMarkIDSelectionColumn,
				Placement: tableSelectionPlacement(rowIndex + 1),
				Key:       "selection:" + selectionKey,
			})
		}
		for colIndex := range data.Columns {
			col := data.Columns[colIndex]
			cellKey := stableTableCellKey(key, stableTableKey(col.Key, col.Label, colIndex))
			content := ""
			if colIndex < len(row.Cells) {
				content = row.Cells[colIndex]
			}
			cell := rowCells[stableTableKey(col.Key, col.Label, colIndex)]
			if cell == nil {
				cell = primitive.NewText(marks.Const(content))
			} else {
				cell.Content = marks.Const(content)
				cell.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
			}
			cell.Typography = marks.Const(theme.TextBodyM)
			cell.Foreground = marks.Const(theme.ColorText)
			cell.Overflow = marks.Const(primitive.TextOverflowTruncate)
			if t.isRowSelected(key) {
				cell.Foreground = marks.Const(theme.ColorPrimary)
			}
			if row.Disabled || t.Disabled.Get() {
				cell.Disabled = marks.Const(true)
			} else {
				cell.Disabled = marks.Const(false)
			}
			rowCells[stableTableKey(col.Key, col.Label, colIndex)] = cell
			childSpecs = append(childSpecs, tableChildSpec{
				Facet:     cell,
				MarkID:    tableMarkIDBodyCell,
				Placement: tableCellPlacement(colIndex+selectionOffset, rowIndex+1, col.Align),
				Key:       "body:" + cellKey,
			})
		}
		bodyCells[key] = rowCells
	}

	t.cachedHeaderCells = headerCells
	t.cachedBodyCells = bodyCells
	t.cachedSelectionCells = selectionCells
	t.cachedSortIndicators = sortIndicators
	t.cachedChildSpecs = childSpecs
	t.cachedColumnKeys = columnKeys
	t.cachedRowKeys = rowKeys
	t.cachedShowSelectionColumn = showSelectionColumn
}

// visibleRowWindow returns the half-open row-window indices [start, end] that
// the table builds (RX-1 FR-7): the viewport's rows plus one overscan viewport
// above and below, clamped to the row count. Before the first arrange (no
// viewport yet) a small leading window is built so the runtime can measure.
func (t *Table) visibleRowWindow(rowCount int) (int, int) {
	if rowCount <= 0 {
		return 0, 0
	}
	if t.cachedViewportBounds.IsEmpty() || t.cachedBodyRowHeight <= 0 {
		end := mathutil.Min(8, rowCount-1)
		return 0, end
	}
	viewportH := t.cachedViewportBounds.Height()
	first := int(t.scrollOffset.Y / t.cachedBodyRowHeight)
	if first < 0 {
		first = 0
	}
	if first >= rowCount {
		first = rowCount - 1
	}
	visible := int(math.Ceil(float64(viewportH)/float64(t.cachedBodyRowHeight))) + 1
	overscan := visible
	start := first - overscan
	if start < 0 {
		start = 0
	}
	end := first + visible + overscan
	if end >= rowCount {
		end = rowCount - 1
	}
	return start, end
}

// VisibleRange returns the built row window as inclusive absolute body-row
// indices [first, last], or (-1, -1) when the table has no rows. It is the
// FR-7 accessor for virtualization tests.
func (t *Table) VisibleRange() (int, int) {
	if t == nil || len(t.cachedAllRows) == 0 {
		return -1, -1
	}
	start := t.cachedRowWindowStart
	end := start + len(t.cachedWindowRows) - 1
	return start, end
}

// contentMetrics computes the table's fixed row heights, stable column widths,
// and full content bounds from typography metrics (measured once per
// font/scale, RX-1 FR-7) plus a longest-cell scan for intrinsic columns. It
// must run before syncChildren/arrange so the virtualization window and the
// fixed-row grid policy have their constants.
func (t *Table) contentMetrics(ctx facet.MeasureContext, data TableData) {
	hasSelection := t.Selection != nil && t.Selection.Get() != ""
	selectionCol := hasSelection
	gap := mathutil.Max(t.gridGap(), 8)

	headerSample := primitive.NewText(marks.Const("M"))
	headerSample.Typography = marks.Const(theme.TextLabelM)
	headerSample.Overflow = marks.Const(primitive.TextOverflowTruncate)
	_ = headerSample.Layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 0}})
	t.cachedHeaderHeight = headerSample.Layout.MeasuredSize.H

	bodySample := primitive.NewText(marks.Const("M"))
	bodySample.Typography = marks.Const(theme.TextBodyM)
	bodySample.Overflow = marks.Const(primitive.TextOverflowTruncate)
	_ = bodySample.Layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 0}})
	t.cachedBodyRowHeight = bodySample.Layout.MeasuredSize.H

	widths := make([]float32, len(data.Columns))
	for i := range data.Columns {
		col := data.Columns[i]
		if col.Width > 0 {
			widths[i] = col.Width
			continue
		}
		longest := col.Label
		for _, row := range data.Rows {
			if i < len(row.Cells) && len(row.Cells[i]) > len(longest) {
				longest = row.Cells[i]
			}
		}
		sample := primitive.NewText(marks.Const(longest))
		sample.Typography = marks.Const(theme.TextBodyM)
		sample.Overflow = marks.Const(primitive.TextOverflowTruncate)
		_ = sample.Layout.Measure(ctx, facet.Constraints{MaxSize: gfx.Size{W: 640, H: 0}})
		widths[i] = mathutil.Max(24, sample.Layout.MeasuredSize.W)
	}
	t.cachedColumnWidths = widths

	colW := float32(0)
	for i, w := range widths {
		colW += w
		if i < len(widths)-1 {
			colW += gap
		}
	}
	if selectionCol {
		colW += t.selectionColumnWidth() + gap
	}
	rowCount := len(data.Rows)
	bodyH := float32(0)
	if rowCount > 0 {
		bodyH = float32(rowCount) * (t.cachedBodyRowHeight + gap)
	}
	t.cachedContentBounds = gfx.RectFromXYWH(0, 0, colW, t.cachedHeaderHeight+bodyH)
}

func (t *Table) buildGridPolicy(data TableData) *layoutgrid.Policy {
	selectionOffset := 0
	if t.cachedShowSelectionColumn {
		selectionOffset = 1
	}
	columns := make([]layoutgrid.TrackDef, len(data.Columns)+selectionOffset)
	if t.cachedShowSelectionColumn {
		columns[0] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: t.selectionColumnWidth()}
	}
	for i := range data.Columns {
		col := data.Columns[i]
		if i < len(t.cachedColumnWidths) && t.cachedColumnWidths[i] > 0 {
			columns[i+selectionOffset] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: t.cachedColumnWidths[i], Min: 24}
		} else if col.Width > 0 {
			columns[i+selectionOffset] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: col.Width, Min: 24}
		} else {
			columns[i+selectionOffset] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackIntrinsic, Min: 24}
		}
	}
	// Fixed row heights (FR-7): the header and every body row use their
	// typography-derived height, which makes the content height deterministic
	// and the virtualization window computable from the scroll offset.
	rows := make([]layoutgrid.TrackDef, len(data.Rows)+1)
	rows[0] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: mathutil.Max(24, t.cachedHeaderHeight)}
	for i := 1; i < len(rows); i++ {
		rows[i] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: t.cachedBodyRowHeight}
	}
	colGap := mathutil.Max(t.gridGap(), 8)
	return layoutgrid.New(layoutgrid.Config{
		Columns:       columns,
		Rows:          rows,
		ColumnGap:     colGap,
		RowGap:        colGap,
		AutoPlacement: layoutgrid.AutoRowFirst,
	})
}

func (t *Table) measure(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
	resolved, ok := ctx.Theme.(theme.ResolvedContext)
	if !ok {
		resolved = theme.DefaultResolvedContext()
	}
	style := theme.StyleContext{Tokens: resolved.TokenSet(), Materials: resolved.Materials, Depth: resolved.Depth}
	slots, _ := uistruct.ResolveTableRecipe(style)
	t.cachedTokens = resolved.TokenSet()
	t.cachedRecipe = slots
	t.cachedWritingDirection = ctx.WritingDirection
	data := t.data()
	t.contentMetrics(ctx, data)
	t.syncChildren()
	children := t.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{})
		t.Layout.MeasuredSize = size
		t.Layout.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return t.Layout.MeasuredResult
	}
	childMeasureCtx := facet.MeasureContext{
		Runtime:          ctx.Runtime,
		Theme:            ctx.Theme,
		ContentScale:     ctx.ContentScale,
		Density:          ctx.Density,
		WritingDirection: ctx.WritingDirection,
	}
	for i := range children {
		if children[i].Layout == nil {
			continue
		}
		_ = children[i].Layout.Measure(childMeasureCtx, constraints)
	}
	// The measured size is the full content (all rows), not the built window:
	// virtualization must not shrink the scrollable content extent (FR-7).
	size := gfx.Size{W: t.cachedContentBounds.Width(), H: t.cachedContentBounds.Height()}
	measured := constraints.Constrain(size)
	t.Layout.MeasuredSize = measured
	t.Layout.MeasuredResult = facet.MeasureResult{
		Size:        measured,
		Intrinsic:   facet.IntrinsicSize{Min: measured, Preferred: measured, Max: measured},
		Constraints: constraints,
	}
	return t.Layout.MeasuredResult
}

func tableFacetByID(t *Table, id facet.FacetID) *facet.Facet {
	for _, spec := range t.cachedChildSpecs {
		if spec.Facet != nil && spec.Facet.Base().ID() == id {
			return spec.Facet.Base()
		}
	}
	return nil
}

func (t *Table) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	if t.cachedBodyRowHeight <= 0 {
		t.contentMetrics(facet.MeasureContext{
			Runtime: ctx.Runtime,
			Theme:   ctx.Theme,
		}, t.data())
	}
	contentSize := t.cachedContentBounds
	t.cachedBounds = bounds
	t.cachedViewportBounds = bounds
	t.cachedVerticalTrack = gfx.Rect{}
	t.cachedVerticalThumb = gfx.Rect{}
	t.cachedHorizontalTrack = gfx.Rect{}
	t.cachedHorizontalThumb = gfx.Rect{}
	t.cachedFocusBounds = bounds.Inset(mathutil.Max(1, bounds.Width()*0.04), mathutil.Max(1, bounds.Height()*0.04))
	t.Layout.ArrangedBounds = bounds
	if bounds.IsEmpty() {
		return
	}
	t.syncChildren()
	children := t.Children()
	if len(children) == 0 {
		return
	}
	data := t.data()
	policy := t.buildGridPolicy(data)
	gridChildren := t.gridChildren(children)
	contentRect := gfx.RectFromXYWH(bounds.Min.X-t.scrollOffset.X, bounds.Min.Y-t.scrollOffset.Y, contentSize.Width(), contentSize.Height())
	arranged, err := policy.Arrange(gridChildren, contentRect)
	if err != nil {
		return
	}
	rtl := t.cachedWritingDirection == facet.WritingDirectionRTL
	childBounds := make(map[facet.FacetID]gfx.Rect, len(arranged))
	order := make([]facet.FacetID, 0, len(arranged))
	rowBounds := make(map[string]gfx.Rect, len(data.Rows)+1)
	columnBounds := make(map[string]gfx.Rect, len(data.Columns))
	cellBounds := make(map[facet.FacetID]gfx.Rect, len(arranged))
	for _, child := range arranged {
		b := child.Bounds
		if rtl {
			b.Min.X = contentRect.Max.X - (child.Bounds.Min.X - contentRect.Min.X) - child.Bounds.Width()
			b.Max.X = b.Min.X + child.Bounds.Width()
		}
		childBounds[child.FacetID] = b
		cellBounds[child.FacetID] = b
		if rtl {
			if tfl := tableFacetByID(t, child.FacetID); tfl != nil && tfl.LayoutRole() != nil {
				tfl.LayoutRole().Arrange(ctx, b)
			}
		}
		order = append(order, child.FacetID)
		if child.Placement.RowStart >= 0 && child.Placement.RowStart < len(t.cachedAllRows)+1 {
			rowKey := t.visibleTableRowKeyAtIndex(child.Placement.RowStart)
			rowBounds[rowKey] = rowBounds[rowKey].Union(child.Bounds)
		}
		if t.cachedShowSelectionColumn && child.Placement.ColStart == 0 {
			columnBounds["__selection__"] = columnBounds["__selection__"].Union(child.Bounds)
			continue
		}
		colStart := child.Placement.ColStart
		if t.cachedShowSelectionColumn {
			colStart--
		}
		if colStart >= 0 && colStart < len(data.Columns) {
			col := data.Columns[colStart]
			colKey := stableTableKey(col.Key, col.Label, colStart)
			columnBounds[colKey] = columnBounds[colKey].Union(child.Bounds)
		}
	}
	t.cachedChildOrder = order
	t.cachedCellBounds = cellBounds
	t.cachedRowBounds = rowBounds
	t.cachedColumnBounds = columnBounds
	t.cachedContentBounds = gfx.RectFromXYWH(contentRect.Min.X, contentRect.Min.Y, contentSize.Width(), contentSize.Height())
	t.updateScrollBounds(bounds)
	t.Viewport.WorldBounds = bounds
}

func (t *Table) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if t == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := t.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if t.Disabled.Get() {
		state = theme.StateDisabled
	}
	root := slots.Root.Resolve(state, tokens)
	surface := slots.TableSurface.Resolve(state, tokens)
	headerRow := slots.HeaderRow.Resolve(state, tokens)
	headerCell := slots.HeaderCell.Resolve(state, tokens)
	bodyRows := slots.BodyRows.Resolve(state, tokens)
	bodyCell := slots.BodyCell.Resolve(state, tokens)
	sortIndicator := slots.SortIndicator.Resolve(state, tokens)
	selectionColumn := slots.SelectionColumnOptional.Resolve(state, tokens)
	focus := slots.FocusRing.Resolve(theme.StateFocused, tokens)

	cmds := make([]gfx.Command, 0, 128)
	if !theme.IsTransparentMaterial(root) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), root)...)
	}
	if !theme.IsTransparentMaterial(surface) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), surface)...)
	}
	if !theme.IsTransparentMaterial(headerRow) && len(t.cachedRowKeys) > 0 {
		headerKey := tableRowKeyForIndex(t.data(), 0)
		if headerBounds := t.cachedRowBounds[headerKey]; !headerBounds.IsEmpty() {
			cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(headerBounds), headerRow)...)
		}
	}
	if !theme.IsTransparentMaterial(bodyRows) {
		// Only the built row window is arranged/projected (FR-7): iterate the
		// window's absolute indices so off-window rows draw no background.
		for windowIndex := range t.cachedWindowRows {
			rowIndex := t.cachedRowWindowStart + windowIndex
			rowKey := stableTableKey(t.cachedWindowRows[windowIndex].Key, "", rowIndex)
			if rowBounds := t.cachedRowBounds[rowKey]; !rowBounds.IsEmpty() {
				cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(rowBounds), bodyRows)...)
			}
		}
	}
	if !theme.IsTransparentMaterial(selectionColumn) {
		for _, spec := range t.cachedChildSpecs {
			if spec.MarkID != tableMarkIDSelectionColumn || spec.Facet == nil {
				continue
			}
			base := spec.Facet.Base()
			if base == nil || base.LayoutRole() == nil {
				continue
			}
			childBounds := t.cachedCellBounds[base.ID()]
			if childBounds.IsEmpty() {
				continue
			}
			cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(childBounds), selectionColumn)...)
		}
	}
	if !t.cachedViewportBounds.IsEmpty() {
		cmds = append(cmds, gfx.PushClipRect{Rect: t.cachedViewportBounds})
		for _, spec := range t.cachedChildSpecs {
			if spec.Facet == nil {
				continue
			}
			base := spec.Facet.Base()
			if base == nil || base.LayoutRole() == nil {
				continue
			}
			childBounds := t.cachedCellBounds[base.ID()]
			if childBounds.IsEmpty() {
				continue
			}
			switch spec.MarkID {
			case tableMarkIDHeaderCell:
				if !theme.IsTransparentMaterial(headerCell) {
					cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(childBounds), headerCell)...)
				}
			case tableMarkIDBodyCell:
				if !theme.IsTransparentMaterial(bodyCell) {
					cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(childBounds), bodyCell)...)
				}
			}
			if projected := base.ProjectionRole().Project(facet.ProjectionContext{
				Runtime:      runtimeServicesOrNil(runtime),
				Bounds:       childBounds,
				ContentScale: contentScale,
			}); projected != nil {
				cmds = append(cmds, projected.Commands...)
			}
		}
		cmds = append(cmds, gfx.PopClip{})
	}
	if !t.cachedVerticalTrack.IsEmpty() {
		cmds = append(cmds, t.barCommands(t.cachedVerticalTrack, sortIndicator, 0.24)...)
	}
	if !t.cachedVerticalThumb.IsEmpty() {
		cmds = append(cmds, t.barCommands(t.cachedVerticalThumb, sortIndicator, 1)...)
	}
	if !t.cachedHorizontalTrack.IsEmpty() {
		cmds = append(cmds, t.barCommands(t.cachedHorizontalTrack, sortIndicator, 0.24)...)
	}
	if !t.cachedHorizontalThumb.IsEmpty() {
		cmds = append(cmds, t.barCommands(t.cachedHorizontalThumb, sortIndicator, 1)...)
	}
	if t.focusedVisible && !theme.IsTransparentMaterial(focus) {
		cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(t.cachedFocusBounds), focus)...)
	}
	return cmds
}

func (t *Table) resolveProjectionTheme(runtime any) (theme.StyleContext, shared.TableSlots) {
	if runtime == nil {
		return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
	}
	type styleTree interface {
		RootStyleContext() any
		FacetByID(id facet.FacetID) facet.FacetImpl
	}
	if tree, ok := runtime.(styleTree); ok {
		if store := theme.NearestStyleContext(tree, t.Base().ID()); store != nil {
			style := store.Get()
			slots, _ := uistruct.ResolveTableRecipe(style)
			return style, slots
		}
	}
	return theme.StyleContext{Tokens: t.cachedTokens}, t.cachedRecipe
}

func (t *Table) hitTest(p gfx.Point) facet.HitResult {
	if t == nil || t.Layout.ArrangedBounds.IsEmpty() || !t.Layout.ArrangedBounds.Contains(p) {
		return facet.HitResult{}
	}
	cursor := t.cursorShape()
	if t.cachedVerticalThumb.Contains(p) || t.cachedVerticalTrack.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: tableMarkIDTableSurface, Cursor: facet.CursorGrab}
	}
	if t.cachedHorizontalThumb.Contains(p) || t.cachedHorizontalTrack.Contains(p) {
		return facet.HitResult{Hit: true, MarkID: tableMarkIDTableSurface, Cursor: facet.CursorGrab}
	}
	for _, spec := range t.cachedChildSpecs {
		if spec.Facet == nil {
			continue
		}
		if b := t.cachedCellBounds[spec.Facet.Base().ID()]; !b.IsEmpty() && b.Contains(p) {
			return facet.HitResult{Hit: true, MarkID: spec.MarkID, Cursor: cursor}
		}
	}
	return facet.HitResult{Hit: true, MarkID: tableMarkIDRoot, Cursor: cursor}
}

func (t *Table) onPointer(e facet.PointerEvent) bool {
	if t.Disabled.Get() {
		return false
	}
	switch e.Kind {
	case platform.PointerEnter:
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerLeave:
		t.hoveredRowIndex = -1
		t.focusFromPointer = false
		t.dragging = false
		t.invalidate(facet.DirtyProjection)
		return true
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return false
		}
		if t.cachedVerticalThumb.Contains(e.Position) {
			t.dragging = true
			t.draggingAxis = ScrollDirectionVertical
			t.updateOffsetFromDrag(e.Position)
			t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
			return true
		}
		if t.cachedHorizontalThumb.Contains(e.Position) {
			t.dragging = true
			t.draggingAxis = ScrollDirectionHorizontal
			t.updateOffsetFromDrag(e.Position)
			t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
			return true
		}
		if rowIndex := t.rowAtPoint(e.Position); rowIndex >= 0 {
			t.focusedRowIndex = rowIndex
			t.focusFromPointer = true
			t.focusedVisible = false
			t.selectRow(rowIndex)
			t.Activated.Emit(rowIndex)
			t.ensureFocusedRowVisible()
			t.invalidate(facet.DirtyProjection)
			return true
		}
		return true
	case platform.PointerMove:
		if t.dragging {
			t.updateOffsetFromDrag(e.Position)
			// Scrollbar drag changes the scroll offset; the virtualization
			// window re-lays (FR-3), like onScroll/onKey.
			t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
			return true
		}
		return false
	case platform.PointerRelease:
		if e.Button != platform.PointerLeft {
			return false
		}
		was := t.dragging
		t.dragging = false
		t.invalidate(facet.DirtyProjection)
		return was
	default:
		return false
	}
}

func (t *Table) onScroll(e facet.ScrollEvent) bool {
	if t.Disabled.Get() {
		return false
	}
	if e.DeltaX == 0 && e.DeltaY == 0 {
		return false
	}
	next := t.scrollOffset
	next.X -= e.DeltaX
	next.Y -= e.DeltaY
	t.scrollOffset = t.clampScrollOffset(next)
	t.Scrolled.Emit(t.scrollOffset)
	// The offset change re-lays the table: the FR-7 virtualization window and
	// the arranged content positions are recomputed at arrange time, so a
	// scroll MUST route a layout pass (RX-1 FR-3) — projection-only
	// invalidation would leave the window frozen at the pre-scroll rows.
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
	return true
}

func (t *Table) onKey(e facet.KeyEvent) bool {
	if t.Disabled.Get() {
		return false
	}
	if e.Kind != platform.KeyPress && e.Kind != platform.KeyRepeat {
		return false
	}
	step := t.keyboardStep()
	switch e.Key {
	case platform.KeyUp:
		t.moveFocus(-1)
	case platform.KeyDown:
		t.moveFocus(1)
	case platform.KeyLeft:
		t.scrollOffset.X -= step
	case platform.KeyRight:
		t.scrollOffset.X += step
	case platform.KeyPageUp:
		t.moveFocus(-t.pageStep())
	case platform.KeyPageDown:
		t.moveFocus(t.pageStep())
	case platform.KeyHome:
		t.focusedRowIndex = 0
		t.ensureFocusedRowVisible()
	case platform.KeyEnd:
		rows := t.cachedAllRows
		if len(rows) == 0 {
			rows = sortedTableRows(t.data())
		}
		if len(rows) > 0 {
			t.focusedRowIndex = len(rows) - 1
			t.ensureFocusedRowVisible()
		}
	case platform.KeyEnter, platform.KeySpace:
		if t.focusedRowIndex >= 0 {
			t.selectRow(t.focusedRowIndex)
			t.Activated.Emit(t.focusedRowIndex)
		}
	default:
		return false
	}
	t.scrollOffset = t.clampScrollOffset(t.scrollOffset)
	t.Scrolled.Emit(t.scrollOffset)
	t.invalidate(facet.DirtyLayout | facet.DirtyProjection)
	return true
}

func (t *Table) onFocusGained() {
	t.focusedVisible = !t.focusFromPointer
	t.focusFromPointer = false
	t.invalidate(facet.DirtyProjection)
}

func (t *Table) onFocusLost() {
	t.focusedVisible = false
	t.focusFromPointer = false
	t.dragging = false
	t.invalidate(facet.DirtyProjection)
}

func (t *Table) gridChildren(children []facet.GroupChild) []layoutgrid.Child {
	out := make([]layoutgrid.Child, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		if !child.Contract.SupportedPlacement.Has(facet.PlacementGrid) {
			continue
		}
		out = append(out, layoutgrid.Child{
			FacetID:    child.FacetID,
			Attachment: child.Attachment,
			Layout:     child.Layout,
			Contract:   child.Contract,
		})
	}
	return out
}

func (t *Table) groupChild(spec tableChildSpec) facet.GroupChild {
	if spec.Facet == nil {
		return facet.GroupChild{}
	}
	base := spec.Facet.Base()
	if base == nil || base.LayoutRole() == nil {
		return facet.GroupChild{}
	}
	return facet.GroupChild{
		FacetID: base.ID(),
		MarkID:  spec.MarkID,
		Attachment: facet.Attachment{
			Placement: spec.Placement,
			ZOrder:    spec.ZOrder,
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func (t *Table) scrollViewport() ScrollViewport {
	return ScrollViewport{
		Content: gfx.Size{W: t.cachedContentBounds.Width(), H: t.cachedContentBounds.Height()},
		View:    t.cachedViewportBounds,
		Track:   t.trackThickness(),
	}
}

func (t *Table) updateScrollBounds(bounds gfx.Rect) {
	t.cachedViewportBounds = bounds
	vp := ScrollViewport{
		Content: gfx.Size{W: t.cachedContentBounds.Width(), H: t.cachedContentBounds.Height()},
		View:    bounds,
		Track:   t.trackThickness(),
	}
	t.scrollOffset = vp.Clamp(t.scrollOffset)
	t.Scrolled.Emit(t.scrollOffset)
	t.cachedVerticalTrack, t.cachedVerticalThumb = vp.Vertical(t.scrollOffset)
	t.cachedHorizontalTrack, t.cachedHorizontalThumb = vp.Horizontal(t.scrollOffset)
}

func (t *Table) updateOffsetFromDrag(p gfx.Point) {
	if t == nil {
		return
	}
	vp := t.scrollViewport()
	switch t.draggingAxis {
	case ScrollDirectionHorizontal:
		t.scrollOffset = vp.DragTo(t.scrollOffset, true, p, t.cachedHorizontalTrack, t.cachedHorizontalThumb)
	default:
		t.scrollOffset = vp.DragTo(t.scrollOffset, false, p, t.cachedVerticalTrack, t.cachedVerticalThumb)
	}
	t.scrollOffset = vp.Clamp(t.scrollOffset)
}

func (t *Table) rowAtPoint(p gfx.Point) int {
	if t == nil {
		return -1
	}
	rows := t.cachedAllRows
	if len(rows) == 0 {
		rows = sortedTableRows(t.data())
	}
	for i := range rows {
		key := stableTableKey(rows[i].Key, "", i)
		if b := t.cachedRowBounds[key]; !b.IsEmpty() && b.Contains(p) {
			return i
		}
	}
	return -1
}

func (t *Table) isRowSelected(key string) bool {
	return t.Selection != nil && t.Selection.Get() == key
}

func (t *Table) selectRow(index int) {
	if t == nil || t.Selection == nil {
		return
	}
	visible := t.cachedAllRows
	if len(visible) == 0 {
		data := t.data()
		visible = sortedTableRows(data)
	}
	if index < 0 || index >= len(visible) {
		return
	}
	t.Selection.Set(visible[index].Key)
}

func (t *Table) ensureFocusedRowVisible() {
	if t == nil {
		return
	}
	if t.focusedRowIndex < 0 {
		return
	}
	data := t.data()
	if t.focusedRowIndex >= len(data.Rows) {
		return
	}
	rowKey := t.visibleRowKeyAtIndex(t.focusedRowIndex)
	rowBounds := t.cachedRowBounds[rowKey]
	if rowBounds.IsEmpty() {
		return
	}
	if rowBounds.Min.Y < t.cachedViewportBounds.Min.Y {
		t.scrollOffset.Y -= t.cachedViewportBounds.Min.Y - rowBounds.Min.Y
	}
	if rowBounds.Max.Y > t.cachedViewportBounds.Max.Y {
		t.scrollOffset.Y += rowBounds.Max.Y - t.cachedViewportBounds.Max.Y
	}
	t.scrollOffset = t.clampScrollOffset(t.scrollOffset)
	t.Scrolled.Emit(t.scrollOffset)
}

func (t *Table) moveFocus(delta int) {
	if t == nil {
		return
	}
	rows := t.cachedAllRows
	if len(rows) == 0 {
		rows = sortedTableRows(t.data())
	}
	if len(rows) == 0 {
		return
	}
	if t.focusedRowIndex < 0 {
		t.focusedRowIndex = 0
	} else {
		t.focusedRowIndex += delta
	}
	if t.focusedRowIndex < 0 {
		t.focusedRowIndex = 0
	}
	if t.focusedRowIndex >= len(rows) {
		t.focusedRowIndex = len(rows) - 1
	}
	t.ensureFocusedRowVisible()
}

func (t *Table) pageStep() int {
	span := t.cachedViewportBounds.Height()
	if span <= 0 {
		return 1
	}
	rows := t.cachedAllRows
	if len(rows) == 0 {
		rows = sortedTableRows(t.data())
	}
	if len(rows) == 0 {
		return 1
	}
	return max(1, int(span/mathutil.Max(1, t.gridGap()*2+18)))
}

func (t *Table) trackThickness() float32 {
	if t.cachedTokens.Spacing.TouchTarget > 0 {
		return mathutil.Max(8, t.cachedTokens.Spacing.TouchTarget*0.12)
	}
	return 8
}

func (t *Table) gridGap() float32 {
	if t.cachedTokens.Spacing.XS > 0 {
		return mathutil.Max(4, t.cachedTokens.Spacing.XS)
	}
	return 4
}

func (t *Table) selectionColumnWidth() float32 {
	if t.cachedTokens.Spacing.TouchTarget > 0 {
		return mathutil.Max(24, t.cachedTokens.Spacing.TouchTarget*0.45)
	}
	return 24
}

func (t *Table) keyboardStep() float32 {
	span := t.cachedViewportBounds.Height()
	if span <= 0 {
		return 24
	}
	return mathutil.Max(24, span*0.12)
}

func (t *Table) visibleRowKeyAtIndex(index int) string {
	if t == nil || index < 0 {
		return ""
	}
	if index >= len(t.cachedAllRows) {
		return ""
	}
	return stableTableKey(t.cachedAllRows[index].Key, "", index)
}

func (t *Table) visibleTableRowKeyAtIndex(index int) string {
	if index == 0 {
		return "__header__"
	}
	return t.visibleRowKeyAtIndex(index - 1)
}

func (t *Table) clampScrollOffset(next gfx.Point) gfx.Point {
	return t.scrollViewport().Clamp(next)
}

func (t *Table) cursorShape() facet.CursorShape {
	if t.Disabled.Get() {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (t *Table) barCommands(bounds gfx.Rect, material theme.Material, opacity float32) []gfx.Command {
	if bounds.IsEmpty() || theme.IsTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, 4)
	if opacity > 0 && opacity < 1 {
		cmds = append(cmds, gfx.PushOpacity{Alpha: opacity})
	}
	cmds = append(cmds, theme.MaterialCommands(gfx.RectPath(bounds), material)...)
	if opacity > 0 && opacity < 1 {
		cmds = append(cmds, gfx.PopOpacity{})
	}
	return cmds
}

func tableRowKeyForIndex(data TableData, index int) string {
	if index < 0 {
		return ""
	}
	if index == 0 {
		return "__header__"
	}
	rowIndex := index - 1
	if rowIndex < 0 || rowIndex >= len(data.Rows) {
		return ""
	}
	return stableTableKey(data.Rows[rowIndex].Key, "", rowIndex)
}

func tableCellPlacement(colStart, rowStart int, align facet.Alignment) facet.Placement {
	if align == 0 {
		align = facet.AlignStretch
	}
	return facet.Placement{
		Mode: facet.PlacementGrid,
		Grid: facet.GridPlacement{
			ColStart: colStart,
			RowStart: rowStart,
			ColSpan:  1,
			RowSpan:  1,
		},
		Align: align,
	}
}

func tableSelectionPlacement(rowStart int) facet.Placement {
	return facet.Placement{
		Mode: facet.PlacementGrid,
		Grid: facet.GridPlacement{
			ColStart: 0,
			RowStart: rowStart,
			ColSpan:  1,
			RowSpan:  1,
		},
		Align: facet.AlignCenter,
	}
}

func tableSortIndicatorPlacement(colStart, rowStart int) facet.Placement {
	return facet.Placement{
		Mode: facet.PlacementGrid,
		Grid: facet.GridPlacement{
			ColStart: colStart,
			RowStart: rowStart,
			ColSpan:  1,
			RowSpan:  1,
		},
		Align: facet.AlignEnd,
	}
}

func stableTableKey(primary, fallback string, index int) string {
	key := strings.TrimSpace(primary)
	if key != "" {
		return key
	}
	key = strings.TrimSpace(fallback)
	if key != "" {
		return key
	}
	return fmt.Sprintf("item-%d", index)
}

func sortedTableRows(data TableData) []TableRow {
	rows := cloneTableRows(data.Rows)
	if len(rows) == 0 {
		return nil
	}
	sortKey := strings.TrimSpace(data.SortColumnKey)
	if sortKey == "" {
		return rows
	}
	columns := make(map[string]int, len(data.Columns))
	for i := range data.Columns {
		col := data.Columns[i]
		columns[stableTableKey(col.Key, col.Label, i)] = i
	}
	sortIndex, ok := columns[sortKey]
	if !ok {
		return rows
	}
	sort.SliceStable(rows, func(i, j int) bool {
		left := ""
		right := ""
		if sortIndex < len(rows[i].Cells) {
			left = rows[i].Cells[sortIndex]
		}
		if sortIndex < len(rows[j].Cells) {
			right = rows[j].Cells[sortIndex]
		}
		if data.SortDescending {
			return right < left
		}
		return left < right
	})
	return rows
}

func stableTableCellKey(rowKey, colKey string) string {
	return rowKey + ":" + colKey
}

func cloneTableData(in TableData) TableData {
	return TableData{
		Columns:        cloneTableColumns(in.Columns),
		Rows:           cloneTableRows(in.Rows),
		SortColumnKey:  strings.TrimSpace(in.SortColumnKey),
		SortDescending: in.SortDescending,
	}
}

func cloneTableColumns(in []TableColumn) []TableColumn {
	if len(in) == 0 {
		return nil
	}
	out := make([]TableColumn, len(in))
	copy(out, in)
	return out
}

func cloneTableRows(in []TableRow) []TableRow {
	if len(in) == 0 {
		return nil
	}
	out := make([]TableRow, len(in))
	for i := range in {
		out[i] = TableRow{
			Key:      strings.TrimSpace(in[i].Key),
			Cells:    cloneTableCells(in[i].Cells),
			Disabled: in[i].Disabled,
		}
	}
	return out
}

func cloneTableCells(in []string) []string {
	if len(in) == 0 {
		return nil
	}
	out := make([]string, len(in))
	copy(out, in)
	return out
}

type tableGroupPolicy struct {
	table *Table
}

func (tableGroupPolicy) Kind() facet.GroupLayoutKind { return facet.GroupLayoutGrid }

func (p tableGroupPolicy) MeasureGroup(ctx facet.GroupMeasureContext, children []facet.GroupChild) (facet.GroupMeasureResult, error) {
	if p.table == nil {
		return facet.GroupMeasureResult{}, nil
	}
	size := p.table.measure(ctx.MeasureContext, facet.Constraints{MaxSize: gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}}).Size
	return facet.GroupMeasureResult{Size: size}, nil
}

func (p tableGroupPolicy) ArrangeGroup(ctx facet.GroupArrangeContext, children []facet.GroupChild) ([]facet.ArrangedGroupChild, error) {
	if p.table == nil {
		return nil, nil
	}
	p.table.arrange(ctx.ArrangeContext, ctx.Bounds)
	arranged := make([]facet.ArrangedGroupChild, 0, len(children))
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		arranged = append(arranged, facet.ArrangedGroupChild{
			FacetID:   child.FacetID,
			MarkID:    child.MarkID,
			Bounds:    p.table.cachedCellBounds[child.FacetID],
			Placement: child.Attachment.Placement,
			ZOrder:    child.Attachment.ZOrder,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
