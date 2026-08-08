package studio

import (
	"strconv"
	"strings"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/data"
	"codeburg.org/lexbit/lurpicui/marks/feedback"
	"codeburg.org/lexbit/lurpicui/marks/input"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/lurpicui/theme/recipes/uiinput"
)

// EditableGrid is the flagship's editable spreadsheet (F-table composition):
// rows are cell bands driven by the never-consumed marks/data.CollectionBinder
// over the shared Rows store; one Value column is editable through an overlay
// text_field that commits back through Rows.Update on the runtime thread.
// Linked brushing (chart ↔ grid) flows through the shared Hover / Selection
// stores, and a brush highlight scrolls the row into view.
type EditableGrid struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	hit    facet.HitRole
	input  facet.InputRole

	rows   *store.CollectionStore[dataset.Row]
	binder *data.CollectionBinder[dataset.Row]
	style  gridStyle

	hover       *store.ValueStore[*store.ItemID]
	hoverRegion *store.ValueStore[string]
	selection   *store.ValueStore[store.ItemID]

	editing   bool
	editID    store.ItemID //lurpiclint:ignore LL012 -- the active cell's row is ephemeral edit-session UI state in a bespoke interactive host (F-lint-hosts); cleared on commit/cancel
	cellValue *store.ValueStore[string]
	invalid   *store.ValueStore[string]
	editor    *input.TextField
	alert     *feedback.Alert

	scroll int // first visible row index

	rowHeight float32
	bg        gfx.Color
	hoverBg   gfx.Color
	selBg     gfx.Color

	rt      facet.RuntimeServices
	cleanup func()
}

// NewEditableGrid builds the spreadsheet over the shared rows store, using the
// given brush stores for linked highlighting.
func NewEditableGrid(rows *store.CollectionStore[dataset.Row], fonts *text.FontRegistry, themeCtx theme.ResolvedContext, brush BrushStores) *EditableGrid {
	g := &EditableGrid{
		rows:        rows,
		hover:       brush.Hover,
		hoverRegion: brush.HoverRegion,
		selection:   brush.Selection,
		cellValue:   store.NewValueStore(""),
		invalid:     store.NewValueStore(""),
		rowHeight:   gridRowHeight,
		bg:          themeCtx.Color(theme.ColorSurfaceVariant),
		hoverBg:     themeCtx.Color(theme.ColorSurface),
		selBg:       themeCtx.Color(theme.ColorPrimary),
	}
	g.style = gridStyle{
		fonts: fonts,
		style: themeCtx.TextStyle(theme.TextBodyS),
		color: themeCtx.Color(theme.ColorText),
	}

	g.Facet = facet.NewFacet()
	g.binder = data.NewCollectionBinder(&g.Facet, rows, func(r dataset.Row) facet.FacetImpl {
		return newGridRow(rows, rows.Identify(r), g.style)
	})
	g.editor = input.NewTextField("Cell", uiinput.TextInputOutlined, g.cellValue)
	g.alert = feedback.NewAlert("Invalid value", "")
	g.alert.Message = marks.FromStore(g.invalid, facet.DirtyProjection)
	facet.AttachLayer(g, g.editor, facet.LayerAttachment{ZPriority: 30})
	facet.AttachLayer(g, g.alert, facet.LayerAttachment{ZPriority: 20})

	g.layout = facet.LayoutRole{ //lurpiclint:ignore * -- bespoke spreadsheet grid host (F-lint-hosts)
		OnMeasure: func(ctx facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
			return facet.MeasureResult{Size: c.Constrain(c.MaxSize)}
		},
		OnArrange: func(ctx facet.ArrangeContext, bounds gfx.Rect) {
			g.arrange(ctx, bounds)
		},
	}
	g.render = facet.RenderRole{
		OnCollect: func(list *gfx.CommandList, bounds gfx.Rect) {
			list.Add(gfx.FillRect{Rect: bounds, Brush: gfx.SolidBrush(g.bg)})
			list.Commands = append(list.Commands, g.highlightCommands(bounds)...)
			list.Commands = append(list.Commands, g.separatorCommands(bounds)...)
		},
	}
	g.hit = facet.HitRole{
		OnHitTest: func(pt gfx.Point) facet.HitResult {
			if !g.layout.ArrangedBounds.IsEmpty() && g.layout.ArrangedBounds.Contains(pt) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorText}
			}
			return facet.HitResult{}
		},
	}
	g.input = facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool { return g.onPointer(e) },
		OnKey:     func(e facet.KeyEvent) bool { return g.onKey(e) },
	}
	g.AddRole(&g.layout)
	g.AddRole(&g.render)
	g.AddRole(&g.hit)
	g.AddRole(&g.input)
	return g
}

// CellValue returns the editor's streaming value store.
func (g *EditableGrid) CellValue() *store.ValueStore[string] { return g.cellValue }

// Invalid returns the invalid-input message store (the inline alert's text).
func (g *EditableGrid) Invalid() *store.ValueStore[string] { return g.invalid }

// Editing reports whether a cell editor is active.
func (g *EditableGrid) Editing() bool { return g.editing }

// EditRow returns the row being edited (when Editing).
func (g *EditableGrid) EditRow() store.ItemID { return g.editID }

// ScrollOffset returns the first visible row index (the brush scroll follow).
func (g *EditableGrid) ScrollOffset() int { return g.scroll }

// RowRect returns the arranged rect of the given row (empty when not in view).
func (g *EditableGrid) RowRect(id store.ItemID) gfx.Rect {
	return g.rowRect(g.layout.ArrangedBounds, id)
}

func (g *EditableGrid) arrange(ctx facet.ArrangeContext, bounds gfx.Rect) {
	children := g.binder.Children()
	visible := int(bounds.Height()/g.rowHeight) + 1
	if visible < 1 {
		visible = 1
	}
	if len(children) > 0 && g.scroll > len(children)-1 {
		g.scroll = len(children) - 1
	}
	for i, child := range children {
		if i < g.scroll || i >= g.scroll+visible {
			child.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
			continue
		}
		y := bounds.Min.Y + float32(i-g.scroll)*g.rowHeight
		child.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), g.rowHeight))
	}

	if g.editing {
		if cell := g.valueCellRect(bounds, g.editID); !cell.IsEmpty() {
			g.editor.Base().LayoutRole().Arrange(ctx, cell)
		} else {
			g.editor.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
		}
	} else {
		g.editor.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}

	if g.invalid.Get() != "" {
		h := float32(30)
		g.alert.Base().LayoutRole().Arrange(ctx, gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-h, bounds.Width(), h))
	} else {
		g.alert.Base().LayoutRole().Arrange(ctx, gfx.Rect{})
	}
}

// valueCellRect returns the arranged rect of the Value cell for the given row
// (empty when the row is not in view).
func (g *EditableGrid) valueCellRect(bounds gfx.Rect, id store.ItemID) gfx.Rect {
	idx := g.rowIndex(id)
	visible := int(bounds.Height()/g.rowHeight) + 1
	if idx < g.scroll || idx >= g.scroll+visible {
		return gfx.Rect{}
	}
	y := bounds.Min.Y + float32(idx-g.scroll)*g.rowHeight
	start := bounds.Width() * gridColumns[1].start
	width := bounds.Width() * (gridColumns[1].end - gridColumns[1].start)
	return gfx.RectFromXYWH(bounds.Min.X+start, y, width, g.rowHeight)
}

func (g *EditableGrid) rowIndex(id store.ItemID) int {
	for i, child := range g.binder.Children() {
		if gr, ok := child.(*gridRow); ok && gr.RowID() == id {
			return i
		}
	}
	return -1
}

func (g *EditableGrid) rowAt(bounds gfx.Rect, pt gfx.Point) (int, store.ItemID, bool) {
	idx := g.scroll + int((pt.Y-bounds.Min.Y)/g.rowHeight)
	children := g.binder.Children()
	if idx < 0 || idx >= len(children) {
		return idx, 0, false
	}
	row := g.rows.All()[idx]
	return idx, g.rows.Identify(row), true
}

func (g *EditableGrid) colAt(bounds gfx.Rect, pt gfx.Point) int {
	frac := (pt.X - bounds.Min.X) / bounds.Width()
	for i, c := range gridColumns {
		if frac >= c.start && frac <= c.end {
			return i
		}
	}
	return -1
}

func (g *EditableGrid) onPointer(e facet.PointerEvent) bool {
	bounds := g.layout.ArrangedBounds
	if bounds.IsEmpty() {
		return false
	}
	switch e.Kind {
	case platform.PointerMove:
		if _, id, ok := g.rowAt(bounds, e.Position); ok {
			if g.hover != nil {
				g.hover.Set(&id)
			}
			if g.hoverRegion != nil {
				g.hoverRegion.Set("")
			}
		}
	case platform.PointerPress:
		if e.Button != platform.PointerLeft {
			return true
		}
		if _, id, ok := g.rowAt(bounds, e.Position); ok {
			if g.selection != nil {
				g.selection.Set(id)
			}
			if g.colAt(bounds, e.Position) == 1 {
				g.activateEdit(bounds, id)
			} else if g.editing {
				g.commitEdit()
			}
		}
	case platform.PointerRelease:
	}
	return true
}

func (g *EditableGrid) activateEdit(bounds gfx.Rect, id store.ItemID) {
	row, ok := g.rows.Get(id)
	if !ok {
		return
	}
	g.editID = id
	g.cellValue.Set(strconv.FormatFloat(row.Value, 'f', 1, 64))
	g.invalid.Set("")
	g.editing = true
	if fs, ok := g.rt.(interface{ SetFocus(facet.FacetImpl) }); ok {
		fs.SetFocus(g.editor)
	}
	invalidateLayout(g, g.rt, "grid.activateEdit")
}

func (g *EditableGrid) cancelEdit() {
	g.editing = false
	g.invalid.Set("")
}

func (g *EditableGrid) onKey(e facet.KeyEvent) bool {
	if !g.editing {
		return false
	}
	switch e.Key {
	case platform.KeyEnter:
		// Commit and advance to the next cell on success. (F-tab-eaten: the
		// input system consumes Tab for global focus traversal before a
		// focused facet sees it, so cell traversal is driven by Enter instead
		// of Tab.)
		if e.Kind == platform.KeyPress {
			if g.commitEdit() {
				g.traverseNext()
			}
			return true
		}
	case platform.KeyEscape:
		// The editor consumes the Escape press; the release bubbles to the
		// grid, which cancels the session.
		if e.Kind == platform.KeyRelease {
			g.cancelEdit()
			return true
		}
	}
	return false
}

// commitEdit validates the editor's value and writes it back through
// Rows.Update on the runtime thread. Returns true when the commit applied;
// an invalid value raises the inline alert and leaves the session open.
func (g *EditableGrid) commitEdit() bool {
	row, ok := g.rows.Get(g.editID)
	if !ok {
		g.cancelEdit()
		return false
	}
	value, err := strconv.ParseFloat(strings.TrimSpace(g.cellValue.Get()), 64)
	if err != nil {
		g.invalid.Set("Value must be a number: " + strings.TrimSpace(g.cellValue.Get()))
		return false
	}
	row.Value = value
	g.rows.Update(row) // runtime thread — CollectionStore asserts it
	g.invalid.Set("")
	g.editing = false
	return true
}

// traverseNext moves the editor to the next row's Value cell (Enter advance).
func (g *EditableGrid) traverseNext() {
	idx := g.rowIndex(g.editID)
	rows := g.rows.All()
	next := idx + 1
	if len(rows) == 0 {
		return
	}
	if next >= len(rows) {
		next = 0
	}
	g.activateEdit(g.layout.ArrangedBounds, g.rows.Identify(rows[next]))
}

// highlightCommands draws the hovered/selected row backgrounds, and scrolls a
// brush target into view is handled via the store subscriptions.
func (g *EditableGrid) highlightCommands(bounds gfx.Rect) []gfx.Command {
	var cmds []gfx.Command
	if g.hover != nil {
		if id := g.hover.Get(); id != nil && g.hoverRegion.Get() == "" {
			if r := g.rowRect(bounds, *id); !r.IsEmpty() {
				cmds = append(cmds, gfx.FillRect{Rect: r, Brush: gfx.SolidBrush(g.hoverBg)})
			}
		}
	}
	if g.hoverRegion != nil && g.hoverRegion.Get() != "" {
		for _, r := range g.regionRowRects(bounds, g.hoverRegion.Get()) {
			cmds = append(cmds, gfx.FillRect{Rect: r, Brush: gfx.SolidBrush(g.hoverBg)})
		}
	}
	if g.selection != nil {
		if r := g.rowRect(bounds, g.selection.Get()); !r.IsEmpty() {
			cmds = append(cmds, gfx.FillRect{Rect: r, Brush: gfx.SolidBrush(g.selBg)})
		}
	}
	return cmds
}

func (g *EditableGrid) separatorCommands(bounds gfx.Rect) []gfx.Command {
	visible := int(bounds.Height()/g.rowHeight) + 1
	if visible < 1 {
		return nil
	}
	cmds := make([]gfx.Command, 0, visible)
	for i := 1; i < visible; i++ {
		y := bounds.Min.Y + float32(i)*g.rowHeight
		cmds = append(cmds, gfx.StrokePath{
			Path: gfx.Path{Segments: []gfx.PathSegment{
				{Verb: gfx.PathMoveTo, Pts: [3]gfx.Point{{X: bounds.Min.X, Y: y}}},
				{Verb: gfx.PathLineTo, Pts: [3]gfx.Point{{X: bounds.Max.X, Y: y}}},
			}},
			Stroke: gfx.StrokeStyle{Width: 1},
			Brush:  gfx.SolidBrush(g.bg),
		})
	}
	return cmds
}

func (g *EditableGrid) rowRect(bounds gfx.Rect, id store.ItemID) gfx.Rect {
	idx := g.rowIndex(id)
	visible := int(bounds.Height()/g.rowHeight) + 1
	if idx < g.scroll || idx >= g.scroll+visible {
		return gfx.Rect{}
	}
	y := bounds.Min.Y + float32(idx-g.scroll)*g.rowHeight
	return gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), g.rowHeight)
}

func (g *EditableGrid) regionRowRects(bounds gfx.Rect, region string) []gfx.Rect {
	rows := g.rows.All()
	var out []gfx.Rect
	for i := range rows {
		if rows[i].Region != region {
			continue
		}
		if r := g.rowRect(bounds, g.rows.Identify(rows[i])); !r.IsEmpty() {
			out = append(out, r)
		}
	}
	return out
}

// scrollTo brings the row with the given id into view (the brush highlight
// follow). It only adjusts the offset when the row is off-screen.
func (g *EditableGrid) scrollTo(id store.ItemID) {
	idx := g.rowIndex(id)
	if idx < 0 {
		return
	}
	visible := int(g.rowViewHeight()/g.rowHeight) + 1
	if visible < 1 {
		visible = 1
	}
	if idx < g.scroll {
		g.scroll = idx
	} else if idx >= g.scroll+visible {
		g.scroll = idx - visible + 1
	}
	if g.scroll < 0 {
		g.scroll = 0
	}
}

func (g *EditableGrid) rowViewHeight() float32 {
	return g.layout.ArrangedBounds.Height()
}

func (g *EditableGrid) OnAttach(ctx facet.AttachContext) {
	g.rt = ctx.Runtime
	g.binder.OnAttach(ctx)
	hoverID := g.hover.OnChange.Subscribe(func(c signal.Change[*store.ItemID]) {
		if c.New != nil {
			g.scrollTo(*c.New)
		}
	})
	selID := g.selection.OnChange.Subscribe(func(c signal.Change[store.ItemID]) {
		g.scrollTo(c.New)
	})
	g.cleanup = func() {
		g.binder.OnDetach()
		g.hover.OnChange.Unsubscribe(hoverID)
		g.selection.OnChange.Unsubscribe(selID)
	}
}

func (g *EditableGrid) OnDetach() {
	if g.cleanup != nil {
		g.cleanup()
		g.cleanup = nil
	}
}

func (g *EditableGrid) Base() *facet.Facet { g.BindImpl(g); return &g.Facet }
func (g *EditableGrid) OnActivate()        {}
func (g *EditableGrid) OnDeactivate()      {}
