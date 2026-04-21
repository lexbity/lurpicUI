package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// LayerMount mounts one child into a specific parent-scoped layer.
// It panics if TargetLayer is zero because that is always a programming error.
type LayerMount struct {
	ID          string
	TargetLayer layout.LayerID
	Child       marks.Mark

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
		Type:              marks.TypeName("structure:layermount"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (m *LayerMount) Base() *facet.Facet { m.ensureInit(); return &m.base }

func (m *LayerMount) Descriptor() marks.Descriptor {
	m.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:layermount"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (m *LayerMount) AuthoredID() string { return m.ID }
func (m *LayerMount) OnAttach(ctx facet.AttachContext) {
	m.syncRoles()
}
func (m *LayerMount) OnDetach()     {}
func (m *LayerMount) OnActivate()   {}
func (m *LayerMount) OnDeactivate() {}

func (m *LayerMount) OnLayerSpecs() []layout.LayerSpec {
	if m.TargetLayer == 0 {
		panic("marks/structure: LayerMount requires a non-zero TargetLayer")
	}
	return []layout.LayerSpec{{
		ID:          m.TargetLayer,
		Placement:   layout.PlacementStack,
		Measurement: layout.MeasureStructural,
		CoordSpace:  layout.CoordParentLayout,
		HitPolicy:   layout.HitNormal,
		RenderOrder: int(m.TargetLayer),
		ClipPolicy:  layout.ClipNone,
	}}
}

func (m *LayerMount) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	m.ensureInit()
	if child := m.childImpl(); child != nil {
		if exporter, ok := child.(layout.AnchorExporter); ok {
			anchors := exporter.ExportAnchors(ctx)
			if len(anchors) > 0 {
				return anchors
			}
		}
	}
	bounds, ok := unionDescendantBounds(&m.base)
	if !ok {
		return nil
	}
	return boundsAnchors(bounds)
}

func (m *LayerMount) ensureInit() {
	m.once.Do(func() {
		if m.base.ID() == 0 {
			m.base = facet.NewFacet()
		}
		m.base.BindImpl(m)
		m.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				if child := m.childImpl(); child != nil {
					if lr := child.Base().LayoutRole(); lr != nil {
						return lr.Measure(c)
					}
				}
				return gfx.Size{}
			},
		}
		m.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		m.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return &gfx.CommandList{}
		}}
		m.base.AddRole(m.layoutRole)
		m.base.AddRole(m.viewportRole)
		m.base.AddRole(m.projectionRole)
		attachSingleChild(&m.base, m.Child)
		syncLayout(m.layoutRole, m.localBounds())
		syncViewport(m.viewportRole, gfx.Identity())
	})
}

func (m *LayerMount) syncRoles() {
	syncLayout(m.layoutRole, m.localBounds())
	syncViewport(m.viewportRole, gfx.Identity())
}

func (m *LayerMount) localBounds() gfx.Rect {
	bounds, ok := unionDescendantBounds(&m.base)
	if !ok {
		return gfx.Rect{}
	}
	return bounds
}

func (m *LayerMount) childImpl() facet.FacetImpl {
	if m == nil || m.base.ID() == 0 {
		return nil
	}
	children := m.base.Children()
	if len(children) == 0 {
		return nil
	}
	return children[0].Impl()
}
