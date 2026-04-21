package structure

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
)

// AnchorProxy forwards anchors from another mark and can rename or offset them.
type AnchorProxy struct {
	ID        string
	Source    AnchorSourceRef
	RenameMap map[string]string
	Offset    gfx.Point
	Children  []marks.Mark

	base         facet.Facet
	once         sync.Once
	layoutRole   *facet.LayoutRole
	viewportRole *facet.ViewportRole
}

func init() {
	registerStructureDescriptor(marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:anchorproxy"),
		AnchorExporting:   true,
		ChildHosting:      true,
	})
}

func (a *AnchorProxy) Base() *facet.Facet { a.ensureInit(); return &a.base }

func (a *AnchorProxy) Descriptor() marks.Descriptor {
	a.ensureInit()
	return marks.Descriptor{
		Family:            marks.FamilyStructure,
		ConstructionClass: marks.ConstructionComposed,
		Type:              marks.TypeName("structure:anchorproxy"),
		AnchorExporting:   true,
		ChildHosting:      true,
	}
}

func (a *AnchorProxy) AuthoredID() string               { return a.ID }
func (a *AnchorProxy) OnAttach(ctx facet.AttachContext) {}
func (a *AnchorProxy) OnDetach()                        {}
func (a *AnchorProxy) OnActivate()                      {}
func (a *AnchorProxy) OnDeactivate()                    {}

func (a *AnchorProxy) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	a.ensureInit()
	child := a.sourceImpl()
	if child == nil {
		return nil
	}
	exporter, ok := child.(layout.AnchorExporter)
	if !ok {
		return nil
	}
	src := exporter.ExportAnchors(ctx)
	if len(src) == 0 {
		return nil
	}
	if a.Source.Anchor != "" {
		if pt, ok := src[layout.AnchorID(a.Source.Anchor)]; ok {
			src = layout.AnchorSet{layout.AnchorID(a.Source.Anchor): pt}
		} else {
			return nil
		}
	}
	return renameAnchors(src, a.RenameMap, a.Offset)
}

func (a *AnchorProxy) ensureInit() {
	a.once.Do(func() {
		if a.base.ID() == 0 {
			a.base = facet.NewFacet()
		}
		a.base.BindImpl(a)
		a.layoutRole = &facet.LayoutRole{OnMeasure: func(c facet.Constraints) gfx.Size {
			if child := a.sourceImpl(); child != nil {
				if lr := child.Base().LayoutRole(); lr != nil {
					return lr.Measure(c)
				}
			}
			return gfx.Size{}
		}}
		a.viewportRole = &facet.ViewportRole{Transform: gfx.Identity()}
		a.base.AddRole(a.layoutRole)
		a.base.AddRole(a.viewportRole)
		attachChildMarks(&a.base, a.Children)
		syncLayout(a.layoutRole, gfx.Rect{})
		syncViewport(a.viewportRole, gfx.Identity())
	})
}

func (a *AnchorProxy) sourceImpl() facet.FacetImpl {
	if a == nil {
		return nil
	}
	if found := findMarkedChild(&a.base, a.Source.MarkID); found != nil {
		return found
	}
	children := a.base.Children()
	if len(children) == 1 {
		return children[0].Impl()
	}
	return nil
}
