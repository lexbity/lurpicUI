package scenes

import (
	"fmt"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks/annotation"
	"codeburg.org/lexbit/lurpicui/marks/basic"
	"codeburg.org/lexbit/lurpicui/theme"
	"codeburg.org/lexbit/ui_diagnostic_scene/scene"
)

type AnnotationScene struct {
	BaseScene
	th      theme.Context
	target  *basic.Rect
	label   *annotation.Label
	badge   *annotation.Badge
	callout *annotation.Callout
	conn    *annotation.Connector
	handle  *annotation.Handle
}

func NewAnnotationScene() *AnnotationScene {
	s := &AnnotationScene{
		BaseScene: NewBaseScene(
			"annotation",
			"Annotation",
			"Validates labels, connectors, badges, and anchor export",
			[]string{"annotation"},
		),
		th: theme.Default(),
	}
	s.capability.SupportsSnapshot = true
	return s
}

func (s *AnnotationScene) BuildRoot() facet.FacetImpl {
	if s.root != nil {
		return s.root
	}
	stack := layout.NewStackLayout(layout.AlignStart)
	s.root = stack

	s.target = &basic.Rect{
		ID:     "annotation-target",
		Bounds: basic.BoundsProps{X: 120, Y: 90, W: 220, H: 130},
		Radius: 14,
		Style: basic.PrimitiveStyleProps{
			Fill:    solidFill(s.th.Color(theme.ColorSurface)),
			Stroke:  solidStroke(s.th.Color(theme.ColorPrimary), 2),
			Visible: true,
			Opacity: 1,
		},
	}
	stack.AddChild(s.target.Base())

	s.label = &annotation.Label{
		ID:         "annotation-label",
		Placement:  annotation.LabelAnchorAttached,
		AnchorRef:  &annotation.AnchorSourceRef{MarkID: s.target.ID, Anchor: "bounds-center"},
		Padding:    gfx.Insets{Top: 6, Right: 10, Bottom: 6, Left: 10},
		Background: true,
		Halo:       true,
		Offset:     gfx.Point{X: 18, Y: -40},
		Text:       *newTextMark("annotation-label-text", "Anchored label", 13),
	}
	stack.AddChild(s.label.Base())

	s.badge = &annotation.Badge{
		ID:     "annotation-badge",
		Host:   annotation.AnchorSourceRef{MarkID: s.target.ID, Anchor: "top-right"},
		Offset: gfx.Point{X: 16, Y: -8},
		Content: newTextMark(
			"annotation-badge-text",
			"New",
			11,
		),
	}
	stack.AddChild(s.badge.Base())

	s.callout = &annotation.Callout{
		ID:        "annotation-callout",
		Target:    annotation.AnchorSourceRef{MarkID: s.target.ID, Anchor: "bottom-left"},
		Direction: annotation.CalloutRight,
		Offset:    gfx.Point{X: 48, Y: 20},
		WithLine:  true,
		Body:      newTextMark("annotation-callout-body", "Callout body", 12),
	}
	stack.AddChild(s.callout.Base())

	s.conn = &annotation.Connector{
		ID:   "annotation-connector",
		Mode: annotation.ConnectorOrthogonal,
		From: annotation.ConnectorEndpoint{Source: annotation.AnchorSourceRef{MarkID: s.target.ID, Anchor: "right"}},
		To:   annotation.ConnectorEndpoint{Source: annotation.AnchorSourceRef{MarkID: s.target.ID, Anchor: "bottom-right"}},
		Label: &annotation.Label{
			ID:         "annotation-connector-label",
			Placement:  annotation.LabelFree,
			Padding:    gfx.Insets{Top: 4, Right: 8, Bottom: 4, Left: 8},
			Background: true,
			Text:       *newTextMark("annotation-connector-text", "Connector", 11),
			Offset:     gfx.Point{X: 260, Y: 60},
		},
	}
	stack.AddChild(s.conn.Base())

	s.handle = &annotation.Handle{
		ID:        "annotation-handle",
		Position:  gfx.Point{X: 360, Y: 108},
		Size:      12,
		Focusable: true,
		Shape:     annotation.HandleCircle,
	}
	stack.AddChild(s.handle.Base())

	return stack
}

func (s *AnnotationScene) ApplyTheme(th theme.Context) {
	s.th = th
	if s.target != nil {
		tintRectStyle(s.target, th.Color(theme.ColorSurface))
	}
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyProjection)
	}
}

func (s *AnnotationScene) ApplyDensity(scale float32) {
	if s.root != nil && s.root.Base() != nil {
		s.root.Base().Invalidate(facet.DirtyLayout | facet.DirtyProjection)
	}
}

func (s *AnnotationScene) Reset() {
	s.BaseScene.Reset()
}

func (s *AnnotationScene) ExportState() map[string]any {
	return map[string]any{
		"scene_id": s.id,
		"target":   fmt.Sprintf("%s", s.target.ID),
	}
}

func (s *AnnotationScene) ImportState(state map[string]any) {}

var _ scene.Scene = (*AnnotationScene)(nil)
