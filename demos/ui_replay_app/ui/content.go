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

// ContentFacet displays the main scenario content viewport.
type ContentFacet struct {
	facet.Facet
	layout      facet.LayoutRole
	render      facet.RenderRole
	th          theme.Context
	shaper      *text.Shaper
	scenarioSub signal.SubscriptionID
	execSub     signal.SubscriptionID
}

// NewContentFacet creates a new content facet.
func NewContentFacet(th theme.Context, shaper *text.Shaper) *ContentFacet {
	c := &ContentFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	c.layout.OnMeasure = func(constraints facet.Constraints) gfx.Size {
		return gfx.Size{W: constraints.MaxSize.W, H: constraints.MaxSize.H}
	}
	c.layout.OnArrange = func(bounds gfx.Rect) {
		c.layout.ArrangedBounds = bounds
	}
	c.AddRole(&c.layout)

	c.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		c.renderContent(list, bounds)
	}
	c.AddRole(&c.render)

	return c
}

// Base returns the base facet.
func (f *ContentFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment and subscribes to stores.
func (f *ContentFacet) OnAttach(ctx facet.AttachContext) {
	f.scenarioSub = store.SelectedScenarioStore.OnChange.Subscribe(func(change signal.Change[model.ScenarioID]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.execSub = store.ExecutionStateStore.OnChange.Subscribe(func(change signal.Change[store.ExecutionState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *ContentFacet) OnDetach() {
	store.SelectedScenarioStore.OnChange.Unsubscribe(f.scenarioSub)
	store.ExecutionStateStore.OnChange.Unsubscribe(f.execSub)
}

// OnActivate handles activation.
func (f *ContentFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *ContentFacet) OnDeactivate() {}

func (f *ContentFacet) renderContent(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorBackground)),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 24)
	if inner.IsEmpty() {
		return
	}

	scenario, ok := store.SelectedScenario()
	if !ok {
		emptyStyle := f.th.TextStyle(theme.TextHeadingS)
		emptyLayout := f.shaper.ShapeSimple("No scenario selected", emptyStyle)
		if emptyLayout != nil && len(emptyLayout.Lines) > 0 {
			line := emptyLayout.Lines[0]
			x := inner.Min.X + (inner.Width()-line.Bounds.Width())/2
			y := inner.Min.Y + inner.Height()/3
			origin := gfx.Point{X: x, Y: y}
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

	y := inner.Min.Y

	titleStyle := f.th.TextStyle(theme.TextHeadingS)
	titleLayout := f.shaper.ShapeSimple(scenario.DisplayName, titleStyle)
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
		y += titleLayout.Bounds.Height() + 8
	}

	idStyle := f.th.TextStyle(theme.TextBodyS)
	idText := "ID: " + string(scenario.ID) + " | Schema: " + scenario.Schema
	idLayout := f.shaper.ShapeSimple(idText, idStyle)
	if idLayout != nil && len(idLayout.Lines) > 0 {
		line := idLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += idLayout.Bounds.Height() + 24
	}

	if scenario.ExpectedState != nil {
		expectedText := fmt.Sprintf(
			"Expected: scene=%s theme=%s density=%s",
			scenario.ExpectedState.SceneID,
			scenario.ExpectedState.Theme,
			scenario.ExpectedState.Density,
		)
		expectedLayout := f.shaper.ShapeSimple(expectedText, idStyle)
		if expectedLayout != nil && len(expectedLayout.Lines) > 0 {
			line := expectedLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
				})
			}
			y += expectedLayout.Bounds.Height() + 16
		}
	}

	sectionStyle := f.th.TextStyle(theme.TextLabelS)
	bodyStyle := f.th.TextStyle(theme.TextBodyS)

	// Show execution state with progress
	exec := store.ExecutionStateStore.Get()
	if exec.IsRunning() || exec.Status == model.StatusPassed || exec.Status == model.StatusFailed {
		progressText := fmt.Sprintf("Progress: Step %d/%d (%.0f%%)", exec.CurrentStep, exec.TotalSteps, exec.Progress*100)
		progressLayout := f.shaper.ShapeSimple(progressText, sectionStyle)
		if progressLayout != nil && len(progressLayout.Lines) > 0 {
			line := progressLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorPrimary)),
				})
			}
			y += progressLayout.Bounds.Height() + 8
		}

		// Show current action if running
		if exec.IsRunning() && exec.CurrentAction != "" {
			actionText := fmt.Sprintf("Current: %s", exec.CurrentAction)
			actionLayout := f.shaper.ShapeSimple(actionText, bodyStyle)
			if actionLayout != nil && len(actionLayout.Lines) > 0 {
				line := actionLayout.Lines[0]
				origin := gfx.Point{X: inner.Min.X, Y: y}
				for _, run := range line.Runs {
					list.Add(gfx.DrawGlyphRun{
						Run:    run,
						Origin: origin,
						Brush:  gfx.SolidBrush(f.th.Color(theme.ColorPrimary)),
					})
				}
				y += actionLayout.Bounds.Height() + 12
			}
		}

		y += 8 // Spacer
	}

	actionHeader := f.shaper.ShapeSimple("Actions:", sectionStyle)
	if actionHeader != nil && len(actionHeader.Lines) > 0 {
		line := actionHeader.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += actionHeader.Bounds.Height() + 8
	}

	for i, action := range scenario.Actions {
		if y > inner.Max.Y {
			break
		}

		// Determine if this is the current step
		isCurrentStep := exec.IsRunning() && exec.CurrentStep == i+1
		isPastStep := exec.CurrentStep > i+1

		// Status indicator
		statusIcon := "○"
		color := f.th.Color(theme.ColorText)
		if isPastStep {
			statusIcon = "✓"
			color = gfx.Color{R: 0.4, G: 0.7, B: 0.4, A: 1}
		} else if isCurrentStep {
			statusIcon = "▶"
			color = f.th.Color(theme.ColorPrimary)
		}

		actionText := fmt.Sprintf("%s %d. %s", statusIcon, i+1, action.Type)
		itemLayout := f.shaper.ShapeSimple(actionText, bodyStyle)
		if itemLayout != nil && len(itemLayout.Lines) > 0 {
			line := itemLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X + 16, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(color),
				})
			}
			y += itemLayout.Bounds.Height() + 4
		}
	}

	if len(exec.AssertionResults) > 0 && y <= inner.Max.Y {
		y += 12
		assertionHeader := f.shaper.ShapeSimple("Assertions", sectionStyle)
		if assertionHeader != nil && len(assertionHeader.Lines) > 0 {
			line := assertionHeader.Lines[0]
			origin := gfx.Point{X: inner.Min.X, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
				})
			}
			y += assertionHeader.Bounds.Height() + 8
		}
		limit := len(exec.AssertionResults)
		if limit > 3 {
			limit = 3
		}
		for i := len(exec.AssertionResults) - limit; i < len(exec.AssertionResults); i++ {
			if i < 0 || y > inner.Max.Y {
				continue
			}
			result := exec.AssertionResults[i]
			lineText := fmt.Sprintf("step %d %s %v", result.Step, result.Type, result.Passed)
			resultLayout := f.shaper.ShapeSimple(lineText, bodyStyle)
			if resultLayout != nil && len(resultLayout.Lines) > 0 {
				line := resultLayout.Lines[0]
				origin := gfx.Point{X: inner.Min.X + 16, Y: y}
				for _, run := range line.Runs {
					list.Add(gfx.DrawGlyphRun{
						Run:    run,
						Origin: origin,
						Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
					})
				}
				y += resultLayout.Bounds.Height() + 4
			}
		}
	}
}
