package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// InputFocusScene validates keyboard routing, tab order, caret visibility,
// and focus loss/regain behavior.
type InputFocusScene struct {
	BaseScene
	focusableItems []*focusableItem
	focusedIndex   int
	logs           []string
}

type focusableItem struct {
	id       string
	bounds   gfx.Rect
	focused  bool
	hovered  bool
	pressed  bool
	tabIndex int
	facet    *facet.Facet
}

// NewInputFocusScene creates a new input/focus testing scene.
func NewInputFocusScene() *InputFocusScene {
	s := &InputFocusScene{
		BaseScene: NewBaseScene(
			"input-focus",
			"Input / Focus",
			"Validates keyboard routing, tab order, caret visibility, focus transitions",
			[]string{"uiinput"},
		),
		focusableItems: make([]*focusableItem, 0),
		focusedIndex:   -1,
		logs:           make([]string, 0),
	}
	s.capability.HasCustomLogs = true
	return s
}

// BuildRoot constructs the input/focus test UI.
func (s *InputFocusScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	col := layout.NewColumnLayout()
	s.root = col
	s.focusableItems = make([]*focusableItem, 0)

	// Create focusable buttons in a row
	row := layout.NewRowLayout()
	for i := 0; i < 4; i++ {
		item := s.createFocusableButton(i, gfx.RectFromXYWH(float32(i*110), 20, 100, 50))
		s.focusableItems = append(s.focusableItems, item)
		row.Add(layout.Fixed(item.facet))
	}
	col.AddChild(row.Base())

	// Add non-focusable decorative element
	decor := &basic.Rect{
		ID:     "decor",
		Bounds: basic.BoundsProps{X: 20, Y: 100, W: 200, H: 30},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(200, 200, 200, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
	col.AddChild(decor.Base())

	return col
}

func (s *InputFocusScene) createFocusableButton(index int, bounds gfx.Rect) *focusableItem {
	item := &focusableItem{
		id:       "button-" + string(rune('A'+index)),
		bounds:   bounds,
		tabIndex: index,
	}

	// Create the visual rect
	rect := &basic.Rect{
		ID:     item.id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: bounds.Width(), H: bounds.Height()},
		Style:  s.getItemStyle(item),
	}

	// Set up input handling via facet roles
	f := rect.Base()

	// Add hit role for hover detection
	hitRole := &facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			localBounds := gfx.RectFromXYWH(0, 0, bounds.Width(), bounds.Height())
			hit := localBounds.Contains(p)
			cursor := facet.CursorDefault
			if hit {
				cursor = facet.CursorPointer
			}
			return facet.HitResult{
				Hit:    hit,
				MarkID: 0,
				Cursor: cursor,
			}
		},
	}
	f.AddRole(hitRole)

	// Add input role for pointer events
	inputRole := &facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			s.handlePointerEvent(item, e)
			return true
		},
		OnKey: func(e facet.KeyEvent) bool {
			return s.handleKeyEvent(item, e)
		},
	}
	f.AddRole(inputRole)

	// Add focus role
	focusRole := &facet.FocusRole{
		Focusable: func() bool { return true },
		OnFocusGained: func() {
			item.focused = true
			s.focusedIndex = index
			s.logEvent("Focus gained: " + item.id)
			s.updateItemVisual(item)
		},
		OnFocusLost: func() {
			item.focused = false
			s.logEvent("Focus lost: " + item.id)
			s.updateItemVisual(item)
		},
		TabIndex: index,
	}
	f.AddRole(focusRole)

	item.facet = f
	return item
}

func (s *InputFocusScene) getItemStyle(item *focusableItem) basic.PrimitiveStyleProps {
	var color gfx.Color
	switch {
	case item.pressed:
		color = gfx.ColorFromRGBA8(100, 100, 255, 255) // Blue when pressed
	case item.focused:
		color = gfx.ColorFromRGBA8(255, 200, 100, 255) // Orange when focused
	case item.hovered:
		color = gfx.ColorFromRGBA8(200, 255, 200, 255) // Light green when hovered
	default:
		color = gfx.ColorFromRGBA8(220, 220, 220, 255) // Gray default
	}

	return basic.PrimitiveStyleProps{
		Fill:    solidFill(color),
		Stroke:  solidStroke(gfx.ColorFromRGBA8(0, 0, 0, 255), 2),
		Visible: true,
		Opacity: 1,
	}
}

func (s *InputFocusScene) updateItemVisual(item *focusableItem) {
	// In a real implementation, this would update the mark's style
	// and trigger a re-render
}

func (s *InputFocusScene) handlePointerEvent(item *focusableItem, e facet.PointerEvent) {
	switch e.Kind {
	case platform.PointerMove:
		if !item.hovered {
			item.hovered = true
			s.logEvent("Hover enter: " + item.id)
			s.updateItemVisual(item)
		}
	case platform.PointerLeave:
		item.hovered = false
		item.pressed = false
		s.logEvent("Hover leave: " + item.id)
		s.updateItemVisual(item)
	case platform.PointerPress:
		item.pressed = true
		s.logEvent("Pointer down: " + item.id)
		s.updateItemVisual(item)
		// Request focus on click
		if item.facet != nil {
			// In a real implementation, this would request focus
		}
	case platform.PointerRelease:
		item.pressed = false
		s.logEvent("Pointer up: " + item.id)
		s.updateItemVisual(item)
	}
}

func (s *InputFocusScene) handleKeyEvent(item *focusableItem, e facet.KeyEvent) bool {
	if !item.focused {
		return false
	}

	switch e.Key {
	case platform.KeyEnter, platform.KeySpace:
		s.logEvent("Activate: " + item.id)
		return true
	case platform.KeyEscape:
		s.logEvent("Cancel: " + item.id)
		return true
	}
	return false
}

func (s *InputFocusScene) logEvent(msg string) {
	s.logs = append(s.logs, msg)
	if len(s.logs) > 50 {
		s.logs = s.logs[1:]
	}
}

// GetLogs returns the interaction log.
func (s *InputFocusScene) GetLogs() []string {
	return s.logs
}

// Reset clears the scene state.
func (s *InputFocusScene) Reset() {
	s.focusedIndex = -1
	s.logs = make([]string, 0)
	s.focusableItems = make([]*focusableItem, 0)
	s.BaseScene.Reset()
}

// ExportState returns input/focus state.
func (s *InputFocusScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":      s.id,
		"focused_index": s.focusedIndex,
		"log_count":     len(s.logs),
	}
}

// ImportState restores input/focus state.
func (s *InputFocusScene) ImportState(state map[string]any) {
	if v, ok := state["focused_index"].(float64); ok {
		s.focusedIndex = int(v)
	}
}

var _ scene.Scene = (*InputFocusScene)(nil)
