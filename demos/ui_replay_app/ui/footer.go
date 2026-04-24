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

// FooterFacet displays the bottom status bar.
type FooterFacet struct {
	facet.Facet
	layout      facet.LayoutRole
	render      facet.RenderRole
	th          theme.Context
	shaper      *text.Shaper
	registrySub signal.SubscriptionID
	execSub     signal.SubscriptionID
}

// NewFooterFacet creates a new footer facet.
func NewFooterFacet(th theme.Context, shaper *text.Shaper) *FooterFacet {
	footer := &FooterFacet{
		Facet:  facet.NewFacet(),
		th:     th,
		shaper: shaper,
	}

	footer.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: footerHeight}
	}
	footer.layout.OnArrange = func(bounds gfx.Rect) {
		footer.layout.ArrangedBounds = bounds
	}
	footer.AddRole(&footer.layout)

	footer.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		footer.renderFooter(list, bounds)
	}
	footer.AddRole(&footer.render)

	return footer
}

// Base returns the base facet.
func (f *FooterFacet) Base() *facet.Facet {
	f.Facet.BindImpl(f)
	return &f.Facet
}

// OnAttach handles attachment and subscribes to stores.
func (f *FooterFacet) OnAttach(ctx facet.AttachContext) {
	f.registrySub = store.ScenarioRegistryStore.OnChange.Subscribe(func(change signal.Change[*store.ScenarioRegistry]) {
		f.Invalidate(facet.DirtyProjection)
	})
	f.execSub = store.ExecutionStateStore.OnChange.Subscribe(func(change signal.Change[store.ExecutionState]) {
		f.Invalidate(facet.DirtyProjection)
	})
}

// OnDetach handles detachment.
func (f *FooterFacet) OnDetach() {
	store.ScenarioRegistryStore.OnChange.Unsubscribe(f.registrySub)
	store.ExecutionStateStore.OnChange.Unsubscribe(f.execSub)
}

// OnActivate handles activation.
func (f *FooterFacet) OnActivate() {}

// OnDeactivate handles deactivation.
func (f *FooterFacet) OnDeactivate() {}

func (f *FooterFacet) renderFooter(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorSurface)),
	})

	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Min.Y, bounds.Width(), 1),
		Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorder)),
	})

	if f.shaper == nil {
		return
	}

	inner := Inset(bounds, 8)
	if inner.IsEmpty() {
		return
	}

	env := store.EnvironmentStore.Get()
	paths := store.GetPaths()

	statusStyle := f.th.TextStyle(theme.TextLabelS)

	registry := store.ScenarioRegistryStore.Get()
	count := 0
	if registry != nil {
		count = registry.Count()
	}

	leftText := fmt.Sprintf("%d scenarios | %s | %s", count, env.DisplayString(), paths.ScenarioDir)
	leftLayout := f.shaper.ShapeSimple(leftText, statusStyle)
	if leftLayout != nil && len(leftLayout.Lines) > 0 {
		line := leftLayout.Lines[0]
		origin := gfx.Point{X: inner.Min.X, Y: inner.Min.Y + 12}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
	}

	// Show execution controls hint on the right
	exec := store.ExecutionStateStore.Get()
	var rightText string
	if exec.IsRunning() {
		rightText = "[Esc] Cancel"
	} else {
		rightText = "[R] Run  [E] Export"
	}

	rightLayout := f.shaper.ShapeSimple(rightText, statusStyle)
	if rightLayout != nil && len(rightLayout.Lines) > 0 {
		line := rightLayout.Lines[0]
		x := inner.Max.X - line.Bounds.Width()
		origin := gfx.Point{X: x, Y: inner.Min.Y + 12}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorTextSecondary)),
			})
		}
	}
}
