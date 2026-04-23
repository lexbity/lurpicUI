package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// LayoutScene validates constraint extremes, nesting, and overflow handling.
type LayoutScene struct {
	BaseScene
	nestingDepth int
}

// NewLayoutScene creates a new layout testing scene.
func NewLayoutScene() *LayoutScene {
	s := &LayoutScene{
		BaseScene: NewBaseScene(
			"layout",
			"Layout",
			"Validates constraint extremes, nesting, clipping, and overflow",
			[]string{"structure"},
		),
		nestingDepth: 4,
	}
	s.capability.HasStressControls = true
	return s
}

// BuildRoot constructs the layout test UI.
func (s *LayoutScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	// Main column
	col := layout.NewColumnLayout()
	s.root = col

	// Row with varied width items
	row := layout.NewRowLayout()

	// Narrow item
	narrow := &basic.Rect{
		ID:     "narrow",
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 30, H: 40},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(255, 100, 100, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
	row.Add(layout.Fixed(narrow.Base()))

	// Wide item
	wide := &basic.Rect{
		ID:     "wide",
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 200, H: 40},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(100, 255, 100, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
	row.Add(layout.Fixed(wide.Base()))

	col.AddChild(row.Base())

	// Deep nesting test
	deepStack := s.createNesting(s.nestingDepth)
	col.AddChild(deepStack.Base())

	return col
}

func (s *LayoutScene) createNesting(depth int) *layout.StackLayout {
	if depth <= 0 {
		return layout.NewStackLayout(layout.AlignStart)
	}

	stack := layout.NewStackLayout(layout.AlignStart)

	// Add a colored rect at this level with padding
	padding := float32(depth) * 10
	color := gfx.ColorFromRGBA8(
		uint8(255-depth*30),
		uint8(100+depth*20),
		uint8(150),
		255,
	)
	rect := &basic.Rect{
		ID:     "nest-" + string(rune('0'+depth)),
		Bounds: basic.BoundsProps{X: padding, Y: padding, W: 200 - padding*2, H: 200 - padding*2},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(color),
			Visible: true,
			Opacity: 1,
		},
	}
	stack.AddChild(rect.Base())

	// Recurse for deeper nesting
	if depth > 1 {
		child := s.createNesting(depth - 1)
		stack.AddChild(child.Base())
	}

	return stack
}

// ExportState returns layout state.
func (s *LayoutScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":      s.id,
		"nesting_depth": s.nestingDepth,
	}
}

var _ scene.Scene = (*LayoutScene)(nil)
