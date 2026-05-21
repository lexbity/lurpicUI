package structure

import (
	"fmt"
	"sort"
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	layoutgrid "codeburg.org/lexbit/lurpicui/layout/grid"
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
	Selected bool
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
	ZPriority int32
	Key       string
}

// Table implements the structure.table canonical mark.
type Table struct {
	facet.Facet

	layoutRole     facet.LayoutRole
	renderRole     facet.RenderRole
	projectionRole facet.ProjectionRole
	hitRole        facet.HitRole
	inputRole      facet.InputRole
	focusRole      facet.FocusRole
	textRole       facet.TextRole
	viewportRole   facet.ViewportRole

	Activated signal.Signal[int]

	Label    string
	Disabled bool
	Data     *store.ValueStore[TableData]

	hoveredRowIndex  int
	pressedRowIndex  int
	focusedRowIndex  int
	focusedVisible   bool
	focusFromPointer bool
	dragging         bool
	draggingAxis     ScrollDirection

	scrollOffset gfx.Point

	cachedTokens              theme.Tokens
	cachedRecipe              shared.TableSlots
	cachedBounds              gfx.Rect
	cachedViewportBounds      gfx.Rect
	cachedContentBounds       gfx.Rect
	cachedVerticalTrack       gfx.Rect
	cachedVerticalThumb       gfx.Rect
	cachedHorizontalTrack     gfx.Rect
	cachedHorizontalThumb     gfx.Rect
	cachedFocusBounds         gfx.Rect
	cachedRowBounds           map[string]gfx.Rect
	cachedColumnBounds        map[string]gfx.Rect
	cachedCellBounds          map[facet.FacetID]gfx.Rect
	cachedChildOrder          []facet.FacetID
	cachedChildSpecs          []tableChildSpec
	cachedHeaderCells         map[string]*primitive.Text
	cachedBodyCells           map[string]map[string]*primitive.Text
	cachedSelectionCells      map[string]*primitive.Text
	cachedSortIndicators      map[string]*primitive.Text
	cachedColumnKeys          []string
	cachedRowKeys             []string
	cachedVisibleRows         []TableRow
	cachedShowSelectionColumn bool
	cachedColumnWidths        []float32
	cachedRowHeights          []float32
	cachedWritingDirection    facet.WritingDirection
}

var _ facet.FacetImpl = (*Table)(nil)
var _ layout.AnchorExporter = (*Table)(nil)

// NewTable constructs a structure.table mark with canonical defaults.
func NewTable(label string, data TableData) *Table {
	t := &Table{
		Facet:                facet.NewFacet(),
		Label:                label,
		Data:                 store.NewValueStore(cloneTableData(data)),
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
	}
	t.layoutRole.Parent = facet.GroupParentContract{
		Kind:     facet.GroupLayoutGrid,
		Policy:   tableGroupPolicy{table: t},
		Children: t,
		Overflow: facet.OverflowScroll,
		Clipping: facet.GroupClipBounds,
	}
	t.layoutRole.Child = facet.GroupChildContract{
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
	t.layoutRole.OnMeasure = func(ctx facet.MeasureContext, constraints facet.Constraints) facet.MeasureResult {
		return t.measure(ctx, constraints)
	}
	t.layoutRole.OnArrange = func(ctx facet.ArrangeContext, bounds gfx.Rect) {
		t.layoutRole.ArrangedBounds = bounds
		t.arrange(ctx, bounds)
	}
	t.renderRole.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		if list == nil {
			return
		}
		cmds := t.buildCommands(bounds, nil, 1)
		if len(cmds) == 0 {
			return
		}
		list.Commands = append(list.Commands, cmds...)
	}
	t.projectionRole.OnProject = func(ctx facet.ProjectionContext) *gfx.CommandList {
		cmds := t.buildCommands(t.layoutRole.ArrangedBounds, ctx.Runtime, ctx.ContentScale)
		if len(cmds) == 0 {
			return nil
		}
		return &gfx.CommandList{Commands: cmds}
	}
	t.hitRole.OnHitTest = func(p gfx.Point) facet.HitResult { return t.hitTest(p) }
	t.inputRole.OnPointer = func(e facet.PointerEvent) bool { return t.onPointer(e) }
	t.inputRole.OnScroll = func(e facet.ScrollEvent) bool { return t.onScroll(e) }
	t.inputRole.OnKey = func(e facet.KeyEvent) bool { return t.onKey(e) }
	t.focusRole.Focusable = func() bool { return !t.Disabled && len(t.cachedRowKeys) > 0 }
	t.focusRole.TabIndex = 0
	t.focusRole.OnFocusGained = func() { t.onFocusGained() }
	t.focusRole.OnFocusLost = func() { t.onFocusLost() }
	t.viewportRole.Transform = gfx.Identity()
	t.textRole.IMEEnabled = false
	t.AddRole(&t.layoutRole)
	t.AddRole(&t.renderRole)
	t.AddRole(&t.projectionRole)
	t.AddRole(&t.hitRole)
	t.AddRole(&t.inputRole)
	t.AddRole(&t.focusRole)
	t.AddRole(&t.textRole)
	t.AddRole(&t.viewportRole)
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
	t.Facet.BindImpl(t)
	return &t.Facet
}

// AccessibilityRole reports the semantic role required by the spec.
func (t *Table) AccessibilityRole() string { return "table" }

// AccessibleName reports the semantic name source required by the spec.
func (t *Table) AccessibleName() string { return strings.TrimSpace(t.Label) }

// SetLabel updates the authored accessible label.
func (t *Table) SetLabel(label string) {
	if t == nil || t.Label == label {
		return
	}
	t.Label = label
	t.invalidate(facet.DirtyProjection)
}

// SetDisabled toggles disabled state.
func (t *Table) SetDisabled(disabled bool) {
	if t == nil || t.Disabled == disabled {
		return
	}
	t.Disabled = disabled
	if disabled {
		t.hoveredRowIndex = -1
		t.pressedRowIndex = -1
		t.focusedVisible = false
		t.focusFromPointer = false
		t.dragging = false
	}
	t.invalidate(facet.DirtyProjection | facet.DirtyHit)
}

// SetData replaces the canonical data-store contents.
func (t *Table) SetData(data TableData) {
	if t == nil {
		return
	}
	if t.Data == nil {
		t.Data = store.NewValueStore(cloneTableData(data))
		t.invalidate(facet.DirtyLayout | facet.DirtyProjection | facet.DirtyHit)
		return
	}
	t.Data.Set(cloneTableData(data))
}

// SetColumns updates only the column model.
func (t *Table) SetColumns(columns []TableColumn) {
	if t == nil {
		return
	}
	data := t.data()
	data.Columns = cloneTableColumns(columns)
	t.SetData(data)
}

// SetRows updates only the row model.
func (t *Table) SetRows(rows []TableRow) {
	if t == nil {
		return
	}
	data := t.data()
	data.Rows = cloneTableRows(rows)
	t.SetData(data)
}

// SetSortColumn updates the active sort column.
func (t *Table) SetSortColumn(key string, descending bool) {
	if t == nil {
		return
	}
	data := t.data()
	data.SortColumnKey = key
	data.SortDescending = descending
	t.SetData(data)
}

// Children returns the immediate child list.
func (t *Table) Children() []facet.GroupChild {
	if t == nil {
		return nil
	}
	if t.Data == nil {
		return nil
	}
	t.syncChildren()
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
	bounds := t.layoutRole.ArrangedBounds
	if bounds.IsEmpty() && !ctx.ResolvedLayer.Bounds.IsEmpty() {
		bounds = ctx.ResolvedLayer.Bounds
	}
	if bounds.IsEmpty() {
		return nil
	}
	out := layout.AnchorSet{
		"bounds_center":       gfx.Point{X: (bounds.Min.X + bounds.Max.X) * 0.5, Y: (bounds.Min.Y + bounds.Max.Y) * 0.5},
		"bounds_top_left":     bounds.Min,
		"bounds_top_right":    gfx.Point{X: bounds.Max.X, Y: bounds.Min.Y},
		"bounds_bottom_left":  gfx.Point{X: bounds.Min.X, Y: bounds.Max.Y},
		"bounds_bottom_right": gfx.Point{X: bounds.Max.X, Y: bounds.Max.Y},
		"baseline":            gfx.Point{X: bounds.Min.X, Y: bounds.Min.Y},
	}
	if !t.cachedViewportBounds.IsEmpty() {
		out["viewport"] = gfx.Point{X: (t.cachedViewportBounds.Min.X + t.cachedViewportBounds.Max.X) * 0.5, Y: (t.cachedViewportBounds.Min.Y + t.cachedViewportBounds.Max.Y) * 0.5}
	}
	if !t.cachedContentBounds.IsEmpty() {
		out["content"] = gfx.Point{X: (t.cachedContentBounds.Min.X + t.cachedContentBounds.Max.X) * 0.5, Y: (t.cachedContentBounds.Min.Y + t.cachedContentBounds.Max.Y) * 0.5}
	}
	return out
}

// OnAttach subscribes to any attached store.
func (t *Table) OnAttach(ctx facet.AttachContext) {}

// OnActivate is unused.
func (t *Table) OnActivate() {}

// OnDeactivate is unused.
func (t *Table) OnDeactivate() {}

// OnDetach clears cached projection state.
func (t *Table) OnDetach() {
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
	t.cachedVisibleRows = nil
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
	t.Base().Invalidate(flags)
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
	visibleRows := sortedTableRows(data)
	if t.focusedRowIndex < 0 && len(visibleRows) > 0 {
		t.focusedRowIndex = 0
	}
	if len(visibleRows) == 0 {
		t.focusedRowIndex = -1
	}
	if t.focusedRowIndex >= len(visibleRows) {
		t.focusedRowIndex = len(visibleRows) - 1
	}

	headerCells := make(map[string]*primitive.Text, len(data.Columns))
	bodyCells := make(map[string]map[string]*primitive.Text, len(visibleRows))
	selectionCells := make(map[string]*primitive.Text, len(visibleRows)+1)
	sortIndicators := make(map[string]*primitive.Text)
	hasSelection := false
	for i := range visibleRows {
		if visibleRows[i].Selected {
			hasSelection = true
			break
		}
	}
	showSelectionColumn := hasSelection
	selectionOffset := 0
	if showSelectionColumn {
		selectionOffset = 1
	}
	childSpecs := make([]tableChildSpec, 0, len(data.Columns)+len(visibleRows)*2+1)
	columnKeys := make([]string, 0, len(data.Columns))
	rowKeys := make([]string, 0, len(visibleRows))
	if showSelectionColumn {
		headerKey := "__selection__"
		header := t.cachedSelectionCells[headerKey]
		if header == nil {
			header = primitive.NewText("")
		} else {
			header.SetContent("")
		}
		header.SetTypography(theme.TextLabelM)
		header.SetForeground(theme.ColorTextSecondary)
		header.SetOverflow(primitive.TextOverflowClip)
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
			cell = primitive.NewText(label)
		} else {
			cell.SetContent(label)
		}
		cell.SetTypography(theme.TextLabelM)
		cell.SetForeground(theme.ColorTextSecondary)
		cell.SetOverflow(primitive.TextOverflowTruncate)
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
				indicator = primitive.NewText("▼")
				if data.SortDescending {
					indicator.SetContent("▼")
				} else {
					indicator.SetContent("▲")
				}
			} else {
				if data.SortDescending {
					indicator.SetContent("▼")
				} else {
					indicator.SetContent("▲")
				}
			}
			indicator.SetTypography(theme.TextLabelM)
			indicator.SetForeground(theme.ColorPrimary)
			indicator.SetOverflow(primitive.TextOverflowClip)
			sortIndicators[key] = indicator
			childSpecs = append(childSpecs, tableChildSpec{
				Facet:     indicator,
				MarkID:    tableMarkIDSortIndicator,
				Placement: tableSortIndicatorPlacement(colIndex+selectionOffset, 0),
				ZPriority: 1,
				Key:       "sort:" + key,
			})
		}
	}

	for rowIndex := range visibleRows {
		row := visibleRows[rowIndex]
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
				indicator = primitive.NewText("")
			}
			indicator.SetTypography(theme.TextLabelM)
			if row.Selected {
				indicator.SetContent("✓")
				indicator.SetForeground(theme.ColorPrimary)
			} else {
				indicator.SetContent("")
				indicator.SetForeground(theme.ColorTextSecondary)
			}
			indicator.SetOverflow(primitive.TextOverflowClip)
			if row.Disabled || t.Disabled {
				indicator.SetDisabled(true)
			} else {
				indicator.SetDisabled(false)
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
				cell = primitive.NewText(content)
			} else {
				cell.SetContent(content)
			}
			cell.SetTypography(theme.TextBodyM)
			cell.SetForeground(theme.ColorText)
			cell.SetOverflow(primitive.TextOverflowTruncate)
			if row.Selected {
				cell.SetForeground(theme.ColorPrimary)
			}
			if row.Disabled || t.Disabled {
				cell.SetDisabled(true)
			} else {
				cell.SetDisabled(false)
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
	t.cachedVisibleRows = visibleRows
	t.cachedShowSelectionColumn = showSelectionColumn
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
		if col.Width > 0 {
			columns[i+selectionOffset] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackFixed, Value: col.Width}
		} else {
			columns[i+selectionOffset] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackIntrinsic}
		}
	}
	rows := make([]layoutgrid.TrackDef, len(data.Rows)+1)
	for i := range rows {
		rows[i] = layoutgrid.TrackDef{Sizing: layoutgrid.TrackIntrinsic}
	}
	return layoutgrid.New(layoutgrid.Config{
		Columns:       columns,
		Rows:          rows,
		ColumnGap:     t.gridGap(),
		RowGap:        t.gridGap(),
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
	t.syncChildren()
	data := t.data()
	children := t.Children()
	if len(children) == 0 {
		size := constraints.Constrain(gfx.Size{})
		t.layoutRole.MeasuredSize = size
		t.layoutRole.MeasuredResult = facet.MeasureResult{Size: size, Intrinsic: facet.IntrinsicSize{Min: size, Preferred: size, Max: size}, Constraints: constraints}
		return t.layoutRole.MeasuredResult
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
	policy := t.buildGridPolicy(data)
	gridChildren := t.gridChildren(children)
	size, err := policy.Measure(gridChildren, constraints.MaxSize)
	if err != nil {
		size = constraints.Constrain(gfx.Size{})
	}
	t.cachedContentBounds = gfx.RectFromXYWH(0, 0, size.W, size.H)
	measured := constraints.Constrain(size)
	t.layoutRole.MeasuredSize = measured
	t.layoutRole.MeasuredResult = facet.MeasureResult{
		Size:        measured,
		Intrinsic:   facet.IntrinsicSize{Min: measured, Preferred: measured, Max: measured},
		Constraints: constraints,
	}
	return t.layoutRole.MeasuredResult
}

func (t *Table) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	contentSize := t.cachedContentBounds
	t.cachedBounds = bounds
	t.cachedViewportBounds = bounds
	t.cachedVerticalTrack = gfx.Rect{}
	t.cachedVerticalThumb = gfx.Rect{}
	t.cachedHorizontalTrack = gfx.Rect{}
	t.cachedHorizontalThumb = gfx.Rect{}
	t.cachedFocusBounds = bounds.Inset(maxFloat(1, bounds.Width()*0.04), maxFloat(1, bounds.Height()*0.04))
	t.layoutRole.ArrangedBounds = bounds
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
	childBounds := make(map[facet.FacetID]gfx.Rect, len(arranged))
	order := make([]facet.FacetID, 0, len(arranged))
	rowBounds := make(map[string]gfx.Rect, len(data.Rows)+1)
	columnBounds := make(map[string]gfx.Rect, len(data.Columns))
	cellBounds := make(map[facet.FacetID]gfx.Rect, len(arranged))
	for _, child := range arranged {
		childBounds[child.FacetID] = child.Bounds
		cellBounds[child.FacetID] = child.Bounds
		order = append(order, child.FacetID)
		if child.Placement.RowStart >= 0 && child.Placement.RowStart < len(t.cachedVisibleRows)+1 {
			rowKey := t.visibleTableRowKeyAtIndex(child.Placement.RowStart)
			rowBounds[rowKey] = unionRect(rowBounds[rowKey], child.Bounds)
		}
		if t.cachedShowSelectionColumn && child.Placement.ColStart == 0 {
			columnBounds["__selection__"] = unionRect(columnBounds["__selection__"], child.Bounds)
			continue
		}
		colStart := child.Placement.ColStart
		if t.cachedShowSelectionColumn {
			colStart--
		}
		if colStart >= 0 && colStart < len(data.Columns) {
			col := data.Columns[colStart]
			colKey := stableTableKey(col.Key, col.Label, colStart)
			columnBounds[colKey] = unionRect(columnBounds[colKey], child.Bounds)
		}
	}
	t.cachedChildOrder = order
	t.cachedCellBounds = cellBounds
	t.cachedRowBounds = rowBounds
	t.cachedColumnBounds = columnBounds
	t.cachedContentBounds = gfx.RectFromXYWH(contentRect.Min.X, contentRect.Min.Y, contentSize.Width(), contentSize.Height())
	t.updateScrollBounds(bounds)
	t.viewportRole.WorldBounds = bounds
}

func (t *Table) buildCommands(bounds gfx.Rect, runtime any, contentScale float32) []gfx.Command {
	if t == nil || bounds.IsEmpty() {
		return nil
	}
	style, slots := t.resolveProjectionTheme(runtime)
	tokens := style.Tokens
	state := theme.StateDefault
	if t.Disabled {
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
	if !isTransparentMaterial(root) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), root)...)
	}
	if !isTransparentMaterial(surface) {
		cmds = append(cmds, materialCommands(gfx.RectPath(bounds), surface)...)
	}
	if !isTransparentMaterial(headerRow) && len(t.cachedRowKeys) > 0 {
		headerKey := tableRowKeyForIndex(t.data(), 0)
		if headerBounds := t.cachedRowBounds[headerKey]; !headerBounds.IsEmpty() {
			cmds = append(cmds, materialCommands(gfx.RectPath(headerBounds), headerRow)...)
		}
	}
	if !isTransparentMaterial(bodyRows) {
		rows := t.cachedVisibleRows
		if len(rows) == 0 {
			rows = sortedTableRows(t.data())
		}
		for i := range rows {
			rowKey := stableTableKey(rows[i].Key, "", i)
			if rowBounds := t.cachedRowBounds[rowKey]; !rowBounds.IsEmpty() {
				cmds = append(cmds, materialCommands(gfx.RectPath(rowBounds), bodyRows)...)
			}
		}
	}
	if !isTransparentMaterial(selectionColumn) {
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
			cmds = append(cmds, materialCommands(gfx.RectPath(childBounds), selectionColumn)...)
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
				if !isTransparentMaterial(headerCell) {
					cmds = append(cmds, materialCommands(gfx.RectPath(childBounds), headerCell)...)
				}
			case tableMarkIDBodyCell:
				if !isTransparentMaterial(bodyCell) {
					cmds = append(cmds, materialCommands(gfx.RectPath(childBounds), bodyCell)...)
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
	if t.focusedVisible && !isTransparentMaterial(focus) {
		cmds = append(cmds, materialCommands(gfx.RectPath(t.cachedFocusBounds), focus)...)
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
	if t == nil || t.layoutRole.ArrangedBounds.IsEmpty() || !t.layoutRole.ArrangedBounds.Contains(p) {
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
	if t.Disabled {
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
			t.invalidate(facet.DirtyProjection)
			return true
		}
		if t.cachedHorizontalThumb.Contains(e.Position) {
			t.dragging = true
			t.draggingAxis = ScrollDirectionHorizontal
			t.updateOffsetFromDrag(e.Position)
			t.invalidate(facet.DirtyProjection)
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
			t.invalidate(facet.DirtyProjection)
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
	if t.Disabled {
		return false
	}
	if e.DeltaX == 0 && e.DeltaY == 0 {
		return false
	}
	next := t.scrollOffset
	next.X -= e.DeltaX
	next.Y -= e.DeltaY
	t.scrollOffset = t.clampScrollOffset(next)
	t.invalidate(facet.DirtyProjection)
	return true
}

func (t *Table) onKey(e facet.KeyEvent) bool {
	if t.Disabled {
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
		rows := t.cachedVisibleRows
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
	t.invalidate(facet.DirtyProjection)
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
			ZPriority: spec.ZPriority,
		},
		Layout:   base.LayoutRole(),
		Contract: base.LayoutRole().Child,
	}
}

func (t *Table) updateScrollBounds(bounds gfx.Rect) {
	t.cachedViewportBounds = bounds
	maxX := maxFloat(0, t.cachedContentBounds.Width()-bounds.Width())
	maxY := maxFloat(0, t.cachedContentBounds.Height()-bounds.Height())
	t.scrollOffset = gfx.Point{
		X: clampFloat(t.scrollOffset.X, 0, maxX),
		Y: clampFloat(t.scrollOffset.Y, 0, maxY),
	}
	track := t.trackThickness()
	if maxY > 0 {
		trackHeight := bounds.Height()
		if maxX > 0 {
			trackHeight -= track
		}
		if trackHeight < 0 {
			trackHeight = 0
		}
		t.cachedVerticalTrack = gfx.RectFromXYWH(bounds.Max.X-track, bounds.Min.Y, track, trackHeight)
		thumbHeight := maxFloat(track*2, trackHeight*(bounds.Height()/maxFloat(1, t.cachedContentBounds.Height())))
		if thumbHeight > trackHeight {
			thumbHeight = trackHeight
		}
		maxOffset := maxFloat(1, maxY)
		thumbY := bounds.Min.Y + (t.scrollOffset.Y/maxOffset)*(trackHeight-thumbHeight)
		t.cachedVerticalThumb = gfx.RectFromXYWH(bounds.Max.X-track, thumbY, track, thumbHeight)
	}
	if maxX > 0 {
		trackWidth := bounds.Width()
		if maxY > 0 {
			trackWidth -= track
		}
		if trackWidth < 0 {
			trackWidth = 0
		}
		t.cachedHorizontalTrack = gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-track, trackWidth, track)
		thumbWidth := maxFloat(track*2, trackWidth*(bounds.Width()/maxFloat(1, t.cachedContentBounds.Width())))
		if thumbWidth > trackWidth {
			thumbWidth = trackWidth
		}
		maxOffset := maxFloat(1, maxX)
		thumbX := bounds.Min.X + (t.scrollOffset.X/maxOffset)*(trackWidth-thumbWidth)
		t.cachedHorizontalThumb = gfx.RectFromXYWH(thumbX, bounds.Max.Y-track, thumbWidth, track)
	}
}

func (t *Table) updateOffsetFromDrag(p gfx.Point) {
	if t == nil {
		return
	}
	switch t.draggingAxis {
	case ScrollDirectionHorizontal:
		trackRect := t.cachedHorizontalTrack
		thumbRect := t.cachedHorizontalThumb
		maxOffset := t.maxScrollX()
		if trackRect.IsEmpty() || thumbRect.IsEmpty() || maxOffset <= 0 {
			return
		}
		trackSpan := maxFloat(1, trackRect.Width()-thumbRect.Width())
		pos := p.X - trackRect.Min.X - thumbRect.Width()*0.5
		t.scrollOffset.X = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
	default:
		trackRect := t.cachedVerticalTrack
		thumbRect := t.cachedVerticalThumb
		maxOffset := t.maxScrollY()
		if trackRect.IsEmpty() || thumbRect.IsEmpty() || maxOffset <= 0 {
			return
		}
		trackSpan := maxFloat(1, trackRect.Height()-thumbRect.Height())
		pos := p.Y - trackRect.Min.Y - thumbRect.Height()*0.5
		t.scrollOffset.Y = clampFloat((pos/trackSpan)*maxOffset, 0, maxOffset)
	}
	t.scrollOffset = t.clampScrollOffset(t.scrollOffset)
}

func (t *Table) rowAtPoint(p gfx.Point) int {
	if t == nil {
		return -1
	}
	rows := t.cachedVisibleRows
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

func (t *Table) selectRow(index int) {
	if t == nil {
		return
	}
	data := t.data()
	if index < 0 || index >= len(data.Rows) {
		return
	}
	visible := t.cachedVisibleRows
	if len(visible) == 0 {
		visible = sortedTableRows(data)
	}
	if index >= len(visible) {
		return
	}
	selectedKey := visible[index].Key
	for i := range data.Rows {
		data.Rows[i].Selected = stableTableKey(data.Rows[i].Key, "", i) == selectedKey
	}
	t.SetData(data)
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
}

func (t *Table) moveFocus(delta int) {
	if t == nil {
		return
	}
	rows := t.cachedVisibleRows
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
	rows := t.cachedVisibleRows
	if len(rows) == 0 {
		rows = sortedTableRows(t.data())
	}
	if len(rows) == 0 {
		return 1
	}
	return max(1, int(span/maxFloat(1, t.gridGap()*2+18)))
}

func (t *Table) trackThickness() float32 {
	if t.cachedTokens.Spacing.TouchTarget > 0 {
		return maxFloat(8, t.cachedTokens.Spacing.TouchTarget*0.12)
	}
	return 8
}

func (t *Table) gridGap() float32 {
	if t.cachedTokens.Spacing.XS > 0 {
		return maxFloat(4, t.cachedTokens.Spacing.XS)
	}
	return 4
}

func (t *Table) selectionColumnWidth() float32 {
	if t.cachedTokens.Spacing.TouchTarget > 0 {
		return maxFloat(24, t.cachedTokens.Spacing.TouchTarget*0.45)
	}
	return 24
}

func (t *Table) keyboardStep() float32 {
	span := t.cachedViewportBounds.Height()
	if span <= 0 {
		return 24
	}
	return maxFloat(24, span*0.12)
}

func (t *Table) maxScrollX() float32 {
	return maxFloat(0, t.cachedContentBounds.Width()-t.cachedViewportBounds.Width())
}

func (t *Table) maxScrollY() float32 {
	return maxFloat(0, t.cachedContentBounds.Height()-t.cachedViewportBounds.Height())
}

func (t *Table) visibleRowKeyAtIndex(index int) string {
	if t == nil || index < 0 {
		return ""
	}
	if index >= len(t.cachedVisibleRows) {
		return ""
	}
	return stableTableKey(t.cachedVisibleRows[index].Key, "", index)
}

func (t *Table) visibleTableRowKeyAtIndex(index int) string {
	if index == 0 {
		return "__header__"
	}
	return t.visibleRowKeyAtIndex(index - 1)
}

func (t *Table) clampScrollOffset(next gfx.Point) gfx.Point {
	return gfx.Point{
		X: clampFloat(next.X, 0, t.maxScrollX()),
		Y: clampFloat(next.Y, 0, t.maxScrollY()),
	}
}

func (t *Table) cursorShape() facet.CursorShape {
	if t.Disabled {
		return facet.CursorDefault
	}
	return facet.CursorPointer
}

func (t *Table) barCommands(bounds gfx.Rect, material theme.Material, opacity float32) []gfx.Command {
	if bounds.IsEmpty() || isTransparentMaterial(material) {
		return nil
	}
	cmds := make([]gfx.Command, 0, 4)
	if opacity > 0 && opacity < 1 {
		cmds = append(cmds, gfx.PushOpacity{Alpha: opacity})
	}
	cmds = append(cmds, materialCommands(gfx.RectPath(bounds), material)...)
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

func alignForColumn(align facet.Alignment) facet.Alignment {
	if align == 0 {
		return facet.AlignStretch
	}
	return align
}

func tableCellPlacement(colStart, rowStart int, align facet.Alignment) facet.Placement {
	return facet.Placement{
		Mode: facet.PlacementGrid,
		Grid: facet.GridPlacement{
			ColStart: colStart,
			RowStart: rowStart,
			ColSpan:  1,
			RowSpan:  1,
		},
		Align: alignForColumn(align),
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
			Selected: in[i].Selected,
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

func unionRect(existing, next gfx.Rect) gfx.Rect {
	if existing.IsEmpty() {
		return next
	}
	if next.IsEmpty() {
		return existing
	}
	minX := existing.Min.X
	minY := existing.Min.Y
	maxX := existing.Max.X
	maxY := existing.Max.Y
	if next.Min.X < minX {
		minX = next.Min.X
	}
	if next.Min.Y < minY {
		minY = next.Min.Y
	}
	if next.Max.X > maxX {
		maxX = next.Max.X
	}
	if next.Max.Y > maxY {
		maxY = next.Max.Y
	}
	return gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
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
			ZPriority: child.Attachment.ZPriority,
			Contract:  child.Contract,
		})
	}
	return arranged, nil
}
