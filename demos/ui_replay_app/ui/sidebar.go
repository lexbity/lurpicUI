package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
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
	input        facet.InputRole
	th           theme.Context
	shaper       *text.Shaper
	registrySub  signal.SubscriptionID
	selectionSub signal.SubscriptionID
	execSub      signal.SubscriptionID
	historySub   signal.SubscriptionID
	itemRects    map[string]gfx.Rect
	activeID     string
}

// NewSidebarFacet creates a new sidebar facet.
func NewSidebarFacet(th theme.Context, shaper *text.Shaper) *SidebarFacet {
	s := &SidebarFacet{
		Facet:     facet.NewFacet(),
		th:        th,
		shaper:    shaper,
		itemRects: make(map[string]gfx.Rect),
	}

	s.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: sidebarWidth, H: c.MaxSize.H}
	}
	s.layout.OnArrange = func(bounds gfx.Rect) {
		s.layout.ArrangedBounds = bounds
	}
	s.AddRole(&s.layout)

	s.input.OnPointer = func(e facet.PointerEvent) bool {
		switch e.Kind {
		case platform.PointerPress:
			id := s.hitScenarioAt(e.Position)
			if id != "" {
				s.activeID = id
				return true
			}
		case platform.PointerRelease:
			id := s.hitScenarioAt(e.Position)
			if id != "" && id == s.activeID {
				store.SelectScenario(model.ScenarioID(id))
			}
			s.activeID = ""
			if id != "" {
				return true
			}
		}
		return false
	}
	s.AddRole(&s.input)

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
	f.registrySub = store.ScenarioRegistryStore.OnChange.Subscribe(func(change signal.Change[*store.ScenarioRegistry]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.selectionSub = store.SelectedScenarioStore.OnChange.Subscribe(func(change signal.Change[model.ScenarioID]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.execSub = store.ExecutionStateStore.OnChange.Subscribe(func(change signal.Change[store.ExecutionState]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.historySub = store.RunHistoryStore.OnChange.Subscribe(func(change signal.Change[*store.RunHistory]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *SidebarFacet) OnDetach() {
	store.ScenarioRegistryStore.OnChange.Unsubscribe(f.registrySub)
	store.SelectedScenarioStore.OnChange.Unsubscribe(f.selectionSub)
	store.ExecutionStateStore.OnChange.Unsubscribe(f.execSub)
	store.RunHistoryStore.OnChange.Unsubscribe(f.historySub)
}

// OnActivate handles activation.
func (f *SidebarFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *SidebarFacet) OnDeactivate() {}

func (f *SidebarFacet) renderSidebar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}
	if f.itemRects == nil {
		f.itemRects = make(map[string]gfx.Rect)
	}
	for k := range f.itemRects {
		delete(f.itemRects, k)
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

	// Render run history section
	y += 20 // Spacer
	f.renderRunHistory(list, inner, &y)
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
	rect := gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), 36)
	if f.itemRects != nil {
		f.itemRects[string(scenario.ID)] = rect
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
		origin := gfx.Point{X: x, Y: y + 20}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(color),
			})
		}
		return y + 36
	}
	return y + 36
}

func (f *SidebarFacet) hitScenarioAt(p gfx.Point) string {
	for id, rect := range f.itemRects {
		if rect.Contains(p) {
			return id
		}
	}
	return ""
}

func (f *SidebarFacet) renderRunHistory(list *gfx.CommandList, bounds gfx.Rect, y *float32) {
	if *y > bounds.Max.Y {
		return
	}

	sectionStyle := f.th.TextStyle(theme.TextLabelS)
	headerLayout := f.shaper.ShapeSimple("Run History", sectionStyle)
	if headerLayout != nil && len(headerLayout.Lines) > 0 {
		line := headerLayout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X, Y: *y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		*y += headerLayout.Bounds.Height() + 12
	}

	history := store.RunHistoryStore.Get()
	if history == nil || history.Count() == 0 {
		emptyStyle := f.th.TextStyle(theme.TextBodyS)
		emptyLayout := f.shaper.ShapeSimple("No runs yet", emptyStyle)
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
		return
	}

	// Show recent runs (up to 5)
	bodyStyle := f.th.TextStyle(theme.TextBodyS)
	runs := history.All()
	count := 0
	for _, run := range runs {
		if *y > bounds.Max.Y || count >= 5 {
			break
		}

		statusIcon := "○"
		color := f.th.Color(theme.ColorTextSecondary)
		switch run.Status {
		case model.StatusPassed:
			statusIcon = "✓"
			color = gfx.Color{R: 0.2, G: 0.8, B: 0.3, A: 1}
		case model.StatusFailed, model.StatusError:
			statusIcon = "✗"
			color = gfx.Color{R: 0.9, G: 0.3, B: 0.3, A: 1}
		case model.StatusRunning:
			statusIcon = "▶"
			color = f.th.Color(theme.ColorPrimary)
		}

		scenarioName := string(run.ScenarioID)
		if len(scenarioName) > 20 {
			scenarioName = scenarioName[:20] + "..."
		}
		runText := fmt.Sprintf("%s %s (%s)", statusIcon, scenarioName, run.Status)

		runLayout := f.shaper.ShapeSimple(runText, bodyStyle)
		if runLayout != nil && len(runLayout.Lines) > 0 {
			line := runLayout.Lines[0]
			origin := gfx.Point{X: bounds.Min.X, Y: *y}
			for _, r := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    r,
					Origin: origin,
					Brush:  gfx.SolidBrush(color),
				})
			}
			*y += runLayout.Bounds.Height() + 4
		}
		count++
	}
}
