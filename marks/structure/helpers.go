package structure

import (
	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
	"codeburg.org/lexbit/lurpicui/marks"
	"fmt"
)

func registerStructureDescriptor(d marks.Descriptor) {
	marks.RegisterDescriptor(d)
}

func syncLayout(layoutRole *facet.LayoutRole, bounds gfx.Rect) {
	if layoutRole == nil {
		return
	}
	layoutRole.Arrange(bounds)
	layoutRole.MeasuredSize = gfx.Size{W: bounds.Width(), H: bounds.Height()}
}

func syncViewport(viewport *facet.ViewportRole, transform gfx.Transform) {
	if viewport == nil {
		return
	}
	viewport.Transform = transform
}

func attachChildMarks(parent *facet.Facet, children []marks.Mark) {
	if parent == nil {
		return
	}
	for _, child := range children {
		if child == nil {
			continue
		}
		impl, ok := child.(facet.FacetImpl)
		if !ok {
			panic("marks/structure: child mark does not implement facet.FacetImpl")
		}
		parent.AddChild(impl.Base())
	}
}

func attachSingleChild(parent *facet.Facet, child marks.Mark) {
	if child == nil {
		return
	}
	attachChildMarks(parent, []marks.Mark{child})
}

func normaliseTransform(t gfx.Transform) gfx.Transform {
	if t == (gfx.Transform{}) {
		return gfx.Identity()
	}
	return t
}

func transformAnchors(tx gfx.Transform, anchors layout.AnchorSet) layout.AnchorSet {
	if len(anchors) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(anchors))
	for id, pt := range anchors {
		out[id] = tx.TransformPoint(pt)
	}
	return out
}

func unionDescendantBounds(base *facet.Facet) (gfx.Rect, bool) {
	if base == nil {
		return gfx.Rect{}, false
	}
	type frame struct {
		base *facet.Facet
		tx   gfx.Transform
	}
	stack := []frame{{base: base, tx: gfx.Identity()}}
	var bounds gfx.Rect
	have := false
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current.base == nil {
			continue
		}
		for _, child := range current.base.Children() {
			if child == nil {
				continue
			}
			childTx := current.tx
			if viewport := child.ViewportRole(); viewport != nil {
				childTx = childTx.Multiply(viewport.Transform)
			}
			if layoutRole := child.LayoutRole(); layoutRole != nil && !layoutRole.ArrangedBounds.IsEmpty() {
				rect := childTx.TransformRect(layoutRole.ArrangedBounds)
				if !have {
					bounds = rect
					have = true
				} else {
					bounds = bounds.Union(rect)
				}
			}
			if len(child.Children()) > 0 {
				stack = append(stack, frame{base: child, tx: childTx})
			}
		}
	}
	return bounds, have
}

func boundsAnchors(bounds gfx.Rect) layout.AnchorSet {
	if bounds.IsEmpty() {
		return nil
	}
	return layout.AnchorSet{
		"bounds-center": {X: bounds.Min.X + bounds.Width()/2, Y: bounds.Min.Y + bounds.Height()/2},
		"top-left":      {X: bounds.Min.X, Y: bounds.Min.Y},
		"top-right":     {X: bounds.Max.X, Y: bounds.Min.Y},
		"bottom-right":  {X: bounds.Max.X, Y: bounds.Max.Y},
		"bottom-left":   {X: bounds.Min.X, Y: bounds.Max.Y},
	}
}

func findMarkedChild(base *facet.Facet, markID string) facet.FacetImpl {
	if base == nil || markID == "" {
		return nil
	}
	stack := []*facet.Facet{base}
	for len(stack) > 0 {
		current := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if current == nil {
			continue
		}
		impl := current.Impl()
		if impl != nil {
			if authored, ok := impl.(interface{ AuthoredID() string }); ok && authored.AuthoredID() == markID || fmt.Sprint(current.ID()) == markID {
				return impl
			}
		}
		children := current.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
	return nil
}

func renameAnchors(src layout.AnchorSet, rename map[string]string, offset gfx.Point) layout.AnchorSet {
	if len(src) == 0 {
		return nil
	}
	out := make(layout.AnchorSet, len(src))
	for id, pt := range src {
		name := string(id)
		if rename != nil {
			if next, ok := rename[name]; ok && next != "" {
				name = next
			}
		}
		out[layout.AnchorID(name)] = gfx.Point{X: pt.X + offset.X, Y: pt.Y + offset.Y}
	}
	return out
}

func pathBounds(path gfx.Path) gfx.Rect {
	if len(path.Segments) == 0 {
		return gfx.Rect{}
	}
	var minPt, maxPt gfx.Point
	havePoint := false
	visit := func(p gfx.Point) {
		if !havePoint {
			minPt = p
			maxPt = p
			havePoint = true
			return
		}
		if p.X < minPt.X {
			minPt.X = p.X
		}
		if p.Y < minPt.Y {
			minPt.Y = p.Y
		}
		if p.X > maxPt.X {
			maxPt.X = p.X
		}
		if p.Y > maxPt.Y {
			maxPt.Y = p.Y
		}
	}
	for _, seg := range path.Segments {
		for _, p := range seg.Pts {
			visit(p)
		}
	}
	if !havePoint {
		return gfx.Rect{}
	}
	return gfx.Rect{Min: minPt, Max: maxPt}
}
