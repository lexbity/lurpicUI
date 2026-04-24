package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// SceneNavFacet displays the list of available scenes
type SceneNavFacet struct {
	facet.Facet
	layout   facet.LayoutRole
	render   facet.RenderRole
	theme    theme.Context
	shaper   *text.Shaper
	registry *scene.Registry

	// Navigation state
	selectedID string
	scrollY    float32

	// Signals
	OnSceneSelected signal.Signal[string]
}

// NewSceneNavFacet constructs the scene navigation panel
func NewSceneNavFacet(th theme.Context, shaper *text.Shaper, registry *scene.Registry) *SceneNavFacet {
	n := &SceneNavFacet{
		Facet:    facet.NewFacet(),
		theme:    th,
		shaper:   shaper,
		registry: registry,
	}

	n.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: c.MaxSize.H}
	}
	n.layout.OnArrange = func(bounds gfx.Rect) {
		n.layout.ArrangedBounds = bounds
	}
	n.AddRole(&n.layout)

	n.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		n.renderNav(list, bounds)
	}
	n.AddRole(&n.render)

	return n
}

func (n *SceneNavFacet) Base() *facet.Facet {
	n.Facet.BindImpl(n)
	return &n.Facet
}

func (n *SceneNavFacet) OnAttach(ctx facet.AttachContext) {}
func (n *SceneNavFacet) OnDetach()                        {}
func (n *SceneNavFacet) OnActivate()                      {}
func (n *SceneNavFacet) OnDeactivate()                    {}

func (n *SceneNavFacet) renderNav(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(n.theme.Color(theme.ColorSurface)),
	})

	// Right border
	list.Add(gfx.FillRect{
		Rect:  gfx.RectFromXYWH(bounds.Max.X-1, bounds.Min.Y, 1, bounds.Height()),
		Brush: gfx.SolidBrush(n.theme.Color(theme.ColorBorder)),
	})

	if n.shaper == nil || n.registry == nil {
		return
	}

	// Header
	y := bounds.Min.Y + 12
	y = n.renderHeader(list, bounds, y)

	// Scene list
	defs := n.registry.GetAll()
	for _, def := range defs {
		y = n.renderSceneItem(list, bounds, y, def)
		if y > bounds.Max.Y {
			break
		}
	}
}

func (n *SceneNavFacet) renderHeader(list *gfx.CommandList, bounds gfx.Rect, y float32) float32 {
	text := "Scenes"
	style := n.theme.TextStyle(theme.TextLabelS)
	layout := n.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(n.theme.Color(theme.ColorTextSecondary)),
			})
		}
		return y + layout.Bounds.Height() + 16
	}
	return y + 24
}

func (n *SceneNavFacet) renderSceneItem(list *gfx.CommandList, bounds gfx.Rect, y float32, def scene.Definition) float32 {
	isSelected := def.ID == n.selectedID

	// Selection background
	if isSelected {
		list.Add(gfx.FillRect{
			Rect:  gfx.RectFromXYWH(bounds.Min.X, y, bounds.Width(), 28),
			Brush: gfx.SolidBrush(n.theme.Color(theme.ColorSelection)),
		})
	}

	// Scene name
	style := n.theme.TextStyle(theme.TextBodyS)
	color := n.theme.Color(theme.ColorText)
	if isSelected {
		color = n.theme.Color(theme.ColorPrimary)
	}

	textLayout := n.shaper.ShapeSimple(def.DisplayName, style)
	if textLayout != nil && len(textLayout.Lines) > 0 {
		line := textLayout.Lines[0]
		origin := gfx.Point{X: bounds.Min.X + 12, Y: y + 18}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(color),
			})
		}
	}

	return y + 28
}

// SelectScene sets the selected scene ID
func (n *SceneNavFacet) SelectScene(id string) {
	if n.selectedID != id {
		n.selectedID = id
		n.OnSceneSelected.Emit(id)
		n.Invalidate(facet.DirtyProjection)
	}
}

// SelectedScene returns the current selection
func (n *SceneNavFacet) SelectedScene() string {
	return n.selectedID
}
