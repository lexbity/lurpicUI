package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/store"
)

// InspectorFacet displays metadata for the selected entry.
type InspectorFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	th           theme.Context
	shaper       *text.Shaper
	subscription signal.SubscriptionID
}

// NewInspectorFacet creates a new inspector facet.
func NewInspectorFacet(th theme.Context, shaper *text.Shaper) *InspectorFacet {
	i := &InspectorFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	i.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		w := c.MaxSize.W
		if w <= 0 {
			w = inspectorWidthDefault
		}
		return gfx.Size{W: w, H: c.MaxSize.H}
	}

	i.layout.OnArrange = func(bounds gfx.Rect) {
		i.layout.ArrangedBounds = bounds
	}
	i.AddRole(&i.layout)

	i.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		i.renderInspector(list, bounds)
	}
	i.AddRole(&i.render)

	return i
}

// Base returns the base facet.
func (f *InspectorFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment.
func (f *InspectorFacet) OnAttach(ctx facet.AttachContext) {
	f.subscription = store.SelectionStore.OnChange.Subscribe(func(change signal.Change[string]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *InspectorFacet) OnDetach() {
	store.SelectionStore.OnChange.Unsubscribe(f.subscription)
}

// OnActivate handles activation.
func (f *InspectorFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *InspectorFacet) OnDeactivate() {}

func (f *InspectorFacet) renderInspector(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	// Left border
	borderBrush := f.th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(borderBrush),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 12)
	if inner.IsEmpty() {
		return
	}

	// Get selected entry
	entry, ok := store.SelectedEntry(store.CatalogInstance)
	if !ok {
		f.renderNoSelection(list, inner)
		return
	}

	// Header
	y := inner.Min.Y
	y = f.renderSectionHeader(list, inner, y, "Details")

	// Entry ID
	y = f.renderProperty(list, inner, y, "ID", entry.ID)

	// Display Name
	y = f.renderProperty(list, inner, y, "Name", entry.DisplayName)

	// Family
	y = f.renderProperty(list, inner, y, "Family", entry.Family.DisplayName())

	// Coverage
	y = f.renderProperty(list, inner, y, "Coverage", entry.Coverage.DisplayName())

	// Interactive
	interactive := "No"
	if entry.Interactive {
		interactive = "Yes"
	}
	y = f.renderProperty(list, inner, y, "Interactive", interactive)

	// Theme Sensitive
	themeSensitive := "No"
	if entry.ThemeSensitive {
		themeSensitive = "Yes"
	}
	y = f.renderProperty(list, inner, y, "Theme Sensitive", themeSensitive)

	// Notes if present
	if entry.Notes != "" {
		y += 8
		y = f.renderSectionHeader(list, inner, y, "Notes")
		y = f.renderWrappedText(list, inner, y, entry.Notes)
	}
}

func (f *InspectorFacet) renderNoSelection(list *gfx.CommandList, bounds gfx.Rect) {
	msg := "Select an entry to view details"
	msgStyle := f.th.TextStyle(theme.TextBodyS)
	msgLayout := f.shaper.ShapeSimple(msg, msgStyle)
	if msgLayout != nil && len(msgLayout.Lines) > 0 {
		line := msgLayout.Lines[0]
		x := bounds.Min.X + (bounds.Width()-line.Bounds.Width())/2
		y := bounds.Min.Y + (bounds.Height()-msgLayout.Bounds.Height())/2
		f.drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *InspectorFacet) renderSectionHeader(list *gfx.CommandList, bounds gfx.Rect, y float32, label string) float32 {
	style := f.th.TextStyle(theme.TextLabelS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + 8
	}
	return y + 16
}

func (f *InspectorFacet) renderProperty(list *gfx.CommandList, bounds gfx.Rect, y float32, name, value string) float32 {
	// Name
	nameStyle := f.th.TextStyle(theme.TextLabelS)
	nameLayout := f.shaper.ShapeSimple(name+":", nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		nameHeight := nameLayout.Bounds.Height()

		// Value
		valueStyle := f.th.TextStyle(theme.TextBodyS)
		valueLayout := f.shaper.ShapeSimple(value, valueStyle)
		if valueLayout != nil && len(valueLayout.Lines) > 0 {
			vLine := valueLayout.Lines[0]
			x := bounds.Min.X + 80
			f.drawTextLine(list, x, y, vLine, f.th.Color(theme.ColorText))
			if valueLayout.Bounds.Height() > nameHeight {
				return y + valueLayout.Bounds.Height() + 4
			}
		}
		return y + nameHeight + 4
	}
	return y + 16
}

func (f *InspectorFacet) renderWrappedText(list *gfx.CommandList, bounds gfx.Rect, y float32, text string) float32 {
	style := f.th.TextStyle(theme.TextBodyS)
	layout := f.shaper.ShapeSimple(text, style)
	if layout == nil {
		return y
	}
	for _, line := range layout.Lines {
		if y > bounds.Max.Y {
			break
		}
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorText))
		y += layout.Bounds.Height()
	}
	return y
}

func (f *InspectorFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  gfx.SolidBrush(color),
		})
	}
}
