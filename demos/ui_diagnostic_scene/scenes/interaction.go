package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/platform"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"

	textpkg "codeburg.org/lexbit/lurpicui/text"
)

// InteractionScene validates hover, press, drag, click, selection, and focus interactions.
type InteractionScene struct {
	BaseScene
	clickCount int
	focusIndex int
	logs       []string
	hoveredID  string
	pressedID  string
}

// NewInteractionScene creates a new interaction testing scene.
func NewInteractionScene() *InteractionScene {
	s := &InteractionScene{
		BaseScene: NewBaseScene(
			"interaction",
			"Interaction",
			"Validates hover, press, drag, click, selection, and focus",
			[]string{"basic"},
		),
		logs: make([]string, 0),
	}
	s.capability.HasCustomLogs = true
	return s
}

// BuildRoot constructs the interaction test UI.
func (s *InteractionScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	col := layout.NewColumnLayout()
	s.root = col

	// Click counter button area with input handling
	clickArea := s.createInteractiveButton("click-area", gfx.RectFromXYWH(20, 20, 200, 60), func() {
		s.clickCount++
		s.logEvent("Button clicked! Count: %d", s.clickCount)
	})
	col.AddChild(clickArea)

	// Click count label
	label := &basic.Text{
		ID: "click-label",
		Paragraph: textpkg.Paragraph{
			Spans: []textpkg.TextSpan{
				{Text: "Clicks: 0", Style: textpkg.TextStyle{Size: 14}},
			},
		},
		MaxWidth:   200,
		Selectable: true,
	}
	col.AddChild(label.Base())

	// Add drag test area
	dragArea := s.createDraggableArea("drag-area", gfx.RectFromXYWH(20, 220, 300, 100))
	col.AddChild(dragArea)

	// Focus chain test elements
	for i := 0; i < 3; i++ {
		focusItem := &basic.Rect{
			ID:     "focus-" + string(rune('0'+i)),
			Bounds: basic.BoundsProps{X: 20, Y: float32(100 + i*40), W: 150, H: 30},
			Style: basic.PrimitiveStyleProps{
				Fill:    solidFill(gfx.ColorFromRGBA8(150, 150, 150, 255)),
				Stroke:  solidStroke(gfx.ColorFromRGBA8(0, 0, 0, 255), 1),
				Visible: true,
				Opacity: 1,
			},
		}
		col.AddChild(focusItem.Base())
	}

	return col
}

// logEvent logs an interaction event
func (s *InteractionScene) logEvent(format string, args ...interface{}) {
	msg := "Interaction: " + format
	if len(args) > 0 {
		// In a real implementation, use fmt.Sprintf
		_ = msg
	}
	s.logs = append(s.logs, msg)
	if len(s.logs) > 50 {
		s.logs = s.logs[1:]
	}
}

// createInteractiveButton creates a button with hover, press, and click handling
func (s *InteractionScene) createInteractiveButton(id string, bounds gfx.Rect, onClick func()) *facet.Facet {
	rect := &basic.Rect{
		ID:     id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: bounds.Width(), H: bounds.Height()},
		Style:  s.getButtonStyle(false, false),
	}

	f := rect.Base()

	// Add hit role
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

	// Add input role
	inputRole := &facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			switch e.Kind {
			case platform.PointerMove:
				if s.hoveredID != id {
					s.hoveredID = id
					s.logEvent("Hover: %s", id)
					rect.Style = s.getButtonStyle(true, s.pressedID == id)
				}
			case platform.PointerLeave:
				if s.hoveredID == id {
					s.hoveredID = ""
					s.pressedID = ""
					s.logEvent("Leave: %s", id)
					rect.Style = s.getButtonStyle(false, false)
				}
			case platform.PointerPress:
				s.pressedID = id
				s.logEvent("Press: %s", id)
				rect.Style = s.getButtonStyle(s.hoveredID == id, true)
			case platform.PointerRelease:
				s.logEvent("Release: %s", id)
				rect.Style = s.getButtonStyle(s.hoveredID == id, false)
				if s.pressedID == id && s.hoveredID == id {
					onClick()
				}
				s.pressedID = ""
			}
			return true
		},
	}
	f.AddRole(inputRole)

	return f
}

func (s *InteractionScene) getButtonStyle(hovered, pressed bool) basic.PrimitiveStyleProps {
	var color gfx.Color
	switch {
	case pressed:
		color = gfx.ColorFromRGBA8(80, 120, 180, 255) // Darker blue when pressed
	case hovered:
		color = gfx.ColorFromRGBA8(120, 170, 220, 255) // Lighter blue when hovered
	default:
		color = gfx.ColorFromRGBA8(100, 150, 200, 255) // Default blue
	}

	return basic.PrimitiveStyleProps{
		Fill:    solidFill(color),
		Stroke:  solidStroke(gfx.ColorFromRGBA8(50, 100, 150, 255), 2),
		Visible: true,
		Opacity: 1,
	}
}

// createDraggableArea creates an area that tracks drag gestures
func (s *InteractionScene) createDraggableArea(id string, bounds gfx.Rect) *facet.Facet {
	rect := &basic.Rect{
		ID:     id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: bounds.Width(), H: bounds.Height()},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(200, 200, 200, 255)),
			Stroke:  solidStroke(gfx.ColorFromRGBA8(100, 100, 100, 255), 1),
			Visible: true,
			Opacity: 1,
		},
	}

	f := rect.Base()
	var dragStart gfx.Point
	var isDragging bool

	// Add hit role
	hitRole := &facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			localBounds := gfx.RectFromXYWH(0, 0, bounds.Width(), bounds.Height())
			hit := localBounds.Contains(p)
			cursor := facet.CursorDefault
			if isDragging {
				cursor = facet.CursorGrab
			} else if hit {
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

	// Add input role for drag handling
	inputRole := &facet.InputRole{
		OnPointer: func(e facet.PointerEvent) bool {
			switch e.Kind {
			case platform.PointerPress:
				dragStart = e.Position
				isDragging = true
				s.logEvent("Drag start at (%.0f, %.0f)", e.Position.X, e.Position.Y)
				rect.Style.Fill = solidFill(gfx.ColorFromRGBA8(180, 220, 180, 255))
			case platform.PointerRelease:
				if isDragging {
					delta := gfx.Point{X: e.Position.X - dragStart.X, Y: e.Position.Y - dragStart.Y}
					s.logEvent("Drag end, delta (%.0f, %.0f)", delta.X, delta.Y)
					isDragging = false
					rect.Style.Fill = solidFill(gfx.ColorFromRGBA8(200, 200, 200, 255))
				}
			case platform.PointerMove:
				if isDragging {
					delta := gfx.Point{X: e.Position.X - dragStart.X, Y: e.Position.Y - dragStart.Y}
					if delta.X != 0 || delta.Y != 0 {
						s.logEvent("Dragging: (%.0f, %.0f)", e.Position.X, e.Position.Y)
					}
				}
			}
			return true
		},
	}
	f.AddRole(inputRole)

	return f
}

// GetLogs returns interaction logs
func (s *InteractionScene) GetLogs() []string {
	return s.logs
}

// ExportState returns interaction state.
func (s *InteractionScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":    s.id,
		"click_count": s.clickCount,
		"focus_index": s.focusIndex,
		"log_count":   len(s.logs),
	}
}

var _ scene.Scene = (*InteractionScene)(nil)
