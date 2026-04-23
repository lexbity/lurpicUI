package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// SceneHostFacet displays the currently loaded scene
type SceneHostFacet struct {
	facet.Facet
	layout facet.LayoutRole
	render facet.RenderRole
	theme  theme.Context
	shaper *text.Shaper

	// Scene state
	currentScene   scene.Scene
	currentSceneID string

	// Signals
	OnSceneChanged signal.Signal[string]
	OnReset        signal.Signal[struct{}]
}

// NewSceneHostFacet constructs the scene host viewport
func NewSceneHostFacet(th theme.Context, shaper *text.Shaper) *SceneHostFacet {
	h := &SceneHostFacet{
		Facet:  facet.NewFacet(),
		theme:  th,
		shaper: shaper,
	}

	h.layout.OnMeasure = func(c facet.Constraints) gfx.Size {
		return gfx.Size{W: c.MaxSize.W, H: c.MaxSize.H}
	}
	h.layout.OnArrange = func(bounds gfx.Rect) {
		h.layout.ArrangedBounds = bounds
	}
	h.AddRole(&h.layout)

	h.render.OnCollect = func(list *gfx.CommandList, bounds gfx.Rect) {
		h.renderHost(list, bounds)
	}
	h.AddRole(&h.render)

	return h
}

func (h *SceneHostFacet) Base() *facet.Facet {
	h.Facet.BindImpl(h)
	return &h.Facet
}

func (h *SceneHostFacet) OnAttach(ctx facet.AttachContext) {}
func (h *SceneHostFacet) OnDetach()                        {}
func (h *SceneHostFacet) OnActivate()                      {}
func (h *SceneHostFacet) OnDeactivate()                    {}

func (h *SceneHostFacet) renderHost(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(h.theme.Color(theme.ColorBackground)),
	})

	// If no scene loaded, show placeholder
	if h.currentScene == nil {
		h.renderPlaceholder(list, bounds)
		return
	}

	// Scene renders its own content; host provides container
	// (Scene facet would be attached as child in a full implementation)
}

func (h *SceneHostFacet) renderPlaceholder(list *gfx.CommandList, bounds gfx.Rect) {
	text := "Select a scene from the left panel"
	style := h.theme.TextStyle(theme.TextBodyM)

	layout := h.shaper.ShapeSimple(text, style)
	if layout != nil && len(layout.Lines) > 0 {
		line := layout.Lines[0]
		// Center text
		textW := line.Bounds.Width()
		textH := layout.Bounds.Height()
		x := bounds.Min.X + (bounds.Width()-textW)/2
		y := bounds.Min.Y + (bounds.Height()-textH)/2

		origin := gfx.Point{X: x, Y: y + line.Baseline}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(h.theme.Color(theme.ColorTextSecondary)),
			})
		}
	}
}

// LoadScene loads a scene by ID from the registry
func (h *SceneHostFacet) LoadScene(id string, registry *scene.Registry) {
	if h.currentSceneID == id {
		return
	}

	// Unload current scene
	if h.currentScene != nil {
		// Cleanup would happen here
	}

	// Load new scene
	newScene, ok := registry.Create(id)
	if !ok {
		h.currentScene = nil
		h.currentSceneID = ""
		h.Invalidate(facet.DirtyProjection)
		return
	}

	h.currentScene = newScene
	h.currentSceneID = id
	h.OnSceneChanged.Emit(id)
	h.Invalidate(facet.DirtyProjection)
}

// Reset resets the current scene to its baseline state
func (h *SceneHostFacet) Reset() {
	if h.currentScene != nil {
		h.currentScene.Reset()
		h.OnReset.Emit(struct{}{})
		h.Invalidate(facet.DirtyProjection)
	}
}

// CurrentScene returns the loaded scene
func (h *SceneHostFacet) CurrentScene() scene.Scene {
	return h.currentScene
}

// CurrentSceneID returns the loaded scene ID
func (h *SceneHostFacet) CurrentSceneID() string {
	return h.currentSceneID
}
