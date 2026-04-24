package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
)

// TopBarFacet displays metadata (theme, backend, platform, build info)
type TopBarFacet struct {
	facet.Facet
	layout facet.LayoutRole
	hit    facet.HitRole
	input  facet.InputRole
	render facet.RenderRole
	theme  theme.Context
	shaper *text.Shaper
	text   string

	buttons      []topBarButton
	buttonRects  map[string]gfx.Rect
	activeButton string

	// Commands
	OnReset        signal.Signal[struct{}]
	OnThemeNext    signal.Signal[struct{}]
	OnDensityNext  signal.Signal[struct{}]
	OnToggleBounds signal.Signal[struct{}]
	OnToggleHit    signal.Signal[struct{}]
	OnToggleFocus  signal.Signal[struct{}]
	OnToggleStress signal.Signal[struct{}]
}

type topBarButton struct {
	ID    string
	Label string
}

// NewTopBarFacet constructs the top metadata bar
func NewTopBarFacet(th theme.Context, shaper *text.Shaper) *TopBarFacet {
	t := &TopBarFacet{
		Facet:       facet.NewFacet(),
		theme:       th,
		shaper:      shaper,
		text:        "UI Diagnostic Scene",
		buttons:     []topBarButton{{"reset", "Reset"}, {"theme", "Theme"}, {"density", "Density"}, {"bounds", "Bounds"}, {"hit", "Hit"}, {"focus", "Focus"}, {"stress", "Stress"}},
		buttonRects: make(map[string]gfx.Rect),
	}

	t.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: 32}
	}
	t.layout.OnArrange = func(bounds gfx.Rect) {
		t.layout.ArrangedBounds = bounds
		t.layoutButtons(bounds)
	}
	t.AddRole(&t.layout)

	t.hit.OnHitTest = func(p gfx.Point) facet.HitResult {
		if t.hitButtonAt(p) != "" {
			return facet.HitResult{Hit: true, Cursor: facet.CursorPointer}
		}
		return facet.HitResult{Hit: true}
	}
	t.AddRole(&t.hit)

	t.input.OnPointer = func(e facet.PointerEvent) bool {
		switch e.Kind {
		case platform.PointerPress:
			t.activeButton = t.hitButtonAt(e.Position)
			if t.activeButton != "" {
				return true
			}
		case platform.PointerRelease:
			button := t.hitButtonAt(e.Position)
			if button != "" && button == t.activeButton {
				t.emitButton(button)
			}
			t.activeButton = ""
			if button != "" {
				return true
			}
		}
		return false
	}
	t.AddRole(&t.input)

	t.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		t.renderTopBar(list, bounds)
	}
	t.AddRole(&t.render)

	return t
}

func (t *TopBarFacet) Base() *facet.Facet {
	t.Facet.BindImpl(t)
	return &t.Facet
}

func (t *TopBarFacet) OnAttach(ctx facet.AttachContext) {}
func (t *TopBarFacet) OnDetach()                        {}
func (t *TopBarFacet) OnActivate()                      {}
func (t *TopBarFacet) OnDeactivate()                    {}

func (t *TopBarFacet) layoutButtons(bounds gfx.Rect) {
	if t == nil {
		return
	}
	if t.buttonRects == nil {
		t.buttonRects = make(map[string]gfx.Rect)
	}
	inner := bounds.Inset(12, 4)
	right := inner.Max.X
	for i := len(t.buttons) - 1; i >= 0; i-- {
		button := t.buttons[i]
		width := float32(68)
		if len(button.Label) > 6 {
			width = float32(76 + len(button.Label)*2)
		}
		rect := gfx.RectFromXYWH(right-width, inner.Min.Y+4, width, inner.Height()-8)
		t.buttonRects[button.ID] = rect
		right = rect.Min.X - 8
	}
}

func (t *TopBarFacet) hitButtonAt(p gfx.Point) string {
	for _, button := range t.buttons {
		if rect, ok := t.buttonRects[button.ID]; ok && rect.Contains(p) {
			return button.ID
		}
	}
	return ""
}

func (t *TopBarFacet) emitButton(id string) {
	switch id {
	case "reset":
		t.OnReset.Emit(struct{}{})
	case "theme":
		t.OnThemeNext.Emit(struct{}{})
	case "density":
		t.OnDensityNext.Emit(struct{}{})
	case "bounds":
		t.OnToggleBounds.Emit(struct{}{})
	case "hit":
		t.OnToggleHit.Emit(struct{}{})
	case "focus":
		t.OnToggleFocus.Emit(struct{}{})
	case "stress":
		t.OnToggleStress.Emit(struct{}{})
	}
}

func (t *TopBarFacet) renderTopBar(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(t.theme.Color(theme.ColorSurface)),
	})

	// Bottom border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Min.X, bounds.Max.Y-1, bounds.Width(), 1),
		Brush: gfx.SolidBrush(t.theme.Color(theme.ColorBorder)),
	})

	if t.shaper == nil {
		return
	}

	// Text and metadata
	style := t.theme.TextStyle(theme.TextLabelS)
	layout := t.shaper.ShapeSimple(t.text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: bounds.Min.Y + 20}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(t.theme.Color(theme.ColorText)),
			})
		}
	}

	meta := t.theme.TextStyle(theme.TextLabelS)
	if t.text != "" {
		_ = meta
	}

	for _, button := range t.buttons {
		rect := t.buttonRects[button.ID]
		if rect.IsEmpty() {
			continue
		}
		fill := t.theme.Color(theme.ColorSurfaceVariant)
		if button.ID == t.activeButton {
			fill = t.theme.Color(theme.ColorSelection)
		}
		list.Add(gfx.FillRect{Rect: rect, Brush: gfx.SolidBrush(fill)})
		list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(rect.Min.X, rect.Min.Y, rect.Width(), 1), Brush: gfx.SolidBrush(t.theme.Color(theme.ColorBorder))})
		list.Add(gfx.FillRect{Rect: gfx.RectFromXYWH(rect.Min.X, rect.Max.Y-1, rect.Width(), 1), Brush: gfx.SolidBrush(t.theme.Color(theme.ColorBorder))})
		labelLayout := t.shaper.ShapeSimple(button.Label, meta)
		if labelLayout == nil || len(labelLayout.Lines) == 0 {
			continue
		}
		line := labelLayout.Lines[0]
		origin := gfx.Point{X: rect.Min.X + 10, Y: rect.Min.Y + rect.Height()/2 + line.Baseline/2}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(t.theme.Color(theme.ColorText)),
			})
		}
	}
}

// SetInfo updates the displayed metadata.
func (t *TopBarFacet) SetInfo(themeName, backend, platform string) {
	t.SetStatus(themeName, "", backend, platform, "")
}

// SetStatus updates the displayed metadata and status text.
func (t *TopBarFacet) SetStatus(themeName, densityName, backend, platform, build string) {
	t.text = "UI Diagnostic Scene | Theme: " + themeName
	if densityName != "" {
		t.text += " | Density: " + densityName
	}
	if backend != "" {
		t.text += " | Backend: " + backend
	}
	if platform != "" {
		t.text += " | Platform: " + platform
	}
	if build != "" {
		t.text += " | Build: " + build
	}
	t.Invalidate(facet.DirtyProjection)
}
