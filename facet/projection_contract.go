package facet

import "codeburg.org/lexbit/lurpicui/gfx"

// IsZero reports whether the layer carries no resolved spatial contract.
func (l ProjectionLayer) IsZero() bool {
	return l == (ProjectionLayer{})
}

// ResolvedLayer returns the layer contract that projection should consume.
func (c ProjectionContext) ResolvedLayer() ProjectionLayer {
	return c.Layer
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

// LayerID returns the resolved layer identifier, if available.
func (c ProjectionContext) LayerID() LayerID {
	return c.ResolvedLayer().LayerID
}

// LayerRecipeVersion returns the resolved layer recipe version.
func (c ProjectionContext) LayerRecipeVersion() uint64 {
	return c.ResolvedLayer().RecipeVersion
}
