package scenes

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	mchart "codeburg.org/lexbit/lurpicui/marks/chart"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type ChartScene struct {
	BaseScene
	th     theme.Context
	axes   []*mchart.Axis
	plot   *basic.Rect
	notes  *basic.Text
	scaleY float32
}

func NewChartScene() *ChartScene {
	s := &ChartScene{
		BaseScene: NewBaseScene(
			"chart",
			"Chart",
			"Validates axes, scaling, and chart-family marks",
			[]string{"chart"},
		),
		th:     theme.Default(),
		scaleY: 1,
	}
	s.capability.SupportsDensity = true
	return s
}

func (s *ChartScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	stack := layout.NewStackLayout(layout.AlignStart)
	s.root = stack

	s.plot = &basic.Rect{
		ID:     "chart-plot",
		Bounds: basic.BoundsProps{X: 40, Y: 24, W: 360, H: 220},
		Radius: 10,
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(s.th.Color(theme.ColorSurface)),
			Stroke:  solidStroke(s.th.Color(theme.ColorBorder), 1),
			Visible: true,
			Opacity: 1,
		},
	}
	stack.AddChild(s.plot.Base())

	leftAxis := &mchart.Axis{
		ID:          "chart-left-axis",
		Orientation: mchart.AxisLeft,
		Scale: mchart.LinearScale{
			DomainMin: 0,
			DomainMax: 100,
			RangeMin:  24,
			RangeMax:  200,
			Precision: 0,
		},
		ShowGrid: true,
		Title:    "Value",
	}
	bottomAxis := &mchart.Axis{
		ID:          "chart-bottom-axis",
		Orientation: mchart.AxisBottom,
		Scale: mchart.LinearScale{
			DomainMin: 0,
			DomainMax: 12,
			RangeMin:  56,
			RangeMax:  360,
			Precision: 0,
		},
		ShowGrid: true,
		Title:    "Samples",
	}
	s.axes = []*mchart.Axis{leftAxis, bottomAxis}
	stack.AddChild(leftAxis.Base())
	stack.AddChild(bottomAxis.Base())

	s.notes = newTextMark("chart-notes", "Linear axes with grid lines and density-aware viewport bounds.", 12)
	stack.AddChild(s.notes.Base())

	return stack
}

func (s *ChartScene) ApplyTheme(th theme.Context) {
	s.th = th
	if s.plot != nil {
		tintRectStyle(s.plot, th.Color(theme.ColorSurface))
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *ChartScene) ApplyDensity(scale float32) {
	if scale <= 0 {
		scale = 1
	}
	s.scaleY = scale
	if s.plot != nil {
		s.plot.Bounds.W = 360 * scale
		s.plot.Bounds.H = 220 * scale
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *ChartScene) Reset() {
	s.scaleY = 1
	s.BaseScene.Reset()
}

func (s *ChartScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": s.id,
		"scale_y":  s.scaleY,
		"axes":     len(s.axes),
	}
}

func (s *ChartScene) ImportState(state map[string]any) {
	if v, ok := state["scale_y"].(float64); ok {
		s.scaleY = float32(v)
	}
}

var _ scene.Scene = (*ChartScene)(nil)
