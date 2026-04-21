package basic

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// PrimitiveStyleProps groups the shared fill/stroke/visibility controls used by the primitive marks.
type PrimitiveStyleProps struct {
	Fill    theme.Material
	Stroke  theme.MaterialStroke
	Opacity float32
	Visible bool
}

// TransformProps groups local transform state.
type TransformProps struct {
	Transform gfx.Transform
}

// BoundsProps stores a rectangle in local coordinates.
type BoundsProps struct {
	X, Y, W, H float32
}

func (b BoundsProps) Rect() gfx.Rect {
	return gfx.RectFromXYWH(b.X, b.Y, b.W, b.H)
}

type primitiveFacet struct {
	facet.Facet
	descriptor marks.Descriptor
}

func (p *primitiveFacet) Base() *facet.Facet {
	if p == nil {
		return nil
	}
	p.Facet.BindImpl(p)
	return &p.Facet
}

func (p *primitiveFacet) Descriptor() marks.Descriptor {
	if p == nil {
		return marks.Descriptor{}
	}
	return p.descriptor
}

func (p *primitiveFacet) AuthoredID() string {
	return ""
}

func (p *primitiveFacet) OnAttach(ctx facet.AttachContext) {}
func (p *primitiveFacet) OnDetach()                        {}
func (p *primitiveFacet) OnActivate()                      {}
func (p *primitiveFacet) OnDeactivate()                    {}
func (p *primitiveFacet) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	return nil
}

func registerPrimitiveDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	if layoutRole == nil {
		return
	}
	layoutRole.Arrange(bounds)
	layoutRole.MeasuredSize = gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	if viewport == nil {
		return
	}
	viewport.Transform = transform
}

func applyTransform(tx TransformProps, p gfx.Point) gfx.Point {
	return tx.Transform.TransformPoint(p)
}

func normalizeTransform(t gfx.Transform) gfx.Transform {
	if t == (gfx.Transform{}) {
		return gfx.Identity()
	}
	return t
}

func inverseTransform(tx TransformProps) (gfx.Transform, bool) {
	return normalizeTransform(tx.Transform).Inverse()
}

func transformRect(tx TransformProps, r gfx.Rect) gfx.Rect {
	return tx.Transform.TransformRect(r)
}

func emptyStyleVisible(style PrimitiveStyleProps) bool {
	if !style.Visible {
		return false
	}
	if style.Opacity <= 0 {
		return false
	}
	return true
}

func colorBrush(fill theme.Fill, opacity float32) gfx.Brush {
	switch fill.Type {
	case theme.FillSolid, theme.FillNone:
		if opacity > 0 && opacity != 1 {
			fill.Color = fill.Color.WithAlpha(fill.Color.A * fill.Opacity * opacity)
		} else {
			fill.Color = fill.Color.WithAlpha(fill.Color.A * fill.Opacity)
		}
		return gfx.SolidBrush(fill.Color)
	case theme.FillGradient:
		stops := make([]gfx.GradientStop, len(fill.Gradient.Stops))
		for i := range fill.Gradient.Stops {
			stops[i] = gfx.GradientStop{
				Offset: fill.Gradient.Stops[i].Position,
				Color:  fill.Gradient.Stops[i].Color,
			}
		}
		return gfx.LinearGradientBrush(fill.Gradient.Start, fill.Gradient.End, stops)
	default:
		return gfx.SolidBrush(fill.Color)
	}
}

func strokeBrush(fill theme.Fill, opacity float32) gfx.Brush {
	color := fill.Color
	scale := fill.Opacity * opacity
	color = color.WithAlpha(color.A * scale)
	return gfx.SolidBrush(color)
}

func strokeBrushFromMaterial(stroke theme.MaterialStroke, opacity float32) gfx.Brush {
	return strokeBrush(stroke.Paint, opacity)
}

func strokeStyle(stroke theme.MaterialStroke) gfx.StrokeStyle {
	style := gfx.DefaultStroke(stroke.Width)
	switch stroke.Cap {
	case theme.CapRound:
		style.Cap = gfx.LineCapRound
	case theme.CapSquare:
		style.Cap = gfx.LineCapSquare
	default:
		style.Cap = gfx.LineCapButt
	}
	switch stroke.Join {
	case theme.JoinRound:
		style.Join = gfx.LineJoinRound
	case theme.JoinBevel:
		style.Join = gfx.LineJoinBevel
	default:
		style.Join = gfx.LineJoinMiter
	}
	style.Dash = append([]float32(nil), stroke.Dash...)
	style.DashOffset = stroke.DashOffset
	return style
}

func attachPrimitiveRoles(base *primitiveFacet, layoutRole *facet.LayoutRole, viewportRole *facet.ViewportRole, projectionRole *facet.ProjectionRole, hitRole *facet.HitRole) {
	if base == nil {
		return
	}
	if layoutRole != nil {
		base.AddRole(layoutRole)
	}
	if viewportRole != nil {
		base.AddRole(viewportRole)
	}
	if projectionRole != nil {
		base.AddRole(projectionRole)
	}
	if hitRole != nil {
		base.AddRole(hitRole)
	}
}

var _ marks.Mark = (*primitiveFacet)(nil)
var _ layout.AnchorExporter = (*primitiveFacet)(nil)
