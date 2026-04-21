package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// ViewportModel represents a projected-space viewport contract for descendants.
type ViewportModel struct {
	Bounds    gfx.Rect
	Transform gfx.Transform
}

// ViewportHost establishes a projected/world-space subtree root.
type ViewportHost struct {
	ID       string
	Viewport ViewportModel
	Children []marks.Mark

	base           facet.Facet
	once           sync.Once
	layoutRole     *facet.LayoutRole
	viewportRole   *facet.ViewportRole
	projectionRole *facet.ProjectionRole
}

func init() {
	registerStructureDescriptor(marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:viewporthost"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (v *ViewportHost) Base() *facet.Facet { v.ensureInit(); return &v.base }

func (v *ViewportHost) Descriptor() marks.Descriptor {
	v.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:viewporthost"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (v *ViewportHost) AuthoredID() string { return v.ID }
func (v *ViewportHost) OnAttach(ctx facet.AttachContext) {
	v.syncRoles()
}
func (v *ViewportHost) OnDetach()     {}
func (v *ViewportHost) OnActivate()   {}
func (v *ViewportHost) OnDeactivate() {}

func (v *ViewportHost) OnLayerSpecs() []layout.LayerSpec {
	bounds := v.Viewport.Bounds
	if bounds.IsEmpty() {
		return nil
	}
	return []layout.LayerSpec{{
		ID:          1,
		Placement:   layout.PlacementProjected,
		Measurement: layout.MeasureNonStructural,
		CoordSpace:  layout.CoordViewport,
		CoordLimits: layout.CoordLimits{Bounds: bounds},
		HitPolicy:   layout.HitNormal,
		RenderOrder: 0,
		ClipPolicy:  layout.ClipToViewport,
	}}
}

func (v *ViewportHost) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	v.ensureInit()
	bounds := v.Viewport.Bounds
	if bounds.IsEmpty() {
		bounds, _ = unionDescendantBounds(&v.base)
	}
	if bounds.IsEmpty() {
		return nil
	}
	transform := normaliseTransform(v.Viewport.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, boundsAnchors(bounds))
}

func (v *ViewportHost) ensureInit() {
	v.once.Do(func() {
		if v.base.ID() == 0 {
			v.base = facet.NewFacet()
		}
		v.base.BindImpl(v)
		v.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				if !v.Viewport.Bounds.IsEmpty() {
					return gfx.Size{W: v.Viewport.Bounds.Width(), H: v.Viewport.Bounds.Height()}
				}
				bounds, ok := unionDescendantBounds(&v.base)
				if !ok {
					return gfx.Size{}
				}
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
		}
		v.viewportRole = &facet.ViewportRole{Transform: normaliseTransform(v.Viewport.Transform)}
		v.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return &gfx.CommandList{}
		}}
		v.base.AddRole(v.layoutRole)
		v.base.AddRole(v.viewportRole)
		v.base.AddRole(v.projectionRole)
		attachChildMarks(&v.base, v.Children)
		syncLayout(v.layoutRole, v.localBounds())
		syncViewport(v.viewportRole, normaliseTransform(v.Viewport.Transform))
	})
}

func (v *ViewportHost) syncRoles() {
	syncLayout(v.layoutRole, v.localBounds())
	syncViewport(v.viewportRole, normaliseTransform(v.Viewport.Transform))
}

func (v *ViewportHost) localBounds() gfx.Rect {
	if !v.Viewport.Bounds.IsEmpty() {
		return v.Viewport.Bounds
	}
	bounds, ok := unionDescendantBounds(&v.base)
	if !ok {
		return gfx.Rect{}
	}
	return bounds
}
