package basic

import (
	"fmt"
	"strings"
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/theme"
)

// FillRule selects the path fill rule.
type FillRule uint8

const (
	FillRuleNonZero FillRule = iota
	FillRuleEvenOdd
)

// Path is a primitive authored path mark.
type Path struct {
	ID       string
	Path     gfx.Path
	FillRule FillRule
	Style    PrimitiveStyleProps
	Tx       TransformProps

	base                 primitiveFacet
	once                 sync.Once
	layoutRole           *facet.LayoutRole
	viewportRole         *facet.ViewportRole
	projectionRole       *facet.ProjectionRole
	hitRole              *facet.HitRole
	cacheMu              sync.Mutex
	cachedSignature      string
	cachedBounds         gfx.Rect
	boundsRecomputeCount int
	boundsCacheHits      int
}

func init() {
	registerPrimitiveDescriptor(marks.Descriptor{
		Family:            marks.FamilyBasic,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("basic:path"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

// Base returns the embedded facet base.
func (p *Path) Base() *facet.Facet {
	p.ensureInit()
	return p.base.Base()
}

// Descriptor returns the path descriptor.
func (p *Path) Descriptor() marks.Descriptor {
	p.ensureInit()
	return p.base.Descriptor()
}

// AuthoredID returns the authored identifier.
func (p *Path) AuthoredID() string { return p.ID }

func (p *Path) OnAttach(ctx facet.AttachContext) { p.syncRoles() }
func (p *Path) OnDetach()                        {}
func (p *Path) OnActivate()                      {}
func (p *Path) OnDeactivate()                    {}

// ExportAnchors exports the path bounds anchors in world space.
func (p *Path) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	p.ensureInit()
	bounds := p.localBounds()
	transform := normalizeTransform(p.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, pathAnchorSet(gfx.RectPath(bounds)))
}

// HitTest returns whether the supplied world point lies within the path.
func (p *Path) HitTest(world gfx.Point) bool {
	p.ensureInit()
	inv, ok := inverseTransform(p.Tx)
	if !ok {
		return false
	}
	return p.hitTestLocal(inv.TransformPoint(world))
}

func (p *Path) ensureInit() {
	p.once.Do(func() {
		p.base.descriptor = marks.Descriptor{
			Family:            marks.FamilyBasic,
			ConstructionClass: marks.ConstructionPrimitive,
			Type:              marks.TypeName("basic:path"),
			HitTestable:       true,
			AnchorExporting:   true,
		}
		p.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				bounds := p.localBounds()
				return gfx.Size{W: bounds.Width(), H: bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				// Path geometry is authored directly; arranging only refreshes the cache.
				_ = bounds
			},
		}
		p.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(p.Tx.Transform)}
		p.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return p.project(ctx)
		}}
		p.hitRole = &facet.HitRole{OnHitTest: func(pos gfx.Point) facet.HitResult {
			if p.hitTestLocal(pos) {
				return facet.HitResult{Hit: true, Cursor: facet.CursorDefault}
			}
			return facet.HitResult{}
		}}
		attachPrimitiveRoles(&p.base, p.layoutRole, p.viewportRole, p.projectionRole, p.hitRole)
		syncLayout(p.layoutRole, p.localBounds())
		syncViewport(p.viewportRole, normalizeTransform(p.Tx.Transform))
	})
}

func (p *Path) syncRoles() {
	syncLayout(p.layoutRole, p.localBounds())
	syncViewport(p.viewportRole, normalizeTransform(p.Tx.Transform))
}

func (p *Path) localBounds() gfx.Rect {
	signature := pathSignature(p.Path)
	p.cacheMu.Lock()
	defer p.cacheMu.Unlock()
	if signature == p.cachedSignature {
		p.boundsCacheHits++
		return p.cachedBounds
	}
	p.cachedSignature = signature
	p.cachedBounds = pathBounds(p.Path)
	p.boundsRecomputeCount++
	return p.cachedBounds
}

func (p *Path) project(ctx facet.ProjectionContext) *gfx.CommandList {
	if !emptyStyleVisible(p.Style) {
		return &gfx.CommandList{}
	}
	var list gfx.CommandList
	path := p.Path
	for _, fill := range p.Style.Fill.Fills {
		if fill.Type == theme.FillNone {
			continue
		}
		list.Add(gfx.FillPath{Path: path, Brush: colorBrush(fill, p.Style.Opacity)})
	}
	if p.Style.Stroke.Width > 0 {
		stroke := p.Style.Stroke
		list.Add(gfx.StrokePath{Path: path, Stroke: strokeStyle(stroke), Brush: strokeBrushFromMaterial(stroke, p.Style.Opacity)})
	}
	return &list
}

func (p *Path) hitTestLocal(pos gfx.Point) bool {
	if len(p.Path.Segments) == 0 {
		return false
	}
	if len(p.Style.Fill.Fills) > 0 && p.Style.Fill.Fills[0].Type != theme.FillNone {
		switch p.FillRule {
		case FillRuleEvenOdd:
			if pathContains(p.Path, pos, true) {
				return true
			}
		default:
			if pathContains(p.Path, pos, false) {
				return true
			}
		}
	}
	if p.Style.Stroke.Width > 0 && pathStrokeHit(p.Path, pos, p.Style.Stroke.Width) {
		return true
	}
	return false
}

func pathSignature(path gfx.Path) string {
	if len(path.Segments) == 0 {
		return ""
	}
	var b strings.Builder
	for _, seg := range path.Segments {
		b.WriteByte(byte(seg.Verb))
		for _, pt := range seg.Pts {
			fmt.Fprintf(&b, "|%.4f,%.4f", pt.X, pt.Y)
		}
		b.WriteByte(';')
	}
	return b.String()
}
