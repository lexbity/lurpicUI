package ui

import (
	"strings"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// InspectorFacet displays metadata for the selected entry.
type InspectorFacet struct {
	facet.Facet
	layout        facet.LayoutRole
	render        facet.RenderRole
	th            theme.Context
	shaper        *text.Shaper
	subscription  signal.SubscriptionID
	layoutProfile LayoutProfile
}

// NewInspectorFacet creates a new inspector facet.
func NewInspectorFacet(th theme.Context, shaper *text.Shaper) *InspectorFacet {
	i := &InspectorFacet{
		Facet:         facet.NewFacet(),
		th:            th,
		shaper:        shaper,
		layoutProfile: DefaultLayoutProfile(),
	}

	i.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		profile := i.layoutProfile
		if profile.InspectorWidthDefault <= 0 {
			profile = DefaultLayoutProfile()
		}
		w := c.MaxSize.W
		if w <= 0 {
			w = profile.InspectorWidthDefault
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

// SetLayoutProfile updates density-driven inspector geometry.
func (f *InspectorFacet) SetLayoutProfile(profile LayoutProfile) {
	if f == nil {
		return
	}
	f.layoutProfile = profile
	f.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

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

	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	inner := Inset(bounds, profile.InspectorInset)
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
	y = f.renderProperty(list, inner, y, "Logical ID", entry.ID)

	// Display Name
	y = f.renderProperty(list, inner, y, "Name", entry.DisplayName)

	// Family
	y = f.renderProperty(list, inner, y, "Family", entry.Family.DisplayName())

	// Subcategory
	y = f.renderProperty(list, inner, y, "Subcategory", entry.Subcategory)

	// Construction class
	y = f.renderProperty(list, inner, y, "Construction", entry.ConstructionClass.String())

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

	// Layout Sensitive
	layoutSensitive := "No"
	if entry.LayoutSensitive {
		layoutSensitive = "Yes"
	}
	y = f.renderProperty(list, inner, y, "Layout Sensitive", layoutSensitive)

	filterState := store.FilterStore.Get()
	if filterState.ShowVariants {
		y += profile.FieldGap * 2
		y = f.renderInventoryMatrix(list, inner, y, "Variants", entry.Variants, entry.MissingVariants, entry.UnsupportedVariants)
	}
	if filterState.ShowStates {
		y += profile.FieldGap * 2
		y = f.renderInventoryMatrixStates(list, inner, y, "States", entry.States, entry.MissingStates, entry.UnsupportedStates)
	}

	// Notes if present
	if entry.Notes != "" {
		y += profile.FieldGap * 2
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
		drawTextLine(list, x, y, line, f.th.Color(theme.ColorTextSecondary))
	}
}

func (f *InspectorFacet) renderSectionHeader(list *gfx.CommandList, bounds gfx.Rect, y float32, label string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	style := f.th.TextStyle(theme.TextLabelS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + profile.FieldGap*2
	}
	return y + profile.FieldGap*4
}

func (f *InspectorFacet) renderProperty(list *gfx.CommandList, bounds gfx.Rect, y float32, name, value string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	// Name
	nameStyle := f.th.TextStyle(theme.TextLabelS)
	nameLayout := f.shaper.ShapeSimple(name+":", nameStyle)
	if nameLayout != nil && len(nameLayout.Lines) > 0 {
		line := nameLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		nameHeight := nameLayout.Bounds.Height()

		// Value
		valueStyle := f.th.TextStyle(theme.TextBodyS)
		valueLayout := f.shaper.ShapeSimple(value, valueStyle)
		if valueLayout != nil && len(valueLayout.Lines) > 0 {
			vLine := valueLayout.Lines[0]
			x := bounds.Min.X + profile.FieldLabelWidth
			drawTextLine(list, x, y, vLine, f.th.Color(theme.ColorText))
			if valueLayout.Bounds.Height() > nameHeight {
				return y + valueLayout.Bounds.Height() + profile.FieldGap
			}
		}
		return y + nameHeight + profile.FieldGap
	}
	return y + profile.FieldGap*4
}

func (f *InspectorFacet) renderInventoryMatrix(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, variants []model.Variant, missing, unsupported []string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}

	style := f.th.TextStyle(theme.TextLabelS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += layout.Bounds.Height() + profile.FieldGap
	}

	if len(variants) == 0 {
		return f.renderMatrixNotice(list, bounds, y, "None recorded")
	}

	for _, variant := range variants {
		y = f.renderMatrixItem(list, bounds, y, variant.Label, variant.ID, theme.TextBodyS)
	}
	y = f.renderMatrixNotes(list, bounds, y, "Missing", missing)
	y = f.renderMatrixNotes(list, bounds, y, "Unsupported", unsupported)
	return y
}

func (f *InspectorFacet) renderInventoryMatrixStates(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, states []model.State, missing, unsupported []string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}

	style := f.th.TextStyle(theme.TextLabelS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		y += layout.Bounds.Height() + profile.FieldGap
	}

	if len(states) == 0 {
		return f.renderMatrixNotice(list, bounds, y, "None recorded")
	}

	for _, state := range states {
		y = f.renderMatrixItem(list, bounds, y, state.Label, state.ID, theme.TextBodyS)
	}
	y = f.renderMatrixNotes(list, bounds, y, "Missing", missing)
	y = f.renderMatrixNotes(list, bounds, y, "Unsupported", unsupported)
	return y
}

func (f *InspectorFacet) renderMatrixNotice(list *gfx.CommandList, bounds gfx.Rect, y float32, text string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	layout := f.shaper.ShapeSimple(text, f.th.TextStyle(theme.TextBodyS))
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.FieldLabelWidth/4, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + profile.FieldGap
	}
	return y + profile.FieldGap*4
}

func (f *InspectorFacet) renderMatrixItem(list *gfx.CommandList, bounds gfx.Rect, y float32, label, id string, token theme.TextToken) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	text := label
	if id != "" {
		text = text + " (" + id + ")"
	}
	layout := f.shaper.ShapeSimple(text, f.th.TextStyle(token))
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.FieldLabelWidth/4, y, line, f.th.Color(theme.ColorText))
		return y + layout.Bounds.Height() + profile.FieldGap
	}
	return y + profile.FieldGap*4
}

func (f *InspectorFacet) renderMatrixNotes(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, items []string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	if len(items) == 0 {
		return y
	}
	text := label + ": " + strings.Join(items, ", ")
	layout := f.shaper.ShapeSimple(text, f.th.TextStyle(theme.TextBodyS))
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.FieldLabelWidth/4, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + profile.FieldGap
	}
	return y + profile.FieldGap*4
}

func (f *InspectorFacet) renderWrappedText(list *gfx.CommandList, bounds gfx.Rect, y float32, text string) float32 {
	profile := f.layoutProfile
	if profile.InspectorInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	style := f.th.TextStyle(theme.TextBodyS)
	layout := f.shaper.ShapeSimple(text, style)
	if layout == nil {
		return y
	}
	for _, line := range layout.Lines {
		if y > bounds.Max.Y {
			break
		}
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorText))
		y += layout.Bounds.Height() + profile.FieldGap
	}
	return y
}
