package ui

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
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
	input       facet.InputRole

	buttonRects  map[string]gfx.Rect
	activeButton string
	compact      bool
	visible      []headerButton

	// Commands
	OnToggleScenarios signal.Signal[struct{}]
	OnToggleDetails   signal.Signal[struct{}]
	OnToggleArtifacts signal.Signal[struct{}]
	OnRun             signal.Signal[struct{}]
	OnCancel          signal.Signal[struct{}]
	OnExport          signal.Signal[struct{}]
}

type headerButton struct {
	ID    string
	Label string
}

// NewHeaderFacet creates a new header facet.
func NewHeaderFacet(th theme.Context, shaper *text.Shaper, meta model.BuildMetadata) *HeaderFacet {
	h := &HeaderFacet{
		Facet:       facet.NewFacet(),
		th:          th,
		shaper:      shaper,
		meta:        meta,
		buttonRects: make(map[string]gfx.Rect),
	}

	h.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: headerHeight}
	}
	h.layout.OnArrange = func(bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
		h.layoutButtons(bounds)
	}
	h.AddRole(&h.layout)

	h.input.OnPointer = func(e facet.PointerEvent) bool {
		switch e.Kind {
		case platform.PointerPress:
			h.activeButton = h.hitButtonAt(e.Position)
			if h.activeButton != "" {
				return true
			}
		case platform.PointerRelease:
			button := h.hitButtonAt(e.Position)
			if button != "" && button == h.activeButton {
				h.emitButton(button)
			}
			h.activeButton = ""
			if button != "" {
				return true
			}
		}
		return false
	}
	h.AddRole(&h.input)

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

func (f *HeaderFacet) layoutButtons(bounds gfx.Rect) {
	if f == nil {
		return
	}
	if f.buttonRects == nil {
		f.buttonRects = make(map[string]gfx.Rect)
	}
	f.visible = f.visible[:0]
	f.compact = bounds.Width() < 900
	if f.compact {
		f.visible = append(f.visible,
			headerButton{ID: "scenarios", Label: "Scenarios"},
			headerButton{ID: "details", Label: "Details"},
			headerButton{ID: "artifacts", Label: "Artifacts"},
			headerButton{ID: "run", Label: "Run"},
			headerButton{ID: "cancel", Label: "Stop"},
			headerButton{ID: "export", Label: "Export"},
		)
	} else {
		return
	}
	inner := bounds.Inset(12, 4)
	right := inner.Max.X
	for i := len(f.visible) - 1; i >= 0; i-- {
		button := f.visible[i]
		width := float32(72)
		if len(button.Label) > 8 {
			width = 84
		}
		if button.ID == "run" || button.ID == "cancel" || button.ID == "export" {
			width = 64
		}
		rect := gfx.RectFromXYWH(right-width, inner.Min.Y+4, width, inner.Height()-8)
		f.buttonRects[button.ID] = rect
		right = rect.Min.X - 8
	}
}

func (f *HeaderFacet) hitButtonAt(p gfx.Point) string {
	for _, button := range f.visible {
		if rect, ok := f.buttonRects[button.ID]; ok && rect.Contains(p) {
			return button.ID
		}
	}
	return ""
}

func (f *HeaderFacet) emitButton(id string) {
	switch id {
	case "scenarios":
		f.OnToggleScenarios.Emit(struct{}{})
	case "details":
		f.OnToggleDetails.Emit(struct{}{})
	case "artifacts":
		f.OnToggleArtifacts.Emit(struct{}{})
	case "run":
		f.OnRun.Emit(struct{}{})
	case "cancel":
		f.OnCancel.Emit(struct{}{})
	case "export":
		f.OnExport.Emit(struct{}{})
	}
}

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

	if !f.compact {
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

	for _, button := range f.visible {
		rect := f.buttonRects[button.ID]
		if rect.IsEmpty() {
			continue
		}
		fill := f.th.Color(theme.ColorSurfaceVariant)
		if button.ID == f.activeButton {
			fill = f.th.Color(theme.ColorSelection)
		}
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fill)})
		list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(rect.Min.X, rect.Min.Y, rect.Width(), 1), Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorder))})
		list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(rect.Min.X, rect.Max.Y-1, rect.Width(), 1), Brush: gfx.SolidBrush(f.th.Color(theme.ColorBorder))})
		labelLayout := f.shaper.ShapeSimple(button.Label, f.th.TextStyle(theme.TextLabelS))
		if labelLayout == nil || len(labelLayout.Lines) == 0 {
			continue
		}
		line := labelLayout.Lines[0]
		origin := gfx.Point{X: rect.Min.X + 8, Y: rect.Min.Y + rect.Height()/2 + line.Baseline/2}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(f.th.Color(theme.ColorText)),
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
