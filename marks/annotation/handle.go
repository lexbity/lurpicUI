package annotation

import (
	"math"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// HandleShape selects the visual handle geometry.
type HandleShape uint8

const (
	HandleCircle HandleShape = iota
	HandleSquare
	HandleDiamond
)

// Handle is an editor affordance with a larger hit target than visual target.
type Handle struct {
	ID           string
	Position     gfx.Point
	Size         float32
	HitExpansion float32
	Shape        HandleShape
	Focusable    bool
	Draggable    bool
	Style        theme.MarkStyle
	State        theme.InteractionState

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
	projection   *facet.ProjectionRole
	hitRole      *facet.HitRole
	focusRole    *facet.FocusRole
	inputRole    *facet.InputRole
}

func init() {
	registerAnnotationDescriptor(marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:handle"),
		Focusable:         true,
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (h *Handle) Base() *facet.Facet { h.ensureInit(); return &h.base }

func (h *Handle) Descriptor() marks.Descriptor {
	return marks.Descriptor{
		Family:            marks.FamilyAnnotation,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("annotation:handle"),
		Focusable:         h.Focusable,
		HitTestable:       true,
		AnchorExporting:   true,
	}
}

func (h *Handle) AuthoredID() string { return h.ID }
func (h *Handle) CanFocus() bool     { return h.Focusable }
func (h *Handle) OnAttach(ctx facet.AttachContext) {
	h.syncRoles()
}
func (h *Handle) OnDetach()     {}
func (h *Handle) OnActivate()   {}
func (h *Handle) OnDeactivate() {}
func (h *Handle) SupportsSubpartCustomization() bool { return false }

func (h *Handle) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	h.ensureInit()
	size := h.visualSize()
	anchors := layout.AnchorSet{
		"bounds-center": {X: h.Position.X, Y: h.Position.Y},
		"center":        {X: h.Position.X, Y: h.Position.Y},
		"top":           {X: h.Position.X, Y: h.Position.Y - size/2},
		"right":         {X: h.Position.X + size/2, Y: h.Position.Y},
		"bottom":        {X: h.Position.X, Y: h.Position.Y + size/2},
		"left":          {X: h.Position.X - size/2, Y: h.Position.Y},
	}
	transform := gfx.Identity()
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform
	}
	return transformAnchors(transform, anchors)
}

func (h *Handle) ensureInit() {
	h.once.Do(func() {
		h.base.BindImpl(h)
		h.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			size := h.visualSize() + h.HitExpansion*2
			return gfx.Size{W: size, H: size}
		}}
		h.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		h.projection = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList { return h.project(ctx) }}
		h.hitRole = &facet.HitRole{OnHitTest: func(p gfx.Point) facet.HitResult {
			if h.hitTestLocal(p) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorGrab}
			}
			return facet.HitResult{}
		}}
		h.focusRole = &facet.FocusRole{
			Focusable: func() bool { return h.Focusable },
			TabIndex:  0,
		}
		h.inputRole = &facet.InputRole{
			OnPointer: func(e facet.PointerEvent) bool { return h.Draggable },
		}
		h.base.AddRole(h.layoutRole)
		h.base.AddRole(h.viewportRole)
		h.base.AddRole(h.projection)
		h.base.AddRole(h.hitRole)
		h.base.AddRole(h.focusRole)
		h.base.AddRole(h.inputRole)
		h.syncRoles()
	})
}

func (h *Handle) syncRoles() {
	syncLayout(h.layoutRole, shapeRect(h.Position, h.visualSize()+h.HitExpansion*2))
	syncViewport(h.viewportRole, gfx.Identity())
}

func (h *Handle) visualSize() float32 {
	if h.Size > 0 {
		return h.Size
	}
	return 12
}

func (h *Handle) project(ctx facet.ProjectionContext) *gfx.CommandList {
	size := h.visualSize()
	half := size / 2
	path := handlePath(h.Shape, half)
	material := h.Style.Resolve(h.State, theme.DefaultTokens())
	var list gfx.CommandList
	list.Add(gfx.PushTransform{Matrix: gfx.Translation(h.Position.X, h.Position.Y)})
	for _, fill := range material.Fills {
		if fill.Type != theme.FillNone {
			list.Add(gfx.FillPath{Path: path, Brush: gfx.SolidBrush(fill.Color)})
		}
	}
	if len(material.Strokes) > 0 {
		stroke := material.Strokes[0]
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(stroke), Brush: strokeBrushFromMaterial(stroke, 1)})
	}
	list.Add(gfx.PopTransform{})
	return &list
}

func (h *Handle) hitTestLocal(p gfx.Point) bool {
	radius := h.visualSize()/2 + h.HitExpansion
	switch h.Shape {
	case HandleSquare:
		return math.Abs(float64(p.X-h.Position.X)) <= float64(radius) && math.Abs(float64(p.Y-h.Position.Y)) <= float64(radius)
	case HandleDiamond:
		return math.Abs(float64(p.X-h.Position.X))+math.Abs(float64(p.Y-h.Position.Y)) <= float64(radius)
	default:
		dx := p.X - h.Position.X
		dy := p.Y - h.Position.Y
		return float32(math.Hypot(float64(dx), float64(dy))) <= radius
	}
}

func handlePath(shape HandleShape, half float32) gfx.Path {
	switch shape {
	case HandleSquare:
		return gfx.RectPath(gfx.RectFromXYWH(-half, -half, half*2, half*2))
	case HandleDiamond:
		return pathFromPoints([]gfx.Point{
			{X: 0, Y: -half},
			{X: half, Y: 0},
			{X: 0, Y: half},
			{X: -half, Y: 0},
		}, true)
	default:
		return gfx.CirclePath(gfx.Point{}, half)
	}
}
