package shell

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/signal"
	"codeburg.org/lexbit/lurpicui/text"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// SceneSnapshot captures the loaded scene state for export and debugging.
type SceneSnapshot struct {
	SceneID   string
	SceneName string
	HasRoot   bool
	State     map[string]any
}

// SceneHostFacet displays the currently loaded scene
type SceneHostFacet struct {
	facet.Facet
	layout       facet.LayoutRole
	render       facet.RenderRole
	theme        theme.Context
	shaper       *text.Shaper
	densityScale float32

	// Scene state
	currentScene   scene.Scene
	currentSceneID string
	mountedRoot    facet.FacetImpl

	attachCtx facet.AttachContext
	attached  bool
	active    bool

	// Signals
	OnSceneChanged signal.Signal[string]
	OnReset        signal.Signal[struct{}]
}

// NewSceneHostFacet constructs the scene host viewport
func NewSceneHostFacet(th theme.Context, shaper *text.Shaper) *SceneHostFacet {
	h := &SceneHostFacet{
		Facet:        facet.NewFacet(),
		theme:        th,
		shaper:       shaper,
		densityScale: 1,
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

func (h *SceneHostFacet) OnAttach(ctx facet.AttachContext) {
	h.attachCtx = ctx
	h.attached = true
}

func (h *SceneHostFacet) OnDetach() {
	h.unmountCurrentScene()
	h.attached = false
	h.active = false
	h.attachCtx = facet.AttachContext{}
	h.currentScene = nil
	h.currentSceneID = ""
	h.mountedRoot = nil
}

func (h *SceneHostFacet) OnActivate() {
	h.active = true
}

func (h *SceneHostFacet) OnDeactivate() {
	h.active = false
}

func (h *SceneHostFacet) renderHost(list *gfx.CommandList, bounds gfx.Rect) {
	if list == nil || bounds.IsEmpty() {
		return
	}

	// Background
	list.Add(gfx.FillRect{
		Rect:  bounds,
		Brush: gfx.SolidBrush(h.theme.Color(theme.ColorBackground)),
	})

	// If no scene root is mounted, show placeholder.
	if h.mountedRoot == nil {
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
	if h.State() == facet.StateDisposed {
		return
	}
	if h.currentSceneID == id && h.currentScene != nil && h.mountedRoot != nil {
		return
	}

	h.unmountCurrentScene()

	if registry == nil {
		h.currentScene = nil
		h.currentSceneID = ""
		h.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		return
	}

	newScene, ok := registry.Create(id)
	if !ok {
		h.currentScene = nil
		h.currentSceneID = ""
		h.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
		return
	}

	root := newScene.BuildRoot()
	h.currentScene = newScene
	h.currentSceneID = id
	h.mountedRoot = nil

	if root != nil {
		h.mountSceneRoot(root)
	}

	h.applyMountedSceneSettings()
	h.OnSceneChanged.Emit(id)
	h.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// Reset resets the current scene to its baseline state
func (h *SceneHostFacet) Reset() {
	if h.currentScene != nil {
		h.currentScene.Reset()
		h.RebuildCurrentScene()
		h.OnReset.Emit(struct{}{})
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

// MountedRoot returns the currently mounted scene root facet.
func (h *SceneHostFacet) MountedRoot() facet.FacetImpl {
	return h.mountedRoot
}

// SnapshotState returns the current scene state in a stable export-friendly form.
func (h *SceneHostFacet) SnapshotState() SceneSnapshot {
	snapshot := SceneSnapshot{
		SceneID: h.currentSceneID,
		HasRoot: h.mountedRoot != nil,
	}
	if h.currentScene != nil {
		snapshot.SceneID = h.currentScene.SceneID()
		snapshot.SceneName = h.currentScene.DisplayName()
		snapshot.State = h.currentScene.ExportState()
	}
	if snapshot.State == nil {
		snapshot.State = map[string]any{}
	}
	return snapshot
}

// ApplyTheme updates the host theme and propagates it to the mounted scene.
func (h *SceneHostFacet) ApplyTheme(th theme.Context) {
	if h == nil {
		return
	}
	h.theme = th
	h.applyMountedSceneSettings()
	h.Invalidate(facet.DirtyProjection)
}

// ApplyDensity updates the host density scale and propagates it to the mounted scene.
func (h *SceneHostFacet) ApplyDensity(scale float32) {
	if h == nil {
		return
	}
	h.densityScale = scale
	h.applyMountedSceneSettings()
	h.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

// RebuildCurrentScene rebuilds the mounted root without resetting scene state.
func (h *SceneHostFacet) RebuildCurrentScene() {
	if h == nil || h.currentScene == nil {
		return
	}
	h.unmountCurrentScene()
	root := h.currentScene.BuildRoot()
	if root != nil {
		h.mountSceneRoot(root)
	}
	h.applyMountedSceneSettings()
	h.Invalidate(facet.DirtyLayout | facet.DirtyProjection)
}

func (h *SceneHostFacet) mountSceneRoot(root facet.FacetImpl) {
	if root == nil {
		h.mountedRoot = nil
		return
	}

	h.Base().AddChildRuntime(root.Base())
	h.mountedRoot = root

	hostState := h.State()
	switch hostState {
	case facet.StateAttached, facet.StateInactive, facet.StateActive:
		facet.Attach(root, h.attachCtx)
		if hostState == facet.StateActive {
			facet.Activate(root)
		}
	}
}

func (h *SceneHostFacet) applyMountedSceneSettings() {
	if h == nil || h.currentScene == nil {
		return
	}
	if h.theme != nil {
		h.currentScene.ApplyTheme(h.theme)
	}
	h.currentScene.ApplyDensity(h.densityScale)
}

func (h *SceneHostFacet) unmountCurrentScene() {
	if h.mountedRoot == nil {
		return
	}

	h.Base().RemoveChild(h.mountedRoot.Base())
	h.mountedRoot = nil
}
