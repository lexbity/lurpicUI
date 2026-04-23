package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// SidebarFacet displays the scenario list and history.
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
		return gfx.Size{W: sidebarWidth, H: c.MaxSize.H}
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
	f.subscription = store.ScenarioRegistryStore.OnChange.Subscribe(func(change signal.Change[*store.ScenarioRegistry]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *SidebarFacet) OnDetach() {
	store.ScenarioRegistryStore.OnChange.Unsubscribe(f.subscription)
}

// OnActivate handles activation.
func (f *SidebarFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *SidebarFacet) OnDeactivate() {}

func (f *SidebarFacet) renderSidebar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Max.X-1, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorder)),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 12)
	if inner.IsEmpty() {
		return
	}

	y := inner.Min.Y

	sectionStyle := f.th.TextStyle(theme.TextLabelS)
	sectionLayout := f.shaper.ShapeSimple("Scenarios", sectionStyle)
	if sectionLayout != nil && len(sectionLayout.Lines) > 0 {
		line := sectionLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += sectionLayout.Bounds.Height() + 12
	}

	registry := store.ScenarioRegistryStore.Get()
	if registry == nil || registry.Count() == 0 {
		f.renderEmptyState(list, inner, &y)
		return
	}

	// Show load stats
	statsText := fmt.Sprintf("%d loaded, %d invalid", registry.ValidCount(), registry.InvalidCount())
	statsStyle := f.th.TextStyle(theme.TextLabelS)
	statsLayout := f.shaper.ShapeSimple(statsText, statsStyle)
	if statsLayout != nil && len(statsLayout.Lines) > 0 {
		line := statsLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += statsLayout.Bounds.Height() + 12
	}

	selectedID := store.SelectedScenarioStore.Get()
	for _, scenario := range registry.All() {
		if y > inner.Max.Y {
			break
		}
		y = f.renderScenarioItem(list, inner, y, scenario, scenario.ID == selectedID)
	}
}

func (f *SidebarFacet) renderEmptyState(list *gfx.CommandList, bounds gfx.Rect, y *float32) {
	emptyStyle := f.th.TextStyle(theme.TextBodyS)
	emptyLayout := f.shaper.ShapeSimple("No scenarios loaded", emptyStyle)
	if emptyLayout != nil && len(emptyLayout.Lines) > 0 {
		line := emptyLayout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X, Y: *y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
	}
}

func (f *SidebarFacet) renderScenarioItem(list *gfx.CommandList, bounds gfx.Rect, y float32, scenario *model.Scenario, selected bool) float32 {
	if scenario == nil {
		return y + 20
	}

	itemStyle := f.th.TextStyle(theme.TextBodyS)
	color := f.th.Color(theme.ColorText)
	if selected {
		color = f.th.Color(theme.ColorPrimary)
	}

	layout := f.shaper.ShapeSimple(scenario.DisplayName, itemStyle)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		x := bounds.Min.X + 8
		origin := gfx.Point{X: x, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(color),
			})
		}
		return y + layout.Bounds.Height() + 8
	}
	return y + 20
}
