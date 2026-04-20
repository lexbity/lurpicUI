package projected

import (
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

// WorldPositioned exposes a child's world-space geometry to the runtime.
type WorldPositioned interface {
	WorldPosition() gfx.Point
	WorldSize() gfx.Size
}

// Policy resolves child positions by applying the layer transform to their world-space geometry.
type Policy struct{}

// New constructs a projected-placement policy.
func New() *Policy {
	return &Policy{}
}

// Measure always returns zero size.
func (p *Policy) Measure(children []layout.ChildNode, constraints gfx.Size) gfx.Size {
	return gfx.Size{}
}

// Arrange positions each child from its world-space position and size.
func (p *Policy) Arrange(children []layout.ChildNode, layer layout.ResolvedLayer) {
	if p == nil || len(children) == 0 {
		return
	}
	for i := range children {
		child := children[i]
		if !child.HasWorldSpace {
			children[i].SetArrangedBounds(gfx.Rect{})
			continue
		}
		pos := layer.Transform.TransformPoint(child.WorldPosition)
		size := transformSize(layer.Transform, child.WorldSize)
		children[i].SetArrangedBounds(gfx.RectFromXYWH(pos.X, pos.Y, size.W, size.H))
	}
}

func transformSize(t gfx.Transform, size gfx.Size) gfx.Size {
	origin := t.TransformPoint(gfx.Point{})
	extent := t.TransformPoint(gfx.Point{X: size.W, Y: size.H})
	return gfx.Size{
		W: extent.X - origin.X,
		H: extent.Y - origin.Y,
	}
}
