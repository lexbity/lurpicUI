package facet

import "codeburg.org/lexbit/lurpicui/gfx"

// IsZero reports whether the layer carries no resolved spatial contract.
func (l ProjectionLayer) IsZero() bool {
	return l == (ProjectionLayer{})
}

// ResolvedLayer returns the layer contract that projection should consume.
//
// When a runtime has already resolved a layer, that resolved record wins.
// When projection is invoked without a resolved layer snapshot, the context
// falls back to the explicit local bounds and viewport transform supplied by
// the facet.
func (c ProjectionContext) ResolvedLayer() ProjectionLayer {
	if !c.Layer.IsZero() {
		return c.Layer
	}
	layer := ProjectionLayer{
		Bounds:   c.Bounds,
		ClipRect: c.Bounds,
	}
	if c.Viewport != nil {
		layer.Transform = c.Viewport.Transform
	} else {
		layer.Transform = gfx.Identity()
	}
	return layer
}

// LayerBounds returns the resolved layer bounds.
func (c ProjectionContext) LayerBounds() gfx.Rect {
	return c.ResolvedLayer().Bounds
}

// LayerTransform returns the resolved layer transform.
func (c ProjectionContext) LayerTransform() gfx.Transform {
	return c.ResolvedLayer().Transform
}

// LayerClipRect returns the resolved layer clip rect.
func (c ProjectionContext) LayerClipRect() gfx.Rect {
	return c.ResolvedLayer().ClipRect
}
