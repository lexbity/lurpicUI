package scenes

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type ThemeScene struct {
	BaseScene
	th           theme.Context
	densityScale float32
	swatches     []*basic.Rect
	labels       []*basic.Text
}

func NewThemeScene() *ThemeScene {
	s := &ThemeScene{
		BaseScene: NewBaseScene(
			"theme",
			"Theme",
			"Validates token propagation, state colors, and density shifts",
			[]string{"basic"},
		),
		th:           theme.Default(),
		densityScale: 1,
	}
	s.capability.SupportsThemeSwitch = true
	s.capability.SupportsDensity = true
	return s
}

func (s *ThemeScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	col := layout.NewColumnLayout()
	s.root = col
	s.swatches = nil
	s.labels = nil

	header := newTextMark("theme-title", "Theme tokens and density", 18)
	col.AddChild(header.Base())

	samples := themeSampleColors(s.th)
	labels := []string{"Background", "Surface", "Primary", "Selection"}
	row := layout.NewRowLayout()
	row.Gap = 12
	for i, color := range samples {
		swatch := &basic.Rect{
			ID:     fmt.Sprintf("theme-swatch-%d", i),
			Bounds: basic.BoundsProps{X: 0, Y: 0, W: 92, H: 64},
			Radius: 10,
			Style: basic.PrimitiveStyleProps{
				Fill:    solidFill(color),
				Stroke:  solidStroke(gfx.ColorFromRGBA8(0, 0, 0, 255), 1),
				Visible: true,
				Opacity: 1,
			},
		}
		label := newTextMark(fmt.Sprintf("theme-label-%d", i), labels[i], 12)
		s.swatches = append(s.swatches, swatch)
		s.labels = append(s.labels, label)

		stack := layout.NewStackLayout(layout.AlignStart)
		stack.AddChild(swatch.Base())
		stack.AddChild(label.Base())
		row.Add(layout.Fixed(stack.Base()))
	}
	col.AddChild(row.Base())

	note := newTextMark("theme-note", "ApplyTheme updates the swatches; ApplyDensity scales the layout.", 13)
	col.AddChild(note.Base())

	s.applyDensity()
	return col
}

func (s *ThemeScene) ApplyTheme(th theme.Context) {
	s.th = th
	if len(s.swatches) == 0 {
		return
	}
	colors := themeSampleColors(th)
	for i, swatch := range s.swatches {
		if i < len(colors) {
			tintRectStyle(swatch, colors[i])
		}
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *ThemeScene) ApplyDensity(scale float32) {
	if scale <= 0 {
		scale = 1
	}
	s.densityScale = scale
	s.applyDensity()
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *ThemeScene) Reset() {
	s.th = theme.Default()
	s.densityScale = 1
	s.BaseScene.Reset()
}

func (s *ThemeScene) applyDensity() {
	if len(s.swatches) == 0 {
		return
	}
	baseW := float32(92) * s.densityScale
	baseH := float32(64) * s.densityScale
	for _, swatch := range s.swatches {
		swatch.Bounds.W = baseW
		swatch.Bounds.H = baseH
	}
}

func (s *ThemeScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id":      s.id,
		"density_scale": s.densityScale,
		"swatch_count":  len(s.swatches),
	}
}

func (s *ThemeScene) ImportState(state map[string]any) {
	if v, ok := state["density_scale"].(float64); ok {
		s.densityScale = float32(v)
	}
}

var _ scene.Scene = (*ThemeScene)(nil)
