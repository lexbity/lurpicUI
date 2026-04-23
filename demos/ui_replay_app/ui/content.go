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

// ContentFacet displays the main scenario content viewport.
type ContentFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	th           theme.Context
	shaper       *text.Shaper
	subscription signal.SubscriptionID
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
	f.subscription = store.SelectedScenarioStore.OnChange.Subscribe(func(change signal.Change[model.ScenarioID]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *ContentFacet) OnDetach() {
	store.SelectedScenarioStore.OnChange.Unsubscribe(f.subscription)
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

	sectionStyle := f.th.TextStyle(theme.TextLabelS)
	actionLayout := f.shaper.ShapeSimple("Actions: "+string(rune(len(scenario.Actions)+48)), sectionStyle)
	if actionLayout != nil && len(actionLayout.Lines) > 0 {
		line := actionLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
		y += actionLayout.Bounds.Height() + 8
	}

	for i, action := range scenario.Actions {
		if y > inner.Max.Y {
			break
		}
		actionText := string(rune(i+1+48)) + ". " + string(action.Type)
		itemStyle := f.th.TextStyle(theme.TextBodyS)
		itemLayout := f.shaper.ShapeSimple(actionText, itemStyle)
		if itemLayout != nil && len(itemLayout.Lines) > 0 {
			line := itemLayout.Lines[0]
			origin := gfx.Point{X: inner.Min.X + 16, Y: y}
			for _, run := range line.Runs {
				list.Add(gfx.DrawGlyphRun{
					Run:    run,
					Origin: origin,
					Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
				})
			}
			y += itemLayout.Bounds.Height() + 4
		}
	}
}
