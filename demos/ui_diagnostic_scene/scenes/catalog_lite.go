package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"

	textpkg "codeburg.org/lexbit/lurpicui/text"
)

// CatalogLiteScene verifies catalog inventory can render without diagnostic noise.
// This is a reduced scene that validates basic mark families render correctly.
type CatalogLiteScene struct {
	BaseScene
}

// NewCatalogLiteScene creates a new catalog-lite scene.
func NewCatalogLiteScene() *CatalogLiteScene {
	return &CatalogLiteScene{
		BaseScene: NewBaseScene(
			"catalog-lite",
			"Catalog Lite",
			"Verifies catalog inventory renders without diagnostic noise",
			[]string{"basic", "structure"},
		),
	}
}

// BuildRoot constructs the scene's facet tree.
func (s *CatalogLiteScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	// Create a column layout with sample marks
	col := layout.NewColumnLayout()
	s.root = col

	// Add a rect mark
	rect := &basic.Rect{
		ID:     "catalog-rect",
		Bounds: basic.BoundsProps{X: 20, Y: 20, W: 100, H: 60},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(100, 150, 200, 255)),
			Stroke:  solidStroke(gfx.ColorFromRGBA8(50, 100, 150, 255), 2),
			Visible: true,
			Opacity: 1,
		},
	}
	col.AddChild(rect.Base())

	// Add an ellipse mark
	ellipse := &basic.Ellipse{
		ID:     "catalog-ellipse",
		Bounds: basic.BoundsProps{X: 140, Y: 20, W: 80, H: 80},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(200, 100, 150, 255)),
			Visible: true,
			Opacity: 1,
		},
	}
	col.AddChild(ellipse.Base())

	// Add a text label
	textMark := &basic.Text{
		ID: "catalog-text",
		Paragraph: textpkg.Paragraph{
			Spans: []textpkg.TextSpan{
				{Text: "Catalog Lite - Basic Rendering", Style: textpkg.TextStyle{Size: 16}},
			},
		},
		MaxWidth:   300,
		Selectable: true,
	}
	col.AddChild(textMark.Base())

	return col
}

func solidFill(color gfx.Color) theme.Material {
	return theme.Material{
		Fills:   []theme.Fill{{Type: theme.FillSolid, Color: color, Opacity: 1}},
		Opacity: 1,
	}
}

func solidStroke(color gfx.Color, width float32) theme.MaterialStroke {
	return theme.MaterialStroke{
		Paint: theme.Fill{Type: theme.FillSolid, Color: color, Opacity: 1},
		Width: width,
	}
}

// ExportState returns scene state.
func (s *CatalogLiteScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":    s.id,
		"description": "Catalog lite rendering test",
	}
}

// Ensure CatalogLiteScene implements Scene interface
var _ scene.Scene = (*CatalogLiteScene)(nil)
