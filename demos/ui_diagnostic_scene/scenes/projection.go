package scenes

import (
	"math"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/marks/structure"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

// ProjectionScene validates child transforms, hit regions, and viewport projection.
// This scene creates nested transformed elements to verify projection correctness.
type ProjectionScene struct {
	BaseScene
	transforms      []gfx.Transform
	hitTestResults  []HitTestResult
	showDiagnostics bool
}

// HitTestResult records a hit test outcome for diagnostics.
type HitTestResult struct {
	Point     gfx.Point
	Hit       bool
	MarkID    string
	Timestamp int64
}

// NewProjectionScene creates a new projection testing scene.
func NewProjectionScene() *ProjectionScene {
	s := &ProjectionScene{
		BaseScene: NewBaseScene(
			"projection",
			"Projection",
			"Validates child transforms, anchor forwarding, hit regions, and viewport projection",
			[]string{"structure"},
		),
		transforms:      make([]gfx.Transform, 0),
		hitTestResults:  make([]HitTestResult, 0),
		showDiagnostics: true,
	}
	s.capability.HasStressControls = true
	return s
}

// BuildRoot constructs the projection test UI.
func (s *ProjectionScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}

	col := layout.NewColumnLayout()
	s.root = col

	// Create a stack with nested transforms
	stack := layout.NewStackLayout(layout.AlignStart)

	// Base layer - untransformed, wrapped by structure.Transform
	base := s.createTransformedRect("base", gfx.Identity(), gfx.ColorFromRGBA8(200, 200, 200, 255))
	baseTransform := &structure.Transform{
		ID:       "projection-base-transform",
		Matrix:   gfx.Identity(),
		Children: []marks.Mark{base},
	}
	stack.AddChild(baseTransform.Base())

	// Rotated layer
	rotation := gfx.Rotation(15 * math.Pi / 180) // 15 degrees in radians
	rotated := s.createTransformedRect("rotated", rotation, gfx.ColorFromRGBA8(255, 100, 100, 200))
	rotatedTransform := &structure.Transform{
		ID:       "projection-rotated-transform",
		Matrix:   rotation,
		Children: []marks.Mark{rotated},
	}
	stack.AddChild(rotatedTransform.Base())

	// Scaled layer
	scale := gfx.Scale(1.5, 0.8)
	scaled := s.createTransformedRect("scaled", scale, gfx.ColorFromRGBA8(100, 255, 100, 200))
	scaledTransform := &structure.Transform{
		ID:       "projection-scaled-transform",
		Matrix:   scale,
		Children: []marks.Mark{scaled},
	}
	stack.AddChild(scaledTransform.Base())

	// Translated layer
	translation := gfx.Translation(50, 30)
	translated := s.createTransformedRect("translated", translation, gfx.ColorFromRGBA8(100, 100, 255, 200))
	translatedTransform := &structure.Transform{
		ID:       "projection-translated-transform",
		Matrix:   translation,
		Children: []marks.Mark{translated},
	}
	stack.AddChild(translatedTransform.Base())

	// Combined transform (translate + rotate + scale)
	// Apply transforms in reverse order of desired effect
	translate := gfx.Translation(25, 25)
	rotate := gfx.Rotation(30 * math.Pi / 180)
	scale2 := gfx.Scale(0.8, 1.2)
	combined := translate.Multiply(rotate).Multiply(scale2)
	complex := s.createTransformedRect("combined", combined, gfx.ColorFromRGBA8(255, 200, 100, 200))
	combinedTransform := &structure.Transform{
		ID:       "projection-combined-transform",
		Matrix:   combined,
		Children: []marks.Mark{complex},
	}
	stack.AddChild(combinedTransform.Base())

	col.AddChild(stack.Base())

	viewport := &structure.ViewportHost{
		ID: "projection-viewport",
		Viewport: structure.ViewportModel{
			Bounds:    gfx.RectFromXYWH(0, 0, 420, 320),
			Transform: gfx.Translation(12, 12),
		},
		Children: []marks.Mark{
			&basic.Rect{
				ID:     "projection-viewport-box",
				Bounds: basic.BoundsProps{X: 0, Y: 0, W: 160, H: 84},
				Style: basic.PrimitiveStyleProps{
					Fill:    solidFill(gfx.ColorFromRGBA8(220, 180, 240, 255)),
					Visible: true,
					Opacity: 1,
				},
			},
			&basic.Text{
				ID:        "projection-viewport-label",
				Paragraph: textParagraph("Viewport host projection"),
				MaxWidth:  200,
			},
		},
	}
	col.AddChild(viewport.Base())

	anchorSource := &structure.Transform{
		ID:     "projection-anchor-source",
		Matrix: gfx.Translation(300, 160),
		Children: []marks.Mark{
			&basic.Rect{
				ID:     "projection-anchor-box",
				Bounds: basic.BoundsProps{X: 0, Y: 0, W: 90, H: 60},
				Style: basic.PrimitiveStyleProps{
					Fill:    solidFill(gfx.ColorFromRGBA8(240, 220, 120, 255)),
					Visible: true,
					Opacity: 1,
				},
			},
		},
	}
	anchorProxy := &structure.AnchorProxy{
		ID:     "projection-anchor-proxy",
		Source: structure.AnchorSourceRef{MarkID: anchorSource.ID, Anchor: "bounds-center"},
		RenameMap: map[string]string{
			"bounds-center": "focus-center",
		},
		Offset:   gfx.Point{X: 28, Y: 12},
		Children: []marks.Mark{anchorSource},
	}
	col.AddChild(anchorProxy.Base())

	// Add hit test visualization layer
	hitTestLayer := s.createHitTestVisualization()
	col.AddChild(hitTestLayer.Base())

	return col
}

func (s *ProjectionScene) createTransformedRect(id string, transform gfx.Transform, color gfx.Color) *basic.Rect {
	rect := &basic.Rect{
		ID:     id,
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 100, H: 100},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(color),
			Stroke:  solidStroke(gfx.ColorFromRGBA8(0, 0, 0, 128), 1),
			Visible: true,
			Opacity: 1,
		},
	}

	// Store the authored transform proxy for diagnostics; the structure wrapper applies the actual transform.
	s.transforms = append(s.transforms, transform)

	// Add hit role with debug logging
	f := rect.Base()
	hitRole := &facet.HitRole{
		OnHitTest: func(p gfx.Point) facet.HitResult {
			localBounds := gfx.RectFromXYWH(0, 0, 100, 100)
			hit := localBounds.Contains(p)

			if s.showDiagnostics {
				result := HitTestResult{
					Point:  p,
					Hit:    hit,
					MarkID: id,
				}
				s.hitTestResults = append(s.hitTestResults, result)
				if len(s.hitTestResults) > 100 {
					s.hitTestResults = s.hitTestResults[1:]
				}
			}

			return facet.HitResult{
				Hit:    hit,
				MarkID: 0,
				Cursor: s.getCursorForHit(hit),
			}
		},
	}
	f.AddRole(hitRole)

	return rect
}

func (s *ProjectionScene) getCursorForHit(hit bool) facet.CursorShape {
	if hit {
		return facet.CursorPointer
	}
	return facet.CursorDefault
}

func (s *ProjectionScene) createHitTestVisualization() *basic.Rect {
	// Visual indicator for hit test coverage (semi-transparent overlay)
	overlay := &basic.Rect{
		ID:     "hit-overlay",
		Bounds: basic.BoundsProps{X: 0, Y: 0, W: 400, H: 400},
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(gfx.ColorFromRGBA8(0, 0, 0, 0)), // Transparent
			Visible: s.showDiagnostics,
			Opacity: 0.3,
		},
	}
	return overlay
}

// SetShowDiagnostics enables/disables diagnostic visualization.
func (s *ProjectionScene) SetShowDiagnostics(enabled bool) {
	s.showDiagnostics = enabled
	// Trigger rebuild to update visibility
	s.Reset()
}

// GetHitTestResults returns the recorded hit test results.
func (s *ProjectionScene) GetHitTestResults() []HitTestResult {
	return s.hitTestResults
}

// GetTransforms returns the registered transforms for inspection.
func (s *ProjectionScene) GetTransforms() []gfx.Transform {
	return s.transforms
}

// ClearHitTestResults clears the hit test history.
func (s *ProjectionScene) ClearHitTestResults() {
	s.hitTestResults = make([]HitTestResult, 0)
}

// Reset clears scene state.
func (s *ProjectionScene) Reset() {
	s.transforms = make([]gfx.Transform, 0)
	s.hitTestResults = make([]HitTestResult, 0)
	s.BaseScene.Reset()
}

// ExportState returns projection state.
func (s *ProjectionScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":         s.id,
		"show_diagnostics": s.showDiagnostics,
		"transform_count":  len(s.transforms),
		"hit_test_count":   len(s.hitTestResults),
	}
}

// ImportState restores projection state.
func (s *ProjectionScene) ImportState(state map[string]any) {
	if v, ok := state["show_diagnostics"].(bool); ok {
		s.showDiagnostics = v
	}
}

var _ scene.Scene = (*ProjectionScene)(nil)
