package runtime

import (
	"sort"

	"codeburg.org/lexbit/lurpicui/facet"
	"codeburg.org/lexbit/lurpicui/gfx"
	"codeburg.org/lexbit/lurpicui/layout"
)

func (rt *Runtime) copyDirtyFacets() map[facet.FacetID]facet.DirtyFlags {
	if len(rt.dirtyFacets) == 0 {
		return nil
	}
	out := make(map[facet.FacetID]facet.DirtyFlags, len(rt.dirtyFacets))
	for id, flags := range rt.dirtyFacets {
		if flags != 0 {
			out[id] = flags
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (rt *Runtime) attachTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	facet.Attach(root, facet.AttachContext{
		Runtime: rt,
		Assets:  facet.AssetServices{Manager: rt.assetManager},
		Stores:  facet.StoreServices{AssetRegistry: rt.config.AssetRegistry},
	})
}

func (rt *Runtime) activateTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	facet.Activate(root)
}

func (rt *Runtime) disposeTree(root facet.FacetImpl) {
	if root == nil {
		return
	}
	facet.Dispose(root)
}

func (rt *Runtime) markTreeDirty(root facet.FacetImpl, flags facet.DirtyFlags) {
	if root == nil {
		return
	}
	stack := []facet.FacetImpl{root}
	for len(stack) > 0 {
		node := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if node == nil || node.Base() == nil {
			continue
		}
		base := node.Base()
		base.InvalidateWithSource(flags, "runtime.markTreeDirty")
		rt.dirtyFacets[base.ID()] = flags
		rt.dirtySources[base.ID()] = "runtime.markTreeDirty"
		children := base.Children()
		for i := len(children) - 1; i >= 0; i-- {
			stack = append(stack, children[i])
		}
	}
}

func (rt *Runtime) hasLayoutDirty() bool {
	for _, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout != 0 {
			return true
		}
	}
	return false
}

func (rt *Runtime) runLayoutPass(windowSize gfx.Size) {
	if len(rt.dirtyFacets) == 0 {
		return
	}
	rt.invalidateDirtyLayoutCaches()
	roots := rt.selectedLayoutRoots()
	for _, root := range roots {
		if root == nil || root.Base() == nil {
			continue
		}
		bounds := gfx.RectFromXYWH(0, 0, windowSize.W, windowSize.H)
		if root.Base().Parent() != nil {
			layoutRole := root.Base().LayoutRole()
			if layoutRole != nil && !layoutRole.ArrangedBounds.IsEmpty() {
				bounds = layoutRole.ArrangedBounds
			}
		}
		rt.measureLayoutChild(root, layout.Loose(gfx.Size{W: bounds.Width(), H: bounds.Height()}))
		rt.arrangeLayoutChild(root, bounds)
		rt.clearLayoutDirtyTree(root)
	}
}

func (rt *Runtime) selectedLayoutRoots() []facet.FacetImpl {
	if len(rt.dirtyFacets) == 0 {
		return nil
	}
	roots := make([]facet.FacetImpl, 0, len(rt.dirtyFacets))
	for id := range rt.dirtyFacets {
		if rt.dirtyFacets[id]&facet.DirtyLayout == 0 {
			continue
		}
		if f := rt.findFacetByID(rt.root, id); f != nil {
			roots = append(roots, f)
		}
	}
	sort.SliceStable(roots, func(i, j int) bool {
		return roots[i].Base().ID() < roots[j].Base().ID()
	})
	filtered := roots[:0]
	for _, f := range roots {
		if !rt.hasLayoutDirtyAncestor(f) {
			filtered = append(filtered, f)
		}
	}
	return filtered
}

func (rt *Runtime) invalidateDirtyLayoutCaches() {
	if rt.root == nil {
		return
	}
	for id, flags := range rt.dirtyFacets {
		if flags&facet.DirtyLayout == 0 {
			continue
		}
		if f := rt.findFacetByID(rt.root, id); f != nil && f.Base() != nil {
			if role := f.Base().LayoutRole(); role != nil {
				role.InvalidateCache()
			}
		}
	}
}

func (rt *Runtime) hasLayoutDirtyAncestor(f facet.FacetImpl) bool {
	if f == nil || f.Base() == nil {
		return false
	}
	for parent := f.Base().Parent(); parent != nil; parent = parent.Parent() {
		if flags := rt.dirtyFacets[parent.ID()]; flags&facet.DirtyLayout != 0 {
			return true
		}
	}
	return false
}

func (rt *Runtime) clearLayoutDirtyTree(f facet.FacetImpl) {
	if f == nil || f.Base() == nil {
		return
	}
	if flags := f.Base().DirtyFlags(); flags&facet.DirtyLayout != 0 {
		f.Base().ClearDirty(facet.DirtyLayout)
	}
	for _, child := range f.Base().Children() {
		if child != nil {
			rt.clearLayoutDirtyTree(child)
		}
	}
}

func (rt *Runtime) measureLayoutChild(f facet.FacetImpl, c layout.Constraints) gfx.Size {
	if f == nil || f.Base() == nil {
		return gfx.Size{}
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return gfx.Size{}
	}
	parentBounds := gfx.RectFromXYWH(0, 0, c.MaxSize.W, c.MaxSize.H)
	var themeCtx any
	var contentScale float32 = 1
	if rt != nil {
		themeCtx = rt.themeContext(parentBounds)
		contentScale = rt.contentScale
	}
	return role.Measure(facet.MeasureContext{
		Runtime:      rt,
		Theme:        themeCtx,
		ContentScale: contentScale,
	}, c).Size
}

func (rt *Runtime) arrangeLayoutChild(f facet.FacetImpl, bounds gfx.Rect) {
	if f == nil || f.Base() == nil {
		return
	}
	role := f.Base().LayoutRole()
	if role == nil {
		return
	}
	var themeCtx any
	if rt != nil {
		themeCtx = rt.themeContext(bounds)
	}
	role.Arrange(facet.ArrangeContext{
		Runtime: rt,
		Theme:   themeCtx,
	}, bounds)
}

func boundsSize(bounds gfx.Rect) gfx.Size {
	return gfx.Size{W: bounds.Width(), H: bounds.Height()}
}
