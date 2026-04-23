package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// SidebarFacet displays family navigation and filters.
type SidebarFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	th           theme.Context
	shaper       *text.Shaper
	subscription signal.SubscriptionID
}

// NewSidebarFacet creates a new sidebar facet.
func NewSidebarFacet(th theme.Context, shaper *text.Shaper) *SidebarFacet {
	s := &SidebarFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	s.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		w := c.MaxSize.W
		if w <= 0 {
			w = sidebarWidthDefault
		}
		return gfx.Size{W: w, H: c.MaxSize.H}
	}

	s.layout.OnArrange = func(bounds gfx.Rect) {
		s.layout.ArrangedBounds = bounds
	}
	s.AddRole(&s.layout)

	s.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		s.renderSidebar(list, bounds)
	}
	s.AddRole(&s.render)

	return s
}

// Base returns the base facet.
func (f *SidebarFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment and subscribes to stores.
func (f *SidebarFacet) OnAttach(ctx facet.AttachContext) {
	// Subscribe to filter changes
	f.subscription = store.FilterStore.OnChange.Subscribe(func(change signal.Change[store.FilterState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment and unsubscribes.
func (f *SidebarFacet) OnDetach() {
	store.FilterStore.OnChange.Unsubscribe(f.subscription)
}

// OnActivate handles activation.
func (f *SidebarFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *SidebarFacet) OnDeactivate() {}

func (f *SidebarFacet) renderSidebar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	// Right border
	borderBrush := f.th.Color(theme.ColorBorder)
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Max.X-1, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(borderBrush),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 12)
	if inner.IsEmpty() {
		return
	}

	filter := store.FilterStore.Get()
	y := inner.Min.Y

	// Section: Families
	y = f.renderSectionHeader(list, inner, y, "Families")

	for _, fam := range model.AllFamilies() {
		selected := filter.IsFamilySelected(fam)
		count := store.CatalogInstance.CountByFamily(fam)
		y = f.renderFamilyItem(list, inner, y, fam, selected, count)
		if y > inner.Max.Y {
			break
		}
	}

	// Section: Filter Options
	y += 16
	y = f.renderSectionHeader(list, inner, y, "Filters")
	y = f.renderFilterToggle(list, inner, y, "Interactive Only", filter.InteractiveOnly)
	y = f.renderFilterToggle(list, inner, y, "Theme Sensitive", filter.ThemeSensitiveOnly)

	// Section: Coverage Status
	y += 16
	y = f.renderSectionHeader(list, inner, y, "Coverage")
	y = f.renderCoverageFilter(list, inner, y, "Implemented", model.CoverageImplemented, filter)
	y = f.renderCoverageFilter(list, inner, y, "Partial", model.CoveragePartial, filter)
	y = f.renderCoverageFilter(list, inner, y, "Placeholder", model.CoveragePlaceholder, filter)
	y = f.renderCoverageFilter(list, inner, y, "Missing", model.CoverageMissing, filter)

	// Section: Search
	y += 16
	y = f.renderSectionHeader(list, inner, y, "Search")
	if filter.Query != "" {
		y = f.renderSearchQuery(list, inner, y, filter.Query)
	}
}

func (f *SidebarFacet) renderCoverageFilter(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, coverage model.CoverageStatus, filter store.FilterState) float32 {
	checked := filter.IsCoverageSelected(coverage)
	return f.renderFilterToggle(list, bounds, y, label, checked)
}

func (f *SidebarFacet) renderSearchQuery(list *gfx.CommandList, bounds gfx.Rect, y float32, query string) float32 {
	label := fmt.Sprintf("Query: %s", query)
	style := f.th.TextStyle(theme.TextBodyS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + 4
	}
	return y + 16
}

func (f *SidebarFacet) renderSectionHeader(list *gfx.CommandList, bounds gfx.Rect, y float32, label string) float32 {
	style := f.th.TextStyle(theme.TextLabelS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + 8
	}
	return y + 16
}

func (f *SidebarFacet) renderFamilyItem(list *gfx.CommandList, bounds gfx.Rect, y float32, fam model.Family, selected bool, count int) float32 {
	label := fam.DisplayName()
	token := theme.TextBodyS
	color := f.th.Color(theme.ColorText)
	if selected {
		color = f.th.Color(theme.ColorPrimary)
	}

	style := f.th.TextStyle(token)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		f.drawTextLine(list, bounds.Min.X+8, y, line, color)

		// Count badge
		countText := fmt.Sprintf("%d", count)
		countStyle := f.th.TextStyle(theme.TextLabelS)
		countLayout := f.shaper.ShapeSimple(countText, countStyle)
		if countLayout != nil && len(countLayout.Lines) > 0 {
			countLine := countLayout.Lines[0]
			countX := bounds.Max.X - countLine.Bounds.Width()
			f.drawTextLine(list, countX, y, countLine, f.th.Color(theme.ColorTextSecondary))
		}

		return y + layout.Bounds.Height() + 4
	}
	return y + 20
}

func (f *SidebarFacet) renderFilterToggle(list *gfx.CommandList, bounds gfx.Rect, y float32, label string, checked bool) float32 {
	// Checkbox indicator
	checkChar := "☐"
	if checked {
		checkChar = "☑"
	}

	checkStyle := f.th.TextStyle(theme.TextBodyS)
	checkLayout := f.shaper.ShapeSimple(checkChar, checkStyle)
	if checkLayout != nil && len(checkLayout.Lines) > 0 {
		checkLine := checkLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X, y, checkLine, f.th.Color(theme.ColorText))
	}

	// Label
	labelStyle := f.th.TextStyle(theme.TextBodyS)
	labelLayout := f.shaper.ShapeSimple(label, labelStyle)
	if labelLayout != nil && len(labelLayout.Lines) > 0 {
		labelLine := labelLayout.Lines[0]
		f.drawTextLine(list, bounds.Min.X+24, y, labelLine, f.th.Color(theme.ColorText))
		return y + labelLayout.Bounds.Height() + 8
	}
	return y + 24
}

func (f *SidebarFacet) drawTextLine(list *gfx.CommandList, x, y float32, line text.ShapedLine, color gfx.Color) {
	origin := gfx.Point{X: x + line.Bounds.Min.X, Y: y + line.Baseline}
	for _, run := range line.Runs {
		list.Add(gfx.DrawGlyphRun{
			Run:    run,
			Origin: origin,
			Brush:  gfx.SolidBrush(color),
		})
	}
}
