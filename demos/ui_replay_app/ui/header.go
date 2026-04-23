package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/model"
	"codeburg.org/lexbit/ui_replay/store"
)

// HeaderFacet displays the top bar with scenario info, environment, and status.
type HeaderFacet struct {
	facet.Facet
	layout      facet.LayoutRole
	render      facet.RenderRole
	th          theme.Context
	shaper      *text.Shaper
	meta        model.BuildMetadata
	scenarioSub signal.SubscriptionID
	execSub     signal.SubscriptionID
}

// NewHeaderFacet creates a new header facet.
func NewHeaderFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *HeaderFacet {
	h := &HeaderFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
		meta:   meta,
	}

	h.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: headerHeight}
	}
	h.layout.OnArrange = func(bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
	}
	h.AddRole(&h.layout)

	h.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		h.renderHeader(list, bounds)
	}
	h.AddRole(&h.render)

	return h
}

// Base returns the base facet.
func (f *HeaderFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment and subscribes to stores.
func (f *HeaderFacet) OnAttach(ctx facet.AttachContext) {
	f.scenarioSub = store.SelectedScenarioStore.OnChange.Subscribe(func(change signal.Change[model.ScenarioID]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.execSub = store.ExecutionStateStore.OnChange.Subscribe(func(change signal.Change[store.ExecutionState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *HeaderFacet) OnDetach() {
	store.SelectedScenarioStore.OnChange.Unsubscribe(f.scenarioSub)
	store.ExecutionStateStore.OnChange.Unsubscribe(f.execSub)
}

// OnActivate handles activation.
func (f *HeaderFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *HeaderFacet) OnDeactivate() {}

func (f *HeaderFacet) renderHeader(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-1, bounds.Width(), 1),
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorder)),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 12)
	if inner.IsEmpty() {
		return
	}

	env := store.EnvironmentStore.Get()
	exec := store.ExecutionStateStore.Get()

	y := inner.Min.Y + 16

	titleStyle := f.th.TextStyle(theme.TextHeadingS)
	title := "UI Replay"
	if scenario, ok := store.SelectedScenario(); ok {
		title = scenario.DisplayName
	}
	titleLayout := f.shaper.ShapeSimple(title, titleStyle)
	if titleLayout != nil && len(titleLayout.Lines) > 0 {
		line := titleLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
			})
		}
	}

	x := inner.Max.X - 200

	metaStyle := f.th.TextStyle(theme.TextLabelS)
	metaText := env.Backend + " / " + env.Platform
	metaLayout := f.shaper.ShapeSimple(metaText, metaStyle)
	if metaLayout != nil && len(metaLayout.Lines) > 0 {
		line := metaLayout.Lines[0]
		origin := gfx.Point{X: x, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
	}

	x += 120
	statusStyle := f.th.TextStyle(theme.TextLabelS)
	statusText := executionStatusText(exec.Status)
	statusColor := executionStatusColor(f.th, exec.Status)
	statusLayout := f.shaper.ShapeSimple(statusText, statusStyle)
	if statusLayout != nil && len(statusLayout.Lines) > 0 {
		line := statusLayout.Lines[0]
		origin := gfx.Point{X: x, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(statusColor),
			})
		}
	}
}

func executionStatusText(status model.ExecutionStatus) string {
	if status == "" {
		return string(model.StatusPending)
	}
	return string(status)
}

func executionStatusColor(th theme.Context, status model.ExecutionStatus) gfx.Color {
	switch status {
	case model.StatusRunning:
		return th.Color(theme.ColorPrimary)
	case model.StatusPassed:
		return gfx.Color{R: 0.2, G: 0.8, B: 0.3, A: 1}
	case model.StatusCancelled:
		return gfx.Color{R: 0.95, G: 0.7, B: 0.2, A: 1}
	case model.StatusFailed, model.StatusError:
		return gfx.Color{R: 0.9, G: 0.3, B: 0.3, A: 1}
	default:
		return th.Color(theme.ColorTextSecondary)
	}
}
