package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_catalog/model"
	"codeburg.org/lexbit/ui_catalog/store"
)

// SidebarFacet displays family navigation and filters.
type SidebarFacet struct {
	facet.Facet
	layout        facet.LayoutRole
	render        facet.RenderRole
	input         facet.InputRole
	hit           facet.HitRole
	th            theme.Context
	shaper        *text.Shaper
	subscription  signal.SubscriptionID
	layoutProfile LayoutProfile
	itemRects     map[string]gfx.Rect
	itemActions   map[string]func()
	activeItem    string
}

// NewSidebarFacet creates a new sidebar facet.
func NewSidebarFacet(th theme.Context, shaper *text.Shaper) *SidebarFacet {
	s := &SidebarFacet{
		Facet:         facet.NewFacet(),
		th:            th,
		shaper:        shaper,
		layoutProfile: DefaultLayoutProfile(),
		itemRects:     make(map[string]gfx.Rect),
		itemActions:   make(map[string]func()),
	}

	s.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		profile := s.layoutProfile
		if profile.SidebarWidthDefault <= 0 {
			profile = DefaultLayoutProfile()
		}
		w := c.MaxSize.W
		if w <= 0 {
			w = profile.SidebarWidthDefault
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

	s.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if !s.layout.ArrangedBounds.Contains(p) {
			return facet.HitResult{}
		}
		cursor := facet.CursorDefault
		if s.hitActionAt(p) != "" {
			cursor = facet.CursorPointer
		}
		return facet.HitResult{Hit: true, Cursor: cursor}
	}
	s.AddRole(&s.hit)

	s.input.OnPointer = func(e facet.PointerEvent) bool {
		switch e.Kind {
		case platform.PointerPress:
			if key := s.hitActionAt(e.Position); key != "" {
				s.activeItem = key
				return true
			}
		case platform.PointerRelease:
			key := s.hitActionAt(e.Position)
			if key != "" && key == s.activeItem {
				if action := s.itemActions[key]; action != nil {
					action()
				}
			}
			s.activeItem = ""
			if key != "" {
				return true
			}
		}
		return false
	}
	s.AddRole(&s.input)

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

// SetLayoutProfile updates density-driven sidebar geometry.
func (f *SidebarFacet) SetLayoutProfile(profile LayoutProfile) {
	if f == nil {
		return
	}
	f.layoutProfile = profile
	f.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

func (f *SidebarFacet) renderSidebar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	if f.itemRects == nil {
		f.itemRects = make(map[string]gfx.Rect)
	}
	if f.itemActions == nil {
		f.itemActions = make(map[string]func())
	}
	for k := range f.itemRects {
		delete(f.itemRects, k)
	}
	for k := range f.itemActions {
		delete(f.itemActions, k)
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

	profile := f.layoutProfile
	if profile.SidebarInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	inner := Inset(bounds, profile.SidebarInset)
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
		key := "family:" + fam.String()
		y = f.renderFamilyItem(list, inner, y, key, fam, selected, count)
		f.registerItemAction(key, func(fam model.Family) func() {
			return func() { store.ToggleFamily(fam) }
		}(fam))
		if y > inner.Max.Y {
			break
		}
	}

	// Section: Filter Options
	y += profile.FieldGap * 4
	y = f.renderSectionHeader(list, inner, y, "Filters")
	y = f.renderFilterToggle(list, inner, y, "filter:interactive", "Interactive Only", filter.InteractiveOnly)
	f.registerItemAction("filter:interactive", func() { store.SetInteractiveOnly(!filter.InteractiveOnly) })
	y = f.renderFilterToggle(list, inner, y, "filter:theme", "Theme Sensitive", filter.ThemeSensitiveOnly)
	f.registerItemAction("filter:theme", func() { store.SetThemeSensitiveOnly(!filter.ThemeSensitiveOnly) })

	// Section: Coverage Status
	y += profile.FieldGap * 4
	y = f.renderSectionHeader(list, inner, y, "Coverage")
	y = f.renderCoverageFilter(list, inner, y, "coverage:implemented", "Implemented", model.CoverageImplemented, filter)
	f.registerItemAction("coverage:implemented", func() { store.ToggleCoverageFilter(model.CoverageImplemented) })
	y = f.renderCoverageFilter(list, inner, y, "coverage:partial", "Partial", model.CoveragePartial, filter)
	f.registerItemAction("coverage:partial", func() { store.ToggleCoverageFilter(model.CoveragePartial) })
	y = f.renderCoverageFilter(list, inner, y, "coverage:placeholder", "Placeholder", model.CoveragePlaceholder, filter)
	f.registerItemAction("coverage:placeholder", func() { store.ToggleCoverageFilter(model.CoveragePlaceholder) })
	y = f.renderCoverageFilter(list, inner, y, "coverage:missing", "Missing", model.CoverageMissing, filter)
	f.registerItemAction("coverage:missing", func() { store.ToggleCoverageFilter(model.CoverageMissing) })
	y = f.renderCoverageFilter(list, inner, y, "coverage:theme", "Theme Dependent", model.CoverageThemeDependent, filter)
	f.registerItemAction("coverage:theme", func() { store.ToggleCoverageFilter(model.CoverageThemeDependent) })
	y = f.renderCoverageFilter(list, inner, y, "coverage:layout", "Layout Dependent", model.CoverageLayoutDependent, filter)
	f.registerItemAction("coverage:layout", func() { store.ToggleCoverageFilter(model.CoverageLayoutDependent) })

	// Section: Search
	y += profile.FieldGap * 4
	y = f.renderSectionHeader(list, inner, y, "Search")
	if filter.Query != "" {
		y = f.renderSearchQuery(list, inner, y, filter.Query)
	}
}

func (f *SidebarFacet) renderCoverageFilter(list *gfx.CommandList, bounds gfx.Rect, y float32, key, label string, coverage model.CoverageStatus, filter store.FilterState) float32 {
	checked := filter.IsCoverageSelected(coverage)
	return f.renderFilterToggle(list, bounds, y, key, label, checked)
}

func (f *SidebarFacet) renderSearchQuery(list *gfx.CommandList, bounds gfx.Rect, y float32, query string) float32 {
	profile := f.layoutProfile
	if profile.SidebarInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	label := fmt.Sprintf("Query: %s", query)
	style := f.th.TextStyle(theme.TextBodyS)
	layout := f.shaper.ShapeSimple(label, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, line, f.th.Color(theme.ColorTextSecondary))
		return y + layout.Bounds.Height() + profile.FieldGap
	}
	return y + profile.FieldGap*4
}

func (f *SidebarFacet) renderSectionHeader(list *gfx.CommandList, bounds gfx.Rect, y float32, label string) float32 {
	profile := f.layoutProfile
	if profile.SidebarInset <= 0 {
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

func (f *SidebarFacet) renderFamilyItem(list *gfx.CommandList, bounds gfx.Rect, y float32, key string, fam model.Family, selected bool, count int) float32 {
	profile := f.layoutProfile
	if profile.SidebarInset <= 0 {
		profile = DefaultLayoutProfile()
	}
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
		drawTextLine(list, bounds.Min.X+profile.SidebarInset/2, y, line, color)

		// Count badge
		countText := fmt.Sprintf("%d", count)
		countStyle := f.th.TextStyle(theme.TextLabelS)
		countLayout := f.shaper.ShapeSimple(countText, countStyle)
		if countLayout != nil && len(countLayout.Lines) > 0 {
			countLine := countLayout.Lines[0]
			countX := bounds.Max.X - countLine.Bounds.Width()
			drawTextLine(list, countX, y, countLine, f.th.Color(theme.ColorTextSecondary))
		}

		f.registerItemRect(key, gfx.RectFromXYWH(bounds.Min.X, y-2, bounds.Width(), layout.Bounds.Height()+4))

		return y + layout.Bounds.Height() + profile.FieldGap
	}
	return y + profile.FieldGap*5
}

func (f *SidebarFacet) renderFilterToggle(list *gfx.CommandList, bounds gfx.Rect, y float32, key, label string, checked bool) float32 {
	profile := f.layoutProfile
	if profile.SidebarInset <= 0 {
		profile = DefaultLayoutProfile()
	}
	// Checkbox indicator
	checkChar := "☐"
	if checked {
		checkChar = "☑"
	}

	checkStyle := f.th.TextStyle(theme.TextBodyS)
	checkLayout := f.shaper.ShapeSimple(checkChar, checkStyle)
	if checkLayout != nil && len(checkLayout.Lines) > 0 {
		checkLine := checkLayout.Lines[0]
		drawTextLine(list, bounds.Min.X, y, checkLine, f.th.Color(theme.ColorText))
	}

	// Label
	labelStyle := f.th.TextStyle(theme.TextBodyS)
	labelLayout := f.shaper.ShapeSimple(label, labelStyle)
	if labelLayout != nil && len(labelLayout.Lines) > 0 {
		labelLine := labelLayout.Lines[0]
		drawTextLine(list, bounds.Min.X+profile.FieldLabelWidth/4, y, labelLine, f.th.Color(theme.ColorText))
		itemHeight := labelLayout.Bounds.Height()
		if checkLayout != nil && len(checkLayout.Lines) > 0 && checkLayout.Bounds.Height() > itemHeight {
			itemHeight = checkLayout.Bounds.Height()
		}
		f.registerItemRect(key, gfx.RectFromXYWH(bounds.Min.X, y-2, bounds.Width(), itemHeight+4))
		return y + labelLayout.Bounds.Height() + 8
	}
	return y + profile.FieldGap*6
}

func (f *SidebarFacet) registerItemRect(key string, rect gfx.Rect) {
	if key == "" {
		return
	}
	if f.itemRects == nil {
		f.itemRects = make(map[string]gfx.Rect)
	}
	f.itemRects[key] = rect
}

func (f *SidebarFacet) registerItemAction(key string, action func()) {
	if key == "" {
		return
	}
	if f.itemActions == nil {
		f.itemActions = make(map[string]func())
	}
	f.itemActions[key] = action
}

func (f *SidebarFacet) hitActionAt(p gfx.Point) string {
	for key, rect := range f.itemRects {
		if rect.Contains(p) {
			return key
		}
	}
	return ""
}
