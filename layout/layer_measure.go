package layout

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	gridpolicy "codeburg.org/lexbit/lurpicui/layout/grid"
)

func (p *gridLayerPolicy) MeasureLayer(ctx LayerMeasureContext, children []LayerChild) (LayerMeasureResult, error) {
	if p == nil {
		return LayerMeasureResult{}, nil
	}
	policy := gridpolicy.New(layerGridConfig(p.recipe))
	size, err := policy.Measure(toGridChildren(children), gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()})
	if err != nil {
		return LayerMeasureResult{}, err
	}
	return LayerMeasureResult{Size: size}, nil
}

func (p *anchorLayerPolicy) MeasureLayer(ctx LayerMeasureContext, children []LayerChild) (LayerMeasureResult, error) {
	if p == nil {
		return LayerMeasureResult{}, nil
	}
	measureChildrenLayer(ctx, children, ctx.Bounds)
	size := gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}
	return LayerMeasureResult{Size: size}, nil
}

func (p *freeLayerPolicy) MeasureLayer(ctx LayerMeasureContext, children []LayerChild) (LayerMeasureResult, error) {
	if p == nil {
		return LayerMeasureResult{}, nil
	}
	measureChildrenLayer(ctx, children, ctx.Bounds)
	size := gfx.Size{W: ctx.Bounds.Width(), H: ctx.Bounds.Height()}
	return LayerMeasureResult{Size: size}, nil
}

func measureChildrenLayer(ctx LayerMeasureContext, children []LayerChild, bounds gfx.Rect) {
	available := gfx.Size{W: bounds.Width(), H: bounds.Height()}
	for i := range children {
		child := children[i]
		if child.Layout == nil {
			continue
		}
		child.Layout.Measure(facet.MeasureContext{
			Runtime:          ctx.Runtime,
			Theme:            ctx.Theme,
			Layer:            ctx.Layer,
			ParentGroup:      child.Layout.Parent,
			ChildGroup:       child.Layout.Child,
			ContentScale:     ctx.ContentScale,
			Density:          ctx.Density,
			WritingDirection: ctx.WritingDirection,
		}, facet.Constraints{
			MinSize: gfx.Size{},
			MaxSize: available,
		})
	}
}

func insetRect(bounds gfx.Rect, insets gfx.Insets) gfx.Rect {
	minX := bounds.Min.X + insets.Left
	minY := bounds.Min.Y + insets.Top
	maxX := bounds.Max.X - insets.Right
	maxY := bounds.Max.Y - insets.Bottom
	if maxX < minX {
		maxX = minX
	}
	if maxY < minY {
		maxY = minY
	}
	return gfx.RectFromXYWH(minX, minY, maxX-minX, maxY-minY)
}
