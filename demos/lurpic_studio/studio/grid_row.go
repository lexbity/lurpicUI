package studio

import (
	"strconv"

	"codeburg.org/lexbit/lurpicui/demos/lurpic_studio/dataset"
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/store"
	"codeburg.org/lexbit/lurpicui/text"
)

// gridRowHeight is the fixed height of one spreadsheet row band.
const gridRowHeight float32 = 26

// gridColumn splits a row band into the Time / Value / Region columns.
var gridColumns = []struct {
	start float32 // fraction of the band width
	end   float32
}{
	{0.02, 0.42}, // Time
	{0.44, 0.62}, // Value (editable)
	{0.64, 1.00}, // Region
}

// gridStyle carries the runtime-owned text service for grid cells.
type gridStyle struct {
	text  *text.Service
	style text.TextStyle
	color gfx.Color
}

// gridRow is one cell band in the editable spreadsheet: it draws the Time,
// Value, and Region columns from the current row data, re-read on every
// projection so an edit reflects immediately. It is a projection-only facet —
// the grid host owns hits, selection, and editing (F-unconsumed: the binder
// drives the row facets' lifecycle).
type gridRow struct {
	marks.Core
	rows   *store.CollectionStore[dataset.Row]
	id     store.ItemID
	style  gridStyle
	shaper *text.ShapeSession
}

// newGridRow builds a cell band for the given row.
func newGridRow(rows *store.CollectionStore[dataset.Row], id store.ItemID, style gridStyle) *gridRow {
	r := &gridRow{rows: rows, id: id, style: style}
	if style.text != nil {
		r.shaper, _ = style.text.AtScale(1)
	}
	r.Facet = facet.NewFacet()
	r.Layout.OnMeasure = func(_ facet.MeasureContext, c facet.Constraints) facet.MeasureResult {
		return facet.MeasureResult{Size: gfx.Size{W: c.MaxSize.W, H: gridRowHeight}}
	}
	r.Layout.OnArrange = func(_ facet.ArrangeContext, bounds gfx.Rect) {
		r.Layout.ArrangedBounds = bounds
	}
	r.BuildCommands = func(ctx facet.ProjectionContext) []gfx.Command {
		return r.cellCommands(r.Layout.ArrangedBounds)
	}
	r.RegisterRoles(r)
	return r
}

// RowID returns the row's collection id.
func (r *gridRow) RowID() store.ItemID { return r.id }

// ItemKey is the KeyedItem adoption (LDP Phase 3): the row's collection id
// is its stable locator key, addressable as e1.grid.row.<key>.
func (r *gridRow) ItemKey() string { return strconv.FormatUint(uint64(r.id), 10) }

// cellCommands draws the three column cells from the current row data.
func (r *gridRow) cellCommands(bounds gfx.Rect) []gfx.Command {
	if bounds.IsEmpty() {
		return nil
	}
	row, ok := r.rows.Get(r.id)
	if !ok {
		return nil
	}
	var cmds []gfx.Command
	if cmd := r.textCell(bounds, 0, row.Time.Format("01-02 15:04:05")); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := r.textCell(bounds, 1, strconv.FormatFloat(row.Value, 'f', 1, 64)); cmd != nil {
		cmds = append(cmds, cmd)
	}
	if cmd := r.textCell(bounds, 2, row.Region); cmd != nil {
		cmds = append(cmds, cmd)
	}
	return cmds
}

// textCell shapes and returns one column's glyph run (nil when unshapeable).
func (r *gridRow) textCell(bounds gfx.Rect, col int, label string) gfx.Command {
	if r.shaper == nil || label == "" {
		return nil
	}
	colSpec := gridColumns[col]
	x := bounds.Min.X + bounds.Width()*colSpec.start
	shaped := r.shaper.ShapeSimple(label, r.style.style)
	if shaped == nil || len(shaped.Lines) == 0 || len(shaped.Lines[0].Runs) == 0 {
		return nil
	}
	line := shaped.Lines[0]
	run := line.Runs[0]
	return gfx.DrawGlyphRun{
		Run: run,
		Origin: gfx.Point{
			X: x,
			Y: bounds.Min.Y + (bounds.Height()-line.Bounds.Height())*0.5,
		},
		Brush: gfx.SolidBrush(r.style.color),
	}
}

func (r *gridRow) Base() *facet.Facet             { r.BindImpl(r); return &r.Facet }
func (r *gridRow) OnAttach(_ facet.AttachContext) {}
func (r *gridRow) OnDetach()                      {}
func (r *gridRow) OnActivate()                    {}
func (r *gridRow) OnDeactivate()                  {}
