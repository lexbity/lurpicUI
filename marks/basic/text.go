package basic

import (
	"sync"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"codeburg.org/lexbit/lurpicui/text"
)

// Text is a primitive authored text mark.
type Text struct {
	ID         string
	Paragraph  text.Paragraph
	Style      text.TextStyle
	MaxWidth   float32
	Align      text.TextAlignment
	Selectable bool
	Tx         TransformProps

	base            primitiveFacet
	once            sync.Once
	layoutRole      *facet.LayoutRole
	viewportRole    *facet.ViewportRole
	projectionRole  *facet.ProjectionRole
	hitRole         *facet.HitRole
	textRole        *facet.TextRole
	layoutBuilds    int
	layoutCacheHits int
}

func init() {
	registerPrimitiveDescriptor(marks.Descriptor{
		Family:            marks.FamilyBasic,
		ConstructionClass: marks.ConstructionPrimitive,
		Type:              marks.TypeName("basic:text"),
		HitTestable:       true,
		AnchorExporting:   true,
	})
}

func (t *Text) Base() *facet.Facet               { t.ensureInit(); return t.base.Base() }
func (t *Text) Descriptor() marks.Descriptor     { t.ensureInit(); return t.base.Descriptor() }
func (t *Text) AuthoredID() string               { return t.ID }
func (t *Text) OnAttach(ctx facet.AttachContext) { t.syncRoles() }
func (t *Text) OnDetach()                        {}
func (t *Text) OnActivate()                      {}
func (t *Text) OnDeactivate()                    {}

func (t *Text) ExportAnchors(ctx layout.AnchorExportContext) layout.AnchorSet {
	t.ensureInit()
	resolved := t.resolveLayout()
	if resolved == nil || len(resolved.Lines) == 0 {
		return nil
	}
	bounds := gfx.RectFromXYWH(resolved.Bounds.Min.X, resolved.Bounds.Min.Y, resolved.Bounds.Width(), resolved.Bounds.Height())
	anchors := layout.AnchorSet{
		"bounds-center":  {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":       {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":      {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":   {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":    {X: bounds.Min.X, Y: bounds.Max.Y},
		"baseline-start": {X: resolved.Lines[0].Bounds.Min.X, Y: resolved.Lines[0].Bounds.Min.Y + resolved.Lines[0].Baseline},
		"baseline-end":   {X: resolved.Lines[0].Bounds.Max.X, Y: resolved.Lines[0].Bounds.Min.Y + resolved.Lines[0].Baseline},
	}
	transform := normalizeTransform(t.Tx.Transform)
	if ctx.Viewport != (layout.Viewport{}) {
		transform = ctx.Viewport.Transform.Multiply(transform)
	}
	return transformAnchors(transform, anchors)
}

func (t *Text) HitTest(world gfx.Point) bool {
	return t.HitPosition(world).Index >= 0
}

// HitPosition returns the nearest text position for a world-space point.
func (t *Text) HitPosition(world gfx.Point) text.TextPosition {
	t.ensureInit()
	layout := t.resolveLayout()
	if layout == nil {
		return text.TextPosition{}
	}
	inv, ok := inverseTransform(t.Tx)
	if !ok {
		return text.TextPosition{}
	}
	pt := inv.TransformPoint(world)
	return layout.HitTest(text.Point{X: pt.X, Y: pt.Y})
}

func (t *Text) ensureInit() {
	t.once.Do(func() {
		t.base.descriptor = marks.Descriptor{Family: marks.FamilyBasic, ConstructionClass: marks.ConstructionPrimitive, Type: marks.TypeName("basic:text"), HitTestable: true, AnchorExporting: true}
		t.layoutRole = &facet.LayoutRole{
			OnMeasure: func(c facet.Constraints) gfx.Size {
				layout := t.resolveLayout()
				if layout == nil {
					return gfx.Size{}
				}
				return gfx.Size{W: layout.Bounds.Width(), H: layout.Bounds.Height()}
			},
			OnArrange: func(bounds gfx.Rect) {
				// Text is laid out at the origin; arranging only confirms the resolved size.
				_ = bounds
			},
		}
		t.viewportRole = &facet.ViewportRole{Transform: normalizeTransform(t.Tx.Transform)}
		t.projectionRole = &facet.ProjectionRole{OnProject: func(ctx facet.ProjectionContext) *gfx.CommandList {
			return t.project(ctx)
		}}
		t.hitRole = &facet.HitRole{OnHitTest: func(pt gfx.Point) facet.HitResult {
			layout := t.resolveLayout()
			if layout == nil {
				return facet.HitResult{}
			}
			_ = layout.HitTest(text.Point{X: pt.X, Y: pt.Y})
			return facet.HitResult{Hit: true, MarkID: 0, Cursor: facet.CursorText}
		}}
		t.textRole = &facet.TextRole{
			CaretVisible: t.Selectable,
		}
		attachPrimitiveRoles(&t.base, t.layoutRole, t.viewportRole, t.projectionRole, t.hitRole)
		t.base.AddRole(t.textRole)
		syncLayout(t.layoutRole, t.localBounds())
		syncViewport(t.viewportRole, normalizeTransform(t.Tx.Transform))
	})
}

func (t *Text) syncRoles() {
	syncLayout(t.layoutRole, t.localBounds())
	syncViewport(t.viewportRole, normalizeTransform(t.Tx.Transform))
}

func (t *Text) localBounds() gfx.Rect {
	layout := t.resolveLayout()
	if layout == nil {
		return gfx.Rect{}
	}
	return gfx.RectFromXYWH(layout.Bounds.Min.X, layout.Bounds.Min.Y, layout.Bounds.Width(), layout.Bounds.Height())
}

func (t *Text) resolveLayout() *text.TextLayout {
	reg := currentTextRegistry()
	key := textLayoutCacheKey(t.paragraph(), t.Style, t.MaxWidth, t.Selectable, reg)
	if layout, ok := lookupTextLayout(key); ok {
		t.layoutCacheHits++
		return layout
	}
	t.layoutBuilds++
	layout := cachedTextLayout(key, func() *text.TextLayout {
		registry := reg
		if registry == nil {
			registry, _ = text.NewFontRegistry()
		}
		shaper := text.NewShaper(registry)
		paragraph := t.paragraph()
		paragraph.MaxWidth = t.MaxWidth
		paragraph.Alignment = t.Align
		return shaper.Shape(paragraph)
	})
	return layout
}

func (t *Text) paragraph() text.Paragraph {
	p := t.Paragraph
	if len(p.Spans) == 0 && t.Style.Family != "" {
		p.Spans = []text.TextSpan{{Text: "", Style: t.Style}}
	}
	for i := range p.Spans {
		if p.Spans[i].Style.Size <= 0 {
			p.Spans[i].Style = t.Style
			if p.Spans[i].Style.Size <= 0 {
				p.Spans[i].Style = text.DefaultStyle()
			}
		}
	}
	return p
}

func (t *Text) project(ctx facet.ProjectionContext) *gfx.CommandList {
	layout := t.resolveLayout()
	if layout == nil {
		return &gfx.CommandList{}
	}
	if t.textRole != nil {
		t.textRole.Layout = layout
		if t.Selectable {
			t.textRole.Selection = text.TextRange{Start: 0, End: layout.RuneCount()}
			t.textRole.CaretVisible = true
			t.textRole.CaretPosition = text.TextPosition{Index: layout.RuneCount(), Affinity: text.AffinityUpstream}
		} else {
			t.textRole.Selection = text.TextRange{}
			t.textRole.CaretVisible = false
		}
	}
	var list gfx.CommandList
	for _, line := range layout.Lines {
		origin := gfx.Point{X: line.Bounds.Min.X, Y: line.Bounds.Min.Y}
		for _, run := range line.Runs {
			list.Add(gfx.DrawGlyphRun{
				Run:    run,
				Origin: origin,
				Brush:  gfx.SolidBrush(gfx.Color{A: 1}),
			})
		}
	}
	return &list
}
