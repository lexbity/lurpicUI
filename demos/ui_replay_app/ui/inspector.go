package ui

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_replay/store"
)

// InspectorFacet displays the diagnostics summary panel.
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
		return gfx.Size{W: inspectorWidth, H: c.MaxSize.H}
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

// OnAttach handles attachment and subscribes to stores.
func (f *InspectorFacet) OnAttach(ctx facet.AttachContext) {
	f.subscription = store.ExecutionStateStore.OnChange.Subscribe(func(change signal.Change[store.ExecutionState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *InspectorFacet) OnDetach() {
	store.ExecutionStateStore.OnChange.Unsubscribe(f.subscription)
}

// OnActivate handles activation.
func (f *InspectorFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *InspectorFacet) OnDeactivate() {}

func (f *InspectorFacet) renderInspector(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, 1, bounds.Height()),
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
	diagLayout := f.shaper.ShapeSimple("Diagnostics", sectionStyle)
	if diagLayout != nil && len(diagLayout.Lines) > 0 {
		line := diagLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += diagLayout.Bounds.Height() + 12
	}

	exec := store.ExecutionStateStore.Get()

	labels := []struct {
		label string
		value string
	}{
		{"Status:", string(exec.Status)},
		{"Step:", fmt.Sprintf("%d/%d", exec.CurrentStep, exec.TotalSteps)},
		{"Progress:", fmt.Sprintf("%.0f%%", exec.Progress*100)},
		{"Assertions:", fmt.Sprintf("%d total / %d failed", exec.AssertionCount(), exec.AssertionFailures())},
	}

	bodyStyle := f.th.TextStyle(theme.TextBodyS)
	for _, item := range labels {
		if y > inner.Max.Y {
			break
		}
		text := item.label + " " + item.value
		textLayout := f.shaper.ShapeSimple(text, bodyStyle)
		if textLayout != nil && len(textLayout.Lines) > 0 {
			line := textLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
				})
			}
			y += textLayout.Bounds.Height() + 8
		}
	}

	if len(exec.AssertionResults) > 0 && y <= inner.Max.Y {
		last := exec.AssertionResults[len(exec.AssertionResults)-1]
		lastText := fmt.Sprintf("Last: step %d %s %v", last.Step, last.Type, last.Passed)
		lastLayout := f.shaper.ShapeSimple(lastText, bodyStyle)
		if lastLayout != nil && len(lastLayout.Lines) > 0 {
			line := lastLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X, Y: y + 8}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
				})
			}
			y += lastLayout.Bounds.Height() + 8
		}
		if !last.Passed && last.Reason != "" && y <= inner.Max.Y {
			reasonLayout := f.shaper.ShapeSimple(last.Reason, bodyStyle)
			if reasonLayout != nil && len(reasonLayout.Lines) > 0 {
				line := reasonLayout.Lines[0]
				origin := gfx.Point{X: inner.Min.X, Y: y}
				for _, run := range line.Runs {
					list.Add(gfx.DrawGlyphRun{
						Run:    run,
						Origin: origin,
						Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
					})
				}
			}
		}
	}
}
